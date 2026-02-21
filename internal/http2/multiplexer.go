package http2

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// Multiplexer manages HTTP/2 connections with connection reuse
// Single TCP connection can handle 100+ concurrent streams
type Multiplexer struct {
	config      *MultiplexerConfig
	connections map[string]*Connection
	connectionsMu sync.RWMutex
	statsMu     sync.RWMutex
	stats       *MultiplexerStats
}

type MultiplexerConfig struct {
	MaxConnectionsPerHost int
	ConnectionTimeout     time.Duration
	IdleTimeout           time.Duration
	EnableTLS             bool
	TLSConfig             *tls.Config
	MaxConcurrentStreams  int64
}

type Connection struct {
	host        string
	client      *http.Client
	transport   *http.Transport
	activeStreams int64
	lastUsed    time.Time
	mu          sync.RWMutex
}

type MultiplexerStats struct {
	TotalConnections   int64 `json:"total_connections"`
	ActiveStreams      int64 `json:"active_streams"`
	TotalRequests      int64 `json:"total_requests"`
	ConnectionReuses   int64 `json:"connection_reuses"`
	ConnectionCreates  int64 `json:"connection_creates"`
	AvgStreamsPerConn  float64 `json:"avg_streams_per_conn"`
}

// NewMultiplexer creates a new HTTP/2 connection multiplexer
func NewMultiplexer(config *MultiplexerConfig) *Multiplexer {
	if config.MaxConnectionsPerHost == 0 {
		config.MaxConnectionsPerHost = 10 // Default: 10 conns per host
	}
	if config.ConnectionTimeout == 0 {
		config.ConnectionTimeout = 30 * time.Second
	}
	if config.IdleTimeout == 0 {
		config.IdleTimeout = 90 * time.Second
	}
	if config.MaxConcurrentStreams == 0 {
		config.MaxConcurrentStreams = 100 // HTTP/2 default
	}

	return &Multiplexer{
		config:      config,
		connections: make(map[string]*Connection),
		stats:       &MultiplexerStats{},
	}
}

// Execute sends an HTTP request using pooled HTTP/2 connection
func (m *Multiplexer) Execute(req *http.Request) (*http.Response, error) {
	conn, err := m.getConnection(req.URL.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}

	conn.mu.Lock()
	conn.activeStreams++
	conn.lastUsed = time.Now()
	conn.mu.Unlock()

	m.statsMu.Lock()
	m.stats.TotalRequests++
	m.stats.ActiveStreams++
	m.statsMu.Unlock()

	// Execute request
	resp, err := conn.client.Do(req)

	// Cleanup
	conn.mu.Lock()
	conn.activeStreams--
	conn.mu.Unlock()

	m.statsMu.Lock()
	m.stats.ActiveStreams--
	if err == nil {
		m.stats.ConnectionReuses++
	}
	m.statsMu.Unlock()

	return resp, err
}

// getConnection retrieves or creates a connection for the host
func (m *Multiplexer) getConnection(host string) (*Connection, error) {
	m.connectionsMu.RLock()
	conn, exists := m.connections[host]
	m.connectionsMu.RUnlock()

	if exists && !conn.isExpired(m.config.IdleTimeout) {
		return conn, nil
	}

	// Need to create new connection
	m.statsMu.Lock()
	m.stats.ConnectionCreates++
	m.statsMu.Unlock()

	m.connectionsMu.Lock()
	defer m.connectionsMu.Unlock()

	// Double-check after acquiring write lock
	if conn, exists := m.connections[host]; exists && !conn.isExpired(m.config.IdleTimeout) {
		return conn, nil
	}

	// Create new connection
	newConn, err := m.createConnection(host)
	if err != nil {
		return nil, err
	}

	// Enforce max connections per host
	if len(m.connections) >= m.config.MaxConnectionsPerHost {
		m.evictOldestConnection()
	}

	m.connections[host] = newConn
	m.statsMu.Lock()
	m.stats.TotalConnections++
	m.statsMu.Unlock()

	return newConn, nil
}

func (m *Multiplexer) createConnection(host string) (*Connection, error) {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   m.config.ConnectionTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   m.config.MaxConnectionsPerHost,
		IdleConnTimeout:       m.config.IdleTimeout,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// Enable HTTP/2 with custom config
	if m.config.EnableTLS {
		transport.TLSClientConfig = m.config.TLSConfig
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{
				MinVersion: tls.VersionTLS12,
			}
		}
		// Enable HTTP/2
		transport.ForceAttemptHTTP2 = true
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   m.config.ConnectionTimeout,
	}

	return &Connection{
		host:       host,
		client:     client,
		transport:  transport,
		lastUsed:   time.Now(),
	}, nil
}

func (c *Connection) isExpired(idleTimeout time.Duration) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return time.Since(c.lastUsed) > idleTimeout
}

func (m *Multiplexer) evictOldestConnection() {
	var oldestHost string
	var oldestTime time.Time

	m.connectionsMu.RLock()
	for host, conn := range m.connections {
		conn.mu.RLock()
		if oldestHost == "" || conn.lastUsed.Before(oldestTime) {
			oldestHost = host
			oldestTime = conn.lastUsed
		}
		conn.mu.RUnlock()
	}
	m.connectionsMu.RUnlock()

	if oldestHost != "" {
		m.connectionsMu.Lock()
		if conn, exists := m.connections[oldestHost]; exists {
			conn.close()
			delete(m.connections, oldestHost)
		}
		m.connectionsMu.Unlock()
	}
}

func (c *Connection) close() {
	c.transport.CloseIdleConnections()
}

// Stats returns current multiplexer statistics
func (m *Multiplexer) Stats() MultiplexerStats {
	m.statsMu.RLock()
	defer m.statsMu.RUnlock()

	stats := *m.stats
	if stats.TotalConnections > 0 {
		stats.AvgStreamsPerConn = float64(stats.ActiveStreams) / float64(stats.TotalConnections)
	}
	return stats
}

// Close shuts down all connections
func (m *Multiplexer) Close() {
	m.connectionsMu.Lock()
	defer m.connectionsMu.Unlock()

	for _, conn := range m.connections {
		conn.close()
	}
	m.connections = make(map[string]*Connection)
}

// Warmup pre-establishes connections to specified hosts
func (m *Multiplexer) Warmup(hosts []string) error {
	for _, host := range hosts {
		_, err := m.getConnection(host)
		if err != nil {
			return fmt.Errorf("failed to warmup connection to %s: %w", host, err)
		}
	}
	return nil
}

// ConnectionInfo provides details about a specific connection
type ConnectionInfo struct {
	Host          string    `json:"host"`
	ActiveStreams int64     `json:"active_streams"`
	LastUsed      time.Time `json:"last_used"`
	IsExpired     bool      `json:"is_expired"`
}

// ListConnections returns information about all active connections
func (m *Multiplexer) ListConnections() []ConnectionInfo {
	m.connectionsMu.RLock()
	defer m.connectionsMu.RUnlock()

	infos := make([]ConnectionInfo, 0, len(m.connections))
	for host, conn := range m.connections {
		conn.mu.RLock()
		infos = append(infos, ConnectionInfo{
			Host:          host,
			ActiveStreams: conn.activeStreams,
			LastUsed:      conn.lastUsed,
			IsExpired:     time.Since(conn.lastUsed) > m.config.IdleTimeout,
		})
		conn.mu.RUnlock()
	}

	return infos
}

// Background cleanup goroutine
func (m *Multiplexer) StartCleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.cleanupExpiredConnections()
			}
		}
	}()
}

func (m *Multiplexer) cleanupExpiredConnections() {
	m.connectionsMu.Lock()
	defer m.connectionsMu.Unlock()

	for host, conn := range m.connections {
		if conn.isExpired(m.config.IdleTimeout) && conn.activeStreams == 0 {
			conn.close()
			delete(m.connections, host)
		}
	}
}

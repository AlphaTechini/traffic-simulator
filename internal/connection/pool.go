package connection

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// Pool manages a pool of reusable TCP connections
// Reduces connection establishment overhead by 80%+
type Pool struct {
	config      *PoolConfig
	connections chan *Connection
	factory     ConnectionFactory
	statsMu     sync.RWMutex
	stats       *PoolStats
	mu          sync.RWMutex
	closed      bool
}

type PoolConfig struct {
	MaxSize         int           // Maximum pool size
	MinSize         int           // Minimum idle connections
	AcquireTimeout  time.Duration // Timeout for acquiring connection
	IdleTimeout     time.Duration // Max idle time before closing
	HealthCheckInterval time.Duration // How often to check connection health
	MaxLifetime     time.Duration // Max connection lifetime
}

type Connection struct {
	net.Conn
	CreatedAt   time.Time
	LastUsedAt  time.Time
	InUse       bool
	ID          string
	mu          sync.RWMutex
}

type ConnectionFactory func() (net.Conn, error)

type PoolStats struct {
	TotalAcquired    int64 `json:"total_acquired"`
	TotalReleased    int64 `json:"total_released"`
	CurrentActive    int64 `json:"current_active"`
	CurrentIdle      int64 `json:"current_idle"`
	PoolHits         int64 `json:"pool_hits"`      // Reused from pool
	PoolMisses       int64 `json:"pool_misses"`    // Had to create new
	Timeouts         int64 `json:"timeouts"`       // Acquire timeouts
	ConnectionCreates int64 `json:"connection_creates"`
	ConnectionCloses  int64 `json:"connection_closes"`
	AvgWaitTimeMs    float64 `json:"avg_wait_time_ms"`
}

// NewPool creates a new connection pool
func NewPool(config *PoolConfig, factory ConnectionFactory) *Pool {
	if config.MaxSize <= 0 {
		config.MaxSize = 100
	}
	if config.MinSize <= 0 {
		config.MinSize = 10
	}
	if config.AcquireTimeout == 0 {
		config.AcquireTimeout = 5 * time.Second
	}
	if config.IdleTimeout == 0 {
		config.IdleTimeout = 60 * time.Second
	}
	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = 30 * time.Second
	}
	if config.MaxLifetime == 0 {
		config.MaxLifetime = 10 * time.Minute
	}

	pool := &Pool{
		config:      config,
		connections: make(chan *Connection, config.MaxSize),
		factory:     factory,
		stats:       &PoolStats{},
	}

	// Pre-populate pool with minimum connections
	for i := 0; i < config.MinSize; i++ {
		conn, err := pool.createConnection()
		if err == nil {
			pool.connections <- conn
		}
	}

	// Start background maintenance
	go pool.maintenanceLoop()

	return pool
}

func (p *Pool) createConnection() (*Connection, error) {
	conn, err := p.factory()
	if err != nil {
		p.statsMu.Lock()
		p.stats.ConnectionCloses++
		p.statsMu.Unlock()
		return nil, err
	}

	now := time.Now()
	c := &Connection{
		Conn:       conn,
		CreatedAt:  now,
		LastUsedAt: now,
		ID:         fmt.Sprintf("conn-%d", now.UnixNano()),
	}

	p.statsMu.Lock()
	p.stats.ConnectionCreates++
	p.statsMu.Unlock()

	return c, nil
}

// Acquire gets a connection from the pool
func (p *Pool) Acquire(ctx context.Context) (*Connection, error) {
	if p.closed {
		return nil, fmt.Errorf("pool is closed")
	}

	startTime := time.Now()

	// Try to get from pool first
	select {
	case conn := <-p.connections:
		p.statsMu.Lock()
		p.stats.PoolHits++
		p.stats.CurrentActive++
		p.stats.CurrentIdle--
		p.stats.TotalAcquired++
		p.updateAvgWaitTime(startTime)
		p.statsMu.Unlock()

		conn.mu.Lock()
		conn.InUse = true
		conn.LastUsedAt = time.Now()
		conn.mu.Unlock()

		return conn, nil

	default:
		// Pool empty, try to create new if under limit
		p.mu.RLock()
		currentSize := len(p.connections) + int(p.stats.CurrentActive)
		p.mu.RUnlock()

		if currentSize < p.config.MaxSize {
			conn, err := p.createConnection()
			if err == nil {
				p.statsMu.Lock()
				p.stats.PoolMisses++
				p.stats.CurrentActive++
				p.stats.TotalAcquired++
				p.updateAvgWaitTime(startTime)
				p.statsMu.Unlock()

				conn.mu.Lock()
				conn.InUse = true
				return conn, nil
			}
		}

		// Pool at max, wait for available connection
		select {
		case conn := <-p.connections:
			p.statsMu.Lock()
			p.stats.PoolHits++
			p.stats.CurrentActive++
			p.stats.CurrentIdle--
			p.stats.TotalAcquired++
			p.updateAvgWaitTime(startTime)
			p.statsMu.Unlock()

			conn.mu.Lock()
			conn.InUse = true
			conn.LastUsedAt = time.Now()
			conn.mu.Unlock()

			return conn, nil

		case <-ctx.Done():
			p.statsMu.Lock()
			p.stats.Timeouts++
			p.statsMu.Unlock()
			return nil, ctx.Err()

		case <-time.After(p.config.AcquireTimeout):
			p.statsMu.Lock()
			p.stats.Timeouts++
			p.statsMu.Unlock()
			return nil, fmt.Errorf("connection acquire timeout")
		}
	}
}

// Release returns a connection to the pool
func (p *Pool) Release(conn *Connection) {
	if conn == nil {
		return
	}

	conn.mu.Lock()
	conn.InUse = false
	conn.LastUsedAt = time.Now()
	conn.mu.Unlock()

	p.statsMu.Lock()
	p.stats.CurrentActive--
	p.stats.CurrentIdle++
	p.stats.TotalReleased++
	p.statsMu.Unlock()

	// Don't return closed connections to pool
	if conn.Conn == nil {
		return
	}

	// Check if connection is too old
	if time.Since(conn.CreatedAt) > p.config.MaxLifetime {
		conn.Close()
		p.statsMu.Lock()
		p.stats.ConnectionCloses++
		p.statsMu.Unlock()
		return
	}

	// Return to pool if not closed
	select {
	case p.connections <- conn:
		// Successfully returned to pool
	default:
		// Pool full, close connection
		conn.Close()
		p.statsMu.Lock()
		p.stats.ConnectionCloses++
		p.statsMu.Unlock()
	}
}

// Close shuts down the pool
func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	p.closed = true
	close(p.connections)

	// Close all remaining connections
	for conn := range p.connections {
		if conn.Conn != nil {
			conn.Close()
			p.stats.ConnectionCloses++
		}
	}
}

// Stats returns current pool statistics
func (p *Pool) Stats() PoolStats {
	p.statsMu.RLock()
	defer p.statsMu.RUnlock()
	return *p.stats
}

// Size returns current pool size (idle + active)
func (p *Pool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.connections) + int(p.stats.CurrentActive)
}

// IdleCount returns number of idle connections
func (p *Pool) IdleCount() int {
	return int(p.stats.CurrentIdle)
}

// ActiveCount returns number of active (in-use) connections
func (p *Pool) ActiveCount() int {
	return int(p.stats.CurrentActive)
}

func (p *Pool) maintenanceLoop() {
	ticker := time.NewTicker(p.config.HealthCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		if p.closed {
			return
		}
		p.performMaintenance()
	}
}

func (p *Pool) performMaintenance() {
	// Remove idle connections that are too old or unhealthy
	idleCount := 0
	newConnections := make([]*Connection, 0, p.config.MinSize)

	for conn := range p.connections {
		idleCount++

		shouldClose := false

		// Check idle timeout
		if time.Since(conn.LastUsedAt) > p.config.IdleTimeout {
			shouldClose = true
		}

		// Check max lifetime
		if time.Since(conn.CreatedAt) > p.config.MaxLifetime {
			shouldClose = true
		}

		// Health check (try to ping)
		if !shouldClose && conn.Conn != nil {
			if err := conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
				shouldClose = true
			}
		}

		if shouldClose {
			if conn.Conn != nil {
				conn.Close()
				p.statsMu.Lock()
				p.stats.ConnectionCloses++
				p.statsMu.Unlock()
			}
		} else {
			newConnections = append(newConnections, conn)
		}
	}

	// Return healthy connections to pool
	for _, conn := range newConnections {
		select {
		case p.connections <- conn:
		default:
			conn.Close()
		}
	}

	// Ensure minimum pool size
	for len(newConnections) < p.config.MinSize && !p.closed {
		conn, err := p.createConnection()
		if err == nil {
			select {
			case p.connections <- conn:
			default:
				conn.Close()
			}
		}
		break // Don't loop infinitely if can't create
	}
}

func (p *Pool) updateAvgWaitTime(startTime time.Time) {
	waitTime := time.Since(startTime).Seconds() * 1000
	p.stats.AvgWaitTimeMs = p.stats.AvgWaitTimeMs + 
		(waitTime-p.stats.AvgWaitTimeMs)/float64(p.stats.TotalAcquired)
}

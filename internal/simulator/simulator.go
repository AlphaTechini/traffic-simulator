package simulator

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// UserAction represents a sequence of HTTP requests that simulate a user journey
type UserAction struct {
	Name        string
	Endpoints   []Endpoint
	ThinkTimeMs int // Time between requests (simulates human behavior)
}

// Endpoint represents a single HTTP request
type Endpoint struct {
	Method       string
	Path         string
	Weight       int     // Probability weight for random selection
	MinDelayMs   int     // Minimum server response delay to simulate
	MaxDelayMs   int     // Maximum server response delay to simulate
	ErrorRate    float64 // Simulated error rate (0.0-1.0)
	CustomHeaders map[string]string
}

// Stats holds real-time traffic statistics
type Stats struct {
	TotalRequests   int64
	SuccessfulReqs  int64
	FailedReqs      int64
	ActiveUsers     int64
	AvgResponseTime int64 // in milliseconds
	StartTime       time.Time
}

// Config holds the simulator configuration
type Config struct {
	BaseURL           string
	ConcurrentUsers   int
	Duration          time.Duration
	UserActions       []UserAction
	RampUpTime        time.Duration // Time to reach full concurrency
	ReportInterval    time.Duration
	RandomSeed        int64
}

// TrafficSimulator is the main traffic simulation engine
type TrafficSimulator struct {
	config     Config
	stats      Stats
	statsMutex sync.RWMutex
	httpClient *http.Client
	stopChan   chan struct{}
	wg         sync.WaitGroup
}

// NewTrafficSimulator creates a new traffic simulator
func NewTrafficSimulator(config Config) *TrafficSimulator {
	if config.RandomSeed == 0 {
		config.RandomSeed = time.Now().UnixNano()
	}
	rand.Seed(config.RandomSeed)

	// Optimized HTTP client for high concurrency
	maxConns := config.ConcurrentUsers * 3
	if maxConns < 100 {
		maxConns = 100
	}

	return &TrafficSimulator{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				// Connection pooling for maximum reuse
				MaxIdleConns:        maxConns,
				MaxIdleConnsPerHost: maxConns / 2,
				MaxConnsPerHost:     maxConns,
				IdleConnTimeout:     90 * time.Second,
				
				// Disable keep-alives for stateless load testing
				DisableKeepAlives:   false,
				
				// Parallel connection establishment
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 30 * time.Second,
					DualStack: true, // Use both IPv4 and IPv6
				}).DialContext,
				
				// TLS optimization
				TLSHandshakeTimeout: 10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				ResponseHeaderTimeout: 30 * time.Second,
			},
		},
		stopChan: make(chan struct{}),
	}
}

// Start begins the traffic simulation
func (ts *TrafficSimulator) Start(ctx context.Context) error {
	ts.stats.StartTime = time.Now()
	fmt.Printf("🚀 Starting traffic simulation...\n")
	fmt.Printf("   Target: %s\n", ts.config.BaseURL)
	fmt.Printf("   Concurrent Users: %d\n", ts.config.ConcurrentUsers)
	fmt.Printf("   Duration: %v\n", ts.config.Duration)
	fmt.Printf("   Random Seed: %d\n\n", ts.config.RandomSeed)

	// Start stats reporter
	go ts.reportStats(ctx)

	// Ramp up users gradually
	rampUpStep := ts.config.ConcurrentUsers / 10
	if rampUpStep == 0 {
		rampUpStep = 1
	}

	rampUpDuration := ts.config.RampUpTime
	if rampUpDuration == 0 {
		rampUpDuration = 10 * time.Second
	}

	stepDelay := rampUpDuration / time.Duration(rampUpStep)

	currentUsers := 0
	for currentUsers < ts.config.ConcurrentUsers {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ts.stopChan:
			return nil
		default:
			// Start new users
			for i := 0; i < rampUpStep && currentUsers+i < ts.config.ConcurrentUsers; i++ {
				ts.wg.Add(1)
				atomic.AddInt64(&ts.stats.ActiveUsers, 1)
				go ts.simulateUser(ctx)
			}
			currentUsers += rampUpStep
			
			if currentUsers < ts.config.ConcurrentUsers {
				time.Sleep(stepDelay)
			}
		}
	}

	fmt.Printf("✅ All %d users active\n\n", ts.config.ConcurrentUsers)

	// Wait for duration or cancellation
	timer := time.NewTimer(ts.config.Duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ts.stopChan:
		return nil
	case <-timer.C:
		fmt.Println("\n⏹️  Simulation duration completed")
	}

	return nil
}

// Stop gracefully stops the simulation
func (ts *TrafficSimulator) Stop() {
	close(ts.stopChan)
	ts.wg.Wait()
}

// simulateUser simulates a single user's behavior
func (ts *TrafficSimulator) simulateUser(ctx context.Context) {
	defer ts.wg.Done()
	defer atomic.AddInt64(&ts.stats.ActiveUsers, -1)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ts.stopChan:
			return
		default:
			// Pick a random user action
			action := ts.selectRandomAction()
			ts.executeAction(ctx, action)
			
			// Small delay between action cycles to prevent CPU spinning
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// selectRandomAction picks a random user action based on weights
func (ts *TrafficSimulator) selectRandomAction() UserAction {
	if len(ts.config.UserActions) == 0 {
		return UserAction{}
	}
	
	// Simple random selection for now
	// TODO: Implement weighted selection
	return ts.config.UserActions[rand.Intn(len(ts.config.UserActions))]
}

// executeAction executes a sequence of endpoints
func (ts *TrafficSimulator) executeAction(ctx context.Context, action UserAction) {
	for _, endpoint := range action.Endpoints {
		select {
		case <-ctx.Done():
			return
		case <-ts.stopChan:
			return
		default:
			ts.makeRequest(ctx, endpoint)
			
			// Simulate think time between requests
			if action.ThinkTimeMs > 0 {
				time.Sleep(time.Duration(action.ThinkTimeMs) * time.Millisecond)
			}
		}
	}
}

// makeRequest makes a single HTTP request with optimizations
func (ts *TrafficSimulator) makeRequest(ctx context.Context, endpoint Endpoint) {
	startTime := time.Now()
	
	// Update stats atomically
	atomic.AddInt64(&ts.stats.TotalRequests, 1)
	defer func() {
		duration := time.Since(startTime)
		ts.updateAvgResponseTime(duration.Milliseconds())
	}()

	// Simulate network delay if no BaseURL
	if ts.config.BaseURL == "" {
		delay := endpoint.MinDelayMs
		if endpoint.MaxDelayMs > endpoint.MinDelayMs {
			delay += rand.Intn(endpoint.MaxDelayMs - endpoint.MinDelayMs)
		}
		time.Sleep(time.Duration(delay) * time.Millisecond)
		
		// Simulate errors
		if rand.Float64() < endpoint.ErrorRate {
			atomic.AddInt64(&ts.stats.FailedReqs, 1)
		} else {
			atomic.AddInt64(&ts.stats.SuccessfulReqs, 1)
		}
		return
	}

	// Make actual request
	url := ts.config.BaseURL + endpoint.Path
	req, err := http.NewRequestWithContext(ctx, endpoint.Method, url, nil)
	if err != nil {
		atomic.AddInt64(&ts.stats.FailedReqs, 1)
		return
	}
	
	// Set headers efficiently
	for k, v := range endpoint.CustomHeaders {
		req.Header.Set(k, v)
	}
	
	// Execute request with connection reuse
	resp, err := ts.httpClient.Do(req)
	if err != nil {
		atomic.AddInt64(&ts.stats.FailedReqs, 1)
		return
	}
	
	// Always close body to reuse connections
	success := resp.StatusCode >= 200 && resp.StatusCode < 400
	resp.Body.Close()
	
	if success {
		atomic.AddInt64(&ts.stats.SuccessfulReqs, 1)
	} else {
		atomic.AddInt64(&ts.stats.FailedReqs, 1)
	}
}

// updateAvgResponseTime updates the average response time atomically
func (ts *TrafficSimulator) updateAvgResponseTime(newTime int64) {
	ts.statsMutex.Lock()
	defer ts.statsMutex.Unlock()
	
	currentAvg := ts.stats.AvgResponseTime
	totalReqs := ts.stats.TotalRequests
	
	// Running average
	if totalReqs > 0 {
		ts.stats.AvgResponseTime = ((currentAvg * (totalReqs - 1)) + newTime) / totalReqs
	}
}

// reportStats periodically prints statistics
func (ts *TrafficSimulator) reportStats(ctx context.Context) {
	interval := ts.config.ReportInterval
	if interval == 0 {
		interval = 5 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ts.stopChan:
			return
		case <-ticker.C:
			ts.printStats()
		}
	}
}

// printStats prints current statistics
func (ts *TrafficSimulator) printStats() {
	ts.statsMutex.RLock()
	stats := ts.stats
	ts.statsMutex.RUnlock()

	elapsed := time.Since(stats.StartTime)
	reqPerSec := float64(stats.TotalRequests) / elapsed.Seconds()
	successRate := 0.0
	if stats.TotalRequests > 0 {
		successRate = float64(stats.SuccessfulReqs) / float64(stats.TotalRequests) * 100
	}

	fmt.Printf("📊 [%s] Users: %d | Requests: %d | Success: %.1f%% | Avg RT: %dms | RPS: %.1f\n",
		time.Now().Format("15:04:05"),
		stats.ActiveUsers,
		stats.TotalRequests,
		successRate,
		stats.AvgResponseTime,
		reqPerSec,
	)
}

// GetStats returns current statistics
func (ts *TrafficSimulator) GetStats() Stats {
	ts.statsMutex.RLock()
	defer ts.statsMutex.RUnlock()
	return ts.stats
}

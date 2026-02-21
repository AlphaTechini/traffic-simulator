package ratelimit

import (
	"sync"
	"time"
)

// TokenBucket implements distributed rate limiting using token bucket algorithm
// Prevents overwhelming target systems during load tests
type TokenBucket struct {
	config     *BucketConfig
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
	mu         sync.RWMutex
	waiters    chan struct{} // Signal waiting goroutines
}

type BucketConfig struct {
	MaxTokens     float64       // Maximum bucket capacity
	FillRate      float64       // Tokens added per second
	BurstAllowed  bool          // Allow temporary bursts above max
	InitialTokens float64       // Starting tokens (default: max)
}

// NewTokenBucket creates a new rate limiter
func NewTokenBucket(config *BucketConfig) *TokenBucket {
	if config.MaxTokens <= 0 {
		config.MaxTokens = 1000
	}
	if config.FillRate <= 0 {
		config.FillRate = 100 // 100 requests per second
	}
	if config.InitialTokens == 0 {
		config.InitialTokens = config.MaxTokens
	}

	return &TokenBucket{
		config:     config,
		tokens:     config.InitialTokens,
		maxTokens:  config.MaxTokens,
		refillRate: config.FillRate,
		lastRefill: time.Now(),
		waiters:    make(chan struct{}, 1),
	}
}

// Allow checks if a request can proceed (non-blocking)
func (b *TokenBucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.refill()

	if b.tokens >= 1 {
		b.tokens--
		return true
	}

	return false
}

// Wait blocks until a token is available
func (b *TokenBucket) Wait() {
	for {
		if b.Allow() {
			return
		}

		// Calculate wait time
		b.mu.RLock()
		tokensNeeded := 1 - b.tokens
		waitDuration := time.Duration(tokensNeeded / b.refillRate * float64(time.Second))
		b.mu.RUnlock()

		time.Sleep(waitDuration)
	}
}

// WaitWithTimeout blocks until token available or timeout
func (b *TokenBucket) WaitWithTimeout(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if b.Allow() {
			return true
		}

		time.Sleep(10 * time.Millisecond)
	}

	return false
}

// TryWait attempts to get token with immediate timeout
func (b *TokenBucket) TryWait() bool {
	return b.WaitWithTimeout(0)
}

func (b *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()

	tokensToAdd := elapsed * b.refillRate
	b.tokens = min(b.tokens+tokensToAdd, b.maxTokens)

	b.lastRefill = now
}

// UpdateRate dynamically changes the fill rate
func (b *TokenBucket) UpdateRate(newRate float64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.refillRate = newRate
}

// UpdateMaxTokens dynamically changes bucket capacity
func (b *TokenBucket) UpdateMaxTokens(newMax float64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.maxTokens = newMax
	if b.tokens > newMax {
		b.tokens = newMax
	}
}

// Stats returns current bucket state
func (b *TokenBucket) Stats() map[string]interface{} {
	b.mu.RLock()
	defer b.mu.RUnlock()

	b.refill() // Update tokens before reporting

	return map[string]interface{}{
		"tokens":       b.tokens,
		"max_tokens":   b.maxTokens,
		"fill_rate":    b.refillRate,
		"utilization":  b.tokens / b.maxTokens,
		"last_refill":  b.lastRefill,
	}
}

// Reset restores bucket to initial state
func (b *TokenBucket) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.tokens = b.config.InitialTokens
	b.lastRefill = time.Now()
}

// Distributed rate limiter using Redis/NATS for coordination
type DistributedBucket struct {
	local  *TokenBucket
	nodeID string
	prefix string
	// Would integrate with Redis/NATS here for distributed state
}

// NewDistributedBucket creates a distributed rate limiter
func NewDistributedBucket(nodeID string, config *BucketConfig) *DistributedBucket {
	return &DistributedBucket{
		local:  NewTokenBucket(config),
		nodeID: nodeID,
		prefix: "ratelimit:",
	}
}

// MultiBucket manages rate limits across multiple endpoints
type MultiBucket struct {
	buckets map[string]*TokenBucket
	mu      sync.RWMutex
	defaultConfig *BucketConfig
}

// NewMultiBucket creates a multi-endpoint rate limiter
func NewMultiBucket(defaultConfig *BucketConfig) *MultiBucket {
	return &MultiBucket{
		buckets:       make(map[string]*TokenBucket),
		defaultConfig: defaultConfig,
	}
}

// GetBucket retrieves or creates bucket for endpoint
func (m *MultiBucket) GetBucket(endpoint string) *TokenBucket {
	m.mu.RLock()
	bucket, exists := m.buckets[endpoint]
	m.mu.RUnlock()

	if exists {
		return bucket
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if bucket, exists = m.buckets[endpoint]; exists {
		return bucket
	}

	// Create new bucket for this endpoint
	bucket = NewTokenBucket(m.defaultConfig)
	m.buckets[endpoint] = bucket

	return bucket
}

// Allow checks rate limit for specific endpoint
func (m *MultiBucket) Allow(endpoint string) bool {
	bucket := m.GetBucket(endpoint)
	return bucket.Allow()
}

// RemoveBucket removes rate limit for endpoint
func (m *MultiBucket) RemoveBucket(endpoint string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.buckets, endpoint)
}

// ListBuckets returns all active endpoints
func (m *MultiBucket) ListBuckets() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	endpoints := make([]string, 0, len(m.buckets))
	for endpoint := range m.buckets {
		endpoints = append(endpoints, endpoint)
	}
	return endpoints
}

// AdaptiveBucket automatically adjusts rate based on error rates
type AdaptiveBucket struct {
	*TokenBucket
	errorRate     float64
	targetErrorRate float64
	minRate       float64
	maxRate       float64
	mu            sync.RWMutex
}

// NewAdaptiveBucket creates a self-adjusting rate limiter
func NewAdaptiveBucket(config *BucketConfig, targetErrorRate float64) *AdaptiveBucket {
	return &AdaptiveBucket{
		TokenBucket:     NewTokenBucket(config),
		targetErrorRate: targetErrorRate,
		minRate:         config.FillRate * 0.1, // Never go below 10% of initial
		maxRate:         config.FillRate * 2.0, // Never exceed 200% of initial
		errorRate:       0,
	}
}

// RecordError records an error and adjusts rate accordingly
func (a *AdaptiveBucket) RecordError() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.errorRate += 0.01 // Increment error rate

	if a.errorRate > a.targetErrorRate {
		// Too many errors, reduce rate
		newRate := a.refillRate * 0.9 // Reduce by 10%
		if newRate < a.minRate {
			newRate = a.minRate
		}
		a.refillRate = newRate
	}
}

// RecordSuccess records a successful request
func (a *AdaptiveBucket) RecordSuccess() {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Decay error rate on success
	a.errorRate *= 0.999

	if a.errorRate < a.targetErrorRate*0.5 && a.refillRate < a.maxRate {
		// Error rate low, can increase throughput
		newRate := a.refillRate * 1.05 // Increase by 5%
		if newRate > a.maxRate {
			newRate = a.maxRate
		}
		a.refillRate = newRate
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

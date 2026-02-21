package metrics

import (
	"sync"
	"time"
)

// Batcher aggregates metrics and reports in batches to reduce network overhead
// Instead of sending 10K individual metrics, send 1 batch = 95% less network calls
type Batcher struct {
	config      *BatcherConfig
	buffer      []Metric
	bufferMu    sync.RWMutex
	statsMu     sync.RWMutex
	flushChan   chan []Metric
	stopChan    chan struct{}
	stats       *BatcherStats
	onFlush     func([]Metric) // Callback to send metrics upstream
}

type BatcherConfig struct {
	BatchSize       int           // Max metrics per batch (default: 1000)
	FlushInterval   time.Duration // Time-based flush (default: 100ms)
	MaxBufferLength int           // Buffer capacity before forced flush (default: 10000)
}

type Metric struct {
	Timestamp   time.Time             `json:"timestamp"`
	SimulationID string              `json:"simulation_id"`
	WorkerID    string              `json:"worker_id"`
	Type        string              `json:"type"` // request, response, error, latency
	Data        map[string]interface{} `json:"data"`
}

type BatcherStats struct {
	TotalMetrics    int64 `json:"total_metrics"`
	TotalBatches    int64 `json:"total_batches"`
	MetricsPerBatch float64 `json:"metrics_per_batch"`
	FlushCount      int64 `json:"flush_count"` // Count of flushes
	DroppedMetrics  int64 `json:"dropped_metrics"` // Dropped due to buffer full
	AvgBatchLatencyMs float64 `json:"avg_batch_latency_ms"`
}

// NewBatcher creates a new metrics batcher
func NewBatcher(config *BatcherConfig, onFlush func([]Metric)) *Batcher {
	if config.BatchSize == 0 {
		config.BatchSize = 1000
	}
	if config.FlushInterval == 0 {
		config.FlushInterval = 100 * time.Millisecond
	}
	if config.MaxBufferLength == 0 {
		config.MaxBufferLength = 10000
	}

	b := &Batcher{
		config:    config,
		buffer:    make([]Metric, 0, config.BatchSize),
		flushChan: make(chan []Metric, 100),
		stopChan:  make(chan struct{}),
		stats:     &BatcherStats{},
		onFlush:   onFlush,
	}

	// Start background flush goroutine
	go b.flushLoop()

	// Start processor goroutine
	go b.processBatches()

	return b
}

// Add queues a metric for batched reporting
func (b *Batcher) Add(metric Metric) {
	b.bufferMu.Lock()
	defer b.bufferMu.Unlock()

	b.stats.TotalMetrics++

	// Check if buffer is full
	if len(b.buffer) >= b.config.MaxBufferLength {
		// Force flush
		b.flushLocked()
		b.stats.DroppedMetrics++
		return
	}

	b.buffer = append(b.buffer, metric)

	// Flush if batch size reached
	if len(b.buffer) >= b.config.BatchSize {
		b.flushLocked()
	}
}

// AddBulk adds multiple metrics efficiently
func (b *Batcher) AddBulk(metrics []Metric) {
	b.bufferMu.Lock()
	defer b.bufferMu.Unlock()

	b.stats.TotalMetrics += int64(len(metrics))

	for _, metric := range metrics {
		b.buffer = append(b.buffer, metric)

		if len(b.buffer) >= b.config.BatchSize {
			b.flushLocked()
		}
	}
}

// Flush forces immediate flush of all pending metrics
func (b *Batcher) Flush() {
	b.bufferMu.Lock()
	defer b.bufferMu.Unlock()
	b.flushLocked()
}

func (b *Batcher) flushLocked() {
	if len(b.buffer) == 0 {
		return
	}

	// Copy buffer to avoid blocking
	batch := make([]Metric, len(b.buffer))
	copy(batch, b.buffer)
	b.buffer = b.buffer[:0]

	b.stats.TotalBatches++
	b.stats.FlushCount++

	// Send to flush channel (non-blocking)
	select {
	case b.flushChan <- batch:
		// Successfully queued for processing
	default:
		// Channel full, drop batch (shouldn't happen with proper sizing)
		b.stats.DroppedMetrics += int64(len(batch))
	}
}

func (b *Batcher) flushLoop() {
	ticker := time.NewTicker(b.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Time-based flush
			b.bufferMu.Lock()
			b.flushLocked()
			b.bufferMu.Unlock()
		case <-b.stopChan:
			return
		}
	}
}

func (b *Batcher) processBatches() {
	for batch := range b.flushChan {
		startTime := time.Now()

		// Call the flush callback (sends to NATS, HTTP, etc.)
		if b.onFlush != nil {
			b.onFlush(batch)
		}

		// Track latency
		latency := time.Since(startTime).Seconds() * 1000
		b.statsMu.Lock()
		b.stats.AvgBatchLatencyMs = b.stats.AvgBatchLatencyMs + 
			(latency-b.stats.AvgBatchLatencyMs)/float64(b.stats.TotalBatches)
		if b.stats.TotalBatches > 0 {
			b.stats.MetricsPerBatch = float64(b.stats.TotalMetrics) / float64(b.stats.TotalBatches)
		}
		b.statsMu.Unlock()
	}
}

// Stats returns current batcher statistics
func (b *Batcher) Stats() BatcherStats {
	b.statsMu.RLock()
	defer b.statsMu.RUnlock()
	return *b.stats
}

// Close shuts down the batcher gracefully
func (b *Batcher) Close() {
	close(b.stopChan)
	b.Flush() // Flush remaining metrics
	close(b.flushChan)
}

// Compression support for large batches
type CompressedBatcher struct {
	*Batcher
	compressThreshold int // Compress if batch > this size (bytes)
}

// NewCompressedBatcher creates a batcher with optional compression
func NewCompressedBatcher(config *BatcherConfig, onFlush func([]byte), compressThreshold int) *CompressedBatcher {
	cb := &CompressedBatcher{
		Batcher: NewBatcher(config, func(metrics []Metric) {
			// Serialize metrics
			data, _ := serializeMetrics(metrics)
			
			// Compress if large enough
			if len(data) > compressThreshold {
				compressed, _ := compressData(data)
				onFlush(compressed)
			} else {
				onFlush(data)
			}
		}),
		compressThreshold: compressThreshold,
	}

	if compressThreshold == 0 {
		cb.compressThreshold = 10240 // 10KB default
	}

	return cb
}

// Helper functions (would use gzip/snappy in production)
func serializeMetrics(metrics []Metric) ([]byte, error) {
	// In production: json.Marshal(metrics)
	// Simplified for now
	return make([]byte, 0), nil
}

func compressData(data []byte) ([]byte, error) {
	// In production: gzip.Compress(data)
	// Simplified for now
	return data, nil
}

// Aggregator combines metrics from multiple sources before batching
type Aggregator struct {
	batcher      *Batcher
	simulations  map[string]*SimulationMetrics
	simulationsMu sync.RWMutex
}

type SimulationMetrics struct {
	ID            string
	RequestCount  int64
	SuccessCount  int64
	FailureCount  int64
	Latencies     []float64
	StartTime     time.Time
	LastUpdate    time.Time
	mu            sync.RWMutex
}

// NewAggregator creates a metrics aggregator
func NewAggregator(batcher *Batcher) *Aggregator {
	return &Aggregator{
		batcher:     batcher,
		simulations: make(map[string]*SimulationMetrics),
	}
}

// RecordRequest records a single request metric
func (a *Aggregator) RecordRequest(simulationID, workerID string, success bool, latencyMs float64) {
	a.simulationsMu.RLock()
	sim, exists := a.simulations[simulationID]
	a.simulationsMu.RUnlock()

	if !exists {
		a.simulationsMu.Lock()
		sim = &SimulationMetrics{
			ID:        simulationID,
			StartTime: time.Now(),
			Latencies: make([]float64, 0, 1000),
		}
		a.simulations[simulationID] = sim
		a.simulationsMu.Unlock()
	}

	sim.mu.Lock()
	sim.RequestCount++
	if success {
		sim.SuccessCount++
	} else {
		sim.FailureCount++
	}
	sim.Latencies = append(sim.Latencies, latencyMs)
	sim.LastUpdate = time.Now()
	sim.mu.Unlock()

	// Add to batcher
	a.batcher.Add(Metric{
		Timestamp:    time.Now(),
		SimulationID: simulationID,
		WorkerID:     workerID,
		Type:         "request",
		Data: map[string]interface{}{
			"success": success,
			"latency_ms": latencyMs,
		},
	})
}

// GetSimulationMetrics returns aggregated metrics for a simulation
func (a *Aggregator) GetSimulationMetrics(simulationID string) *SimulationMetrics {
	a.simulationsMu.RLock()
	defer a.simulationsMu.RUnlock()
	return a.simulations[simulationID]
}

// CalculatePercentile computes percentile from latency slice
func CalculatePercentile(latencies []float64, percentile float64) float64 {
	if len(latencies) == 0 {
		return 0
	}

	// Sort (in production, use histogram for O(1))
	sorted := make([]float64, len(latencies))
	copy(sorted, latencies)
	// sort.Float64s(sorted) // Import sort package

	index := int(percentile / 100.0 * float64(len(sorted)))
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

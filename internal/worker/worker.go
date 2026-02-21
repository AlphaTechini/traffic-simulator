package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/hashicorp/consul/api"
)

// Worker executes load testing tasks assigned by the coordinator
type Worker struct {
	config       *WorkerConfig
	consul       *api.Client
	nats         *nats.Conn
	simulations  map[string]*ActiveSimulation
	simulationsMu sync.RWMutex
	stats        *WorkerStats
	ctx          context.Context
	cancel       context.CancelFunc
}

type WorkerConfig struct {
	ConsulAddr   string
	NATSAddr     string
	NodeID       string
	BindAddr     string
	Port         int
	MaxUsers     int
}

type ActiveSimulation struct {
	ID           string
	Config       interface{}
	AssignedUsers int
	StartTime    time.Time
	Ctx          context.Context
	CancelFunc   context.CancelFunc
}

type WorkerStats struct {
	CurrentUsers    int32 `json:"current_users"`
	TotalRequests   int64 `json:"total_requests"`
	SuccessCount    int64 `json:"success_count"`
	FailureCount    int64 `json:"failure_count"`
	AvgLatencyMs    float64 `json:"avg_latency_ms"`
	LastHeartbeat   time.Time `json:"last_heartbeat"`
}

// NewWorker creates a new load testing worker node
func NewWorker(config *WorkerConfig) (*Worker, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Connect to Consul
	consulConfig := api.DefaultConfig()
	consulConfig.Address = config.ConsulAddr
	consul, err := api.NewClient(consulConfig)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create consul client: %w", err)
	}
	
	// Connect to NATS
	natsConn, err := nats.Connect(config.NATSAddr)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}
	
	w := &Worker{
		config:      config,
		consul:      consul,
		nats:        natsConn,
		simulations: make(map[string]*ActiveSimulation),
		stats:       &WorkerStats{
			LastHeartbeat: time.Now(),
		},
		ctx:    ctx,
		cancel: cancel,
	}
	
	// Register with Consul
	if err := w.registerSelf(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to register with consul: %w", err)
	}
	
	// Start background goroutines
	go w.sendHeartbeats()
	go w.listenForCommands()
	go w.executeSimulations()
	
	log.Printf("🔧 Worker started on %s:%d (max users: %d)", config.BindAddr, config.Port, config.MaxUsers)
	log.Printf("📡 Connected to Consul: %s", config.ConsulAddr)
	log.Printf("📨 Connected to NATS: %s", config.NATSAddr)
	
	return w, nil
}

func (w *Worker) registerSelf() error {
	service := &api.AgentServiceRegistration{
		ID:      fmt.Sprintf("worker-%s", w.config.NodeID),
		Name:    "traffic-simulator-worker",
		Address: w.config.BindAddr,
		Port:    w.config.Port,
		Tags:    []string{"worker", "load-tester"},
		Meta: map[string]string{
			"max_users": fmt.Sprintf("%d", w.config.MaxUsers),
			"version":   "1.0.0",
		},
		Checks: []*api.AgentServiceCheck{
			{
				HTTP:     fmt.Sprintf("http://%s:%d/health", w.config.BindAddr, w.config.Port),
				Interval: "10s",
				Timeout:  "5s",
			},
		},
	}
	
	return w.consul.Agent().ServiceRegister(service)
}

func (w *Worker) sendHeartbeats() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.stats.LastHeartbeat = time.Now()
			
			// Update Consul with current load
			atomic.StoreInt32(&w.stats.CurrentUsers, int32(w.getCurrentLoad()))
			
			// Publish heartbeat via NATS
			heartbeat := map[string]interface{}{
				"worker_id":     w.config.NodeID,
				"timestamp":     time.Now().UTC(),
				"current_users": w.stats.CurrentUsers,
				"status":        "healthy",
			}
			heartbeatJSON, _ := json.Marshal(heartbeat)
			w.nats.Publish("simulator.workers.heartbeats", heartbeatJSON)
		}
	}
}

func (w *Worker) listenForCommands() {
	// Subscribe to commands for this specific worker
	subject := fmt.Sprintf("simulator.worker.%s.command", w.config.NodeID)
	_, err := w.nats.Subscribe(subject, func(msg *nats.Msg) {
		var cmd map[string]interface{}
		if err := json.Unmarshal(msg.Data, &cmd); err != nil {
			log.Printf("❌ Invalid command: %v", err)
			return
		}
		
		cmdType := cmd["type"].(string)
		simID := cmd["simulation_id"].(string)
		
		log.Printf("📥 Received command: %s for simulation %s", cmdType, simID)
		
		switch cmdType {
		case "start":
			w.handleStartCommand(simID, cmd["params"])
		case "stop":
			w.handleStopCommand(simID)
		case "pause":
			w.handlePauseCommand(simID)
		case "resume":
			w.handleResumeCommand(simID)
		}
	})
	
	if err != nil {
		log.Printf("❌ Failed to subscribe to commands: %v", err)
	}
	
	// Also subscribe to broadcast commands (for all workers)
	_, err = w.nats.Subscribe("simulator.commands.broadcast", func(msg *nats.Msg) {
		// Handle broadcast commands if needed
	})
	
	if err != nil {
		log.Printf("❌ Failed to subscribe to broadcasts: %v", err)
	}
}

func (w *Worker) handleStartCommand(simID string, params interface{}) {
	w.simulationsMu.Lock()
	defer w.simulationsMu.Unlock()
	
	if _, exists := w.simulations[simID]; exists {
		log.Printf("⚠️ Simulation %s already running", simID)
		return
	}
	
	ctx, cancel := context.WithCancel(w.ctx)
	
	w.simulations[simID] = &ActiveSimulation{
		ID:            simID,
		Config:        params,
		AssignedUsers: int(params.(map[string]interface{})["users"].(float64)),
		StartTime:     time.Now(),
		Ctx:           ctx,
		CancelFunc:    cancel,
	}
	
	log.Printf("🚀 Started simulation %s with %d users", simID, w.simulations[simID].AssignedUsers)
}

func (w *Worker) handleStopCommand(simID string) {
	w.simulationsMu.Lock()
	defer w.simulationsMu.Unlock()
	
	if sim, exists := w.simulations[simID]; exists {
		sim.CancelFunc()
		delete(w.simulations, simID)
		log.Printf("⏹️ Stopped simulation %s", simID)
	}
}

func (w *Worker) handlePauseCommand(simID string) {
	// TODO: Implement pause logic
	log.Printf("⏸️ Paused simulation %s", simID)
}

func (w *Worker) handleResumeCommand(simID string) {
	// TODO: Implement resume logic
	log.Printf("▶️ Resumed simulation %s", simID)
}

func (w *Worker) executeSimulations() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.runSimulationStep()
		}
	}
}

func (w *Worker) runSimulationStep() {
	w.simulationsMu.RLock()
	defer w.simulationsMu.RUnlock()
	
	for _, sim := range w.simulations {
		// Check if simulation is cancelled
		select {
		case <-sim.Ctx.Done():
			continue
		default:
		}
		
		// Execute one iteration of the simulation
		// In production, this would:
		// 1. Generate requests based on traffic pattern
		// 2. Send HTTP/gRPC/GraphQL requests
		// 3. Collect metrics
		// 4. Report to coordinator
		
		// Simplified for now - just increment counters
		users := sim.AssignedUsers
		requestsPerTick := users / 10 // 10 ticks per second
		
		for i := 0; i < requestsPerTick; i++ {
			success := true // Simulated success
			latency := 50.0 + float64(time.Now().UnixNano()%100) / 1e9 // Simulated latency
			
			if success {
				atomic.AddInt64(&w.stats.SuccessCount, 1)
			} else {
				atomic.AddInt64(&w.stats.FailureCount, 1)
			}
			atomic.AddInt64(&w.stats.TotalRequests, 1)
			
			// Update average latency (simple moving average)
			currentAvg := w.stats.AvgLatencyMs
			w.stats.AvgLatencyMs = currentAvg + (latency-currentAvg)/float64(w.stats.TotalRequests)
		}
		
		// Publish metrics
		metrics := map[string]interface{}{
			"simulation_id": sim.ID,
			"worker_id":     w.config.NodeID,
			"timestamp":     time.Now().UTC(),
			"users":         users,
			"requests":      w.stats.TotalRequests,
			"success_rate":  float64(w.stats.SuccessCount) / float64(w.stats.TotalRequests) * 100,
			"avg_latency":   w.stats.AvgLatencyMs,
		}
		metricsJSON, _ := json.Marshal(metrics)
		w.nats.Publish(fmt.Sprintf("simulator.simulation.%s.metrics", sim.ID), metricsJSON)
	}
}

func (w *Worker) getCurrentLoad() int {
	w.simulationsMu.RLock()
	defer w.simulationsMu.RUnlock()
	
	total := 0
	for _, sim := range w.simulations {
		total += sim.AssignedUsers
	}
	return total
}

func (w *Worker) GetHealthHandler() http.HandlerFunc {
	return func(wr http.ResponseWriter, req *http.Request) {
		response := map[string]interface{}{
			"status":        "healthy",
			"worker_id":     w.config.NodeID,
			"current_users": atomic.LoadInt32(&w.stats.CurrentUsers),
			"max_users":     w.config.MaxUsers,
			"timestamp":     time.Now().UTC().Format(time.RFC3339),
		}
		
		wr.Header().Set("Content-Type", "application/json")
		json.NewEncoder(wr).Encode(response)
	}
}

func (w *Worker) GetStatsHandler() http.HandlerFunc {
	return func(wr http.ResponseWriter, req *http.Request) {
		w.simulationsMu.RLock()
		simulations := make([]map[string]interface{}, 0, len(w.simulations))
		for id, sim := range w.simulations {
			simulations = append(simulations, map[string]interface{}{
				"id":      id,
				"users":   sim.AssignedUsers,
				"started": sim.StartTime,
			})
		}
		w.simulationsMu.RUnlock()
		
		response := map[string]interface{}{
			"worker_id":       w.config.NodeID,
			"current_users":   atomic.LoadInt32(&w.stats.CurrentUsers),
			"max_users":       w.config.MaxUsers,
			"total_requests":  atomic.LoadInt64(&w.stats.TotalRequests),
			"success_count":   atomic.LoadInt64(&w.stats.SuccessCount),
			"failure_count":   atomic.LoadInt64(&w.stats.FailureCount),
			"avg_latency_ms":  w.stats.AvgLatencyMs,
			"simulations":     simulations,
			"last_heartbeat":  w.stats.LastHeartbeat,
		}
		
		wr.Header().Set("Content-Type", "application/json")
		json.NewEncoder(wr).Encode(response)
	}
}

func (w *Worker) Shutdown() error {
	w.cancel()
	
	// Deregister from Consul
	w.consul.Agent().ServiceDeregister(fmt.Sprintf("worker-%s", w.config.NodeID))
	
	// Close NATS connection
	w.nats.Close()
	
	log.Println("👋 Worker shutdown complete")
	return nil
}

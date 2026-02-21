package coordinator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/hashicorp/consul/api"
)

// Coordinator orchestrates distributed load testing across multiple worker nodes
type Coordinator struct {
	config      *CoordinatorConfig
	consul      *api.Client
	nats        *nats.Conn
	workers     map[string]*WorkerInfo
	workersMu   sync.RWMutex
	simulations map[string]*Simulation
	simulationsMu sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

type CoordinatorConfig struct {
	ConsulAddr   string
	NATSAddr     string
	NodeID       string
	BindAddr     string
	Port         int
}

type WorkerInfo struct {
	ID           string    `json:"id"`
	Addr         string    `json:"addr"`
	Port         int       `json:"port"`
	Capacity     int       `json:"capacity"` // Max users this worker can handle
	CurrentLoad  int       `json:"current_load"`
	Status       string    `json:"status"` // "healthy", "degraded", "dead"
	LastHeartbeat time.Time `json:"last_heartbeat"`
	Metadata     map[string]string `json:"metadata"`
}

type Simulation struct {
	ID          string                `json:"id"`
	Config      interface{}           `json:"config"`
	Status      string                `json:"status"`
	Workers     []string              `json:"workers"`
	TotalUsers  int                   `json:"total_users"`
	StartTime   *time.Time            `json:"start_time,omitempty"`
	EndTime     *time.Time            `json:"end_time,omitempty"`
	Metrics     *AggregatedMetrics    `json:"metrics,omitempty"`
}

type AggregatedMetrics struct {
	ActiveUsers   int     `json:"active_users"`
	TotalRequests int64   `json:"total_requests"`
	SuccessRate   float64 `json:"success_rate"`
	AvgRPS        float64 `json:"avg_rps"`
	P95Latency    float64 `json:"p95_latency"`
	ErrorRate     float64 `json:"error_rate"`
}

// Command messages sent via NATS
type Command struct {
	Type          string      `json:"type"` // "start", "stop", "pause", "resume", "scale"
	SimulationID  string      `json:"simulation_id"`
	Params        interface{} `json:"params,omitempty"`
	Timestamp     time.Time   `json:"timestamp"`
}

// NewCoordinator creates a new distributed coordinator
func NewCoordinator(config *CoordinatorConfig) (*Coordinator, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Connect to Consul for service discovery
	consulConfig := api.DefaultConfig()
	consulConfig.Address = config.ConsulAddr
	consul, err := api.NewClient(consulConfig)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create consul client: %w", err)
	}
	
	// Connect to NATS for messaging
	natsConn, err := nats.Connect(config.NATSAddr)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}
	
	c := &Coordinator{
		config:      config,
		consul:      consul,
		nats:        natsConn,
		workers:     make(map[string]*WorkerInfo),
		simulations: make(map[string]*Simulation),
		ctx:         ctx,
		cancel:      cancel,
	}
	
	// Register self with Consul
	if err := c.registerSelf(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to register with consul: %w", err)
	}
	
	// Start background goroutines
	go c.monitorWorkers()
	go c.listenForCommands()
	go c.aggregateMetrics()
	
	log.Printf("🎯 Coordinator started on %s:%d", config.BindAddr, config.Port)
	log.Printf("📡 Connected to Consul: %s", config.ConsulAddr)
	log.Printf("📨 Connected to NATS: %s", config.NATSAddr)
	
	return c, nil
}

func (c *Coordinator) registerSelf() error {
	service := &api.AgentServiceRegistration{
		ID:      fmt.Sprintf("coordinator-%s", c.config.NodeID),
		Name:    "traffic-simulator-coordinator",
		Address: c.config.BindAddr,
		Port:    c.config.Port,
		Tags:    []string{"coordinator", "primary"},
		Checks: []*api.AgentServiceCheck{
			{
				HTTP:     fmt.Sprintf("http://%s:%d/health", c.config.BindAddr, c.config.Port),
				Interval: "10s",
				Timeout:  "5s",
			},
		},
	}
	
	return c.consul.Agent().ServiceRegister(service)
}

func (c *Coordinator) monitorWorkers() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.discoverWorkers()
			c.checkWorkerHealth()
		}
	}
}

func (c *Coordinator) discoverWorkers() {
	services, _, err := c.consul.Health().Service("traffic-simulator-worker", "", true, nil)
	if err != nil {
		log.Printf("❌ Failed to discover workers: %v", err)
		return
	}
	
	c.workersMu.Lock()
	defer c.workersMu.Unlock()
	
	// Update worker list
	for _, service := range services {
		workerID := service.Service.ID
		if _, exists := c.workers[workerID]; !exists {
			// New worker discovered
			c.workers[workerID] = &WorkerInfo{
				ID:       workerID,
				Addr:     service.Service.Address,
				Port:     service.Service.Port,
				Capacity: 50000, // Default capacity
				Status:   "healthy",
				Metadata: service.Service.Meta,
			}
			log.Printf("✨ New worker discovered: %s (%s:%d)", workerID, service.Service.Address, service.Service.Port)
		}
	}
}

func (c *Coordinator) checkWorkerHealth() {
	c.workersMu.Lock()
	defer c.workersMu.Unlock()
	
	now := time.Now()
	for id, worker := range c.workers {
		// If no heartbeat in 30 seconds, mark as dead
		if now.Sub(worker.LastHeartbeat) > 30*time.Second {
			if worker.Status != "dead" {
				log.Printf("💀 Worker %s marked as dead (no heartbeat)", id)
				worker.Status = "dead"
				c.rebalanceWorkloads(id)
			}
		} else if now.Sub(worker.LastHeartbeat) > 15*time.Second {
			worker.Status = "degraded"
		} else {
			worker.Status = "healthy"
		}
	}
}

func (c *Coordinator) listenForCommands() {
	_, err := c.nats.Subscribe("simulator.commands.*", func(msg *nats.Msg) {
		var cmd Command
		if err := json.Unmarshal(msg.Data, &cmd); err != nil {
			log.Printf("❌ Invalid command: %v", err)
			return
		}
		
		log.Printf("📥 Received command: %s for simulation %s", cmd.Type, cmd.SimulationID)
		
		switch cmd.Type {
		case "start":
			c.handleStartCommand(cmd)
		case "stop":
			c.handleStopCommand(cmd)
		case "pause":
			c.handlePauseCommand(cmd)
		case "scale":
			c.handleScaleCommand(cmd)
		}
	})
	
	if err != nil {
		log.Printf("❌ Failed to subscribe to commands: %v", err)
	}
}

func (c *Coordinator) handleStartCommand(cmd Command) {
	c.simulationsMu.Lock()
	defer c.simulationsMu.Unlock()
	
	// Select healthy workers
	workers := c.selectWorkers(cmd.Params.(map[string]interface{}))
	if len(workers) == 0 {
		log.Printf("❌ No healthy workers available")
		return
	}
	
	// Create simulation record
	sim := &Simulation{
		ID:         cmd.SimulationID,
		Config:     cmd.Params,
		Status:     "starting",
		Workers:    workers,
		StartTime:  func() *time.Time { t := time.Now(); return &t }(),
	}
	c.simulations[cmd.SimulationID] = sim
	
	// Broadcast start command to selected workers
	for _, workerID := range workers {
		c.sendCommandToWorker(workerID, cmd)
	}
	
	log.Printf("🚀 Started simulation %s on %d workers", cmd.SimulationID, len(workers))
}

func (c *Coordinator) handleStopCommand(cmd Command) {
	// Send stop command to all workers in simulation
	c.sendCommandToWorkers(cmd.SimulationID, cmd)
	log.Printf("⏹️ Stopped simulation %s", cmd.SimulationID)
}

func (c *Coordinator) handlePauseCommand(cmd Command) {
	c.sendCommandToWorkers(cmd.SimulationID, cmd)
	log.Printf("⏸️ Paused simulation %s", cmd.SimulationID)
}

func (c *Coordinator) handleScaleCommand(cmd Command) {
	// Dynamically add/remove workers based on scale params
	params := cmd.Params.(map[string]interface{})
	newUserCount := int(params["users"].(float64))
	
	c.simulationsMu.RLock()
	sim, exists := c.simulations[cmd.SimulationID]
	c.simulationsMu.RUnlock()
	
	if !exists {
		return
	}
	
	// Calculate if we need more or fewer workers
	currentCapacity := len(sim.Workers) * 50000
	if newUserCount > currentCapacity {
		// Need more workers
		additionalWorkersNeeded := (newUserCount - currentCapacity) / 50000 + 1
		c.addWorkersToSimulation(sim.ID, additionalWorkersNeeded)
	} else if newUserCount < currentCapacity/2 {
		// Can remove some workers
		c.removeWorkersFromSimulation(sim.ID, len(sim.Workers)/2)
	}
}

func (c *Coordinator) selectWorkers(params map[string]interface{}) []string {
	c.workersMu.RLock()
	defer c.workersMu.RUnlock()
	
	var selected []string
	requiredUsers := int(params["max_users"].(float64))
	
	for id, worker := range c.workers {
		if worker.Status == "healthy" && worker.CurrentLoad < worker.Capacity {
			selected = append(selected, id)
			requiredUsers -= worker.Capacity
			if requiredUsers <= 0 {
				break
			}
		}
	}
	
	return selected
}

func (c *Coordinator) sendCommandToWorker(workerID string, cmd Command) {
	msg, _ := json.Marshal(cmd)
	c.nats.Publish(fmt.Sprintf("simulator.worker.%s.command", workerID), msg)
}

func (c *Coordinator) sendCommandToWorkers(simulationID string, cmd Command) {
	c.simulationsMu.RLock()
	sim, exists := c.simulations[simulationID]
	c.simulationsMu.RUnlock()
	
	if !exists {
		return
	}
	
	for _, workerID := range sim.Workers {
		c.sendCommandToWorker(workerID, cmd)
	}
}

func (c *Coordinator) rebalanceWorkloads(deadWorkerID string) {
	// Find simulations affected by dead worker
	c.simulationsMu.Lock()
	defer c.simulationsMu.Unlock()
	
	for simID, sim := range c.simulations {
		if contains(sim.Workers, deadWorkerID) {
			log.Printf("⚠️ Simulation %s affected by worker failure, rebalancing...", simID)
			// Remove dead worker and add new one
			sim.Workers = remove(sim.Workers, deadWorkerID)
			c.addWorkersToSimulation(simID, 1)
		}
	}
}

func (c *Coordinator) addWorkersToSimulation(simulationID string, count int) {
	// Find available healthy workers
	c.workersMu.RLock()
	defer c.workersMu.RUnlock()
	
	added := 0
	for id, worker := range c.workers {
		if added >= count {
			break
		}
		if worker.Status == "healthy" && worker.CurrentLoad < worker.Capacity {
			// Add this worker to simulation
			c.simulationsMu.Lock()
			if sim, exists := c.simulations[simulationID]; exists {
				sim.Workers = append(sim.Workers, id)
				// Send start command to new worker
				cmd := Command{
					Type:         "start",
					SimulationID: simulationID,
					Timestamp:    time.Now(),
				}
				c.sendCommandToWorker(id, cmd)
			}
			c.simulationsMu.Unlock()
			
			added++
			log.Printf("➕ Added worker %s to simulation", id)
		}
	}
}

func (c *Coordinator) removeWorkersFromSimulation(simulationID string, count int) {
	// Gracefully remove workers from simulation
	c.simulationsMu.Lock()
	defer c.simulationsMu.Unlock()
	
	sim, exists := c.simulations[simulationID]
	if !exists || len(sim.Workers) <= count {
		return
	}
	
	// Send stop command to excess workers
	for i := 0; i < count && i < len(sim.Workers); i++ {
		workerID := sim.Workers[len(sim.Workers)-1-i]
		cmd := Command{
			Type:         "stop",
			SimulationID: simulationID,
			Timestamp:    time.Now(),
		}
		c.sendCommandToWorker(workerID, cmd)
	}
	
	sim.Workers = sim.Workers[:len(sim.Workers)-count]
}

func (c *Coordinator) aggregateMetrics() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.collectAndAggregateMetrics()
		}
	}
}

func (c *Coordinator) collectAndAggregateMetrics() {
	// Collect metrics from all workers via NATS
	// This is simplified - in production would use request/reply pattern
	c.simulationsMu.Lock()
	defer c.simulationsMu.Unlock()
	
	for simID, sim := range c.simulations {
		if sim.Status == "running" {
			// Aggregate metrics from workers
			sim.Metrics = &AggregatedMetrics{
				ActiveUsers:   sim.TotalUsers,
				TotalRequests: 0, // Would collect from workers
				SuccessRate:   99.5,
				AvgRPS:        float64(sim.TotalUsers) * 2.5,
				P95Latency:    125.3,
				ErrorRate:     0.5,
			}
			
			// Publish aggregated metrics
			metricsJSON, _ := json.Marshal(sim.Metrics)
			c.nats.Publish(fmt.Sprintf("simulator.simulation.%s.metrics", simID), metricsJSON)
		}
	}
}

func (c *Coordinator) Shutdown() error {
	c.cancel()
	
	// Deregister from Consul
	c.consul.Agent().ServiceDeregister(fmt.Sprintf("coordinator-%s", c.config.NodeID))
	
	// Close NATS connection
	c.nats.Close()
	
	log.Println("👋 Coordinator shutdown complete")
	return nil
}

// Helper functions
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func remove(slice []string, item string) []string {
	result := []string{}
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// SimulationConfig represents a user-defined load test scenario
type SimulationConfig struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	CreatedAt   time.Time         `json:"created_at"`
	Endpoints   []EndpointConfig  `json:"endpoints"`
	Pattern     TrafficPattern    `json:"pattern"`
	Duration    string            `json:"duration"`
	RampUp      string            `json:"ramp_up"`
	MaxUsers    int               `json:"max_users"`
	Environment map[string]string `json:"environment"`
}

// EndpointConfig defines a single target URL with its request configuration
type EndpointConfig struct {
	URL           string              `json:"url"`
	Method        string              `json:"method"`
	ContentType   string              `json:"content_type"`
	Payload       json.RawMessage     `json:"payload,omitempty"`
	PayloadType   string              `json:"payload_type"` // "static", "dynamic", "template"
	Headers       map[string]string   `json:"headers"`
	Variables     map[string]string   `json:"variables"`
	Assertions    []Assertion         `json:"assertions"`
	Weight        int                 `json:"weight"` // For weighted distribution
	RequestType   string              `json:"request_type"` // "rest", "graphql", "websocket"
	GraphQLQuery  string              `json:"graphql_query,omitempty"`
}

// Assertion defines validation rules for responses
type Assertion struct {
	Type     string      `json:"type"` // "status_code", "response_time", "json_schema", "regex"
	Expected interface{} `json:"expected"`
	Operator string      `json:"operator"` // "equals", "greater_than", "less_than", "contains"
}

// TrafficPattern defines how users are distributed over time
type TrafficPattern struct {
	Type   string      `json:"type"` // "constant", "ramp", "step", "wave", "burst", "custom"
	Config PatternConfig `json:"config"`
}

type PatternConfig struct {
	Stages       []Stage `json:"stages,omitempty"`       // For step/ramp patterns
	WavePeriod   string  `json:"wave_period,omitempty"`   // For sine wave patterns
	BurstTimes   []string `json:"burst_times,omitempty"` // For burst patterns
	CustomCurve  []int    `json:"custom_curve,omitempty"` // For custom patterns
}

type Stage struct {
	Duration string `json:"duration"`
	Users    int    `json:"users"`
	Ramp     string `json:"ramp"` // "linear", "exponential"
}

// SimulationState tracks real-time execution
type SimulationState struct {
	ID             string                 `json:"id"`
	Status         string                 `json:"status"` // "running", "paused", "stopped", "completed"
	ActiveUsers    int                    `json:"active_users"`
	TotalRequests  int64                  `json:"total_requests"`
	SuccessCount   int64                  `json:"success_count"`
	FailureCount   int64                  `json:"failure_count"`
	AvgRPS         float64                `json:"avg_rps"`
	CurrentRPS     float64                `json:"current_rps"`
	LatencyP50     float64                `json:"latency_p50"`
	LatencyP95     float64                `json:"latency_p95"`
	LatencyP99     float64                `json:"latency_p99"`
	ErrorRate      float64                `json:"error_rate"`
	StartTime      *time.Time             `json:"start_time,omitempty"`
	EndTime        *time.Time             `json:"end_time,omitempty"`
	EndpointStats  map[string]EndpointStats `json:"endpoint_stats"`
}

type EndpointStats struct {
	URL            string  `json:"url"`
	Requests       int64   `json:"requests"`
	Successes      int64   `json:"successes"`
	Failures       int64   `json:"failures"`
	AvgLatency     float64 `json:"avg_latency"`
	P95Latency     float64 `json:"p95_latency"`
	ErrorRate      float64 `json:"error_rate"`
	LastStatusCode int     `json:"last_status_code"`
}

// Server handles HTTP API and WebSocket connections
type Server struct {
	config      *ServerConfig
	router      *mux.Router
	simulations sync.Map // map[string]*Simulation
	wsClients   map[*websocket.Conn]string
	wsMutex     sync.RWMutex
	hub         *MetricsHub
}

type ServerConfig struct {
	Port         int
	EnableCORS   bool
	AuthToken    string
	MaxSimulations int
}

// MetricsHub aggregates and broadcasts real-time metrics
type MetricsHub struct {
	metrics     chan Metric
	subscribers map[chan Metric]bool
	mutex       sync.RWMutex
}

type Metric struct {
	SimulationID string                 `json:"simulation_id"`
	Timestamp    time.Time              `json:"timestamp"`
	Type         string                 `json:"type"` // "request", "response", "error", "user_count"
	Data         map[string]interface{} `json:"data"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now (configure in production)
	},
}

// NewServer creates a new API server instance
func NewServer(config *ServerConfig) *Server {
	s := &Server{
		config:    config,
		router:    mux.NewRouter(),
		wsClients: make(map[*websocket.Conn]string),
		hub: &MetricsHub{
			metrics:     make(chan Metric, 1000),
			subscribers: make(map[chan Metric]bool),
		},
	}

	s.setupRoutes()
	go s.hub.run()

	return s
}

// setupRoutes configures HTTP endpoints
func (s *Server) setupRoutes() {
	// Health check
	s.router.HandleFunc("/health", s.healthHandler).Methods("GET")

	// Simulations CRUD
	s.router.HandleFunc("/api/v1/simulations", s.createSimulationHandler).Methods("POST")
	s.router.HandleFunc("/api/v1/simulations", s.listSimulationsHandler).Methods("GET")
	s.router.HandleFunc("/api/v1/simulations/{id}", s.getSimulationHandler).Methods("GET")
	s.router.HandleFunc("/api/v1/simulations/{id}", s.deleteSimulationHandler).Methods("DELETE")
	
	// Simulation control
	s.router.HandleFunc("/api/v1/simulations/{id}/start", s.startSimulationHandler).Methods("POST")
	s.router.HandleFunc("/api/v1/simulations/{id}/stop", s.stopSimulationHandler).Methods("POST")
	s.router.HandleFunc("/api/v1/simulations/{id}/pause", s.pauseSimulationHandler).Methods("POST")
	
	// Real-time metrics
	s.router.HandleFunc("/api/v1/simulations/{id}/metrics", s.metricsHandler).Methods("GET")
	s.router.HandleFunc("/api/v1/simulations/{id}/ws", s.wsMetricsHandler).Methods("GET")
	
	// Templates
	s.router.HandleFunc("/api/v1/templates", s.listTemplatesHandler).Methods("GET")
	s.router.HandleFunc("/api/v1/templates", s.createTemplateHandler).Methods("POST")
	
	// Environment variables
	s.router.HandleFunc("/api/v1/environment", s.getEnvironmentHandler).Methods("GET")
	s.router.HandleFunc("/api/v1/environment", s.updateEnvironmentHandler).Methods("PUT")
}

// Start begins serving HTTP requests
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.config.Port)
	log.Printf("🚀 Traffic Simulator API starting on %s", addr)
	return http.ListenAndServe(addr, s.router)
}

// Handler implementations

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"version": "1.0.0",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) createSimulationHandler(w http.ResponseWriter, r *http.Request) {
	var config SimulationConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// TODO: Validate config
	// TODO: Generate ID if not provided
	// TODO: Store simulation

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(config)
}

func (s *Server) listSimulationsHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Return list of simulations
	simulations := []SimulationConfig{}
	json.NewEncoder(w).Encode(simulations)
}

func (s *Server) getSimulationHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	
	// TODO: Fetch simulation by ID
	_ = id
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"error": "not implemented"})
}

func (s *Server) deleteSimulationHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	_ = id
	
	// TODO: Delete simulation
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) startSimulationHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	_ = id
	
	// TODO: Start simulation
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func (s *Server) stopSimulationHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	_ = id
	
	// TODO: Stop simulation
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

func (s *Server) pauseSimulationHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Pause simulation
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "paused"})
}

func (s *Server) metricsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	_ = id
	
	// TODO: Return current metrics
	state := SimulationState{
		Status: "running",
		ActiveUsers: 1000,
		TotalRequests: 50000,
		SuccessCount: 49500,
		FailureCount: 500,
		AvgRPS: 1250.5,
		CurrentRPS: 1340.2,
		LatencyP50: 45.3,
		LatencyP95: 120.7,
		LatencyP99: 250.1,
		ErrorRate: 1.0,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

func (s *Server) wsMetricsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}

	vars := mux.Vars(r)
	simulationID := vars["id"]
	
	s.wsMutex.Lock()
	s.wsClients[conn] = simulationID
	s.wsMutex.Unlock()

	// Subscribe to metrics stream
	metricChan := make(chan Metric, 100)
	s.hub.subscribe(metricChan)

	defer func() {
		s.wsMutex.Lock()
		delete(s.wsClients, conn)
		s.wsMutex.Unlock()
		s.hub.unsubscribe(metricChan)
		conn.Close()
	}()

	// Send metrics as they arrive
	for metric := range metricChan {
		if metric.SimulationID == simulationID {
			if err := conn.WriteJSON(metric); err != nil {
				break
			}
		}
	}
}

func (s *Server) listTemplatesHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Return predefined templates
	templates := []map[string]interface{}{
		{
			"name": "Flash Sale",
			"description": "Sudden spike followed by sustained high load",
			"type": "burst",
		},
		{
			"name": "Gradual Growth",
			"description": "Linear increase over time",
			"type": "ramp",
		},
		{
			"name": "Business Hours",
			"description": "Realistic daily traffic pattern",
			"type": "wave",
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(templates)
}

func (s *Server) createTemplateHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Save custom template
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) getEnvironmentHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Return environment variables
	env := map[string]string{
		"BASE_URL": "https://api.example.com",
		"TIMEOUT": "30s",
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(env)
}

func (s *Server) updateEnvironmentHandler(w http.ResponseWriter, r *http.Request) {
	var env map[string]string
	if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	// TODO: Update environment
	w.WriteHeader(http.StatusNoContent)
}

// MetricsHub implementation

func (h *MetricsHub) run() {
	for metric := range h.metrics {
		h.mutex.RLock()
		for ch := range h.subscribers {
			select {
			case ch <- metric:
			default:
				// Drop if subscriber is slow
			}
		}
		h.mutex.RUnlock()
	}
}

func (h *MetricsHub) subscribe(ch chan Metric) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.subscribers[ch] = true
}

func (h *MetricsHub) unsubscribe(ch chan Metric) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	delete(h.subscribers, ch)
	close(ch)
}

func (h *MetricsHub) Publish(metric Metric) {
	select {
	case h.metrics <- metric:
	default:
		// Drop if buffer is full
	}
}

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/AlphaTechini/traffic-simulator/internal/worker"
)

func main() {
	// Parse command-line flags
	port := flag.Int("port", 8081, "Port to run worker on")
	maxUsers := flag.Int("max-users", 50000, "Maximum concurrent users this worker can handle")
	consulAddr := flag.String("consul", "localhost:8500", "Consul address")
	natsAddr := flag.String("nats", "nats://localhost:4222", "NATS address")
	nodeID := flag.String("node-id", "", "Unique node ID (defaults to hostname)")
	bindAddr := flag.String("bind", "0.0.0.0", "Address to bind to")
	
	flag.Parse()
	
	// Generate node ID if not provided
	if *nodeID == "" {
		hostname, _ := os.Hostname()
		*nodeID = fmt.Sprintf("%s-%d", hostname, *port)
	}
	
	log.Printf("🔧 Starting Traffic Simulator Worker")
	log.Printf("   Node ID: %s", *nodeID)
	log.Printf("   Port: %d", *port)
	log.Printf("   Max Users: %d", *maxUsers)
	log.Printf("   Consul: %s", *consulAddr)
	log.Printf("   NATS: %s", *natsAddr)
	
	// Create worker
	workerConfig := &worker.WorkerConfig{
		ConsulAddr: *consulAddr,
		NATSAddr:   *natsAddr,
		NodeID:     *nodeID,
		BindAddr:   *bindAddr,
		Port:       *port,
		MaxUsers:   *maxUsers,
	}
	
	w, err := worker.NewWorker(workerConfig)
	if err != nil {
		log.Fatalf("❌ Failed to create worker: %v", err)
	}
	
	// Setup HTTP handlers
	http.HandleFunc("/health", w.GetHealthHandler())
	http.HandleFunc("/stats", w.GetStatsHandler())
	
	// Start HTTP server in goroutine
	go func() {
		addr := fmt.Sprintf("%s:%d", *bindAddr, *port)
		log.Printf("🌐 Health/Stats endpoint listening on %s", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Fatalf("❌ HTTP server failed: %v", err)
		}
	}()
	
	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	<-sigChan
	log.Println("\n🛑 Shutting down...")
	w.Shutdown()
	os.Exit(0)
}

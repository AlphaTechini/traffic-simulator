package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/AlphaTechini/traffic-simulator/internal/api"
	"github.com/AlphaTechini/traffic-simulator/internal/coordinator"
)

func main() {
	// Parse command-line flags
	port := flag.Int("port", 8080, "Port to run coordinator on")
	consulAddr := flag.String("consul", "localhost:8500", "Consul address")
	natsAddr := flag.String("nats", "nats://localhost:4222", "NATS address")
	nodeID := flag.String("node-id", "", "Unique node ID (defaults to hostname)")
	bindAddr := flag.String("bind", "0.0.0.0", "Address to bind to")
	
	flag.Parse()
	
	// Generate node ID if not provided
	if *nodeID == "" {
		hostname, _ := os.Hostname()
		*nodeID = hostname
	}
	
	log.Printf("🎯 Starting Traffic Simulator Coordinator")
	log.Printf("   Node ID: %s", *nodeID)
	log.Printf("   Port: %d", *port)
	log.Printf("   Consul: %s", *consulAddr)
	log.Printf("   NATS: %s", *natsAddr)
	
	// Create coordinator
	coordConfig := &coordinator.CoordinatorConfig{
		ConsulAddr: *consulAddr,
		NATSAddr:   *natsAddr,
		NodeID:     *nodeID,
		BindAddr:   *bindAddr,
		Port:       *port,
	}
	
	coord, err := coordinator.NewCoordinator(coordConfig)
	if err != nil {
		log.Fatalf("❌ Failed to create coordinator: %v", err)
	}
	
	// Create API server
	apiConfig := &api.ServerConfig{
		Port:         *port + 1, // API runs on port+1
		EnableCORS:   true,
		MaxSimulations: 100,
	}
	
	apiServer := api.NewServer(apiConfig)
	
	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		log.Println("\n🛑 Shutting down...")
		coord.Shutdown()
		os.Exit(0)
	}()
	
	// Start API server in goroutine
	go func() {
		if err := apiServer.Start(); err != nil {
			log.Fatalf("❌ API server failed: %v", err)
		}
	}()
	
	// Block forever (coordinator runs until killed)
	select {}
}

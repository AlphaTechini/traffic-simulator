package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AlphaTechini/traffic-simulator/internal/config"
	"github.com/AlphaTechini/traffic-simulator/internal/scanner"
	"github.com/AlphaTechini/traffic-simulator/internal/simulator"
)

func main() {
	// Command line flags
	baseURL := flag.String("url", "", "Base URL to test (e.g., http://localhost:8080)")
	scan := flag.Bool("scan", false, "Auto-discover routes from backend")
	configFile := flag.String("config", "", "Path to configuration file (JSON)")
	users := flag.Int("users", 100, "Number of concurrent users")
	duration := flag.Duration("duration", 1*time.Minute, "Test duration")
	rampUp := flag.Duration("rampup", 10*time.Second, "Ramp-up time to reach full concurrency")
	auth := flag.String("auth", "", "Authentication header (e.g., 'Bearer token123')")
	timeout := flag.Duration("timeout", 10*time.Second, "HTTP request timeout")
	graphql := flag.Bool("graphql", false, "Enable GraphQL endpoint scanning")
	fast := flag.Bool("fast", false, "Ultra-fast mode: no think times or delays")
	
	flag.Parse()

	if *baseURL == "" && !*scan && *configFile == "" {
		fmt.Println("❌ Error: -url is required (or use -scan or -config)")
		flag.Usage()
		os.Exit(1)
	}

	var simConfig simulator.Config
	var userActions []simulator.UserAction
	var configLoaded bool

	// Load from config file if provided
	if *configFile != "" {
		cfg, err := loadConfig(*configFile, *baseURL, *auth, *timeout)
		if err != nil {
			fmt.Printf("❌ Error loading config: %v\n", err)
			os.Exit(1)
		}
		
		simConfig = cfg.ToSimulatorConfig()
		configLoaded = true
		userActions = simConfig.UserActions
		
		fmt.Println("📄 Loaded configuration from:", *configFile)
	}

	// Auto-discover routes if requested (overrides config)
	if *scan && *baseURL != "" && !configLoaded {
		userActions = scanBackend(*baseURL, *auth, *timeout, *graphql)
		simConfig.UserActions = userActions
	} else if !configLoaded {
		// Use default user actions
		userActions = getDefaultUserActions()
		simConfig.UserActions = userActions
	}
	
	// Override config with command-line flags if provided
	if *baseURL != "" {
		simConfig.BaseURL = *baseURL
	}
	if *users != 100 {
		simConfig.ConcurrentUsers = *users
	}
	if *duration != 1*time.Minute {
		simConfig.Duration = *duration
	}
	if *rampUp != 10*time.Second {
		simConfig.RampUpTime = *rampUp
	}
	
	// Ultra-fast mode: remove all delays
	if *fast {
		fmt.Println("⚡ Ultra-fast mode enabled (no delays)\n")
		for i := range simConfig.UserActions {
			simConfig.UserActions[i].ThinkTimeMs = 0
			for j := range simConfig.UserActions[i].Endpoints {
				simConfig.UserActions[i].Endpoints[j].MinDelayMs = 0
				simConfig.UserActions[i].Endpoints[j].MaxDelayMs = 10 // Minimal delay
			}
		}
	}

	// Handle Ctrl+C gracefully
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n⚠️  Received interrupt signal, stopping...")
		cancel()
	}()

	// Create and start simulator
	sim := simulator.NewTrafficSimulator(simConfig)
	
	fmt.Println("🎯 Traffic Simulator v1.2.0")
	fmt.Println("============================\n")

	if err := sim.Start(ctx); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}

	// Print final statistics
	stats := sim.GetStats()
	fmt.Println("\n📊 Final Statistics:")
	fmt.Printf("   Total Requests:    %d\n", stats.TotalRequests)
	fmt.Printf("   Successful:        %d (%.1f%%)\n", 
		stats.SuccessfulReqs,
		float64(stats.SuccessfulReqs)/float64(stats.TotalRequests)*100)
	fmt.Printf("   Failed:            %d (%.1f%%)\n",
		stats.FailedReqs,
		float64(stats.FailedReqs)/float64(stats.TotalRequests)*100)
	fmt.Printf("   Avg Response Time: %dms\n", stats.AvgResponseTime)
	fmt.Printf("   Duration:          %v\n", time.Since(stats.StartTime))
	
	if stats.TotalRequests > 0 {
		rps := float64(stats.TotalRequests) / time.Since(stats.StartTime).Seconds()
		fmt.Printf("   Requests/Second:   %.2f\n", rps)
	}
}

// loadConfig loads configuration from file with CLI overrides
func loadConfig(configPath, baseURL, auth string, timeout time.Duration) (*config.TrafficSimConfig, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}
	
	// Apply CLI overrides
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	if auth != "" {
		cfg.AuthHeader = auth
	}
	if timeout != 10*time.Second {
		cfg.Timeout = timeout
	}
	
	return cfg, nil
}

// scanBackend discovers routes and generates user actions
func scanBackend(baseURL, auth string, timeout time.Duration, enableGraphQL bool) []simulator.UserAction {
	fmt.Println("🔍 Starting backend discovery...\n")

	scannerConfig := scanner.ScannerConfig{
		BaseURL:         baseURL,
		Timeout:         timeout,
		AuthHeader:      auth,
		FollowRedirects: false,
		GraphQL: scanner.GraphQLConfig{
			Introspection: enableGraphQL,
		},
	}

	sc := scanner.NewRouteScanner(scannerConfig)
	routes, err := sc.Scan(context.Background())
	if err != nil {
		fmt.Printf("⚠️  Warning: Route scanning had issues: %v\n", err)
		fmt.Println("   Falling back to default user actions\n")
		return getDefaultUserActions()
	}

	// Try GraphQL scanning if enabled
	var graphQLActions []simulator.UserAction
	if enableGraphQL {
		graphqlScanner := scanner.NewGraphQLScanner(scannerConfig)
		schema, err := graphqlScanner.Scan(context.Background())
		if err == nil && schema != nil {
			graphQLActions = convertGraphQLActions(graphqlScanner.GenerateGraphQLActions())
			fmt.Printf("✅ Discovered GraphQL: %d queries, %d mutations\n", 
				len(schema.Queries), len(schema.Mutations))
		}
	}

	if len(routes) == 0 && len(graphQLActions) == 0 {
		fmt.Println("⚠️  No routes discovered, using defaults\n")
		return getDefaultUserActions()
	}

	// Convert REST routes to user actions
	restActions := convertRoutesToActions(routes)
	
	// Combine REST and GraphQL actions
	allActions := append(restActions, graphQLActions...)
	
	fmt.Printf("\n✅ Generated %d user action patterns (%d REST + %d GraphQL)\n\n", 
		len(allActions), len(restActions), len(graphQLActions))
	return allActions
}

// convertGraphQLActions converts scanner UserAction to simulator UserAction
func convertGraphQLActions(actions []scanner.UserAction) []simulator.UserAction {
	result := make([]simulator.UserAction, len(actions))
	for i, action := range actions {
		endpoints := make([]simulator.Endpoint, len(action.Endpoints))
		for j, ep := range action.Endpoints {
			endpoints[j] = simulator.Endpoint{
				Method:        ep.Method,
				Path:          ep.Path,
				Weight:        ep.Weight,
				MinDelayMs:    ep.MinDelayMs,
				MaxDelayMs:    ep.MaxDelayMs,
				ErrorRate:     ep.ErrorRate,
				CustomHeaders: ep.CustomHeaders,
			}
		}
		result[i] = simulator.UserAction{
			Name:        action.Name,
			Endpoints:   endpoints,
			ThinkTimeMs: action.ThinkTimeMs,
		}
	}
	return result
}

// convertRoutesToActions converts discovered routes to user actions
func convertRoutesToActions(routes []scanner.Route) []simulator.UserAction {
	actions := make([]simulator.UserAction, 0)

	// Group by resource
	resourceGroups := make(map[string][]simulator.Endpoint)
	for _, route := range routes {
		resource := extractResource(route.Path)
		endpoint := simulator.Endpoint{
			Method: route.Method,
			Path:   route.Path,
			Weight: route.Weight,
			MinDelayMs: 50,
			MaxDelayMs: 500,
			ErrorRate: 0.02,
		}
		resourceGroups[resource] = append(resourceGroups[resource], endpoint)
	}

	// Create actions
	for resource, endpoints := range resourceGroups {
		action := simulator.UserAction{
			Name:        fmt.Sprintf("Browse %s", resource),
			Endpoints:   endpoints,
			ThinkTimeMs: 1000 + len(resource)*100, // Vary think time by resource
		}
		actions = append(actions, action)
	}

	return actions
}

// extractResource extracts resource name from path
func extractResource(path string) string {
	path = path[1:] // Remove leading slash
	if path == "" {
		return "root"
	}
	
	parts := splitPath(path)
	if len(parts) > 0 {
		return parts[0]
	}
	return "root"
}

// splitPath splits path by slashes, handling parameters
func splitPath(path string) []string {
	result := make([]string, 0)
	current := ""
	
	for _, char := range path {
		if char == '/' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else if char != ':' { // Skip parameter markers
			current += string(char)
		}
	}
	
	if current != "" {
		result = append(result, current)
	}
	
	return result
}

// getDefaultUserActions returns default user action patterns
func getDefaultUserActions() []simulator.UserAction {
	return []simulator.UserAction{
		{
			Name: "Homepage Visit",
			Endpoints: []simulator.Endpoint{
				{
					Method:       "GET",
					Path:         "/",
					MinDelayMs:   50,
					MaxDelayMs:   200,
					ErrorRate:    0.01,
				},
			},
			ThinkTimeMs: 1000,
		},
		{
			Name: "API Health Check",
			Endpoints: []simulator.Endpoint{
				{
					Method:       "GET",
					Path:         "/health",
					MinDelayMs:   10,
					MaxDelayMs:   50,
					ErrorRate:    0.0,
				},
			},
			ThinkTimeMs: 500,
		},
		{
			Name: "Browse Content",
			Endpoints: []simulator.Endpoint{
				{
					Method:       "GET",
					Path:         "/api/items",
					MinDelayMs:   100,
					MaxDelayMs:   500,
					ErrorRate:    0.02,
				},
				{
					Method:       "GET",
					Path:         "/api/items/1",
					MinDelayMs:   50,
					MaxDelayMs:   300,
					ErrorRate:    0.01,
				},
			},
			ThinkTimeMs: 2000,
		},
		{
			Name: "User Login Flow",
			Endpoints: []simulator.Endpoint{
				{
					Method:       "POST",
					Path:         "/api/login",
					MinDelayMs:   200,
					MaxDelayMs:   1000,
					ErrorRate:    0.05,
					CustomHeaders: map[string]string{
						"Content-Type": "application/json",
					},
				},
				{
					Method:       "GET",
					Path:         "/api/user/profile",
					MinDelayMs:   100,
					MaxDelayMs:   400,
					ErrorRate:    0.02,
				},
			},
			ThinkTimeMs: 3000,
		},
		{
			Name: "Heavy Load - Search",
			Endpoints: []simulator.Endpoint{
				{
					Method:       "GET",
					Path:         "/api/search?q=test",
					MinDelayMs:   500,
					MaxDelayMs:   2000,
					ErrorRate:    0.03,
				},
			},
			ThinkTimeMs: 1500,
		},
	}
}

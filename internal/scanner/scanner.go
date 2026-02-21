package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Route represents a discovered API endpoint
type Route struct {
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Description string            `json:"description,omitempty"`
	Params      []string          `json:"params,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Weight      int               `json:"weight"` // For random selection
}

// FrameworkType identifies the backend framework
type FrameworkType string

const (
	FrameworkExpress   FrameworkType = "express"
	FrameworkFastify   FrameworkType = "fastify"
	FrameworkNestJS    FrameworkType = "nestjs"
	FrameworkDjango    FrameworkType = "django"
	FrameworkFlask     FrameworkType = "flask"
	FrameworkGin       FrameworkType = "gin"
	FrameworkEcho      FrameworkType = "echo"
	FrameworkUnknown   FrameworkType = "unknown"
)

// ScannerConfig holds configuration for route scanning
type ScannerConfig struct {
	BaseURL        string
	Timeout        time.Duration
	AuthHeader     string // Optional authentication header
	SkipPaths      []string // Paths to skip
	MaxDepth       int    // Maximum routing depth to explore
	FollowRedirects bool
	GraphQL        GraphQLConfig // GraphQL-specific settings
}

// GraphQLConfig holds GraphQL scanning configuration
type GraphQLConfig struct {
	Introspection bool   // Enable schema introspection
	Endpoint      string // GraphQL endpoint path (default: /graphql)
}

// RouteScanner discovers endpoints from a running backend
type RouteScanner struct {
	config  ScannerConfig
	client  *http.Client
	routes  []Route
	framework FrameworkType
}

// NewRouteScanner creates a new route scanner
func NewRouteScanner(config ScannerConfig) *RouteScanner {
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	if config.MaxDepth == 0 {
		config.MaxDepth = 3
	}

	return &RouteScanner{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if !config.FollowRedirects {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
		routes: make([]Route, 0),
	}
}

// Scan attempts to discover routes from the backend
func (rs *RouteScanner) Scan(ctx context.Context) ([]Route, error) {
	fmt.Printf("🔍 Scanning backend for routes: %s\n", rs.config.BaseURL)

	// Step 1: Try to detect framework
	rs.detectFramework(ctx)

	// Step 2: Try common route discovery methods
	rs.tryOpenAPIDiscovery(ctx)
	rs.tryCommonEndpoints(ctx)
	rs.fuzzCommonPatterns(ctx)

	// Step 3: Remove duplicates and sort
	rs.deduplicateRoutes()

	fmt.Printf("✅ Discovered %d routes\n", len(rs.routes))
	return rs.routes, nil
}

// detectFramework tries to identify the backend framework
func (rs *RouteScanner) detectFramework(ctx context.Context) {
	// Check common framework signatures
	signatures := map[FrameworkType][]string{
		FrameworkExpress: {
			"X-Powered-By: Express",
			"Express",
		},
		FrameworkFastify: {
			"X-Powered-By: Fastify",
			"Fastify",
		},
		FrameworkNestJS: {
			"X-Powered-By: NestJS",
			"NestJS",
		},
	}

	req, _ := http.NewRequestWithContext(ctx, "GET", rs.config.BaseURL, nil)
	resp, err := rs.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	headers := resp.Header.Get("X-Powered-By")
	server := resp.Header.Get("Server")

	for framework, sigs := range signatures {
		for _, sig := range sigs {
			if strings.Contains(headers, sig) || strings.Contains(server, sig) {
				rs.framework = framework
				fmt.Printf("   Detected framework: %s\n", framework)
				return
			}
		}
	}

	rs.framework = FrameworkUnknown
}

// tryOpenAPIDiscovery looks for OpenAPI/Swagger documentation
func (rs *RouteScanner) tryOpenAPIDiscovery(ctx context.Context) {
	openAPIPaths := []string{
		"/openapi.json",
		"/swagger.json",
		"/api/openapi.json",
		"/api/swagger.json",
		"/docs/openapi.json",
		"/v1/openapi.json",
		"/v1/swagger.json",
	}

	for _, path := range openAPIPaths {
		url := rs.config.BaseURL + path
		req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
		resp, err := rs.client.Do(req)
		if err != nil {
			continue
		}

		if resp.StatusCode == 200 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			var openAPI map[string]interface{}
			if err := json.Unmarshal(body, &openAPI); err == nil {
				rs.parseOpenAPI(openAPI)
				fmt.Printf("   Found OpenAPI spec at: %s\n", path)
				return
			}
		}
	}
}

// parseOpenAPI extracts routes from OpenAPI specification
func (rs *RouteScanner) parseOpenAPI(spec map[string]interface{}) {
	paths, ok := spec["paths"].(map[string]interface{})
	if !ok {
		return
	}

	for path, methods := range paths {
		methodMap, ok := methods.(map[string]interface{})
		if !ok {
			continue
		}

		for method, details := range methodMap {
			if method == "$ref" || method == "parameters" {
				continue
			}

			route := Route{
				Method: strings.ToUpper(method),
				Path:   path,
				Weight: 10, // Default weight
			}

			// Extract description if available
			if detailMap, ok := details.(map[string]interface{}); ok {
				if desc, ok := detailMap["summary"].(string); ok {
					route.Description = desc
				}
				if params, ok := detailMap["parameters"].([]interface{}); ok {
					route.Params = rs.extractParams(params)
				}
			}

			rs.routes = append(rs.routes, route)
		}
	}
}

// extractParams extracts parameter names from OpenAPI parameters
func (rs *RouteScanner) extractParams(params []interface{}) []string {
	result := make([]string, 0)
	for _, p := range params {
		if param, ok := p.(map[string]interface{}); ok {
			if name, ok := param["name"].(string); ok {
				result = append(result, name)
			}
		}
	}
	return result
}

// tryCommonEndpoints checks for common API endpoints
func (rs *RouteScanner) tryCommonEndpoints(ctx context.Context) {
	commonEndpoints := []struct {
		Method string
		Path   string
		Weight int
	}{
		{"GET", "/", 5},
		{"GET", "/health", 10},
		{"GET", "/api/health", 10},
		{"GET", "/status", 8},
		{"GET", "/api/status", 8},
		{"GET", "/api/users", 7},
		{"GET", "/api/posts", 7},
		{"GET", "/api/items", 7},
		{"POST", "/api/login", 9},
		{"POST", "/api/auth/login", 9},
		{"GET", "/api/products", 6},
		{"GET", "/api/search", 6},
	}

	for _, endpoint := range commonEndpoints {
		// Skip if already found
		found := false
		for _, r := range rs.routes {
			if r.Method == endpoint.Method && r.Path == endpoint.Path {
				found = true
				break
			}
		}
		if found {
			continue
		}

		url := rs.config.BaseURL + endpoint.Path
		req, _ := http.NewRequestWithContext(ctx, endpoint.Method, url, nil)
		if rs.config.AuthHeader != "" {
			req.Header.Set("Authorization", rs.config.AuthHeader)
		}

		resp, err := rs.client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()

		// If we get any response (even 404), the route exists
		if resp.StatusCode != 404 || resp.StatusCode < 500 {
			rs.routes = append(rs.routes, Route{
				Method: endpoint.Method,
				Path:   endpoint.Path,
				Weight: endpoint.Weight,
			})
		}
	}
}

// fuzzCommonPatterns tries common URL patterns
func (rs *RouteScanner) fuzzCommonPatterns(ctx context.Context) {
	// Resource names to try
	resources := []string{
		"users", "posts", "items", "products", "orders",
		"comments", "categories", "tags", "settings", "profile",
		"auth", "login", "register", "logout", "password",
	}

	// Patterns to try
	patterns := []struct {
		Prefix string
		Suffix string
		Method string
	}{
		{"/api/", "", "GET"},
		{"/api/", "/:id", "GET"},
		{"/api/v1/", "", "GET"},
		{"/v1/", "", "GET"},
		{"/", "", "GET"},
		{"/api/", "", "POST"},
	}

	for _, resource := range resources {
		for _, pattern := range patterns {
			path := pattern.Prefix + resource + pattern.Suffix

			// Skip if already found
			found := false
			for _, r := range rs.routes {
				if r.Path == path {
					found = true
					break
				}
			}
			if found {
				continue
			}

			url := rs.config.BaseURL + path
			req, _ := http.NewRequestWithContext(ctx, pattern.Method, url, nil)
			if rs.config.AuthHeader != "" {
				req.Header.Set("Authorization", rs.config.AuthHeader)
			}

			resp, err := rs.client.Do(req)
			if err != nil {
				continue
			}
			resp.Body.Close()

			// Accept any non-5xx response as valid route
			if resp.StatusCode < 500 {
				rs.routes = append(rs.routes, Route{
					Method: pattern.Method,
					Path:   path,
					Weight: 3, // Lower weight for fuzzed routes
				})
			}
		}
	}
}

// deduplicateRoutes removes duplicate routes
func (rs *RouteScanner) deduplicateRoutes() {
	seen := make(map[string]bool)
	unique := make([]Route, 0)

	for _, route := range rs.routes {
		key := fmt.Sprintf("%s:%s", route.Method, route.Path)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, route)
		}
	}

	rs.routes = unique
}

// GetFramework returns the detected framework
func (rs *RouteScanner) GetFramework() FrameworkType {
	return rs.framework
}

// GenerateUserActions creates user action patterns from discovered routes
func (rs *RouteScanner) GenerateUserActions(routes []Route) []UserAction {
	actions := make([]UserAction, 0)

	// Group routes by resource
	resourceGroups := make(map[string][]Endpoint)
	for _, route := range routes {
		resource := rs.extractResource(route.Path)
		endpoint := Endpoint{
			Method:   route.Method,
			Path:     route.Path,
			Weight:   route.Weight,
			MinDelayMs: 50,
			MaxDelayMs: 500,
			ErrorRate:  0.02,
		}
		resourceGroups[resource] = append(resourceGroups[resource], endpoint)
	}

	// Create user actions for each resource group
	for resource, endpoints := range resourceGroups {
		action := UserAction{
			Name:        fmt.Sprintf("Browse %s", resource),
			Endpoints:   endpoints,
			ThinkTimeMs: 1000,
		}
		actions = append(actions, action)
	}

	// Add common flows
	actions = append(actions, rs.createCommonFlows(routes)...)

	return actions
}

// extractResource extracts resource name from path
func (rs *RouteScanner) extractResource(path string) string {
	// Remove leading slash and query params
	path = strings.TrimPrefix(path, "/")
	path = strings.Split(path, "?")[0]

	// Split by slash and take first segment
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return "root"
}

// createCommonFlows creates common user journey patterns
func (rs *RouteScanner) createCommonFlows(routes []Route) []UserAction {
	flows := make([]UserAction, 0)

	// Look for login flow
	var loginRoute, profileRoute *Endpoint
	for i := range routes {
		if routes[i].Path == "/api/login" || routes[i].Path == "/api/auth/login" {
			loginRoute = &Endpoint{
				Method:   routes[i].Method,
				Path:     routes[i].Path,
				Weight:   routes[i].Weight,
				MinDelayMs: 200,
				MaxDelayMs: 1000,
				ErrorRate:  0.05,
			}
		}
		if routes[i].Path == "/api/user/profile" || routes[i].Path == "/api/profile" {
			profileRoute = &Endpoint{
				Method:   routes[i].Method,
				Path:     routes[i].Path,
				Weight:   routes[i].Weight,
				MinDelayMs: 100,
				MaxDelayMs: 400,
				ErrorRate:  0.02,
			}
		}
	}

	if loginRoute != nil && profileRoute != nil {
		flows = append(flows, UserAction{
			Name: "User Login Flow",
			Endpoints: []Endpoint{
				*loginRoute,
				*profileRoute,
			},
			ThinkTimeMs: 2000,
		})
	}

	return flows
}

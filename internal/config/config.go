package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlphaTechini/traffic-simulator/internal/simulator"
)

// TrafficSimConfig holds complete simulator configuration
type TrafficSimConfig struct {
	// Target backend
	BaseURL string `json:"base_url" yaml:"base_url"`
	
	// Load test parameters
	ConcurrentUsers int           `json:"concurrent_users" yaml:"concurrent_users"`
	Duration        time.Duration `json:"duration" yaml:"duration"`
	RampUpTime      time.Duration `json:"rampup_time" yaml:"rampup_time"`
	
	// Route scanning
	AutoScan       bool     `json:"auto_scan" yaml:"auto_scan"`
	SkipPaths      []string `json:"skip_paths" yaml:"skip_paths"`
	IncludePatterns []string `json:"include_patterns" yaml:"include_patterns"`
	
	// Authentication
	AuthHeader string `json:"auth_header" yaml:"auth_header"`
	APIKey     string `json:"api_key" yaml:"api_key"`
	
	// HTTP settings
	Timeout         time.Duration `json:"timeout" yaml:"timeout"`
	MaxConnections  int           `json:"max_connections" yaml:"max_connections"`
	FollowRedirects bool          `json:"follow_redirects" yaml:"follow_redirects"`
	
	// User actions (if not using auto-scan)
	UserActions []UserActionConfig `json:"user_actions" yaml:"user_actions"`
	
	// GraphQL specific
	GraphQL GraphQLConfig `json:"graphql" yaml:"graphql"`
	
	// Reporting
	ReportInterval time.Duration `json:"report_interval" yaml:"report_interval"`
	OutputFormat   string        `json:"output_format" yaml:"output_format"` // text, json, csv
	OutputFile     string        `json:"output_file" yaml:"output_file"`
	
	// Advanced
	RandomSeed      int64 `json:"random_seed" yaml:"random_seed"`
	Verbose         bool  `json:"verbose" yaml:"verbose"`
	Quiet           bool  `json:"quiet" yaml:"quiet"`
}

// UserActionConfig defines a user journey
type UserActionConfig struct {
	Name        string             `json:"name" yaml:"name"`
	Endpoints   []EndpointConfig   `json:"endpoints" yaml:"endpoints"`
	ThinkTimeMs int                `json:"think_time_ms" yaml:"think_time_ms"`
	Weight      int                `json:"weight" yaml:"weight"` // Probability weight
	Condition   string             `json:"condition" yaml:"condition"` // Optional condition
}

// EndpointConfig defines a single HTTP request
type EndpointConfig struct {
	Method        string            `json:"method" yaml:"method"`
	Path          string            `json:"path" yaml:"path"`
	MinDelayMs    int               `json:"min_delay_ms" yaml:"min_delay_ms"`
	MaxDelayMs    int               `json:"max_delay_ms" yaml:"max_delay_ms"`
	ErrorRate     float64           `json:"error_rate" yaml:"error_rate"`
	CustomHeaders map[string]string `json:"custom_headers" yaml:"custom_headers"`
	Body          string            `json:"body" yaml:"body"`
	ContentType   string            `json:"content_type" yaml:"content_type"`
}

// GraphQLConfig holds GraphQL-specific settings
type GraphQLConfig struct {
	Enabled       bool              `json:"enabled" yaml:"enabled"`
	Endpoint      string            `json:"endpoint" yaml:"endpoint"` // Usually /graphql
	Queries       []GraphQLQuery    `json:"queries" yaml:"queries"`
	Mutations     []GraphQLMutation `json:"mutations" yaml:"mutations"`
	Introspection bool              `json:"introspection" yaml:"introspection"` // Auto-discover schema
	BatchSize     int               `json:"batch_size" yaml:"batch_size"`       // Number of ops per request
}

// GraphQLQuery defines a GraphQL query
type GraphQLQuery struct {
	Name       string            `json:"name" yaml:"name"`
	Query      string            `json:"query" yaml:"query"`
	Variables  map[string]interface{} `json:"variables" yaml:"variables"`
	Weight     int               `json:"weight" yaml:"weight"`
	Operation  string            `json:"operation" yaml:"operation"` // query, subscription
}

// GraphQLMutation defines a GraphQL mutation
type GraphQLMutation struct {
	Name       string            `json:"name" yaml:"name"`
	Mutation   string            `json:"mutation" yaml:"mutation"`
	Variables  map[string]interface{} `json:"variables" yaml:"variables"`
	Weight     int               `json:"weight" yaml:"weight"`
}

// Load reads configuration from file
func Load(configPath string) (*TrafficSimConfig, error) {
	if configPath == "" {
		return nil, fmt.Errorf("config path is required")
	}

	// Expand ~ to home directory
	if strings.HasPrefix(configPath, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		configPath = filepath.Join(homeDir, configPath[2:])
	}

	// Read file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config TrafficSimConfig

	// Detect format by extension
	ext := strings.ToLower(filepath.Ext(configPath))
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	case ".yaml", ".yml":
		// YAML support requires gopkg.in/yaml.v3
		// For now, we'll just support JSON
		return nil, fmt.Errorf("YAML format not yet supported, please use JSON")
	default:
		// Try JSON first
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("unknown config format '%s', expected .json", ext)
		}
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// Validate checks configuration validity
func (c *TrafficSimConfig) Validate() error {
	if c.BaseURL == "" && !c.AutoScan {
		return fmt.Errorf("base_url is required when auto_scan is disabled")
	}

	if c.ConcurrentUsers < 1 {
		return fmt.Errorf("concurrent_users must be at least 1")
	}

	if c.Duration <= 0 {
		return fmt.Errorf("duration must be positive")
	}

	if c.Timeout <= 0 {
		c.Timeout = 30 * time.Second // Default
	}

	if c.MaxConnections == 0 {
		c.MaxConnections = c.ConcurrentUsers * 2 // Default
	}

	// Validate GraphQL config if enabled
	if c.GraphQL.Enabled {
		if c.GraphQL.Endpoint == "" {
			c.GraphQL.Endpoint = "/graphql" // Default
		}
		if len(c.GraphQL.Queries) == 0 && len(c.GraphQL.Mutations) == 0 {
			return fmt.Errorf("graphql enabled but no queries or mutations defined")
		}
	}

	return nil
}

// ToSimulatorConfig converts to simulator.Config
func (c *TrafficSimConfig) ToSimulatorConfig() simulator.Config {
	userActions := make([]simulator.UserAction, 0)

	// Convert user actions from config
	for _, actionCfg := range c.UserActions {
		endpoints := make([]simulator.Endpoint, 0)
		for _, epCfg := range actionCfg.Endpoints {
			endpoint := simulator.Endpoint{
				Method:        epCfg.Method,
				Path:          epCfg.Path,
				MinDelayMs:    epCfg.MinDelayMs,
				MaxDelayMs:    epCfg.MaxDelayMs,
				ErrorRate:     epCfg.ErrorRate,
				CustomHeaders: epCfg.CustomHeaders,
			}
			endpoints = append(endpoints, endpoint)
		}

		userActions = append(userActions, simulator.UserAction{
			Name:        actionCfg.Name,
			Endpoints:   endpoints,
			ThinkTimeMs: actionCfg.ThinkTimeMs,
		})
	}

	// Add GraphQL operations as user actions if enabled
	if c.GraphQL.Enabled {
		userActions = append(userActions, c.buildGraphQLActions()...)
	}

	return simulator.Config{
		BaseURL:         c.BaseURL,
		ConcurrentUsers: c.ConcurrentUsers,
		Duration:        c.Duration,
		RampUpTime:      c.RampUpTime,
		UserActions:     userActions,
		ReportInterval:  c.ReportInterval,
		RandomSeed:      c.RandomSeed,
	}
}

// buildGraphQLActions creates user actions from GraphQL queries/mutations
func (c *TrafficSimConfig) buildGraphQLActions() []simulator.UserAction {
	actions := make([]simulator.UserAction, 0)

	// Create action for queries
	if len(c.GraphQL.Queries) > 0 {
		endpoints := make([]simulator.Endpoint, 0)
		for _, query := range c.GraphQL.Queries {
			endpoint := simulator.Endpoint{
				Method: "POST",
				Path:   c.GraphQL.Endpoint,
				Weight: query.Weight,
			}
			endpoints = append(endpoints, endpoint)
		}

		actions = append(actions, simulator.UserAction{
			Name:        "GraphQL Queries",
			Endpoints:   endpoints,
			ThinkTimeMs: 500,
		})
	}

	// Create action for mutations
	if len(c.GraphQL.Mutations) > 0 {
		endpoints := make([]simulator.Endpoint, 0)
		for _, mutation := range c.GraphQL.Mutations {
			endpoint := simulator.Endpoint{
				Method: "POST",
				Path:   c.GraphQL.Endpoint,
				Weight: mutation.Weight,
			}
			endpoints = append(endpoints, endpoint)
		}

		actions = append(actions, simulator.UserAction{
			Name:        "GraphQL Mutations",
			Endpoints:   endpoints,
			ThinkTimeMs: 1000,
		})
	}

	return actions
}

// Save writes configuration to file
func (c *TrafficSimConfig) Save(configPath string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() *TrafficSimConfig {
	return &TrafficSimConfig{
		ConcurrentUsers: 100,
		Duration:        1 * time.Minute,
		RampUpTime:      10 * time.Second,
		Timeout:         30 * time.Second,
		AutoScan:        false,
		ReportInterval:  5 * time.Second,
		OutputFormat:    "text",
		GraphQL: GraphQLConfig{
			Enabled:   false,
			Endpoint:  "/graphql",
			BatchSize: 1,
		},
	}
}

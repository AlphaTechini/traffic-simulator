package patterns

import (
	"fmt"
	"sync"
	"time"
)

// Pattern defines how user count changes over time during a simulation
type Pattern interface {
	// GetUserCount returns desired number of users at given elapsed time
	GetUserCount(elapsed time.Duration) int
	
	// GetDuration returns total pattern duration
	GetDuration() time.Duration
	
	// Validate checks if pattern configuration is valid
	Validate() error
	
	// GetType returns pattern type name
	GetType() string
}

// Engine manages pattern registry and creation
type Engine struct {
	registry map[string]PatternFactory
	mu       sync.RWMutex
}

type PatternFactory func(config map[string]interface{}) (Pattern, error)

// NewEngine creates a new pattern engine
func NewEngine() *Engine {
	e := &Engine{
		registry: make(map[string]PatternFactory),
	}
	
	// Register built-in patterns
	e.Register("constant", NewConstantPattern)
	e.Register("ramp", NewRampPattern)
	// TODO: Implement remaining patterns in separate commits
	// e.Register("step", NewStepPattern)
	// e.Register("wave", NewWavePattern)
	// e.Register("burst", NewBurstPattern)
	// e.Register("custom", NewCustomPattern)
	// e.Register("business_hours", NewBusinessHoursPattern)
	
	return e
}

// Register adds a pattern type to the engine
func (e *Engine) Register(name string, factory PatternFactory) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.registry[name] = factory
}

// Create instantiates a pattern by name with config
func (e *Engine) Create(name string, config map[string]interface{}) (Pattern, error) {
	e.mu.RLock()
	factory, exists := e.registry[name]
	e.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("unknown pattern type: %s", name)
	}
	
	return factory(config)
}

// List returns all registered pattern types
func (e *Engine) List() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	types := make([]string, 0, len(e.registry))
	for t := range e.registry {
		types = append(types, t)
	}
	return types
}

// ConstantPattern maintains steady user count throughout simulation
type ConstantPattern struct {
	userCount int
	duration  time.Duration
}

func NewConstantPattern(config map[string]interface{}) (Pattern, error) {
	users, ok := config["users"].(int)
	if !ok || users <= 0 {
		return nil, fmt.Errorf("invalid users count")
	}
	
	duration, ok := config["duration"].(time.Duration)
	if !ok || duration <= 0 {
		return nil, fmt.Errorf("invalid duration")
	}
	
	return &ConstantPattern{
		userCount: users,
		duration:  duration,
	}, nil
}

func (p *ConstantPattern) GetUserCount(elapsed time.Duration) int {
	if elapsed > p.duration {
		return 0
	}
	return p.userCount
}

func (p *ConstantPattern) GetDuration() time.Duration {
	return p.duration
}

func (p *ConstantPattern) Validate() error {
	if p.userCount <= 0 {
		return fmt.Errorf("user count must be positive")
	}
	if p.duration <= 0 {
		return fmt.Errorf("duration must be positive")
	}
	return nil
}

func (p *ConstantPattern) GetType() string {
	return "constant"
}

// RampPattern linearly increases/decreases users over time
type RampPattern struct {
	startUsers    int
	endUsers      int
	duration      time.Duration
	rampType      string // "linear" or "exponential"
}

func NewRampPattern(config map[string]interface{}) (Pattern, error) {
	start, _ := config["start_users"].(int)
	end, _ := config["end_users"].(int)
	duration, _ := config["duration"].(time.Duration)
	rampType, _ := config["ramp_type"].(string)
	
	if rampType == "" {
		rampType = "linear"
	}
	
	p := &RampPattern{
		startUsers: start,
		endUsers:   end,
		duration:   duration,
		rampType:   rampType,
	}
	
	return p, p.Validate()
}

func (p *RampPattern) GetUserCount(elapsed time.Duration) int {
	if elapsed > p.duration {
		return 0
	}
	
	progress := float64(elapsed) / float64(p.duration)
	
	if p.rampType == "exponential" {
		// Exponential curve
		ratio := float64(p.endUsers) / float64(p.startUsers)
		current := float64(p.startUsers) * pow(ratio, progress)
		return int(current)
	}
	
	// Linear interpolation
	delta := p.endUsers - p.startUsers
	return p.startUsers + int(float64(delta)*progress)
}

func (p *RampPattern) GetDuration() time.Duration {
	return p.duration
}

func (p *RampPattern) Validate() error {
	if p.startUsers < 0 {
		return fmt.Errorf("start users cannot be negative")
	}
	if p.endUsers < 0 {
		return fmt.Errorf("end users cannot be negative")
	}
	if p.duration <= 0 {
		return fmt.Errorf("duration must be positive")
	}
	return nil
}

func (p *RampPattern) GetType() string {
	return "ramp"
}

// Helper function for exponential calculation
func pow(base float64, exp float64) float64 {
	result := 1.0
	for i := 0; i < int(exp*100); i++ {
		result *= base
	}
	return result
}

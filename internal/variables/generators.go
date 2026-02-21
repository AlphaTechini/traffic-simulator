package variables

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"time"
)

// Generator produces dynamic values for request templates
type Generator interface {
	Generate() string
	Name() string
}

// UUIDGenerator generates random UUIDs
type UUIDGenerator struct{}

func NewUUIDGenerator() *UUIDGenerator {
	return &UUIDGenerator{}
}

func (g *UUIDGenerator) Generate() string {
	uuid := make([]byte, 16)
	rand.Read(uuid)
	
	// Set version and variant bits
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // Version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // Variant
	
	return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])
}

func (g *UUIDGenerator) Name() string {
	return "uuid"
}

// EmailGenerator generates random email addresses
type EmailGenerator struct {
	domain string
	mu     sync.RWMutex
}

func NewEmailGenerator(domain string) *EmailGenerator {
	if domain == "" {
		domain = "test.example.com"
	}
	return &EmailGenerator{domain: domain}
}

func (g *EmailGenerator) Generate() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	timestamp := time.Now().UnixNano()
	random, _ := rand.Int(rand.Reader, big.NewInt(10000))
	
	return fmt.Sprintf("user_%d_%d@%s", timestamp, random, g.domain)
}

func (g *EmailGenerator) Name() string {
	return "email"
}

// TimestampGenerator generates Unix timestamps
type TimestampGenerator struct {
	format string // "unix", "iso", "rfc3339"
	offset time.Duration
}

func NewTimestampGenerator(format string, offset time.Duration) *TimestampGenerator {
	if format == "" {
		format = "unix"
	}
	return &TimestampGenerator{format: format, offset: offset}
}

func (g *TimestampGenerator) Generate() string {
	t := time.Now().Add(g.offset)
	
	switch g.format {
	case "unix":
		return fmt.Sprintf("%d", t.Unix())
	case "unix_ms":
		return fmt.Sprintf("%d", t.UnixNano()/1e6)
	case "iso":
		return t.Format("2006-01-02T15:04:05Z")
	case "rfc3339":
		return t.Format(time.RFC3339)
	default:
		return fmt.Sprintf("%d", t.Unix())
	}
}

func (g *TimestampGenerator) Name() string {
	return "timestamp"
}

// IncrementGenerator generates incrementing numbers
type IncrementGenerator struct {
	current int64
	step    int64
	mu      sync.Mutex
}

func NewIncrementGenerator(start, step int64) *IncrementGenerator {
	return &IncrementGenerator{
		current: start,
		step:    step,
	}
}

func (g *IncrementGenerator) Generate() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	value := g.current
	g.current += g.step
	
	return fmt.Sprintf("%d", value)
}

func (g *IncrementGenerator) Name() string {
	return "increment"
}

// RandomStringGenerator generates random alphanumeric strings
type RandomStringGenerator struct {
	length int
	charset string
}

func NewRandomStringGenerator(length int, charset string) *RandomStringGenerator {
	if length <= 0 {
		length = 8
	}
	if charset == "" {
		charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	}
	return &RandomStringGenerator{
		length:  length,
		charset: charset,
	}
}

func (g *RandomStringGenerator) Generate() string {
	result := make([]byte, g.length)
	charsetLen := len(g.charset)
	
	for i := 0; i < g.length; i++ {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(charsetLen)))
		result[i] = g.charset[idx.Int64()]
	}
	
	return string(result)
}

func (g *RandomStringGenerator) Name() string {
	return "random_string"
}

// Registry manages all available generators
type Registry struct {
	generators map[string]Generator
	mu         sync.RWMutex
}

func NewRegistry() *Registry {
	r := &Registry{
		generators: make(map[string]Generator),
	}
	
	// Register built-in generators
	r.Register(NewUUIDGenerator())
	r.Register(NewEmailGenerator(""))
	r.Register(NewTimestampGenerator("unix", 0))
	r.Register(NewIncrementGenerator(1, 1))
	r.Register(NewRandomStringGenerator(8, ""))
	
	return r
}

func (r *Registry) Register(gen Generator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.generators[gen.Name()] = gen
}

func (r *Registry) Get(name string) (Generator, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	gen, exists := r.generators[name]
	return gen, exists
}

func (r *Registry) Generate(name string) (string, error) {
	gen, exists := r.Get(name)
	if !exists {
		return "", fmt.Errorf("unknown generator: %s", name)
	}
	return gen.Generate(), nil
}

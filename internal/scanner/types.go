package scanner

// UserAction represents a sequence of HTTP requests (mirroring simulator.UserAction)
type UserAction struct {
	Name        string
	Endpoints   []Endpoint
	ThinkTimeMs int
}

// Endpoint represents a single HTTP request (mirroring simulator.Endpoint)
type Endpoint struct {
	Method        string
	Path          string
	Weight        int
	MinDelayMs    int
	MaxDelayMs    int
	ErrorRate     float64
	CustomHeaders map[string]string
}

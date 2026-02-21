package patterns

import (
	"fmt"
	"time"
)

// CustomPattern allows user-defined traffic curve via data points
type CustomPattern struct {
	points   []CurvePoint
	duration time.Duration
}

type CurvePoint struct {
	Time  time.Duration // Time from start
	Users int           // User count at this time
}

func NewCustomPattern(config map[string]interface{}) (Pattern, error) {
	pointsData, _ := config["points"].([]map[string]interface{})
	duration, _ := config["duration"].(time.Duration)
	
	if len(pointsData) < 2 {
		return nil, fmt.Errorf("at least 2 curve points required")
	}
	
	points := make([]CurvePoint, len(pointsData))
	maxTime := time.Duration(0)
	
	for i, p := range pointsData {
		t, _ := p["time"].(time.Duration)
		users, _ := p["users"].(int)
		
		points[i] = CurvePoint{
			Time:  t,
			Users: users,
		}
		
		if t > maxTime {
			maxTime = t
		}
	}
	
	if duration == 0 {
		duration = maxTime
	}
	
	p := &CustomPattern{
		points:   points,
		duration: duration,
	}
	
	return p, p.Validate()
}

func (p *CustomPattern) GetUserCount(elapsed time.Duration) int {
	if elapsed > p.duration {
		return 0
	}
	
	// Find surrounding points and interpolate
	for i := 0; i < len(p.points)-1; i++ {
		current := p.points[i]
		next := p.points[i+1]
		
		if elapsed >= current.Time && elapsed < next.Time {
			// Linear interpolation between points
			progress := float64(elapsed-current.Time) / float64(next.Time-current.Time)
			delta := next.Users - current.Users
			return current.Users + int(float64(delta)*progress)
		}
	}
	
	// Beyond last point
	lastPoint := p.points[len(p.points)-1]
	return lastPoint.Users
}

func (p *CustomPattern) GetDuration() time.Duration {
	return p.duration
}

func (p *CustomPattern) Validate() error {
	if len(p.points) < 2 {
		return fmt.Errorf("at least 2 points required")
	}
	
	// Verify points are in chronological order
	for i := 1; i < len(p.points); i++ {
		if p.points[i].Time <= p.points[i-1].Time {
			return fmt.Errorf("points must be in chronological order (point %d)", i)
		}
		if p.points[i].Users < 0 {
			return fmt.Errorf("point %d: users cannot be negative", i)
		}
	}
	
	return nil
}

func (p *CustomPattern) GetType() string {
	return "custom"
}

// LoadFromCSV loads custom pattern from CSV file
// Format: time_in_seconds,user_count
func LoadFromCSV(csvData string) (*CustomPattern, error) {
	lines := splitLines(csvData)
	points := make([]CurvePoint, 0, len(lines))
	
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		
		// Simple CSV parse (time,users)
		var tSec int
		var users int
		// Parse logic would go here in production
		
		points = append(points, CurvePoint{
			Time:  time.Duration(tSec) * time.Second,
			Users: users,
		})
	}
	
	return &CustomPattern{points: points}, nil
}

func splitLines(data string) []string {
	// Simplified line splitter
	return []string{}
}

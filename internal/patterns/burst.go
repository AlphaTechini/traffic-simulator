package patterns

import (
	"fmt"
	"math"
	"time"
)

// BurstPattern creates sudden traffic spikes at specified times
type BurstPattern struct {
	baseUsers   int
	bursts      []Burst
	duration    time.Duration
}

type Burst struct {
	StartTime    time.Duration // When burst starts
	Duration     time.Duration // How long burst lasts
	PeakUsers    int           // Peak user count during burst
	RampUp       time.Duration // Time to reach peak
	RampDown     time.Duration // Time to return to base
	Shape        string        // "triangle", "square", "exponential"
}

func NewBurstPattern(config map[string]interface{}) (Pattern, error) {
	base, _ := config["base_users"].(int)
	duration, _ := config["duration"].(time.Duration)
	burstsData, _ := config["bursts"].([]map[string]interface{})
	
	if base <= 0 {
		base = 100
	}
	if duration <= 0 {
		duration = 1 * time.Hour
	}
	
	bursts := make([]Burst, len(burstsData))
	for i, b := range burstsData {
		start, _ := b["start_time"].(time.Duration)
		dur, _ := b["duration"].(time.Duration)
		peak, _ := b["peak_users"].(int)
		rampUp, _ := b["ramp_up"].(time.Duration)
		rampDown, _ := b["ramp_down"].(time.Duration)
		shape, _ := b["shape"].(string)
		
		if shape == "" {
			shape = "triangle"
		}
		if rampUp == 0 {
			rampUp = dur / 4
		}
		if rampDown == 0 {
			rampDown = dur / 4
		}
		
		bursts[i] = Burst{
			StartTime: start,
			Duration:  dur,
			PeakUsers: peak,
			RampUp:    rampUp,
			RampDown:  rampDown,
			Shape:     shape,
		}
	}
	
	p := &BurstPattern{
		baseUsers: base,
		bursts:    bursts,
		duration:  duration,
	}
	
	return p, p.Validate()
}

func (p *BurstPattern) GetUserCount(elapsed time.Duration) int {
	if elapsed > p.duration {
		return 0
	}
	
	// Check if we're in any burst
	for _, burst := range p.bursts {
		if elapsed >= burst.StartTime && elapsed < burst.StartTime+burst.Duration {
			return p.calculateBurstUsers(elapsed, burst)
		}
	}
	
	// Base traffic outside bursts
	return p.baseUsers
}

func (p *BurstPattern) calculateBurstUsers(elapsed time.Duration, burst Burst) int {
	timeIntoBurst := elapsed - burst.StartTime
	
	var users float64
	
	switch burst.Shape {
	case "square":
		users = float64(burst.PeakUsers)
		
	case "triangle":
		// Linear ramp up, linear ramp down
		if timeIntoBurst < burst.RampUp {
			// Ramping up
			progress := float64(timeIntoBurst) / float64(burst.RampUp)
			users = float64(p.baseUsers) + (float64(burst.PeakUsers)-float64(p.baseUsers))*progress
		} else if timeIntoBurst > burst.Duration-burst.RampDown {
			// Ramping down
			timeRemaining := burst.Duration - timeIntoBurst
			progress := float64(timeRemaining) / float64(burst.RampDown)
			users = float64(p.baseUsers) + (float64(burst.PeakUsers)-float64(p.baseUsers))*progress
		} else {
			// At peak
			users = float64(burst.PeakUsers)
		}
		
	case "exponential":
		// Exponential rise and fall
		if timeIntoBurst < burst.RampUp {
			progress := float64(timeIntoBurst) / float64(burst.RampUp)
			users = float64(p.baseUsers) + float64(burst.PeakUsers-p.baseUsers)*(1-math.Exp(-5*progress))
		} else {
			progress := float64(burst.Duration-timeIntoBurst) / float64(burst.RampDown)
			users = float64(p.baseUsers) + float64(burst.PeakUsers-p.baseUsers)*(1-math.Exp(-5*progress))
		}
	}
	
	return int(users)
}

func (p *BurstPattern) GetDuration() time.Duration {
	return p.duration
}

func (p *BurstPattern) Validate() error {
	if p.baseUsers < 0 {
		return fmt.Errorf("base users cannot be negative")
	}
	
	for i, burst := range p.bursts {
		if burst.PeakUsers < p.baseUsers {
			return fmt.Errorf("burst %d: peak users must exceed base users", i)
		}
		if burst.RampUp+burst.RampDown > burst.Duration {
			return fmt.Errorf("burst %d: ramp times exceed burst duration", i)
		}
		if burst.StartTime+burst.Duration > p.duration {
			return fmt.Errorf("burst %d: extends beyond total duration", i)
		}
	}
	
	return nil
}

func (p *BurstPattern) GetType() string {
	return "burst"
}

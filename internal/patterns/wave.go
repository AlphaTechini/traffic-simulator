package patterns

import (
	"fmt"
	"math"
	"time"
)

// WavePattern creates cyclical sine wave traffic patterns
type WavePattern struct {
	baseUsers    int     // Average user count
	amplitude    int     // Variation from base (±)
	period       time.Duration // Time for one complete cycle
	duration     time.Duration
	phase        float64 // Phase shift in radians
}

func NewWavePattern(config map[string]interface{}) (Pattern, error) {
	base, _ := config["base_users"].(int)
	amplitude, _ := config["amplitude"].(int)
	period, _ := config["period"].(time.Duration)
	duration, _ := config["duration"].(time.Duration)
	phase, _ := config["phase"].(float64)
	
	if base <= 0 {
		base = 100
	}
	if amplitude <= 0 {
		amplitude = base / 2
	}
	if period <= 0 {
		period = 1 * time.Hour
	}
	if duration <= 0 {
		duration = 24 * time.Hour
	}
	
	p := &WavePattern{
		baseUsers: base,
		amplitude: amplitude,
		period:    period,
		duration:  duration,
		phase:     phase,
	}
	
	return p, p.Validate()
}

func (p *WavePattern) GetUserCount(elapsed time.Duration) int {
	if elapsed > p.duration {
		return 0
	}
	
	// Calculate position in cycle (0 to 2π)
	cyclePosition := (float64(elapsed) / float64(p.period)) * 2 * math.Pi
	
	// Apply phase shift
	cyclePosition += p.phase
	
	// Sine wave: base + amplitude * sin(position)
	users := float64(p.baseUsers) + float64(p.amplitude)*math.Sin(cyclePosition)
	
	// Ensure non-negative
	if users < 0 {
		users = 0
	}
	
	return int(users)
}

func (p *WavePattern) GetDuration() time.Duration {
	return p.duration
}

func (p *WavePattern) Validate() error {
	if p.baseUsers <= 0 {
		return fmt.Errorf("base users must be positive")
	}
	if p.amplitude < 0 {
		return fmt.Errorf("amplitude cannot be negative")
	}
	if p.amplitude > p.baseUsers {
		return fmt.Errorf("amplitude cannot exceed base users (would go negative)")
	}
	if p.period <= 0 {
		return fmt.Errorf("period must be positive")
	}
	return nil
}

func (p *WavePattern) GetType() string {
	return "wave"
}

// BusinessHoursPattern simulates realistic business day traffic
type BusinessHoursPattern struct {
	workdayUsers     int
	afterHoursUsers  int
	peakHour         int // Hour of peak traffic (0-23)
	peakMultiplier   float64
	workStartHour    int
	workEndHour      int
	duration         time.Duration
}

func NewBusinessHoursPattern(config map[string]interface{}) (Pattern, error) {
	workday, _ := config["workday_users"].(int)
	afterHours, _ := config["after_hours_users"].(int)
	peakHour, _ := config["peak_hour"].(int)
	peakMult, _ := config["peak_multiplier"].(float64)
	workStart, _ := config["work_start_hour"].(int)
	workEnd, _ := config["work_end_hour"].(int)
	duration, _ := config["duration"].(time.Duration)
	
	if workday <= 0 {
		workday = 100
	}
	if afterHours <= 0 {
		afterHours = workday / 5
	}
	if peakHour < 0 || peakHour > 23 {
		peakHour = 14 // 2 PM peak
	}
	if peakMult <= 0 {
		peakMult = 1.5
	}
	if workStart < 0 || workStart > 23 {
		workStart = 9
	}
	if workEnd <= workStart || workEnd > 23 {
		workEnd = 17
	}
	if duration <= 0 {
		duration = 7 * 24 * time.Hour // 1 week
	}
	
	p := &BusinessHoursPattern{
		workdayUsers:    workday,
		afterHoursUsers: afterHours,
		peakHour:        peakHour,
		peakMultiplier:  peakMult,
		workStartHour:   workStart,
		workEndHour:     workEnd,
		duration:        duration,
	}
	
	return p, p.Validate()
}

func (p *BusinessHoursPattern) GetUserCount(elapsed time.Duration) int {
	if elapsed > p.duration {
		return 0
	}
	
	// Calculate current hour in simulation (0-23)
	totalHours := elapsed.Hours()
	currentHour := int(totalHours) % 24
	
	// Outside work hours
	if currentHour < p.workStartHour || currentHour >= p.workEndHour {
		return p.afterHoursUsers
	}
	
	// During work hours - use bell curve centered on peak hour
	hoursFromPeak := math.Abs(float64(currentHour - p.peakHour))
	workDayLength := float64(p.workEndHour - p.workStartHour)
	
	// Bell curve factor (1.0 at peak, decreasing as we move away)
	bellCurve := math.Exp(-0.5 * math.Pow(hoursFromPeak/(workDayLength/4), 2))
	
	// Calculate users: base + peak bonus
	users := float64(p.workdayUsers) + float64(p.workdayUsers)*(p.peakMultiplier-1)*bellCurve
	
	return int(users)
}

func (p *BusinessHoursPattern) GetDuration() time.Duration {
	return p.duration
}

func (p *BusinessHoursPattern) Validate() error {
	if p.workdayUsers <= 0 {
		return fmt.Errorf("workday users must be positive")
	}
	if p.afterHoursUsers < 0 {
		return fmt.Errorf("after hours users cannot be negative")
	}
	if p.peakMultiplier < 1.0 {
		return fmt.Errorf("peak multiplier must be >= 1.0")
	}
	if p.workStartHour >= p.workEndHour {
		return fmt.Errorf("work start hour must be before work end hour")
	}
	return nil
}

func (p *BusinessHoursPattern) GetType() string {
	return "business_hours"
}

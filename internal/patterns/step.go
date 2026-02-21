package patterns

import (
	"fmt"
	"time"
)

// StepPattern increases users in discrete steps/tiers
type StepPattern struct {
	steps    []Step
	duration time.Duration
}

type Step struct {
	Duration time.Duration // How long this step lasts
	Users    int           // Target user count for this step
	Ramp     string        // "instant", "linear", or "exponential"
}

func NewStepPattern(config map[string]interface{}) (Pattern, error) {
	stepsData, ok := config["steps"].([]map[string]interface{})
	if !ok || len(stepsData) == 0 {
		return nil, fmt.Errorf("steps configuration required")
	}
	
	duration, _ := config["total_duration"].(time.Duration)
	
	steps := make([]Step, len(stepsData))
	totalDuration := time.Duration(0)
	
	for i, stepData := range stepsData {
		stepDuration, _ := stepData["duration"].(time.Duration)
		users, _ := stepData["users"].(int)
		ramp, _ := stepData["ramp"].(string)
		
		if ramp == "" {
			ramp = "instant"
		}
		
		steps[i] = Step{
			Duration: stepDuration,
			Users:    users,
			Ramp:     ramp,
		}
		totalDuration += stepDuration
	}
	
	if duration == 0 {
		duration = totalDuration
	}
	
	p := &StepPattern{
		steps:    steps,
		duration: duration,
	}
	
	return p, p.Validate()
}

func (p *StepPattern) GetUserCount(elapsed time.Duration) int {
	if elapsed > p.duration {
		return 0
	}
	
	currentTime := time.Duration(0)
	
	for _, step := range p.steps {
		if elapsed < currentTime+step.Duration {
			// We're in this step
			stepElapsed := elapsed - currentTime
			
			switch step.Ramp {
			case "instant":
				return step.Users
			case "linear":
				if stepElapsed <= 0 {
					return 0
				}
				progress := float64(stepElapsed) / float64(step.Duration)
				return int(float64(step.Users) * progress)
			case "exponential":
				if stepElapsed <= 0 {
					return 0
				}
				progress := float64(stepElapsed) / float64(step.Duration)
				return int(float64(step.Users) * (progress * progress))
			default:
				return step.Users
			}
		}
		currentTime += step.Duration
	}
	
	return 0
}

func (p *StepPattern) GetDuration() time.Duration {
	return p.duration
}

func (p *StepPattern) Validate() error {
	if len(p.steps) == 0 {
		return fmt.Errorf("at least one step required")
	}
	
	for i, step := range p.steps {
		if step.Users < 0 {
			return fmt.Errorf("step %d: users cannot be negative", i)
		}
		if step.Duration <= 0 {
			return fmt.Errorf("step %d: duration must be positive", i)
		}
		if step.Ramp != "instant" && step.Ramp != "linear" && step.Ramp != "exponential" {
			return fmt.Errorf("step %d: invalid ramp type '%s'", i, step.Ramp)
		}
	}
	
	return nil
}

func (p *StepPattern) GetType() string {
	return "step"
}

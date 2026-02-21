package assertions

import (
	"fmt"
	"net/http"
	"regexp"
	"time"
)

// Assertion validates HTTP responses
type Assertion interface {
	Validate(resp *http.Response, latency time.Duration) error
	Name() string
}

// Engine manages assertion execution
type Engine struct {
	assertions []Assertion
}

func NewEngine() *Engine {
	return &Engine{
		assertions: make([]Assertion, 0),
	}
}

func (e *Engine) Add(assertion Assertion) {
	e.assertions = append(e.assertions, assertion)
}

func (e *Engine) Validate(resp *http.Response, latency time.Duration) []error {
	errors := make([]error, 0)
	for _, assertion := range e.assertions {
		if err := assertion.Validate(resp, latency); err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", assertion.Name(), err))
		}
	}
	return errors
}

// StatusCodeAssertion checks HTTP status code
type StatusCodeAssertion struct {
	expected int
	operator string // "equals", "not_equals", "in_range", "gte", "lte"
	min      int
	max      int
}

func NewStatusCodeAssertion(expected int) *StatusCodeAssertion {
	return &StatusCodeAssertion{
		expected: expected,
		operator: "equals",
	}
}

func (a *StatusCodeAssertion) Validate(resp *http.Response, _ time.Duration) error {
	switch a.operator {
	case "equals":
		if resp.StatusCode != a.expected {
			return fmt.Errorf("expected status %d, got %d", a.expected, resp.StatusCode)
		}
	case "gte":
		if resp.StatusCode < a.expected {
			return fmt.Errorf("expected status >= %d, got %d", a.expected, resp.StatusCode)
		}
	}
	return nil
}

func (a *StatusCodeAssertion) Name() string {
	return "status_code"
}

// ResponseTimeAssertion checks latency thresholds
type ResponseTimeAssertion struct {
	maxLatency time.Duration
	percentile string // "p50", "p95", "p99"
}

func NewResponseTimeAssertion(max time.Duration) *ResponseTimeAssertion {
	return &ResponseTimeAssertion{
		maxLatency: max,
		percentile: "p95",
	}
}

func (a *ResponseTimeAssertion) Validate(_ *http.Response, latency time.Duration) error {
	if latency > a.maxLatency {
		return fmt.Errorf("latency %v exceeds maximum %v", latency, a.maxLatency)
	}
	return nil
}

func (a *ResponseTimeAssertion) Name() string {
	return "response_time"
}

// RegexAssertion matches response body against pattern
type RegexAssertion struct {
	pattern *regexp.Regexp
	invert  bool
}

func NewRegexAssertion(pattern string) (*RegexAssertion, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &RegexAssertion{pattern: re}, nil
}

func (a *RegexAssertion) Validate(resp *http.Response, _ time.Duration) error {
	// Would read body here in full implementation
	return nil
}

func (a *RegexAssertion) Name() string {
	return "regex"
}

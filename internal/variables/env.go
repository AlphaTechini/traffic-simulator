package variables

import (
	"fmt"
	"os"
	"regexp"
	"sync"
)

// EnvSubstitutor replaces {{env.VAR_NAME}} patterns with environment values
type EnvSubstitutor struct {
	cache      map[string]string
	cacheMu    sync.RWMutex
	prefix     string
	suffix     string
	required   []string // Required env vars (fail if missing)
}

var envPattern = regexp.MustCompile(`\{\{env\.([a-zA-Z_][a-zA-Z0-9_]*)\}\}`)

func NewEnvSubstitutor() *EnvSubstitutor {
	return &EnvSubstitutor{
		cache:    make(map[string]string),
		prefix:   "{{env.",
		suffix:   "}}",
		required: make([]string, 0),
	}
}

// Substitute replaces all {{env.VAR}} patterns in template
func (e *EnvSubstitutor) Substitute(template string) (string, error) {
	result := envPattern.ReplaceAllStringFunc(template, func(match string) string {
		// Extract variable name from {{env.VAR_NAME}}
		matches := envPattern.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match // Keep original if pattern doesn't match
		}
		
		varName := matches[1]
		value, err := e.get(varName)
		
		if err != nil {
			return fmt.Sprintf("{{ERROR: %v}}", err)
		}
		
		return value
	})
	
	return result, nil
}

// MustSubstitute substitutes or panics on error
func (e *EnvSubstitutor) MustSubstitute(template string) string {
	result, err := e.Substitute(template)
	if err != nil {
		panic(err)
	}
	return result
}

func (e *EnvSubstitutor) get(varName string) (string, error) {
	// Check cache first
	e.cacheMu.RLock()
	if value, exists := e.cache[varName]; exists {
		e.cacheMu.RUnlock()
		return value, nil
	}
	e.cacheMu.RUnlock()
	
	// Get from environment
	value, exists := os.LookupEnv(varName)
	if !exists {
		// Check if required
		for _, req := range e.required {
			if req == varName {
				return "", fmt.Errorf("required environment variable %s not set", varName)
			}
		}
		// Not required, return empty
		value = ""
	}
	
	// Cache the value
	e.cacheMu.Lock()
	e.cache[varName] = value
	e.cacheMu.Unlock()
	
	return value, nil
}

// Require marks variables as required (will error if not set)
func (e *EnvSubstitutor) Require(varNames ...string) {
	e.required = append(e.required, varNames...)
}

// Set manually sets a variable in cache (for testing)
func (e *EnvSubstitutor) Set(varName, value string) {
	e.cacheMu.Lock()
	defer e.cacheMu.Unlock()
	e.cache[varName] = value
}

// ClearCache clears the environment variable cache
func (e *EnvSubstitutor) ClearCache() {
	e.cacheMu.Lock()
	defer e.cacheMu.Unlock()
	e.cache = make(map[string]string)
}

// Validate checks if all required variables are set
func (e *EnvSubstitutor) Validate() error {
	for _, varName := range e.required {
		if _, exists := os.LookupEnv(varName); !exists {
			return fmt.Errorf("required environment variable %s not set", varName)
		}
	}
	return nil
}

// ListRequired returns list of required but unset variables
func (e *EnvSubstitutor) ListMissing() []string {
	missing := make([]string, 0)
	for _, varName := range e.required {
		if _, exists := os.LookupEnv(varName); !exists {
			missing = append(missing, varName)
		}
	}
	return missing
}

// FindAll extracts all {{env.VAR}} references from template
func (e *EnvSubstitutor) FindAll(template string) []string {
	matches := envPattern.FindAllStringSubmatch(template, -1)
	
	vars := make([]string, len(matches))
	for i, match := range matches {
		if len(match) >= 2 {
			vars[i] = match[1]
		}
	}
	
	return vars
}

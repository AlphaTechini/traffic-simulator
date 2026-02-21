package scanner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GraphQLSchema represents a discovered GraphQL schema
type GraphQLSchema struct {
	Types         map[string]GraphQLType `json:"types"`
	Queries       []string               `json:"queries"`
	Mutations     []string               `json:"mutations"`
	Subscriptions []string               `json:"subscriptions"`
}

// GraphQLType represents a GraphQL type definition
type GraphQLType struct {
	Name        string   `json:"name"`
	Fields      []string `json:"fields"`
	Description string   `json:"description"`
	Kind        string   `json:"kind"` // OBJECT, INPUT_OBJECT, SCALAR, etc.
}

// GraphQLIntrospectionQuery is the standard introspection query
const GraphQLIntrospectionQuery = `
query IntrospectionQuery {
  __schema {
    queryType { name }
    mutationType { name }
    subscriptionType { name }
    types {
      ...FullType
    }
    directives {
      name
      description
      locations
      args {
        ...InputValue
      }
    }
  }
}

fragment FullType on __Type {
  kind
  name
  description
  fields(includeDeprecated: true) {
    name
    description
    args {
      ...InputValue
    }
    type {
      ...TypeRef
    }
    isDeprecated
    deprecationReason
  }
  inputFields {
    ...InputValue
  }
  interfaces {
    ...TypeRef
  }
  enumValues(includeDeprecated: true) {
    name
    description
    isDeprecated
    deprecationReason
  }
  possibleTypes {
    ...TypeRef
  }
}

fragment InputValue on __InputValue {
  name
  description
  type {
    ...TypeRef
  }
  defaultValue
}

fragment TypeRef on __Type {
  kind
  name
  ofType {
    kind
    name
    ofType {
      kind
      name
      ofType {
        kind
        name
        ofType {
          kind
          name
          ofType {
            kind
            name
            ofType {
              kind
              name
              ofType {
                kind
                name
              }
            }
          }
        }
      }
    }
  }
}
`

// GraphQLScanner discovers GraphQL endpoints and schemas
type GraphQLScanner struct {
	config  ScannerConfig
	client  *http.Client
	schema  *GraphQLSchema
}

// NewGraphQLScanner creates a new GraphQL scanner
func NewGraphQLScanner(config ScannerConfig) *GraphQLScanner {
	return &GraphQLScanner{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Scan attempts to discover GraphQL endpoints and schema
func (gs *GraphQLScanner) Scan(ctx context.Context) (*GraphQLSchema, error) {
	fmt.Printf("🔍 Scanning for GraphQL endpoint...\n")

	// Step 1: Try common GraphQL endpoints
	endpoints := []string{
		"/graphql",
		"/api/graphql",
		"/v1/graphql",
		"/graph",
		"/api/graph",
	}

	for _, endpoint := range endpoints {
		url := gs.config.BaseURL + endpoint
		if gs.testGraphQLEndpoint(ctx, url) {
			fmt.Printf("   Found GraphQL endpoint at: %s\n", endpoint)
			
			// Step 2: Fetch schema via introspection
			if gs.config.GraphQL.Introspection {
				schema, err := gs.fetchSchema(ctx, url)
				if err == nil {
					gs.schema = schema
					fmt.Printf("   Discovered %d queries, %d mutations\n", 
						len(schema.Queries), len(schema.Mutations))
					return schema, nil
				}
			}
			
			// Return basic schema even without introspection
			gs.schema = &GraphQLSchema{}
			return gs.schema, nil
		}
	}

	return nil, fmt.Errorf("no GraphQL endpoint found")
}

// testGraphQLEndpoint checks if a URL is a valid GraphQL endpoint
func (gs *GraphQLScanner) testGraphQLEndpoint(ctx context.Context, url string) bool {
	// Send a simple introspection-like query
	query := `query { __typename }`
	
	body := map[string]string{
		"query": query,
	}
	
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	
	if gs.config.AuthHeader != "" {
		req.Header.Set("Authorization", gs.config.AuthHeader)
	}
	
	resp, err := gs.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	
	// Check for successful response
	if resp.StatusCode == 200 {
		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		if err := json.Unmarshal(respBody, &result); err == nil {
			if data, ok := result["data"].(map[string]interface{}); ok {
				if _, exists := data["__typename"]; exists {
					return true
				}
			}
		}
	}
	
	return false
}

// fetchSchema retrieves the full GraphQL schema via introspection
func (gs *GraphQLScanner) fetchSchema(ctx context.Context, endpoint string) (*GraphQLSchema, error) {
	body := map[string]string{
		"query": GraphQLIntrospectionQuery,
	}
	
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	
	if gs.config.AuthHeader != "" {
		req.Header.Set("Authorization", gs.config.AuthHeader)
	}
	
	resp, err := gs.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("introspection request failed: %w", err)
	}
	defer resp.Body.Close()
	
	respBody, _ := io.ReadAll(resp.Body)
	
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse introspection response: %w", err)
	}
	
	if errors, ok := result["errors"].([]interface{}); ok && len(errors) > 0 {
		return nil, fmt.Errorf("introspection errors: %v", errors)
	}
	
	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid introspection response format")
	}
	
	schemaData, ok := data["__schema"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing __schema in response")
	}
	
	return gs.parseSchema(schemaData)
}

// parseSchema extracts queries, mutations, and types from introspection result
func (gs *GraphQLScanner) parseSchema(schemaData map[string]interface{}) (*GraphQLSchema, error) {
	schema := &GraphQLSchema{
		Types:         make(map[string]GraphQLType),
		Queries:       make([]string, 0),
		Mutations:     make([]string, 0),
		Subscriptions: make([]string, 0),
	}
	
	// Get query type name
	queryTypeName := "Query"
	if queryType, ok := schemaData["queryType"].(map[string]interface{}); ok {
		if name, ok := queryType["name"].(string); ok {
			queryTypeName = name
		}
	}
	
	// Get mutation type name
	mutationTypeName := "Mutation"
	if mutationType, ok := schemaData["mutationType"].(map[string]interface{}); ok {
		if name, ok := mutationType["name"].(string); ok {
			mutationTypeName = name
		}
	}
	
	// Get subscription type name
	subscriptionTypeName := "Subscription"
	if subscriptionType, ok := schemaData["subscriptionType"].(map[string]interface{}); ok {
		if name, ok := subscriptionType["name"].(string); ok {
			subscriptionTypeName = name
		}
	}
	
	// Parse types
	if types, ok := schemaData["types"].([]interface{}); ok {
		for _, t := range types {
			typeData, ok := t.(map[string]interface{})
			if !ok {
				continue
			}
			
			name, _ := typeData["name"].(string)
			kind, _ := typeData["kind"].(string)
			description, _ := typeData["description"].(string)
			
			graphQLType := GraphQLType{
				Name:        name,
				Kind:        kind,
				Description: description,
				Fields:      make([]string, 0),
			}
			
			// Extract fields
			if fields, ok := typeData["fields"].([]interface{}); ok {
				for _, f := range fields {
					if fieldData, ok := f.(map[string]interface{}); ok {
						if fieldName, ok := fieldData["name"].(string); ok {
							graphQLType.Fields = append(graphQLType.Fields, fieldName)
							
							// Add to queries/mutations/subscriptions lists
							if name == queryTypeName {
								schema.Queries = append(schema.Queries, fieldName)
							} else if name == mutationTypeName {
								schema.Mutations = append(schema.Mutations, fieldName)
							} else if name == subscriptionTypeName {
								schema.Subscriptions = append(schema.Subscriptions, fieldName)
							}
						}
					}
				}
			}
			
			schema.Types[name] = graphQLType
		}
	}
	
	return schema, nil
}

// GenerateGraphQLActions creates user actions from discovered GraphQL schema
func (gs *GraphQLScanner) GenerateGraphQLActions() []UserAction {
	if gs.schema == nil {
		return nil
	}
	
	actions := make([]UserAction, 0)
	
	// Create query action
	if len(gs.schema.Queries) > 0 {
		endpoints := make([]Endpoint, 0)
		for range gs.schema.Queries {
			endpoint := Endpoint{
				Method:     "POST",
				Path:       gs.config.GraphQL.Endpoint,
				Weight:     10,
				MinDelayMs: 50,
				MaxDelayMs: 500,
				ErrorRate:  0.02,
			}
			endpoints = append(endpoints, endpoint)
		}
		
		actions = append(actions, UserAction{
			Name:        "GraphQL Queries",
			Endpoints:   endpoints,
			ThinkTimeMs: 500,
		})
	}
	
	// Create mutation action
	if len(gs.schema.Mutations) > 0 {
		endpoints := make([]Endpoint, 0)
		for range gs.schema.Mutations {
			endpoint := Endpoint{
				Method:     "POST",
				Path:       gs.config.GraphQL.Endpoint,
				Weight:     5, // Lower weight for mutations (they're expensive)
				MinDelayMs: 100,
				MaxDelayMs: 1000,
				ErrorRate:  0.05,
			}
			endpoints = append(endpoints, endpoint)
		}
		
		actions = append(actions, UserAction{
			Name:        "GraphQL Mutations",
			Endpoints:   endpoints,
			ThinkTimeMs: 1000,
		})
	}
	
	return actions
}

// IsGraphQLAvailable returns true if GraphQL was detected
func (gs *GraphQLScanner) IsGraphQLAvailable() bool {
	return gs.schema != nil
}

// GetSchema returns the discovered GraphQL schema
func (gs *GraphQLScanner) GetSchema() *GraphQLSchema {
	return gs.schema
}

// BuildGraphQLQuery creates a GraphQL query string
func BuildGraphQLQuery(operation string, fields []string) string {
	var sb strings.Builder
	
	sb.WriteString(operation)
	sb.WriteString(" {\n")
	
	for _, field := range fields {
		sb.WriteString("  ")
		sb.WriteString(field)
		sb.WriteString("\n")
	}
	
	sb.WriteString("}")
	
	return sb.String()
}

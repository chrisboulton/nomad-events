package routing

import (
	"fmt"

	"nomad-events/internal/config"
	"nomad-events/internal/nomad"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
)

type Router struct {
	routes []routeNode
}

type routeNode struct {
	filter         cel.Program
	output         string      // empty if no output
	shouldContinue bool        // true by default
	children       []routeNode // child routes
}

func NewRouter(routes []config.Route) (*Router, error) {
	env, err := cel.NewEnv(
		cel.Variable("event", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("diff", cel.MapType(cel.StringType, cel.DynType)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	routeNodes, err := buildRouteNodes(routes, env)
	if err != nil {
		return nil, err
	}

	return &Router{routes: routeNodes}, nil
}

// buildRouteNodes recursively builds route nodes from config routes
func buildRouteNodes(routes []config.Route, env *cel.Env) ([]routeNode, error) {
	nodes := make([]routeNode, len(routes))

	for i, route := range routes {
		var program cel.Program
		var err error

		// Compile filter
		if route.Filter == "" {
			ast, issues := env.Compile("true")
			if issues.Err() != nil {
				return nil, fmt.Errorf("failed to compile empty filter for route %d: %w", i, issues.Err())
			}
			program, err = env.Program(ast)
			if err != nil {
				return nil, fmt.Errorf("failed to create program for empty filter for route %d: %w", i, err)
			}
		} else {
			ast, issues := env.Compile(route.Filter)
			if issues.Err() != nil {
				return nil, fmt.Errorf("failed to compile filter for route %d: %w", i, issues.Err())
			}

			program, err = env.Program(ast)
			if err != nil {
				return nil, fmt.Errorf("failed to create program for route %d: %w", i, err)
			}
		}

		// Set continue flag (defaults to true)
		continueFlag := true
		if route.Continue != nil {
			continueFlag = *route.Continue
		}

		// Build child routes
		children, err := buildRouteNodes(route.Routes, env)
		if err != nil {
			return nil, fmt.Errorf("failed to build child routes for route %d: %w", i, err)
		}

		nodes[i] = routeNode{
			filter:         program,
			output:         route.Output,
			shouldContinue: continueFlag,
			children:       children,
		}
	}

	return nodes, nil
}

func (r *Router) Route(event nomad.Event) ([]string, error) {
	eventMap := map[string]interface{}{
		"Topic":     event.Topic,
		"Type":      event.Type,
		"Key":       event.Key,
		"Namespace": event.Namespace,
		"Index":     event.Index,
		"Payload":   event.Payload,
		"Diff":      event.Diff,
	}

	evalContext := map[string]interface{}{
		"event": eventMap,
		"diff":  event.Diff,
	}

	return r.processRoutes(r.routes, evalContext)
}

// processRoutes recursively processes routes and returns matched outputs
func (r *Router) processRoutes(routes []routeNode, evalContext map[string]interface{}) ([]string, error) {
	var matchedOutputs []string

	for _, route := range routes {
		// Evaluate filter
		result, _, err := route.filter.Eval(evalContext)
		if err != nil {
			// Continue to next route if filter evaluation fails
			continue
		}

		if result == types.True {
			// Route matched - add output if specified
			if route.output != "" {
				matchedOutputs = append(matchedOutputs, route.output)
			}

			// Process child routes
			if len(route.children) > 0 {
				childOutputs, err := r.processRoutes(route.children, evalContext)
				if err != nil {
					// Don't fail entire routing if child route fails
					continue
				}
				matchedOutputs = append(matchedOutputs, childOutputs...)
			}

			// If continue is false, stop processing siblings
			if !route.shouldContinue {
				break
			}
		}
	}

	return matchedOutputs, nil
}

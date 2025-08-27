package routing

import (
	"fmt"

	"nomad-events/internal/config"
	"nomad-events/internal/nomad"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
)

type Router struct {
	rules []rule
}

type rule struct {
	filter         cel.Program
	outputs        []string
	stopProcessing bool
}

func NewRouter(routes []config.Route) (*Router, error) {
	env, err := cel.NewEnv(
		cel.Variable("event", cel.MapType(cel.StringType, cel.DynType)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	rules := make([]rule, 0, len(routes))

	for i, route := range routes {
		var program cel.Program

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

		rules = append(rules, rule{
			filter:         program,
			outputs:        []string{route.Output},
			stopProcessing: route.StopProcessing,
		})
	}

	return &Router{rules: rules}, nil
}

func (r *Router) Route(event nomad.Event) ([]string, error) {
	var matchedOutputs []string

	eventMap := map[string]interface{}{
		"Topic":     event.Topic,
		"Type":      event.Type,
		"Key":       event.Key,
		"Namespace": event.Namespace,
		"Index":     event.Index,
		"Payload":   event.Payload,
	}

	for _, rule := range r.rules {
		result, _, err := rule.filter.Eval(map[string]interface{}{
			"event": eventMap,
		})
		if err != nil {
			continue
		}

		if result == types.True {
			matchedOutputs = append(matchedOutputs, rule.outputs...)
			if rule.stopProcessing {
				break
			}
		}
	}

	return matchedOutputs, nil
}

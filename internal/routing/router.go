package routing

import (
	"encoding/json"
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"nomad-events/internal/config"
	"nomad-events/internal/nomad"
)

type Router struct {
	rules []rule
}

type rule struct {
	filter  cel.Program
	outputs []string
}

func NewRouter(routes []config.Route) (*Router, error) {
	env, err := cel.NewEnv(
		cel.Variable("event", cel.ObjectType("nomad.Event")),
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
			filter:  program,
			outputs: []string{route.Output},
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
		"Payload":   parsePayload(event.Payload),
	}

	for _, rule := range r.rules {
		result, _, err := rule.filter.Eval(map[string]interface{}{
			"event": eventMap,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate filter: %w", err)
		}

		if result == types.True {
			matchedOutputs = append(matchedOutputs, rule.outputs...)
		}
	}

	return matchedOutputs, nil
}

func parsePayload(payload []byte) interface{} {
	if len(payload) == 0 {
		return nil
	}

	var result interface{}
	if err := json.Unmarshal(payload, &result); err != nil {
		return string(payload)
	}
	return result
}
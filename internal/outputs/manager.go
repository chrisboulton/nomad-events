package outputs

import (
	"fmt"
	"time"

	"nomad-events/internal/config"
	"nomad-events/internal/nomad"

	"github.com/hashicorp/nomad/api"
)

type Output interface {
	Send(event nomad.Event) error
}

type Manager struct {
	outputs     map[string]Output
	nomadClient *api.Client
}

func NewManager(outputConfigs map[string]config.Output, nomadClient *api.Client) (*Manager, error) {
	outputs := make(map[string]Output)

	for name, cfg := range outputConfigs {
		output, err := createOutput(cfg, nomadClient)
		if err != nil {
			return nil, fmt.Errorf("failed to create output %q: %w", name, err)
		}
		outputs[name] = output
	}

	return &Manager{outputs: outputs, nomadClient: nomadClient}, nil
}

func createOutput(cfg config.Output, nomadClient *api.Client) (Output, error) {
	var baseOutput Output
	var err error

	switch cfg.Type {
	case "stdout":
		baseOutput, err = NewStdoutOutput(cfg.Properties, nomadClient)
	case "slack":
		baseOutput, err = NewSlackOutput(cfg.Properties, nomadClient)
	case "http":
		baseOutput, err = NewHTTPOutput(cfg.Properties)
	case "rabbitmq":
		baseOutput, err = NewRabbitMQOutput(cfg.Properties)
	case "exec":
		baseOutput, err = NewExecOutput(cfg.Properties)
	default:
		return nil, fmt.Errorf("unsupported output type: %q", cfg.Type)
	}

	if err != nil {
		return nil, err
	}

	// Wrap with retry logic if configured
	if cfg.Retry != nil {
		retryConfig := RetryConfig{
			MaxRetries: cfg.Retry.MaxRetries,
		}

		if cfg.Retry.BaseDelay != "" {
			delay, err := time.ParseDuration(cfg.Retry.BaseDelay)
			if err != nil {
				return nil, fmt.Errorf("invalid base_delay format: %w. use a duration like \"1s\", \"500ms\", \"2m\"", err)
			}
			retryConfig.BaseDelay = delay
		}

		return NewRetryOutput(baseOutput, retryConfig), nil
	}

	return baseOutput, nil
}

func (m *Manager) Send(outputName string, event nomad.Event) error {
	output, exists := m.outputs[outputName]
	if !exists {
		return fmt.Errorf("output %q not found", outputName)
	}

	return output.Send(event)
}

func (m *Manager) GetOutput(name string) (Output, bool) {
	output, exists := m.outputs[name]
	return output, exists
}

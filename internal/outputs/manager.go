package outputs

import (
	"fmt"

	"nomad-events/internal/config"
	"nomad-events/internal/nomad"
)

type Output interface {
	Send(event nomad.Event) error
}

type Manager struct {
	outputs map[string]Output
}

func NewManager(outputConfigs map[string]config.Output) (*Manager, error) {
	outputs := make(map[string]Output)

	for name, cfg := range outputConfigs {
		output, err := createOutput(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create output %q: %w", name, err)
		}
		outputs[name] = output
	}

	return &Manager{outputs: outputs}, nil
}

func createOutput(cfg config.Output) (Output, error) {
	switch cfg.Type {
	case "stdout":
		return NewStdoutOutput(cfg.Properties)
	case "slack":
		return NewSlackOutput(cfg.Properties)
	case "http":
		return NewHTTPOutput(cfg.Properties)
	case "rabbitmq":
		return NewRabbitMQOutput(cfg.Properties)
	case "exec":
		return NewExecOutput(cfg.Properties)
	default:
		return nil, fmt.Errorf("unsupported output type: %s", cfg.Type)
	}
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
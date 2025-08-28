package outputs

import (
	"strings"
	"testing"

	"nomad-events/internal/nomad"
	"nomad-events/internal/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRabbitMQTemplating(t *testing.T) {
	
	tests := []struct {
		name            string
		config          map[string]interface{}
		event           nomad.Event
		expectedRouting string
	}{
		{
			name: "basic routing key templating",
			config: map[string]interface{}{
				"url":         "amqp://fake-url", // Won't actually connect in test
				"exchange":    "nomad",
				"queue":       "events",
				"routing_key": "{{ .Topic }}.{{ .Type }}",
			},
			event: nomad.Event{
				Topic: "Job",
				Type:  "JobRegistered",
				Key:   "test-job",
				Index: 1,
				Payload: map[string]interface{}{
					"Job": map[string]interface{}{
						"ID": "my-job",
					},
				},
			},
			expectedRouting: "Job.JobRegistered",
		},
		{
			name: "default routing key",
			config: map[string]interface{}{
				"url": "amqp://fake-url",
			},
			event: nomad.Event{
				Topic: "Node",
				Type:  "NodeRegistration",
			},
			expectedRouting: "nomad.Node.NodeRegistration",
		},
		{
			name: "whitespace trimming in routing key",
			config: map[string]interface{}{
				"url":         "amqp://fake-url",
				"routing_key": " {{ .Topic }}.{{ .Type }} ",
			},
			event: nomad.Event{
				Topic: "Job",
				Type:  "JobRegistered",
			},
			expectedRouting: "Job.JobRegistered",
		},
		{
			name: "payload templating in routing key",
			config: map[string]interface{}{
				"url":         "amqp://fake-url",
				"exchange":    "jobs",
				"queue":       "job-events",
				"routing_key": "job.{{ .Payload.Job.ID }}.{{ .Type }}",
			},
			event: nomad.Event{
				Topic: "Job",
				Type:  "JobRegistered",
				Payload: map[string]interface{}{
					"Job": map[string]interface{}{
						"ID": "my-app",
					},
				},
			},
			expectedRouting: "job.my-app.JobRegistered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create the output (but don't actually connect to RabbitMQ)
			output := &RabbitMQOutput{
				exchange:          getStringFromConfig(tt.config, "exchange"),
				queue:             getStringFromConfig(tt.config, "queue"),
				routingKeyTemplate: getStringFromConfig(tt.config, "routing_key"),
			}
			
			// Set default routing key if empty
			if output.routingKeyTemplate == "" {
				output.routingKeyTemplate = "nomad.{{ .Topic }}.{{ .Type }}"
			}
			
			// Create template engine
			output.templateEngine = template.NewEngine()
			
			// Test routing key template processing
			routingKey, err := output.processTemplate(output.routingKeyTemplate, tt.event)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedRouting, strings.TrimSpace(routingKey))
		})
	}
}

func TestRabbitMQDefaultRoutingKey(t *testing.T) {
	// This tests the default routing key template
	output := &RabbitMQOutput{
		routingKeyTemplate: "nomad.{{ .Topic }}.{{ .Type }}",
		templateEngine:     template.NewEngine(),
	}

	event := nomad.Event{
		Topic: "Job",
		Type:  "JobRegistered",
	}

	routingKey, err := output.processTemplate(output.routingKeyTemplate, event)
	require.NoError(t, err)
	assert.Equal(t, "nomad.Job.JobRegistered", routingKey)
}

// Helper function to safely get string from config map
func getStringFromConfig(config map[string]interface{}, key string) string {
	if val, ok := config[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}
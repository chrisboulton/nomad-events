package outputs

import (
	"testing"

	"nomad-events/internal/nomad"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSlackOutputWithBlocks(t *testing.T) {
	config := map[string]interface{}{
		"webhook_url": "https://hooks.slack.com/services/test",
		"channel":     "#test",
		"blocks": []interface{}{
			map[string]interface{}{
				"type": "header",
				"text": "Test Header {{ .Topic }}",
			},
			map[string]interface{}{
				"type": "section",
				"text": map[string]interface{}{
					"type": "mrkdwn",
					"text": "*Event:* {{ .Type }}",
				},
			},
		},
	}

	output, err := NewSlackOutput(config)
	require.NoError(t, err)
	assert.NotNil(t, output.templateEngine)
	assert.Len(t, output.blockConfigs, 2)
	assert.Equal(t, "header", output.blockConfigs[0].Type)
	assert.Equal(t, "section", output.blockConfigs[1].Type)
}

func TestNewSlackOutputWithoutBlocks(t *testing.T) {
	config := map[string]interface{}{
		"webhook_url": "https://hooks.slack.com/services/test",
		"channel":     "#test",
	}

	output, err := NewSlackOutput(config)
	require.NoError(t, err)
	assert.Nil(t, output.templateEngine)
	assert.Empty(t, output.blockConfigs)
}

func TestSlackOutputFormatEventWithBlocks(t *testing.T) {
	config := map[string]interface{}{
		"webhook_url": "https://hooks.slack.com/services/test",
		"channel":     "#deployments",
		"blocks": []interface{}{
			map[string]interface{}{
				"type": "header",
				"text": "Deployment: {{ .Payload.DeploymentID }}",
			},
			map[string]interface{}{
				"type": "section",
				"text": map[string]interface{}{
					"type": "mrkdwn",
					"text": "*Status:* {{ .Payload.Status }}\n*Node:* {{ .Payload.Node }}",
				},
			},
		},
	}

	output, err := NewSlackOutput(config)
	require.NoError(t, err)

	event := nomad.Event{
		Topic: "Deployment",
		Type:  "DeploymentStatusUpdate",
		Index: 12345,
		Payload: mustMarshalJSON(map[string]interface{}{
			"DeploymentID": "deploy-123",
			"Status":       "successful",
			"Node":         "worker-1",
		}),
	}

	message, err := output.formatEventWithBlocks(event)
	require.NoError(t, err)
	assert.Equal(t, "#deployments", message.Channel)
	assert.NotNil(t, message.Blocks)
}

func TestSlackOutputBlockConfigParsing(t *testing.T) {
	tests := []struct {
		name           string
		blockConfig    map[string]interface{}
		expectedType   string
		expectedFields []string
	}{
		{
			name: "header block",
			blockConfig: map[string]interface{}{
				"type": "header",
				"text": "Header Text",
			},
			expectedType: "header",
		},
		{
			name: "section with fields",
			blockConfig: map[string]interface{}{
				"type": "section",
				"text": "Section text",
				"fields": []interface{}{
					"field1", "field2",
				},
			},
			expectedType: "section",
		},
		{
			name: "actions with elements",
			blockConfig: map[string]interface{}{
				"type": "actions",
				"elements": []interface{}{
					map[string]interface{}{
						"type": "button",
						"text": "Click me",
					},
				},
			},
			expectedType: "actions",
		},
		{
			name: "range block",
			blockConfig: map[string]interface{}{
				"type":  "context",
				"range": ".Payload.Items",
				"elements": []interface{}{
					map[string]interface{}{
						"type": "mrkdwn",
						"text": "Item: {{ .Name }}",
					},
				},
			},
			expectedType: "context",
		},
		{
			name: "image block",
			blockConfig: map[string]interface{}{
				"type":      "image",
				"image_url": "https://example.com/image.png",
				"alt_text":  "Example image",
				"title":     "Image Title",
			},
			expectedType: "image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := map[string]interface{}{
				"webhook_url": "https://hooks.slack.com/services/test",
				"blocks":      []interface{}{tt.blockConfig},
			}

			output, err := NewSlackOutput(config)
			require.NoError(t, err)
			require.Len(t, output.blockConfigs, 1)

			blockConfig := output.blockConfigs[0]
			assert.Equal(t, tt.expectedType, blockConfig.Type)

			// Note: Range field is handled separately by the template engine
			// and is not part of the BlockConfig struct
		})
	}
}

func TestSlackOutputFallbackToBehavior(t *testing.T) {
	config := map[string]interface{}{
		"webhook_url": "https://hooks.slack.com/services/test",
		"channel":     "#test",
		"blocks": []interface{}{
			map[string]interface{}{
				"type": "invalid_block_type",
				"text": "This should cause an error",
			},
		},
	}

	output, err := NewSlackOutput(config)
	require.NoError(t, err)

	event := nomad.Event{
		Topic: "Node",
		Type:  "NodeRegistration",
		Index: 123,
	}

	message, err := output.formatEventWithBlocks(event)
	assert.Error(t, err)
	assert.Empty(t, message.Blocks)

	fallbackMessage := output.formatEvent(event)
	assert.Equal(t, "Nomad Event: Node/NodeRegistration", fallbackMessage.Text)
	assert.NotEmpty(t, fallbackMessage.Attachments)
}

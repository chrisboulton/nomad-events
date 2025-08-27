package outputs

import (
	"testing"

	"nomad-events/internal/nomad"

	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlackOutputWithConditionalBlocks(t *testing.T) {
	config := map[string]interface{}{
		"webhook_url": "https://hooks.slack.com/services/test",
		"channel":     "#test",
		"blocks": []interface{}{
			map[string]interface{}{
				"condition": "event.Topic == 'Deployment'",
				"type":      "header",
				"text":      "🚀 Deployment Event",
			},
			map[string]interface{}{
				"condition": "event.Topic == 'Node'",
				"type":      "header",
				"text":      "📡 Node Event",
			},
			map[string]interface{}{
				"type": "section",
				"text": map[string]interface{}{
					"type": "mrkdwn",
					"text": "Event: {{ .Type }}",
				},
			},
		},
	}

	output, err := NewSlackOutput(config, nil)
	require.NoError(t, err)

	// Test with Deployment event - should include deployment header
	deploymentEvent := nomad.Event{
		Topic: "Deployment",
		Type:  "DeploymentStatusUpdate",
		Index: 123,
		Payload: map[string]interface{}{
			"DeploymentID": "deploy-123",
		},
	}

	message, err := output.formatEvent(deploymentEvent)
	require.NoError(t, err)

	blocks, ok := message.Blocks.([]slack.Block)
	require.True(t, ok)
	require.Len(t, blocks, 2) // Deployment header + section

	headerBlock, ok := blocks[0].(*slack.HeaderBlock)
	require.True(t, ok)
	assert.Contains(t, headerBlock.Text.Text, "🚀 Deployment Event")

	// Test with Node event - should include node header
	nodeEvent := nomad.Event{
		Topic: "Node",
		Type:  "NodeRegistration",
		Index: 124,
		Payload: map[string]interface{}{
			"Node": map[string]interface{}{
				"Name": "worker-1",
			},
		},
	}

	message, err = output.formatEvent(nodeEvent)
	require.NoError(t, err)

	blocks, ok = message.Blocks.([]slack.Block)
	require.True(t, ok)
	require.Len(t, blocks, 2) // Node header + section

	headerBlock, ok = blocks[0].(*slack.HeaderBlock)
	require.True(t, ok)
	assert.Contains(t, headerBlock.Text.Text, "📡 Node Event")

	// Test with Job event - should only include section (no conditional headers)
	jobEvent := nomad.Event{
		Topic: "Job",
		Type:  "JobRegistered",
		Index: 125,
	}

	message, err = output.formatEvent(jobEvent)
	require.NoError(t, err)

	blocks, ok = message.Blocks.([]slack.Block)
	require.True(t, ok)
	require.Len(t, blocks, 1) // Only section block
}

func TestSlackOutputWithConditionalFields(t *testing.T) {
	config := map[string]interface{}{
		"webhook_url": "https://hooks.slack.com/services/test",
		"channel":     "#test",
		"blocks": []interface{}{
			map[string]interface{}{
				"type": "section",
				"text": map[string]interface{}{
					"type": "mrkdwn",
					"text": "Event Details",
				},
				"fields": []interface{}{
					map[string]interface{}{
						"type": "mrkdwn",
						"text": "*Topic:*",
					},
					map[string]interface{}{
						"type": "plain_text",
						"text": "{{ .Topic }}",
					},
					map[string]interface{}{
						"condition": "has(event.Payload.StartTime)",
						"type":      "mrkdwn",
						"text":      "*Started:*",
					},
					map[string]interface{}{
						"condition": "has(event.Payload.StartTime)",
						"type":      "plain_text",
						"text":      "{{ .Payload.StartTime }}",
					},
				},
			},
		},
	}

	output, err := NewSlackOutput(config, nil)
	require.NoError(t, err)

	// Test with event that has StartTime - should include all fields
	eventWithStartTime := nomad.Event{
		Topic: "Job",
		Type:  "JobStarted",
		Index: 123,
		Payload: map[string]interface{}{
			"StartTime": "2023-01-01T10:00:00Z",
		},
	}

	message, err := output.formatEvent(eventWithStartTime)
	require.NoError(t, err)

	blocks, ok := message.Blocks.([]slack.Block)
	require.True(t, ok)
	require.Len(t, blocks, 1)

	sectionBlock, ok := blocks[0].(*slack.SectionBlock)
	require.True(t, ok)
	assert.Len(t, sectionBlock.Fields, 4) // All fields should be present

	// Test with event that doesn't have StartTime - should exclude conditional fields
	eventWithoutStartTime := nomad.Event{
		Topic: "Job",
		Type:  "JobStopped",
		Index: 124,
		Payload: map[string]interface{}{
			"JobID": "job-123",
		},
	}

	message, err = output.formatEvent(eventWithoutStartTime)
	require.NoError(t, err)

	blocks, ok = message.Blocks.([]slack.Block)
	require.True(t, ok)
	require.Len(t, blocks, 1)

	sectionBlock, ok = blocks[0].(*slack.SectionBlock)
	require.True(t, ok)
	assert.Len(t, sectionBlock.Fields, 2) // Only non-conditional fields
}

func TestSlackOutputWithConditionalElements(t *testing.T) {
	config := map[string]interface{}{
		"webhook_url": "https://hooks.slack.com/services/test",
		"channel":     "#test",
		"blocks": []interface{}{
			map[string]interface{}{
				"type": "actions",
				"elements": []interface{}{
					map[string]interface{}{
						"type": "button",
						"text": map[string]interface{}{
							"type": "plain_text",
							"text": "View Details",
						},
						"url": "https://example.com/details",
					},
					map[string]interface{}{
						"condition": "event.Payload.Failed > 0",
						"type":      "button",
						"text": map[string]interface{}{
							"type": "plain_text",
							"text": "View Failures",
						},
						"url": "https://example.com/failures",
					},
				},
			},
		},
	}

	output, err := NewSlackOutput(config, nil)
	require.NoError(t, err)

	// Test with event that has failures - should include both buttons
	eventWithFailures := nomad.Event{
		Topic: "Job",
		Type:  "JobFailed",
		Index: 123,
		Payload: map[string]interface{}{
			"Failed": 3,
		},
	}

	message, err := output.formatEvent(eventWithFailures)
	require.NoError(t, err)

	blocks, ok := message.Blocks.([]slack.Block)
	require.True(t, ok)
	require.Len(t, blocks, 1)

	actionBlock, ok := blocks[0].(*slack.ActionBlock)
	require.True(t, ok)
	assert.Len(t, actionBlock.Elements.ElementSet, 2) // Both buttons should be present

	// Test with event that has no failures - should only include first button
	eventWithoutFailures := nomad.Event{
		Topic: "Job",
		Type:  "JobCompleted",
		Index: 124,
		Payload: map[string]interface{}{
			"Failed": 0,
		},
	}

	message, err = output.formatEvent(eventWithoutFailures)
	require.NoError(t, err)

	blocks, ok = message.Blocks.([]slack.Block)
	require.True(t, ok)
	require.Len(t, blocks, 1)

	actionBlock, ok = blocks[0].(*slack.ActionBlock)
	require.True(t, ok)
	assert.Len(t, actionBlock.Elements.ElementSet, 1) // Only first button
}

func TestSlackOutputWithConditionalRangeItems(t *testing.T) {
	config := map[string]interface{}{
		"webhook_url": "https://hooks.slack.com/services/test",
		"channel":     "#test",
		"blocks": []interface{}{
			map[string]interface{}{
				"type": "section",
				"text": map[string]interface{}{
					"type": "mrkdwn",
					"text": "Services Status",
				},
				"fields": []interface{}{
					map[string]interface{}{
						"range":     ".Payload.Services",
						"condition": "event.Status == 'running'", // In range context, event refers to the item
						"type":      "mrkdwn",
						"text":      "{{ .Name }}: {{ .Status }}",
					},
				},
			},
		},
	}

	output, err := NewSlackOutput(config, nil)
	require.NoError(t, err)

	// Test with mixed service statuses - should only include running services
	event := nomad.Event{
		Topic: "Deployment",
		Type:  "DeploymentStatusUpdate",
		Index: 123,
		Payload: map[string]interface{}{
			"Services": []interface{}{
				map[string]interface{}{
					"Name":   "web",
					"Status": "running",
				},
				map[string]interface{}{
					"Name":   "api",
					"Status": "stopped",
				},
				map[string]interface{}{
					"Name":   "worker",
					"Status": "running",
				},
			},
		},
	}

	message, err := output.formatEvent(event)
	require.NoError(t, err)

	blocks, ok := message.Blocks.([]slack.Block)
	require.True(t, ok)
	require.Len(t, blocks, 1)

	sectionBlock, ok := blocks[0].(*slack.SectionBlock)
	require.True(t, ok)
	// Should have 2 fields (web and worker are running, api is stopped and excluded)
	assert.Len(t, sectionBlock.Fields, 2)
}

func TestSlackTemplateEngineConditionEvaluation(t *testing.T) {
	engine := NewSlackTemplateEngine(nil)

	eventData := map[string]interface{}{
		"Topic": "Node",
		"Type":  "NodeRegistration",
		"Index": uint64(123),
		"Payload": map[string]interface{}{
			"Node": map[string]interface{}{
				"Name": "worker-1",
			},
		},
	}

	tests := []struct {
		name      string
		condition string
		expected  bool
	}{
		{
			name:      "empty condition always true",
			condition: "",
			expected:  true,
		},
		{
			name:      "topic match",
			condition: "event.Topic == 'Node'",
			expected:  true,
		},
		{
			name:      "topic mismatch",
			condition: "event.Topic == 'Job'",
			expected:  false,
		},
		{
			name:      "payload field check",
			condition: "event.Payload.Node.Name == 'worker-1'",
			expected:  true,
		},
		{
			name:      "payload field mismatch",
			condition: "event.Payload.Node.Name == 'worker-2'",
			expected:  false,
		},
		{
			name:      "complex condition with AND",
			condition: "event.Topic == 'Node' && event.Type == 'NodeRegistration'",
			expected:  true,
		},
		{
			name:      "complex condition with OR",
			condition: "event.Topic == 'Job' || event.Type == 'NodeRegistration'",
			expected:  true,
		},
		{
			name:      "invalid condition - graceful degradation",
			condition: "invalid..syntax",
			expected:  true, // Should default to true on error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.evaluateCondition(tt.condition, eventData)
			assert.Equal(t, tt.expected, result)
		})
	}
}
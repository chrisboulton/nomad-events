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
		"webhook_url": "http://invalid-fake-url.local/webhook",
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
		"webhook_url": "http://invalid-fake-url.local/webhook",
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
		"webhook_url": "http://invalid-fake-url.local/webhook",
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
		"webhook_url": "http://invalid-fake-url.local/webhook",
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

func TestSlackOutputSkipsEmptyMessages(t *testing.T) {
	tests := []struct {
		name         string
		config       map[string]interface{}
		event        nomad.Event
		shouldSend   bool
		description  string
	}{
		{
			name: "skip when all blocks filtered out",
			config: map[string]interface{}{
				"webhook_url": "http://invalid-fake-url.local/webhook",
				"channel":     "#test",
				"blocks": []interface{}{
					map[string]interface{}{
						"condition": "event.Topic == 'NonExistentTopic'",
						"type":      "header",
						"text":      "This should not appear",
					},
				},
			},
			event: nomad.Event{
				Topic: "Job",
				Type:  "JobRegistered",
				Index: 123,
			},
			shouldSend:  false,
			description: "All blocks should be filtered out, message should not be sent",
		},
		{
			name: "send when some blocks pass conditions",
			config: map[string]interface{}{
				"webhook_url": "http://invalid-fake-url.local/webhook",
				"channel":     "#test",
				"blocks": []interface{}{
					map[string]interface{}{
						"condition": "event.Topic == 'NonExistentTopic'",
						"type":      "header",
						"text":      "This should not appear",
					},
					map[string]interface{}{
						"condition": "event.Topic == 'Job'",
						"type":      "section",
						"text": map[string]interface{}{
							"type": "mrkdwn",
							"text": "This should appear",
						},
					},
				},
			},
			event: nomad.Event{
				Topic: "Job",
				Type:  "JobRegistered",
				Index: 123,
			},
			shouldSend:  true,
			description: "Some blocks should pass, message should be sent",
		},
		{
			name: "send text-only message even without blocks",
			config: map[string]interface{}{
				"webhook_url": "http://invalid-fake-url.local/webhook",
				"channel":     "#test",
				"text":        "Job event: {{ .Type }}",
			},
			event: nomad.Event{
				Topic: "Job",
				Type:  "JobRegistered",
				Index: 123,
			},
			shouldSend:  true,
			description: "Text-only messages should always be sent",
		},
		{
			name: "send when blocks exist and no conditions",
			config: map[string]interface{}{
				"webhook_url": "http://invalid-fake-url.local/webhook",
				"channel":     "#test",
				"blocks": []interface{}{
					map[string]interface{}{
						"type": "header",
						"text": "Unconditional header",
					},
				},
			},
			event: nomad.Event{
				Topic: "Job",
				Type:  "JobRegistered",
				Index: 123,
			},
			shouldSend:  true,
			description: "Unconditional blocks should always be sent",
		},
		{
			name: "skip when no blocks and no text",
			config: map[string]interface{}{
				"webhook_url": "http://invalid-fake-url.local/webhook",
				"channel":     "#test",
			},
			event: nomad.Event{
				Topic: "Job",
				Type:  "JobRegistered",
				Index: 123,
			},
			shouldSend:  false,
			description: "Messages with no content should be skipped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := NewSlackOutput(tt.config, nil)
			require.NoError(t, err)

			// Try to send the message
			err = output.Send(tt.event)
			
			if tt.shouldSend {
				// We expect this to fail because we're using a fake webhook URL,
				// but it should fail at the HTTP request stage, not before
				assert.Error(t, err, tt.description)
				assert.Contains(t, err.Error(), "failed to send Slack message", tt.description)
			} else {
				// We expect this to return nil (no error) because the message was skipped
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

func TestShouldSkipMessageFunction(t *testing.T) {
	tests := []struct {
		name         string
		message      SlackMessage
		blockConfigs []BlockConfig
		textTemplate string
		expected     bool
	}{
		{
			name: "skip when blocks configured but none returned",
			message: SlackMessage{
				Text:   "",
				Blocks: []slack.Block{}, // Empty blocks slice
			},
			blockConfigs: []BlockConfig{
				{Type: "header"},
			},
			textTemplate: "",
			expected:     true,
		},
		{
			name: "don't skip when blocks returned",
			message: SlackMessage{
				Text: "",
				Blocks: []slack.Block{
					&slack.HeaderBlock{}, // At least one block
				},
			},
			blockConfigs: []BlockConfig{
				{Type: "header"},
			},
			textTemplate: "",
			expected:     false,
		},
		{
			name: "don't skip when no block configs but has text",
			message: SlackMessage{
				Text:   "Some text message",
				Blocks: nil,
			},
			blockConfigs: []BlockConfig{},
			textTemplate: "{{ .Type }}",
			expected:     false,
		},
		{
			name: "skip when no blocks and no text",
			message: SlackMessage{
				Text:   "",
				Blocks: nil,
			},
			blockConfigs: []BlockConfig{},
			textTemplate: "",
			expected:     true,
		},
		{
			name: "don't skip when blocks interface is not slice",
			message: SlackMessage{
				Text:   "",
				Blocks: "invalid", // Not a slice
			},
			blockConfigs: []BlockConfig{
				{Type: "header"},
			},
			textTemplate: "",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldSkipMessage(tt.message, tt.blockConfigs, tt.textTemplate)
			assert.Equal(t, tt.expected, result)
		})
	}
}
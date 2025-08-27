package outputs

import (
	"fmt"
	"testing"

	"nomad-events/internal/nomad"

	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSlackTemplateEngine(t *testing.T) {
	engine := NewSlackTemplateEngine(nil)
	assert.NotNil(t, engine)
	assert.NotNil(t, engine.engine)
}

func TestSlackTemplateEngineProcessText(t *testing.T) {
	engine := NewSlackTemplateEngine(nil)
	eventData := map[string]interface{}{
		"Topic": "Node",
		"Type":  "NodeRegistration",
		"Payload": map[string]interface{}{
			"Node": map[string]interface{}{
				"Name": "worker-1",
				"ID":   "node-123",
			},
		},
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "simple text",
			template: "Hello World",
			expected: "Hello World",
		},
		{
			name:     "basic interpolation",
			template: "Topic: {{ .Topic }}",
			expected: "Topic: Node",
		},
		{
			name:     "nested field access",
			template: "Node: {{ .Payload.Node.Name }}",
			expected: "Node: worker-1",
		},
		{
			name:     "multiple fields",
			template: "{{ .Topic }}/{{ .Type }}: {{ .Payload.Node.Name }}",
			expected: "Node/NodeRegistration: worker-1",
		},
		{
			name:     "template function",
			template: "{{ upper .Topic }}",
			expected: "NODE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.processText(tt.template, eventData)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSlackTemplateEngineCreateHeaderBlock(t *testing.T) {
	engine := NewSlackTemplateEngine(nil)
	eventData := map[string]interface{}{
		"Topic": "Deployment",
		"Payload": map[string]interface{}{
			"DeploymentID": "deploy-123",
		},
	}

	blockConfig := BlockConfig{
		Type: "header",
		Text: "🚀 Deployment: {{ .Payload.DeploymentID }}",
	}

	block, err := engine.createHeaderBlock(blockConfig, eventData)
	require.NoError(t, err)

	headerBlock, ok := block.(*slack.HeaderBlock)
	require.True(t, ok)
	assert.Equal(t, "🚀 Deployment: deploy-123", headerBlock.Text.Text)
}

func TestSlackTemplateEngineCreateSectionBlock(t *testing.T) {
	engine := NewSlackTemplateEngine(nil)
	eventData := map[string]interface{}{
		"Topic": "Node",
		"Payload": map[string]interface{}{
			"Node": map[string]interface{}{
				"Name":   "worker-1",
				"Status": "ready",
			},
		},
	}

	blockConfig := BlockConfig{
		Type: "section",
		Text: map[string]interface{}{
			"type": "mrkdwn",
			"text": "*Node:* {{ .Payload.Node.Name }}\n*Status:* {{ .Payload.Node.Status }}",
		},
		Fields: []interface{}{
			map[string]interface{}{
				"type": "mrkdwn",
				"text": "*Topic:*",
			},
			map[string]interface{}{
				"type": "plain_text",
				"text": "{{ .Topic }}",
			},
		},
	}

	block, err := engine.createSectionBlock(blockConfig, eventData)
	require.NoError(t, err)

	sectionBlock, ok := block.(*slack.SectionBlock)
	require.True(t, ok)
	assert.Equal(t, "*Node:* worker-1\n*Status:* ready", sectionBlock.Text.Text)
	assert.Len(t, sectionBlock.Fields, 2)
	assert.Equal(t, "*Topic:*", sectionBlock.Fields[0].Text)
	assert.Equal(t, "Node", sectionBlock.Fields[1].Text)
}

func TestSlackTemplateEngineCreateActionBlock(t *testing.T) {
	engine := NewSlackTemplateEngine(nil)
	eventData := map[string]interface{}{
		"Payload": map[string]interface{}{
			"DeploymentID": "deploy-123",
		},
	}

	blockConfig := BlockConfig{
		Type: "actions",
		Elements: []interface{}{
			map[string]interface{}{
				"type": "button",
				"text": map[string]interface{}{
					"type": "plain_text",
					"text": "View Deployment",
				},
				"url":       "https://nomad.example.com/ui/deployments/{{ .Payload.DeploymentID }}",
				"action_id": "view_deployment",
			},
		},
	}

	block, err := engine.createActionBlock(blockConfig, eventData)
	require.NoError(t, err)

	actionBlock, ok := block.(*slack.ActionBlock)
	require.True(t, ok)
	assert.Len(t, actionBlock.Elements.ElementSet, 1)

	buttonElement, ok := actionBlock.Elements.ElementSet[0].(*slack.ButtonBlockElement)
	require.True(t, ok)
	assert.Equal(t, "View Deployment", buttonElement.Text.Text)
	assert.Equal(t, "https://nomad.example.com/ui/deployments/deploy-123", buttonElement.URL)
}

func TestSlackTemplateEngineProcessRangeBlocks(t *testing.T) {
	engine := NewSlackTemplateEngine(nil)
	eventData := map[string]interface{}{
		"Topic": "Node",
		"Payload": map[string]interface{}{
			"Services": []interface{}{
				map[string]interface{}{
					"Name":    "web",
					"Version": "v1.2.3",
					"Status":  "running",
				},
				map[string]interface{}{
					"Name":    "api",
					"Version": "v2.1.0",
					"Status":  "starting",
				},
			},
		},
	}

	// Create a block config map with range field for testing
	blockConfigMap := map[string]interface{}{
		"type":  "context",
		"range": ".Payload.Services",
		"elements": []interface{}{
			map[string]interface{}{
				"type": "mrkdwn",
				"text": "Service: {{ .Name }} ({{ .Version }}) - {{ .Status }}",
			},
		},
	}

	// Test the range expansion directly
	isRange, rangePath := engine.isRangeItem(blockConfigMap)
	require.True(t, isRange)
	assert.Equal(t, ".Payload.Services", rangePath)

	blocks, err := engine.expandRangeItem(blockConfigMap, rangePath, eventData, func(templateItem interface{}, itemData map[string]interface{}) (interface{}, error) {
		// Convert template item back to BlockConfig for processing
		if templateMap, ok := templateItem.(map[string]interface{}); ok {
			blockConfig := mapBlockConfig(templateMap)
			return engine.processBlock(blockConfig, itemData)
		}
		return nil, fmt.Errorf("invalid template item")
	})
	require.NoError(t, err)
	assert.Len(t, blocks, 2)

	for i, block := range blocks {
		contextBlock, ok := block.(*slack.ContextBlock)
		require.True(t, ok)
		assert.Len(t, contextBlock.ContextElements.Elements, 1)

		textElement, ok := contextBlock.ContextElements.Elements[0].(*slack.TextBlockObject)
		require.True(t, ok)

		if i == 0 {
			assert.Equal(t, "Service: web (v1.2.3) - running", textElement.Text)
		} else {
			assert.Equal(t, "Service: api (v2.1.0) - starting", textElement.Text)
		}
	}
}

func TestSlackTemplateEngineProcessBlocks(t *testing.T) {
	engine := NewSlackTemplateEngine(nil)

	event := nomad.Event{
		Topic: "Deployment",
		Type:  "DeploymentPromotion",
		Index: 12345,
		Payload: map[string]interface{}{
			"DeploymentID": "deploy-abc123",
			"Status":       "successful",
			"Node":         "worker-1",
		},
	}

	blockConfigs := []BlockConfig{
		{
			Type: "header",
			Text: "🚀 {{ .Type }}",
		},
		{
			Type: "divider",
		},
		{
			Type: "section",
			Text: map[string]interface{}{
				"type": "mrkdwn",
				"text": "*Deployment:* {{ .Payload.DeploymentID }}\n*Status:* {{ .Payload.Status }}",
			},
		},
	}

	blocks, err := engine.ProcessBlocks(blockConfigs, event)
	require.NoError(t, err)
	assert.Len(t, blocks, 3)

	headerBlock, ok := blocks[0].(*slack.HeaderBlock)
	require.True(t, ok)
	assert.Equal(t, "🚀 DeploymentPromotion", headerBlock.Text.Text)

	_, ok = blocks[1].(*slack.DividerBlock)
	require.True(t, ok)

	sectionBlock, ok := blocks[2].(*slack.SectionBlock)
	require.True(t, ok)
	assert.Equal(t, "*Deployment:* deploy-abc123\n*Status:* successful", sectionBlock.Text.Text)
}

func TestSlackTemplateEngineParseTextConfig(t *testing.T) {
	engine := NewSlackTemplateEngine(nil)
	eventData := map[string]interface{}{
		"Topic": "Node",
	}

	tests := []struct {
		name         string
		textConfig   interface{}
		expectedType string
		expectedText string
	}{
		{
			name:         "string text",
			textConfig:   "Topic: {{ .Topic }}",
			expectedType: slack.MarkdownType,
			expectedText: "Topic: Node",
		},
		{
			name: "object with type",
			textConfig: map[string]interface{}{
				"type": "plain_text",
				"text": "Topic: {{ .Topic }}",
			},
			expectedType: "plain_text",
			expectedText: "Topic: Node",
		},
		{
			name: "object without type defaults to mrkdwn",
			textConfig: map[string]interface{}{
				"text": "Topic: {{ .Topic }}",
			},
			expectedType: slack.MarkdownType,
			expectedText: "Topic: Node",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := engine.parseTextConfig(tt.textConfig, eventData)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedType, config.Type)
			assert.Equal(t, tt.expectedText, config.Text)
		})
	}
}

func TestSlackTemplateEngineCreateTemplateData(t *testing.T) {
	engine := NewSlackTemplateEngine(nil)

	event := nomad.Event{
		Topic:     "Node",
		Type:      "NodeRegistration",
		Key:       "node-123",
		Namespace: "default",
		Index:     12345,
		Payload: map[string]interface{}{
			"Node": map[string]interface{}{
				"Name": "worker-1",
				"ID":   "node-123",
			},
		},
	}

	data := engine.engine.CreateTemplateData(event)

	assert.Equal(t, "Node", data["Topic"])
	assert.Equal(t, "NodeRegistration", data["Type"])
	assert.Equal(t, "node-123", data["Key"])
	assert.Equal(t, "default", data["Namespace"])
	assert.Equal(t, uint64(12345), data["Index"])

	payload, ok := data["Payload"].(map[string]interface{})
	require.True(t, ok)

	node, ok := payload["Node"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "worker-1", node["Name"])
	assert.Equal(t, "node-123", node["ID"])
}

func TestSlackTemplateEngineGetNestedValue(t *testing.T) {
	engine := NewSlackTemplateEngine(nil)
	data := map[string]interface{}{
		"Payload": map[string]interface{}{
			"Node": map[string]interface{}{
				"Name": "worker-1",
				"Attributes": map[string]interface{}{
					"cpu.arch": "amd64",
				},
			},
		},
	}

	tests := []struct {
		name     string
		path     string
		expected interface{}
		hasError bool
	}{
		{
			name:     "simple path",
			path:     "Payload.Node.Name",
			expected: "worker-1",
			hasError: false,
		},
		{
			name:     "nested path",
			path:     "Payload.Node.Attributes",
			expected: map[string]interface{}{"cpu.arch": "amd64"},
			hasError: false,
		},
		{
			name:     "non-existent path",
			path:     "Payload.Node.NonExistent",
			expected: nil,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.getNestedValue(data, tt.path)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}


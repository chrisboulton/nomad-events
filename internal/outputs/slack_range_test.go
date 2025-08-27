package outputs

import (
	"testing"

	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"nomad-events/internal/nomad"
)

func TestSlackTemplateEngineMixedStaticDynamicFields(t *testing.T) {
	engine := NewSlackTemplateEngine()
	eventData := map[string]interface{}{
		"Topic": "Node",
		"Type":  "NodeUpdate",
		"Payload": map[string]interface{}{
			"ServiceCount": "3",
			"Services": []interface{}{
				map[string]interface{}{
					"Name":    "web",
					"Status":  "running",
					"Version": "v1.2.3",
				},
				map[string]interface{}{
					"Name":    "api",
					"Status":  "starting",
					"Version": "v2.1.0",
				},
			},
		},
	}

	blockConfig := BlockConfig{
		Type: "section",
		Text: map[string]interface{}{
			"type": "mrkdwn",
			"text": "Service Status Report",
		},
		Fields: []interface{}{
			// Static field
			map[string]interface{}{
				"type": "mrkdwn",
				"text": "*Total Services:*",
			},
			// Static field with template
			map[string]interface{}{
				"type": "plain_text",
				"text": "{{ .Payload.ServiceCount }}",
			},
			// Dynamic fields from range
			map[string]interface{}{
				"range": ".Payload.Services",
				"type":  "mrkdwn",
				"text":  "*{{ .Name }}:*",
			},
			map[string]interface{}{
				"range": ".Payload.Services",
				"type":  "plain_text",
				"text":  "{{ .Status }} ({{ .Version }})",
			},
		},
	}

	block, err := engine.createSectionBlock(blockConfig, eventData)
	require.NoError(t, err)

	sectionBlock, ok := block.(*slack.SectionBlock)
	require.True(t, ok)
	assert.Equal(t, "Service Status Report", sectionBlock.Text.Text)
	
	// Should have 6 fields: 2 static + 2x2 from range (2 services)
	assert.Len(t, sectionBlock.Fields, 6)
	
	// Check static fields
	assert.Equal(t, "*Total Services:*", sectionBlock.Fields[0].Text)
	assert.Equal(t, "3", sectionBlock.Fields[1].Text)
	
	// Check dynamic fields from first range (service names)
	assert.Equal(t, "*web:*", sectionBlock.Fields[2].Text)
	assert.Equal(t, "*api:*", sectionBlock.Fields[3].Text)
	
	// Check dynamic fields from second range (service status)
	assert.Equal(t, "running (v1.2.3)", sectionBlock.Fields[4].Text)
	assert.Equal(t, "starting (v2.1.0)", sectionBlock.Fields[5].Text)
}

func TestSlackTemplateEngineMixedStaticDynamicActionElements(t *testing.T) {
	engine := NewSlackTemplateEngine()
	eventData := map[string]interface{}{
		"Topic": "Deployment",
		"Type":  "DeploymentUpdate",
		"Payload": map[string]interface{}{
			"DeploymentID": "deploy-123",
			"QuickActions": []interface{}{
				map[string]interface{}{
					"Label": "Promote",
					"URL":   "https://example.com/promote",
					"ID":    "promote",
				},
				map[string]interface{}{
					"Label": "Cancel",
					"URL":   "https://example.com/cancel",
					"ID":    "cancel",
				},
			},
		},
	}

	blockConfig := BlockConfig{
		Type: "actions",
		Elements: []interface{}{
			// Static button
			map[string]interface{}{
				"type": "button",
				"text": map[string]interface{}{
					"type": "plain_text",
					"text": "View All",
				},
				"url": "https://example.com/ui/deployments",
			},
			// Dynamic buttons from range
			map[string]interface{}{
				"range": ".Payload.QuickActions",
				"type":  "button",
				"text": map[string]interface{}{
					"type": "plain_text",
					"text": "{{ .Label }}",
				},
				"url":       "{{ .URL }}",
				"action_id": "quick_{{ .ID }}",
			},
		},
	}

	block, err := engine.createActionBlock(blockConfig, eventData)
	require.NoError(t, err)

	actionBlock, ok := block.(*slack.ActionBlock)
	require.True(t, ok)
	
	// Should have 3 elements: 1 static + 2 from range
	assert.Len(t, actionBlock.Elements.ElementSet, 3)
	
	// Check static button
	staticButton, ok := actionBlock.Elements.ElementSet[0].(*slack.ButtonBlockElement)
	require.True(t, ok)
	assert.Equal(t, "View All", staticButton.Text.Text)
	assert.Equal(t, "https://example.com/ui/deployments", staticButton.URL)
	
	// Check dynamic buttons
	promoteButton, ok := actionBlock.Elements.ElementSet[1].(*slack.ButtonBlockElement)
	require.True(t, ok)
	assert.Equal(t, "Promote", promoteButton.Text.Text)
	assert.Equal(t, "https://example.com/promote", promoteButton.URL)
	assert.Equal(t, "quick_promote", promoteButton.ActionID)
	
	cancelButton, ok := actionBlock.Elements.ElementSet[2].(*slack.ButtonBlockElement)
	require.True(t, ok)
	assert.Equal(t, "Cancel", cancelButton.Text.Text)
	assert.Equal(t, "https://example.com/cancel", cancelButton.URL)
	assert.Equal(t, "quick_cancel", cancelButton.ActionID)
}

func TestSlackTemplateEngineMixedStaticDynamicSelectOptions(t *testing.T) {
	engine := NewSlackTemplateEngine()
	eventData := map[string]interface{}{
		"Topic": "Node",
		"Payload": map[string]interface{}{
			"ManageableServices": []interface{}{
				map[string]interface{}{
					"Name":   "web",
					"ID":     "service-web",
					"Status": "running",
				},
				map[string]interface{}{
					"Name":   "api", 
					"ID":     "service-api",
					"Status": "stopped",
				},
			},
		},
	}

	elemConfig := map[string]interface{}{
		"type": "static_select",
		"placeholder": map[string]interface{}{
			"type": "plain_text",
			"text": "Choose service",
		},
		"action_id": "service_select",
		"options": []interface{}{
			// Static option
			map[string]interface{}{
				"text": map[string]interface{}{
					"type": "plain_text",
					"text": "All Services",
				},
				"value": "all",
			},
			// Dynamic options from range
			map[string]interface{}{
				"range": ".Payload.ManageableServices",
				"text": map[string]interface{}{
					"type": "plain_text",
					"text": "{{ .Name }} ({{ .Status }})",
				},
				"value": "{{ .ID }}",
			},
			// Another static option
			map[string]interface{}{
				"text": map[string]interface{}{
					"type": "plain_text",
					"text": "Other",
				},
				"value": "other",
			},
		},
	}

	element, err := engine.createStaticSelectElement(elemConfig, eventData)
	require.NoError(t, err)

	selectElement, ok := element.(*slack.SelectBlockElement)
	require.True(t, ok)
	
	// Should have 4 options: 1 static + 2 from range + 1 static
	assert.Len(t, selectElement.Options, 4)
	
	// Check static option
	assert.Equal(t, "All Services", selectElement.Options[0].Text.Text)
	assert.Equal(t, "all", selectElement.Options[0].Value)
	
	// Check dynamic options
	assert.Equal(t, "web (running)", selectElement.Options[1].Text.Text)
	assert.Equal(t, "service-web", selectElement.Options[1].Value)
	
	assert.Equal(t, "api (stopped)", selectElement.Options[2].Text.Text)
	assert.Equal(t, "service-api", selectElement.Options[2].Value)
	
	// Check final static option
	assert.Equal(t, "Other", selectElement.Options[3].Text.Text)
	assert.Equal(t, "other", selectElement.Options[3].Value)
}

func TestSlackTemplateEngineMixedStaticDynamicContextElements(t *testing.T) {
	engine := NewSlackTemplateEngine()
	eventData := map[string]interface{}{
		"Topic": "Node",
		"Payload": map[string]interface{}{
			"NodeName": "worker-1",
			"Attributes": []interface{}{
				map[string]interface{}{
					"Key":   "cpu.arch",
					"Value": "amd64",
				},
				map[string]interface{}{
					"Key":   "memory.total",
					"Value": "16GB",
				},
			},
		},
	}

	blockConfig := BlockConfig{
		Type: "context",
		Elements: []interface{}{
			// Static element
			map[string]interface{}{
				"type": "mrkdwn",
				"text": "*Node:* {{ .Payload.NodeName }}",
			},
			// Dynamic elements from range
			map[string]interface{}{
				"range": ".Payload.Attributes",
				"type":  "mrkdwn",
				"text":  "{{ .Key }}: {{ .Value }}",
			},
		},
	}

	block, err := engine.createContextBlock(blockConfig, eventData)
	require.NoError(t, err)

	contextBlock, ok := block.(*slack.ContextBlock)
	require.True(t, ok)
	
	// Should have 3 elements: 1 static + 2 from range
	assert.Len(t, contextBlock.ContextElements.Elements, 3)
	
	// Check static element
	staticElement, ok := contextBlock.ContextElements.Elements[0].(*slack.TextBlockObject)
	require.True(t, ok)
	assert.Equal(t, "*Node:* worker-1", staticElement.Text)
	
	// Check dynamic elements
	attr1Element, ok := contextBlock.ContextElements.Elements[1].(*slack.TextBlockObject)
	require.True(t, ok)
	assert.Equal(t, "cpu.arch: amd64", attr1Element.Text)
	
	attr2Element, ok := contextBlock.ContextElements.Elements[2].(*slack.TextBlockObject)
	require.True(t, ok)
	assert.Equal(t, "memory.total: 16GB", attr2Element.Text)
}

func TestSlackTemplateEngineComplexMixedBlocks(t *testing.T) {
	engine := NewSlackTemplateEngine()
	
	event := nomad.Event{
		Topic: "Deployment",
		Type:  "DeploymentStatusUpdate",
		Index: 12345,
		Payload: mustMarshalJSON(map[string]interface{}{
			"DeploymentID": "deploy-abc123",
			"Status":       "successful",
			"Services": []interface{}{
				map[string]interface{}{
					"Name":    "web",
					"Version": "v1.2.3",
					"Status":  "running",
					"ID":      "service-web",
				},
				map[string]interface{}{
					"Name":    "api",
					"Version": "v2.1.0", 
					"Status":  "running",
					"ID":      "service-api",
				},
			},
			"QuickActions": []interface{}{
				map[string]interface{}{
					"Label": "Rollback",
					"URL":   "https://example.com/rollback",
					"ID":    "rollback",
				},
			},
		}),
	}

	blockConfigs := []BlockConfig{
		{
			Type: "header",
			Text: "🚀 Deployment: {{ .Payload.DeploymentID }}",
		},
		{
			Type: "section",
			Text: map[string]interface{}{
				"type": "mrkdwn",
				"text": "*Status:* {{ .Payload.Status }}",
			},
			Fields: []interface{}{
				// Static field
				map[string]interface{}{
					"type": "mrkdwn",
					"text": "*Services:*",
				},
				// Dynamic service fields
				map[string]interface{}{
					"range": ".Payload.Services",
					"type":  "plain_text",
					"text":  "{{ .Name }} {{ .Version }} ({{ .Status }})",
				},
			},
		},
		{
			Type: "actions",
			Elements: []interface{}{
				// Static button
				map[string]interface{}{
					"type": "button",
					"text": map[string]interface{}{
						"type": "plain_text",
						"text": "View Deployment",
					},
					"url": "https://example.com/ui/deployments/{{ .Payload.DeploymentID }}",
				},
				// Dynamic action buttons
				map[string]interface{}{
					"range": ".Payload.QuickActions",
					"type":  "button",
					"text": map[string]interface{}{
						"type": "plain_text",
						"text": "{{ .Label }}",
					},
					"url":       "{{ .URL }}",
					"action_id": "quick_{{ .ID }}",
				},
				// Select with mixed options
				map[string]interface{}{
					"type": "static_select",
					"placeholder": map[string]interface{}{
						"type": "plain_text",
						"text": "Manage service...",
					},
					"action_id": "manage_service",
					"options": []interface{}{
						// Static option
						map[string]interface{}{
							"text": map[string]interface{}{
								"type": "plain_text",
								"text": "All Services",
							},
							"value": "all",
						},
						// Dynamic options
						map[string]interface{}{
							"range": ".Payload.Services",
							"text": map[string]interface{}{
								"type": "plain_text",
								"text": "{{ .Name }} {{ .Version }}",
							},
							"value": "{{ .ID }}",
						},
					},
				},
			},
		},
	}

	blocks, err := engine.ProcessBlocks(blockConfigs, event)
	require.NoError(t, err)
	assert.Len(t, blocks, 3)

	// Check header block
	headerBlock, ok := blocks[0].(*slack.HeaderBlock)
	require.True(t, ok)
	assert.Equal(t, "🚀 Deployment: deploy-abc123", headerBlock.Text.Text)

	// Check section block with mixed fields
	sectionBlock, ok := blocks[1].(*slack.SectionBlock)
	require.True(t, ok)
	assert.Equal(t, "*Status:* successful", sectionBlock.Text.Text)
	assert.Len(t, sectionBlock.Fields, 3) // 1 static + 2 dynamic
	assert.Equal(t, "*Services:*", sectionBlock.Fields[0].Text)
	assert.Equal(t, "web v1.2.3 (running)", sectionBlock.Fields[1].Text)
	assert.Equal(t, "api v2.1.0 (running)", sectionBlock.Fields[2].Text)

	// Check actions block with mixed elements
	actionBlock, ok := blocks[2].(*slack.ActionBlock)
	require.True(t, ok)
	assert.Len(t, actionBlock.Elements.ElementSet, 3) // 1 static button + 1 dynamic button + 1 select

	// Check static button
	viewButton, ok := actionBlock.Elements.ElementSet[0].(*slack.ButtonBlockElement)
	require.True(t, ok)
	assert.Equal(t, "View Deployment", viewButton.Text.Text)
	assert.Equal(t, "https://example.com/ui/deployments/deploy-abc123", viewButton.URL)

	// Check dynamic button
	rollbackButton, ok := actionBlock.Elements.ElementSet[1].(*slack.ButtonBlockElement)
	require.True(t, ok)
	assert.Equal(t, "Rollback", rollbackButton.Text.Text)
	assert.Equal(t, "https://example.com/rollback", rollbackButton.URL)
	assert.Equal(t, "quick_rollback", rollbackButton.ActionID)

	// Check select with mixed options
	selectElement, ok := actionBlock.Elements.ElementSet[2].(*slack.SelectBlockElement)
	require.True(t, ok)
	assert.Len(t, selectElement.Options, 3) // 1 static + 2 dynamic
	assert.Equal(t, "All Services", selectElement.Options[0].Text.Text)
	assert.Equal(t, "web v1.2.3", selectElement.Options[1].Text.Text)
	assert.Equal(t, "api v2.1.0", selectElement.Options[2].Text.Text)
}

func TestSlackTemplateEngineRangeErrorHandling(t *testing.T) {
	engine := NewSlackTemplateEngine()
	eventData := map[string]interface{}{
		"Topic": "Node",
	}

	// Test with non-existent range path
	blockConfig := BlockConfig{
		Type: "section",
		Fields: []interface{}{
			map[string]interface{}{
				"range": ".Payload.NonExistentServices",
				"type":  "mrkdwn", 
				"text":  "{{ .Name }}",
			},
			// Should still process static field
			map[string]interface{}{
				"type": "plain_text",
				"text": "Static field",
			},
		},
	}

	block, err := engine.createSectionBlock(blockConfig, eventData)
	require.NoError(t, err)

	sectionBlock, ok := block.(*slack.SectionBlock)
	require.True(t, ok)
	
	// Should only have the static field since range failed
	assert.Len(t, sectionBlock.Fields, 1)
	assert.Equal(t, "Static field", sectionBlock.Fields[0].Text)
}
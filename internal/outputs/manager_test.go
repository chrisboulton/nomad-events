package outputs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"nomad-events/internal/config"
	"nomad-events/internal/nomad"
)

func TestNewManager(t *testing.T) {
	tests := []struct {
		name           string
		outputConfigs  map[string]config.Output
		expectError    bool
		errorMsg       string
		expectedCount  int
	}{
		{
			name: "valid outputs",
			outputConfigs: map[string]config.Output{
				"stdout": {
					Type: "stdout",
				},
				"exec": {
					Type: "exec",
					Properties: map[string]interface{}{
						"command": "cat",
					},
				},
			},
			expectError:   false,
			expectedCount: 2,
		},
		{
			name: "unsupported output type",
			outputConfigs: map[string]config.Output{
				"invalid": {
					Type: "unsupported_type",
				},
			},
			expectError: true,
			errorMsg:    "unsupported output type",
		},
		{
			name: "invalid output configuration",
			outputConfigs: map[string]config.Output{
				"slack": {
					Type: "slack",
					Properties: map[string]interface{}{
						// Missing required webhook_url
					},
				},
			},
			expectError: true,
			errorMsg:    "webhook_url is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewManager(tt.outputConfigs, nil)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, manager)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, manager)
				assert.Len(t, manager.outputs, tt.expectedCount)
			}
		})
	}
}

func TestManagerSend(t *testing.T) {
	outputConfigs := map[string]config.Output{
		"stdout": {
			Type: "stdout",
		},
		"exec": {
			Type: "exec",
			Properties: map[string]interface{}{
				"command": "cat",
			},
		},
	}

	manager, err := NewManager(outputConfigs, nil)
	require.NoError(t, err)

	event := nomad.Event{
		Topic: "Node",
		Type:  "NodeRegistration",
		Index: 123,
	}

	t.Run("send to existing output", func(t *testing.T) {
		err := manager.Send("stdout", event)
		assert.NoError(t, err)
	})

	t.Run("send to non-existent output", func(t *testing.T) {
		err := manager.Send("non_existent", event)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "output \"non_existent\" not found")
	})
}

func TestManagerGetOutput(t *testing.T) {
	outputConfigs := map[string]config.Output{
		"stdout": {
			Type: "stdout",
		},
	}

	manager, err := NewManager(outputConfigs, nil)
	require.NoError(t, err)

	t.Run("get existing output", func(t *testing.T) {
		output, exists := manager.GetOutput("stdout")
		assert.True(t, exists)
		assert.NotNil(t, output)
		assert.IsType(t, &StdoutOutput{}, output)
	})

	t.Run("get non-existent output", func(t *testing.T) {
		output, exists := manager.GetOutput("non_existent")
		assert.False(t, exists)
		assert.Nil(t, output)
	})
}

func TestCreateOutput(t *testing.T) {
	tests := []struct {
		name         string
		config       config.Output
		expectedType interface{}
		expectError  bool
		errorMsg     string
	}{
		{
			name: "stdout output",
			config: config.Output{
				Type: "stdout",
			},
			expectedType: &StdoutOutput{},
			expectError:  false,
		},
		{
			name: "exec output",
			config: config.Output{
				Type: "exec",
				Properties: map[string]interface{}{
					"command": "echo test",
				},
			},
			expectedType: &ExecOutput{},
			expectError:  false,
		},
		{
			name: "slack output with valid config",
			config: config.Output{
				Type: "slack",
				Properties: map[string]interface{}{
					"webhook_url": "https://hooks.slack.com/test",
				},
			},
			expectedType: &SlackOutput{},
			expectError:  false,
		},
		{
			name: "http output with valid config",
			config: config.Output{
				Type: "http",
				Properties: map[string]interface{}{
					"url": "https://example.com/webhook",
				},
			},
			expectedType: &HTTPOutput{},
			expectError:  false,
		},
		{
			name: "unsupported output type",
			config: config.Output{
				Type: "unsupported",
			},
			expectError: true,
			errorMsg:    "unsupported output type: unsupported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := createOutput(tt.config, nil)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, output)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, output)
				assert.IsType(t, tt.expectedType, output)
			}
		})
	}
}
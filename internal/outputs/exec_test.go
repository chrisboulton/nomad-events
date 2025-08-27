package outputs

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"nomad-events/internal/nomad"
)

func TestNewExecOutput(t *testing.T) {
	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid string command",
			config: map[string]interface{}{
				"command": "echo hello",
			},
			expectError: false,
		},
		{
			name: "valid array command",
			config: map[string]interface{}{
				"command": []string{"echo", "hello"},
			},
			expectError: false,
		},
		{
			name: "valid interface array command",
			config: map[string]interface{}{
				"command": []interface{}{"echo", "hello"},
			},
			expectError: false,
		},
		{
			name: "command with timeout",
			config: map[string]interface{}{
				"command": "echo test",
				"timeout": 5,
			},
			expectError: false,
		},
		{
			name: "command with workdir and env",
			config: map[string]interface{}{
				"command": "pwd",
				"workdir": "/tmp",
				"env": map[string]interface{}{
					"TEST_VAR": "test_value",
				},
			},
			expectError: false,
		},
		{
			name: "missing command",
			config: map[string]interface{}{
				"timeout": 5,
			},
			expectError: true,
			errorMsg:    "command is required",
		},
		{
			name: "empty command",
			config: map[string]interface{}{
				"command": "",
			},
			expectError: true,
			errorMsg:    "command cannot be empty",
		},
		{
			name: "invalid command type",
			config: map[string]interface{}{
				"command": 123,
			},
			expectError: true,
			errorMsg:    "command must be a string or array of strings",
		},
		{
			name: "invalid command array element",
			config: map[string]interface{}{
				"command": []interface{}{"echo", 123},
			},
			expectError: true,
			errorMsg:    "command arguments must be strings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := NewExecOutput(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, output)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, output)
			}
		})
	}
}

func TestExecOutputSend(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping exec tests on Windows")
	}

	tests := []struct {
		name        string
		config      map[string]interface{}
		event       nomad.Event
		expectError bool
		errorMsg    string
	}{
		{
			name: "successful command execution",
			config: map[string]interface{}{
				"command": "cat",
				"timeout": 5,
			},
			event: nomad.Event{
				Topic: "Node",
				Type:  "NodeRegistration",
				Index: 123,
			},
			expectError: false,
		},
		{
			name: "command with jq parsing",
			config: map[string]interface{}{
				"command": []string{"sh", "-c", "jq -r .Topic || echo 'jq not available'"},
				"timeout": 5,
			},
			event: nomad.Event{
				Topic: "Node",
				Type:  "NodeRegistration",
				Index: 123,
			},
			expectError: false,
		},
		{
			name: "command timeout",
			config: map[string]interface{}{
				"command": "sleep 2",
				"timeout": 1,
			},
			event: nomad.Event{
				Topic: "Node",
				Type:  "NodeRegistration",
				Index: 123,
			},
			expectError: true,
			errorMsg:    "command execution failed",
		},
		{
			name: "command not found",
			config: map[string]interface{}{
				"command": "non_existent_command_12345",
				"timeout": 5,
			},
			event: nomad.Event{
				Topic: "Node",
				Type:  "NodeRegistration",
				Index: 123,
			},
			expectError: true,
			errorMsg:    "command execution failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := NewExecOutput(tt.config)
			require.NoError(t, err)

			err = output.Send(tt.event)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExecOutputConfiguration(t *testing.T) {
	config := map[string]interface{}{
		"command": []string{"echo", "test"},
		"timeout": 10,
		"workdir": "/tmp",
		"env": map[string]interface{}{
			"TEST_VAR": "test_value",
			"DEBUG":    "true",
		},
	}

	output, err := NewExecOutput(config)
	require.NoError(t, err)

	assert.Equal(t, []string{"echo", "test"}, output.command)
	assert.Equal(t, 10*time.Second, output.timeout)
	assert.Equal(t, "/tmp", output.workdir)
	assert.Contains(t, output.env, "TEST_VAR=test_value")
	assert.Contains(t, output.env, "DEBUG=true")
}
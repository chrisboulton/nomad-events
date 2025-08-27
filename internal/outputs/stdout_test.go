package outputs

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"nomad-events/internal/nomad"
)

func TestNewStdoutOutput(t *testing.T) {
	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name:        "default json format",
			config:      map[string]interface{}{},
			expectError: false,
		},
		{
			name:        "explicit json format",
			config:      map[string]interface{}{"format": "json"},
			expectError: false,
		},
		{
			name:        "text format with template",
			config:      map[string]interface{}{"format": "text", "text": "{{ .Topic }}/{{ .Type }}"},
			expectError: false,
		},
		{
			name:        "text format without template",
			config:      map[string]interface{}{"format": "text"},
			expectError: true,
			errorMsg:    "text template is required when format is 'text'",
		},
		{
			name:        "invalid format",
			config:      map[string]interface{}{"format": "xml"},
			expectError: true,
			errorMsg:    "invalid format \"xml\": must be 'json' or 'text'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := NewStdoutOutput(tt.config, nil)
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

func TestStdoutOutputSend(t *testing.T) {
	output, err := NewStdoutOutput(map[string]interface{}{}, nil)
	require.NoError(t, err)

	event := nomad.Event{
		Topic:     "Node",
		Type:      "NodeRegistration",
		Key:       "node-1",
		Namespace: "default",
		Index:     123,
		Payload: map[string]interface{}{
			"Node": map[string]interface{}{
				"Name": "worker-1",
			},
		},
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = output.Send(event)
	assert.NoError(t, err)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output_str := buf.String()

	var outputEvent nomad.Event
	err = json.Unmarshal([]byte(output_str), &outputEvent)
	assert.NoError(t, err)
	assert.Equal(t, event.Topic, outputEvent.Topic)
	assert.Equal(t, event.Type, outputEvent.Type)
	assert.Equal(t, event.Index, outputEvent.Index)
}
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
	output, err := NewStdoutOutput(map[string]interface{}{})
	assert.NoError(t, err)
	assert.NotNil(t, output)
}

func TestStdoutOutputSend(t *testing.T) {
	output, err := NewStdoutOutput(map[string]interface{}{})
	require.NoError(t, err)

	event := nomad.Event{
		Topic:     "Node",
		Type:      "NodeRegistration",
		Key:       "node-1",
		Namespace: "default",
		Index:     123,
		Payload:   []byte(`{"Node": {"Name": "worker-1"}}`),
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
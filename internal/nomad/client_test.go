package nomad

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEventStream(t *testing.T) {
	tests := []struct {
		name        string
		address     string
		token       string
		expectError bool
	}{
		{
			name:        "valid address without token",
			address:     "http://localhost:4646",
			token:       "",
			expectError: false,
		},
		{
			name:        "valid address with token",
			address:     "http://localhost:4646",
			token:       "test-token",
			expectError: false,
		},
		{
			name:        "invalid address format still creates client",
			address:     "not-a-valid-url",
			token:       "",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream, err := NewEventStream(tt.address, tt.token)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, stream)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, stream)
				assert.Equal(t, time.Second, stream.retryBackoff)
				assert.Equal(t, 10, stream.maxRetries)
				assert.Equal(t, uint64(0), stream.lastIndex)
			}
		})
	}
}

func TestEventStreamExponentialBackoff(t *testing.T) {
	stream, err := NewEventStream("http://localhost:4646", "")
	require.NoError(t, err)

	assert.Equal(t, time.Second, stream.retryBackoff)

	stream.exponentialBackoff()
	assert.Equal(t, 2*time.Second, stream.retryBackoff)

	stream.exponentialBackoff()
	assert.Equal(t, 4*time.Second, stream.retryBackoff)

	for i := 0; i < 10; i++ {
		stream.exponentialBackoff()
	}
	assert.Equal(t, time.Minute, stream.retryBackoff)
}

func TestEventStructure(t *testing.T) {
	event := Event{
		Topic:     "Node",
		Type:      "NodeRegistration",
		Key:       "node-123",
		Namespace: "default",
		Index:     12345,
		Payload:   json.RawMessage(`{"Node": {"Name": "worker-1"}}`),
	}

	assert.Equal(t, "Node", event.Topic)
	assert.Equal(t, "NodeRegistration", event.Type)
	assert.Equal(t, "node-123", event.Key)
	assert.Equal(t, "default", event.Namespace)
	assert.Equal(t, uint64(12345), event.Index)

	var payload map[string]interface{}
	err := json.Unmarshal(event.Payload, &payload)
	assert.NoError(t, err)
	
	node, ok := payload["Node"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "worker-1", node["Name"])
}

func TestEventJSONSerialization(t *testing.T) {
	originalEvent := Event{
		Topic:     "Job",
		Type:      "JobRegistered",
		Key:       "job-123",
		Namespace: "production",
		Index:     98765,
		Payload:   json.RawMessage(`{"Job": {"ID": "example-job", "Name": "example"}}`),
	}

	eventJSON, err := json.Marshal(originalEvent)
	require.NoError(t, err)

	var deserializedEvent Event
	err = json.Unmarshal(eventJSON, &deserializedEvent)
	require.NoError(t, err)

	assert.Equal(t, originalEvent.Topic, deserializedEvent.Topic)
	assert.Equal(t, originalEvent.Type, deserializedEvent.Type)
	assert.Equal(t, originalEvent.Key, deserializedEvent.Key)
	assert.Equal(t, originalEvent.Namespace, deserializedEvent.Namespace)
	assert.Equal(t, originalEvent.Index, deserializedEvent.Index)

	var originalPayload, deserializedPayload map[string]interface{}
	err = json.Unmarshal(originalEvent.Payload, &originalPayload)
	require.NoError(t, err)
	err = json.Unmarshal(deserializedEvent.Payload, &deserializedPayload)
	require.NoError(t, err)

	assert.Equal(t, originalPayload, deserializedPayload)
}
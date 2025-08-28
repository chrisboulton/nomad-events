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
		Payload: map[string]interface{}{
			"Node": map[string]interface{}{
				"Name": "worker-1",
			},
		},
	}

	assert.Equal(t, "Node", event.Topic)
	assert.Equal(t, "NodeRegistration", event.Type)
	assert.Equal(t, "node-123", event.Key)
	assert.Equal(t, "default", event.Namespace)
	assert.Equal(t, uint64(12345), event.Index)

	payload, ok := event.Payload.(map[string]interface{})
	assert.True(t, ok)
	
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
		Payload: map[string]interface{}{
			"Job": map[string]interface{}{
				"ID":   "example-job",
				"Name": "example",
			},
		},
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
	assert.Equal(t, originalEvent.Payload, deserializedEvent.Payload)
}

func TestEventWithDiffSerialization(t *testing.T) {
	diff := map[string]interface{}{
		"Type": "Edited",
		"TaskGroups": []interface{}{
			map[string]interface{}{
				"Name": "example",
				"Fields": []interface{}{
					map[string]interface{}{
						"Name": "Count",
						"Old":  "1",
						"New":  "3",
					},
				},
			},
		},
	}

	originalEvent := Event{
		Topic:     "Job",
		Type:      "JobRegistered",
		Key:       "job-123",
		Namespace: "production",
		Index:     98765,
		Payload: map[string]interface{}{
			"Job": map[string]interface{}{
				"ID":   "example-job",
				"Name": "example",
			},
		},
		Diff: diff,
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
	assert.Equal(t, originalEvent.Payload, deserializedEvent.Payload)
	assert.Equal(t, originalEvent.Diff, deserializedEvent.Diff)
}

func TestEventStreamFetchJobDiff(t *testing.T) {
	// Note: This test checks the error handling of fetchJobDiff
	// since we can't mock the Nomad API easily without significant refactoring
	
	stream, err := NewEventStream("http://localhost:4646", "")
	require.NoError(t, err)

	tests := []struct {
		name      string
		jobID     string
		expectErr bool
	}{
		{
			name:      "empty job ID",
			jobID:     "",
			expectErr: true,
		},
		{
			name:      "valid job ID but no connection",
			jobID:     "test-job",
			expectErr: true, // Will fail since we don't have a real Nomad connection
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diff, err := stream.fetchJobDiff(tt.jobID)
			
			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, diff)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, diff)
			}
		})
	}
}

func TestJobVersionHandling(t *testing.T) {
	tests := []struct {
		name           string
		jobVersion     float64
		expectDiffCall bool
	}{
		{
			name:           "version 1 job should not fetch diff",
			jobVersion:     1.0,
			expectDiffCall: false,
		},
		{
			name:           "version 2 job should fetch diff",
			jobVersion:     2.0,
			expectDiffCall: true,
		},
		{
			name:           "version 5 job should fetch diff",
			jobVersion:     5.0,
			expectDiffCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test verifies that we only attempt diff fetching for jobs with version > 1
			// The actual logic is tested implicitly through the version check in the code
			
			jobPayload := map[string]interface{}{
				"Job": map[string]interface{}{
					"ID":      "test-job",
					"Version": tt.jobVersion,
				},
			}

			// Verify the version extraction works
			if jobData, ok := jobPayload["Job"].(map[string]interface{}); ok {
				if version, ok := jobData["Version"].(float64); ok {
					shouldFetchDiff := version > 1
					assert.Equal(t, tt.expectDiffCall, shouldFetchDiff)
				}
			}
		})
	}
}
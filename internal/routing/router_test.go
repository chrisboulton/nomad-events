package routing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"nomad-events/internal/config"
	"nomad-events/internal/nomad"
)

func TestNewRouter(t *testing.T) {
	tests := []struct {
		name        string
		routes      []config.Route
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid routes",
			routes: []config.Route{
				{Filter: "", Output: "stdout"},
				{Filter: "event.Topic == 'Node'", Output: "slack"},
			},
			expectError: false,
		},
		{
			name: "empty filter",
			routes: []config.Route{
				{Filter: "", Output: "stdout"},
			},
			expectError: false,
		},
		{
			name: "invalid CEL expression",
			routes: []config.Route{
				{Filter: "invalid cel expression ===", Output: "stdout"},
			},
			expectError: true,
			errorMsg:    "failed to compile filter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, err := NewRouter(tt.routes)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, router)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, router)
				assert.Len(t, router.rules, len(tt.routes))
			}
		})
	}
}

func TestRouterRoute(t *testing.T) {
	routes := []config.Route{
		{Filter: "", Output: "all_events"},
		{Filter: "event.Topic == 'Node'", Output: "node_events"},
		{Filter: "event.Type == 'NodeRegistration'", Output: "registrations"},
		{Filter: "event.Topic == 'Job' && event.Type == 'JobRegistered'", Output: "job_registered"},
		{Filter: "event.Payload.Node.Name == 'worker-1'", Output: "worker1_events"},
	}

	router, err := NewRouter(routes)
	require.NoError(t, err)

	tests := []struct {
		name            string
		event           nomad.Event
		expectedOutputs []string
	}{
		{
			name: "node registration event",
			event: nomad.Event{
				Topic: "Node",
				Type:  "NodeRegistration",
				Key:   "node-1",
				Index: 1,
				Payload: map[string]interface{}{
					"Node": map[string]interface{}{
						"Name": "worker-1",
						"ID":   "node-1",
					},
				},
			},
			expectedOutputs: []string{"all_events", "node_events", "registrations", "worker1_events"},
		},
		{
			name: "job registered event",
			event: nomad.Event{
				Topic: "Job",
				Type:  "JobRegistered",
				Key:   "job-1",
				Index: 2,
				Payload: map[string]interface{}{
					"Job": map[string]interface{}{
						"Name": "example-job",
						"ID":   "job-1",
					},
				},
			},
			expectedOutputs: []string{"all_events", "job_registered"},
		},
		{
			name: "allocation event",
			event: nomad.Event{
				Topic: "Allocation",
				Type:  "AllocationCreated",
				Key:   "alloc-1",
				Index: 3,
				Payload: map[string]interface{}{
					"Allocation": map[string]interface{}{
						"ID":    "alloc-1",
						"JobID": "job-1",
					},
				},
			},
			expectedOutputs: []string{"all_events"},
		},
		{
			name: "node event for different worker",
			event: nomad.Event{
				Topic: "Node",
				Type:  "NodeUpdate",
				Key:   "node-2",
				Index: 4,
				Payload: map[string]interface{}{
					"Node": map[string]interface{}{
						"Name": "worker-2",
						"ID":   "node-2",
					},
				},
			},
			expectedOutputs: []string{"all_events", "node_events"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs, err := router.Route(tt.event)
			assert.NoError(t, err)
			assert.ElementsMatch(t, tt.expectedOutputs, outputs)
		})
	}
}

func TestRouterRouteNonExistentField(t *testing.T) {
	routes := []config.Route{
		{Filter: "event.NonExistentField == 'value'", Output: "test"},
		{Filter: "", Output: "all_events"},
	}

	router, err := NewRouter(routes)
	require.NoError(t, err)

	event := nomad.Event{
		Topic: "Node",
		Type:  "NodeRegistration",
		Index: 1,
	}

	outputs, err := router.Route(event)
	assert.NoError(t, err)
	assert.Equal(t, []string{"all_events"}, outputs)
}



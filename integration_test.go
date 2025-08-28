package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"nomad-events/internal/config"
	"nomad-events/internal/nomad"
	"nomad-events/internal/outputs"
	"nomad-events/internal/routing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigLoadingIntegration(t *testing.T) {
	tests := []struct {
		name       string
		configYAML string
		shouldWork bool
	}{
		{
			name: "valid_flat_config",
			configYAML: `
nomad:
  address: "http://localhost:4646"
  token: ""

outputs:
  test_stdout:
    type: stdout
    format: json

routes:
  - filter: ""
    output: test_stdout
`,
			shouldWork: true,
		},
		{
			name: "valid_hierarchical_config",
			configYAML: `
nomad:
  address: "http://localhost:4646"
  token: ""

outputs:
  node_output:
    type: stdout
    format: json
  job_output:
    type: stdout
    format: text
    text: "Job: {{ .Payload.Job.ID }}"

routes:
  - filter: event.Topic == 'Node'
    output: node_output
    routes:
      - filter: event.Type == 'NodeRegistration'
        output: job_output
  - filter: event.Topic == 'Job'
    continue: false
    routes:
      - filter: event.Type == 'JobRegistered'
        output: job_output
`,
			shouldWork: true,
		},
		{
			name: "config_with_retry",
			configYAML: `
nomad:
  address: "http://localhost:4646"
  token: ""

outputs:
  reliable_output:
    type: stdout
    format: json
    retry:
      max_retries: 3
      base_delay: "100ms"

routes:
  - filter: ""
    output: reliable_output
`,
			shouldWork: true,
		},
		{
			name: "invalid_config",
			configYAML: `
nomad:
  # Missing address
  
outputs:
  test_output:
    type: stdout

routes:
  - filter: ""
    output: test_output
`,
			shouldWork: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			err := os.WriteFile(configPath, []byte(tt.configYAML), 0644)
			require.NoError(t, err)

			// Load config
			cfg, err := config.LoadConfig(configPath)
			if !tt.shouldWork {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, cfg)

			// Test router creation
			router, err := routing.NewRouter(cfg.Routes)
			require.NoError(t, err)
			assert.NotNil(t, router)

			// Test output manager creation
			outputManager, err := outputs.NewManager(cfg.Outputs, nil)
			require.NoError(t, err)
			assert.NotNil(t, outputManager)
		})
	}
}

func TestEventRoutingIntegration(t *testing.T) {
	// Create a test configuration
	configYAML := `
nomad:
  address: "http://localhost:4646"
  token: ""

outputs:
  all_events:
    type: stdout
    format: json
  node_events:
    type: stdout
    format: text
    text: "Node: {{ .Payload.Node.Name }} ({{ .Type }})"
  job_events:
    type: stdout
    format: text
    text: "Job: {{ .Payload.Job.ID }} ({{ .Type }})"

routes:
  - filter: event.Topic == 'Node'
    output: node_events
    routes:
      - filter: event.Type == 'NodeRegistration'
        output: job_events  # This should trigger for NodeRegistration
  - filter: event.Topic == 'Job'
    continue: false  # Should stop processing catch-all
    routes:
      - filter: event.Type == 'JobRegistered'
        output: job_events
  - filter: ""
    output: all_events  # This catch-all comes last
`

	// Setup
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(configYAML), 0644)
	require.NoError(t, err)

	cfg, err := config.LoadConfig(configPath)
	require.NoError(t, err)

	router, err := routing.NewRouter(cfg.Routes)
	require.NoError(t, err)

	// Test cases
	tests := []struct {
		name            string
		event           nomad.Event
		expectedOutputs []string
	}{
		{
			name: "node_registration_event",
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
			expectedOutputs: []string{"node_events", "job_events", "all_events"},
		},
		{
			name: "job_registered_event", 
			event: nomad.Event{
				Topic: "Job",
				Type:  "JobRegistered",
				Key:   "job-1",
				Index: 2,
				Payload: map[string]interface{}{
					"Job": map[string]interface{}{
						"ID":   "job-1",
						"Name": "example-job",
					},
				},
			},
			expectedOutputs: []string{"job_events"}, // continue: false stops processing catch-all
		},
		{
			name: "allocation_event",
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
			expectedOutputs: []string{"all_events"}, // Only matches catch-all
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

func TestEventProcessingPipeline(t *testing.T) {
	// Create a minimal config for testing the full pipeline
	configYAML := `
nomad:
  address: "http://localhost:4646"
  token: ""

outputs:
  test_output:
    type: stdout
    format: json

routes:
  - filter: event.Topic == 'Job'
    output: test_output
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(configYAML), 0644)
	require.NoError(t, err)

	cfg, err := config.LoadConfig(configPath)
	require.NoError(t, err)

	router, err := routing.NewRouter(cfg.Routes)
	require.NoError(t, err)

	outputManager, err := outputs.NewManager(cfg.Outputs, nil)
	require.NoError(t, err)

	// Test event processing
	testEvent := nomad.Event{
		Topic: "Job",
		Type:  "JobRegistered",
		Key:   "job-1",
		Index: 1,
		Payload: map[string]interface{}{
			"Job": map[string]interface{}{
				"ID":   "job-1",
				"Name": "test-job",
			},
		},
	}

	// Route the event
	matchedOutputs, err := router.Route(testEvent)
	require.NoError(t, err)
	assert.Equal(t, []string{"test_output"}, matchedOutputs)

	// Send to outputs (this would normally write to stdout)
	for _, outputName := range matchedOutputs {
		err := outputManager.Send(outputName, testEvent)
		assert.NoError(t, err)
	}
}

func TestConfigValidationEdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		configYAML string
		errorMsg   string
	}{
		{
			name: "route_without_output_or_children",
			configYAML: `
nomad:
  address: "http://localhost:4646"

outputs:
  test_output:
    type: stdout

routes:
  - filter: "event.Topic == 'Node'"
    # No output or child routes
`,
			errorMsg: "route must have either an output or child routes",
		},
		{
			name: "route_with_invalid_output_reference",
			configYAML: `
nomad:
  address: "http://localhost:4646"

outputs:
  test_output:
    type: stdout

routes:
  - filter: ""
    output: nonexistent_output
`,
			errorMsg: "output \"nonexistent_output\" does not exist",
		},
		{
			name: "invalid_retry_delay_format",
			configYAML: `
nomad:
  address: "http://localhost:4646"

outputs:
  test_output:
    type: stdout
    retry:
      max_retries: 3
      base_delay: "invalid-duration"

routes:
  - filter: ""
    output: test_output
`,
			errorMsg: "invalid base_delay format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			err := os.WriteFile(configPath, []byte(tt.configYAML), 0644)
			require.NoError(t, err)

			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				// Error during config loading
				assert.Contains(t, err.Error(), tt.errorMsg)
				return
			}

			// Error might be during output manager creation
			_, err = outputs.NewManager(cfg.Outputs, nil)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errorMsg)
		})
	}
}

// Helper function to create a test event
func createTestEvent(topic, eventType string, payload interface{}) nomad.Event {
	return nomad.Event{
		Topic:   topic,
		Type:    eventType,
		Key:     "test-key",
		Index:   uint64(time.Now().Unix()),
		Payload: payload,
	}
}

// Test JSON serialization/deserialization of events
func TestEventSerialization(t *testing.T) {
	originalEvent := createTestEvent("Job", "JobRegistered", map[string]interface{}{
		"Job": map[string]interface{}{
			"ID":      "test-job",
			"Name":    "Test Job",
			"Version": 1,
		},
	})

	// Serialize to JSON
	jsonData, err := json.Marshal(originalEvent)
	require.NoError(t, err)

	// Deserialize from JSON
	var deserializedEvent nomad.Event
	err = json.Unmarshal(jsonData, &deserializedEvent)
	require.NoError(t, err)

	// Compare
	assert.Equal(t, originalEvent.Topic, deserializedEvent.Topic)
	assert.Equal(t, originalEvent.Type, deserializedEvent.Type)
	assert.Equal(t, originalEvent.Key, deserializedEvent.Key)
	assert.Equal(t, originalEvent.Index, deserializedEvent.Index)
}
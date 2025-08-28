package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name        string
		configYAML  string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			configYAML: `
nomad:
  address: "http://localhost:4646"
  token: "test-token"

outputs:
  test_stdout:
    type: stdout
  test_slack:
    type: slack
    webhook_url: "https://hooks.slack.com/test"
    channel: "#test"

routes:
  - filter: ""
    output: test_stdout
  - filter: event.Topic == 'Node'
    output: test_slack
`,
			expectError: false,
		},
		{
			name: "missing nomad address",
			configYAML: `
nomad:
  token: "test-token"

outputs:
  test_stdout:
    type: stdout

routes:
  - filter: ""
    output: test_stdout
`,
			expectError: true,
			errorMsg:    "nomad.address is required",
		},
		{
			name: "output without type",
			configYAML: `
nomad:
  address: "http://localhost:4646"

outputs:
  test_invalid:
    webhook_url: "https://example.com"

routes:
  - filter: ""
    output: test_invalid
`,
			expectError: true,
			errorMsg:    "type is required",
		},
		{
			name: "route referencing non-existent output",
			configYAML: `
nomad:
  address: "http://localhost:4646"

outputs:
  test_stdout:
    type: stdout

routes:
  - filter: ""
    output: non_existent
`,
			expectError: true,
			errorMsg:    "output \"non_existent\" does not exist",
		},
		{
			name: "route without output or child routes",
			configYAML: `
nomad:
  address: "http://localhost:4646"

outputs:
  test_stdout:
    type: stdout

routes:
  - filter: ""
`,
			expectError: true,
			errorMsg:    "route must have either an output or child routes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			
			err := os.WriteFile(configPath, []byte(tt.configYAML), 0644)
			require.NoError(t, err)

			config, err := LoadConfig(configPath)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, config)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
				assert.Equal(t, "http://localhost:4646", config.Nomad.Address)
			}
		})
	}
}

func TestLoadConfigFileNotFound(t *testing.T) {
	_, err := LoadConfig("/non/existent/path")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")
	
	err := os.WriteFile(configPath, []byte("invalid: yaml: content: ["), 0644)
	require.NoError(t, err)

	_, err = LoadConfig(configPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse config file")
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected string
	}{
		{
			name: "valid config",
			config: Config{
				Nomad: NomadConfig{Address: "http://localhost:4646"},
				Outputs: map[string]Output{
					"test": {Type: "stdout"},
				},
				Routes: []Route{
					{Filter: "", Output: "test"},
				},
			},
			expected: "",
		},
		{
			name: "missing nomad address",
			config: Config{
				Nomad: NomadConfig{},
			},
			expected: "nomad.address is required",
		},
		{
			name: "output without type",
			config: Config{
				Nomad: NomadConfig{Address: "http://localhost:4646"},
				Outputs: map[string]Output{
					"test": {},
				},
			},
			expected: "type is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()
			if tt.expected == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expected)
			}
		})
	}
}
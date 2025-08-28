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

func TestTLSConfigValidation(t *testing.T) {
	// Create temporary files for testing certificate validation
	tmpDir := t.TempDir()
	
	// Create valid test certificate files
	caCertPath := filepath.Join(tmpDir, "ca.pem")
	clientCertPath := filepath.Join(tmpDir, "client.pem")
	clientKeyPath := filepath.Join(tmpDir, "client-key.pem")
	
	err := os.WriteFile(caCertPath, []byte("dummy ca cert"), 0644)
	require.NoError(t, err)
	
	err = os.WriteFile(clientCertPath, []byte("dummy client cert"), 0644)
	require.NoError(t, err)
	
	err = os.WriteFile(clientKeyPath, []byte("dummy client key"), 0600)
	require.NoError(t, err)

	tests := []struct {
		name     string
		config   Config
		expected string
	}{
		{
			name: "valid TLS config with CA cert",
			config: Config{
				Nomad: NomadConfig{
					Address: "https://localhost:4646",
					TLS: &TLSConfig{
						Enabled: true,
						CACert:  caCertPath,
					},
				},
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
			name: "valid mTLS config",
			config: Config{
				Nomad: NomadConfig{
					Address: "https://localhost:4646",
					TLS: &TLSConfig{
						Enabled:    true,
						CACert:     caCertPath,
						ClientCert: clientCertPath,
						ClientKey:  clientKeyPath,
						ServerName: "nomad.example.com",
					},
				},
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
			name: "TLS disabled - no validation",
			config: Config{
				Nomad: NomadConfig{
					Address: "https://localhost:4646",
					TLS: &TLSConfig{
						Enabled: false,
						CACert:  "/non/existent/path",
					},
				},
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
			name: "missing client key with client cert",
			config: Config{
				Nomad: NomadConfig{
					Address: "https://localhost:4646",
					TLS: &TLSConfig{
						Enabled:    true,
						ClientCert: clientCertPath,
					},
				},
				Outputs: map[string]Output{
					"test": {Type: "stdout"},
				},
				Routes: []Route{
					{Filter: "", Output: "test"},
				},
			},
			expected: "both client_cert and client_key must be provided together",
		},
		{
			name: "missing client cert with client key",
			config: Config{
				Nomad: NomadConfig{
					Address: "https://localhost:4646",
					TLS: &TLSConfig{
						Enabled:   true,
						ClientKey: clientKeyPath,
					},
				},
				Outputs: map[string]Output{
					"test": {Type: "stdout"},
				},
				Routes: []Route{
					{Filter: "", Output: "test"},
				},
			},
			expected: "both client_cert and client_key must be provided together",
		},
		{
			name: "non-existent CA cert file",
			config: Config{
				Nomad: NomadConfig{
					Address: "https://localhost:4646",
					TLS: &TLSConfig{
						Enabled: true,
						CACert:  "/non/existent/ca.pem",
					},
				},
				Outputs: map[string]Output{
					"test": {Type: "stdout"},
				},
				Routes: []Route{
					{Filter: "", Output: "test"},
				},
			},
			expected: "file does not exist: /non/existent/ca.pem",
		},
		{
			name: "non-existent client cert file",
			config: Config{
				Nomad: NomadConfig{
					Address: "https://localhost:4646",
					TLS: &TLSConfig{
						Enabled:    true,
						ClientCert: "/non/existent/client.pem",
						ClientKey:  clientKeyPath,
					},
				},
				Outputs: map[string]Output{
					"test": {Type: "stdout"},
				},
				Routes: []Route{
					{Filter: "", Output: "test"},
				},
			},
			expected: "file does not exist: /non/existent/client.pem",
		},
		{
			name: "non-existent client key file",
			config: Config{
				Nomad: NomadConfig{
					Address: "https://localhost:4646",
					TLS: &TLSConfig{
						Enabled:    true,
						ClientCert: clientCertPath,
						ClientKey:  "/non/existent/client-key.pem",
					},
				},
				Outputs: map[string]Output{
					"test": {Type: "stdout"},
				},
				Routes: []Route{
					{Filter: "", Output: "test"},
				},
			},
			expected: "file does not exist: /non/existent/client-key.pem",
		},
		{
			name: "insecure skip verify enabled",
			config: Config{
				Nomad: NomadConfig{
					Address: "https://localhost:4646",
					TLS: &TLSConfig{
						Enabled:            true,
						InsecureSkipVerify: true,
					},
				},
				Outputs: map[string]Output{
					"test": {Type: "stdout"},
				},
				Routes: []Route{
					{Filter: "", Output: "test"},
				},
			},
			expected: "",
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

package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Nomad   NomadConfig       `yaml:"nomad"`
	Outputs map[string]Output `yaml:"outputs"`
	Routes  []Route           `yaml:"routes"`
}

type NomadConfig struct {
	Address string     `yaml:"address"`
	Token   string     `yaml:"token"`
	TLS     *TLSConfig `yaml:"tls,omitempty"`
}

type TLSConfig struct {
	Enabled            bool   `yaml:"enabled"`
	CACert             string `yaml:"ca_cert,omitempty"`              // Path to CA certificate file
	ClientCert         string `yaml:"client_cert,omitempty"`          // Path to client certificate file
	ClientKey          string `yaml:"client_key,omitempty"`           // Path to client key file
	ServerName         string `yaml:"server_name,omitempty"`          // Override server name for verification
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify,omitempty"` // Skip certificate verification (dev only)
}

type Output struct {
	Type       string                 `yaml:"type"`
	Retry      *RetryConfig           `yaml:"retry,omitempty"`
	Properties map[string]interface{} `yaml:",inline"`
}

type RetryConfig struct {
	MaxRetries int    `yaml:"max_retries"`
	BaseDelay  string `yaml:"base_delay"` // e.g., "1s", "500ms"
}

type Route struct {
	Filter   string  `yaml:"filter"`
	Output   string  `yaml:"output,omitempty"`   // Optional - parent routes can just filter
	Continue *bool   `yaml:"continue,omitempty"` // Optional - defaults to true
	Routes   []Route `yaml:"routes,omitempty"`   // Child routes
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

func (c *Config) validate() error {
	if c.Nomad.Address == "" {
		return fmt.Errorf("nomad.address is required - please specify the Nomad API address (e.g., \"http://localhost:4646\")")
	}

	// Validate TLS configuration
	if err := c.validateTLS(); err != nil {
		return err
	}

	if len(c.Outputs) == 0 {
		return fmt.Errorf("at least one output must be defined - add an output configuration under the 'outputs' section")
	}

	for name, output := range c.Outputs {
		if output.Type == "" {
			return fmt.Errorf("output %q: type is required - specify one of the supported output types", name)
		}
	}

	if len(c.Routes) == 0 {
		return fmt.Errorf("at least one route must be defined - add a route configuration under the 'routes' section")
	}

	for i, route := range c.Routes {
		if err := c.validateRoute(route, fmt.Sprintf("route %d", i)); err != nil {
			return err
		}
	}

	return nil
}

// validateRoute recursively validates a route and its children
func (c *Config) validateRoute(route Route, path string) error {
	// Route must have either an output or child routes (or both)
	if route.Output == "" && len(route.Routes) == 0 {
		return fmt.Errorf("%s: route must have either an output or child routes", path)
	}

	// If output is specified, it must exist
	if route.Output != "" {
		if _, exists := c.Outputs[route.Output]; !exists {
			availableOutputs := make([]string, 0, len(c.Outputs))
			for name := range c.Outputs {
				availableOutputs = append(availableOutputs, name)
			}
			return fmt.Errorf("%s: output %q does not exist - available outputs: %v", path, route.Output, availableOutputs)
		}
	}

	// Validate CEL filter expression if provided
	if route.Filter != "" {
		// Basic validation - check if it's not obviously invalid
		if len(route.Filter) > 1000 {
			return fmt.Errorf("%s: filter expression is too long (max 1000 characters)", path)
		}
	}

	// Recursively validate child routes
	for i, childRoute := range route.Routes {
		childPath := fmt.Sprintf("%s.routes[%d]", path, i)
		if err := c.validateRoute(childRoute, childPath); err != nil {
			return err
		}
	}

	return nil
}

// validateTLS validates TLS configuration
func (c *Config) validateTLS() error {
	if c.Nomad.TLS == nil || !c.Nomad.TLS.Enabled {
		return nil
	}

	tls := c.Nomad.TLS

	// Validate client certificate configuration
	if (tls.ClientCert == "" && tls.ClientKey != "") || (tls.ClientCert != "" && tls.ClientKey == "") {
		return fmt.Errorf("nomad.tls: both client_cert and client_key must be provided together for mutual TLS")
	}

	// Validate certificate files exist if specified
	if tls.CACert != "" {
		if _, err := os.Stat(tls.CACert); os.IsNotExist(err) {
			return fmt.Errorf("nomad.tls.ca_cert: file does not exist: %s", tls.CACert)
		} else if err != nil {
			return fmt.Errorf("nomad.tls.ca_cert: cannot access file %s: %w", tls.CACert, err)
		}
	}

	if tls.ClientCert != "" {
		if _, err := os.Stat(tls.ClientCert); os.IsNotExist(err) {
			return fmt.Errorf("nomad.tls.client_cert: file does not exist: %s", tls.ClientCert)
		} else if err != nil {
			return fmt.Errorf("nomad.tls.client_cert: cannot access file %s: %w", tls.ClientCert, err)
		}
	}

	if tls.ClientKey != "" {
		if _, err := os.Stat(tls.ClientKey); os.IsNotExist(err) {
			return fmt.Errorf("nomad.tls.client_key: file does not exist: %s", tls.ClientKey)
		} else if err != nil {
			return fmt.Errorf("nomad.tls.client_key: cannot access file %s: %w", tls.ClientKey, err)
		}
	}

	return nil
}

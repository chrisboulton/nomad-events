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
	Address string `yaml:"address"`
	Token   string `yaml:"token"`
}

type Output struct {
	Type       string                 `yaml:"type"`
	Properties map[string]interface{} `yaml:",inline"`
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
		return fmt.Errorf("nomad.address is required")
	}

	for name, output := range c.Outputs {
		if output.Type == "" {
			return fmt.Errorf("output %q: type is required", name)
		}
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
			return fmt.Errorf("%s: output %q does not exist", path, route.Output)
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

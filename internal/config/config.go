package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Nomad   NomadConfig          `yaml:"nomad"`
	Outputs map[string]Output    `yaml:"outputs"`
	Routes  []Route              `yaml:"routes"`
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
	Filter string `yaml:"filter"`
	Output string `yaml:"output"`
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
		if route.Output == "" {
			return fmt.Errorf("route %d: output is required", i)
		}
		if _, exists := c.Outputs[route.Output]; !exists {
			return fmt.Errorf("route %d: output %q does not exist", i, route.Output)
		}
	}

	return nil
}
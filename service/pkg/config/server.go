// Package config provides configuration management for monitoring services.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

// ServerConfig represents the server configuration.
type ServerConfig struct {
	// ListenAddress is the address on which the server listens (e.g., ":8080")
	ListenAddress string `yaml:"addr" json:"addr"`

	// MetricsHost is the Victoria Metrics/Prometheus endpoint
	MetricsHost string `yaml:"metricsHost,omitempty" json:"metricsHost,omitempty"`

	// KubernetesHost is the Kubernetes API server endpoint
	KubernetesHost string `yaml:"kubernetesHost,omitempty" json:"kubernetesHost,omitempty"`

	// WhitelistHosts is a comma-separated list of allowed Kubernetes API hosts
	WhitelistHosts string `yaml:"whitelistHosts,omitempty" json:"whitelistHosts,omitempty"`

	// LogLevel controls logging verbosity: debug, info, warn, error
	LogLevel string `yaml:"logLevel,omitempty" json:"logLevel,omitempty"`

	// EnablePprof enables pprof profiling endpoints
	EnablePprof bool `yaml:"enablePprof,omitempty" json:"enablePprof,omitempty"`

	// ReadTimeout is the maximum duration for reading the entire request
	ReadTimeoutSeconds int `yaml:"readTimeoutSeconds,omitempty" json:"readTimeoutSeconds,omitempty"`

	// WriteTimeout is the maximum duration before timing out writes of the response
	WriteTimeoutSeconds int `yaml:"writeTimeoutSeconds,omitempty" json:"writeTimeoutSeconds,omitempty"`
}

// DefaultConfig returns a ServerConfig with sensible defaults.
func DefaultConfig() *ServerConfig {
	return &ServerConfig{
		ListenAddress:       ":8080",
		LogLevel:            "info",
		EnablePprof:         false,
		ReadTimeoutSeconds:  30,
		WriteTimeoutSeconds: 30,
	}
}

// LoadConfig loads server configuration from a YAML file.
// If the file doesn't exist or cannot be read, it returns default configuration.
func LoadConfig(configPath string) (*ServerConfig, error) {
	cfg := DefaultConfig()

	// If no config file specified, return defaults
	if configPath == "" {
		return cfg, nil
	}

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return cfg, nil
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return cfg, nil
}

// Validate checks if the configuration is valid.
func (c *ServerConfig) Validate() error {
	if c.ListenAddress == "" {
		return fmt.Errorf("listen address cannot be empty")
	}

	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}

	if c.LogLevel != "" && !validLogLevels[c.LogLevel] {
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", c.LogLevel)
	}

	return nil
}

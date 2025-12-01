// Package config provides configuration management for monitoring services.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

// ServerConfig represents the server configuration.
type ServerConfig struct {
	// Server settings
	ListenAddress       string `yaml:"addr" json:"addr"`
	LogLevel            string `yaml:"logLevel,omitempty" json:"logLevel,omitempty"`
	EnablePprof         bool   `yaml:"enablePprof,omitempty" json:"enablePprof,omitempty"`
	ReadTimeoutSeconds  int    `yaml:"readTimeoutSeconds,omitempty" json:"readTimeoutSeconds,omitempty"`
	WriteTimeoutSeconds int    `yaml:"writeTimeoutSeconds,omitempty" json:"writeTimeoutSeconds,omitempty"`

	// Metrics configuration
	MetricsHost string        `yaml:"metricsHost,omitempty" json:"metricsHost,omitempty"`
	Metrics     MetricsConfig `yaml:"metrics,omitempty" json:"metrics,omitempty"`

	// Kubernetes configuration
	KubernetesHost string           `yaml:"kubernetesHost,omitempty" json:"kubernetesHost,omitempty"`
	WhitelistHosts string           `yaml:"whitelistHosts,omitempty" json:"whitelistHosts,omitempty"`
	Kubernetes     KubernetesConfig `yaml:"kubernetes,omitempty" json:"kubernetes,omitempty"`

	// Service-specific configuration
	Minio    MinioConfig    `yaml:"minio,omitempty" json:"minio,omitempty"`
	Database DatabaseConfig `yaml:"database,omitempty" json:"database,omitempty"`

	// Performance tuning
	Performance PerformanceConfig `yaml:"performance,omitempty" json:"performance,omitempty"`
}

// MetricsConfig contains metrics client configuration.
type MetricsConfig struct {
	// HTTP client settings
	TimeoutSeconds         int  `yaml:"timeoutSeconds,omitempty" json:"timeoutSeconds,omitempty"`
	MaxIdleConns           int  `yaml:"maxIdleConns,omitempty" json:"maxIdleConns,omitempty"`
	MaxIdleConnsPerHost    int  `yaml:"maxIdleConnsPerHost,omitempty" json:"maxIdleConnsPerHost,omitempty"`
	IdleConnTimeoutSeconds int  `yaml:"idleConnTimeoutSeconds,omitempty" json:"idleConnTimeoutSeconds,omitempty"`
	DialTimeoutSeconds     int  `yaml:"dialTimeoutSeconds,omitempty" json:"dialTimeoutSeconds,omitempty"`
	KeepAliveSeconds       int  `yaml:"keepAliveSeconds,omitempty" json:"keepAliveSeconds,omitempty"`
	TLSHandshakeTimeout    int  `yaml:"tlsHandshakeTimeoutSeconds,omitempty" json:"tlsHandshakeTimeoutSeconds,omitempty"`
	ExpectContinueTimeout  int  `yaml:"expectContinueTimeoutSeconds,omitempty" json:"expectContinueTimeoutSeconds,omitempty"`
	DisableCompression     bool `yaml:"disableCompression,omitempty" json:"disableCompression,omitempty"`
	DisableKeepAlives      bool `yaml:"disableKeepAlives,omitempty" json:"disableKeepAlives,omitempty"`
	MaxConnsPerHost        int  `yaml:"maxConnsPerHost,omitempty" json:"maxConnsPerHost,omitempty"`
}

// KubernetesConfig contains Kubernetes client configuration.
type KubernetesConfig struct {
	ServiceHost     string `yaml:"serviceHost,omitempty" json:"serviceHost,omitempty"`
	ServicePort     string `yaml:"servicePort,omitempty" json:"servicePort,omitempty"`
	WhitelistHosts  string `yaml:"whitelistHosts,omitempty" json:"whitelistHosts,omitempty"`
	CacheTTLMinutes int    `yaml:"cacheTTLMinutes,omitempty" json:"cacheTTLMinutes,omitempty"`
}

// MinioConfig contains Minio-specific configuration.
type MinioConfig struct {
	Instance string `yaml:"instance,omitempty" json:"instance,omitempty"`
}

// DatabaseConfig contains Database-specific configuration.
type DatabaseConfig struct {
	// Add database-specific settings if needed
}

// PerformanceConfig contains performance tuning settings.
type PerformanceConfig struct {
	// Response cache settings
	EnableResponseCache  bool `yaml:"enableResponseCache,omitempty" json:"enableResponseCache,omitempty"`
	ResponseCacheTTL     int  `yaml:"responseCacheTTLSeconds,omitempty" json:"responseCacheTTLSeconds,omitempty"`
	ResponseCacheMaxSize int  `yaml:"responseCacheMaxSize,omitempty" json:"responseCacheMaxSize,omitempty"`

	// Query optimization
	EnableQueryCache bool `yaml:"enableQueryCache,omitempty" json:"enableQueryCache,omitempty"`
	QueryCacheTTL    int  `yaml:"queryCacheTTLSeconds,omitempty" json:"queryCacheTTLSeconds,omitempty"`

	// Buffer pool settings
	ReadBufferSize  int `yaml:"readBufferSize,omitempty" json:"readBufferSize,omitempty"`
	WriteBufferSize int `yaml:"writeBufferSize,omitempty" json:"writeBufferSize,omitempty"`
}

// DefaultConfig returns a ServerConfig with sensible defaults.
func DefaultConfig() *ServerConfig {
	return &ServerConfig{
		ListenAddress:       ":8080",
		LogLevel:            "info",
		EnablePprof:         false,
		ReadTimeoutSeconds:  30,
		WriteTimeoutSeconds: 30,
		Metrics: MetricsConfig{
			TimeoutSeconds:         30,
			MaxIdleConns:           100,
			MaxIdleConnsPerHost:    10,
			IdleConnTimeoutSeconds: 90,
			DialTimeoutSeconds:     30,
			KeepAliveSeconds:       30,
			TLSHandshakeTimeout:    10,
			ExpectContinueTimeout:  1,
			DisableCompression:     true,
			DisableKeepAlives:      false,
			MaxConnsPerHost:        0, // 0 means no limit
		},
		Kubernetes: KubernetesConfig{
			CacheTTLMinutes: 5,
		},
		Minio: MinioConfig{
			Instance: "object-storage.objectstorage-system.svc.cluster.local:80",
		},
		Performance: PerformanceConfig{
			EnableResponseCache:  false,
			ResponseCacheTTL:     60,
			ResponseCacheMaxSize: 1000,
			EnableQueryCache:     false,
			QueryCacheTTL:        300,
			ReadBufferSize:       4096,
			WriteBufferSize:      4096,
		},
	}
}

// GetMetricsTimeout returns the metrics timeout as a time.Duration.
func (c *ServerConfig) GetMetricsTimeout() time.Duration {
	if c.Metrics.TimeoutSeconds <= 0 {
		return 30 * time.Second
	}
	return time.Duration(c.Metrics.TimeoutSeconds) * time.Second
}

// GetMetricsDialTimeout returns the dial timeout as a time.Duration.
func (c *ServerConfig) GetMetricsDialTimeout() time.Duration {
	if c.Metrics.DialTimeoutSeconds <= 0 {
		return 30 * time.Second
	}
	return time.Duration(c.Metrics.DialTimeoutSeconds) * time.Second
}

// GetMetricsKeepAlive returns the keep-alive duration.
func (c *ServerConfig) GetMetricsKeepAlive() time.Duration {
	if c.Metrics.KeepAliveSeconds <= 0 {
		return 30 * time.Second
	}
	return time.Duration(c.Metrics.KeepAliveSeconds) * time.Second
}

// GetMetricsIdleConnTimeout returns the idle connection timeout.
func (c *ServerConfig) GetMetricsIdleConnTimeout() time.Duration {
	if c.Metrics.IdleConnTimeoutSeconds <= 0 {
		return 90 * time.Second
	}
	return time.Duration(c.Metrics.IdleConnTimeoutSeconds) * time.Second
}

// GetMetricsTLSHandshakeTimeout returns the TLS handshake timeout.
func (c *ServerConfig) GetMetricsTLSHandshakeTimeout() time.Duration {
	if c.Metrics.TLSHandshakeTimeout <= 0 {
		return 10 * time.Second
	}
	return time.Duration(c.Metrics.TLSHandshakeTimeout) * time.Second
}

// GetMetricsExpectContinueTimeout returns the expect-continue timeout.
func (c *ServerConfig) GetMetricsExpectContinueTimeout() time.Duration {
	if c.Metrics.ExpectContinueTimeout <= 0 {
		return 1 * time.Second
	}
	return time.Duration(c.Metrics.ExpectContinueTimeout) * time.Second
}

// GetKubernetesCacheTTL returns the Kubernetes cache TTL.
func (c *ServerConfig) GetKubernetesCacheTTL() time.Duration {
	if c.Kubernetes.CacheTTLMinutes <= 0 {
		return 5 * time.Minute
	}
	return time.Duration(c.Kubernetes.CacheTTLMinutes) * time.Minute
}

// GetResponseCacheTTL returns the response cache TTL.
func (c *ServerConfig) GetResponseCacheTTL() time.Duration {
	if c.Performance.ResponseCacheTTL <= 0 {
		return 60 * time.Second
	}
	return time.Duration(c.Performance.ResponseCacheTTL) * time.Second
}

// GetQueryCacheTTL returns the query cache TTL.
func (c *ServerConfig) GetQueryCacheTTL() time.Duration {
	if c.Performance.QueryCacheTTL <= 0 {
		return 300 * time.Second
	}
	return time.Duration(c.Performance.QueryCacheTTL) * time.Second
}

// LoadConfig loads server configuration from a YAML file.
// If the file doesn't exist or cannot be read, it returns default configuration.
// Environment variables can override configuration values.
func LoadConfig(configPath string) (*ServerConfig, error) {
	cfg := DefaultConfig()

	// If config file specified and exists, load it
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			data, err := os.ReadFile(configPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}

			// Parse YAML
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("failed to parse config file: %w", err)
			}
		}
	}

	// Override with environment variables if present
	cfg.applyEnvOverrides()

	return cfg, nil
}

// applyEnvOverrides applies environment variable overrides to the configuration.
func (c *ServerConfig) applyEnvOverrides() {
	// Kubernetes overrides
	if host := os.Getenv("KUBERNETES_SERVICE_HOST"); host != "" {
		c.Kubernetes.ServiceHost = host
	}
	if port := os.Getenv("KUBERNETES_SERVICE_PORT"); port != "" {
		c.Kubernetes.ServicePort = port
	}
	if whitelist := os.Getenv("WHITELIST_KUBERNETES_HOSTS"); whitelist != "" {
		c.Kubernetes.WhitelistHosts = whitelist
		c.WhitelistHosts = whitelist // Backward compatibility
	}

	// Minio overrides
	if instance := os.Getenv("OBJECT_STORAGE_INSTANCE"); instance != "" {
		c.Minio.Instance = instance
	}

	// Metrics host override
	if metricsHost := os.Getenv("METRICS_HOST"); metricsHost != "" {
		c.MetricsHost = metricsHost
	}
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

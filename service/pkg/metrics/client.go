// Package metrics provides high-performance clients for querying Victoria Metrics and Prometheus.
package metrics

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/labring/sealos/service/pkg/api"
	"github.com/labring/sealos/service/pkg/config"
)

const (
	defaultTimeout         = 30 * time.Second
	defaultMaxIdleConns    = 100
	defaultMaxConnsPerHost = 10
	defaultIdleConnTimeout = 90 * time.Second
)

// Client is a high-performance HTTP client for querying metrics endpoints.
// It uses connection pooling and keep-alive for optimal performance.
type Client struct {
	httpClient *http.Client
	baseURL    string
	mu         sync.RWMutex
}

// NewClient creates a new metrics client with default connection pooling.
func NewClient(baseURL string) *Client {
	return NewClientWithConfig(baseURL, nil)
}

// NewClientWithConfig creates a new metrics client with custom configuration.
func NewClientWithConfig(baseURL string, cfg *config.ServerConfig) *Client {
	// Use defaults if no config provided
	var (
		timeout               = defaultTimeout
		maxIdleConns          = defaultMaxIdleConns
		maxIdleConnsPerHost   = defaultMaxConnsPerHost
		idleConnTimeout       = defaultIdleConnTimeout
		dialTimeout           = 30 * time.Second
		keepAlive             = 30 * time.Second
		tlsHandshakeTimeout   = 10 * time.Second
		expectContinueTimeout = 1 * time.Second
		disableCompression    = true
		disableKeepAlives     = false
		maxConnsPerHost       = 0
	)

	// Apply config if provided
	if cfg != nil {
		timeout = cfg.GetMetricsTimeout()
		maxIdleConns = cfg.Metrics.MaxIdleConns
		if maxIdleConns <= 0 {
			maxIdleConns = defaultMaxIdleConns
		}
		maxIdleConnsPerHost = cfg.Metrics.MaxIdleConnsPerHost
		if maxIdleConnsPerHost <= 0 {
			maxIdleConnsPerHost = defaultMaxConnsPerHost
		}
		idleConnTimeout = cfg.GetMetricsIdleConnTimeout()
		dialTimeout = cfg.GetMetricsDialTimeout()
		keepAlive = cfg.GetMetricsKeepAlive()
		tlsHandshakeTimeout = cfg.GetMetricsTLSHandshakeTimeout()
		expectContinueTimeout = cfg.GetMetricsExpectContinueTimeout()
		disableCompression = cfg.Metrics.DisableCompression
		disableKeepAlives = cfg.Metrics.DisableKeepAlives
		maxConnsPerHost = cfg.Metrics.MaxConnsPerHost
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   dialTimeout,
			KeepAlive: keepAlive,
		}).DialContext,
		MaxIdleConns:          maxIdleConns,
		MaxIdleConnsPerHost:   maxIdleConnsPerHost,
		IdleConnTimeout:       idleConnTimeout,
		TLSHandshakeTimeout:   tlsHandshakeTimeout,
		ExpectContinueTimeout: expectContinueTimeout,
		DisableCompression:    disableCompression,
		DisableKeepAlives:     disableKeepAlives,
		MaxConnsPerHost:       maxConnsPerHost,
		ForceAttemptHTTP2:     true,
		// Use buffer pool for better memory efficiency
		WriteBufferSize: getBufferSize(cfg, true),
		ReadBufferSize:  getBufferSize(cfg, false),
	}

	return &Client{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
		baseURL: baseURL,
	}
}

// getBufferSize returns the buffer size from config or default.
func getBufferSize(cfg *config.ServerConfig, write bool) int {
	if cfg == nil {
		return 4096
	}
	if write {
		if cfg.Performance.WriteBufferSize > 0 {
			return cfg.Performance.WriteBufferSize
		}
	} else {
		if cfg.Performance.ReadBufferSize > 0 {
			return cfg.Performance.ReadBufferSize
		}
	}
	return 4096
}

// Query executes an instant query against the metrics endpoint.
// endpoint should be one of: "/api/v1/query" or "/api/v1/query_range"
func (c *Client) Query(ctx context.Context, endpoint string, query string, timeRange *api.TimeRange) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Build form data
	formData := url.Values{}
	formData.Set("query", query)

	// Add time range parameters
	if timeRange != nil {
		if timeRange.IsInstantQuery() {
			// Instant query
			if timeRange.Time != "" {
				formData.Set("time", timeRange.Time)
			}
		} else {
			// Range query
			formData.Set("start", timeRange.Start)
			formData.Set("end", timeRange.End)
			if timeRange.Step != "" {
				formData.Set("step", timeRange.Step)
			}
		}
	}

	// Create request
	reqURL := c.baseURL + endpoint
	body := bytes.NewBufferString(formData.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("metrics server returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Read response body
	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return result, nil
}

// QueryInstant executes an instant query.
func (c *Client) QueryInstant(ctx context.Context, query string, timestamp string) ([]byte, error) {
	timeRange := &api.TimeRange{
		Time: timestamp,
	}
	return c.Query(ctx, "/select/0/prometheus/", query, timeRange)
}

// QueryRange executes a range query.
func (c *Client) QueryRange(ctx context.Context, query string, start, end, step string) ([]byte, error) {
	timeRange := &api.TimeRange{
		Start: start,
		End:   end,
		Step:  step,
	}
	return c.Query(ctx, "/select/0/prometheus/", query, timeRange)
}

// Close closes idle connections in the client's connection pool.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.httpClient.CloseIdleConnections()
}

// SetTimeout updates the client timeout.
func (c *Client) SetTimeout(timeout time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.httpClient.Timeout = timeout
}

// ClientPool manages a pool of metrics clients for different endpoints.
// This allows reusing connections across different metrics hosts.
type ClientPool struct {
	clients sync.Map // map[string]*Client
}

// NewClientPool creates a new client pool.
func NewClientPool() *ClientPool {
	return &ClientPool{}
}

// GetClient returns a client for the given base URL, creating one if it doesn't exist.
func (p *ClientPool) GetClient(baseURL string) *Client {
	if client, ok := p.clients.Load(baseURL); ok {
		return client.(*Client)
	}

	client := NewClient(baseURL)
	p.clients.Store(baseURL, client)
	return client
}

// Close closes all clients in the pool.
func (p *ClientPool) Close() {
	p.clients.Range(func(key, value interface{}) bool {
		if client, ok := value.(*Client); ok {
			client.Close()
		}
		return true
	})
}

// Global client pool
var globalPool = NewClientPool()

// GetGlobalClient returns a client from the global pool.
func GetGlobalClient(baseURL string) *Client {
	return globalPool.GetClient(baseURL)
}

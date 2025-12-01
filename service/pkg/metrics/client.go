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

// NewClient creates a new metrics client with connection pooling.
func NewClient(baseURL string) *Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          defaultMaxIdleConns,
		MaxIdleConnsPerHost:   defaultMaxConnsPerHost,
		IdleConnTimeout:       defaultIdleConnTimeout,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		// Enable HTTP/2
		ForceAttemptHTTP2: true,
		// Disable compression to save CPU (metrics are already compact)
		DisableCompression: true,
	}

	return &Client{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   defaultTimeout,
		},
		baseURL: baseURL,
	}
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
	return c.Query(ctx, "/api/v1/query", query, timeRange)
}

// QueryRange executes a range query.
func (c *Client) QueryRange(ctx context.Context, query string, start, end, step string) ([]byte, error) {
	timeRange := &api.TimeRange{
		Start: start,
		End:   end,
		Step:  step,
	}
	return c.Query(ctx, "/api/v1/query_range", query, timeRange)
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

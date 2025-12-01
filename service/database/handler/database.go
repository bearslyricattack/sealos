// Package handler provides Database-specific query handlers.
package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/labring/sealos/service/pkg/api"
	"github.com/labring/sealos/service/pkg/config"
	"github.com/labring/sealos/service/pkg/metrics"
)

// DatabaseHandler handles database metric queries.
type DatabaseHandler struct {
	metricsHost string
	client      *metrics.Client
}

// NewDatabaseHandler creates a new DatabaseHandler.
func NewDatabaseHandler(cfg *config.ServerConfig) (*DatabaseHandler, error) {
	if len(cfg.MetricsHost) == 0 {
		return nil, api.ErrNoMetricsHost
	}
	metricsHost := cfg.MetricsHost

	// Create metrics client with connection pooling
	client := metrics.NewClient(metricsHost)

	return &DatabaseHandler{
		metricsHost: metricsHost,
		client:      client,
	}, nil
}

// HandleQuery handles a database query request.
func (h *DatabaseHandler) HandleQuery(c *gin.Context) (interface{}, error) {
	// Parse request
	req := &api.DatabaseRequest{}

	// Bind form parameters
	req.Namespace = c.PostForm("namespace")
	req.Type = c.PostForm("type")
	req.Query = c.PostForm("query")
	req.Cluster = c.PostForm("app")
	req.Range.Start = c.PostForm("start")
	req.Range.End = c.PostForm("end")
	req.Range.Step = c.PostForm("step")
	req.Range.Time = c.PostForm("time")

	// Validate required parameters
	if req.Namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	if req.Type == "" {
		return nil, fmt.Errorf("type (database type) is required")
	}
	if req.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	// Build query
	query, err := h.buildQuery(req)
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	// Execute query
	ctx := c.Request.Context()
	var result []byte

	if req.Range.IsInstantQuery() {
		result, err = h.client.QueryInstant(ctx, query, req.Range.Time)
	} else {
		result, err = h.client.QueryRange(ctx, query, req.Range.Start, req.Range.End, req.Range.Step)
	}

	if err != nil {
		return nil, fmt.Errorf("metrics query failed: %w", err)
	}

	return result, nil
}

// buildQuery constructs a PromQL query for database metrics.
func (h *DatabaseHandler) buildQuery(req *api.DatabaseRequest) (string, error) {
	// Try to get predefined query template
	query, ok := metrics.BuildQuery(req.Type, req.Query, req.Namespace, req.Cluster)
	if !ok {
		// If not a predefined query, treat it as a custom PromQL expression
		// Still replace template variables
		query = replaceTemplateVars(req.Query, req.Namespace, req.Cluster)
	}

	return query, nil
}

// replaceTemplateVars replaces template variables in custom queries.
func replaceTemplateVars(query, namespace, cluster string) string {
	result := query
	// Simple replacement
	for i := 0; i < len(result); i++ {
		if result[i] == '#' {
			result = result[:i] + namespace + result[i+1:]
			i += len(namespace) - 1
		} else if result[i] == '@' {
			result = result[:i] + cluster + result[i+1:]
			i += len(cluster) - 1
		}
	}
	return result
}

// Close closes the handler's resources.
func (h *DatabaseHandler) Close() {
	if h.client != nil {
		h.client.Close()
	}
}

// GetMetricsHost returns the metrics host URL.
func (h *DatabaseHandler) GetMetricsHost() string {
	return h.metricsHost
}

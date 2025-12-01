// Package handler provides Launchpad-specific query handlers.
package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/sealos/service/pkg/api"
	"github.com/labring/sealos/service/pkg/config"
	"github.com/labring/sealos/service/pkg/metrics"
)

// LaunchpadHandler handles Launchpad metric queries.
type LaunchpadHandler struct {
	metricsHost string
	client      *metrics.Client
}

// NewLaunchpadHandler creates a new LaunchpadHandler.
func NewLaunchpadHandler(cfg *config.ServerConfig) (*LaunchpadHandler, error) {
	if len(cfg.MetricsHost) == 0 {
		return nil, api.ErrNoMetricsHost
	}
	metricsHost := cfg.MetricsHost

	// Create metrics client with configuration-based connection pooling
	client := metrics.NewClientWithConfig(metricsHost, cfg)

	return &LaunchpadHandler{
		metricsHost: metricsHost,
		client:      client,
	}, nil
}

// HandleQuery handles a Launchpad query request.
func (h *LaunchpadHandler) HandleQuery(c *gin.Context) (interface{}, error) {
	// Parse request
	req := &api.LaunchpadRequest{}

	// Bind form parameters
	req.Namespace = c.PostForm("namespace")
	req.LaunchPadName = c.PostForm("launchPadName")
	req.Type = c.PostForm("type")
	req.PvcName = c.PostForm("pvcName")
	req.Range.Start = c.PostForm("start")
	req.Range.End = c.PostForm("end")
	req.Range.Step = c.PostForm("step")
	req.Range.Time = c.PostForm("time")

	// Validate required parameters
	if req.Namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	if req.LaunchPadName == "" {
		return nil, fmt.Errorf("launchPadName is required")
	}
	if req.Type == "" {
		return nil, fmt.Errorf("type is required")
	}

	// Build query
	query, err := h.buildQuery(req)
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	// Determine endpoint and execute query
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

// buildQuery constructs a PromQL query for Launchpad metrics.
func (h *LaunchpadHandler) buildQuery(req *api.LaunchpadRequest) (string, error) {
	// Get query template
	query, ok := metrics.BuildQuery("launchpad", req.Type, req.Namespace, req.LaunchPadName)
	if !ok {
		return "", fmt.Errorf("unsupported query type: %s", req.Type)
	}

	// Special handling for storage queries (PVC replacement)
	if req.Type == "storage" {
		if req.PvcName == "" {
			return "", fmt.Errorf("pvcName is required for storage queries")
		}
		query = strings.ReplaceAll(query, "$PVC", req.PvcName)
	}

	return query, nil
}

// Close closes the handler's resources.
func (h *LaunchpadHandler) Close() {
	if h.client != nil {
		h.client.Close()
	}
}

// GetMetricsHost returns the metrics host URL.
func (h *LaunchpadHandler) GetMetricsHost() string {
	return h.metricsHost
}

// SetContext allows injecting a custom context (for testing).
func (h *LaunchpadHandler) SetContext(ctx context.Context) {
	// No-op for now, but can be extended for testing
}

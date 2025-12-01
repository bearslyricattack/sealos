// Package handler provides Minio-specific query handlers.
package handler

import (
	"fmt"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/sealos/service/pkg/api"
	"github.com/labring/sealos/service/pkg/config"
	"github.com/labring/sealos/service/pkg/metrics"
)

// MinioHandler handles Minio metric queries.
type MinioHandler struct {
	metricsHost   string
	minioInstance string
	client        *metrics.Client
}

// NewMinioHandler creates a new MinioHandler.
func NewMinioHandler(cfg *config.ServerConfig) (*MinioHandler, error) {
	if len(cfg.MetricsHost) == 0 {
		return nil, api.ErrNoMetricsHost
	}
	metricsHost := cfg.MetricsHost

	// Get Minio instance identifier
	minioInstance := os.Getenv("OBJECT_STORAGE_INSTANCE")
	if minioInstance == "" {
		minioInstance = "object-storage.objectstorage-system.svc.cluster.local:80"
	}

	// Create metrics client with connection pooling
	client := metrics.NewClient(metricsHost)

	return &MinioHandler{
		metricsHost:   metricsHost,
		minioInstance: minioInstance,
		client:        client,
	}, nil
}

// HandleQuery handles a Minio query request.
func (h *MinioHandler) HandleQuery(c *gin.Context) (interface{}, error) {
	// Parse request
	req := &api.MinioRequest{}

	// Bind form parameters
	req.Namespace = c.PostForm("namespace")
	req.Query = c.PostForm("query")
	req.Bucket = c.PostForm("app") // Bucket name is passed as "app" parameter
	req.Range.Start = c.PostForm("start")
	req.Range.End = c.PostForm("end")
	req.Range.Step = c.PostForm("step")
	req.Range.Time = c.PostForm("time")

	// Validate required parameters
	if req.Namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	if req.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if req.Bucket == "" {
		return nil, fmt.Errorf("app (bucket name) is required")
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

// buildQuery constructs a PromQL query for Minio metrics.
func (h *MinioHandler) buildQuery(req *api.MinioRequest) (string, error) {
	// Get query template from minio queries
	// For Minio, the template uses @ for bucket and # for instance
	query, ok := metrics.BuildQuery("minio", req.Query, h.minioInstance, req.Bucket)
	if !ok {
		// If not a predefined query, treat it as a custom PromQL expression
		query = req.Query
		query = h.replaceMinioVars(query, req.Bucket)
	}

	return query, nil
}

// replaceMinioVars replaces Minio-specific template variables.
// For Minio: @ = bucket name, # = instance
func (h *MinioHandler) replaceMinioVars(query, bucket string) string {
	// Replace @ with bucket name
	query = strings.ReplaceAll(query, "@", bucket)
	// Replace # with instance
	query = strings.ReplaceAll(query, "#", h.minioInstance)
	return query
}

// Close closes the handler's resources.
func (h *MinioHandler) Close() {
	if h.client != nil {
		h.client.Close()
	}
}

// GetMetricsHost returns the metrics host URL.
func (h *MinioHandler) GetMetricsHost() string {
	return h.metricsHost
}

// GetMinioInstance returns the configured Minio instance identifier.
func (h *MinioHandler) GetMinioInstance() string {
	return h.minioInstance
}

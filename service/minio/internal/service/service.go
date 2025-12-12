package service

import (
	"context"
	"fmt"

	"github.com/labring/sealos/service/minio/internal/query"
	"github.com/labring/sealos/service/pkg/api"
	"github.com/labring/sealos/service/pkg/config"
	"github.com/labring/sealos/service/pkg/metrics"
)

// MinioService handles business logic for minio metrics
type MinioService struct {
	metricsHost   string
	minioInstance string
	client        *metrics.Client
	queryBuilder  *query.Builder
	queryExecutor *query.Executor
}

// NewMinioService creates a new MinioService
func NewMinioService(cfg *config.ServerConfig) (*MinioService, error) {
	if len(cfg.MetricsHost) == 0 {
		return nil, api.ErrNoMetricsHost
	}

	// Get Minio instance from config
	minioInstance := cfg.Minio.Instance
	if minioInstance == "" {
		minioInstance = "object-storage.objectstorage-system.svc.cluster.local:80"
	}

	// Create metrics client
	client := metrics.NewClientWithConfig(cfg.MetricsHost, cfg)

	// Create query components
	builder := query.NewBuilder(minioInstance)
	executor := query.NewExecutor(client)

	return &MinioService{
		metricsHost:   cfg.MetricsHost,
		minioInstance: minioInstance,
		client:        client,
		queryBuilder:  builder,
		queryExecutor: executor,
	}, nil
}

// ExecuteQuery executes a minio metrics query
func (s *MinioService) ExecuteQuery(ctx context.Context, req *api.MinioRequest) ([]byte, error) {
	// Build query
	promQL, err := s.queryBuilder.Build(req)
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	// Execute query
	result, err := s.queryExecutor.Execute(ctx, promQL, req.Range)
	if err != nil {
		return nil, fmt.Errorf("metrics query failed: %w", err)
	}

	return result, nil
}

// Close releases resources
func (s *MinioService) Close() {
	if s.client != nil {
		s.client.Close()
	}
}

// GetMetricsHost returns the metrics host URL
func (s *MinioService) GetMetricsHost() string {
	return s.metricsHost
}

// GetMinioInstance returns the configured Minio instance identifier
func (s *MinioService) GetMinioInstance() string {
	return s.minioInstance
}

package service

import (
	"context"
	"fmt"

	"github.com/labring/sealos/service/devboxmonitor/internal/query"
	"github.com/labring/sealos/service/pkg/api"
	"github.com/labring/sealos/service/pkg/config"
	"github.com/labring/sealos/service/pkg/metrics"
)

// DevboxService handles business logic for devbox metrics
type DevboxService struct {
	metricsHost   string
	client        *metrics.Client
	queryBuilder  *query.Builder
	queryExecutor *query.Executor
}

// NewDevboxService creates a new DevboxService
func NewDevboxService(cfg *config.ServerConfig) (*DevboxService, error) {
	if len(cfg.MetricsHost) == 0 {
		return nil, api.ErrNoMetricsHost
	}

	// Create metrics client
	client := metrics.NewClientWithConfig(cfg.MetricsHost, cfg)

	// Create query components
	builder := query.NewBuilder()
	executor := query.NewExecutor(client)

	return &DevboxService{
		metricsHost:   cfg.MetricsHost,
		client:        client,
		queryBuilder:  builder,
		queryExecutor: executor,
	}, nil
}

// ExecuteQuery executes a devbox metrics query
func (s *DevboxService) ExecuteQuery(ctx context.Context, req *api.DevboxRequest) ([]byte, error) {
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
func (s *DevboxService) Close() {
	if s.client != nil {
		s.client.Close()
	}
}

// GetMetricsHost returns the metrics host URL
func (s *DevboxService) GetMetricsHost() string {
	return s.metricsHost
}

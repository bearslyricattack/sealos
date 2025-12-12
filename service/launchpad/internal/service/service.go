package service

import (
	"context"
	"fmt"

	"github.com/labring/sealos/service/launchpad/internal/query"
	"github.com/labring/sealos/service/pkg/api"
	"github.com/labring/sealos/service/pkg/config"
	"github.com/labring/sealos/service/pkg/metrics"
)

// LaunchpadService handles business logic for launchpad metrics
type LaunchpadService struct {
	metricsHost   string
	client        *metrics.Client
	queryBuilder  *query.Builder
	queryExecutor *query.Executor
}

// NewLaunchpadService creates a new LaunchpadService
func NewLaunchpadService(cfg *config.ServerConfig) (*LaunchpadService, error) {
	if len(cfg.MetricsHost) == 0 {
		return nil, api.ErrNoMetricsHost
	}

	// Create metrics client
	client := metrics.NewClientWithConfig(cfg.MetricsHost, cfg)

	// Create query components
	builder := query.NewBuilder()
	executor := query.NewExecutor(client)

	return &LaunchpadService{
		metricsHost:   cfg.MetricsHost,
		client:        client,
		queryBuilder:  builder,
		queryExecutor: executor,
	}, nil
}

// ExecuteQuery executes a launchpad metrics query
func (s *LaunchpadService) ExecuteQuery(ctx context.Context, req *api.LaunchpadRequest) ([]byte, error) {
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
func (s *LaunchpadService) Close() {
	if s.client != nil {
		s.client.Close()
	}
}

// GetMetricsHost returns the metrics host URL
func (s *LaunchpadService) GetMetricsHost() string {
	return s.metricsHost
}

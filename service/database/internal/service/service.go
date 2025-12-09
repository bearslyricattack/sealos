package service

import (
	"context"
	"fmt"

	"github.com/labring/sealos/service/database/internal/query"
	"github.com/labring/sealos/service/pkg/api"
	"github.com/labring/sealos/service/pkg/config"
	"github.com/labring/sealos/service/pkg/metrics"
)

// DatabaseService handles business logic for database metrics
type DatabaseService struct {
	metricsHost   string
	client        *metrics.Client
	queryBuilder  *query.Builder
	queryExecutor *query.Executor
}

// NewDatabaseService creates a new DatabaseService
func NewDatabaseService(cfg *config.ServerConfig) (*DatabaseService, error) {
	if len(cfg.MetricsHost) == 0 {
		return nil, api.ErrNoMetricsHost
	}

	// Create metrics client
	client := metrics.NewClientWithConfig(cfg.MetricsHost, cfg)

	// Create query components
	builder := query.NewBuilder()
	executor := query.NewExecutor(client)

	return &DatabaseService{
		metricsHost:   cfg.MetricsHost,
		client:        client,
		queryBuilder:  builder,
		queryExecutor: executor,
	}, nil
}

// ExecuteQuery executes a database metrics query
func (s *DatabaseService) ExecuteQuery(ctx context.Context, req *api.DatabaseRequest) ([]byte, error) {
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
func (s *DatabaseService) Close() {
	if s.client != nil {
		s.client.Close()
	}
}

// GetMetricsHost returns the metrics host URL
func (s *DatabaseService) GetMetricsHost() string {
	return s.metricsHost
}

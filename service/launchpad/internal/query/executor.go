package query

import (
	"context"

	"github.com/labring/sealos/service/pkg/api"
	"github.com/labring/sealos/service/pkg/metrics"
)

// Executor executes PromQL queries
type Executor struct {
	client *metrics.Client
}

// NewExecutor creates a new query executor
func NewExecutor(client *metrics.Client) *Executor {
	return &Executor{
		client: client,
	}
}

// Execute runs a PromQL query
func (e *Executor) Execute(ctx context.Context, query string, timeRange api.TimeRange) ([]byte, error) {
	if timeRange.IsInstantQuery() {
		return e.client.QueryInstant(ctx, query, timeRange.Time)
	}
	return e.client.QueryRange(ctx, query, timeRange.Start, timeRange.End, timeRange.Step)
}

package query

import (
	"strings"

	"github.com/labring/sealos/service/pkg/api"
	"github.com/labring/sealos/service/pkg/metrics"
)

// Builder constructs PromQL queries
type Builder struct{}

// NewBuilder creates a new query builder
func NewBuilder() *Builder {
	return &Builder{}
}

// Build constructs a PromQL query from request
func (b *Builder) Build(req *api.DatabaseRequest) (string, error) {
	// Try predefined query template
	query, ok := metrics.BuildQuery(req.Type, req.Query, req.Namespace, req.Cluster)
	if !ok {
		// Use custom query with variable replacement
		query = b.replaceVariables(req.Query, req.Namespace, req.Cluster)
	}

	return query, nil
}

// replaceVariables replaces template variables in query
func (b *Builder) replaceVariables(query, namespace, cluster string) string {
	replacer := strings.NewReplacer(
		"#", namespace,
		"@", cluster,
	)
	return replacer.Replace(query)
}

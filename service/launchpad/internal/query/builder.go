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
func (b *Builder) Build(req *api.LaunchpadRequest) (string, error) {
	// Try predefined query template
	query, ok := metrics.BuildQuery("launchpad", req.Type)
	if !ok {
		// Use custom query with variable replacement
		query = b.replaceVariables(req.Type, req.Namespace, req.LaunchPadName, req.PvcName)
	} else {
		// For storage queries, replace $PVC placeholder with actual PVC name
		if req.Type == "storage" && req.PvcName != "" {
			query = strings.ReplaceAll(query, "$PVC", req.PvcName)
		}
	}

	return query, nil
}

// replaceVariables replaces template variables in query
func (b *Builder) replaceVariables(query, namespace, launchPadName, pvcName string) string {

	replacer := strings.NewReplacer(
		"#", namespace,
		"@", launchPadName,
		"$PVC", pvcName,
	)
	return replacer.Replace(query)
}

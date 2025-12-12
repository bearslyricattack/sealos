package query

import (
	"strings"

	"github.com/labring/sealos/service/pkg/api"
	"github.com/labring/sealos/service/pkg/metrics"
)

// Builder constructs PromQL queries
type Builder struct {
	minioInstance string
}

// NewBuilder creates a new query builder
func NewBuilder(minioInstance string) *Builder {
	return &Builder{
		minioInstance: minioInstance,
	}
}

// Build constructs a PromQL query from request
func (b *Builder) Build(req *api.MinioRequest) (string, error) {
	// Try predefined query template
	// For Minio: # = instance, @ = bucket
	query, ok := metrics.BuildQuery("minio", req.Query, b.minioInstance, req.Bucket)
	if !ok {
		// Use custom query with variable replacement
		query = b.replaceVariables(req.Query, req.Bucket)
	}

	return query, nil
}

// replaceVariables replaces template variables in query
// For Minio: # = instance, @ = bucket
func (b *Builder) replaceVariables(query, bucket string) string {
	replacer := strings.NewReplacer(
		"#", b.minioInstance,
		"@", bucket,
	)
	return replacer.Replace(query)
}

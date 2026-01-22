package query

import (
	"fmt"
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

func (b *Builder) Build(req *api.LaunchpadRequest) (string, error) {
	// Try predefined query template
	query, ok := metrics.BuildQuery("launchpad", req.Type)
	if ok {
		query = b.replaceVariables(query, req.Type, req.Namespace, req.LaunchPadName, req.PvcName, req.Service, req.Port)
		fmt.Printf(req.Service)
		fmt.Printf(req.Port)
	} else {
		// For storage queries, replace $PVC placeholder with actual PVC name
		if req.Type == "storage" && req.PvcName != "" {
			query = strings.ReplaceAll(query, "$PVC", req.PvcName)
		}
	}
	fmt.Println(query)
	return query, nil
}

// replaceVariables replaces template variables in query
func (b *Builder) replaceVariables(query, queryType, namespace, launchPadName, pvcName, service, port string) string {
	if queryType == "network_service_request_count" || queryType == "network_service_request_percent" {
		clusterName := BuildClusterName(service, namespace, port)
		fmt.Println(clusterName)
		replacer := strings.NewReplacer(
			"@", clusterName,
		)
		return replacer.Replace(query)
	}
	replacer := strings.NewReplacer(
		"#", namespace,
		"@", launchPadName,
		"$PVC", pvcName,
	)
	return replacer.Replace(query)
}

func BuildClusterName(serviceName, namespace string, port string) string {
	return fmt.Sprintf("outbound|%s||%s.%s.svc.cluster.local.internal", port, serviceName, namespace)
}

// Package api provides common API types for monitoring services.
package api

import "errors"

// TimeRange represents the time range for metric queries.
// It supports both instant queries (using Time) and range queries (using Start, End, Step).
type TimeRange struct {
	Start string `json:"start" form:"start" mapstructure:"start"` // Start time (unix timestamp or RFC3339)
	End   string `json:"end" form:"end" mapstructure:"end"`       // End time (unix timestamp or RFC3339)
	Step  string `json:"step" form:"step" mapstructure:"step"`    // Query resolution step width
	Time  string `json:"time" form:"time" mapstructure:"time"`    // Evaluation timestamp for instant query
}

// IsInstantQuery returns true if this is an instant query (no time range).
func (tr *TimeRange) IsInstantQuery() bool {
	return tr.Start == "" && tr.End == ""
}

// LaunchpadRequest represents a request to query Launchpad metrics.
type LaunchpadRequest struct {
	Namespace     string    `json:"namespace" form:"namespace"`          // Kubernetes namespace
	LaunchPadName string    `json:"launchPadName" form:"launchPadName"`  // Launchpad application name
	Type          string    `json:"type" form:"type"`                    // Query type: cpu, memory, storage, average_cpu, average_memory
	PvcName       string    `json:"pvcName" form:"pvcName"`              // PVC name (required for storage queries)
	Range         TimeRange `json:"range" form:"range" mapstructure:","` // Time range for the query
}

// DatabaseRequest represents a request to query database metrics.
type DatabaseRequest struct {
	Namespace string    `json:"namespace" form:"namespace"`          // Kubernetes namespace
	Type      string    `json:"type" form:"type"`                    // Database type: apecloud-mysql, postgresql, mongodb, redis, kafka, milvus
	Query     string    `json:"query" form:"query"`                  // Query type or custom PromQL
	Cluster   string    `json:"app" form:"app"`                      // Application instance name (cluster name)
	Range     TimeRange `json:"range" form:"range" mapstructure:","` // Time range for the query
}

// MinioRequest represents a request to query Minio metrics.
type MinioRequest struct {
	Namespace string    `json:"namespace" form:"namespace"`          // Kubernetes namespace
	Query     string    `json:"query" form:"query"`                  // Query type (metric name)
	Bucket    string    `json:"app" form:"app"`                      // Bucket name
	Range     TimeRange `json:"range" form:"range" mapstructure:","` // Time range for the query
}

// DevboxRequest represents a request to query Devbox metrics.
type DevboxRequest struct {
	Namespace  string    `json:"namespace" form:"namespace"`          // Kubernetes namespace
	DevboxName string    `json:"devboxName" form:"devboxName"`        // Devbox name
	Type       string    `json:"type" form:"type"`                    // Query type: cpu, memory, disk, network_in, network_out
	PodName    string    `json:"podName" form:"podName"`              // Pod name (optional, for pod-specific queries)
	Range      TimeRange `json:"range" form:"range" mapstructure:","` // Time range for the query
}

// Common errors
var (
	ErrNoMetricsHost = errors.New("unable to get the metrics host from environment")
)

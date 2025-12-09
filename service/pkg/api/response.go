// Package api provides common API response types.
package api

// MetricResult represents a single metric result from Victoria Metrics/Prometheus.
type MetricResult struct {
	Metric map[string]string `json:"metric"`           // Metric labels
	Value  []interface{}     `json:"value,omitempty"`  // Single value [timestamp, value]
	Values [][]interface{}   `json:"values,omitempty"` // Multiple values [[timestamp, value], ...]
}

// MetricResponse represents the standard response format for metric queries.
// This format is compatible with both Prometheus and Victoria Metrics APIs.
type MetricResponse struct {
	Status    string           `json:"status"` // "success" or "error"
	Data      MetricResultData `json:"data,omitempty"`
	Error     string           `json:"error,omitempty"`     // Error message if status is "error"
	ErrorType string           `json:"errorType,omitempty"` // Error type
}

// MetricResultData represents the data section of a metric response.
type MetricResultData struct {
	ResultType string         `json:"resultType"` // "matrix" or "vector"
	Result     []MetricResult `json:"result"`
}

// LaunchpadResponse extends MetricResponse with Victoria Metrics specific fields.
type LaunchpadResponse struct {
	Status    string           `json:"status"`
	IsPartial bool             `json:"isPartial,omitempty"` // True if the result is partial
	Data      MetricResultData `json:"data"`
	Stats     *QueryStats      `json:"stats,omitempty"` // Query execution statistics
}

// QueryStats contains query execution statistics from Victoria Metrics.
type QueryStats struct {
	SeriesFetched     string `json:"seriesFetched,omitempty"`
	ExecutionTimeMsec int    `json:"executionTimeMsec,omitempty"`
}

// internal/metrics/metrics.go
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Simple metrics for securities API
var (
	// Function call tracking
	FunctionCalls = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "securities_function_calls_total",
			Help: "Total function calls by endpoint and status",
		},
		[]string{"function", "status"}, // function name, success/error
	)

	// Function duration tracking
	FunctionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "securities_function_duration_seconds",
			Help:    "Function execution duration",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
		},
		[]string{"function"},
	)

	// Database query tracking
	DatabaseQueries = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "securities_db_query_duration_seconds",
			Help:    "Database query duration",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5},
		},
		[]string{"function"}, // Added table parameter
	)

	// Results tracking
	ResultsCount = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "securities_results_count",
			Help:    "Number of results returned",
			Buckets: []float64{0, 1, 2, 5, 10, 15, 20},
		},
		[]string{"function"},
	)
)

// Helper functions to make recording metrics simple

// RecordFunctionCall records a function call with status and duration
func RecordFunctionCall(functionName, status string, duration float64) {
	FunctionCalls.WithLabelValues(functionName, status).Inc()
	FunctionDuration.WithLabelValues(functionName).Observe(duration)
}

// RecordDatabaseQuery records database query timing with operation and table
func RecordDatabaseQuery(functionName string, duration float64) {
	DatabaseQueries.WithLabelValues(functionName).Observe(duration)
}

// RecordResults records the number of results returned
func RecordResults(functionName string, count int) {
	ResultsCount.WithLabelValues(functionName).Observe(float64(count))
}

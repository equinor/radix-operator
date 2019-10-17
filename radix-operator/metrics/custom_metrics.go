package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	nrCrAdded = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "radix_operator_cr_added",
		Help: "The total number of radix custom resources added",
	}, []string{"cr_type"})
	nrCrDeleted = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "radix_operator_cr_deleted",
		Help: "The total number of radix custom resources deleted",
	}, []string{"cr_type"})
	nrErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "radix_operator_errors",
		Help: "The total number of radix operator errors",
	}, []string{"err_type", "method"})
)

// DefaultBuckets Holds the buckets used as default
func DefaultBuckets() []float64 {
	return []float64{0.03, 0.1, 0.3, 1, 2, 3, 5, 8, 15}
}

// CustomResourceAdded Increments metric to count the number of cr added
func CustomResourceAdded(kind string) {
	nrCrAdded.With(prometheus.Labels{"cr_type": kind}).Inc()
}

// CustomResourceDeleted Increments metric to count the number of cr deleted
func CustomResourceDeleted(kind string) {
	nrCrDeleted.With(prometheus.Labels{"cr_type": kind}).Inc()
}

// OperatorError Add error
func OperatorError(method, errorType string) {
	nrErrors.With(prometheus.Labels{
		"method":   method,
		"err_type": errorType,
	}).Inc()
}

package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	nrCrQueued = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "radix_operator_cr_queued",
		Help: "The total number of radix custom resources added, updated or deleted in queue",
	}, []string{"cr_type", "operation", "skipped", "requeued"})
	nrErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "radix_operator_errors",
		Help: "The total number of radix operator errors",
	}, []string{"cr_type", "err_type", "method"})
	nrCrDeQueued = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "radix_operator_cr_de_queued",
		Help: "The total number of radix custom resources removed from queue",
	}, []string{"cr_type"})
	recTimeBucket = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "radix_operator_reconciliation_duration_seconds_hist",
			Help:    "Request duration seconds bucket",
			Buckets: DefaultBuckets(),
		},
		[]string{"cr_type"},
	)
)

func init() {
	prometheus.MustRegister(recTimeBucket)
}

// DefaultBuckets Holds the buckets used as default
func DefaultBuckets() []float64 {
	return []float64{0.03, 0.1, 0.3, 1, 2, 3, 5, 8, 15, 23}
}

// CustomResourceAdded Increments metric to count the number of cr added
func CustomResourceAdded(kind string) {
	nrCrQueued.With(prometheus.Labels{"cr_type": kind, "operation": "add", "skipped": "false", "requeued": "false"}).Inc()
}

// CustomResourceUpdated Increments metric to count the number of cr updated
func CustomResourceUpdated(kind string) {
	nrCrQueued.With(prometheus.Labels{"cr_type": kind, "operation": "update", "skipped": "false", "requeued": "false"}).Inc()
}

// CustomResourceUpdatedAndRequeued Increments metric to count the number of cr updated due to update to child
func CustomResourceUpdatedAndRequeued(kind string) {
	nrCrQueued.With(prometheus.Labels{"cr_type": kind, "operation": "update", "skipped": "false", "requeued": "true"}).Inc()
}

// CustomResourceAddedButSkipped Increments metric to count the number of cr added and ignored
func CustomResourceAddedButSkipped(kind string) {
	nrCrQueued.With(prometheus.Labels{"cr_type": kind, "operation": "add", "skipped": "true", "requeued": "false"}).Inc()
}

// CustomResourceUpdatedButSkipped Increments metric to count the number of cr updated and ignored
func CustomResourceUpdatedButSkipped(kind string) {
	nrCrQueued.With(prometheus.Labels{"cr_type": kind, "operation": "update", "skipped": "true", "requeued": "false"}).Inc()
}

// CustomResourceDeleted Increments metric to count the number of cr deleted
func CustomResourceDeleted(kind string) {
	nrCrQueued.With(prometheus.Labels{"cr_type": kind, "operation": "update", "skipped": "true", "requeued": "false"}).Inc()
}

// CustomResourceRemovedFromQueue Decrements metric to count the number of cr in queue
func CustomResourceRemovedFromQueue(kind string) {
	nrCrDeQueued.With(prometheus.Labels{"cr_type": kind}).Inc()
}

// OperatorError Add error
func OperatorError(kind, method, errorType string) {
	nrErrors.With(prometheus.Labels{
		"cr_type":  kind,
		"method":   method,
		"err_type": errorType,
	}).Inc()
}

// AddDurrationOfReconciliation Add duration it takes to reconcile
func AddDurrationOfReconciliation(kind string, duration time.Duration) {
	recTimeBucket.WithLabelValues(kind).Observe(duration.Seconds())
}

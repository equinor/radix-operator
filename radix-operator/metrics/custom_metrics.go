package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	nrCrAdded = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "radix_operator_cr_added",
		Help: "The total number of radix custom resources added",
	}, []string{"cr_type", "skipped"})
	nrCrUpdated = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "radix_operator_cr_updated",
		Help: "The total number of radix custom resources updated",
	}, []string{"cr_type", "skipped"})
	nrCrDeleted = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "radix_operator_cr_deleted",
		Help: "The total number of radix custom resources deleted",
	}, []string{"cr_type"})
	nrErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "radix_operator_errors",
		Help: "The total number of radix operator errors",
	}, []string{"err_type", "method"})
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
	nrCrAdded.With(prometheus.Labels{"cr_type": kind, "skipped": "false"}).Inc()
}

// CustomResourceUpdated Increments metric to count the number of cr updated
func CustomResourceUpdated(kind string) {
	nrCrAdded.With(prometheus.Labels{"cr_type": kind, "skipped": "false"}).Inc()
}

// CustomResourceAddedButSkipped Increments metric to count the number of cr added and ignored
func CustomResourceAddedButSkipped(kind string) {
	nrCrAdded.With(prometheus.Labels{"cr_type": kind, "skipped": "true"}).Inc()
}

// CustomResourceUpdatedButSkipped Increments metric to count the number of cr updated and ignored
func CustomResourceUpdatedButSkipped(kind string) {
	nrCrAdded.With(prometheus.Labels{"cr_type": kind, "skipped": "true"}).Inc()
}

// CustomResourceDeleted Increments metric to count the number of cr deleted
func CustomResourceDeleted(kind string) {
	nrCrDeleted.With(prometheus.Labels{"cr_type": kind}).Inc()
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

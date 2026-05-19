package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	jobsTriggeredMetric         = "radix_api_jobs_triggered"
	requestDurationMetric       = "radix_api_request_duration_seconds"
	requestDurationBucketMetric = "radix_api_request_duration_seconds_hist"

	appNameLabel  = "app_name"
	pipelineLabel = "pipeline"
	pathLabel     = "path"
	methodLabel   = "method"
)

var (
	nrJobsTriggered = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: jobsTriggeredMetric,
			Help: "The total number of jobs triggered",
		}, []string{appNameLabel, pipelineLabel})
	resTime = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       requestDurationMetric,
			Help:       "Request duration seconds",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{pathLabel, methodLabel},
	)
	resTimeBucket = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    requestDurationBucketMetric,
			Help:    "Request duration seconds bucket",
			Buckets: DefaultBuckets(),
		},
		[]string{pathLabel, methodLabel},
	)
)

func init() {
	prometheus.MustRegister(resTime)
	prometheus.MustRegister(resTimeBucket)
}

// DefaultBuckets Holds the buckets used as default
func DefaultBuckets() []float64 {
	return []float64{0.03, 0.1, 0.3, 1, 2, 3, 5, 10}
}

// AddJobTriggered New job triggered for application
func AddJobTriggered(appName, pipeline string) {
	nrJobsTriggered.With(prometheus.Labels{appNameLabel: appName, pipelineLabel: pipeline}).Inc()
}

// AddRequestDuration Add request duration for given endpoint
func AddRequestDuration(path, method string, duration time.Duration) {
	resTime.WithLabelValues(path, method).Observe(duration.Seconds())
	resTimeBucket.WithLabelValues(path, method).Observe(duration.Seconds())
}

package metrics

import (
	"time"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"k8s.io/apimachinery/pkg/api/resource"
)

var (
	nrCrQueued = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "radix_operator_cr_queued",
		Help: "The total number of radix custom resources added, updated or deleted in queue",
	}, []string{"cr_type", "operation", "skipped", "requeued"})
	nrCrDeleted = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "radix_operator_cr_deleted",
		Help: "The total number of radix custom resources deleted",
	}, []string{"cr_type"})
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

	radixRequestedCPU = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "radix_operator_requested_cpu",
		Help: "Requested cpu in millicore by environment and component",
	}, []string{"application", "environment", "component", "wbs"})
	radixRequestedMemory = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "radix_operator_requested_memory",
		Help: "Requested memory in megabyte by environment and component. 1Mi = 1024 * 1024 bytes > 1MB = 1000000 bytes (ref https://simple.wikipedia.org/wiki/Mebibyte)",
	}, []string{"application", "environment", "component", "wbs"})
	radixRequestedReplicas = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "radix_operator_requested_replicas",
		Help: "Requested replicas by environment and component",
	}, []string{"application", "environment", "component", "wbs"})

	radixJobProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "radix_operator_radix_job_processed",
		Help: "The number of radix jobs processed with status",
	}, []string{"application", "pipeline_type", "status", "docker_registry", "pipeline_image"})
)

func init() {
	prometheus.MustRegister(recTimeBucket)
}

// RequestedResources adds metrics for requested resources
func RequestedResources(rr *v1.RadixRegistration, rd *v1.RadixDeployment) {
	if rd == nil || rd.Status.Condition == v1.DeploymentInactive || rr == nil {
		return
	}
	defaultCPU := defaults.GetDefaultCPURequest()
	defaultMemory := defaults.GetDefaultMemoryRequest()

	for _, comp := range rd.Spec.Components {
		resources := comp.GetResourceRequirements()
		nrReplicas := float64(comp.GetNrOfReplicas())

		if resources == nil {
			if defaultCPU != nil {
				radixRequestedCPU.With(prometheus.Labels{"application": rd.Spec.AppName, "environment": rd.Spec.Environment, "component": comp.Name, "wbs": rr.Spec.WBS}).Set(float64(defaultCPU.MilliValue()))
			}
			if defaultMemory != nil {
				radixRequestedMemory.With(prometheus.Labels{"application": rd.Spec.AppName, "environment": rd.Spec.Environment, "component": comp.Name, "wbs": rr.Spec.WBS}).Set(float64(defaultMemory.ScaledValue(resource.Mega)))
			}
			radixRequestedReplicas.With(prometheus.Labels{"application": rd.Spec.AppName, "environment": rd.Spec.Environment, "component": comp.Name, "wbs": rr.Spec.WBS}).Set(nrReplicas)
			continue
		}
		requestedResources := resources.Requests

		if cpu := requestedResources.Cpu(); cpu != nil {
			radixRequestedCPU.With(prometheus.Labels{"application": rd.Spec.AppName, "environment": rd.Spec.Environment, "component": comp.Name, "wbs": rr.Spec.WBS}).Set(float64(cpu.MilliValue()))
		}

		if memory := requestedResources.Memory(); memory != nil {
			radixRequestedMemory.With(prometheus.Labels{"application": rd.Spec.AppName, "environment": rd.Spec.Environment, "component": comp.Name, "wbs": rr.Spec.WBS}).Set(float64(memory.ScaledValue(resource.Mega)))
		}

		radixRequestedReplicas.With(prometheus.Labels{"application": rd.Spec.AppName, "environment": rd.Spec.Environment, "component": comp.Name, "wbs": rr.Spec.WBS}).Set(nrReplicas)
	}
}

// InitiateRadixJobStatusChanged initiate metric with value 0 to count the number of radix jobs processed.
func InitiateRadixJobStatusChanged(rj *v1.RadixJob) {
	if rj == nil {
		return
	}

	radixJobProcessed.With(prometheus.Labels{"application": rj.Spec.AppName, "pipeline_type": string(rj.Spec.PipeLineType),
		"status": string(v1.JobWaiting), "docker_registry": rj.Spec.DockerRegistry, "pipeline_image": rj.Spec.PipelineImage}).Add(0)
	radixJobProcessed.With(prometheus.Labels{"application": rj.Spec.AppName, "pipeline_type": string(rj.Spec.PipeLineType),
		"status": string(v1.JobQueued), "docker_registry": rj.Spec.DockerRegistry, "pipeline_image": rj.Spec.PipelineImage}).Add(0)
	radixJobProcessed.With(prometheus.Labels{"application": rj.Spec.AppName, "pipeline_type": string(rj.Spec.PipeLineType),
		"status": string(v1.JobRunning), "docker_registry": rj.Spec.DockerRegistry, "pipeline_image": rj.Spec.PipelineImage}).Add(0)
	radixJobProcessed.With(prometheus.Labels{"application": rj.Spec.AppName, "pipeline_type": string(rj.Spec.PipeLineType),
		"status": string(v1.JobFailed), "docker_registry": rj.Spec.DockerRegistry, "pipeline_image": rj.Spec.PipelineImage}).Add(0)
	radixJobProcessed.With(prometheus.Labels{"application": rj.Spec.AppName, "pipeline_type": string(rj.Spec.PipeLineType),
		"status": string(v1.JobStopped), "docker_registry": rj.Spec.DockerRegistry, "pipeline_image": rj.Spec.PipelineImage}).Add(0)
	radixJobProcessed.With(prometheus.Labels{"application": rj.Spec.AppName, "pipeline_type": string(rj.Spec.PipeLineType),
		"status": string(v1.JobSucceeded), "docker_registry": rj.Spec.DockerRegistry, "pipeline_image": rj.Spec.PipelineImage}).Add(0)
}

// RadixJobStatusChanged increments metric to count the number of radix jobs processed
func RadixJobStatusChanged(rj *v1.RadixJob) {
	if rj == nil {
		return
	}
	radixJobProcessed.With(prometheus.Labels{"application": rj.Spec.AppName, "pipeline_type": string(rj.Spec.PipeLineType),
		"status": string(rj.Status.Condition), "docker_registry": rj.Spec.DockerRegistry, "pipeline_image": rj.Spec.PipelineImage}).Inc()
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

package internal

import (
	modelsv1 "github.com/equinor/radix-operator/job-scheduler/models/v1"
	corev1 "k8s.io/api/core/v1"
)

// SetBatchJobEventMessageToBatchJobStatus sets the event message for the batch job status
func SetBatchJobEventMessageToBatchJobStatus(jobStatus *modelsv1.JobStatus, batchJobPodsMap map[string]corev1.Pod, eventMessageForPods map[string]string) {
	if jobStatus == nil || len(jobStatus.Message) > 0 {
		return
	}
	if batchJobPod, ok := batchJobPodsMap[jobStatus.Name]; ok {
		if eventMessage, ok := eventMessageForPods[batchJobPod.Name]; ok {
			jobStatus.Message = eventMessage
		}
	}
}

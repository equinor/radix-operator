package scheduledjob

import (
	"context"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *syncer) syncStatus() error {
	newStatus := radixv1.RadixScheduledJobStatus{
		Phase:  radixv1.ScheduledJobPhaseWaiting,
		Reason: "",
	}

	existingJobs, err := s.kubeclient.BatchV1().Jobs(s.radixScheduledJob.GetNamespace()).List(context.TODO(), metav1.ListOptions{LabelSelector: s.scheduledJobLabelIdentifier().String()})
	if err != nil {
		return err
	}

	if len(existingJobs.Items) > 0 {
		newStatus, err = s.buildStatusFromKubernetesJob(&existingJobs.Items[0])
		if err != nil {
			return err
		}
	}

	return s.updateStatus(func(currStatus *radixv1.RadixScheduledJobStatus) {
		*currStatus = newStatus
	})
}

func (s *syncer) buildStatusFromKubernetesJob(job *batchv1.Job) (radixv1.RadixScheduledJobStatus, error) {
	var phase radixv1.RadixScheduledJobPhase
	var reason string

	switch {
	case job.Status.Succeeded > 0:
		phase = radixv1.ScheduledJobPhaseSucceeded
	case job.Status.Failed > 0:
		phase = radixv1.ScheduledJobPhaseFailed
		reason = "job failed" // TODO: get reason from pod
	default:
		phase, reason = radixv1.ScheduledJobPhaseWaiting, "waiting reason" // TODO: get reason from pod
	}

	status := radixv1.RadixScheduledJobStatus{
		Phase:   phase,
		Reason:  reason,
		Created: &job.CreationTimestamp,
		Started: job.Status.StartTime,
		Ended:   job.Status.CompletionTime,
	}

	return status, nil
}

package scheduledjob

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sort"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *syncer) syncStatus(reconcileError error) error {
	if status := reconcileStatus(nil); errors.As(reconcileError, &status) {
		if err := s.updateStatus(func(currStatus *radixv1.RadixScheduledJobStatus) { *currStatus = status.Status() }); err != nil {
			return err
		}
		return reconcileError
	}

	status := radixv1.RadixScheduledJobStatus{
		Phase: radixv1.ScheduledJobPhaseWaiting,
	}

	existingJobs, err := s.kubeclient.BatchV1().Jobs(s.radixScheduledJob.GetNamespace()).List(context.TODO(), metav1.ListOptions{LabelSelector: s.scheduledJobLabelIdentifier().String()})
	if err != nil {
		return err
	}

	if len(existingJobs.Items) > 0 {
		status, err = s.buildStatusFromKubernetesJob(&existingJobs.Items[0])
		if err != nil {
			return err
		}
	}

	if err := s.updateStatus(func(currStatus *radixv1.RadixScheduledJobStatus) { *currStatus = status }); err != nil {
		return err
	}

	return reconcileError
}

func (s *syncer) updateStatus(changeStatusFunc func(currStatus *radixv1.RadixScheduledJobStatus)) error {
	changeStatusFunc(&s.radixScheduledJob.Status)
	updatedJob, err := s.radixclient.
		RadixV1().
		RadixScheduledJobs(s.radixScheduledJob.GetNamespace()).
		UpdateStatus(context.TODO(), s.radixScheduledJob, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	s.radixScheduledJob = updatedJob
	return nil
}

func isJobStatusCondition(conditionType batchv1.JobConditionType) func(batchv1.JobCondition) bool {
	return func(condition batchv1.JobCondition) bool {
		return condition.Type == conditionType && condition.Status == v1.ConditionTrue
	}

}

func (s *syncer) buildStatusFromKubernetesJob(job *batchv1.Job) (radixv1.RadixScheduledJobStatus, error) {
	var phase radixv1.RadixScheduledJobPhase = radixv1.ScheduledJobPhaseWaiting
	var reason, message string
	var started, ended *metav1.Time

	pods, err := s.kubeclient.CoreV1().Pods(job.GetNamespace()).List(context.TODO(), metav1.ListOptions{LabelSelector: jobNameLabelSelector(job.GetName()).String()})
	if err != nil {
		return radixv1.RadixScheduledJobStatus{}, err
	}

	switch {
	case slice.Any(job.Status.Conditions, isJobStatusCondition(batchv1.JobComplete)):
		phase = radixv1.ScheduledJobPhaseSucceeded
	case slice.Any(job.Status.Conditions, isJobStatusCondition(batchv1.JobFailed)):
		phase = radixv1.ScheduledJobPhaseFailed
	case job.Status.Active > 0:
		phase = radixv1.ScheduledJobPhaseRunning
	}

	switch phase {
	case radixv1.ScheduledJobPhaseRunning:
		started = job.Status.StartTime
	case radixv1.ScheduledJobPhaseSucceeded:
		started = job.Status.StartTime
		ended = job.Status.CompletionTime
	case radixv1.ScheduledJobPhaseFailed:
		started = job.Status.StartTime
	}

	if phase == radixv1.ScheduledJobPhaseFailed || phase == radixv1.ScheduledJobPhaseWaiting {
		reason, message = getReasonAndMessageFromPendingOrFailedPod(pods.Items)
	}

	status := radixv1.RadixScheduledJobStatus{
		Phase:   phase,
		Reason:  reason,
		Message: message,
		Created: &job.CreationTimestamp,
		Started: started,
		Ended:   ended,
	}

	return status, nil
}

func (s *syncer) restoreStatus() error {
	if restoredStatus, ok := s.radixScheduledJob.Annotations[kube.RestoredStatusAnnotation]; ok && len(restoredStatus) > 0 {
		if reflect.ValueOf(s.radixScheduledJob.Status).IsZero() {
			var status radixv1.RadixScheduledJobStatus
			if err := json.Unmarshal([]byte(restoredStatus), &status); err != nil {
				log.Warnf("unable to restore status for scheduled job %s.%s from annotation", s.radixScheduledJob.GetNamespace(), s.radixScheduledJob.GetName())
				return nil
			}
			return s.updateStatus(func(currStatus *radixv1.RadixScheduledJobStatus) {
				*currStatus = status
			})
		}
	}

	return nil
}

func getReasonAndMessageFromPendingOrFailedPod(pods []v1.Pod) (reason, message string) {
	if len(pods) == 0 {
		return
	}
	sort.Slice(pods, func(i, j int) bool { return pods[i].CreationTimestamp.After(pods[j].CreationTimestamp.Time) })
	pod := pods[0]
	switch pod.Status.Phase {
	case v1.PodPending, v1.PodUnknown:
		reason = pod.Status.Reason
		message = pod.Status.Message

		if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].State.Waiting != nil {
			reason = pod.Status.ContainerStatuses[0].State.Waiting.Reason
			message = pod.Status.ContainerStatuses[0].State.Waiting.Message
		}
	case v1.PodFailed:
		reason = pod.Status.Reason
		message = pod.Status.Message

		if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].State.Terminated != nil {
			if len(reason) == 0 {
				reason = pod.Status.ContainerStatuses[0].State.Terminated.Reason
			}
			if len(message) == 0 {
				message = pod.Status.ContainerStatuses[0].State.Terminated.Message
			}
		}
	}

	return
}

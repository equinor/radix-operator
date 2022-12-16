package scheduledjob

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

const (
	RadixScheduledJobControllerUIDLabel = "radix-scheduled-job-controller-uid"
	kubernetesJobNameLabel              = "job-name"
)

type Syncer interface {
	OnSync() error
}

type syncer struct {
	kubeclient        kubernetes.Interface
	kubeutil          *kube.Kube
	radixclient       radixclient.Interface
	radixScheduledJob *radixv1.RadixScheduledJob
}

func (s *syncer) OnSync() error {
	// Steps:
	// - restore status from annotation (velero)
	// - exit if status.phase indicates completed job (succeeded, failed, stopped)
	// - status = reconcile()
	//   - if spec.stop: delete existing job and return stopped status
	//   - if k8s job not exist: verify spec and try create job, return status
	//   - if k8s job exist: build status as return
	// - update status

	if err := s.restoreStatus(); err != nil {
		return err
	}
	if s.isScheduledJobDone() {
		return nil
	}
	if s.isStopRequested() {
		return s.stopJob()
	}
	return s.reconcile()
}

func (s *syncer) reconcile() error {
	if err := s.reconcileService(); err != nil {
		return err
	}

	if err := s.reconcileJob(); err != nil {
		return err
	}

	return s.syncStatus()
}

func (s *syncer) getRadixDeploymentAndJobComponent() (*radixv1.RadixDeployment, *radixv1.RadixDeployJobComponent, error) {
	rd, err := s.getRadixDeployment()
	if err != nil {
		return nil, nil, err
	}
	jobComponent := rd.GetJobComponentByName(s.radixScheduledJob.Spec.RadixDeploymentJobRef.Job)
	if jobComponent == nil {
		return nil, nil, fmt.Errorf("radix deployment %s does not contain a job with name %s", rd.GetName(), s.radixScheduledJob.Spec.RadixDeploymentJobRef.Job)
	}

	return rd, jobComponent, nil
}

func ownerReference(job *radixv1.RadixScheduledJob) []metav1.OwnerReference {
	trueVar := true
	return []metav1.OwnerReference{
		{
			APIVersion: "radix.equinor.com/v1",
			Kind:       "RadixScheduledJob",
			Name:       job.Name,
			UID:        job.UID,
			Controller: &trueVar,
		},
	}
}

func (s *syncer) getRadixDeployment() (*radixv1.RadixDeployment, error) {
	return s.radixclient.RadixV1().RadixDeployments(s.radixScheduledJob.GetNamespace()).Get(context.TODO(), s.radixScheduledJob.Spec.RadixDeploymentJobRef.Name, metav1.GetOptions{})
}

func (s *syncer) scheduledJobLabelIdentifier() labels.Set {
	return radixlabels.ForJobName(s.radixScheduledJob.GetName())
	// return labels.Set{RadixScheduledJobControllerUIDLabel: string(job.GetUID())}
}

func (s *syncer) stopJob() error {
	selector := s.scheduledJobLabelIdentifier()
	err := s.kubeclient.BatchV1().Jobs(s.radixScheduledJob.GetNamespace()).DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	return s.updateStatus(func(currStatus *radixv1.RadixScheduledJobStatus) {
		currStatus.Phase = radixv1.ScheduledJobPhaseStopped
	})
}

func (s *syncer) isStopRequested() bool {
	return s.radixScheduledJob.Spec.Stop != nil && *s.radixScheduledJob.Spec.Stop
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

func (s *syncer) isScheduledJobDone() bool {
	phase := s.radixScheduledJob.Status.Phase
	return phase == radixv1.ScheduledJobPhaseSucceeded ||
		phase == radixv1.ScheduledJobPhaseFailed ||
		phase == radixv1.ScheduledJobPhaseStopped
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

func NewSyncer(kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	radixScheduledJob *radixv1.RadixScheduledJob) Syncer {
	return &syncer{
		kubeclient:        kubeclient,
		kubeutil:          kubeutil,
		radixclient:       radixclient,
		radixScheduledJob: radixScheduledJob,
	}
}

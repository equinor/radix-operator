package scheduledjob

import (
	"context"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
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

type syncer struct {
	kubeclient        kubernetes.Interface
	kubeutil          *kube.Kube
	radixclient       radixclient.Interface
	radixScheduledJob *radixv1.RadixScheduledJob
}

func (s *syncer) OnSync() error {
	if err := s.restoreStatus(); err != nil {
		return err
	}
	if s.isScheduledJobDone() {
		return nil
	}

	if s.isStopRequested() {
		return s.stopJob()
	}

	return s.syncStatus(s.reconcile())
}

func (s *syncer) reconcile() error {
	if err := s.reconcileService(); err != nil {
		return err
	}

	return s.reconcileJob()
}

func (s *syncer) getRadixDeploymentAndJobComponent() (*radixv1.RadixDeployment, *radixv1.RadixDeployJobComponent, error) {
	rd, err := s.getRadixDeployment()
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil, newReconcileRadixDeploymentNotFoundError(s.radixScheduledJob.Spec.RadixDeploymentJobRef.Name)
		}
		return nil, nil, err
	}
	jobComponent := rd.GetJobComponentByName(s.radixScheduledJob.Spec.RadixDeploymentJobRef.Job)
	if jobComponent == nil {
		return nil, nil, newReconcileRadixDeploymentJobSpecNotFoundError(rd.GetName(), s.radixScheduledJob.Spec.RadixDeploymentJobRef.Job)
	}

	return rd, jobComponent, nil
}

func (s *syncer) getRadixDeployment() (*radixv1.RadixDeployment, error) {
	return s.kubeutil.GetRadixDeployment(s.radixScheduledJob.GetNamespace(), s.radixScheduledJob.Spec.RadixDeploymentJobRef.Name)
}

func (s *syncer) scheduledJobIdentifierLabel() labels.Set {
	return radixlabels.ForJobName(s.radixScheduledJob.GetName())
}

func (s *syncer) stopJob() error {
	selector := s.scheduledJobIdentifierLabel()
	background := metav1.DeletePropagationBackground
	err := s.kubeclient.BatchV1().Jobs(s.radixScheduledJob.GetNamespace()).DeleteCollection(context.TODO(), metav1.DeleteOptions{PropagationPolicy: &background}, metav1.ListOptions{LabelSelector: selector.String()})
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

func (s *syncer) isScheduledJobDone() bool {
	phase := s.radixScheduledJob.Status.Phase
	return phase == radixv1.ScheduledJobPhaseSucceeded ||
		phase == radixv1.ScheduledJobPhaseFailed ||
		phase == radixv1.ScheduledJobPhaseStopped
}

func ownerReference(job *radixv1.RadixScheduledJob) []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			APIVersion: "radix.equinor.com/v1",
			Kind:       "RadixScheduledJob",
			Name:       job.Name,
			UID:        job.UID,
			Controller: utils.BoolPtr(true),
		},
	}
}

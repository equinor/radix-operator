package batch

import (
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

type Syncer interface {
	OnSync() error
}

func NewSyncer(kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	batch *radixv1.RadixBatch) Syncer {
	return &syncer{
		kubeclient:  kubeclient,
		kubeutil:    kubeutil,
		radixclient: radixclient,
		batch:       batch,
	}
}

type syncer struct {
	kubeclient  kubernetes.Interface
	kubeutil    *kube.Kube
	radixclient radixclient.Interface
	batch       *radixv1.RadixBatch
}

func (s *syncer) OnSync() error {
	if err := s.restoreStatus(); err != nil {
		return err
	}

	if isBatchDone(s.batch) {
		return nil
	}

	return s.syncStatus(s.reconcile())
}

func (s *syncer) reconcile() error {
	const syncStatusForEveryNumberOfBatchJobsReconciled = 10

	rd, jobComponent, err := s.getRadixDeploymentAndJobComponent()
	if err != nil {
		return err
	}

	existingJobs, err := s.kubeutil.ListJobsWithSelector(s.batch.GetNamespace(), s.batchIdentifierLabel().String())
	if err != nil {
		return err
	}

	existingServices, err := s.kubeutil.ListServicesWithSelector(s.batch.GetNamespace(), s.batchIdentifierLabel().String())
	if err != nil {
		return err
	}

	for i, batchJob := range s.batch.Spec.Jobs {
		if err := s.reconcileService(batchJob, rd, jobComponent, existingServices); err != nil {
			return err
		}

		if err := s.reconcileKubeJob(batchJob, rd, jobComponent, existingJobs); err != nil {
			return err
		}

		if i%syncStatusForEveryNumberOfBatchJobsReconciled == 0 {
			s.syncStatus(nil)
		}
	}

	return nil
}

func (s *syncer) getRadixDeploymentAndJobComponent() (*radixv1.RadixDeployment, *radixv1.RadixDeployJobComponent, error) {
	rd, err := s.getRadixDeployment()
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil, newReconcileRadixDeploymentNotFoundError(s.batch.Spec.RadixDeploymentJobRef.Name)
		}
		return nil, nil, err
	}

	jobComponent := rd.GetJobComponentByName(s.batch.Spec.RadixDeploymentJobRef.Job)
	if jobComponent == nil {
		return nil, nil, newReconcileRadixDeploymentJobSpecNotFoundError(rd.GetName(), s.batch.Spec.RadixDeploymentJobRef.Job)
	}

	return rd, jobComponent, nil
}

func (s *syncer) getRadixDeployment() (*radixv1.RadixDeployment, error) {
	return s.kubeutil.GetRadixDeployment(s.batch.GetNamespace(), s.batch.Spec.RadixDeploymentJobRef.Name)
}

func (s *syncer) batchIdentifierLabel() labels.Set {
	return radixlabels.Merge(
		radixlabels.ForBatchName(s.batch.GetName()),
	)
}

func (s *syncer) batchJobIdentifierLabel(batchJobName, appName string) labels.Set {
	return radixlabels.Merge(
		radixlabels.ForApplicationName(appName),
		radixlabels.ForComponentName(s.batch.Spec.RadixDeploymentJobRef.Job),
		s.batchIdentifierLabel(),
		radixlabels.ForJobScheduleJobType(),
		radixlabels.ForBatchJobName(batchJobName),
	)
}

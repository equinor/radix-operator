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

// Syncer of  RadixBatch
type Syncer interface {
	// OnSync Syncs RadixBatch
	OnSync() error
}

// NewSyncer Constructor os RadixBatches Syncer
func NewSyncer(kubeclient kubernetes.Interface,
	kubeUtil *kube.Kube,
	radixClient radixclient.Interface,
	radixBatch *radixv1.RadixBatch) Syncer {
	return &syncer{
		kubeClient:  kubeclient,
		kubeUtil:    kubeUtil,
		radixClient: radixClient,
		radixBatch:  radixBatch,
	}
}

type syncer struct {
	kubeClient  kubernetes.Interface
	kubeUtil    *kube.Kube
	radixClient radixclient.Interface
	radixBatch  *radixv1.RadixBatch
}

// OnSync Syncs RadixBatches
func (s *syncer) OnSync() error {
	if err := s.restoreStatus(); err != nil {
		return err
	}

	if isBatchDone(s.radixBatch) {
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

	existingJobs, err := s.kubeUtil.ListJobsWithSelector(s.radixBatch.GetNamespace(), s.batchIdentifierLabel().String())
	if err != nil {
		return err
	}

	existingServices, err := s.kubeUtil.ListServicesWithSelector(s.radixBatch.GetNamespace(), s.batchIdentifierLabel().String())
	if err != nil {
		return err
	}

	for i, batchJob := range s.radixBatch.Spec.Jobs {
		if err := s.reconcileService(&batchJob, rd, jobComponent, existingServices); err != nil {
			return err
		}

		if err := s.reconcileKubeJob(&batchJob, rd, jobComponent, existingJobs); err != nil {
			return err
		}

		if i%syncStatusForEveryNumberOfBatchJobsReconciled == 0 {
			if err := s.syncStatus(nil); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *syncer) getRadixDeploymentAndJobComponent() (*radixv1.RadixDeployment, *radixv1.RadixDeployJobComponent, error) {
	rd, err := s.getRadixDeployment()
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil, newReconcileRadixDeploymentNotFoundError(s.radixBatch.Spec.RadixDeploymentJobRef.Name)
		}
		return nil, nil, err
	}

	jobComponent := rd.GetJobComponentByName(s.radixBatch.Spec.RadixDeploymentJobRef.Job)
	if jobComponent == nil {
		return nil, nil, newReconcileRadixDeploymentJobSpecNotFoundError(rd.GetName(), s.radixBatch.Spec.RadixDeploymentJobRef.Job)
	}

	return rd, jobComponent, nil
}

func (s *syncer) getRadixDeployment() (*radixv1.RadixDeployment, error) {
	return s.kubeUtil.GetRadixDeployment(s.radixBatch.GetNamespace(), s.radixBatch.Spec.RadixDeploymentJobRef.Name)
}

func (s *syncer) batchIdentifierLabel() labels.Set {
	return radixlabels.Merge(
		radixlabels.ForBatchName(s.radixBatch.GetName()),
	)
}

func (s *syncer) batchJobIdentifierLabel(batchJobName, appName string) labels.Set {
	return radixlabels.Merge(
		radixlabels.ForApplicationName(appName),
		radixlabels.ForComponentName(s.radixBatch.Spec.RadixDeploymentJobRef.Job),
		s.batchIdentifierLabel(),
		radixlabels.ForJobScheduleJobType(),
		radixlabels.ForBatchJobName(batchJobName),
	)
}

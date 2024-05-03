package batch

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// Syncer of  RadixBatch
type Syncer interface {
	// OnSync Syncs RadixBatch
	OnSync(ctx context.Context) error
}

// NewSyncer Constructor os RadixBatches Syncer
func NewSyncer(kubeclient kubernetes.Interface, kubeUtil *kube.Kube, radixClient radixclient.Interface, radixBatch *radixv1.RadixBatch) Syncer {
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
func (s *syncer) OnSync(ctx context.Context) error {
	ctx = log.Ctx(ctx).With().
		Str("resource_kind", radixv1.KindRadixBatch).
		Str("resource_name", cache.MetaObjectToName(&s.radixBatch.ObjectMeta).String()).
		Logger().WithContext(ctx)

	if err := s.restoreStatus(ctx); err != nil {
		return err
	}

	if isBatchDone(s.radixBatch) {
		return nil
	}

	return s.syncStatus(ctx, s.reconcile(ctx))
}

func (s *syncer) reconcile(ctx context.Context) error {
	const syncStatusForEveryNumberOfBatchJobsReconciled = 10

	rd, jobComponent, err := s.getRadixDeploymentAndJobComponent(ctx)
	if err != nil {
		return err
	}

	existingJobs, err := s.kubeUtil.ListJobsWithSelector(ctx, s.radixBatch.GetNamespace(), s.batchIdentifierLabel().String())
	if err != nil {
		return err
	}

	existingServices, err := s.kubeUtil.ListServicesWithSelector(ctx, s.radixBatch.GetNamespace(), s.batchIdentifierLabel().String())
	if err != nil {
		return err
	}

	for i, batchJob := range s.radixBatch.Spec.Jobs {
		if err := s.reconcileService(ctx, &batchJob, rd, jobComponent, existingServices); err != nil {
			return fmt.Errorf("batchjob %s: failed to reconcile service: %w", batchJob.Name, err)
		}

		if err := s.reconcileKubeJob(ctx, &batchJob, rd, jobComponent, existingJobs); err != nil {
			return fmt.Errorf("batchjob %s: failed to reconcile kubejob: %w", batchJob.Name, err)
		}

		if i%syncStatusForEveryNumberOfBatchJobsReconciled == 0 {
			if err := s.syncStatus(ctx, nil); err != nil {
				return fmt.Errorf("batchjob %s: failed to sync status: %w", batchJob.Name, err)
			}
		}
	}

	return nil
}

func (s *syncer) getRadixDeploymentAndJobComponent(ctx context.Context) (*radixv1.RadixDeployment, *radixv1.RadixDeployJobComponent, error) {
	rd, err := s.getRadixDeployment(ctx)
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

func (s *syncer) getRadixDeployment(ctx context.Context) (*radixv1.RadixDeployment, error) {
	return s.kubeUtil.GetRadixDeployment(ctx, s.radixBatch.GetNamespace(), s.radixBatch.Spec.RadixDeploymentJobRef.Name)
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

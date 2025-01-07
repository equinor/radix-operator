package batch

import (
	"context"
	"fmt"
	commonutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/pkg/apis/volumemount"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

const (
	syncStatusForEveryNumberOfBatchJobsReconciled = 10
)

// Syncer of  RadixBatch
type Syncer interface {
	// OnSync Syncs RadixBatch
	OnSync(ctx context.Context) error
}

type SyncerOption func(syncer *syncer)

func WithClock(clock commonutils.Clock) SyncerOption {
	return func(syncer *syncer) {
		syncer.clock = clock
	}
}

// NewSyncer Constructor os RadixBatches Syncer
func NewSyncer(kubeclient kubernetes.Interface, kubeUtil *kube.Kube, radixClient radixclient.Interface, radixBatch *radixv1.RadixBatch, config *config.Config, options ...SyncerOption) Syncer {
	syncer := &syncer{
		kubeClient:    kubeclient,
		kubeUtil:      kubeUtil,
		radixClient:   radixClient,
		radixBatch:    radixBatch,
		config:        config,
		restartedJobs: map[string]radixv1.RadixBatchJob{},
		clock:         commonutils.RealClock{},
	}

	for _, opt := range options {
		opt(syncer)
	}

	return syncer
}

type syncer struct {
	kubeClient    kubernetes.Interface
	kubeUtil      *kube.Kube
	radixClient   radixclient.Interface
	radixBatch    *radixv1.RadixBatch
	config        *config.Config
	restartedJobs map[string]radixv1.RadixBatchJob
	clock         commonutils.Clock
}

// OnSync Syncs RadixBatches
func (s *syncer) OnSync(ctx context.Context) error {
	ctx = log.Ctx(ctx).With().Str("resource_kind", radixv1.KindRadixBatch).Logger().WithContext(ctx)
	log.Ctx(ctx).Info().Msg("Syncing")

	if err := s.restoreStatus(ctx); err != nil {
		return err
	}

	if s.isBatchDone() && !s.isRestartRequestedForAnyBatchJob() {
		return nil
	}

	return s.syncStatus(ctx, s.reconcile(ctx))
}

func (s *syncer) reconcile(ctx context.Context) error {
	rd, jobComponent, err := s.getRadixDeploymentAndJobComponent(ctx)
	if err != nil {
		return err
	}

	namespace := s.radixBatch.GetNamespace()
	existingJobs, err := s.kubeUtil.ListJobsWithSelector(ctx, namespace, s.batchIdentifierLabel().String())
	if err != nil {
		return err
	}

	existingServices, err := s.kubeUtil.ListServicesWithSelector(ctx, namespace, s.batchIdentifierLabel().String())
	if err != nil {
		return err
	}
	existingVolumes, err := volumemount.GetExistingJobAuxComponentVolumes(ctx, s.kubeUtil, namespace, jobComponent.GetName())
	if err != nil {
		return err
	}
	desiredVolumes, err := volumemount.GetVolumes(ctx, s.kubeUtil, namespace, jobComponent, rd.Name, existingVolumes)
	if err != nil {
		return err
	}
	actualVolumes, err := volumemount.CreateOrUpdateCsiAzureVolumeResources(ctx, s.kubeUtil.KubeClient(), rd, namespace, jobComponent, desiredVolumes)
	if err != nil {
		return fmt.Errorf("failed to create or update csi azure volume resources: %w", err)
	}

	for i, batchJob := range s.radixBatch.Spec.Jobs {
		if err := s.reconcileService(ctx, &batchJob, rd, jobComponent, existingServices); err != nil {
			return fmt.Errorf("batchjob %s: failed to reconcile service: %w", batchJob.Name, err)
		}

		if err := s.reconcileKubeJob(ctx, &batchJob, rd, jobComponent, existingJobs, actualVolumes); err != nil {
			return fmt.Errorf("batchjob %s: failed to reconcile kubejob: %w", batchJob.Name, err)
		}

		if i%syncStatusForEveryNumberOfBatchJobsReconciled == (syncStatusForEveryNumberOfBatchJobsReconciled - 1) {
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

func (s *syncer) jobRequiresRestart(job radixv1.RadixBatchJob) bool {
	if job.Restart == "" {
		return false
	}

	currentStatus, found := slice.FindFirst(s.radixBatch.Status.JobStatuses, func(jobStatus radixv1.RadixBatchJobStatus) bool {
		return jobStatus.Name == job.Name
	})

	return !found || job.Restart != currentStatus.Restart
}

func (s *syncer) isBatchDone() bool {
	return s.radixBatch.Status.Condition.Type == radixv1.BatchConditionTypeCompleted
}

func (s *syncer) isBatchJobDone(batchJobName string) bool {
	return slice.Any(s.radixBatch.Status.JobStatuses,
		func(jobStatus radixv1.RadixBatchJobStatus) bool {
			return jobStatus.Name == batchJobName && isJobStatusDone(jobStatus)
		})
}

func (s *syncer) isRestartRequestedForAnyBatchJob() bool {
	return slice.Any(s.radixBatch.Spec.Jobs, s.jobRequiresRestart)
}

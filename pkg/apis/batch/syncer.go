package batch

import (
	"context"
	"fmt"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/clock"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/pkg/apis/volumemount"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
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

func WithClock(clock clock.Clock) SyncerOption {
	return func(syncer *syncer) {
		syncer.clock = clock
	}
}

// NewSyncer Constructor os RadixBatches Syncer
func NewSyncer(kubeclient kubernetes.Interface, kubeUtil *kube.Kube, radixClient radixclient.Interface, registration *radixv1.RadixRegistration, radixBatch *radixv1.RadixBatch, config config.Config, options ...SyncerOption) Syncer {
	syncer := &syncer{
		kubeClient:    kubeclient,
		kubeUtil:      kubeUtil,
		radixClient:   radixClient,
		radixBatch:    radixBatch,
		registration:  registration,
		config:        config,
		restartedJobs: map[string]radixv1.RadixBatchJob{},
		clock:         clock.RealClock{},
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
	registration  *radixv1.RadixRegistration
	config        config.Config
	restartedJobs map[string]radixv1.RadixBatchJob
	clock         clock.Clock
}

// OnSync Syncs RadixBatches
func (s *syncer) OnSync(ctx context.Context) error {
	ctx = log.Ctx(ctx).With().Str("resource_kind", radixv1.KindRadixBatch).Logger().WithContext(ctx)
	log.Ctx(ctx).Info().Msg("Syncing")

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

	// Volumes are ensured at most once per reconcile, and only when a Kubernetes Job is going to be created.
	// The RadixDeployment referenced by the batch may be an older, inactive RadixDeployment; its jobs get the storage defined in it.
	// Ensuring volumes never deletes anything, storage that is no longer needed is deleted when the RadixDeployment syncs.
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount
	volumesEnsured := false

	for i, batchJob := range s.radixBatch.Spec.Jobs {
		if err := s.reconcileService(ctx, &batchJob, rd, jobComponent, existingServices); err != nil {
			return fmt.Errorf("batchjob %s: failed to reconcile service: %w", batchJob.Name, err)
		}

		if !volumesEnsured && s.kubeJobMustBeCreated(&batchJob, existingJobs) {
			if volumes, volumeMounts, err = volumemount.EnsureComponentVolumes(ctx, s.kubeUtil, s.registration.Name, rd, jobComponent); err != nil {
				return fmt.Errorf("batchjob %s: failed to ensure volumes: %w", batchJob.Name, err)
			}
			volumesEnsured = true
		}

		if err := s.reconcileKubeJob(ctx, &batchJob, rd, jobComponent, existingJobs, volumes, volumeMounts); err != nil {
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
		radixlabels.ForApplicationID(s.registration.Spec.AppID),
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

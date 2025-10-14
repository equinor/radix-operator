package batch

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/job-scheduler/internal"
	"github.com/equinor/radix-operator/job-scheduler/models"
	pkgInternal "github.com/equinor/radix-operator/job-scheduler/pkg/internal"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// History Interface for job History
type History interface {
	// Cleanup the pipeline job history for the Radix application
	Cleanup(ctx context.Context, existingRadixBatchNamesMap map[string]struct{}) error
}

type history struct {
	kubeUtil                *kube.Kube
	env                     *models.Env
	radixDeployJobComponent *radixv1.RadixDeployJobComponent
	lastCleanupTime         time.Time
}

// NewHistory Constructor for job History
func NewHistory(kubeUtil *kube.Kube, env *models.Env, radixDeployJobComponent *radixv1.RadixDeployJobComponent) History {
	return &history{
		kubeUtil:                kubeUtil,
		env:                     env,
		radixDeployJobComponent: radixDeployJobComponent,
	}
}

// Cleanup the pipeline job history
func (h *history) Cleanup(ctx context.Context, existingRadixBatchNamesMap map[string]struct{}) error {
	logger := log.Ctx(ctx)
	const (
		minimumAgeSeconds    = 3600 // TODO add as default env-var and/or job-component property
		cleanupPeriodSeconds = 60
	)
	if h.lastCleanupTime.After(time.Now().Add(-time.Second * cleanupPeriodSeconds)) {
		logger.Debug().Msg("skip cleanup RadixBatch history")
		return nil
	}
	h.lastCleanupTime = time.Now()
	completedBefore := time.Now().Add(-time.Second * minimumAgeSeconds)
	completedRadixBatches, err := h.getCompletedRadixBatchesSortedByCompletionTimeAsc(ctx, completedBefore)
	if err != nil {
		return err
	}

	logger.Debug().Msg("cleanup RadixBatch history for succeeded batches")
	var errs []error
	historyLimit := h.env.RadixJobSchedulersPerEnvironmentHistoryLimit
	if err := h.cleanupRadixBatchHistory(ctx, completedRadixBatches.SucceededRadixBatches, historyLimit, existingRadixBatchNamesMap); err != nil {
		errs = append(errs, err)
	}
	logger.Debug().Msg("cleanup RadixBatch history for not succeeded batches")
	if err := h.cleanupRadixBatchHistory(ctx, completedRadixBatches.NotSucceededRadixBatches, historyLimit, existingRadixBatchNamesMap); err != nil {
		errs = append(errs, err)
	}
	logger.Debug().Msg("cleanup RadixBatch history for succeeded single jobs")
	if err := h.cleanupRadixBatchHistory(ctx, completedRadixBatches.SucceededSingleJobs, historyLimit, existingRadixBatchNamesMap); err != nil {
		errs = append(errs, err)
	}
	logger.Debug().Msg("cleanup RadixBatch history for not succeeded single jobs")
	if err := h.cleanupRadixBatchHistory(ctx, completedRadixBatches.NotSucceededSingleJobs, historyLimit, existingRadixBatchNamesMap); err != nil {
		errs = append(errs, err)
	}
	logger.Debug().Msg("delete orphaned payload secrets")
	if err = internal.GarbageCollectPayloadSecrets(ctx, h.kubeUtil, h.env.RadixDeploymentNamespace, h.env.RadixComponentName); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (h *history) getCompletedRadixBatchesSortedByCompletionTimeAsc(ctx context.Context, completedBefore time.Time) (*CompletedRadixBatchNames, error) {
	radixBatches, err := internal.GetRadixBatches(ctx, h.kubeUtil.RadixClient(), h.env.RadixDeploymentNamespace, radixLabels.ForComponentName(h.env.RadixComponentName))
	if err != nil {
		return nil, err
	}
	radixBatches = sortRJSchByCompletionTimeAsc(radixBatches)
	return &CompletedRadixBatchNames{
		SucceededRadixBatches:    h.getSucceededRadixBatches(radixBatches, completedBefore),
		NotSucceededRadixBatches: h.getNotSucceededRadixBatches(radixBatches, completedBefore),
		SucceededSingleJobs:      h.getSucceededSingleJobs(radixBatches, completedBefore),
		NotSucceededSingleJobs:   h.getNotSucceededSingleJobs(radixBatches, completedBefore),
	}, nil
}

func (h *history) getNotSucceededRadixBatches(radixBatches []*radixv1.RadixBatch, completedBefore time.Time) []string {
	return slice.Reduce(radixBatches, []string{}, func(acc []string, radixBatch *radixv1.RadixBatch) []string {
		if radixBatchHasType(radixBatch, kube.RadixBatchTypeBatch) && isRadixBatchNotSucceeded(radixBatch) && radixBatchIsCompletedBefore(completedBefore, radixBatch) {
			acc = append(acc, radixBatch.Name)
		}
		return acc
	})
}

func (h *history) getSucceededRadixBatches(radixBatches []*radixv1.RadixBatch, completedBefore time.Time) []string {
	return slice.Reduce(radixBatches, []string{}, func(acc []string, radixBatch *radixv1.RadixBatch) []string {
		if radixBatchHasType(radixBatch, kube.RadixBatchTypeBatch) && isRadixBatchSucceeded(radixBatch) && radixBatchIsCompletedBefore(completedBefore, radixBatch) {
			acc = append(acc, radixBatch.Name)
		}
		return acc
	})
}

func radixBatchIsCompletedBefore(completedBefore time.Time, radixBatch *radixv1.RadixBatch) bool {
	return radixBatch.Status.Condition.CompletionTime != nil && (*radixBatch.Status.Condition.CompletionTime).Before(&metav1.Time{Time: completedBefore})
}

func (h *history) getNotSucceededSingleJobs(radixBatches []*radixv1.RadixBatch, completedBefore time.Time) []string {
	return slice.Reduce(radixBatches, []string{}, func(acc []string, radixBatch *radixv1.RadixBatch) []string {
		if radixBatchHasType(radixBatch, kube.RadixBatchTypeJob) && isRadixBatchNotSucceeded(radixBatch) && radixBatchIsCompletedBefore(completedBefore, radixBatch) {
			acc = append(acc, radixBatch.Name)
		}
		return acc
	})
}

func (h *history) getSucceededSingleJobs(radixBatches []*radixv1.RadixBatch, completedBefore time.Time) []string {
	return slice.Reduce(radixBatches, []string{}, func(acc []string, radixBatch *radixv1.RadixBatch) []string {
		if radixBatchHasType(radixBatch, kube.RadixBatchTypeJob) && isRadixBatchSucceeded(radixBatch) && radixBatchIsCompletedBefore(completedBefore, radixBatch) {
			acc = append(acc, radixBatch.Name)
		}
		return acc
	})
}

func radixBatchHasType(radixBatch *radixv1.RadixBatch, radixBatchType kube.RadixBatchType) bool {
	return radixBatch.GetLabels()[kube.RadixBatchTypeLabel] == string(radixBatchType)
}

func (h *history) cleanupRadixBatchHistory(ctx context.Context, radixBatchNamesSortedByCompletionTimeAsc []string, historyLimit int, existingRadixBatchNamesMap map[string]struct{}) error {
	logger := log.Ctx(ctx)
	numToDelete := len(radixBatchNamesSortedByCompletionTimeAsc) - historyLimit
	if numToDelete <= 0 {
		logger.Debug().Msgf("no history batches to delete: %d batches, %d history limit", len(radixBatchNamesSortedByCompletionTimeAsc), historyLimit)
		return nil
	}
	logger.Debug().Msgf("history batches to delete: %v", numToDelete)

	for i := 0; i < numToDelete; i++ {
		if ctx.Err() != nil {
			return nil
		}
		radixBatchName := radixBatchNamesSortedByCompletionTimeAsc[i]
		logger.Debug().Msgf("deleting batch %s", radixBatchName)
		if err := DeleteRadixBatchByName(ctx, h.kubeUtil.RadixClient(), h.env.RadixDeploymentNamespace, radixBatchName); err != nil {
			return err
		}
		delete(existingRadixBatchNamesMap, radixBatchName)
	}
	return nil
}

func sortRJSchByCompletionTimeAsc(batches []*radixv1.RadixBatch) []*radixv1.RadixBatch {
	sort.Slice(batches, func(i, j int) bool {
		batch1 := (batches)[i]
		batch2 := (batches)[j]
		return isRJS1CompletedBeforeRJS2(batch1, batch2)
	})
	return batches
}

func isRJS1CompletedBeforeRJS2(batch1 *radixv1.RadixBatch, batch2 *radixv1.RadixBatch) bool {
	rd1ActiveFrom := getCompletionTimeFrom(batch1)
	rd2ActiveFrom := getCompletionTimeFrom(batch2)

	return rd1ActiveFrom.Before(rd2ActiveFrom)
}

func getCompletionTimeFrom(radixBatch *radixv1.RadixBatch) *metav1.Time {
	if radixBatch.Status.Condition.CompletionTime.IsZero() {
		return pointers.Ptr(radixBatch.GetCreationTimestamp())
	}
	return radixBatch.Status.Condition.CompletionTime
}

func isRadixBatchNotSucceeded(batch *radixv1.RadixBatch) bool {
	return batch.Status.Condition.Type == radixv1.BatchConditionTypeCompleted && slice.Any(batch.Status.JobStatuses, func(jobStatus radixv1.RadixBatchJobStatus) bool {
		return !pkgInternal.IsRadixBatchJobSucceeded(jobStatus)
	})
}

func isRadixBatchSucceeded(batch *radixv1.RadixBatch) bool {
	return batch.Status.Condition.Type == radixv1.BatchConditionTypeCompleted && slice.All(batch.Status.JobStatuses, func(jobStatus radixv1.RadixBatchJobStatus) bool {
		return pkgInternal.IsRadixBatchJobSucceeded(jobStatus)
	})
}

package batch

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/job-scheduler/internal"
	modelsv1 "github.com/equinor/radix-operator/job-scheduler/models/v1"
	apiErrors "github.com/equinor/radix-operator/job-scheduler/pkg/errors"
	pkgInternal "github.com/equinor/radix-operator/job-scheduler/pkg/internal"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

// CompletedRadixBatchNames Completed BatchStatus lists
type CompletedRadixBatchNames struct {
	SucceededRadixBatches    []string
	NotSucceededRadixBatches []string
	SucceededSingleJobs      []string
	NotSucceededSingleJobs   []string
}

// GetRadixBatchStatus Get radix batch
func GetRadixBatchStatus(radixBatch *radixv1.RadixBatch, radixDeployJobComponent *radixv1.RadixDeployJobComponent) *modelsv1.BatchStatus {
	var started, ended *time.Time
	if radixBatch.Status.Condition.ActiveTime != nil {
		started = &radixBatch.Status.Condition.ActiveTime.Time
	}
	if radixBatch.Status.Condition.CompletionTime != nil {
		ended = &radixBatch.Status.Condition.CompletionTime.Time
	}
	batchType := radixBatch.Labels[kube.RadixBatchTypeLabel]
	return &modelsv1.BatchStatus{
		JobStatus: modelsv1.JobStatus{
			Name:           radixBatch.Name,
			BatchId:        utils.TernaryString(batchType == string(kube.RadixBatchTypeJob), "", radixBatch.Spec.BatchId),
			Created:        pointers.Ptr(radixBatch.GetCreationTimestamp().Time),
			Started:        started,
			Ended:          ended,
			Status:         pkgInternal.GetRadixBatchStatus(radixBatch, radixDeployJobComponent),
			Message:        radixBatch.Status.Condition.Message,
			DeploymentName: radixBatch.Spec.RadixDeploymentJobRef.Name,
		},
		JobStatuses: getRadixBatchJobStatusesFromRadixBatch(radixBatch, radixBatch.Status.JobStatuses),
		BatchType:   batchType,
	}
}

// DeleteRadixBatchByName Delete a batch by name
func DeleteRadixBatchByName(ctx context.Context, radixClient versioned.Interface, namespace, batchName string) error {
	radixBatch, err := internal.GetRadixBatch(ctx, radixClient, namespace, batchName)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return DeleteRadixBatch(ctx, radixClient, radixBatch)
}

func getRadixBatchJobStatusesFromRadixBatch(radixBatch *radixv1.RadixBatch, radixBatchJobStatuses []radixv1.RadixBatchJobStatus) []modelsv1.JobStatus {
	radixBatchJobsStatuses := getRadixBatchJobsStatusesMap(radixBatchJobStatuses)
	jobStatuses := make([]modelsv1.JobStatus, 0, len(radixBatch.Spec.Jobs))
	for _, radixBatchJob := range radixBatch.Spec.Jobs {
		stopJob := radixBatchJob.Stop != nil && *radixBatchJob.Stop
		jobName := fmt.Sprintf("%s-%s", radixBatch.Name, radixBatchJob.Name) // composed name in models are always consist of a batchName and original jobName
		radixBatchJobStatus := modelsv1.JobStatus{
			Name:  jobName,
			JobId: radixBatchJob.JobId,
		}

		if jobStatus, ok := radixBatchJobsStatuses[radixBatchJob.Name]; ok {
			if jobStatus.CreationTime != nil {
				radixBatchJobStatus.Created = &jobStatus.CreationTime.Time
			}
			if jobStatus.StartTime != nil {
				radixBatchJobStatus.Started = &jobStatus.StartTime.Time
			}
			if jobStatus.EndTime != nil {
				radixBatchJobStatus.Ended = &jobStatus.EndTime.Time
			}
			radixBatchJobStatus.Status = string(pkgInternal.GetScheduledJobStatus(jobStatus, stopJob))
			radixBatchJobStatus.Message = jobStatus.Message
			radixBatchJobStatus.Failed = jobStatus.Failed
			radixBatchJobStatus.Restart = jobStatus.Restart
			radixBatchJobStatus.PodStatuses = pkgInternal.GetPodStatusByRadixBatchJobPodStatus(radixBatch, jobStatus.RadixBatchJobPodStatuses)
		} else {
			radixBatchJobStatus.Status = string(pkgInternal.GetScheduledJobStatus(radixv1.RadixBatchJobStatus{Phase: radixv1.RadixBatchJobApiStatusWaiting}, stopJob))
		}
		jobStatuses = append(jobStatuses, radixBatchJobStatus)
	}
	return jobStatuses
}

func getRadixBatchJobsStatusesMap(radixBatchJobStatuses []radixv1.RadixBatchJobStatus) map[string]radixv1.RadixBatchJobStatus {
	radixBatchJobsStatuses := make(map[string]radixv1.RadixBatchJobStatus)
	for _, jobStatus := range radixBatchJobStatuses {
		jobStatus := jobStatus
		radixBatchJobsStatuses[jobStatus.Name] = jobStatus
	}
	return radixBatchJobsStatuses
}

// GetRadixBatchStatuses Convert to radix batch statuses
func GetRadixBatchStatuses(radixBatches []*radixv1.RadixBatch, radixDeployJobComponent *radixv1.RadixDeployJobComponent) []modelsv1.BatchStatus {
	return slice.Reduce(radixBatches, make([]modelsv1.BatchStatus, 0, len(radixBatches)), func(acc []modelsv1.BatchStatus, radixBatch *radixv1.RadixBatch) []modelsv1.BatchStatus {
		return append(acc, *GetRadixBatchStatus(radixBatch, radixDeployJobComponent))
	})
}

// CopyRadixBatchOrJob Copy the Radix batch or job
func CopyRadixBatchOrJob(ctx context.Context, radixClient versioned.Interface, sourceRadixBatch *radixv1.RadixBatch, sourceJobName string,
	radixDeployJobComponent *radixv1.RadixDeployJobComponent, radixDeploymentName string) (*modelsv1.BatchStatus, error) {
	radixComponentName := radixDeployJobComponent.GetName()
	logger := log.Ctx(ctx)
	logger.Info().Msgf("copy a jobs %s of the batch %s", sourceJobName, sourceRadixBatch.GetName())
	radixBatch := radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{
			Name:   internal.GenerateBatchName(radixComponentName),
			Labels: sourceRadixBatch.GetLabels(),
		},
		Spec: radixv1.RadixBatchSpec{
			BatchId: sourceRadixBatch.Spec.BatchId,
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: radixDeploymentName},
				Job:                  radixComponentName,
			},
			Jobs: copyBatchJobs(ctx, sourceRadixBatch, sourceJobName),
		},
	}

	logger.Debug().Msgf("Create the copied Radix Batch %s with %d jobs in the cluster", radixBatch.GetName(), len(radixBatch.Spec.Jobs))
	createdRadixBatch, err := radixClient.RadixV1().RadixBatches(sourceRadixBatch.GetNamespace()).Create(ctx, &radixBatch, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	logger.Debug().Msgf("copied the batch %s for the component %s", radixBatch.GetName(), radixComponentName)
	return GetRadixBatchStatus(createdRadixBatch, radixDeployJobComponent), nil
}

// StopRadixBatch Stop a batch
func StopRadixBatch(ctx context.Context, radixClient versioned.Interface, appName, envName, jobComponentName, batchName string) error {
	return stopRadixBatchJob(ctx, radixClient, appName, envName, jobComponentName, kube.RadixBatchTypeBatch, batchName, "")
}

func stopRadixBatchJob(ctx context.Context, radixClient versioned.Interface, appName, envName, jobComponentName string, batchType kube.RadixBatchType, batchName, jobName string) error {
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	logger := log.Ctx(ctx)
	logger.Info().Msgf("stop the batch %s for the application %s, environment %s", batchName, appName, envName)
	batchesSelector := radixLabels.ForComponentName(jobComponentName)
	if batchType != "" {
		batchesSelector = radixLabels.Merge(batchesSelector, radixLabels.ForBatchType(batchType))
	}
	var externalError error
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		radixBatches, err := internal.GetRadixBatches(ctx, radixClient, namespace, batchesSelector)
		if err != nil {
			return err
		}
		var foundBatch, foundJob bool
		foundBatch, foundJob, externalError, err = stopJobsInBatches(radixBatches, batchName, jobName, radixClient, namespace, ctx)
		if err != nil {
			return err
		}
		if externalError != nil {
			return nil
		}
		if len(batchName) > 0 && !foundBatch {
			externalError = k8sErrors.NewNotFound(radixv1.Resource("radixbatches"), batchName)
		} else if len(jobName) > 0 && !foundJob {
			externalError = k8sErrors.NewNotFound(radixv1.Resource("radixbatches.job"), jobName)
		}
		return nil
	})
	if err != nil {
		return apiErrors.NewFromError(fmt.Errorf("failed to update the batch %s: %w", batchName, err))
	}
	if externalError != nil {
		return externalError
	}
	logger.Debug().Msgf("Patched BatchStatus: %s in namespace %s", batchName, namespace)
	return nil
}

func stopJobsInBatches(radixBatches []*radixv1.RadixBatch, batchName string, jobName string, radixClient versioned.Interface, namespace string, ctx context.Context) (bool, bool, error, error) {
	var foundBatch, foundJob, appliedChanges bool
	var updateJobErrors []error
	for _, radixBatch := range radixBatches {
		if len(batchName) > 0 {
			if strings.EqualFold(radixBatch.GetName(), batchName) {
				foundBatch = true
			} else {
				continue
			}
		}
		if !isBatchStoppable(radixBatch.Status.Condition) {
			if len(batchName) == 0 {
				continue
			}
			if radixBatch.GetLabels()[kube.RadixBatchTypeLabel] == string(kube.RadixBatchTypeJob) {
				return false, false, apiErrors.NewBadRequest(fmt.Sprintf("cannot stop the job %s with the status %s", radixBatch.GetName(), radixBatch.Status.Condition.Type)), nil
			}
			return false, false, apiErrors.NewBadRequest(fmt.Sprintf("cannot stop the batch %s with the status %s", batchName, radixBatch.Status.Condition.Type)), nil
		}
		newRadixBatch := radixBatch.DeepCopy()
		var err error
		foundJob, appliedChanges, err = stopJobsInBatch(newRadixBatch, batchName, jobName, foundJob)
		if err != nil {
			return false, false, err, nil
		}
		if appliedChanges {
			_, err = radixClient.RadixV1().RadixBatches(namespace).Update(ctx, newRadixBatch, metav1.UpdateOptions{})
			if err != nil {
				updateJobErrors = append(updateJobErrors, err)
			}
		}
	}
	if len(updateJobErrors) > 0 {
		return false, false, nil, errors.Join(updateJobErrors...)
	}
	return foundBatch, foundJob, nil, nil
}

func stopJobsInBatch(radixBatch *radixv1.RadixBatch, batchName string, jobName string, foundJob bool) (bool, bool, error) {
	var appliedChanges bool
	for jobIndex, radixBatchJob := range radixBatch.Spec.Jobs {
		if len(jobName) > 0 {
			if strings.EqualFold(radixBatchJob.Name, jobName) {
				foundJob = true
			} else {
				continue
			}
		}
		if jobStatus, ok := slice.FindFirst(radixBatch.Status.JobStatuses, func(status radixv1.RadixBatchJobStatus) bool {
			return status.Name == radixBatchJob.Name
		}); ok &&
			(pkgInternal.IsRadixBatchJobSucceeded(jobStatus) || isRadixBatchJobFailed(jobStatus)) {
			if len(batchName) > 0 && radixBatch.GetLabels()[kube.RadixBatchTypeLabel] == string(kube.RadixBatchTypeJob) {
				return false, false, apiErrors.NewBadRequest(fmt.Sprintf("cannot stop the job %s with the status %s", radixBatch.GetName(), jobStatus.Phase))
			}
			if len(jobName) > 0 {
				return false, false, apiErrors.NewBadRequest(fmt.Sprintf("cannot stop the job %s with the status %s in the batch %s", jobName, jobStatus.Phase, batchName))
			}
			continue
		}
		radixBatch.Spec.Jobs[jobIndex].Stop = pointers.Ptr(true)
		appliedChanges = true
		if foundJob {
			break
		}
	}
	return foundJob, appliedChanges, nil
}

// StopAllRadixBatches Stop all batches
func StopAllRadixBatches(ctx context.Context, radixClient versioned.Interface, appName, envName, jobComponentName string, batchType kube.RadixBatchType) error {
	return stopRadixBatchJob(ctx, radixClient, appName, envName, jobComponentName, batchType, "", "")
}

// StopRadixBatchJob Stop a job
func StopRadixBatchJob(ctx context.Context, radixClient versioned.Interface, appName, envName, jobComponentName, batchName, jobName string) error {
	return stopRadixBatchJob(ctx, radixClient, appName, envName, jobComponentName, "", batchName, jobName)
}

// RestartRadixBatch Restart a batch
func RestartRadixBatch(ctx context.Context, radixClient versioned.Interface, radixBatch *radixv1.RadixBatch) error {
	logger := log.Ctx(ctx)
	logger.Info().Msgf("restart the batch %s", radixBatch.GetName())
	restartTimestamp := utils.FormatTimestamp(time.Now())
	for jobIdx := 0; jobIdx < len(radixBatch.Spec.Jobs); jobIdx++ {
		setRestartJobTimeout(radixBatch, jobIdx, restartTimestamp)
	}
	if _, err := radixClient.RadixV1().RadixBatches(radixBatch.GetNamespace()).Update(ctx, radixBatch, metav1.UpdateOptions{}); err != nil {
		return err
	}
	return nil
}

// RestartRadixBatchJob Restart a job
func RestartRadixBatchJob(ctx context.Context, radixClient versioned.Interface, radixBatch *radixv1.RadixBatch, jobName string) error {
	logger := log.Ctx(ctx)
	logger.Info().Msgf("restart a job %s in the batch %s", jobName, radixBatch.GetName())
	jobIdx := slice.FindIndex(radixBatch.Spec.Jobs, func(job radixv1.RadixBatchJob) bool { return job.Name == jobName })
	if jobIdx == -1 {
		return fmt.Errorf("job %s not found", jobName)
	}
	setRestartJobTimeout(radixBatch, jobIdx, utils.FormatTimestamp(time.Now()))
	if _, err := radixClient.RadixV1().RadixBatches(radixBatch.GetNamespace()).Update(ctx, radixBatch, metav1.UpdateOptions{}); err != nil {
		return err
	}
	return nil
}

// DeleteRadixBatch Delete a batch
func DeleteRadixBatch(ctx context.Context, radixClient versioned.Interface, radixBatch *radixv1.RadixBatch) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("delete batch %s", radixBatch.GetName())
	if err := radixClient.RadixV1().RadixBatches(radixBatch.GetNamespace()).Delete(ctx, radixBatch.GetName(), metav1.DeleteOptions{PropagationPolicy: pointers.Ptr(metav1.DeletePropagationBackground)}); err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}
	return nil
}

func copyBatchJobs(ctx context.Context, sourceRadixBatch *radixv1.RadixBatch, sourceJobName string) []radixv1.RadixBatchJob {
	logger := log.Ctx(ctx)
	radixBatchJobs := make([]radixv1.RadixBatchJob, 0, len(sourceRadixBatch.Spec.Jobs))
	for _, sourceJob := range sourceRadixBatch.Spec.Jobs {
		if sourceJobName != "" && sourceJob.Name != sourceJobName {
			continue
		}
		job := sourceJob.DeepCopy()
		job.Name = internal.CreateJobName()
		logger.Debug().Msgf("Copy Radix Batch Job %s", job.Name)
		radixBatchJobs = append(radixBatchJobs, *job)
	}
	return radixBatchJobs
}

// check if batch can be stopped
func isBatchStoppable(condition radixv1.RadixBatchCondition) bool {
	return condition.Type == "" ||
		condition.Type == radixv1.BatchConditionTypeActive ||
		condition.Type == radixv1.BatchConditionTypeWaiting
}

func setRestartJobTimeout(batch *radixv1.RadixBatch, jobIdx int, restartTimestamp string) {
	batch.Spec.Jobs[jobIdx].Stop = nil
	batch.Spec.Jobs[jobIdx].Restart = restartTimestamp
}

func isRadixBatchJobFailed(jobStatus radixv1.RadixBatchJobStatus) bool {
	return jobStatus.Phase == radixv1.BatchJobPhaseFailed
}

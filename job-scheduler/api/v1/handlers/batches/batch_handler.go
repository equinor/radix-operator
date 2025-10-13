package batchesv1

import (
	"context"
	"fmt"

	handlerInternal "github.com/equinor/radix-operator/job-scheduler/api/v1/handlers/internal"
	"github.com/equinor/radix-operator/job-scheduler/internal"
	"github.com/equinor/radix-operator/job-scheduler/models"
	"github.com/equinor/radix-operator/job-scheduler/models/common"
	modelsv1 "github.com/equinor/radix-operator/job-scheduler/models/v1"
	"github.com/equinor/radix-operator/job-scheduler/pkg/batch"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
)

type batchHandler struct {
	handlerInternal.Handler
}

type BatchHandler interface {
	// GetBatches Get status of all batches
	GetBatches(ctx context.Context) ([]modelsv1.BatchStatus, error)
	// GetBatch Get status of a batch
	GetBatch(ctx context.Context, batchName string) (*modelsv1.BatchStatus, error)
	// GetBatchJob Get status of a batch job
	GetBatchJob(ctx context.Context, batchName string, jobName string) (*modelsv1.JobStatus, error)
	// CreateBatch Create a batch with parameters
	CreateBatch(ctx context.Context, batchScheduleDescription *common.BatchScheduleDescription) (*modelsv1.BatchStatus, error)
	// CopyBatch creates a copy of an existing batch with deploymentName as value for radixDeploymentJobRef.name
	CopyBatch(ctx context.Context, batchName string, deploymentName string) (*modelsv1.BatchStatus, error)
	// DeleteBatch Delete a batch
	DeleteBatch(ctx context.Context, batchName string) error
	// StopBatch Stop a batch
	StopBatch(ctx context.Context, batchName string) error
	// StopBatchJob Stop a batch job
	StopBatchJob(ctx context.Context, batchName, jobName string) error
	// StopAllBatches Stop all batches
	StopAllBatches(ctx context.Context) error
}

// New Constructor of the batch handler
func New(kube *kube.Kube, env *models.Env, radixDeployJobComponent *radixv1.RadixDeployJobComponent) BatchHandler {
	return &batchHandler{handlerInternal.New(kube, env, radixDeployJobComponent)}
}

// GetBatches Get status of all batches
func (handler *batchHandler) GetBatches(ctx context.Context) ([]modelsv1.BatchStatus, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Get batches for the namespace: %s", handler.GetEnv().RadixDeploymentNamespace)

	batchesStatuses, err := handler.GetRadixBatchStatuses(ctx)
	if err != nil {
		return nil, err
	}
	if len(batchesStatuses) == 0 {
		logger.Debug().Msgf("No batches found for namespace %s", handler.GetEnv().RadixDeploymentNamespace)
		return nil, nil
	}

	labelSelectorForAllRadixBatchesPods := handlerInternal.GetLabelSelectorForAllRadixBatchesPods(handler.GetEnv().RadixComponentName)
	eventMessageForPods, batchJobPodsMap, err := handler.GetRadixBatchJobMessagesAndPodMaps(ctx, labelSelectorForAllRadixBatchesPods)
	if err != nil {
		return nil, err
	}
	for i := range batchesStatuses {
		setBatchJobEventMessages(&batchesStatuses[i], batchJobPodsMap, eventMessageForPods)
	}
	logger.Debug().Msgf("Found %v batches for namespace %s", len(batchesStatuses), handler.GetEnv().RadixDeploymentNamespace)
	return batchesStatuses, nil
}

// GetBatchJob Get status of a batch job
func (handler *batchHandler) GetBatchJob(ctx context.Context, batchName string, jobName string) (*modelsv1.JobStatus, error) {
	batchStatus, err := handler.GetRadixBatchStatus(ctx, batchName)
	if err != nil {
		return nil, err
	}
	return handlerInternal.GetBatchJobStatus(batchStatus, jobName)
}

// GetBatch Get status of a batch
func (handler *batchHandler) GetBatch(ctx context.Context, batchName string) (*modelsv1.BatchStatus, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("get batches for namespace: %s", handler.GetEnv().RadixDeploymentNamespace)
	batchStatus, err := handler.GetRadixBatchStatus(ctx, batchName)
	if err != nil {
		return nil, err
	}
	labelSelectorForRadixBatchesPods := handlerInternal.GetLabelSelectorForRadixBatchesPods(handler.GetEnv().RadixComponentName, batchName)
	eventMessageForPods, batchJobPodsMap, err := handler.GetRadixBatchJobMessagesAndPodMaps(ctx, labelSelectorForRadixBatchesPods)
	if err != nil {
		return nil, err
	}
	setBatchJobEventMessages(batchStatus, batchJobPodsMap, eventMessageForPods)
	return batchStatus, nil
}

// DeleteBatch Delete a batch
func (handler *batchHandler) DeleteBatch(ctx context.Context, batchName string) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("delete batch %s for namespace: %s", batchName, handler.GetEnv().RadixDeploymentNamespace)
	err := batch.DeleteRadixBatchByName(ctx, handler.GetKubeUtil().RadixClient(), handler.GetEnv().RadixDeploymentNamespace, batchName)
	if err != nil {
		return err
	}
	return internal.GarbageCollectPayloadSecrets(ctx, handler.GetKubeUtil(), handler.GetEnv().RadixDeploymentNamespace, handler.GetEnv().RadixComponentName)
}

// StopBatchJob Stop a batch job
func (handler *batchHandler) StopBatchJob(ctx context.Context, batchName string, jobName string) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("delete the job %s in the batch %s for namespace: %s", jobName, batchName, handler.GetEnv().RadixDeploymentNamespace)
	return handler.StopJob(ctx, fmt.Sprintf("%s-%s", batchName, jobName))
}

func setBatchJobEventMessages(radixBatchStatus *modelsv1.BatchStatus, batchJobPodsMap map[string]corev1.Pod, eventMessageForPods map[string]string) {
	for i := 0; i < len(radixBatchStatus.JobStatuses); i++ {
		handlerInternal.SetBatchJobEventMessageToBatchJobStatus(&radixBatchStatus.JobStatuses[i], batchJobPodsMap, eventMessageForPods)
	}
}

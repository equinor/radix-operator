package internal

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"

	"dario.cat/mergo"
	mergoutils "github.com/equinor/radix-common/utils/mergo"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/job-scheduler/internal"
	"github.com/equinor/radix-operator/job-scheduler/models"
	"github.com/equinor/radix-operator/job-scheduler/models/common"
	modelsv1 "github.com/equinor/radix-operator/job-scheduler/models/v1"
	"github.com/equinor/radix-operator/job-scheduler/pkg/batch"
	apiErrors "github.com/equinor/radix-operator/job-scheduler/pkg/errors"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Max size of the secret description, including description, metadata, base64 encodes secret values, etc.
	maxPayloadSecretSize = 1024 * 512 // 0.5MB
	// Standard secret description, metadata, etc.
	payloadSecretAuxDataSize = 600
	// Each entry in a secret Data has name, etc.
	payloadSecretEntryAuxDataSize = 128
)

var (
	jobDescriptionTransformer mergo.Transformers = mergoutils.CombinedTransformer{Transformers: []mergo.Transformers{mergoutils.BoolPtrTransformer{}, common.RuntimeTransformer{}}}
)

type Handler struct {
	kubeUtil                *kube.Kube
	env                     *models.Env
	radixDeployJobComponent *radixv1.RadixDeployJobComponent
	jobHistory              batch.History
}

// New Constructor of the batch Handler
func New(kubeUtil *kube.Kube, env *models.Env, radixDeployJobComponent *radixv1.RadixDeployJobComponent) Handler {
	return Handler{
		kubeUtil:                kubeUtil,
		env:                     env,
		radixDeployJobComponent: radixDeployJobComponent,
		jobHistory:              batch.NewHistory(kubeUtil, env, radixDeployJobComponent),
	}
}

type radixBatchJobWithDescription struct {
	radixBatchJob          *radixv1.RadixBatchJob
	jobScheduleDescription *common.JobScheduleDescription
}

// GetEnv Get environment information
func (h *Handler) GetEnv() *models.Env {
	return h.env
}

// GetKubeUtil Get kube utility
func (h *Handler) GetKubeUtil() *kube.Kube {
	return h.kubeUtil
}

// GetRadixBatchStatuses Get statuses of all batches
func (h *Handler) GetRadixBatchStatuses(ctx context.Context) ([]modelsv1.BatchStatus, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Get batches for the namespace: %s", h.env.RadixDeploymentNamespace)
	return h.getRadixBatchStatuses(ctx, kube.RadixBatchTypeBatch)
}

// GetRadixBatchStatusSingleJobs Get statuses of all single jobs
func (h *Handler) GetRadixBatchStatusSingleJobs(ctx context.Context) ([]modelsv1.BatchStatus, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Get sigle jobs for the namespace: %s", h.env.RadixDeploymentNamespace)
	return h.getRadixBatchStatuses(ctx, kube.RadixBatchTypeJob)
}

// GetRadixBatchStatus Get status of a batch
func (h *Handler) GetRadixBatchStatus(ctx context.Context, batchName string) (*modelsv1.BatchStatus, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("get batch status for the batch %s", batchName)
	radixBatch, err := internal.GetRadixBatch(ctx, h.kubeUtil.RadixClient(), h.env.RadixDeploymentNamespace, batchName)
	if err != nil {
		return nil, err
	}
	return batch.GetRadixBatchStatus(radixBatch, h.radixDeployJobComponent), nil
}

// CreateBatch Create a batch with parameters
func (h *Handler) CreateBatch(ctx context.Context, batchScheduleDescription *common.BatchScheduleDescription) (*modelsv1.BatchStatus, error) {
	logger := log.Ctx(ctx)
	if batchScheduleDescription == nil {
		return nil, apiErrors.NewInvalidWithReason("BatchScheduleDescription", "empty request body")
	}
	logger.Info().Msgf("Create Radix Batch for %d jobs", len(batchScheduleDescription.JobScheduleDescriptions))

	if len(batchScheduleDescription.JobScheduleDescriptions) == 0 {
		return nil, apiErrors.NewInvalidWithReason("BatchScheduleDescription", "empty job description list ")
	}

	radixBatch, err := h.createRadixBatchOrJob(ctx, *batchScheduleDescription, kube.RadixBatchTypeBatch)
	if err != nil {
		return nil, err
	}
	logger.Info().Msgf("Radix Batch %s has been created", radixBatch.Name)
	return radixBatch, nil
}

// CopyBatch Copy a batch with deployment and optional parameters
func (h *Handler) CopyBatch(ctx context.Context, sourceBatchName, deploymentName string) (*modelsv1.BatchStatus, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Copy Radix Batch %s for the deployment %s", sourceBatchName, deploymentName)
	sourceRadixBatch, err := internal.GetRadixBatch(ctx, h.kubeUtil.RadixClient(), h.env.RadixDeploymentNamespace, sourceBatchName)
	if err != nil {
		return nil, err
	}
	return batch.CopyRadixBatchOrJob(ctx, h.kubeUtil.RadixClient(), sourceRadixBatch, "", h.radixDeployJobComponent, deploymentName)
}

func (h *Handler) createRadixBatchOrJob(ctx context.Context, batchScheduleDescription common.BatchScheduleDescription, radixBatchType kube.RadixBatchType) (*modelsv1.BatchStatus, error) {
	logger := log.Ctx(ctx)
	namespace := h.env.RadixDeploymentNamespace
	radixComponentName := h.env.RadixComponentName
	radixDeploymentName := h.env.RadixDeploymentName
	logger.Info().Msgf("Create batch for namespace %s, component %s, deployment %s", namespace, radixComponentName, radixDeploymentName)

	radixDeployment, err := h.kubeUtil.RadixClient().RadixV1().RadixDeployments(namespace).Get(ctx, radixDeploymentName, metav1.GetOptions{})
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			return nil, apiErrors.NewNotFound("radix deployment", radixDeploymentName)
		}
		return nil, apiErrors.NewFromError(err)
	}

	radixJobComponent := radixDeployment.GetJobComponentByName(radixComponentName)
	if radixJobComponent == nil {
		return nil, apiErrors.NewNotFound("job component", radixComponentName)
	}

	appName := radixDeployment.Spec.AppName

	createdRadixBatch, err := h.createRadixBatch(ctx, namespace, appName, radixDeployment.GetName(), *radixJobComponent, batchScheduleDescription, radixBatchType)
	if err != nil {
		return nil, apiErrors.NewFromError(err)
	}

	logger.Debug().Msgf("created batch %s for component %s, environment %s, in namespace: %s", createdRadixBatch.GetName(),
		radixComponentName, radixDeployment.Spec.Environment, namespace)
	return batch.GetRadixBatchStatus(createdRadixBatch, h.radixDeployJobComponent), nil
}

// CreateRadixBatchSingleJob Create a batch single job with parameters
func (h *Handler) CreateRadixBatchSingleJob(ctx context.Context, jobScheduleDescription *common.JobScheduleDescription) (*modelsv1.BatchStatus, error) {
	logger := log.Ctx(ctx)
	logger.Info().Msg("Create Radix Batch single job")
	if jobScheduleDescription == nil {
		return nil, apiErrors.NewInvalidWithReason("JobScheduleDescription", "empty request body")
	}
	radixBatchJob, err := h.createRadixBatchOrJob(ctx, common.BatchScheduleDescription{
		JobScheduleDescriptions:        []common.JobScheduleDescription{*jobScheduleDescription},
		DefaultRadixJobComponentConfig: nil,
	}, kube.RadixBatchTypeJob)
	if err != nil {
		return nil, err
	}
	logger.Info().Msgf("Radix single job %s has been created", radixBatchJob.Name)
	return radixBatchJob, err
}

// CopyRadixBatchJob Copy a batch job
func (h *Handler) CopyRadixBatchJob(ctx context.Context, sourceJobName, deploymentName string) (*modelsv1.BatchStatus, error) {
	batchName, jobName, ok := internal.ParseBatchAndJobNameFromScheduledJobName(sourceJobName)
	if !ok {
		return nil, fmt.Errorf("copy of this job is not supported or invalid job name")
	}
	sourceRadixBatch, err := internal.GetRadixBatch(ctx, h.kubeUtil.RadixClient(), h.env.RadixDeploymentNamespace, batchName)
	if err != nil {
		return nil, err
	}
	return batch.CopyRadixBatchOrJob(ctx, h.kubeUtil.RadixClient(), sourceRadixBatch, jobName, h.radixDeployJobComponent, deploymentName)
}

// StopBatch Stop a batch
func (h *Handler) StopBatch(ctx context.Context, batchName string) error {
	return batch.StopRadixBatch(ctx, h.kubeUtil.RadixClient(), h.env.RadixAppName, h.env.RadixEnvironmentName, h.env.RadixComponentName, batchName)
}

// StopAllBatches Stop all batches
func (h *Handler) StopAllBatches(ctx context.Context) error {
	return batch.StopAllRadixBatches(ctx, h.kubeUtil.RadixClient(), h.env.RadixAppName, h.env.RadixEnvironmentName, h.env.RadixComponentName, kube.RadixBatchTypeBatch)
}

// StopJob Stop a batch job
func (h *Handler) StopJob(ctx context.Context, jobName string) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("stop the job %s for namespace: %s", jobName, h.env.RadixDeploymentNamespace)
	if batchName, batchJobName, ok := internal.ParseBatchAndJobNameFromScheduledJobName(jobName); ok {
		return batch.StopRadixBatchJob(ctx, h.kubeUtil.RadixClient(), h.env.RadixAppName, h.env.RadixEnvironmentName, h.env.RadixComponentName, batchName, batchJobName)
	}
	return fmt.Errorf("stop of this job is not supported")
}

// StopAllSingleRadixJobs Stop all single jobs
func (h *Handler) StopAllSingleRadixJobs(ctx context.Context) error {
	return batch.StopAllRadixBatches(ctx, h.kubeUtil.RadixClient(), h.env.RadixAppName, h.env.RadixEnvironmentName, h.env.RadixComponentName, kube.RadixBatchTypeJob)
}

func (h *Handler) createRadixBatch(ctx context.Context, namespace, appName, radixDeploymentName string, radixJobComponent radixv1.RadixDeployJobComponent, batchScheduleDescription common.BatchScheduleDescription, radixBatchType kube.RadixBatchType) (*radixv1.RadixBatch, error) {
	logger := log.Ctx(ctx)
	batchName := internal.GenerateBatchName(radixJobComponent.GetName())
	logger.Debug().Msgf("Create Radix Batch %s", batchName)
	radixJobComponentName := radixJobComponent.GetName()
	radixBatchJobs, err := h.buildRadixBatchJobs(ctx, namespace, appName, radixJobComponentName, batchName, batchScheduleDescription, radixJobComponent.Payload)
	if err != nil {
		return nil, err
	}
	logger.Debug().Msgf("Built Radix Batch with %d jobs", len(radixBatchJobs))
	radixBatch := radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{
			Name: batchName,
			Labels: radixLabels.Merge(
				radixLabels.ForApplicationName(appName),
				radixLabels.ForComponentName(radixJobComponentName),
				radixLabels.ForBatchType(radixBatchType),
			),
		},
		Spec: radixv1.RadixBatchSpec{
			BatchId: batchScheduleDescription.BatchId,
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: radixDeploymentName},
				Job:                  radixJobComponentName,
			},
		},
	}
	radixBatch.Spec.Jobs = radixBatchJobs
	logger.Debug().Msgf("Create Radix Batch in the cluster")
	createdRadixBatch, err := h.kubeUtil.RadixClient().RadixV1().RadixBatches(namespace).Create(ctx, &radixBatch,
		metav1.CreateOptions{})
	if err != nil {
		return nil, apiErrors.NewFromError(err)
	}
	return createdRadixBatch, nil
}

func (h *Handler) buildRadixBatchJobs(ctx context.Context, namespace, appName, radixJobComponentName, batchName string, batchScheduleDescription common.BatchScheduleDescription, radixJobComponentPayload *radixv1.RadixJobComponentPayload) ([]radixv1.RadixBatchJob, error) {
	logger := log.Ctx(ctx)
	var radixBatchJobWithDescriptions []radixBatchJobWithDescription
	var errs []error
	logger.Debug().Msg("Build Radix Batch")
	for _, jobScheduleDescription := range batchScheduleDescription.JobScheduleDescriptions {
		jobScheduleDescription := jobScheduleDescription.DeepCopy()
		logger.Debug().Msgf("Build Radix Batch Job. JobId: '%s', Payload length: %d", jobScheduleDescription.JobId, len(jobScheduleDescription.Payload))
		radixBatchJob, err := buildRadixBatchJob(jobScheduleDescription, batchScheduleDescription.DefaultRadixJobComponentConfig.DeepCopy())
		if err != nil {
			errs = append(errs, err)
			continue
		}
		logger.Debug().Msgf("Built  Radix Batch Job %s", radixBatchJob.Name)
		description := radixBatchJobWithDescription{
			radixBatchJob:          radixBatchJob,
			jobScheduleDescription: jobScheduleDescription,
		}
		radixBatchJobWithDescriptions = append(radixBatchJobWithDescriptions, description)
	}
	if len(errs) > 0 {
		return nil, apiErrors.NewFromError(errors.Join(errs...))
	}
	radixJobComponentHasPayloadPath := radixJobComponentPayload != nil && len(radixJobComponentPayload.Path) > 0
	err := h.createRadixBatchJobPayloadSecrets(ctx, namespace, appName, radixJobComponentName, batchName, radixBatchJobWithDescriptions, radixJobComponentHasPayloadPath)
	if err != nil {
		return nil, err
	}
	radixBatchJobs := make([]radixv1.RadixBatchJob, 0, len(radixBatchJobWithDescriptions))
	for _, item := range radixBatchJobWithDescriptions {
		radixBatchJobs = append(radixBatchJobs, *item.radixBatchJob)
	}
	return radixBatchJobs, nil
}

func (h *Handler) createRadixBatchJobPayloadSecrets(ctx context.Context, namespace, appName, radixJobComponentName, batchName string, radixJobWithDescriptions []radixBatchJobWithDescription, radixJobComponentHasPayloadPath bool) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Create Payload secrets for the batch %s", batchName)
	accumulatedSecretSize := 0
	var payloadSecrets []*corev1.Secret
	payloadsSecret := buildPayloadSecret(ctx, appName, radixJobComponentName, batchName, 0)
	payloadSecrets = append(payloadSecrets, payloadsSecret)
	var errs []error
	for jobIndex, radixJobWithDescriptions := range radixJobWithDescriptions {
		payload := []byte(strings.TrimSpace(radixJobWithDescriptions.jobScheduleDescription.Payload))
		if len(payload) == 0 {
			logger.Info().Msgf("No payload in the job #%d", jobIndex)
			continue
		}
		if !radixJobComponentHasPayloadPath {
			errs = append(errs, fmt.Errorf("missing an expected payload path, but there is a payload in the job #%d", jobIndex))
			continue
		}

		logger.Info().Msgf("Payload for the job #%d, JobId: '%s', length: %d", jobIndex, radixJobWithDescriptions.jobScheduleDescription.JobId, len(payload))
		radixBatchJob := radixJobWithDescriptions.radixBatchJob
		payloadBase64 := base64.RawStdEncoding.EncodeToString(payload)
		secretEntrySize := len(payloadBase64) + len(radixBatchJob.Name) + payloadSecretEntryAuxDataSize // preliminary estimate of a payload secret entry
		logger.Debug().Msgf("Prelimenary esptimated payload size: %d", secretEntrySize)
		newAccumulatedPayloadSecretSize := payloadSecretAuxDataSize + accumulatedSecretSize + secretEntrySize
		logger.Debug().Msgf("New evaluated accumulated payload size with aux secret data: %d", newAccumulatedPayloadSecretSize)
		if newAccumulatedPayloadSecretSize > maxPayloadSecretSize {
			if len(payloadsSecret.Data) == 0 {
				// this is the first entry in the secret, and it is too large to be stored to the secret - no reason to create new secret.
				return fmt.Errorf("payload is too large in the job #%d - its base64 size is %d bytes, but it is expected to be less then %d bytes", jobIndex, secretEntrySize, maxPayloadSecretSize)
			}
			logger.Debug().Msgf("New evaluated accumulated payload size is great then the max size %d - build a new payload secret", maxPayloadSecretSize)
			payloadsSecret = buildPayloadSecret(ctx, appName, radixJobComponentName, batchName, len(payloadSecrets))
			payloadSecrets = append(payloadSecrets, payloadsSecret)
			accumulatedSecretSize = 0
		}

		payloadsSecret.Data[radixBatchJob.Name] = payload
		accumulatedSecretSize = accumulatedSecretSize + secretEntrySize
		logger.Debug().Msgf("New accumulated payload size: %d", newAccumulatedPayloadSecretSize)
		logger.Debug().Msgf("Added a reference to the payload secret %s, key %s", payloadsSecret.GetName(), radixBatchJob.Name)
		radixBatchJob.PayloadSecretRef = &radixv1.PayloadSecretKeySelector{
			LocalObjectReference: radixv1.LocalObjectReference{Name: payloadsSecret.GetName()},
			Key:                  radixBatchJob.Name,
		}
	}
	if len(errs) > 0 {
		return apiErrors.NewFromError(errors.Join(errs...))
	}
	logger.Debug().Msg("Create payload secrets")
	return h.createSecrets(ctx, namespace, payloadSecrets)
}

func (h *Handler) createSecrets(ctx context.Context, namespace string, secrets []*corev1.Secret) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Create %d secrets", len(secrets))
	for _, secret := range secrets {
		if len(secret.Data) == 0 {
			logger.Debug().Msgf("Do not create a secret %s - Data is empty, the secret is not used in any jobs", secret.GetName())
			continue
		}
		logger.Debug().Msgf("Create a secret %s in the cluster", secret.GetName())
		_, err := h.kubeUtil.KubeClient().CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			return apiErrors.NewFromError(err)
		}
	}
	return nil
}

func buildPayloadSecret(ctx context.Context, appName, radixJobComponentName, batchName string, secretIndex int) *corev1.Secret {
	logger := log.Ctx(ctx)
	secretName := fmt.Sprintf("%s-payloads-%d", batchName, secretIndex)
	logger.Debug().Msgf("build payload secret %s", secretName)
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: radixLabels.Merge(
				radixLabels.ForApplicationName(appName),
				radixLabels.ForComponentName(radixJobComponentName),
				radixLabels.ForBatchName(batchName),
				radixLabels.ForJobScheduleJobType(),
				radixLabels.ForRadixSecretType(kube.RadixSecretJobPayload),
			),
		},
		Data: make(map[string][]byte),
	}
}

func buildRadixBatchJob(jobScheduleDescription *common.JobScheduleDescription, defaultJobScheduleDescription *common.RadixJobComponentConfig) (*radixv1.RadixBatchJob, error) {
	if err := applyDefaultJobDescriptionProperties(jobScheduleDescription, defaultJobScheduleDescription); err != nil {
		return nil, apiErrors.NewFromError(err)
	}
	batchJob := radixv1.RadixBatchJob{
		Name:             internal.CreateJobName(),
		JobId:            jobScheduleDescription.JobId,
		Resources:        jobScheduleDescription.Resources.MapToRadixResourceRequirements(),
		Node:             jobScheduleDescription.Node.MapToRadixNode(), // nolint:staticcheck // SA1019: Ignore linting deprecated fields
		Runtime:          jobScheduleDescription.Runtime.MapToRadixRuntime(),
		Variables:        jobScheduleDescription.Variables.MapToRadixEnvVarsMap(),
		TimeLimitSeconds: jobScheduleDescription.TimeLimitSeconds,
		BackoffLimit:     jobScheduleDescription.BackoffLimit,
		Image:            jobScheduleDescription.Image,
		ImageTagName:     jobScheduleDescription.ImageTagName,
		FailurePolicy:    jobScheduleDescription.FailurePolicy.MapToRadixFailurePolicy(),
		Command:          jobScheduleDescription.Command,
		Args:             jobScheduleDescription.Args,
	}
	return &batchJob, nil
}

func (h *Handler) getRadixBatchStatuses(ctx context.Context, radixBatchType kube.RadixBatchType) ([]modelsv1.BatchStatus, error) {
	logger := log.Ctx(ctx)
	radixBatches, err := internal.GetRadixBatches(ctx, h.kubeUtil.RadixClient(), h.env.RadixDeploymentNamespace, radixLabels.ForComponentName(h.radixDeployJobComponent.GetName()), radixLabels.ForBatchType(radixBatchType))
	if err != nil {
		return nil, err
	}
	logger.Debug().Msgf("Found %v batches", len(radixBatches))
	return batch.GetRadixBatchStatuses(radixBatches, h.radixDeployJobComponent), nil
}

func applyDefaultJobDescriptionProperties(jobScheduleDescription *common.JobScheduleDescription, defaultRadixJobComponentConfig *common.RadixJobComponentConfig) error {
	if jobScheduleDescription == nil || defaultRadixJobComponentConfig == nil {
		return nil
	}
	jobComponentConfig := common.RadixJobComponentConfig{}
	if err := mergo.Merge(&jobComponentConfig, defaultRadixJobComponentConfig, mergo.WithTransformers(jobDescriptionTransformer), mergo.WithOverride, mergo.WithOverrideEmptySlice); err != nil {
		return fmt.Errorf("failed to merge default job description properties: %w", err)
	}
	if err := mergo.Merge(&jobComponentConfig, jobScheduleDescription.RadixJobComponentConfig, mergo.WithTransformers(jobDescriptionTransformer), mergo.WithOverride, mergo.WithOverrideEmptySlice); err != nil {
		return fmt.Errorf("failed to merge job description properties: %w", err)
	}
	jobScheduleDescription.RadixJobComponentConfig = jobComponentConfig
	return nil
}

func (h *Handler) GetPodsForLabelSelector(ctx context.Context, labelSelector string) ([]corev1.Pod, error) {
	podList, err := h.kubeUtil.KubeClient().
		CoreV1().
		Pods(h.env.RadixDeploymentNamespace).
		List(
			ctx,
			metav1.ListOptions{LabelSelector: labelSelector},
		)

	if err != nil {
		return nil, err
	}

	return podList.Items, nil
}

const (
	// k8sJobNameLabel A label that k8s automatically adds to a Pod created by a Job
	k8sJobNameLabel = "job-name"
)

// GetLastEventMessageForPods returns the last event message for pods
func (h *Handler) GetLastEventMessageForPods(ctx context.Context, pods []corev1.Pod) (map[string]string, error) {
	podNamesMap := slice.Reduce(pods, make(map[string]struct{}), func(acc map[string]struct{}, pod corev1.Pod) map[string]struct{} {
		acc[pod.Name] = struct{}{}
		return acc
	})
	eventMap := make(map[string]string)
	eventsList, err := h.kubeUtil.KubeClient().CoreV1().Events(h.env.RadixDeploymentNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return eventMap, err
	}
	events := sortEventsAsc(eventsList.Items)
	for _, event := range events {
		if _, ok := podNamesMap[event.InvolvedObject.Name]; !ok || event.InvolvedObject.Kind != "Pod" {
			continue
		}
		if strings.Contains(event.Message, "container init was OOM-killed (memory limit too low?)") {
			eventMap[event.InvolvedObject.Name] = fmt.Sprintf("Memory limit is probably too low. Error: %s", event.Message)
			continue
		}
		eventMap[event.InvolvedObject.Name] = event.Message
	}
	return eventMap, nil
}

func sortEventsAsc(events []corev1.Event) []corev1.Event {
	sort.Slice(events, func(i, j int) bool {
		if events[i].CreationTimestamp.IsZero() || events[j].CreationTimestamp.IsZero() {
			return false
		}
		return events[i].CreationTimestamp.Before(&events[j].CreationTimestamp)
	})
	return events
}

// GetRadixBatchJobMessagesAndPodMaps returns the event messages for the batch job statuses
func (h *Handler) GetRadixBatchJobMessagesAndPodMaps(ctx context.Context, selectorForRadixBatchPods string) (map[string]string, map[string]corev1.Pod, error) {
	radixBatchesPods, err := h.GetPodsForLabelSelector(ctx, selectorForRadixBatchPods)
	if err != nil {
		return nil, nil, err
	}
	eventMessageForPods, err := h.GetLastEventMessageForPods(ctx, radixBatchesPods)
	if err != nil {
		return nil, nil, err
	}
	batchJobPodsMap := slice.Reduce(radixBatchesPods, make(map[string]corev1.Pod), func(acc map[string]corev1.Pod, pod corev1.Pod) map[string]corev1.Pod {
		if batchJobName, ok := pod.GetLabels()[k8sJobNameLabel]; ok {
			acc[batchJobName] = pod
		}
		return acc
	})
	return eventMessageForPods, batchJobPodsMap, nil
}

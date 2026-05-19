package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	jobModels "github.com/equinor/radix-operator/api-server/api/jobs/models"
	"github.com/equinor/radix-operator/api-server/api/kubequery"
	"github.com/equinor/radix-operator/api-server/api/middleware/auth"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	k8sObjectUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	jobConditionsNotValidForJobStop = []radixv1.RadixJobCondition{radixv1.JobFailed, radixv1.JobStopped, radixv1.JobStoppedNoChanges}
	jobConditionsValidForJobRerun   = []radixv1.RadixJobCondition{radixv1.JobFailed, radixv1.JobStopped}
)

// StopJob Stops an application job
func (jh JobHandler) StopJob(ctx context.Context, appName, jobName string) error {
	log.Ctx(ctx).Info().Msgf("Stopping the job: %s, %s", jobName, appName)
	radixJob, err := jh.getPipelineJobByName(ctx, appName, jobName)
	if err != nil {
		return err
	}
	if radixJob.Spec.Stop {
		return jobModels.JobAlreadyRequestedToStopError(appName, jobName)
	}
	if slice.Any(jobConditionsNotValidForJobStop, func(condition radixv1.RadixJobCondition) bool { return condition == radixJob.Status.Condition }) {
		return jobModels.JobHasInvalidConditionToStopError(appName, jobName, radixJob.Status.Condition)
	}

	radixJob.Spec.Stop = true

	_, err = jh.userAccount.RadixClient.RadixV1().RadixJobs(radixJob.GetNamespace()).Update(ctx, radixJob, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch job object: %v", err)
	}
	return nil
}

// RerunJob Reruns the pipeline job as a copy
func (jh JobHandler) RerunJob(ctx context.Context, appName, jobName string) error {
	log.Ctx(ctx).Info().Msgf("Rerunning the job %s in the application %s", jobName, appName)
	radixJob, err := jh.getPipelineJobByName(ctx, appName, jobName)
	if err != nil {
		return err
	}
	if !slice.Any(jobConditionsValidForJobRerun, func(condition radixv1.RadixJobCondition) bool { return condition == radixJob.Status.Condition }) {
		return jobModels.JobHasInvalidConditionToRerunError(appName, jobName, radixJob.Status.Condition)
	}

	copiedRadixJob := jh.buildPipelineJobToRerunFrom(ctx, radixJob)
	_, err = jh.createPipelineJob(ctx, appName, copiedRadixJob)
	if err != nil {
		return fmt.Errorf("failed to create a job %s to rerun: %v", radixJob.GetName(), err)
	}

	return nil
}
func (jh JobHandler) createPipelineJob(ctx context.Context, appName string, job *radixv1.RadixJob) (*jobModels.JobSummary, error) {
	log.Ctx(ctx).Info().Msgf("Starting job: %s, %s", job.GetName(), WorkerImage)
	appNamespace := k8sObjectUtils.GetAppNamespace(appName)
	job, err := jh.userAccount.RadixClient.RadixV1().RadixJobs(appNamespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	log.Ctx(ctx).Info().Msgf("Started job: %s, %s", job.GetName(), WorkerImage)
	return jobModels.GetSummaryFromRadixJob(job), nil
}

func (jh JobHandler) buildPipelineJobToRerunFrom(ctx context.Context, radixJob *radixv1.RadixJob) *radixv1.RadixJob {
	rerunJobName, imageTag := GetUniqueJobName()
	rerunRadixJob := radixv1.RadixJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:        rerunJobName,
			Labels:      radixJob.Labels,
			Annotations: radixJob.Annotations,
		},
		Spec: radixJob.Spec,
	}
	if rerunRadixJob.Annotations == nil {
		rerunRadixJob.Annotations = make(map[string]string)
	}
	rerunRadixJob.Annotations[jobModels.RadixPipelineJobRerunAnnotation] = radixJob.GetName()
	if len(rerunRadixJob.Spec.Build.ImageTag) > 0 {
		rerunRadixJob.Spec.Build.ImageTag = imageTag
	}
	rerunRadixJob.Spec.Stop = false
	rerunRadixJob.Spec.TriggeredBy = auth.GetOriginator(ctx)
	return &rerunRadixJob
}

func (jh JobHandler) getPipelineJobByName(ctx context.Context, appName string, jobName string) (*radixv1.RadixJob, error) {
	radixJob, err := kubequery.GetRadixJob(ctx, jh.userAccount.RadixClient, appName, jobName)
	if err != nil {
		if errors.IsNotFound(err) {
			err = jobModels.PipelineNotFoundError(appName, jobName)
		}
		return nil, err
	}
	return radixJob, nil
}

func GetUniqueJobName() (string, string) {
	var jobName []string
	randomStr := strings.ToLower(radixutils.RandString(5))
	timestamp := time.Now().Format("20060102150405")
	jobName = append(jobName, WorkerImage, "-", timestamp, "-", randomStr)

	return strings.Join(jobName, ""), randomStr
}

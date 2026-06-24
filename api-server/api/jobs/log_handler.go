package jobs

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/equinor/radix-operator/api-server/api/jobs/internal"

	jobModels "github.com/equinor/radix-operator/api-server/api/jobs/models"
	"github.com/equinor/radix-operator/api-server/api/pods"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog/log"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetTektonPipelineRunTaskStepLogs Get logs of a pipeline run task for a pipeline job
func (jh JobHandler) GetTektonPipelineRunTaskStepLogs(ctx context.Context, appName, jobName, pipelineRunName, taskName, stepName string, sinceTime *time.Time, logLines *int64, follow bool) (io.ReadCloser, error) {
	taskRunsMap, err := internal.GetTektonPipelineTaskRuns(ctx, jh.userAccount.TektonClient, appName, jobName, pipelineRunName)
	if err != nil {
		return nil, err
	}
	podName, containerName, err := jh.getTaskPodAndContainerName(taskRunsMap, taskName, stepName)
	if err != nil {
		return nil, err
	}
	podHandler := pods.Init(jh.userAccount.Client)
	return podHandler.HandleGetAppPodLog(ctx, appName, podName, containerName, sinceTime, logLines, follow)
}

func (jh JobHandler) getTaskPodAndContainerName(taskRunsMap map[string]*pipelinev1.TaskRun, taskRealName, stepName string) (string, string, error) {
	var podName, containerName string
	if taskRun, ok := taskRunsMap[taskRealName]; ok {
		podName = taskRun.Status.PodName
		for _, step := range taskRun.Status.Steps {
			if !strings.EqualFold(step.Name, stepName) {
				continue
			}
			return podName, step.Container, nil
		}
		return "", "", fmt.Errorf("missing step %s in the task %s", stepName, taskRealName)
	}
	if len(podName) == 0 || len(containerName) == 0 {
		return "", "", fmt.Errorf("missing task %s or step %s", taskRealName, stepName)
	}
	return podName, containerName, nil
}

// GetPipelineJobStepLogs Get logs of a pipeline job step
func (jh JobHandler) GetPipelineJobStepLogs(ctx context.Context, appName, jobName, stepName string, sinceTime *time.Time, logLines *int64, follow bool) (io.ReadCloser, error) {
	job, err := jh.userAccount.RadixClient.RadixV1().RadixJobs(crdUtils.GetAppNamespace(appName)).Get(ctx, jobName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, jobModels.PipelineNotFoundError(appName, jobName)
		}
		return nil, err
	}
	stepPodName := getPodNameForStep(job, stepName)
	if len(stepPodName) == 0 {
		return nil, jobModels.PipelineStepNotFoundError(appName, jobName, stepName)
	}

	podHandler := pods.Init(jh.userAccount.Client)
	logReader, err := podHandler.HandleGetAppPodLog(ctx, appName, stepPodName, stepName, sinceTime, logLines, follow)
	if err != nil {
		log.Ctx(ctx).Warn().Msgf("Failed to get build logs. %v", err)
		return nil, err
	}
	return logReader, nil
}

func getPodNameForStep(job *radixv1.RadixJob, stepName string) string {
	for _, jobStep := range job.Status.Steps {
		if strings.EqualFold(jobStep.Name, stepName) {
			return jobStep.PodName
		}
	}
	return ""
}

package internal

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/api-server/api/jobs/defaults"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeLabels "k8s.io/apimachinery/pkg/labels"
)

// GetTektonPipelineTaskRuns Get Tekton TaskRuns for the Radix pipeline job
func GetTektonPipelineTaskRuns(ctx context.Context, tektonClient tektonclient.Interface, appName, jobName string, pipelineRunName string) (map[string]*pipelinev1.TaskRun, error) {
	taskRunList, err := getTaskRuns(ctx, tektonClient, appName, jobName, pipelineRunName)
	if err != nil {
		return nil, err
	}
	return slice.Reduce(taskRunList.Items, make(map[string]*pipelinev1.TaskRun), func(acc map[string]*pipelinev1.TaskRun, taskRun pipelinev1.TaskRun) map[string]*pipelinev1.TaskRun {
		if taskRun.Spec.TaskRef != nil {
			acc[taskRun.Spec.TaskRef.Name] = &taskRun
		}
		return acc
	}), nil
}

func getTaskRuns(ctx context.Context, tektonClient tektonclient.Interface, appName string, jobName string, pipelineRunName string) (*pipelinev1.TaskRunList, error) {
	namespace := crdUtils.GetAppNamespace(appName)
	return tektonClient.TektonV1().TaskRuns(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: kubeLabels.Set{
			kube.RadixJobNameLabel:         jobName,
			defaults.TektonPipelineRunName: pipelineRunName,
		}.String(),
	})
}

// GetTektonPipelineTaskRunByTaskName Get Tekton TaskRun for the Radix pipeline job Tekton Pipeline by a task name
func GetTektonPipelineTaskRunByTaskName(ctx context.Context, tektonClient tektonclient.Interface, appName, jobName, pipelineRunName, taskName string) (*pipelinev1.TaskRun, error) {
	taskRunList, err := getTaskRuns(ctx, tektonClient, appName, jobName, pipelineRunName)
	if err != nil {
		return nil, err
	}
	if taskRun, ok := slice.FindFirst(taskRunList.Items, func(taskRun pipelinev1.TaskRun) bool {
		return taskRun.GetLabels()[kube.RadixJobNameLabel] == jobName &&
			taskRun.GetLabels()[defaults.TektonPipelineRunName] == pipelineRunName &&
			taskRun.GetLabels()[defaults.TektonTaskName] == taskName
	}); ok {
		return &taskRun, nil
	}
	return nil, fmt.Errorf("task run %s not found", taskName)
}

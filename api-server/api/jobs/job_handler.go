package jobs

import (
	"context"
	"fmt"
	"sort"
	"strings"

	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/api-server/api/deployments"
	deploymentModels "github.com/equinor/radix-operator/api-server/api/deployments/models"
	"github.com/equinor/radix-operator/api-server/api/jobs/defaults"
	"github.com/equinor/radix-operator/api-server/api/jobs/internal"
	jobModels "github.com/equinor/radix-operator/api-server/api/jobs/models"
	"github.com/equinor/radix-operator/api-server/api/kubequery"
	"github.com/equinor/radix-operator/api-server/api/utils"
	"github.com/equinor/radix-operator/api-server/models"
	operatorDefaults "github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

const (
	WorkerImage = "radix-pipeline"
)

// JobHandler Instance variables
type JobHandler struct {
	accounts       models.Accounts
	userAccount    models.Account
	serviceAccount models.Account
	deploy         deployments.DeployHandler
}

// Init Constructor
func Init(accounts models.Accounts, deployHandler deployments.DeployHandler) JobHandler {
	return JobHandler{
		accounts:       accounts,
		userAccount:    accounts.UserAccount,
		serviceAccount: accounts.ServiceAccount,
		deploy:         deployHandler,
	}
}

// GetApplicationJobs Handler for GetApplicationJobs
func (jh JobHandler) GetApplicationJobs(ctx context.Context, appName string) ([]*jobModels.JobSummary, error) {
	jobs, err := jh.getJobs(ctx, appName)
	if err != nil {
		return nil, err
	}

	// Sort jobs descending
	sort.Slice(jobs, func(i, j int) bool {
		return utils.IsBefore(jobs[j], jobs[i])
	})

	return jobs, nil
}

// GetApplicationJob Handler for GetApplicationJob
func (jh JobHandler) GetApplicationJob(ctx context.Context, appName, jobName string) (*jobModels.Job, error) {
	job, err := jh.userAccount.RadixClient.RadixV1().RadixJobs(crdUtils.GetAppNamespace(appName)).Get(ctx, jobName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	jobDeployments, err := jh.deploy.GetDeploymentsForPipelineJob(ctx, appName, jobName)
	if err != nil {
		return nil, err
	}

	return jh.getJobFromRadixJob(ctx, job, jobDeployments, appName, jobName)
}

func (jh JobHandler) getSubPipelinesInfo(ctx context.Context, appName string, jobName string) ([]pipelinev1.TaskRun, error) {
	labelSelectorByJobName := labels.Set{kube.RadixJobNameLabel: jobName}.String()
	subPipelineTaskRuns, err := jh.userAccount.TektonClient.TektonV1().TaskRuns(crdUtils.GetAppNamespace(appName)).List(ctx, metav1.ListOptions{LabelSelector: labelSelectorByJobName})
	if err != nil {
		return nil, err
	}
	return subPipelineTaskRuns.Items, nil
}

// GetTektonPipelineRuns Get the Tekton pipeline runs
func (jh JobHandler) GetTektonPipelineRuns(ctx context.Context, appName, jobName string) ([]jobModels.PipelineRun, error) {
	pipelineRuns, err := internal.GetTektonPipelineRuns(ctx, jh.userAccount.TektonClient, appName, jobName)
	if err != nil {
		return nil, err
	}
	var pipelineRunModels []jobModels.PipelineRun
	for _, pipelineRun := range pipelineRuns {
		pipelineRunModel := getPipelineRunModel(&pipelineRun)
		pipelineRunModels = append(pipelineRunModels, *pipelineRunModel)
	}
	return pipelineRunModels, nil
}

// GetTektonPipelineRun Get the Tekton pipeline run
func (jh JobHandler) GetTektonPipelineRun(ctx context.Context, appName, jobName, pipelineRunName string) (*jobModels.PipelineRun, error) {
	pipelineRun, err := internal.GetPipelineRun(ctx, jh.userAccount.TektonClient, appName, jobName, pipelineRunName)
	if err != nil {
		return nil, err
	}
	return getPipelineRunModel(pipelineRun), nil
}

// GetTektonPipelineRunTasks Get the Tekton pipeline run tasks
func (jh JobHandler) GetTektonPipelineRunTasks(ctx context.Context, appName, jobName, pipelineRunName string) ([]jobModels.PipelineRunTask, error) {
	pipelineRun, taskNameToTaskRunMap, err := jh.getPipelineRunWithTasks(ctx, appName, jobName, pipelineRunName)
	if err != nil {
		return nil, err
	}
	taskModels := getPipelineRunTaskModels(pipelineRun, taskNameToTaskRunMap)
	return sortPipelineTasks(taskModels), nil
}

func (jh JobHandler) getPipelineRunWithTasks(ctx context.Context, appName string, jobName string, pipelineRunName string) (*pipelinev1.PipelineRun, map[string]*pipelinev1.TaskRun, error) {
	pipelineRun, err := jh.getPipelineRunWithRef(ctx, appName, jobName, pipelineRunName)
	if err != nil {
		return nil, nil, err
	}
	taskRunsMap, err := internal.GetTektonPipelineTaskRuns(ctx, jh.userAccount.TektonClient, appName, jobName, pipelineRunName)
	if err != nil {
		return nil, nil, err
	}
	taskNameToTaskRunMap := getPipelineTaskNameToTaskRunMap(pipelineRun.Status.PipelineSpec.Tasks, taskRunsMap)
	return pipelineRun, taskNameToTaskRunMap, nil
}

func (jh JobHandler) getPipelineRunWithRef(ctx context.Context, appName string, jobName string, pipelineRunName string) (*pipelinev1.PipelineRun, error) {
	pipelineRun, err := internal.GetPipelineRun(ctx, jh.userAccount.TektonClient, appName, jobName, pipelineRunName)
	if err != nil {
		return nil, err
	}
	if pipelineRun.Spec.PipelineRef == nil || len(pipelineRun.Spec.PipelineRef.Name) == 0 {
		return nil, fmt.Errorf("the Pipeline Run %s does not have reference to the Pipeline", pipelineRunName)
	}
	return pipelineRun, nil
}

func getPipelineTaskNameToTaskRunMap(pipelineTasks []pipelinev1.PipelineTask, taskRunsMap map[string]*pipelinev1.TaskRun) map[string]*pipelinev1.TaskRun {
	return slice.Reduce(pipelineTasks, make(map[string]*pipelinev1.TaskRun), func(acc map[string]*pipelinev1.TaskRun, task pipelinev1.PipelineTask) map[string]*pipelinev1.TaskRun {
		if task.TaskRef == nil {
			return acc
		}
		if taskRun, ok := taskRunsMap[task.TaskRef.Name]; ok {
			acc[task.Name] = taskRun
		}
		return acc
	})
}

// GetTektonPipelineRunTask Get the Tekton pipeline run task
func (jh JobHandler) GetTektonPipelineRunTask(ctx context.Context, appName, jobName, pipelineRunName, taskName string) (*jobModels.PipelineRunTask, error) {
	pipelineRun, taskRun, err := jh.getPipelineRunAndTaskRun(ctx, appName, jobName, pipelineRunName, taskName)
	if err != nil {
		return nil, err
	}
	return getPipelineRunTaskModelByTaskSpec(pipelineRun, taskRun), nil
}

// GetTektonPipelineRunTaskSteps Get the Tekton pipeline run task steps
func (jh JobHandler) GetTektonPipelineRunTaskSteps(ctx context.Context, appName, jobName, pipelineRunName, taskName string) ([]jobModels.PipelineRunTaskStep, error) {
	_, taskRun, err := jh.getPipelineRunAndTaskRun(ctx, appName, jobName, pipelineRunName, taskName)
	if err != nil {
		return nil, err
	}
	return buildPipelineRunTaskStepModels(taskRun), nil
}

// GetTektonPipelineRunTaskStep Get the Tekton pipeline run task step
func (jh JobHandler) GetTektonPipelineRunTaskStep(ctx context.Context, appName, jobName, pipelineRunName, taskName, stepName string) (*jobModels.Step, error) {
	_, taskRun, err := jh.getPipelineRunAndTaskRun(ctx, appName, jobName, pipelineRunName, taskName)
	if err != nil {
		return nil, err
	}
	taskStep, ok := slice.FindFirst(taskRun.Status.Steps, func(step pipelinev1.StepState) bool { return step.Name == stepName })
	if !ok {
		return nil, fmt.Errorf("step %s not found in the task %s", stepName, taskName)
	}
	envName := taskRun.GetLabels()[kube.RadixEnvLabel]
	taskKubeName := taskRun.GetLabels()[defaults.TektonTaskKubeName]
	pipelineName := taskRun.GetAnnotations()[operatorDefaults.PipelineNameAnnotation]
	stepModel := jh.getTaskRunStepModel(envName, pipelineName, pipelineRunName, taskName, taskKubeName, taskStep)
	return &stepModel, nil
}

func (jh JobHandler) getPipelineRunAndTaskRun(ctx context.Context, appName string, jobName string, pipelineRunName string, taskName string) (*pipelinev1.PipelineRun, *pipelinev1.TaskRun, error) {
	pipelineRun, err := jh.getPipelineRunWithRef(ctx, appName, jobName, pipelineRunName)
	if err != nil {
		return nil, nil, err
	}
	taskRun, err := internal.GetTektonPipelineTaskRunByTaskName(ctx, jh.userAccount.TektonClient, appName, jobName, pipelineRunName, taskName)
	if err != nil {
		return nil, nil, err
	}
	return pipelineRun, taskRun, nil
}

func getPipelineRunModel(pipelineRun *pipelinev1.PipelineRun) *jobModels.PipelineRun {
	pipelineRunModel := jobModels.PipelineRun{
		Name:     pipelineRun.Annotations[operatorDefaults.PipelineNameAnnotation],
		Env:      pipelineRun.Labels[kube.RadixEnvLabel],
		KubeName: pipelineRun.GetName(),
		Started:  radixutils.FormatTime(pipelineRun.Status.StartTime),
		Ended:    radixutils.FormatTime(pipelineRun.Status.CompletionTime),
	}
	runCondition := getLastReadyCondition(pipelineRun.Status.Conditions)
	if runCondition != nil {
		pipelineRunModel.Status = jobModels.TaskRunReason(runCondition.Reason)
		pipelineRunModel.StatusMessage = runCondition.Message
	}
	return &pipelineRunModel
}

func getPipelineRunTaskModels(pipelineRun *pipelinev1.PipelineRun, taskNameToTaskRunMap map[string]*pipelinev1.TaskRun) []jobModels.PipelineRunTask {
	return slice.Reduce(pipelineRun.Status.ChildReferences, make([]jobModels.PipelineRunTask, 0), func(acc []jobModels.PipelineRunTask, taskRunSpec pipelinev1.ChildStatusReference) []jobModels.PipelineRunTask {
		if taskRun, ok := taskNameToTaskRunMap[taskRunSpec.PipelineTaskName]; ok {
			pipelineTaskModel := getPipelineRunTaskModelByTaskSpec(pipelineRun, taskRun)
			acc = append(acc, *pipelineTaskModel)
		}
		return acc
	})
}

func getPipelineRunTaskModelByTaskSpec(pipelineRun *pipelinev1.PipelineRun, taskRun *pipelinev1.TaskRun) *jobModels.PipelineRunTask {
	pipelineTaskModel := jobModels.PipelineRunTask{
		Name:           taskRun.Labels[defaults.TektonTaskName],
		KubeName:       taskRun.Spec.TaskRef.Name,
		PipelineRunEnv: pipelineRun.Labels[kube.RadixEnvLabel],
		PipelineName:   pipelineRun.Annotations[operatorDefaults.PipelineNameAnnotation],
	}
	pipelineTaskModel.Started = radixutils.FormatTime(taskRun.Status.StartTime)
	pipelineTaskModel.Ended = radixutils.FormatTime(taskRun.Status.CompletionTime)
	taskCondition := getLastReadyCondition(taskRun.Status.Conditions)
	if taskCondition != nil {
		pipelineTaskModel.Status = jobModels.PipelineRunReason(taskCondition.Reason)
		pipelineTaskModel.StatusMessage = taskCondition.Message
	}
	logEmbeddedCommandIndex := strings.Index(pipelineTaskModel.StatusMessage, "for logs run")
	if logEmbeddedCommandIndex >= 0 { // Avoid to publish kubectl command, provided by Tekton component after "for logs run" prefix for failed task step
		pipelineTaskModel.StatusMessage = pipelineTaskModel.StatusMessage[0:logEmbeddedCommandIndex]
	}
	return &pipelineTaskModel
}

func buildPipelineRunTaskStepModels(taskRun *pipelinev1.TaskRun) []jobModels.PipelineRunTaskStep {
	var stepsModels []jobModels.PipelineRunTaskStep
	for _, step := range taskRun.Status.Steps {
		stepsModels = append(stepsModels, buildPipelineRunTaskStepModel(step))
	}
	return stepsModels
}

func buildPipelineRunTaskStepModel(step pipelinev1.StepState) jobModels.PipelineRunTaskStep {
	stepModel := jobModels.PipelineRunTaskStep{Name: step.Name}
	if step.Terminated != nil {
		stepModel.Started = radixutils.FormatTime(&step.Terminated.StartedAt)
		stepModel.Ended = radixutils.FormatTime(&step.Terminated.FinishedAt)
		stepModel.Status = jobModels.TaskRunReason(step.Terminated.Reason)
		stepModel.StatusMessage = step.Terminated.Message
	} else if step.Running != nil {
		stepModel.Started = radixutils.FormatTime(&step.Running.StartedAt)
		stepModel.Status = jobModels.TaskRunReasonRunning
	} else if step.Waiting != nil {
		stepModel.Status = jobModels.TaskRunReason(step.Waiting.Reason)
		stepModel.StatusMessage = step.Waiting.Message
	}
	return stepModel
}

func getLastReadyCondition(conditions []apis.Condition) *apis.Condition {
	if len(conditions) == 1 {
		return &conditions[0]
	}
	conditions = sortPipelineTaskStatusConditionsDesc(conditions)
	for _, condition := range conditions {
		if condition.Status == corev1.ConditionTrue {
			return &condition
		}
	}
	if len(conditions) > 0 {
		return &conditions[0]
	}
	return nil
}

func sortPipelineTaskStatusConditionsDesc(conditions []apis.Condition) duckv1.Conditions {
	sort.Slice(conditions, func(i, j int) bool {
		if conditions[i].LastTransitionTime.Inner.IsZero() || conditions[j].LastTransitionTime.Inner.IsZero() {
			return false
		}
		return conditions[j].LastTransitionTime.Inner.Before(&conditions[i].LastTransitionTime.Inner)
	})
	return conditions
}

func sortPipelineTasks(tasks []jobModels.PipelineRunTask) []jobModels.PipelineRunTask {
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Started == "" || tasks[j].Started == "" {
			return false
		}
		return tasks[i].Started < tasks[j].Started
	})
	return tasks
}

func (jh JobHandler) getJobs(ctx context.Context, appName string) ([]*jobModels.JobSummary, error) {
	jobs, err := kubequery.GetRadixJobs(ctx, jh.accounts.UserAccount.RadixClient, appName)
	if err != nil {
		return nil, err
	}

	return slice.Map(jobs, func(j v1.RadixJob) *jobModels.JobSummary {
		// Pass nil for RadixApplication - will fetch if needed by individual job handlers
		return jobModels.GetSummaryFromRadixJob(&j)
	}), nil
}

func (jh JobHandler) getJobFromRadixJob(ctx context.Context, job *v1.RadixJob, jobDeployments []*deploymentModels.DeploymentSummary, appName, jobName string) (*jobModels.Job, error) {
	steps, err := jh.getJobStepsFromRadixJob(ctx, job, appName, jobName)
	if err != nil {
		return nil, err
	}

	created := radixutils.FormatTime(&job.CreationTimestamp)
	if job.Status.Created != nil {
		// Use this instead, because in a migration this may be more correct
		// as migrated jobs will have the same creation timestamp in the new cluster
		created = radixutils.FormatTime(job.Status.Created)
	}

	var jobComponents []*deploymentModels.ComponentSummary
	if len(jobDeployments) > 0 {
		jobComponents = jobDeployments[0].Components
	}

	jobModel := jobModels.Job{
		Name:                 job.GetName(),
		Created:              created,
		Started:              radixutils.FormatTime(job.Status.Started),
		Ended:                radixutils.FormatTime(job.Status.Ended),
		Status:               jobModels.GetStatusFromRadixJobStatus(job.Status, job.Spec.Stop),
		Pipeline:             string(job.Spec.PipeLineType),
		Steps:                steps,
		Deployments:          jobDeployments,
		Components:           jobComponents,
		TriggeredFromWebhook: job.Spec.TriggeredFromWebhook,
		TriggeredBy:          job.Spec.TriggeredBy,
		RerunFromJob:         job.Annotations[jobModels.RadixPipelineJobRerunAnnotation],
	}
	switch job.Spec.PipeLineType {
	case v1.Build, v1.BuildDeploy:
		jobModel.Branch = job.Spec.Build.Branch //nolint:staticcheck
		jobModel.GitRef = job.Spec.Build.GitRef
		jobModel.GitRefType = string(job.Spec.Build.GitRefType)
		jobModel.DeployedToEnvironment = job.Spec.Build.ToEnvironment
		jobModel.CommitID = job.Spec.Build.CommitID
		jobModel.UseBuildKit = jobModels.IsUsingBuildKit(job)
		jobModel.UseBuildCache = jobModels.IsUsingBuildCache(job)
		jobModel.OverrideUseBuildCache = job.Spec.Build.OverrideUseBuildCache
		jobModel.RefreshBuildCache = job.Spec.Build.RefreshBuildCache
	case v1.Deploy:
		jobModel.ImageTagNames = job.Spec.Deploy.ImageTagNames
		jobModel.DeployedToEnvironment = job.Spec.Deploy.ToEnvironment
		jobModel.CommitID = job.Spec.Deploy.CommitID
	case v1.Promote:
		jobModel.PromotedFromDeployment = job.Spec.Promote.DeploymentName
		jobModel.PromotedFromEnvironment = job.Spec.Promote.FromEnvironment
		jobModel.PromotedToEnvironment = job.Spec.Promote.ToEnvironment
		jobModel.CommitID = job.Spec.Promote.CommitID
	case v1.ApplyConfig:
		jobModel.DeployExternalDNS = pointers.Ptr(job.Spec.ApplyConfig.DeployExternalDNS)
	}
	return &jobModel, nil
}

func (jh JobHandler) getJobStepsFromRadixJob(ctx context.Context, job *v1.RadixJob, appName, jobName string) ([]jobModels.Step, error) {
	var steps []jobModels.Step
	var buildSteps []jobModels.Step

	for _, jobStep := range job.Status.Steps {
		step := jobModels.Step{
			Name:       jobStep.Name,
			Status:     string(jobStep.Condition),
			PodName:    jobStep.PodName,
			Components: jobStep.Components,
		}
		if jobStep.Started != nil {
			step.Started = &jobStep.Started.Time
		}
		if jobStep.Ended != nil {
			step.Ended = &jobStep.Ended.Time
		}
		if strings.HasPrefix(step.Name, "build-") {
			buildSteps = append(buildSteps, step)
		} else {
			steps = append(steps, step)
		}
	}

	tasksSteps, err := jh.getSubPipelineTasksSteps(ctx, appName, jobName)
	if err != nil {
		return nil, err
	}
	steps = append(steps, tasksSteps...)

	sort.Slice(buildSteps, func(i, j int) bool { return buildSteps[i].Name < buildSteps[j].Name })
	return append(steps, buildSteps...), nil
}

func (jh JobHandler) getSubPipelineTasksSteps(ctx context.Context, appName, jobName string) ([]jobModels.Step, error) {
	subPipelineTaskRuns, err := jh.getSubPipelinesInfo(ctx, appName, jobName)
	if err != nil {
		return nil, err
	}
	var steps []jobModels.Step
	for _, taskRun := range subPipelineTaskRuns {
		envName := taskRun.GetLabels()[kube.RadixEnvLabel]
		pipelineRunName := taskRun.GetLabels()[defaults.TektonPipelineRunName]
		taskName := taskRun.GetLabels()[defaults.TektonTaskName]
		taskKubeName := taskRun.GetLabels()[defaults.TektonTaskKubeName]
		pipelineName := taskRun.GetAnnotations()[operatorDefaults.PipelineNameAnnotation]
		for _, taskStep := range taskRun.Status.Steps {
			stepModel := jh.getTaskRunStepModel(envName, pipelineName, pipelineRunName, taskName, taskKubeName, taskStep)
			steps = append(steps, stepModel)
		}
	}
	return steps, nil
}

func (jh JobHandler) getTaskRunStepModel(envName, pipelineName, pipelineRunName, taskName, taskKubeName string, taskStep pipelinev1.StepState) jobModels.Step {
	stepModel := jobModels.Step{
		Name: "sub-pipeline-step",
		SubPipelineTaskStep: &jobModels.SubPipelineTaskStep{
			Name:            taskStep.Name,
			PipelineName:    pipelineName,
			Environment:     envName,
			PipelineRunName: pipelineRunName,
			TaskName:        taskName,
			KubeName:        taskKubeName,
		},
	}
	if taskStep.Terminated != nil {
		stepModel.Started = &taskStep.Terminated.StartedAt.Time
		stepModel.Ended = &taskStep.Terminated.FinishedAt.Time
		stepModel.Status = getStepStatusBySubPipelineTaskStepTerminationStatus(taskStep.Terminated.Reason)
		stepModel.SubPipelineTaskStep.Status = taskStep.Terminated.Reason
	} else if taskStep.Running != nil {
		stepModel.Started = &taskStep.Running.StartedAt.Time
		stepModel.Status = "Running"
		stepModel.SubPipelineTaskStep.Status = string(pipelinev1.TaskRunReasonRunning)
	} else if taskStep.Waiting != nil {
		stepModel.Status = "Waiting"
		stepModel.SubPipelineTaskStep.Status = getStepStatusBySubPipelineTaskStepWaitingStatus(taskStep.Waiting.Reason)
	}
	return stepModel
}

func getStepStatusBySubPipelineTaskStepWaitingStatus(reason string) string {
	switch reason {
	case "PodInitializing":
		return "Starting"
	default:
		return reason
	}
}

func getStepStatusBySubPipelineTaskStepTerminationStatus(reason string) string {
	switch pipelinev1.TaskRunReason(reason) {
	case pipelinev1.TaskRunReasonStarted, pipelinev1.TaskRunReasonRunning:
		return ""
	case pipelinev1.TaskRunReasonSuccessful:
		return "Succeeded"
	default:
		return reason
	}
}

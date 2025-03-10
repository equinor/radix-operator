package preparepipeline

import (
	"context"
	"errors"
	"fmt"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	pipelineDefaults "github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/validation"
	"github.com/equinor/radix-operator/pipeline-runner/utils/annotations"
	"github.com/equinor/radix-operator/pipeline-runner/utils/git"
	"github.com/equinor/radix-operator/pipeline-runner/utils/labels"
	"github.com/equinor/radix-operator/pipeline-runner/utils/radix/deployment/commithash"
	operatorDefaults "github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	validate "github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog/log"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

var privateSshFolderMode int32 = 0444

// GetBuildContext Prepare build context
func (ctx *pipelineContext) GetBuildContext() (*model.PrepareBuildContext, error) {
	gitHash, err := ctx.getGitHash()
	if err != nil {
		return nil, err
	}

	pipelineType := ctx.GetPipelineInfo().GetRadixPipelineType()

	if gitHash == "" && ctx.GetPipelineInfo().GetRadixPipelineType() != v1.BuildDeploy {
		// if no git hash, don't run sub-pipelines
		return &buildContext, nil
	}

	if err = git.ResetGitHead(ctx.GetPipelineInfo().GetGitWorkspace(), gitHash); err != nil && pipelineType == radixv1.Promote {
		log.Error().Msgf("Failed to find Git CommitID %s. Error: %v. Ignore the error for the Promote pipeline. If Sub-pipeline exists it is skipped.", gitHash, err)
		return nil, nil
	}

	buildContext := model.PrepareBuildContext{}

	if pipelineType == radixv1.BuildDeploy {
		buildContext.EnvironmentsToBuild, buildContext.ChangedRadixConfig, err = ctx.prepareBuildDeployPipeline()
		if err != nil {
			return nil, err
		}
	}
	return &buildContext, nil
}

// GetEnvironmentSubPipelinesToRun Prepare sub-pipelines for the target environments
func (ctx *pipelineContext) GetEnvironmentSubPipelinesToRun() ([]model.EnvironmentSubPipelineToRun, error) {
	var environmentSubPipelinesToRun []model.EnvironmentSubPipelineToRun
	var errs []error
	timestamp := time.Now().Format("20060102150405")
	for _, targetEnv := range ctx.targetEnvironments {
		log.Debug().Msgf("create a sub-pipeline for the environment %s", targetEnv)
		runSubPipeline, pipelineFilePath, err := ctx.preparePipelinesJobForTargetEnv(targetEnv, timestamp)
		if err != nil {
			errs = append(errs, err)
		}
		if runSubPipeline {
			environmentSubPipelinesToRun = append(environmentSubPipelinesToRun, model.EnvironmentSubPipelineToRun{
				Environment:  targetEnv,
				PipelineFile: pipelineFilePath,
			})
		}
	}
	err := errors.Join(errs...)
	if err != nil {
		return nil, err
	}
	if len(environmentSubPipelinesToRun) > 0 {
		log.Info().Msg("Run sub-pipelines:")
		for _, subPipelineToRun := range environmentSubPipelinesToRun {
			log.Info().Msgf("- environment %s, pipeline file %s", subPipelineToRun.Environment, subPipelineToRun.PipelineFile)
		}
		return environmentSubPipelinesToRun, nil
	}
	log.Info().Msg("No sub-pipelines to run")
	return nil, nil
}

func (ctx *pipelineContext) prepareBuildDeployPipeline() ([]model.EnvironmentToBuild, bool, error) {
	pipelineArgs := ctx.GetPipelineInfo().PipelineArguments
	pipelineTargetCommitHash, commitTags, err := git.GetCommitHashAndTags(pipelineArgs.GitWorkspace, pipelineArgs.CommitID, pipelineArgs.Branch)
	if err != nil {
		return nil, false, err
	}
	if err = validate.GitTagsContainIllegalChars(commitTags); err != nil {
		return nil, false, err
	}
	ctx.GetPipelineInfo().SetGitAttributes(pipelineTargetCommitHash, commitTags)

	if len(pipelineArgs.CommitID) > 0 {
		return nil, false, nil
	}

	radixConfigWasChanged, environmentsToBuild, err := ctx.analyseSourceRepositoryChanges(pipelineTargetCommitHash)
	if err != nil {
		return nil, false, err
	}
	return environmentsToBuild, radixConfigWasChanged, nil
}

func (ctx *pipelineContext) analyseSourceRepositoryChanges(pipelineTargetCommitHash string) (bool, []model.EnvironmentToBuild, error) {
	radixDeploymentCommitHashProvider := commithash.NewProvider(ctx.kubeClient, ctx.radixClient, ctx.pipelineInfo.GetAppName(), ctx.targetEnvironments)
	lastCommitHashesForEnvs, err := radixDeploymentCommitHashProvider.GetLastCommitHashesForEnvironments()
	if err != nil {
		return false, nil, err
	}

	changesFromGitRepository, radixConfigWasChanged, err := git.GetChangesFromGitRepository(ctx.pipelineInfo.GetGitWorkspace(),
		ctx.pipelineInfo.GetRadixConfigBranch(),
		ctx.pipelineInfo.GetRadixConfigFile(),
		pipelineTargetCommitHash,
		lastCommitHashesForEnvs)
	if err != nil {
		return false, nil, err
	}

	environmentsToBuild := ctx.getEnvironmentsToBuild(changesFromGitRepository)
	return radixConfigWasChanged, environmentsToBuild, nil
}

func (ctx *pipelineContext) getEnvironmentsToBuild(changesFromGitRepository map[string][]string) []model.EnvironmentToBuild {
	var environmentsToBuild []model.EnvironmentToBuild
	for envName, changedFolders := range changesFromGitRepository {
		var componentsWithChangedSource []string
		for _, radixComponent := range ctx.GetRadixApplication().Spec.Components {
			if componentHasChangedSource(envName, &radixComponent, changedFolders) {
				componentsWithChangedSource = append(componentsWithChangedSource, radixComponent.GetName())
			}
		}
		for _, radixJobComponent := range ctx.GetRadixApplication().Spec.Jobs {
			if componentHasChangedSource(envName, &radixJobComponent, changedFolders) {
				componentsWithChangedSource = append(componentsWithChangedSource, radixJobComponent.GetName())
			}
		}
		environmentsToBuild = append(environmentsToBuild, model.EnvironmentToBuild{
			Environment: envName,
			Components:  componentsWithChangedSource,
		})
	}
	return environmentsToBuild
}

func componentHasChangedSource(envName string, component radixv1.RadixCommonComponent, changedFolders []string) bool {
	image := component.GetImageForEnvironment(envName)
	if len(image) > 0 {
		return false
	}
	environmentConfig := component.GetEnvironmentConfigByName(envName)
	if !component.GetEnabledForEnvironmentConfig(environmentConfig) {
		return false
	}

	componentSource := component.GetSourceForEnvironment(envName)
	sourceFolder := cleanPathAndSurroundBySlashes(componentSource.Folder)
	if path.Dir(sourceFolder) == path.Dir("/") && len(changedFolders) > 0 {
		return true // for components with the repository root as a 'src' - changes in any repository sub-folders are considered also as the component changes
	}

	for _, folder := range changedFolders {
		if strings.HasPrefix(cleanPathAndSurroundBySlashes(folder), sourceFolder) {
			return true
		}
	}
	return false
}

func cleanPathAndSurroundBySlashes(dir string) string {
	if !strings.HasSuffix(dir, "/") {
		dir = fmt.Sprintf("%s/", dir)
	}
	dir = fmt.Sprintf("%s/", path.Dir(dir))
	if !strings.HasPrefix(dir, "/") {
		return fmt.Sprintf("/%s", dir)
	}
	return dir
}

func (ctx *pipelineContext) preparePipelinesJobForTargetEnv(envName, timestamp string) (bool, string, error) {
	pipelineFilePath, err := ctx.getPipelineFilePath("") // TODO - get pipeline for the envName
	if err != nil {
		return false, "", err
	}

	err = ctx.pipelineFileExists(pipelineFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Info().Msgf("There is no Tekton pipeline file: %s. Skip Tekton pipeline", pipelineFilePath)
			return false, "", nil
		}
		return false, "", err
	}

	pipeline, err := getPipeline(pipelineFilePath)
	if err != nil {
		return false, "", err
	}
	log.Debug().Msgf("loaded a pipeline with %d tasks", len(pipeline.Spec.Tasks))

	tasks, err := ctx.getPipelineTasks(pipelineFilePath, pipeline)
	if err != nil {
		return false, "", err
	}
	log.Debug().Msg("all pipeline tasks found")
	err = ctx.createPipeline(envName, pipeline, tasks, timestamp)
	if err != nil {
		return false, "", err
	}
	return true, pipelineFilePath, nil
}

func (ctx *pipelineContext) pipelineFileExists(pipelineFilePath string) error {
	_, err := os.Stat(pipelineFilePath)
	return err
}

func (ctx *pipelineContext) buildTasks(envName string, tasks []pipelinev1.Task, timestamp string) (map[string]pipelinev1.Task, error) {
	var errs []error
	taskMap := make(map[string]pipelinev1.Task)
	for _, task := range tasks {
		originalTaskName := task.Name
		task.ObjectMeta.Name = fmt.Sprintf("radix-task-%s-%s-%s-%s", internal.GetShortName(envName), internal.GetShortName(originalTaskName), timestamp, ctx.hash)
		if task.ObjectMeta.Labels == nil {
			task.ObjectMeta.Labels = map[string]string{}
		}
		if task.ObjectMeta.Annotations == nil {
			task.ObjectMeta.Annotations = map[string]string{}
		}

		for k, v := range labels.GetLabelsForEnvironment(ctx.GetPipelineInfo(), envName) {
			task.ObjectMeta.Labels[k] = v
		}

		if val, ok := task.ObjectMeta.Labels[labels.AzureWorkloadIdentityUse]; ok {
			if val != "true" {
				errs = append(errs, fmt.Errorf("label %s is invalid, %s must be lowercase true in task %s: %w", labels.AzureWorkloadIdentityUse, val, originalTaskName, validation.ErrInvalidTaskLabelValue))
			}

			err := sanitizeAzureSkipContainersAnnotation(&task)
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to sanitize task %s: %w", originalTaskName, err))
			}
		}

		task.ObjectMeta.Annotations[pipelineDefaults.PipelineTaskNameAnnotation] = originalTaskName
		if ctx.ownerReference != nil {
			task.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*ctx.ownerReference}
		}
		ensureCorrectSecureContext(&task)
		taskMap[originalTaskName] = task
		log.Debug().Msgf("created the task %s", task.Name)
	}
	return taskMap, errors.Join(errs...)
}

func sanitizeAzureSkipContainersAnnotation(task *pipelinev1.Task) error {
	skipSteps := strings.Split(task.ObjectMeta.Annotations[annotations.AzureWorkloadIdentitySkipContainers], ";")

	var errs []error
	for _, step := range skipSteps {
		sanitizedSkipStepName := strings.ToLower(strings.TrimSpace(step))
		if sanitizedSkipStepName == "" {
			continue
		}

		containsStep := slices.ContainsFunc(task.Spec.Steps, func(s pipelinev1.Step) bool {
			return strings.ToLower(strings.TrimSpace(s.Name)) == sanitizedSkipStepName
		})
		if !containsStep {
			errs = append(errs, fmt.Errorf("step %s is not defined: %w", sanitizedSkipStepName, validation.ErrSkipStepNotFound))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	skipContainers := []string{"place-scripts", "prepare"}
	for _, stepName := range skipSteps {
		skipContainers = append(skipContainers, "step-"+stepName)
	}

	task.ObjectMeta.Annotations[annotations.AzureWorkloadIdentitySkipContainers] = strings.Join(skipContainers, ";")
	return nil
}

func ensureCorrectSecureContext(task *pipelinev1.Task) {
	for i := 0; i < len(task.Spec.Steps); i++ {
		if task.Spec.Steps[i].SecurityContext == nil {
			task.Spec.Steps[i].SecurityContext = &corev1.SecurityContext{}
		}
		setNotElevatedPrivileges(task.Spec.Steps[i].SecurityContext)
	}
	for i := 0; i < len(task.Spec.Sidecars); i++ {
		if task.Spec.Sidecars[i].SecurityContext == nil {
			task.Spec.Sidecars[i].SecurityContext = &corev1.SecurityContext{}
		}
		setNotElevatedPrivileges(task.Spec.Sidecars[i].SecurityContext)
	}
	if task.Spec.StepTemplate != nil {
		if task.Spec.StepTemplate.SecurityContext == nil {
			task.Spec.StepTemplate.SecurityContext = &corev1.SecurityContext{}
		}
		setNotElevatedPrivileges(task.Spec.StepTemplate.SecurityContext)
	}
}

func setNotElevatedPrivileges(securityContext *corev1.SecurityContext) {
	securityContext.RunAsNonRoot = commonUtils.BoolPtr(true)
	if securityContext.RunAsUser != nil && *securityContext.RunAsUser == 0 {
		securityContext.RunAsUser = nil
	}
	if securityContext.RunAsGroup != nil && *securityContext.RunAsGroup == 0 {
		securityContext.RunAsGroup = nil
	}
	securityContext.WindowsOptions = nil
	securityContext.SELinuxOptions = nil
	securityContext.Privileged = commonUtils.BoolPtr(false)
	securityContext.AllowPrivilegeEscalation = commonUtils.BoolPtr(false)
	if securityContext.Capabilities == nil {
		securityContext.Capabilities = &corev1.Capabilities{}
	}
	securityContext.Capabilities.Drop = []corev1.Capability{"ALL"}
}

func (ctx *pipelineContext) getPipelineTasks(pipelineFilePath string, pipeline *pipelinev1.Pipeline) ([]pipelinev1.Task, error) {
	taskMap, err := getTasks(pipelineFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed get tasks: %v", err)
	}
	if len(taskMap) == 0 {
		return nil, fmt.Errorf("no tasks found: %v", err)
	}
	var tasks []pipelinev1.Task
	var validateTaskErrors []error
	for _, pipelineSpecTask := range pipeline.Spec.Tasks {
		task, taskExists := taskMap[pipelineSpecTask.TaskRef.Name]
		if !taskExists {
			validateTaskErrors = append(validateTaskErrors, fmt.Errorf("missing the pipeline task %s, referenced to the task %s", pipelineSpecTask.Name, pipelineSpecTask.TaskRef.Name))
			continue
		}
		validateTaskErrors = append(validateTaskErrors, validation.ValidateTask(&task))
		tasks = append(tasks, task)
	}
	return tasks, errors.Join(validateTaskErrors...)
}

func (ctx *pipelineContext) getPipelineFilePath(pipelineFile string) (string, error) {
	if len(pipelineFile) == 0 {
		pipelineFile = pipelineDefaults.DefaultPipelineFileName
		log.Debug().Msgf("Tekton pipeline file name is not specified, using the default file name %s", pipelineDefaults.DefaultPipelineFileName)
	}
	pipelineFile = strings.TrimPrefix(pipelineFile, "/") // Tekton pipeline folder currently is relative to the Radix config file repository folder
	configFolder := filepath.Dir(ctx.pipelineInfo.PipelineArguments.RadixConfigFile)
	return filepath.Join(configFolder, pipelineFile), nil
}

func (ctx *pipelineContext) createPipeline(envName string, pipeline *pipelinev1.Pipeline, tasks []pipelinev1.Task, timestamp string) error {

	originalPipelineName := pipeline.Name
	var errs []error
	taskMap, err := ctx.buildTasks(envName, tasks, timestamp)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to build task for pipeline %s: %w", originalPipelineName, err))
	}

	_, azureClientIdPipelineParamExist := ctx.GetEnvVars(envName)[pipelineDefaults.AzureClientIdEnvironmentVariable]
	if azureClientIdPipelineParamExist {
		ensureAzureClientIdParamExistInPipelineParams(pipeline)
	}

	for taskIndex, pipelineSpecTask := range pipeline.Spec.Tasks {
		task, ok := taskMap[pipelineSpecTask.TaskRef.Name]
		if !ok {
			errs = append(errs, fmt.Errorf("task %s has not been created", pipelineSpecTask.Name))
			continue
		}
		pipeline.Spec.Tasks[taskIndex].TaskRef = &pipelinev1.TaskRef{Name: task.Name}
		if azureClientIdPipelineParamExist {
			ensureAzureClientIdParamExistInTaskParams(pipeline, taskIndex, task)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	pipelineName := fmt.Sprintf("radix-pipeline-%s-%s-%s-%s", internal.GetShortName(envName), internal.GetShortName(originalPipelineName), timestamp, ctx.hash)
	pipeline.ObjectMeta.Name = pipelineName
	pipeline.ObjectMeta.Labels = labels.GetLabelsForEnvironment(ctx.GetPipelineInfo(), envName)
	pipeline.ObjectMeta.Annotations = map[string]string{
		kube.RadixBranchAnnotation:              ctx.pipelineInfo.PipelineArguments.Branch,
		pipelineDefaults.PipelineNameAnnotation: originalPipelineName,
	}
	if ctx.ownerReference != nil {
		pipeline.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*ctx.ownerReference}
	}
	err = ctx.createTasks(taskMap)
	if err != nil {
		return fmt.Errorf("tasks have not been created. Error: %w", err)
	}
	log.Info().Msgf("creates %d tasks for the environment %s", len(taskMap), envName)

	_, err = ctx.tektonClient.TektonV1().Pipelines(utils.GetAppNamespace(ctx.pipelineInfo.GetAppName())).Create(context.Background(), pipeline, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("pipeline %s has not been created. Error: %w", pipeline.Name, err)
	}
	log.Info().Msgf("created the pipeline %s for the environment %s", pipeline.Name, envName)
	return nil
}

func (ctx *pipelineContext) createTasks(taskMap map[string]pipelinev1.Task) error {
	namespace := utils.GetAppNamespace(ctx.pipelineInfo.PipelineArguments.AppName)
	var errs []error
	for _, task := range taskMap {
		_, err := ctx.tektonClient.TektonV1().Tasks(namespace).Create(context.Background(), &task,
			metav1.CreateOptions{})
		if err != nil {
			errs = append(errs, fmt.Errorf("task %s has not been created. Error: %w", task.Name, err))
		}
	}
	return errors.Join(errs...)
}

func getPipeline(pipelineFileName string) (*pipelinev1.Pipeline, error) {
	pipelineFolder := filepath.Dir(pipelineFileName)
	if _, err := os.Stat(pipelineFolder); os.IsNotExist(err) {
		return nil, fmt.Errorf("missing pipeline folder: %s", pipelineFolder)
	}
	pipelineData, err := os.ReadFile(pipelineFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read the pipeline file %s: %v", pipelineFileName, err)
	}
	var pipeline pipelinev1.Pipeline
	err = yaml.Unmarshal(pipelineData, &pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to load the pipeline from the file %s: %v", pipelineFileName, err)
	}
	hotfixForPipelineDefaultParamsWithBrokenValue(&pipeline)
	hotfixForPipelineTasksParamsWithBrokenValue(&pipeline)

	log.Debug().Msgf("loaded pipeline %s", pipelineFileName)
	err = validation.ValidatePipeline(&pipeline)
	if err != nil {
		return nil, err
	}
	return &pipeline, nil
}

func hotfixForPipelineDefaultParamsWithBrokenValue(pipeline *pipelinev1.Pipeline) {
	for ip, p := range pipeline.Spec.Params {
		if p.Default != nil && p.Default.ObjectVal != nil && p.Type == "string" && p.Default.ObjectVal["stringVal"] != "" {
			pipeline.Spec.Params[ip].Default = &pipelinev1.ParamValue{
				Type:      "string",
				StringVal: p.Default.ObjectVal["stringVal"],
			}
		}
	}
}
func hotfixForPipelineTasksParamsWithBrokenValue(pipeline *pipelinev1.Pipeline) {
	for it, task := range pipeline.Spec.Tasks {
		for ip, p := range task.Params {
			if p.Value.ObjectVal != nil && p.Value.ObjectVal["type"] == "string" && p.Value.ObjectVal["stringVal"] != "" {
				pipeline.Spec.Tasks[it].Params[ip].Value = pipelinev1.ParamValue{
					Type:      "string",
					StringVal: p.Value.ObjectVal["stringVal"],
				}
			}
		}
	}
}

func getTasks(pipelineFilePath string) (map[string]pipelinev1.Task, error) {
	pipelineFolder := filepath.Dir(pipelineFilePath)
	if _, err := os.Stat(pipelineFolder); os.IsNotExist(err) {
		return nil, fmt.Errorf("missing pipeline folder: %s", pipelineFolder)
	}

	fileNameList, err := filepath.Glob(filepath.Join(pipelineFolder, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("failed to scan pipeline folder %s: %v", pipelineFolder, err)
	}
	taskMap := make(map[string]pipelinev1.Task)
	for _, fileName := range fileNameList {
		if strings.EqualFold(fileName, pipelineFilePath) {
			continue
		}
		fileData, err := os.ReadFile(fileName)
		if err != nil {
			return nil, fmt.Errorf("failed to read the file %s: %v", fileName, err)
		}
		fileData = []byte(strings.ReplaceAll(string(fileData), pipelineDefaults.SubstitutionRadixBuildSecretsSource, pipelineDefaults.SubstitutionRadixBuildSecretsTarget))
		fileData = []byte(strings.ReplaceAll(string(fileData), pipelineDefaults.SubstitutionRadixGitDeployKeySource, pipelineDefaults.SubstitutionRadixGitDeployKeyTarget))

		task := pipelinev1.Task{}
		err = yaml.Unmarshal(fileData, &task)
		if err != nil {
			return nil, fmt.Errorf("failed to read data from the file %s: %v", fileName, err)
		}
		if !taskIsValid(&task) {
			log.Debug().Msgf("skip the file %s - not a Tekton task", fileName)
			continue
		}
		addGitDeployKeyVolume(&task)
		taskMap[task.Name] = task
	}
	return taskMap, nil
}

func addGitDeployKeyVolume(task *pipelinev1.Task) {
	task.Spec.Volumes = append(task.Spec.Volumes, corev1.Volume{
		Name: pipelineDefaults.SubstitutionRadixGitDeployKeyTarget,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  operatorDefaults.GitPrivateKeySecretName,
				DefaultMode: &privateSshFolderMode,
			},
		},
	})
}

func taskIsValid(task *pipelinev1.Task) bool {
	return strings.HasPrefix(task.APIVersion, "tekton.dev/") && task.Kind == "Task" && len(task.ObjectMeta.Name) > 1
}

func ensureAzureClientIdParamExistInPipelineParams(pipeline *pipelinev1.Pipeline) {
	if !pipelineHasAzureIdentityClientIdParam(pipeline) {
		addAzureIdentityClientIdParamToPipeline(pipeline)
	}
}

func ensureAzureClientIdParamExistInTaskParams(pipeline *pipelinev1.Pipeline, pipelineTaskIndex int, task pipelinev1.Task) {
	if taskHasAzureIdentityClientIdParam(task) && !pipelineTaskHasAzureIdentityClientIdParam(pipeline, pipelineTaskIndex) {
		addAzureIdentityClientIdParamToPipelineTask(pipeline, pipelineTaskIndex)
	}
}

func addAzureIdentityClientIdParamToPipeline(pipeline *pipelinev1.Pipeline) {
	pipeline.Spec.Params = append(pipeline.Spec.Params, pipelinev1.ParamSpec{Name: pipelineDefaults.AzureClientIdEnvironmentVariable, Type: pipelinev1.ParamTypeString, Description: "Defines the Client ID for a user defined managed identity or application ID for an application registration"})
}

func pipelineHasAzureIdentityClientIdParam(pipeline *pipelinev1.Pipeline) bool {
	return slice.Any(pipeline.Spec.Params, func(paramSpec pipelinev1.ParamSpec) bool {
		return paramSpec.Name == pipelineDefaults.AzureClientIdEnvironmentVariable
	})
}

func addAzureIdentityClientIdParamToPipelineTask(pipeline *pipelinev1.Pipeline, taskIndex int) {
	pipeline.Spec.Tasks[taskIndex].Params = append(pipeline.Spec.Tasks[taskIndex].Params,
		pipelinev1.Param{
			Name:  pipelineDefaults.AzureClientIdEnvironmentVariable,
			Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: fmt.Sprintf("$(params.%s)", pipelineDefaults.AzureClientIdEnvironmentVariable)},
		})
}

func taskHasAzureIdentityClientIdParam(task pipelinev1.Task) bool {
	return slice.Any(task.Spec.Params, func(paramSpec pipelinev1.ParamSpec) bool {
		return paramSpec.Name == pipelineDefaults.AzureClientIdEnvironmentVariable
	})
}

func pipelineTaskHasAzureIdentityClientIdParam(pipeline *pipelinev1.Pipeline, taskIndex int) bool {
	return slice.Any(pipeline.Spec.Tasks[taskIndex].Params, func(param pipelinev1.Param) bool {
		return param.Name == pipelineDefaults.AzureClientIdEnvironmentVariable
	})
}

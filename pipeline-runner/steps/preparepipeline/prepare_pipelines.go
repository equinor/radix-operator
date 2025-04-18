package preparepipeline

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/labels"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/validation"
	"github.com/equinor/radix-operator/pipeline-runner/utils/annotations"
	"github.com/equinor/radix-operator/pipeline-runner/utils/git"
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
func (pipelineCtx *pipelineContext) GetBuildContext() (*model.BuildContext, error) {
	gitHash, err := pipelineCtx.getGitHash()
	if err != nil {
		return nil, err
	}
	if err = git.ResetGitHead(pipelineCtx.GetPipelineInfo().GetGitWorkspace(), gitHash); err != nil {
		return nil, err
	}

	pipelineType := pipelineCtx.GetPipelineInfo().GetRadixPipelineType()
	buildContext := model.BuildContext{}

	if pipelineType == radixv1.BuildDeploy || pipelineType == radixv1.Build {
		pipelineTargetCommitHash, commitTags, err := pipelineCtx.getGitAttributes()
		if err != nil {
			return nil, err
		}
		pipelineCtx.GetPipelineInfo().SetGitAttributes(pipelineTargetCommitHash, commitTags)

		if pipelineCtx.HasCommitID() {
			radixConfigWasChanged, environmentsToBuild, err := pipelineCtx.analyseSourceRepositoryChanges(pipelineTargetCommitHash)
			if err != nil {
				return nil, err
			}
			buildContext.ChangedRadixConfig = radixConfigWasChanged
			buildContext.EnvironmentsToBuild = environmentsToBuild
		} // when commit hash is not provided, build all
	}
	return &buildContext, nil
}

func (pipelineCtx *pipelineContext) getGitAttributes() (string, string, error) {
	pipelineArgs := pipelineCtx.GetPipelineInfo().PipelineArguments
	pipelineTargetCommitHash, commitTags, err := git.GetCommitHashAndTags(pipelineArgs.GitWorkspace, pipelineArgs.CommitID, pipelineArgs.Branch)
	if err != nil {
		return "", "", err
	}
	if err = validate.GitTagsContainIllegalChars(commitTags); err != nil {
		return "", "", err
	}
	return pipelineTargetCommitHash, commitTags, nil
}

// GetEnvironmentSubPipelinesToRun Prepare sub-pipelines for the target environments
func (pipelineCtx *pipelineContext) GetEnvironmentSubPipelinesToRun() ([]model.EnvironmentSubPipelineToRun, error) {
	var environmentSubPipelinesToRun []model.EnvironmentSubPipelineToRun
	if pipelineCtx.GetPipelineInfo().StopPipeline {
		log.Info().Msg("Pipeline is stopped, skip sub-pipelines")
		return nil, nil
	}
	var errs []error
	timestamp := time.Now().Format("20060102150405")
	for _, targetEnv := range pipelineCtx.GetPipelineTargetEnvironments() {
		log.Debug().Msgf("create a sub-pipeline for the environment %s", targetEnv)
		runSubPipeline, pipelineFilePath, err := pipelineCtx.prepareSubPipelineForTargetEnv(targetEnv, timestamp)
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
	if err := errors.Join(errs...); err != nil {
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

func (pipelineCtx *pipelineContext) analyseSourceRepositoryChanges(pipelineTargetCommitHash string) (bool, []model.EnvironmentToBuild, error) {
	radixDeploymentCommitHashProvider := commithash.NewProvider(pipelineCtx.GetKubeClient(), pipelineCtx.GetRadixClient(), pipelineCtx.pipelineInfo.GetAppName(), pipelineCtx.GetPipelineTargetEnvironments())
	lastCommitHashesForEnvs, err := radixDeploymentCommitHashProvider.GetLastCommitHashesForEnvironments()
	if err != nil {
		return false, nil, err
	}

	changesFromGitRepository, radixConfigWasChanged, err := git.GetChangesFromGitRepository(pipelineCtx.pipelineInfo.GetGitWorkspace(),
		pipelineCtx.pipelineInfo.GetRadixConfigBranch(),
		pipelineCtx.pipelineInfo.GetRadixConfigFileInWorkspace(),
		pipelineTargetCommitHash,
		lastCommitHashesForEnvs)
	if err != nil {
		return false, nil, err
	}

	environmentsToBuild := pipelineCtx.getEnvironmentsToBuild(changesFromGitRepository)
	return radixConfigWasChanged, environmentsToBuild, nil
}

func (pipelineCtx *pipelineContext) getEnvironmentsToBuild(changesFromGitRepository map[string][]string) []model.EnvironmentToBuild {
	var environmentsToBuild []model.EnvironmentToBuild
	for envName, changedFolders := range changesFromGitRepository {
		var componentsWithChangedSource []string
		for _, radixComponent := range pipelineCtx.GetRadixApplication().Spec.Components {
			if componentHasChangedSource(envName, &radixComponent, changedFolders) {
				componentsWithChangedSource = append(componentsWithChangedSource, radixComponent.GetName())
			}
		}
		for _, radixJobComponent := range pipelineCtx.GetRadixApplication().Spec.Jobs {
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

func (pipelineCtx *pipelineContext) prepareSubPipelineForTargetEnv(envName, timestamp string) (bool, string, error) {
	pipelineFilePath, err := pipelineCtx.getPipelineFilePath("") // TODO - get pipeline for the envName
	if err != nil {
		return false, "", err
	}

	err = pipelineCtx.pipelineFileExists(pipelineFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Info().Msgf("There is no Tekton pipeline file: %s for the environment %s. Skip Tekton pipeline", pipelineFilePath, envName)
			return false, "", nil
		}
		return false, "", err
	}

	pipeline, err := getPipeline(pipelineFilePath)
	if err != nil {
		return false, "", err
	}
	log.Debug().Msgf("loaded a pipeline with %d tasks", len(pipeline.Spec.Tasks))

	tasks, err := pipelineCtx.getPipelineTasks(pipelineFilePath, pipeline)
	if err != nil {
		return false, "", err
	}
	log.Debug().Msg("all pipeline tasks found")
	err = pipelineCtx.createPipeline(envName, pipeline, tasks, timestamp)
	if err != nil {
		return false, "", err
	}
	return true, pipelineFilePath, nil
}

func (pipelineCtx *pipelineContext) pipelineFileExists(pipelineFilePath string) error {
	_, err := os.Stat(pipelineFilePath)
	return err
}

func (pipelineCtx *pipelineContext) buildTasks(envName string, tasks []pipelinev1.Task, timestamp string) (map[string]pipelinev1.Task, error) {
	var errs []error
	taskMap := make(map[string]pipelinev1.Task)
	for _, task := range tasks {
		originalTaskName := task.Name
		task.ObjectMeta.Name = fmt.Sprintf("radix-task-%s-%s-%s-%s", internal.GetShortName(envName), internal.GetShortName(originalTaskName), timestamp, pipelineCtx.GetHash())
		if task.ObjectMeta.Labels == nil {
			task.ObjectMeta.Labels = map[string]string{}
		}
		if task.ObjectMeta.Annotations == nil {
			task.ObjectMeta.Annotations = map[string]string{}
		}

		for k, v := range labels.GetSubPipelineLabelsForEnvironment(pipelineCtx.GetPipelineInfo(), envName) {
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

		task.ObjectMeta.Annotations[operatorDefaults.PipelineTaskNameAnnotation] = originalTaskName
		if pipelineCtx.ownerReference != nil {
			task.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*pipelineCtx.ownerReference}
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

func (pipelineCtx *pipelineContext) getPipelineTasks(pipelineFilePath string, pipeline *pipelinev1.Pipeline) ([]pipelinev1.Task, error) {
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

func (pipelineCtx *pipelineContext) getPipelineFilePath(pipelineFile string) (string, error) {
	if len(pipelineFile) == 0 {
		pipelineFile = defaults.DefaultPipelineFileName
		log.Debug().Msgf("Tekton pipeline file name is not specified, using the default file name %s", defaults.DefaultPipelineFileName)
	}
	pipelineFile = strings.TrimPrefix(pipelineFile, "/") // Tekton pipeline folder currently is relative to the Radix config file repository folder
	configFolder := filepath.Dir(pipelineCtx.pipelineInfo.GetRadixConfigFileInWorkspace())
	return filepath.Join(configFolder, pipelineFile), nil
}

func (pipelineCtx *pipelineContext) createPipeline(envName string, pipeline *pipelinev1.Pipeline, tasks []pipelinev1.Task, timestamp string) error {
	originalPipelineName := pipeline.Name
	var errs []error
	taskMap, err := pipelineCtx.buildTasks(envName, tasks, timestamp)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to build task for pipeline %s: %w", originalPipelineName, err))
	}

	_, azureClientIdPipelineParamExist := pipelineCtx.GetEnvVars(envName)[defaults.AzureClientIdEnvironmentVariable]
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
	pipelineName := fmt.Sprintf("radix-pipeline-%s-%s-%s-%s", internal.GetShortName(envName), internal.GetShortName(originalPipelineName), timestamp, pipelineCtx.GetHash())
	pipeline.ObjectMeta.Name = pipelineName
	pipeline.ObjectMeta.Labels = labels.GetSubPipelineLabelsForEnvironment(pipelineCtx.GetPipelineInfo(), envName)
	pipeline.ObjectMeta.Annotations = map[string]string{
		kube.RadixBranchAnnotation:              pipelineCtx.pipelineInfo.PipelineArguments.Branch,
		operatorDefaults.PipelineNameAnnotation: originalPipelineName,
	}
	if pipelineCtx.ownerReference != nil {
		pipeline.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*pipelineCtx.ownerReference}
	}
	err = pipelineCtx.createTasks(taskMap)
	if err != nil {
		return fmt.Errorf("tasks have not been created. Error: %w", err)
	}
	log.Info().Msgf("creates %d tasks for the environment %s", len(taskMap), envName)

	_, err = pipelineCtx.tektonClient.TektonV1().Pipelines(utils.GetAppNamespace(pipelineCtx.pipelineInfo.GetAppName())).Create(context.Background(), pipeline, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("pipeline %s has not been created. Error: %w", pipeline.Name, err)
	}
	log.Info().Msgf("created the pipeline %s for the environment %s", pipeline.Name, envName)
	return nil
}

func (pipelineCtx *pipelineContext) createTasks(taskMap map[string]pipelinev1.Task) error {
	namespace := utils.GetAppNamespace(pipelineCtx.pipelineInfo.PipelineArguments.AppName)
	var errs []error
	for _, task := range taskMap {
		_, err := pipelineCtx.tektonClient.TektonV1().Tasks(namespace).Create(context.Background(), &task,
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
		fileData = []byte(strings.ReplaceAll(string(fileData), defaults.SubstitutionRadixBuildSecretsSource, defaults.SubstitutionRadixBuildSecretsTarget))
		fileData = []byte(strings.ReplaceAll(string(fileData), defaults.SubstitutionRadixGitDeployKeySource, defaults.SubstitutionRadixGitDeployKeyTarget))

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
		Name: defaults.SubstitutionRadixGitDeployKeyTarget,
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
	pipeline.Spec.Params = append(pipeline.Spec.Params, pipelinev1.ParamSpec{Name: defaults.AzureClientIdEnvironmentVariable, Type: pipelinev1.ParamTypeString, Description: "Defines the Client ID for a user defined managed identity or application ID for an application registration"})
}

func pipelineHasAzureIdentityClientIdParam(pipeline *pipelinev1.Pipeline) bool {
	return slice.Any(pipeline.Spec.Params, func(paramSpec pipelinev1.ParamSpec) bool {
		return paramSpec.Name == defaults.AzureClientIdEnvironmentVariable
	})
}

func addAzureIdentityClientIdParamToPipelineTask(pipeline *pipelinev1.Pipeline, taskIndex int) {
	pipeline.Spec.Tasks[taskIndex].Params = append(pipeline.Spec.Tasks[taskIndex].Params,
		pipelinev1.Param{
			Name:  defaults.AzureClientIdEnvironmentVariable,
			Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: fmt.Sprintf("$(params.%s)", defaults.AzureClientIdEnvironmentVariable)},
		})
}

func taskHasAzureIdentityClientIdParam(task pipelinev1.Task) bool {
	return slice.Any(task.Spec.Params, func(paramSpec pipelinev1.ParamSpec) bool {
		return paramSpec.Name == defaults.AzureClientIdEnvironmentVariable
	})
}

func pipelineTaskHasAzureIdentityClientIdParam(pipeline *pipelinev1.Pipeline, taskIndex int) bool {
	return slice.Any(pipeline.Spec.Tasks[taskIndex].Params, func(param pipelinev1.Param) bool {
		return param.Name == defaults.AzureClientIdEnvironmentVariable
	})
}

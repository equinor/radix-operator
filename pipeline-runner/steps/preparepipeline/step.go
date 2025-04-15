package preparepipeline

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sigs.k8s.io/yaml"
	"slices"
	"strings"
	"time"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	pipelineDefaults "github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/labels"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/validation"
	"github.com/equinor/radix-operator/pipeline-runner/utils/annotations"
	"github.com/equinor/radix-operator/pipeline-runner/utils/git"
	ownerreferences "github.com/equinor/radix-operator/pipeline-runner/utils/owner_references"
	"github.com/equinor/radix-operator/pipeline-runner/utils/radix/applicationconfig"
	"github.com/equinor/radix-operator/pipeline-runner/utils/radix/deployment/commithash"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/rs/zerolog/log"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// PreparePipelinesStepImplementation Step to prepare radixconfig and Tekton pipelines
type PreparePipelinesStepImplementation struct {
	stepType pipeline.StepType
	model.DefaultStepImplementation
}

// NewPreparePipelinesStep Constructor.
func NewPreparePipelinesStep() model.Step {
	return &PreparePipelinesStepImplementation{
		stepType: pipeline.PreparePipelinesStep,
	}
}

func (cli *PreparePipelinesStepImplementation) Init(ctx context.Context, kubeClient kubernetes.Interface, radixClient radixclient.Interface, kubeUtil *kube.Kube, prometheusOperatorClient monitoring.Interface, tektonClient tektonclient.Interface, rr *radixv1.RadixRegistration) {
	cli.DefaultStepImplementation.Init(ctx, kubeClient, radixClient, kubeUtil, prometheusOperatorClient, tektonClient, rr)
}

// SucceededMsg Override of default step method
func (cli *PreparePipelinesStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Succeded: prepare pipelines step for application %s", cli.GetAppName())
}

// ErrorMsg Override of default step method
func (cli *PreparePipelinesStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed prepare pipelines for the application %s. Error: %v", cli.GetAppName(), err)
}

// Run Override of default step method
func (cli *PreparePipelinesStepImplementation) Run(ctx context.Context, pipelineInfo *model.PipelineInfo) error {
	branch := pipelineInfo.PipelineArguments.Branch
	commitID := pipelineInfo.PipelineArguments.CommitID
	appName := cli.GetAppName()
	logPipelineInfo(ctx, pipelineInfo.Definition.Type, appName, branch, commitID)

	if pipelineInfo.IsPipelineType(radixv1.Promote) {
		sourceDeploymentGitCommitHash, sourceDeploymentGitBranch, err := cli.getSourceDeploymentGitInfo(ctx, appName, pipelineInfo.PipelineArguments.FromEnvironment, pipelineInfo.PipelineArguments.DeploymentName)
		if err != nil {
			return err
		}
		pipelineInfo.SourceDeploymentGitCommitHash = sourceDeploymentGitCommitHash
		pipelineInfo.SourceDeploymentGitBranch = sourceDeploymentGitBranch
	}

	radixApplication, err := LoadRadixAppConfig(cli.GetRadixClient(), pipelineInfo)
	if err != nil {
		return err
	}
	pipelineInfo.RadixApplication = radixApplication
	targetEnvironments, ignoredForWebhookEnvs, err := internal.GetPipelineTargetEnvironments(ctx, pipelineInfo)
	if err != nil {
		return err
	}
	if len(targetEnvironments) > 0 {
		log.Ctx(ctx).Info().Msgf("Environment(s) %v are mapped to the branch %s.", strings.Join(targetEnvironments, ", "), pipelineInfo.GetBranch())
	} else {
		log.Ctx(ctx).Info().Msgf("No environments are mapped to the branch %s.", pipelineInfo.GetBranch())
	}
	if len(ignoredForWebhookEnvs) > 0 {
		log.Ctx(ctx).Info().Msgf("Following environment(s) are ignored for the webhook: %s.", strings.Join(ignoredForWebhookEnvs, ", "))
	}

	log.Ctx(ctx).Info().Msgf("Pipeline type: %s", pipelineInfo.GetRadixPipelineType())

	buildContext, err := cli.GetBuildContext(pipelineInfo, targetEnvironments)
	if err != nil {
		return err
	}
	pipelineInfo.SetBuildContext(buildContext)

	if pipelineInfo.IsPipelineType(radixv1.BuildDeploy) {
		pipelineInfo.StopPipeline, pipelineInfo.StopPipelineMessage = getPipelineShouldBeStopped(ctx, pipelineInfo.BuildContext)
	}

	buildContext.EnvironmentSubPipelinesToRun, err = cli.GetEnvironmentSubPipelinesToRun(pipelineInfo, targetEnvironments)
	if err != nil {
		return err
	}
	return nil
}

func getPipelineShouldBeStopped(ctx context.Context, buildContext *model.BuildContext) (bool, string) {
	if buildContext == nil || buildContext.ChangedRadixConfig ||
		len(buildContext.EnvironmentsToBuild) == 0 ||
		len(buildContext.EnvironmentSubPipelinesToRun) > 0 {
		return false, ""
	}
	for _, environmentToBuild := range buildContext.EnvironmentsToBuild {
		if len(environmentToBuild.Components) > 0 {
			return false, ""
		}
	}
	message := "No components with changed source code and the Radix config file was not changed. The pipeline will not proceed."
	log.Ctx(ctx).Info().Msg(message)
	return true, message
}

func logPipelineInfo(ctx context.Context, pipelineType radixv1.RadixPipelineType, appName, branch, commitID string) {
	stringBuilder := strings.Builder{}
	stringBuilder.WriteString(fmt.Sprintf("Prepare pipeline %s for the app %s", pipelineType, appName))
	if len(branch) > 0 {
		stringBuilder.WriteString(fmt.Sprintf(", the branch %s", branch))
	}
	if len(branch) > 0 {
		stringBuilder.WriteString(fmt.Sprintf(", the commit %s", commitID))
	}
	log.Ctx(ctx).Info().Msg(stringBuilder.String())
}

func (cli *PreparePipelinesStepImplementation) getSourceDeploymentGitInfo(ctx context.Context, appName, sourceEnvName, sourceDeploymentName string) (string, string, error) {
	ns := utils.GetEnvironmentNamespace(appName, sourceEnvName)
	rd, err := cli.GetKubeUtil().GetRadixDeployment(ctx, ns, sourceDeploymentName)
	if err != nil {
		return "", "", err
	}
	gitHash := internal.GetGitCommitHashFromDeployment(rd)
	gitBranch := rd.Annotations[kube.RadixBranchAnnotation]
	return gitHash, gitBranch, err
}

// GetBuildContext Prepare build context
func (cli *PreparePipelinesStepImplementation) GetBuildContext(pipelineInfo *model.PipelineInfo, targetEnvironments []string) (*model.BuildContext, error) {
	gitHash, err := getGitHash(pipelineInfo)
	if err != nil {
		return nil, err
	}
	if err = git.ResetGitHead(pipelineInfo.GetGitWorkspace(), gitHash); err != nil {
		return nil, err
	}

	pipelineType := pipelineInfo.GetRadixPipelineType()
	buildContext := model.BuildContext{}

	if pipelineType == radixv1.BuildDeploy || pipelineType == radixv1.Build {
		pipelineTargetCommitHash, commitTags, err := getGitAttributes(pipelineInfo)
		if err != nil {
			return nil, err
		}
		pipelineInfo.SetGitAttributes(pipelineTargetCommitHash, commitTags)

		if len(pipelineInfo.PipelineArguments.CommitID) > 0 {
			radixConfigWasChanged, environmentsToBuild, err := cli.analyseSourceRepositoryChanges(pipelineInfo, targetEnvironments, pipelineTargetCommitHash)
			if err != nil {
				return nil, err
			}
			buildContext.ChangedRadixConfig = radixConfigWasChanged
			buildContext.EnvironmentsToBuild = environmentsToBuild
		} // when commit hash is not provided, build all
	}
	return &buildContext, nil
}

func getGitAttributes(pipelineInfo *model.PipelineInfo) (string, string, error) {
	pipelineArgs := pipelineInfo.PipelineArguments
	pipelineTargetCommitHash, commitTags, err := git.GetCommitHashAndTags(pipelineArgs.GitWorkspace, pipelineArgs.CommitID, pipelineArgs.Branch)
	if err != nil {
		return "", "", err
	}
	if err = radixvalidators.GitTagsContainIllegalChars(commitTags); err != nil {
		return "", "", err
	}
	return pipelineTargetCommitHash, commitTags, nil
}

var privateSshFolderMode int32 = 0444

// GetEnvironmentSubPipelinesToRun Prepare sub-pipelines for the target environments
func (cli *PreparePipelinesStepImplementation) GetEnvironmentSubPipelinesToRun(pipelineInfo *model.PipelineInfo, targetEnvironments []string) ([]model.EnvironmentSubPipelineToRun, error) {
	var environmentSubPipelinesToRun []model.EnvironmentSubPipelineToRun
	if pipelineInfo.StopPipeline {
		log.Info().Msg("Pipeline is stopped, skip sub-pipelines")
		return nil, nil
	}
	var errs []error
	timestamp := time.Now().Format("20060102150405")
	for _, targetEnv := range targetEnvironments {
		log.Debug().Msgf("create a sub-pipeline for the environment %s", targetEnv)
		runSubPipeline, pipelineFilePath, err := cli.prepareSubPipelineForTargetEnv(pipelineInfo, targetEnv, timestamp)
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

func (cli *PreparePipelinesStepImplementation) analyseSourceRepositoryChanges(pipelineInfo *model.PipelineInfo, targetEnvironments []string, pipelineTargetCommitHash string) (bool, []model.EnvironmentToBuild, error) {
	radixDeploymentCommitHashProvider := commithash.NewProvider(cli.GetKubeClient(), cli.GetRadixClient(), pipelineInfo.GetAppName(), targetEnvironments)
	lastCommitHashesForEnvs, err := radixDeploymentCommitHashProvider.GetLastCommitHashesForEnvironments()
	if err != nil {
		return false, nil, err
	}

	changesFromGitRepository, radixConfigWasChanged, err := git.GetChangesFromGitRepository(pipelineInfo.GetGitWorkspace(),
		pipelineInfo.GetRadixConfigBranch(),
		pipelineInfo.GetRadixConfigFileInWorkspace(),
		pipelineTargetCommitHash,
		lastCommitHashesForEnvs)
	if err != nil {
		return false, nil, err
	}

	environmentsToBuild := cli.getEnvironmentsToBuild(pipelineInfo, changesFromGitRepository)
	return radixConfigWasChanged, environmentsToBuild, nil
}

func (cli *PreparePipelinesStepImplementation) getEnvironmentsToBuild(pipelineInfo *model.PipelineInfo, changesFromGitRepository map[string][]string) []model.EnvironmentToBuild {
	var environmentsToBuild []model.EnvironmentToBuild
	for envName, changedFolders := range changesFromGitRepository {
		var componentsWithChangedSource []string
		for _, radixComponent := range pipelineInfo.GetRadixApplication().Spec.Components {
			if componentHasChangedSource(envName, &radixComponent, changedFolders) {
				componentsWithChangedSource = append(componentsWithChangedSource, radixComponent.GetName())
			}
		}
		for _, radixJobComponent := range pipelineInfo.GetRadixApplication().Spec.Jobs {
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

func (cli *PreparePipelinesStepImplementation) prepareSubPipelineForTargetEnv(pipelineInfo *model.PipelineInfo, envName, timestamp string) (bool, string, error) {
	pipelineFilePath, err := getPipelineFilePath(pipelineInfo, "") // TODO - get pipeline for the envName
	if err != nil {
		return false, "", err
	}

	exists, err := fileExists(pipelineFilePath)
	if err != nil {
		return false, "", err
	}
	if !exists {
		log.Info().Msgf("There is no Tekton pipeline file: %s for the environment %s. Skip Tekton pipeline", pipelineFilePath, envName)
		return false, "", nil
	}
	pipeline, err := getPipeline(pipelineFilePath)
	if err != nil {
		return false, "", err
	}
	log.Debug().Msgf("loaded a pipeline with %d tasks", len(pipeline.Spec.Tasks))

	tasks, err := getPipelineTasks(pipelineFilePath, pipeline)
	if err != nil {
		return false, "", err
	}
	log.Debug().Msg("all pipeline tasks found")
	err = cli.createPipeline(envName, pipeline, tasks, timestamp, pipelineInfo)
	if err != nil {
		return false, "", err
	}
	return true, pipelineFilePath, nil
}

func fileExists(filePath string) (bool, error) {
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (cli *PreparePipelinesStepImplementation) buildTasks(envName string, tasks []v1.Task, timestamp string, pipelineInfo *model.PipelineInfo) (map[string]v1.Task, error) {
	var errs []error
	taskMap := make(map[string]v1.Task)
	hash := internal.GetJobNameHash(pipelineInfo)
	for _, task := range tasks {
		originalTaskName := task.Name
		task.ObjectMeta.Name = fmt.Sprintf("radix-task-%s-%s-%s-%s", internal.GetShortName(envName), internal.GetShortName(originalTaskName), timestamp, hash)
		if task.ObjectMeta.Labels == nil {
			task.ObjectMeta.Labels = map[string]string{}
		}
		if task.ObjectMeta.Annotations == nil {
			task.ObjectMeta.Annotations = map[string]string{}
		}

		for k, v := range labels.GetSubPipelineLabelsForEnvironment(pipelineInfo, envName) {
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

		task.ObjectMeta.Annotations[defaults.PipelineTaskNameAnnotation] = originalTaskName
		ownerReference := ownerreferences.GetOwnerReferenceOfJobFromLabels()
		if ownerReference != nil {
			task.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*ownerReference}
		}

		ensureCorrectSecureContext(&task)
		taskMap[originalTaskName] = task
		log.Debug().Msgf("created the task %s", task.Name)
	}
	return taskMap, errors.Join(errs...)
}

func sanitizeAzureSkipContainersAnnotation(task *v1.Task) error {
	skipSteps := strings.Split(task.ObjectMeta.Annotations[annotations.AzureWorkloadIdentitySkipContainers], ";")

	var errs []error
	for _, step := range skipSteps {
		sanitizedSkipStepName := strings.ToLower(strings.TrimSpace(step))
		if sanitizedSkipStepName == "" {
			continue
		}

		containsStep := slices.ContainsFunc(task.Spec.Steps, func(s v1.Step) bool {
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

func ensureCorrectSecureContext(task *v1.Task) {
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

func getPipelineTasks(pipelineFilePath string, pipeline *v1.Pipeline) ([]v1.Task, error) {
	taskMap, err := getTasks(pipelineFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed get tasks: %v", err)
	}
	if len(taskMap) == 0 {
		return nil, fmt.Errorf("no tasks found: %v", err)
	}
	var tasks []v1.Task
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

func getPipelineFilePath(pipelineInfo *model.PipelineInfo, pipelineFile string) (string, error) {
	if len(pipelineFile) == 0 {
		pipelineFile = pipelineDefaults.DefaultPipelineFileName
		log.Debug().Msgf("Tekton pipeline file name is not specified, using the default file name %s", pipelineDefaults.DefaultPipelineFileName)
	}
	pipelineFile = strings.TrimPrefix(pipelineFile, "/") // Tekton pipeline folder currently is relative to the Radix config file repository folder
	configFolder := filepath.Dir(pipelineInfo.GetRadixConfigFileInWorkspace())
	return filepath.Join(configFolder, pipelineFile), nil
}

func (cli *PreparePipelinesStepImplementation) createPipeline(envName string, pipeline *v1.Pipeline, tasks []v1.Task, timestamp string, pipelineInfo *model.PipelineInfo) error {
	originalPipelineName := pipeline.Name
	var errs []error
	taskMap, err := cli.buildTasks(envName, tasks, timestamp, pipelineInfo)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to build task for pipeline %s: %w", originalPipelineName, err))
	}

	_, azureClientIdPipelineParamExist := cli.GetEnvVars(pipelineInfo.RadixApplication, envName)[pipelineDefaults.AzureClientIdEnvironmentVariable]
	if azureClientIdPipelineParamExist {
		ensureAzureClientIdParamExistInPipelineParams(pipeline)
	}

	for taskIndex, pipelineSpecTask := range pipeline.Spec.Tasks {
		task, ok := taskMap[pipelineSpecTask.TaskRef.Name]
		if !ok {
			errs = append(errs, fmt.Errorf("task %s has not been created", pipelineSpecTask.Name))
			continue
		}
		pipeline.Spec.Tasks[taskIndex].TaskRef = &v1.TaskRef{Name: task.Name}
		if azureClientIdPipelineParamExist {
			ensureAzureClientIdParamExistInTaskParams(pipeline, taskIndex, task)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	hash := internal.GetJobNameHash(pipelineInfo)
	pipelineName := fmt.Sprintf("radix-pipeline-%s-%s-%s-%s", internal.GetShortName(envName), internal.GetShortName(originalPipelineName), timestamp, hash)
	pipeline.ObjectMeta.Name = pipelineName
	pipeline.ObjectMeta.Labels = labels.GetSubPipelineLabelsForEnvironment(pipelineInfo, envName)
	pipeline.ObjectMeta.Annotations = map[string]string{
		kube.RadixBranchAnnotation:      pipelineInfo.PipelineArguments.Branch,
		defaults.PipelineNameAnnotation: originalPipelineName,
	}
	ownerReference := ownerreferences.GetOwnerReferenceOfJobFromLabels()
	if ownerReference != nil {
		pipeline.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*ownerReference}
	}
	err = cli.createTasks(taskMap)
	if err != nil {
		return fmt.Errorf("tasks have not been created. Error: %w", err)
	}
	log.Info().Msgf("creates %d tasks for the environment %s", len(taskMap), envName)

	_, err = cli.GetTektonClient().TektonV1().Pipelines(utils.GetAppNamespace(pipelineInfo.GetAppName())).Create(context.Background(), pipeline, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("pipeline %s has not been created. Error: %w", pipeline.Name, err)
	}
	log.Info().Msgf("created the pipeline %s for the environment %s", pipeline.Name, envName)
	return nil
}

func (cli *PreparePipelinesStepImplementation) createTasks(taskMap map[string]v1.Task) error {
	namespace := utils.GetAppNamespace(cli.GetAppName())
	var errs []error
	for _, task := range taskMap {
		_, err := cli.GetTektonClient().TektonV1().Tasks(namespace).Create(context.Background(), &task,
			metav1.CreateOptions{})
		if err != nil {
			errs = append(errs, fmt.Errorf("task %s has not been created. Error: %w", task.Name, err))
		}
	}
	return errors.Join(errs...)
}

func getPipeline(pipelineFileName string) (*v1.Pipeline, error) {
	pipelineFolder := filepath.Dir(pipelineFileName)
	if _, err := os.Stat(pipelineFolder); os.IsNotExist(err) {
		return nil, fmt.Errorf("missing pipeline folder: %s", pipelineFolder)
	}
	pipelineData, err := os.ReadFile(pipelineFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read the pipeline file %s: %v", pipelineFileName, err)
	}
	var pipeline v1.Pipeline
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

func hotfixForPipelineDefaultParamsWithBrokenValue(pipeline *v1.Pipeline) {
	for ip, p := range pipeline.Spec.Params {
		if p.Default != nil && p.Default.ObjectVal != nil && p.Type == "string" && p.Default.ObjectVal["stringVal"] != "" {
			pipeline.Spec.Params[ip].Default = &v1.ParamValue{
				Type:      "string",
				StringVal: p.Default.ObjectVal["stringVal"],
			}
		}
	}
}
func hotfixForPipelineTasksParamsWithBrokenValue(pipeline *v1.Pipeline) {
	for it, task := range pipeline.Spec.Tasks {
		for ip, p := range task.Params {
			if p.Value.ObjectVal != nil && p.Value.ObjectVal["type"] == "string" && p.Value.ObjectVal["stringVal"] != "" {
				pipeline.Spec.Tasks[it].Params[ip].Value = v1.ParamValue{
					Type:      "string",
					StringVal: p.Value.ObjectVal["stringVal"],
				}
			}
		}
	}
}

func getTasks(pipelineFilePath string) (map[string]v1.Task, error) {
	pipelineFolder := filepath.Dir(pipelineFilePath)
	if _, err := os.Stat(pipelineFolder); os.IsNotExist(err) {
		return nil, fmt.Errorf("missing pipeline folder: %s", pipelineFolder)
	}

	fileNameList, err := filepath.Glob(filepath.Join(pipelineFolder, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("failed to scan pipeline folder %s: %v", pipelineFolder, err)
	}
	taskMap := make(map[string]v1.Task)
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

		task := v1.Task{}
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

func addGitDeployKeyVolume(task *v1.Task) {
	task.Spec.Volumes = append(task.Spec.Volumes, corev1.Volume{
		Name: pipelineDefaults.SubstitutionRadixGitDeployKeyTarget,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  defaults.GitPrivateKeySecretName,
				DefaultMode: &privateSshFolderMode,
			},
		},
	})
}

func taskIsValid(task *v1.Task) bool {
	return strings.HasPrefix(task.APIVersion, "tekton.dev/") && task.Kind == "Task" && len(task.ObjectMeta.Name) > 1
}

func ensureAzureClientIdParamExistInPipelineParams(pipeline *v1.Pipeline) {
	if !pipelineHasAzureIdentityClientIdParam(pipeline) {
		addAzureIdentityClientIdParamToPipeline(pipeline)
	}
}

func ensureAzureClientIdParamExistInTaskParams(pipeline *v1.Pipeline, pipelineTaskIndex int, task v1.Task) {
	if taskHasAzureIdentityClientIdParam(task) && !pipelineTaskHasAzureIdentityClientIdParam(pipeline, pipelineTaskIndex) {
		addAzureIdentityClientIdParamToPipelineTask(pipeline, pipelineTaskIndex)
	}
}

func addAzureIdentityClientIdParamToPipeline(pipeline *v1.Pipeline) {
	pipeline.Spec.Params = append(pipeline.Spec.Params, v1.ParamSpec{Name: pipelineDefaults.AzureClientIdEnvironmentVariable, Type: v1.ParamTypeString, Description: "Defines the Client ID for a user defined managed identity or application ID for an application registration"})
}

func pipelineHasAzureIdentityClientIdParam(pipeline *v1.Pipeline) bool {
	return slice.Any(pipeline.Spec.Params, func(paramSpec v1.ParamSpec) bool {
		return paramSpec.Name == pipelineDefaults.AzureClientIdEnvironmentVariable
	})
}

func addAzureIdentityClientIdParamToPipelineTask(pipeline *v1.Pipeline, taskIndex int) {
	pipeline.Spec.Tasks[taskIndex].Params = append(pipeline.Spec.Tasks[taskIndex].Params,
		v1.Param{
			Name:  pipelineDefaults.AzureClientIdEnvironmentVariable,
			Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: fmt.Sprintf("$(params.%s)", pipelineDefaults.AzureClientIdEnvironmentVariable)},
		})
}

func taskHasAzureIdentityClientIdParam(task v1.Task) bool {
	return slice.Any(task.Spec.Params, func(paramSpec v1.ParamSpec) bool {
		return paramSpec.Name == pipelineDefaults.AzureClientIdEnvironmentVariable
	})
}

func pipelineTaskHasAzureIdentityClientIdParam(pipeline *v1.Pipeline, taskIndex int) bool {
	return slice.Any(pipeline.Spec.Tasks[taskIndex].Params, func(param v1.Param) bool {
		return param.Name == pipelineDefaults.AzureClientIdEnvironmentVariable
	})
}

// GetEnvVars Gets build env vars
func (cli *PreparePipelinesStepImplementation) GetEnvVars(ra *radixv1.RadixApplication, envName string) radixv1.EnvVarsMap {
	envVarsMap := make(radixv1.EnvVarsMap)
	cli.setPipelineRunParamsFromBuild(ra, envVarsMap)
	cli.setPipelineRunParamsFromEnvironmentBuilds(ra, envName, envVarsMap)
	return envVarsMap
}

func (cli *PreparePipelinesStepImplementation) setPipelineRunParamsFromBuild(ra *radixv1.RadixApplication, envVarsMap radixv1.EnvVarsMap) {
	if ra.Spec.Build == nil {
		return
	}
	setBuildIdentity(envVarsMap, ra.Spec.Build.SubPipeline)
	setBuildVariables(envVarsMap, ra.Spec.Build.SubPipeline, ra.Spec.Build.Variables)
}

func setBuildVariables(envVarsMap radixv1.EnvVarsMap, subPipeline *radixv1.SubPipeline, variables radixv1.EnvVarsMap) {
	if subPipeline != nil {
		setVariablesToEnvVarsMap(envVarsMap, subPipeline.Variables) // sub-pipeline variables have higher priority over build variables
		return
	}
	setVariablesToEnvVarsMap(envVarsMap, variables) // keep for backward compatibility
}

func setVariablesToEnvVarsMap(envVarsMap radixv1.EnvVarsMap, variables radixv1.EnvVarsMap) {
	for name, envVar := range variables {
		envVarsMap[name] = envVar
	}
}

func setBuildIdentity(envVarsMap radixv1.EnvVarsMap, subPipeline *radixv1.SubPipeline) {
	if subPipeline != nil {
		setIdentityToEnvVarsMap(envVarsMap, subPipeline.Identity)
	}
}

func setIdentityToEnvVarsMap(envVarsMap radixv1.EnvVarsMap, identity *radixv1.Identity) {
	if identity == nil || identity.Azure == nil {
		return
	}
	if len(identity.Azure.ClientId) > 0 {
		envVarsMap[pipelineDefaults.AzureClientIdEnvironmentVariable] = identity.Azure.ClientId // if build env-var or build environment env-var have this variable explicitly set, it will override this identity set env-var
	} else {
		delete(envVarsMap, pipelineDefaults.AzureClientIdEnvironmentVariable)
	}
}

func (cli *PreparePipelinesStepImplementation) setPipelineRunParamsFromEnvironmentBuilds(ra *radixv1.RadixApplication, targetEnv string, envVarsMap radixv1.EnvVarsMap) {
	for _, buildEnv := range ra.Spec.Environments {
		if strings.EqualFold(buildEnv.Name, targetEnv) {
			setBuildIdentity(envVarsMap, buildEnv.SubPipeline)
			setBuildVariables(envVarsMap, buildEnv.SubPipeline, buildEnv.Build.Variables)
		}
	}
}

func getGitHash(pipelineInfo *model.PipelineInfo) (string, error) {
	// getGitHash return git commit to which the user repository should be reset before parsing sub-pipelines.
	pipelineArgs := pipelineInfo.PipelineArguments
	pipelineType := pipelineInfo.GetRadixPipelineType()
	if pipelineType == radixv1.ApplyConfig {
		return "", nil
	}

	if pipelineType == radixv1.Promote {
		sourceRdHashFromAnnotation := pipelineInfo.SourceDeploymentGitCommitHash
		sourceDeploymentGitBranch := pipelineInfo.SourceDeploymentGitBranch
		if sourceRdHashFromAnnotation != "" {
			return sourceRdHashFromAnnotation, nil
		}
		if sourceDeploymentGitBranch == "" {
			log.Info().Msg("source deployment has no git metadata, skipping sub-pipelines")
			return "", nil
		}
		sourceRdHashFromBranchHead, err := git.GetCommitHashFromHead(pipelineInfo.GetGitWorkspace(), sourceDeploymentGitBranch)
		if err != nil {
			return "", nil
		}
		return sourceRdHashFromBranchHead, nil
	}

	if pipelineType == radixv1.Deploy {
		pipelineJobBranch := ""
		re := applicationconfig.GetEnvironmentFromRadixApplication(pipelineInfo.GetRadixApplication(), pipelineArgs.ToEnvironment)
		if re != nil {
			pipelineJobBranch = re.Build.From
		}
		if pipelineJobBranch == "" {
			log.Info().Msg("deploy job with no build branch, skipping sub-pipelines.")
			return "", nil
		}
		if containsRegex(pipelineJobBranch) {
			log.Info().Msg("deploy job with build branch having regex pattern, skipping sub-pipelines.")
			return "", nil
		}
		gitHash, err := git.GetCommitHashFromHead(pipelineArgs.GitWorkspace, pipelineJobBranch)
		if err != nil {
			return "", err
		}
		return gitHash, nil
	}

	if pipelineType == radixv1.BuildDeploy || pipelineType == radixv1.Build {
		gitHash, err := git.GetCommitHash(pipelineArgs.GitWorkspace, pipelineArgs.CommitID, pipelineArgs.Branch)
		if err != nil {
			return "", err
		}
		return gitHash, nil
	}
	return "", fmt.Errorf("unknown pipeline type %s", pipelineType)
}

func containsRegex(value string) bool {
	if simpleSentence := regexp.MustCompile(`^[a-zA-Z0-9\s\.\-/]+$`); simpleSentence.MatchString(value) {
		return false
	}
	// Regex value that looks for typical regex special characters
	if specialRegexChars := regexp.MustCompile(`[\[\](){}.*+?^$\\|]`); specialRegexChars.FindStringIndex(value) != nil {
		_, err := regexp.Compile(value)
		return err == nil
	}
	return false
}

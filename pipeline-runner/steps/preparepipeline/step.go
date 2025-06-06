package preparepipeline

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	internalsubpipeline "github.com/equinor/radix-operator/pipeline-runner/internal/subpipeline"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	pipelineDefaults "github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/labels"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/ownerreferences"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/validation"
	prepareInternal "github.com/equinor/radix-operator/pipeline-runner/steps/preparepipeline/internal"
	"github.com/equinor/radix-operator/pipeline-runner/utils/annotations"
	"github.com/equinor/radix-operator/pipeline-runner/utils/git"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/rs/zerolog/log"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// PreparePipelinesStepImplementation Step to prepare radixconfig and Tekton pipelines
type PreparePipelinesStepImplementation struct {
	stepType pipeline.StepType
	model.DefaultStepImplementation
	contextBuilder        prepareInternal.ContextBuilder
	subPipelineReader     prepareInternal.SubPipelineReader
	ownerReferenceFactory ownerreferences.OwnerReferenceFactory
	radixConfigReader     prepareInternal.RadixConfigReader
	openGitRepo           func(path string) (git.Repository, error)
}
type Option func(step *PreparePipelinesStepImplementation)

func WithBuildContextBuilder(v prepareInternal.ContextBuilder) Option {
	return func(step *PreparePipelinesStepImplementation) {
		step.contextBuilder = v
	}
}

func WithSubPipelineReader(v prepareInternal.SubPipelineReader) Option {
	return func(step *PreparePipelinesStepImplementation) {
		step.subPipelineReader = v
	}
}

func WithOwnerReferenceFactory(v ownerreferences.OwnerReferenceFactory) Option {
	return func(step *PreparePipelinesStepImplementation) {
		step.ownerReferenceFactory = v
	}
}

func WithRadixConfigReader(v prepareInternal.RadixConfigReader) Option {
	return func(step *PreparePipelinesStepImplementation) {
		step.radixConfigReader = v
	}
}

func WithOpenGitRepoFunc(v func(path string) (git.Repository, error)) Option {
	return func(step *PreparePipelinesStepImplementation) {
		step.openGitRepo = v
	}
}

// NewPreparePipelinesStep Constructor.
func NewPreparePipelinesStep(opt ...Option) model.Step {
	implementation := PreparePipelinesStepImplementation{
		stepType:    pipeline.PreparePipelinesStep,
		openGitRepo: git.Open,
	}
	for _, option := range opt {
		option(&implementation)
	}
	return &implementation
}

func (step *PreparePipelinesStepImplementation) Init(ctx context.Context, kubeClient kubernetes.Interface, radixClient radixclient.Interface, kubeUtil *kube.Kube, prometheusOperatorClient monitoring.Interface, tektonClient tektonclient.Interface, rr *radixv1.RadixRegistration) {
	step.DefaultStepImplementation.Init(ctx, kubeClient, radixClient, kubeUtil, prometheusOperatorClient, tektonClient, rr)
	if step.contextBuilder == nil {
		step.contextBuilder = prepareInternal.NewContextBuilder()
	}
	if step.subPipelineReader == nil {
		step.subPipelineReader = prepareInternal.NewSubPipelineReader()
	}
	if step.ownerReferenceFactory == nil {
		step.ownerReferenceFactory = ownerreferences.NewOwnerReferenceFactory()
	}
	if step.radixConfigReader == nil {
		step.radixConfigReader = prepareInternal.NewRadixConfigReader(radixClient)
	}
}

// ImplementationForType Override of default step method
func (step *PreparePipelinesStepImplementation) ImplementationForType() pipeline.StepType {
	return step.stepType
}

// SucceededMsg Override of default step method
func (step *PreparePipelinesStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Succeded: prepare pipelines step for application %s", step.GetAppName())
}

// ErrorMsg Override of default step method
func (step *PreparePipelinesStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed prepare pipelines for application %s. Error: %v", step.GetAppName(), err)
}

// Run Override of default step method
func (step *PreparePipelinesStepImplementation) Run(ctx context.Context, pipelineInfo *model.PipelineInfo) error {
	var err error
	step.logPipelineInfo(ctx, pipelineInfo)

	repo, err := step.openGitRepo(pipelineInfo.GetGitWorkspace())
	if err != nil {
		return fmt.Errorf("failed to open git repository: %w", err)
	}

	if err := step.setRadixConfig(pipelineInfo, repo); err != nil {
		return err
	}

	if err = step.setTargetEnvironments(ctx, pipelineInfo); err != nil {
		return err
	}

	if slice.Any([]radixv1.RadixPipelineType{radixv1.Build, radixv1.BuildDeploy}, pipelineInfo.IsPipelineType) {
		if err = step.setBuildInfo(ctx, pipelineInfo, repo); err != nil {
			return err
		}
	}

	if err = step.setSubPipelinesToRun(ctx, pipelineInfo, repo); err != nil {
		return err
	}

	if pipelineInfo.IsPipelineType(radixv1.BuildDeploy) {
		pipelineInfo.StopPipeline, pipelineInfo.StopPipelineMessage = getBuildDeployPipelineShouldBeStopped(pipelineInfo)
	}

	return nil
}

func (step *PreparePipelinesStepImplementation) getGitInfoForBuild(pipelineInfo *model.PipelineInfo, repo git.Repository) (string, string, error) {
	var err error
	refName := pipelineInfo.PipelineArguments.GetGitRefOrDefault()

	commit := pipelineInfo.PipelineArguments.CommitID
	if len(commit) == 0 {
		commit, err = repo.ResolveCommitForReference(refName)
		if err != nil {
			return "", "", fmt.Errorf("failed to get latest commit for branch %s: %w", refName, err)
		}
	}

	isAncestor, err := repo.IsAncestor(commit, refName)
	if err != nil {
		return "", "", fmt.Errorf("failed to verify if commit %s is ancestor of branch %s: %w", commit, refName, err)
	}
	if !isAncestor {
		return "", "", fmt.Errorf("commit %s is not ancestor of branch %s", commit, refName)
	}

	tags, err := repo.ResolveTagsForCommit(commit)
	if err != nil {
		return "", "", err
	}
	tagsConcat := strings.Join(tags, " ")

	if err = radixvalidators.GitTagsContainIllegalChars(tagsConcat); err != nil {
		return "", "", err
	}

	return commit, tagsConcat, nil
}

func (step *PreparePipelinesStepImplementation) setBuildInfo(ctx context.Context, pipelineInfo *model.PipelineInfo, repo git.Repository) error {
	commit, tags, err := step.getGitInfoForBuild(pipelineInfo, repo)
	if err != nil {
		return err
	}
	pipelineInfo.GitCommitHash = commit
	pipelineInfo.GitTags = tags

	if len(pipelineInfo.PipelineArguments.CommitID) == 0 {
		return nil
	}

	buildContext, err := step.contextBuilder.GetBuildContext(ctx, pipelineInfo, repo)
	if err != nil {
		return err
	}

	pipelineInfo.BuildContext = buildContext
	return err
}

func (step *PreparePipelinesStepImplementation) setRadixConfig(pipelineInfo *model.PipelineInfo, repo git.Repository) error {
	configBranch := step.GetRegistration().Spec.ConfigBranch
	err := repo.Checkout(configBranch)
	if err != nil {
		return fmt.Errorf("failed to checkout config branch %s: %w", configBranch, err)
	}

	radixConfig, err := step.radixConfigReader.Read(pipelineInfo)
	if err != nil {
		return err
	}

	pipelineInfo.RadixApplication = radixConfig
	return nil
}

func (step *PreparePipelinesStepImplementation) setSubPipelinesToRun(ctx context.Context, pipelineInfo *model.PipelineInfo, repo git.Repository) error {
	gitCommit, err := step.getTargetGitCommitForSubPipelines(ctx, pipelineInfo, repo)
	if err != nil {
		return err
	}

	if len(gitCommit) == 0 {
		return nil
	}

	err = repo.Checkout(gitCommit)
	if err != nil {
		return fmt.Errorf("failed to checkout commit %s: %w", gitCommit, err)
	}

	var errs []error
	var environmentSubPipelinesToRun []model.EnvironmentSubPipelineToRun
	timestamp := time.Now().Format("20060102150405")

	for _, targetEnv := range pipelineInfo.TargetEnvironments {
		log.Ctx(ctx).Debug().Msgf("Create sub-pipeline for environment %s", targetEnv.Environment)
		runSubPipeline, pipelineFilePath, err := step.prepareSubPipelineForEnvironment(pipelineInfo, targetEnv.Environment, timestamp)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to prepare sub-pipeline for environment %s: %w", targetEnv.Environment, err))
		}

		if runSubPipeline {
			environmentSubPipelinesToRun = append(environmentSubPipelinesToRun, model.EnvironmentSubPipelineToRun{
				Environment:  targetEnv.Environment,
				PipelineFile: pipelineFilePath,
			})
		}
	}

	pipelineInfo.EnvironmentSubPipelinesToRun = environmentSubPipelinesToRun
	return errors.Join(errs...)
}

func (step *PreparePipelinesStepImplementation) getTargetGitCommitForSubPipelines(ctx context.Context, pipelineInfo *model.PipelineInfo, repo git.Repository) (string, error) {
	pipelineArgs := pipelineInfo.PipelineArguments
	pipelineType := pipelineInfo.GetRadixPipelineType()

	if pipelineType == radixv1.ApplyConfig {
		return "", nil
	}

	if pipelineType == radixv1.Promote {
		sourceRdHashFromAnnotation, sourceDeploymentGitBranch, err := step.getPromoteSourceDeploymentGitInfo(ctx, pipelineInfo.PipelineArguments.FromEnvironment, pipelineInfo.PipelineArguments.DeploymentName)
		if err != nil {
			return "", err
		}

		if sourceRdHashFromAnnotation != "" {
			return sourceRdHashFromAnnotation, nil
		}

		// TODO: Should we fail if sourcecommithash is empty instead of trying to resolve commit from source RD branch?
		if sourceDeploymentGitBranch == "" {
			log.Ctx(ctx).Info().Msg("Source deployment has no git metadata, skipping sub-pipelines")
			return "", nil
		}
		sourceRdHashFromBranchHead, err := repo.ResolveCommitForReference(sourceDeploymentGitBranch)
		if err != nil {
			return "", nil
		}
		return sourceRdHashFromBranchHead, nil
	}

	if pipelineType == radixv1.Deploy {
		pipelineJobBranch := step.GetRegistration().Spec.ConfigBranch

		if env, ok := pipelineInfo.GetRadixApplication().GetEnvironmentByName(pipelineArgs.ToEnvironment); ok && len(env.Build.From) > 0 {
			pipelineJobBranch = env.Build.From
		}

		if containsRegex(pipelineJobBranch) {
			log.Ctx(ctx).Info().Msg("Deploy job with build branch having regex pattern, skipping sub-pipelines.")
			return "", nil
		}

		gitHash, err := repo.ResolveCommitForReference(pipelineJobBranch)
		if err != nil {
			return "", err
		}
		return gitHash, nil
	}

	if pipelineType == radixv1.BuildDeploy || pipelineType == radixv1.Build {
		return pipelineInfo.GitCommitHash, nil
	}

	return "", fmt.Errorf("unknown pipeline type %s", pipelineType)
}

func (step *PreparePipelinesStepImplementation) getPromoteSourceDeploymentGitInfo(ctx context.Context, sourceEnvName, sourceDeploymentName string) (string, string, error) {
	ns := utils.GetEnvironmentNamespace(step.GetAppName(), sourceEnvName)
	rd, err := step.GetRadixClient().RadixV1().RadixDeployments(ns).Get(ctx, sourceDeploymentName, metav1.GetOptions{}) //step.GetKubeUtil().GetRadixDeployment(ctx, ns, sourceDeploymentName)
	if err != nil {
		return "", "", err
	}
	gitHash := internal.GetGitCommitHashFromDeployment(rd)
	gitBranch := internal.GetGitRefNameFromDeployment(rd)
	return gitHash, gitBranch, err
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

func getBuildDeployPipelineShouldBeStopped(pipelineInfo *model.PipelineInfo) (bool, string) {
	isRadixConfigChangedForAnyEnvironments := slice.Any(pipelineInfo.TargetEnvironments, func(t model.TargetEnvironment) bool {
		isEqual, err := t.CompareApplicationWithDeploymentHash(pipelineInfo.RadixApplication)
		if err != nil {
			return true
		}
		return !isEqual
	})

	if pipelineInfo.BuildContext == nil || isRadixConfigChangedForAnyEnvironments ||
		len(pipelineInfo.BuildContext.EnvironmentsToBuild) == 0 ||
		len(pipelineInfo.EnvironmentSubPipelinesToRun) > 0 {
		return false, ""
	}

	for _, environmentToBuild := range pipelineInfo.BuildContext.EnvironmentsToBuild {
		if len(environmentToBuild.Components) > 0 {
			return false, ""
		}
	}

	return true, "No components with changed source code and the Radix config file was not changed. The pipeline will not proceed."
}

func (step *PreparePipelinesStepImplementation) logPipelineInfo(ctx context.Context, pipelineInfo *model.PipelineInfo) {
	stringBuilder := strings.Builder{}
	stringBuilder.WriteString(fmt.Sprintf("Prepare pipeline %s for the app %s", pipelineInfo.Definition.Type, step.GetAppName()))
	if len(pipelineInfo.GetGitRefOrDefault()) > 0 {
		stringBuilder.WriteString(fmt.Sprintf(", the %s %s", pipelineInfo.GetGitRefTypeOrDefault(), pipelineInfo.GetGitRefOrDefault()))
	}
	if len(pipelineInfo.PipelineArguments.CommitID) > 0 {
		stringBuilder.WriteString(fmt.Sprintf(", the commit %s", pipelineInfo.PipelineArguments.CommitID))
	}
	log.Ctx(ctx).Info().Msg(stringBuilder.String())
}

func (step *PreparePipelinesStepImplementation) prepareSubPipelineForEnvironment(pipelineInfo *model.PipelineInfo, envName, timestamp string) (bool, string, error) {
	subPipelineExists, pipelineFilePath, pl, tasks, err := step.subPipelineReader.ReadPipelineAndTasks(pipelineInfo, envName)
	if err != nil {
		return false, "", err
	}
	if !subPipelineExists {
		return false, "", nil
	}
	if err = step.createSubPipelineAndTasks(envName, pl, tasks, timestamp, pipelineInfo); err != nil {
		return false, "", err
	}
	return true, pipelineFilePath, nil
}

func (step *PreparePipelinesStepImplementation) buildSubPipelineTasks(envName string, tasks []v1.Task, timestamp string, pipelineInfo *model.PipelineInfo) (map[string]v1.Task, error) {
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

		if ownerReference := step.ownerReferenceFactory.Create(); ownerReference != nil {
			task.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*ownerReference}
		}

		ensureCorrectSecureContext(&task)
		taskMap[originalTaskName] = task
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

func (step *PreparePipelinesStepImplementation) createSubPipelineAndTasks(envName string, pipeline *v1.Pipeline, tasks []v1.Task, timestamp string, pipelineInfo *model.PipelineInfo) error {
	originalPipelineName := pipeline.Name
	var errs []error
	taskMap, err := step.buildSubPipelineTasks(envName, tasks, timestamp, pipelineInfo)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to build task for pipeline %s: %w", originalPipelineName, err))
	}

	_, azureClientIdPipelineParamExist := internalsubpipeline.GetEnvVars(pipelineInfo.GetRadixApplication(), envName)[pipelineDefaults.AzureClientIdEnvironmentVariable]
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
		kube.RadixBranchAnnotation:      pipelineInfo.PipelineArguments.Branch, // nolint:staticcheck
		kube.RadixGitRefAnnotation:      pipelineInfo.PipelineArguments.GitRef,
		kube.RadixGitRefTypeAnnotation:  pipelineInfo.PipelineArguments.GitRefType,
		defaults.PipelineNameAnnotation: originalPipelineName,
	}
	if ownerReference := step.ownerReferenceFactory.Create(); ownerReference != nil {
		pipeline.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*ownerReference}
	}
	err = step.createSubPipelineTasks(taskMap)
	if err != nil {
		return fmt.Errorf("tasks have not been created. Error: %w", err)
	}
	log.Info().Msgf("Created %d task(s) for environment %s", len(taskMap), envName)

	_, err = step.GetTektonClient().TektonV1().Pipelines(utils.GetAppNamespace(pipelineInfo.GetAppName())).Create(context.Background(), pipeline, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("pipeline %s has not been created. Error: %w", pipeline.Name, err)
	}
	log.Info().Msgf("Created pipeline %s for environment %s", pipeline.Name, envName)
	return nil
}

func (step *PreparePipelinesStepImplementation) createSubPipelineTasks(taskMap map[string]v1.Task) error {
	namespace := utils.GetAppNamespace(step.GetAppName())
	var errs []error
	for _, task := range taskMap {
		_, err := step.GetTektonClient().TektonV1().Tasks(namespace).Create(context.Background(), &task,
			metav1.CreateOptions{})
		if err != nil {
			errs = append(errs, fmt.Errorf("task %s has not been created. Error: %w", task.Name, err))
		}
	}
	return errors.Join(errs...)
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

func (step *PreparePipelinesStepImplementation) setTargetEnvironments(ctx context.Context, pipelineInfo *model.PipelineInfo) error {
	log.Ctx(ctx).Debug().Msg("Set target environments")
	targetEnvironmentNames, ignoredForWebhookEnvs, ignoredForGitRefsType, err := getTargetEnvironmentNames(ctx, pipelineInfo)
	if err != nil {
		return err
	}

	targetEnvironments := make([]model.TargetEnvironment, 0, len(targetEnvironmentNames))
	for _, targetEnvName := range targetEnvironmentNames {
		activeRD, err := internal.GetActiveRadixDeployment(ctx, step.GetKubeUtil(), utils.GetEnvironmentNamespace(pipelineInfo.GetAppName(), targetEnvName))
		if err != nil {
			return fmt.Errorf("failed to get active depoyment for environment %s: %w", targetEnvName, err)
		}
		targetEnvironments = append(targetEnvironments, model.TargetEnvironment{Environment: targetEnvName, ActiveRadixDeployment: activeRD})
	}
	pipelineInfo.TargetEnvironments = targetEnvironments

	if len(pipelineInfo.TargetEnvironments) > 0 {
		log.Ctx(ctx).Info().Msgf("Environment(s) %v are mapped to %s %s.", strings.Join(targetEnvironmentNames, ", "), pipelineInfo.GetGitRefTypeOrDefault(), pipelineInfo.GetGitRef())
	} else {
		log.Ctx(ctx).Info().Msgf("No environments are mapped to %s %s.", pipelineInfo.GetGitRefTypeOrDefault(), pipelineInfo.GetGitRef())
	}
	if len(ignoredForWebhookEnvs) > 0 || len(ignoredForGitRefsType) > 0 {
		log.Ctx(ctx).Info().Msg("The following environment(s) are configured to be ignored when triggered from GitHub webhook:")
		if len(ignoredForWebhookEnvs) > 0 {
			log.Ctx(ctx).Info().Msgf(" - %s", strings.Join(ignoredForWebhookEnvs, ", "))
		}
		if len(ignoredForGitRefsType) > 0 {
			log.Ctx(ctx).Info().Msgf(" - for %s: %s", pipelineInfo.GetGitRefTypeOrDefault(), strings.Join(ignoredForGitRefsType, ", "))
		}
	}
	return nil
}

func getTargetEnvironmentNames(ctx context.Context, pipelineInfo *model.PipelineInfo) ([]string, []string, []string, error) {
	switch pipelineInfo.GetRadixPipelineType() {
	case radixv1.ApplyConfig:
		return nil, nil, nil, nil
	case radixv1.Promote:
		environmentsForPromote, err := getTargetEnvironmentsForPromote(pipelineInfo)
		return environmentsForPromote, nil, nil, err
	case radixv1.Deploy:
		environmentsForDeploy, err := getTargetEnvironmentsForDeploy(ctx, pipelineInfo)
		return environmentsForDeploy, nil, nil, err
	}

	deployToEnvironment := pipelineInfo.GetRadixDeployToEnvironment()
	targetEnvironments, ignoredForWebhookEnvs, ignoredForGitRefsType := applicationconfig.GetTargetEnvironments(pipelineInfo.GetGitRefOrDefault(), pipelineInfo.GetGitRefType(), pipelineInfo.GetRadixApplication(), pipelineInfo.PipelineArguments.TriggeredFromWebhook)
	applicableTargetEnvironments := slice.FindAll(targetEnvironments, func(envName string) bool { return len(deployToEnvironment) == 0 || deployToEnvironment == envName })
	return applicableTargetEnvironments, ignoredForWebhookEnvs, ignoredForGitRefsType, nil
}

func getTargetEnvironmentsForPromote(pipelineInfo *model.PipelineInfo) ([]string, error) {
	var errs []error
	if len(pipelineInfo.GetRadixPromoteDeployment()) == 0 {
		errs = append(errs, fmt.Errorf("missing promote deployment name"))
	}
	if len(pipelineInfo.GetRadixPromoteFromEnvironment()) == 0 {
		errs = append(errs, fmt.Errorf("missing promote source environment name"))
	}
	if len(pipelineInfo.GetRadixDeployToEnvironment()) == 0 {
		errs = append(errs, fmt.Errorf("missing promote target environment name"))
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return []string{pipelineInfo.GetRadixDeployToEnvironment()}, nil // run Tekton pipelines for the promote target environment
}

func getTargetEnvironmentsForDeploy(ctx context.Context, pipelineInfo *model.PipelineInfo) ([]string, error) {
	targetEnvironment := pipelineInfo.GetRadixDeployToEnvironment()
	if len(targetEnvironment) == 0 {
		return nil, fmt.Errorf("no target environment is specified for the deploy pipeline")
	}
	log.Ctx(ctx).Info().Msgf("Target environment: %v", targetEnvironment)
	return []string{targetEnvironment}, nil
}

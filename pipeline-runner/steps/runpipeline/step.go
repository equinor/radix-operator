package runpipeline

import (
	"context"
	"errors"
	"fmt"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/labels"
	"github.com/equinor/radix-operator/pipeline-runner/utils/owner_references"
	"github.com/equinor/radix-operator/pipeline-runner/utils/radix/applicationconfig"
	defaults2 "github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	v2 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	v3 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"time"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/wait"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/rs/zerolog/log"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

// RunPipelinesStepImplementation Step to run Tekton pipelines
type RunPipelinesStepImplementation struct {
	stepType pipeline.StepType
	model.DefaultStepImplementation
	waiter wait.PipelineRunsCompletionWaiter
}

// NewRunPipelinesStep Constructor.
// jobWaiter is optional and will be set by Init(...) function if nil.
func NewRunPipelinesStep(options ...RunPipelinesStepImplementationOption) model.Step {
	step := &RunPipelinesStepImplementation{
		stepType: pipeline.RunPipelinesStep,
	}
	for _, option := range options {
		option(step)
	}
	return step
}

func (step *RunPipelinesStepImplementation) Init(ctx context.Context, kubeClient kubernetes.Interface, radixClient radixclient.Interface, kubeUtil *kube.Kube, prometheusOperatorClient monitoring.Interface, tektonClient tektonclient.Interface, rr *radixv1.RadixRegistration) {
	step.DefaultStepImplementation.Init(ctx, kubeClient, radixClient, kubeUtil, prometheusOperatorClient, tektonClient, rr)
	if step.waiter == nil {
		step.waiter = wait.NewPipelineRunsCompletionWaiter(tektonClient)
	}
}

// ImplementationForType Override of default step method
func (step *RunPipelinesStepImplementation) ImplementationForType() pipeline.StepType {
	return step.stepType
}

// SucceededMsg Override of default step method
func (step *RunPipelinesStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Succeded: run pipelines step for application %s", step.GetAppName())
}

// ErrorMsg Override of default step method
func (step *RunPipelinesStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed run pipelines for the application %s. Error: %v", step.GetAppName(), err)
}

// Run Override of default step method
func (step *RunPipelinesStepImplementation) Run(ctx context.Context, pipelineInfo *model.PipelineInfo) error {
	if pipelineInfo.BuildContext != nil && len(pipelineInfo.BuildContext.EnvironmentSubPipelinesToRun) == 0 {
		log.Ctx(ctx).Info().Msg("There is no configured sub-pipelines. Skip the step.")
		return nil
	}
	targetEnvironments, ignoredForWebhookEnvs, err := internal.GetPipelineTargetEnvironments(ctx, pipelineInfo)
	if err != nil {
		return err
	}
	if len(ignoredForWebhookEnvs) > 0 {
		log.Ctx(ctx).Info().Msgf("Run sub-pipelines for following environment(s) are ignored to be triggered by the webhook: %s", strings.Join(ignoredForWebhookEnvs, ", "))
	}
	branch := pipelineInfo.PipelineArguments.Branch
	commitID := pipelineInfo.GitCommitHash
	appName := step.GetAppName()
	log.Ctx(ctx).Info().Msgf("Run pipelines app %s for branch %s and commit %s", appName, branch, commitID)
	return step.RunPipelinesJob(pipelineInfo, targetEnvironments)
}

// GetEnvVars Gets build env vars
func (step *RunPipelinesStepImplementation) GetEnvVars(pipelineInfo *model.PipelineInfo, envName string) radixv1.EnvVarsMap {
	envVarsMap := make(radixv1.EnvVarsMap)
	step.setPipelineRunParamsFromBuild(pipelineInfo, envVarsMap)
	step.setPipelineRunParamsFromEnvironmentBuilds(pipelineInfo, envName, envVarsMap)
	return envVarsMap
}

func (step *RunPipelinesStepImplementation) setPipelineRunParamsFromBuild(pipelineInfo *model.PipelineInfo, envVarsMap radixv1.EnvVarsMap) {
	ra := pipelineInfo.GetRadixApplication()
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
		envVarsMap[defaults.AzureClientIdEnvironmentVariable] = identity.Azure.ClientId // if build env-var or build environment env-var have this variable explicitly set, it will override this identity set env-var
	} else {
		delete(envVarsMap, defaults.AzureClientIdEnvironmentVariable)
	}
}

func (step *RunPipelinesStepImplementation) setPipelineRunParamsFromEnvironmentBuilds(pipelineInfo *model.PipelineInfo, targetEnv string, envVarsMap radixv1.EnvVarsMap) {
	for _, buildEnv := range pipelineInfo.GetRadixApplication().Spec.Environments {
		if strings.EqualFold(buildEnv.Name, targetEnv) {
			setBuildIdentity(envVarsMap, buildEnv.SubPipeline)
			setBuildVariables(envVarsMap, buildEnv.SubPipeline, buildEnv.Build.Variables)
		}
	}
}

type RunPipelinesStepImplementationOption func(step *RunPipelinesStepImplementation)

// WithPipelineRunsWaiter Set pipeline runs waiter
func WithPipelineRunsWaiter(waiter wait.PipelineRunsCompletionWaiter) RunPipelinesStepImplementationOption {
	return func(step *RunPipelinesStepImplementation) {
		step.waiter = waiter
	}
}

// RunPipelinesJob Run the job, which creates Tekton PipelineRun-s for each preliminary prepared pipelines of the specified branch
func (step *RunPipelinesStepImplementation) RunPipelinesJob(pipelineInfo *model.PipelineInfo, targetEnvironments []string) error {
	if pipelineInfo.GetRadixPipelineType() == radixv1.Build {
		log.Info().Msg("Pipeline type is build, skip Tekton pipeline run.")
		return nil
	}
	namespace := utils.GetAppNamespace(step.GetAppName())
	labelSelector := fmt.Sprintf("%s=%s", kube.RadixJobNameLabel, pipelineInfo.GetRadixPipelineJobName())
	pipelineList, err := step.GetTektonClient().TektonV1().Pipelines(namespace).List(context.Background(), v1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return err
	}
	if len(pipelineList.Items) == 0 {
		log.Info().Msg("no pipelines exist, skip Tekton pipeline run.")
		return nil
	}

	tektonPipelineBranch := pipelineInfo.GetBranch()
	if pipelineInfo.GetRadixPipelineType() == radixv1.Deploy {
		re := applicationconfig.GetEnvironmentFromRadixApplication(pipelineInfo.GetRadixApplication(), pipelineInfo.GetRadixDeployToEnvironment())
		if re != nil && len(re.Build.From) > 0 {
			tektonPipelineBranch = re.Build.From
		} else {
			tektonPipelineBranch = pipelineInfo.GetRadixConfigBranch() // if the branch for the deploy-toEnvironment is not defined - fallback to the config branch
		}
	}
	log.Info().Msgf("Run tekton pipelines for the branch %s", tektonPipelineBranch)

	pipelineRunMap, err := step.runPipelines(pipelineList.Items, namespace, pipelineInfo, targetEnvironments)

	if err != nil {
		return fmt.Errorf("failed to run pipelines: %w", err)
	}

	if err = step.waiter.Wait(pipelineRunMap, pipelineInfo); err != nil {
		return fmt.Errorf("failed tekton pipelines for the application %s, for environment(s) %s. %w",
			pipelineInfo.GetAppName(),
			strings.Join(targetEnvironments, ","),
			err)
	}
	return nil
}

func (step *RunPipelinesStepImplementation) runPipelines(pipelines []v2.Pipeline, namespace string, pipelineInfo *model.PipelineInfo, targetEnvironments []string) (map[string]*v2.PipelineRun, error) {
	timestamp := time.Now().Format("20060102150405")
	pipelineRunMap := make(map[string]*v2.PipelineRun)
	var errs []error
	for _, pipeline := range pipelines {
		createdPipelineRun, err := step.createPipelineRun(namespace, &pipeline, timestamp, pipelineInfo, targetEnvironments)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		pipelineRunMap[createdPipelineRun.GetName()] = createdPipelineRun
	}
	return pipelineRunMap, errors.Join(errs...)
}

func (step *RunPipelinesStepImplementation) createPipelineRun(namespace string, pipeline *v2.Pipeline, timestamp string, pipelineInfo *model.PipelineInfo, targetEnvironments []string) (*v2.PipelineRun, error) {
	targetEnv, pipelineTargetEnvDefined := pipeline.ObjectMeta.Labels[kube.RadixEnvLabel]
	if !pipelineTargetEnvDefined {
		return nil, fmt.Errorf("missing target environment in labels of the pipeline %s", pipeline.Name)
	}

	log.Debug().Msgf("run pipelinerun for the target environment %s", targetEnv)
	if !slice.Any(targetEnvironments, func(envName string) bool { return envName == targetEnv }) {
		return nil, fmt.Errorf("missing target environment %s for the pipeline %s", targetEnv, pipeline.Name)
	}

	pipelineRun := step.buildPipelineRun(pipeline, targetEnv, timestamp, pipelineInfo)

	return step.GetTektonClient().TektonV1().PipelineRuns(namespace).Create(context.Background(), &pipelineRun, v1.CreateOptions{})
}

func (step *RunPipelinesStepImplementation) buildPipelineRun(pipeline *v2.Pipeline, targetEnv, timestamp string, pipelineInfo *model.PipelineInfo) v2.PipelineRun {
	originalPipelineName := pipeline.ObjectMeta.Annotations[defaults2.PipelineNameAnnotation]
	pipelineRunName := fmt.Sprintf("radix-pipelinerun-%s-%s-%s", internal.GetShortName(targetEnv), timestamp, internal.GetJobNameHash(pipelineInfo))
	pipelineParams := step.getPipelineParams(pipeline, targetEnv, pipelineInfo)
	pipelineRun := v2.PipelineRun{
		ObjectMeta: v1.ObjectMeta{
			Name:   pipelineRunName,
			Labels: labels.GetSubPipelineLabelsForEnvironment(pipelineInfo, targetEnv),
			Annotations: map[string]string{
				kube.RadixBranchAnnotation:       pipelineInfo.PipelineArguments.Branch,
				defaults2.PipelineNameAnnotation: originalPipelineName,
			},
		},
		Spec: v2.PipelineRunSpec{
			PipelineRef: &v2.PipelineRef{Name: pipeline.GetName()},
			Params:      pipelineParams,
			TaskRunTemplate: v2.PipelineTaskRunTemplate{
				PodTemplate:        step.buildPipelineRunPodTemplate(pipelineInfo),
				ServiceAccountName: utils.GetSubPipelineServiceAccountName(targetEnv),
			},
		},
	}
	ownerReference := ownerreferences.GetOwnerReferenceOfJobFromLabels()
	if ownerReference != nil {
		pipelineRun.ObjectMeta.OwnerReferences = []v1.OwnerReference{*ownerReference}
	}
	var taskRunSpecs []v2.PipelineTaskRunSpec
	for _, task := range pipeline.Spec.Tasks {
		taskRunSpecs = append(taskRunSpecs, pipelineRun.GetTaskRunSpec(task.Name))
	}
	pipelineRun.Spec.TaskRunSpecs = taskRunSpecs
	return pipelineRun
}

func (step *RunPipelinesStepImplementation) buildPipelineRunPodTemplate(pipelineInfo *model.PipelineInfo) *pod.Template {
	podTemplate := pod.Template{
		SecurityContext: &v3.PodSecurityContext{
			RunAsNonRoot: pointers.Ptr(true),
		},
		NodeSelector: map[string]string{
			v3.LabelArchStable: "amd64",
			v3.LabelOSStable:   "linux",
		},
	}

	ra := pipelineInfo.GetRadixApplication()
	if ra != nil && len(ra.Spec.PrivateImageHubs) > 0 {
		podTemplate.ImagePullSecrets = []v3.LocalObjectReference{{Name: defaults2.PrivateImageHubSecretName}}
	}

	return &podTemplate
}

func (step *RunPipelinesStepImplementation) getPipelineParams(pipeline *v2.Pipeline, targetEnv string, pipelineInfo *model.PipelineInfo) []v2.Param {
	envVars := step.GetEnvVars(pipelineInfo, targetEnv)
	pipelineParamsMap := getPipelineParamSpecsMap(pipeline)
	var pipelineParams []v2.Param
	for envVarName, envVarValue := range envVars {
		paramSpec, envVarExistInParamSpecs := getPipelineParamSpec(pipelineParamsMap, envVarName)
		if !envVarExistInParamSpecs {
			continue // Add to pipelineRun params only env-vars, existing in the pipeline paramSpecs or Azure identity clientId
		}
		param := v2.Param{Name: envVarName, Value: v2.ParamValue{Type: paramSpec.Type}}
		if param.Value.Type == v2.ParamTypeArray { // Param can contain a string value or a comma-separated values array
			param.Value.ArrayVal = strings.Split(envVarValue, ",")
		} else {
			param.Value.StringVal = envVarValue
		}
		pipelineParams = append(pipelineParams, param)
		delete(pipelineParamsMap, envVarName)
	}
	for paramName, paramSpec := range pipelineParamsMap {
		if paramName == defaults.AzureClientIdEnvironmentVariable && len(envVars[defaults.AzureClientIdEnvironmentVariable]) > 0 {
			continue // Azure identity clientId was set by radixconfig build env-var or identity
		}
		param := v2.Param{Name: paramName, Value: v2.ParamValue{Type: paramSpec.Type}}
		if paramSpec.Default != nil {
			param.Value.StringVal = paramSpec.Default.StringVal
			param.Value.ArrayVal = paramSpec.Default.ArrayVal
			param.Value.ObjectVal = paramSpec.Default.ObjectVal
		}
		pipelineParams = append(pipelineParams, param)
	}
	return pipelineParams
}

func getPipelineParamSpec(pipelineParamsMap map[string]v2.ParamSpec, envVarName string) (v2.ParamSpec, bool) {
	if envVarName == defaults.AzureClientIdEnvironmentVariable {
		return v2.ParamSpec{Name: envVarName, Type: v2.ParamTypeString}, true
	}
	paramSpec, ok := pipelineParamsMap[envVarName]
	return paramSpec, ok
}

func getPipelineParamSpecsMap(pipeline *v2.Pipeline) map[string]v2.ParamSpec {
	paramSpecMap := make(map[string]v2.ParamSpec)
	for _, paramSpec := range pipeline.PipelineSpec().Params {
		paramSpecMap[paramSpec.Name] = paramSpec
	}
	return paramSpecMap
}

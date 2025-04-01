package runpipeline

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/labels"
	"github.com/equinor/radix-operator/pipeline-runner/utils/radix/applicationconfig"
	operatorDefaults "github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog/log"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RunPipelinesJob Run the job, which creates Tekton PipelineRun-s for each preliminary prepared pipelines of the specified branch
func (pipelineCtx *pipelineContext) RunPipelinesJob() error {
	pipelineInfo := pipelineCtx.pipelineInfo
	if pipelineInfo.GetRadixPipelineType() == radixv1.Build {
		log.Info().Msg("Pipeline type is build, skip Tekton pipeline run.")
		return nil
	}
	namespace := pipelineInfo.GetAppNamespace()
	pipelineList, err := pipelineCtx.tektonClient.TektonV1().Pipelines(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", kube.RadixJobNameLabel, pipelineInfo.GetRadixPipelineJobName()),
	})
	if err != nil {
		return err
	}
	if len(pipelineList.Items) == 0 {
		log.Info().Msg("no pipelines exist, skip Tekton pipeline run.")
		return nil
	}

	pipelineCtx.targetEnvironments, err = internal.GetPipelineTargetEnvironments(pipelineInfo)
	if err != nil {
		return err
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

	pipelineRunMap, err := pipelineCtx.runPipelines(pipelineList.Items, namespace)

	if err != nil {
		return fmt.Errorf("failed to run pipelines: %w", err)
	}

	if err = pipelineCtx.waiter.Wait(pipelineRunMap, pipelineInfo); err != nil {
		return fmt.Errorf("failed tekton pipelines for the application %s, for environment(s) %s. %w",
			pipelineInfo.GetAppName(),
			pipelineCtx.getTargetEnvsAsString(),
			err)
	}
	return nil
}

func (pipelineCtx *pipelineContext) getTargetEnvsAsString() string {
	return strings.Join(pipelineCtx.targetEnvironments, ", ")
}

func (pipelineCtx *pipelineContext) runPipelines(pipelines []pipelinev1.Pipeline, namespace string) (map[string]*pipelinev1.PipelineRun, error) {
	timestamp := time.Now().Format("20060102150405")
	pipelineRunMap := make(map[string]*pipelinev1.PipelineRun)
	var errs []error
	for _, pipeline := range pipelines {
		createdPipelineRun, err := pipelineCtx.createPipelineRun(namespace, &pipeline, timestamp)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		pipelineRunMap[createdPipelineRun.GetName()] = createdPipelineRun
	}
	return pipelineRunMap, errors.Join(errs...)
}

func (pipelineCtx *pipelineContext) createPipelineRun(namespace string, pipeline *pipelinev1.Pipeline, timestamp string) (*pipelinev1.PipelineRun, error) {
	targetEnv, pipelineTargetEnvDefined := pipeline.ObjectMeta.Labels[kube.RadixEnvLabel]
	if !pipelineTargetEnvDefined {
		return nil, fmt.Errorf("missing target environment in labels of the pipeline %s", pipeline.Name)
	}

	log.Debug().Msgf("run pipelinerun for the target environment %s", targetEnv)
	if !slice.Any(pipelineCtx.targetEnvironments, func(envName string) bool { return envName == targetEnv }) {
		return nil, fmt.Errorf("missing target environment %s for the pipeline %s", targetEnv, pipeline.Name)
	}

	pipelineRun := pipelineCtx.buildPipelineRun(pipeline, targetEnv, timestamp)

	return pipelineCtx.tektonClient.TektonV1().PipelineRuns(namespace).Create(context.Background(), &pipelineRun, metav1.CreateOptions{})
}

func (pipelineCtx *pipelineContext) buildPipelineRun(pipeline *pipelinev1.Pipeline, targetEnv, timestamp string) pipelinev1.PipelineRun {
	originalPipelineName := pipeline.ObjectMeta.Annotations[operatorDefaults.PipelineNameAnnotation]
	pipelineRunName := fmt.Sprintf("radix-pipelinerun-%s-%s-%s", internal.GetShortName(targetEnv), timestamp, pipelineCtx.hash)
	pipelineParams := pipelineCtx.getPipelineParams(pipeline, targetEnv)
	pipelineInfo := pipelineCtx.pipelineInfo
	pipelineRun := pipelinev1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:   pipelineRunName,
			Labels: labels.GetSubPipelineLabelsForEnvironment(pipelineInfo, targetEnv),
			Annotations: map[string]string{
				kube.RadixBranchAnnotation:              pipelineCtx.pipelineInfo.PipelineArguments.Branch,
				operatorDefaults.PipelineNameAnnotation: originalPipelineName,
			},
		},
		Spec: pipelinev1.PipelineRunSpec{
			PipelineRef: &pipelinev1.PipelineRef{Name: pipeline.GetName()},
			Params:      pipelineParams,
			TaskRunTemplate: pipelinev1.PipelineTaskRunTemplate{
				PodTemplate:        pipelineCtx.buildPipelineRunPodTemplate(),
				ServiceAccountName: utils.GetSubPipelineServiceAccountName(targetEnv),
			},
		},
	}
	if pipelineCtx.ownerReference != nil {
		pipelineRun.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*pipelineCtx.ownerReference}
	}
	var taskRunSpecs []pipelinev1.PipelineTaskRunSpec
	for _, task := range pipeline.Spec.Tasks {
		taskRunSpecs = append(taskRunSpecs, pipelineRun.GetTaskRunSpec(task.Name))
	}
	pipelineRun.Spec.TaskRunSpecs = taskRunSpecs
	return pipelineRun
}

func (pipelineCtx *pipelineContext) buildPipelineRunPodTemplate() *pod.Template {
	podTemplate := pod.Template{
		SecurityContext: &corev1.PodSecurityContext{
			RunAsNonRoot: pointers.Ptr(true),
		},
		NodeSelector: map[string]string{
			corev1.LabelArchStable: "amd64",
			corev1.LabelOSStable:   "linux",
		},
	}

	ra := pipelineCtx.pipelineInfo.GetRadixApplication()
	if ra != nil && len(ra.Spec.PrivateImageHubs) > 0 {
		podTemplate.ImagePullSecrets = []corev1.LocalObjectReference{{Name: operatorDefaults.PrivateImageHubSecretName}}
	}

	return &podTemplate
}

func (pipelineCtx *pipelineContext) getPipelineParams(pipeline *pipelinev1.Pipeline, targetEnv string) []pipelinev1.Param {
	envVars := pipelineCtx.GetEnvVars(targetEnv)
	pipelineParamsMap := getPipelineParamSpecsMap(pipeline)
	var pipelineParams []pipelinev1.Param
	for envVarName, envVarValue := range envVars {
		paramSpec, envVarExistInParamSpecs := getPipelineParamSpec(pipelineParamsMap, envVarName)
		if !envVarExistInParamSpecs {
			continue // Add to pipelineRun params only env-vars, existing in the pipeline paramSpecs or Azure identity clientId
		}
		param := pipelinev1.Param{Name: envVarName, Value: pipelinev1.ParamValue{Type: paramSpec.Type}}
		if param.Value.Type == pipelinev1.ParamTypeArray { // Param can contain a string value or a comma-separated values array
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
		param := pipelinev1.Param{Name: paramName, Value: pipelinev1.ParamValue{Type: paramSpec.Type}}
		if paramSpec.Default != nil {
			param.Value.StringVal = paramSpec.Default.StringVal
			param.Value.ArrayVal = paramSpec.Default.ArrayVal
			param.Value.ObjectVal = paramSpec.Default.ObjectVal
		}
		pipelineParams = append(pipelineParams, param)
	}
	return pipelineParams
}

func getPipelineParamSpec(pipelineParamsMap map[string]pipelinev1.ParamSpec, envVarName string) (pipelinev1.ParamSpec, bool) {
	if envVarName == defaults.AzureClientIdEnvironmentVariable {
		return pipelinev1.ParamSpec{Name: envVarName, Type: pipelinev1.ParamTypeString}, true
	}
	paramSpec, ok := pipelineParamsMap[envVarName]
	return paramSpec, ok
}

func getPipelineParamSpecsMap(pipeline *pipelinev1.Pipeline) map[string]pipelinev1.ParamSpec {
	paramSpecMap := make(map[string]pipelinev1.ParamSpec)
	for _, paramSpec := range pipeline.PipelineSpec().Params {
		paramSpecMap[paramSpec.Name] = paramSpec
	}
	return paramSpecMap
}

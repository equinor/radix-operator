package internal

import (
	"context"
	"strconv"
	"strings"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PreservingDeployComponents DeployComponents, not to be updated during deployment, but transferred from an active deployment
type PreservingDeployComponents struct {
	DeployComponents    []radixv1.RadixDeployComponent
	DeployJobComponents []radixv1.RadixDeployJobComponent
}

// ConstructForTargetEnvironment Will build a deployment for target environment
func ConstructForTargetEnvironment(ctx context.Context, pipelineInfo *model.PipelineInfo, activeRadixDeployment *radixv1.RadixDeployment, commitID, gitTags, envName, radixConfigHash, buildSecretHash string) (*radixv1.RadixDeployment, error) {
	preservingDeployComponents := getPreservingDeployComponents(ctx, activeRadixDeployment, envName, pipelineInfo.BuildContext, pipelineInfo.PipelineArguments.ComponentsToDeploy)

	defaultEnvVars := radixv1.EnvVarsMap{
		defaults.RadixCommitHashEnvironmentVariable: commitID,
		defaults.RadixGitTagsEnvironmentVariable:    gitTags,
	}
	componentImages := pipelineInfo.DeployEnvironmentComponentImages[envName]

	deployComponents, err := deployment.GetRadixComponentsForEnv(ctx, pipelineInfo.RadixApplication, activeRadixDeployment, envName, componentImages, defaultEnvVars, preservingDeployComponents.DeployComponents)
	if err != nil {
		return nil, err
	}

	jobs, err := deployment.NewJobComponentsBuilder(pipelineInfo.RadixApplication, envName, componentImages, defaultEnvVars, preservingDeployComponents.DeployJobComponents).JobComponents(ctx)
	if err != nil {
		return nil, err
	}

	radixDeployment := constructRadixDeployment(pipelineInfo, envName, commitID, gitTags, deployComponents, jobs, radixConfigHash, buildSecretHash)
	return radixDeployment, nil
}

func GetGitCommitHashFromDeployment(radixDeployment *radixv1.RadixDeployment) string {
	if gitCommitHash, ok := radixDeployment.Annotations[kube.RadixCommitAnnotation]; ok {
		return gitCommitHash
	}
	if gitCommitHash, ok := radixDeployment.Labels[kube.RadixCommitLabel]; ok {
		return gitCommitHash
	}
	return ""
}

func constructRadixDeployment(pipelineInfo *model.PipelineInfo, env, commitID, gitTags string, components []radixv1.RadixDeployComponent, jobs []radixv1.RadixDeployJobComponent, radixConfigHash, buildSecretHash string) *radixv1.RadixDeployment {
	radixApplication := pipelineInfo.RadixApplication
	appName := radixApplication.GetName()
	jobName := pipelineInfo.PipelineArguments.JobName
	imageTag := pipelineInfo.PipelineArguments.ImageTag
	deployName := utils.GetDeploymentName(env, imageTag)
	imagePullSecrets := make([]corev1.LocalObjectReference, 0)
	if len(radixApplication.Spec.PrivateImageHubs) > 0 {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: defaults.PrivateImageHubSecretName})
	}
	annotations := map[string]string{
		kube.RadixBranchAnnotation:      pipelineInfo.PipelineArguments.Branch,
		kube.RadixGitRefsTypeAnnotation: pipelineInfo.PipelineArguments.GitRefsType,
		kube.RadixGitTagsAnnotation:     gitTags,
		kube.RadixCommitAnnotation:      commitID,
		kube.RadixBuildSecretHash:       buildSecretHash,
		kube.RadixConfigHash:            radixConfigHash,
		kube.RadixUseBuildKit:           strconv.FormatBool(pipelineInfo.IsUsingBuildKit()),
		kube.RadixUseBuildCache:         strconv.FormatBool(pipelineInfo.IsUsingBuildCache()),
		kube.RadixRefreshBuildCache:     strconv.FormatBool(pipelineInfo.IsRefreshingBuildCache()),
	}

	radixDeployment := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployName,
			Namespace: utils.GetEnvironmentNamespace(appName, env),
			Labels: map[string]string{
				kube.RadixAppLabel:     appName,
				kube.RadixEnvLabel:     env,
				kube.RadixCommitLabel:  commitID,
				kube.RadixJobNameLabel: jobName,
			},
			Annotations: annotations,
		},
		Spec: radixv1.RadixDeploymentSpec{
			AppName:          appName,
			Environment:      env,
			Components:       components,
			Jobs:             jobs,
			ImagePullSecrets: imagePullSecrets,
		},
	}
	return radixDeployment
}

func getPreservingDeployComponents(ctx context.Context, activeRadixDeployment *radixv1.RadixDeployment, envName string, buildContext *model.BuildContext, componentsToDeploy []string) PreservingDeployComponents {
	preservingDeployComponents := PreservingDeployComponents{}
	existEnvironmentComponentsToBuild := buildContext != nil && !buildContext.ChangedRadixConfig && slice.Any(buildContext.EnvironmentsToBuild, func(environmentToBuild model.EnvironmentToBuild) bool {
		return len(environmentToBuild.Components) > 0
	})
	if activeRadixDeployment == nil || (len(componentsToDeploy) == 0 && !existEnvironmentComponentsToBuild) {
		return preservingDeployComponents
	}
	if len(componentsToDeploy) == 0 && existEnvironmentComponentsToBuild {
		componentsToDeploy = slice.Reduce(buildContext.EnvironmentsToBuild, make([]string, 0), func(acc []string, envComponentsToBuild model.EnvironmentToBuild) []string {
			if envName == envComponentsToBuild.Environment && len(envComponentsToBuild.Components) > 0 {
				return append(acc, envComponentsToBuild.Components...)
			}
			return acc
		})
	}

	log.Ctx(ctx).Info().Msgf("Deploy only following component(s): %s", strings.Join(componentsToDeploy, ","))
	componentNames := slice.Reduce(componentsToDeploy, make(map[string]bool), func(acc map[string]bool, componentName string) map[string]bool {
		componentName = strings.TrimSpace(componentName)
		if len(componentName) > 0 {
			acc[componentName] = true
		}
		return acc
	})
	preservingDeployComponents.DeployComponents = slice.FindAll(activeRadixDeployment.Spec.Components, func(component radixv1.RadixDeployComponent) bool {
		return !componentNames[component.GetName()]
	})
	preservingDeployComponents.DeployJobComponents = slice.FindAll(activeRadixDeployment.Spec.Jobs, func(component radixv1.RadixDeployJobComponent) bool {
		return !componentNames[component.GetName()]
	})
	return preservingDeployComponents
}

// GetActiveRadixDeployment Returns active RadixDeployment if it exists and if it is available to get
func GetActiveRadixDeployment(ctx context.Context, kubeUtil *kube.Kube, namespace string) (*radixv1.RadixDeployment, error) {
	var currentRd *radixv1.RadixDeployment
	// For new applications, or applications with new environments defined in radixconfig, the namespace
	// or rolebinding may not be configured yet by radix-operator.
	// We skip getting active deployment if namespace does not exist or pipeline-runner does not have access

	if _, err := kubeUtil.KubeClient().CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{}); err != nil {
		if !kubeerrors.IsNotFound(err) && !kubeerrors.IsForbidden(err) {
			return nil, err
		}
		log.Ctx(ctx).Info().Msg("namespace for environment does not exist yet")
	} else {
		currentRd, err = kubeUtil.GetActiveDeployment(ctx, namespace)
		if err != nil {
			return nil, err
		}
	}
	return currentRd, nil
}

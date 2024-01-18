package internal

import (
	"strings"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConstructForTargetEnvironment Will build a deployment for target environment
func ConstructForTargetEnvironment(config *radixv1.RadixApplication, activeRadixDeployment *radixv1.RadixDeployment, jobName, imageTag, branch string, componentImages pipeline.DeployComponentImages, env string, defaultEnvVars radixv1.EnvVarsMap, radixConfigHash, buildSecretHash string, componentsToDeploy []string) (*radixv1.RadixDeployment, error) {
	preservingDeployComponents, preservingDeployJobComponents, err := getDeployComponents(activeRadixDeployment, componentsToDeploy)
	if err != nil {
		return nil, err
	}

	commitID := defaultEnvVars[defaults.RadixCommitHashEnvironmentVariable]
	gitTags := defaultEnvVars[defaults.RadixGitTagsEnvironmentVariable]
	deployComponents, err := deployment.GetRadixComponentsForEnv(config, env, componentImages, defaultEnvVars, preservingDeployComponents)
	if err != nil {
		return nil, err
	}
	jobs, err := deployment.NewJobComponentsBuilder(config, env, componentImages, defaultEnvVars, preservingDeployJobComponents).JobComponents()
	if err != nil {
		return nil, err
	}
	radixDeployment := constructRadixDeployment(config, env, jobName, imageTag, branch, commitID, gitTags, deployComponents, jobs, radixConfigHash, buildSecretHash)
	return radixDeployment, nil
}

func constructRadixDeployment(radixApplication *radixv1.RadixApplication, env, jobName, imageTag, branch, commitID, gitTags string, components []radixv1.RadixDeployComponent, jobs []radixv1.RadixDeployJobComponent, radixConfigHash, buildSecretHash string) *radixv1.RadixDeployment {
	appName := radixApplication.GetName()
	deployName := utils.GetDeploymentName(appName, env, imageTag)
	imagePullSecrets := []corev1.LocalObjectReference{}
	if len(radixApplication.Spec.PrivateImageHubs) > 0 {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: defaults.PrivateImageHubSecretName})
	}
	annotations := map[string]string{
		kube.RadixBranchAnnotation:  branch,
		kube.RadixGitTagsAnnotation: gitTags,
		kube.RadixCommitAnnotation:  commitID,
		kube.RadixBuildSecretHash:   buildSecretHash,
		kube.RadixConfigHash:        radixConfigHash,
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

func getDeployComponents(activeRadixDeployment *radixv1.RadixDeployment, components []string) ([]radixv1.RadixDeployComponent, []radixv1.RadixDeployJobComponent, error) {
	if activeRadixDeployment == nil || len(components) == 0 {
		return nil, nil, nil
	}
	log.Infof("Deploy only following component(s): %s", strings.Join(components, ","))
	componentNames := slice.Reduce(components, make(map[string]bool), func(acc map[string]bool, name string) map[string]bool {
		acc[name] = true
		return acc
	})
	deployComponents := slice.FindAll(activeRadixDeployment.Spec.Components, func(component radixv1.RadixDeployComponent) bool {
		return !componentNames[component.GetName()]
	})
	deployJobComponents := slice.FindAll(activeRadixDeployment.Spec.Jobs, func(component radixv1.RadixDeployJobComponent) bool {
		return !componentNames[component.GetName()]
	})
	return deployComponents, deployJobComponents, nil
}

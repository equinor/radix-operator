package internal

import (
	"errors"
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

// PreservingDeployComponents DeployComponents, not to be updated during deployment, but transferred from an active deployment
type PreservingDeployComponents struct {
	DeployComponents    []radixv1.RadixDeployComponent
	DeployJobComponents []radixv1.RadixDeployJobComponent
}

// ConstructForTargetEnvironment Will build a deployment for target environment
func ConstructForTargetEnvironment(config *radixv1.RadixApplication, activeRadixDeployment *radixv1.RadixDeployment, jobName, imageTag, branch string,
	componentImages pipeline.DeployComponentImages, env string, defaultEnvVars radixv1.EnvVarsMap, radixConfigHash, buildSecretHash string,
	componentsToDeploy []string) (*radixv1.RadixDeployment, error) {

	preservingDeployComponents, err := getPreservingDeployComponents(activeRadixDeployment, componentsToDeploy)
	if err != nil {
		return nil, err
	}

	commitID := defaultEnvVars[defaults.RadixCommitHashEnvironmentVariable]
	gitTags := defaultEnvVars[defaults.RadixGitTagsEnvironmentVariable]
	deployComponents, err := deployment.GetRadixComponentsForEnv(config, env, componentImages, defaultEnvVars, preservingDeployComponents.DeployComponents)
	if err != nil {
		return nil, err
	}
	jobs, err := deployment.NewJobComponentsBuilder(config, env, componentImages, defaultEnvVars, preservingDeployComponents.DeployJobComponents).JobComponents()
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

func getPreservingDeployComponents(activeRadixDeployment *radixv1.RadixDeployment, componentsToDeploy []string) (PreservingDeployComponents, error) {
	preservingDeployComponents := PreservingDeployComponents{}
	if activeRadixDeployment == nil || len(componentsToDeploy) == 0 {
		return preservingDeployComponents, nil
	}
	log.Infof("Deploy only following component(s): %s", strings.Join(componentsToDeploy, ","))
	componentNames := slice.Reduce(componentsToDeploy, make(map[string]bool), func(acc map[string]bool, name string) map[string]bool {
		acc[name] = true
		return acc
	})
	preservingDeployComponents.DeployComponents = slice.FindAll(activeRadixDeployment.Spec.Components, func(component radixv1.RadixDeployComponent) bool {
		return !componentNames[component.GetName()]
	})
	preservingDeployComponents.DeployJobComponents = slice.FindAll(activeRadixDeployment.Spec.Jobs, func(component radixv1.RadixDeployJobComponent) bool {
		return !componentNames[component.GetName()]
	})
	if len(preservingDeployComponents.DeployComponents)+len(preservingDeployComponents.DeployJobComponents) != len(componentsToDeploy) {
		return PreservingDeployComponents{}, errors.New("not all components of jobs requested for deployment found")
	}
	return preservingDeployComponents, nil
}

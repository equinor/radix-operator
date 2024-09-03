package build

import (
	"errors"
	"fmt"
	"strings"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	corev1 "k8s.io/api/core/v1"
)

// Will ensure that all build secrets are mounted from build-secrets secret with BUILD_SECRET_ prefix
func getBuildSecretsAsVariables(pipelineInfo *model.PipelineInfo) ([]corev1.EnvVar, error) {
	if pipelineInfo.RadixApplication.Spec.Build == nil || len(pipelineInfo.RadixApplication.Spec.Build.Secrets) == 0 {
		return nil, nil
	}

	if pipelineInfo.BuildSecret == nil {
		return nil, errors.New("build secrets has not been set")
	}

	var environmentVariables []corev1.EnvVar
	for _, secretName := range pipelineInfo.RadixApplication.Spec.Build.Secrets {
		if secretValue, ok := pipelineInfo.BuildSecret.Data[secretName]; !ok || strings.EqualFold(string(secretValue), defaults.BuildSecretDefaultData) {
			return nil, fmt.Errorf("build secret %s has not been set", secretName)
		}

		buildSecretName := defaults.BuildSecretPrefix + secretName

		secretKeySelector := corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: defaults.BuildSecretsName,
			},
			Key: secretName,
		}
		envVarSource := corev1.EnvVarSource{
			SecretKeyRef: &secretKeySelector,
		}
		secretEnvVar := corev1.EnvVar{
			Name:      buildSecretName,
			ValueFrom: &envVarSource,
		}
		environmentVariables = append(environmentVariables, secretEnvVar)
	}

	return environmentVariables, nil
}

// Will ensure that all build secrets are mounted from build-secrets secret with BUILD_SECRET_ prefix
func validateBuildSecrets(pipelineInfo *model.PipelineInfo) error {
	if pipelineInfo.RadixApplication.Spec.Build == nil || len(pipelineInfo.RadixApplication.Spec.Build.Secrets) == 0 {
		return nil
	}

	if pipelineInfo.BuildSecret == nil {
		return errors.New("build secrets has not been set")
	}

	for _, secretName := range pipelineInfo.RadixApplication.Spec.Build.Secrets {
		if secretValue, ok := pipelineInfo.BuildSecret.Data[secretName]; !ok || strings.EqualFold(string(secretValue), defaults.BuildSecretDefaultData) {
			return fmt.Errorf("build secret %s has not been set", secretName)
		}
	}

	return nil
}

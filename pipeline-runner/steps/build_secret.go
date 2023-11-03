package steps

import (
	"errors"
	"fmt"
	"strings"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	corev1 "k8s.io/api/core/v1"
)

// Will ensure that all build secrets are mounted from build-secrets secret with BUILD_SECRET_ prefix
func getBuildSecretsAsVariables(piplineInfo *model.PipelineInfo) ([]corev1.EnvVar, error) {
	if piplineInfo.RadixApplication.Spec.Build == nil || len(piplineInfo.RadixApplication.Spec.Build.Secrets) == 0 {
		return nil, nil
	}

	if piplineInfo.BuildSecret == nil {
		return nil, errors.New("build secrets has not been set")
	}

	var environmentVariables []corev1.EnvVar
	for _, secretName := range piplineInfo.RadixApplication.Spec.Build.Secrets {
		if _, ok := piplineInfo.BuildSecret.Data[secretName]; !ok {
			return nil, fmt.Errorf("build secret %s has not been set", secretName)
		}

		secretValue := string(piplineInfo.BuildSecret.Data[secretName])
		if strings.EqualFold(secretValue, defaults.BuildSecretDefaultData) {
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

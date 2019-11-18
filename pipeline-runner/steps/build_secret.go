package steps

import (
	"errors"
	"fmt"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Will ensure that all build secrets are mounted from build-secrets secret with BUILD_SECRET_ prefix
func getBuildSecretsAsVariables(kubeclient kubernetes.Interface, ra *v1.RadixApplication, appNamespace string) ([]corev1.EnvVar, error) {
	var environmentVariables []corev1.EnvVar

	if ra.Spec.Build != nil {
		buildSecrets, err := kubeclient.CoreV1().Secrets(appNamespace).Get(defaults.BuildSecretsName, metav1.GetOptions{})
		if err != nil || buildSecrets == nil {
			return nil, errors.New("Build secrets has not been set")
		}

		for _, secretName := range ra.Spec.Build.Secrets {
			if _, ok := buildSecrets.Data[secretName]; !ok {
				return nil, fmt.Errorf("Build secret '%s' has not been set", secretName)
			}

			secretValue := string(buildSecrets.Data[secretName])
			if strings.EqualFold(secretValue, defaults.BuildSecretDefaultData) {
				return nil, fmt.Errorf("Build secret '%s' has not been set", secretName)
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
	}

	return environmentVariables, nil
}

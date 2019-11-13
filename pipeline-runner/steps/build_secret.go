package steps

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func getBuildSecretsAsVariables(kubeclient kubernetes.Interface, appNamespace string) []corev1.EnvVar {
	var environmentVariables []corev1.EnvVar

	buildSecrets, err := kubeclient.CoreV1().Secrets(appNamespace).Get(defaults.BuildSecretsName, metav1.GetOptions{})

	if err == nil && buildSecrets != nil {
		for secretName := range buildSecrets.Data {
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

	return environmentVariables
}

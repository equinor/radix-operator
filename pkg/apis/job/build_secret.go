package job

import (
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const buildSecretPrefix = "BUILD_SECRET_"

func (job *Job) getBuildSecretsAsVariables() []corev1.EnvVar {
	var environmentVariables []corev1.EnvVar

	buildSecrets, err := job.kubeclient.CoreV1().Secrets(job.radixJob.Namespace).List(metav1.ListOptions{
		LabelSelector: kube.RadixBuildSecretLabel,
	})

	if err == nil {
		for _, buildSecret := range buildSecrets.Items {
			buildSecretName := buildSecretPrefix + strings.ToUpper(buildSecret.Name)

			secretKeySelector := corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: buildSecret.Name,
				},
				Key: buildSecretName,
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

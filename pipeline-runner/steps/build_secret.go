package steps

import (
	"context"
	"errors"
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"strings"
)

// Will ensure that all build secrets are mounted from build-secrets secret with BUILD_SECRET_ prefix
func getBuildSecretsAsVariables(kubeclient kubernetes.Interface, ra *v1.RadixApplication, appNamespace string) ([]corev1.EnvVar, error) {
	if ra.Spec.Build == nil || len(ra.Spec.Build.Secrets) == 0 {
		return nil, nil
	}

	var environmentVariables []corev1.EnvVar
	buildSecrets, err := kubeclient.CoreV1().Secrets(appNamespace).Get(context.TODO(), defaults.BuildSecretsName, metav1.GetOptions{})
	if err != nil || buildSecrets == nil {
		return nil, errors.New("build secrets has not been set")
	}

	for _, secretName := range ra.Spec.Build.Secrets {
		if _, ok := buildSecrets.Data[secretName]; !ok {
			return nil, fmt.Errorf("build secret %s has not been set", secretName)
		}

		secretValue := string(buildSecrets.Data[secretName])
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

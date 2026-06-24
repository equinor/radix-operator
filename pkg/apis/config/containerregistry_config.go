package config

import corev1 "k8s.io/api/core/v1"

type ContainerRegistryConfig struct {
	// Name of the secret container docker authentication for external registries
	ExternalRegistryAuthSecret string `envconfig:"RADIX_EXTERNAL_REGISTRY_DEFAULT_AUTH_SECRET" required:"true"`
}

func (c ContainerRegistryConfig) ImagePullSecretsFromExternalRegistryAuth() []corev1.LocalObjectReference {
	if len(c.ExternalRegistryAuthSecret) == 0 {
		return nil
	}

	return []corev1.LocalObjectReference{{Name: c.ExternalRegistryAuthSecret}}
}

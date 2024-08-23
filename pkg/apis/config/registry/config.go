package registry

import corev1 "k8s.io/api/core/v1"

type RegistryConfig struct {
	DefaultAuthSecret string
}

func (c RegistryConfig) ImagePullSecretsFromDefaultAuth() []corev1.LocalObjectReference {
	if len(c.DefaultAuthSecret) == 0 {
		return nil
	}

	return []corev1.LocalObjectReference{{Name: c.DefaultAuthSecret}}
}

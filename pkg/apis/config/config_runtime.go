package config

import (
	corev1 "k8s.io/api/core/v1"
)

type RuntimeConfig struct {
	// Architectures is validated before DefaultArchitecture so an empty map is reported on this field.
	// Architectures       map[string]ArchitectureSpec `json:"architectures" required:"true" validate:"size(self) > 0"`
	// DefaultArchitecture string                      `json:"defaultArchitecture" required:"true" validate:"self in config.runtime.architectures && config.runtime.architectures[self].enabled"`
	// SpecialNodeTypes    map[string]NodeTypeConfig   `json:"specialNodeTypes"`

	// JobSchedulerTemplate       corev1.PodTemplateSpec      `json:"jobSchedulerTemplate"`
	// JobSchedulerAuxTemplate    corev1.PodTemplateSpec      `json:"jobSchedulerAuxTemplate"`
	PipelineRunnerTemplate     RuntimeBaseOverlayPodConfig `json:"pipelineRunnerTemplate"`
	Oauth2ProxyTemplate        RuntimeBaseOverlayPodConfig `json:"oauth2ProxyTemplate"`
	Oauth2SessionStoreTemplate RuntimeBaseOverlayPodConfig `json:"oauth2SessionStoreTemplate"`
}

type RuntimeBaseOverlayPodConfig struct {
	Base    corev1.PodTemplateSpec `json:"base" required:"true"`
	Overlay corev1.PodTemplateSpec `json:"overlay"`
}

type NodeTypeConfig struct {
	Description     string                 `json:"description"`
	Architecture    string                 `json:"architecture" required:"true"`
	Template        corev1.PodTemplateSpec `json:"template"`
	BuilderTemplate corev1.PodTemplateSpec `json:"builderTemplate"`
}

type ArchitectureSpec struct {
	Enabled           bool                   `json:"enabled"`
	BuilderTemplate   corev1.PodTemplateSpec `json:"builderTemplate"`
	JobTemplate       corev1.PodTemplateSpec `json:"jobTemplate"`
	ComponentTemplate corev1.PodTemplateSpec `json:"componentTemplate"`
}

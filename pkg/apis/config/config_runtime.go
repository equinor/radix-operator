package config

import (
	corev1 "k8s.io/api/core/v1"
)

type RuntimeConfig struct {
	Architectures       map[string]ArchitectureSpec `json:"architectures"`
	DefaultArchitecture string                      `json:"defaultArchitecture" required:"true" validate:"self in config.runtime.architectures && !config.runtime.architectures[self].disabled"`
	NodeTypes           map[string]NodeTypeConfig   `json:"nodeTypes"`

	// JobSchedulerTemplate       corev1.PodTemplateSpec      `json:"jobSchedulerTemplate"`
	// JobSchedulerAuxTemplate    corev1.PodTemplateSpec      `json:"jobSchedulerAuxTemplate"`
	PipelineRunnerTemplate     RuntimeBaseOverlayPodConfig `json:"pipelineRunnerTemplate"`
	PipelineBuilderTemplate    RuntimeBaseOverlayPodConfig `json:"pipelineBuilderTemplate"`
	Oauth2ProxyTemplate        RuntimeBaseOverlayPodConfig `json:"oauth2ProxyTemplate"`
	Oauth2SessionStoreTemplate RuntimeBaseOverlayPodConfig `json:"oauth2SessionStoreTemplate"`
}

type RuntimeBaseOverlayPodConfig struct {
	Base    corev1.PodTemplateSpec `json:"base" required:"true"`
	Overlay corev1.PodTemplateSpec `json:"overlay"`
}

type NodeTypeConfig struct {
	Disabled        bool                   `json:"disabled"`
	Description     string                 `json:"description"`
	Architecture    string                 `json:"architecture" required:"true"`
	BuilderTemplate corev1.PodTemplateSpec `json:"builderTemplate"`
	// JobTemplate       corev1.PodTemplateSpec `json:"jobTemplate"`
	// ComponentTemplate corev1.PodTemplateSpec `json:"componentTemplate"`
}

type ArchitectureSpec struct {
	Disabled        bool                   `json:"disabled"`
	BuilderTemplate corev1.PodTemplateSpec `json:"builderTemplate"`
	// JobTemplate       corev1.PodTemplateSpec `json:"jobTemplate"`
	// ComponentTemplate corev1.PodTemplateSpec `json:"componentTemplate"`
}

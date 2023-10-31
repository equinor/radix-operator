package v1

import (
	"github.com/equinor/radix-common/utils/numbers"
)

type RadixCommonEnvironmentConfig interface {
	GetEnvironment() string
	GetVariables() EnvVarsMap
	GetSecretRefs() RadixSecretRefs
	GetResources() ResourceRequirements
	GetNode() RadixNode
	GetImageTagName() string
	GetHorizontalScaling() *RadixHorizontalScaling
	GetReplicas() *int
	GetIdentity() *Identity
	GetEnabled() *bool
}

func (config RadixEnvironmentConfig) GetEnvironment() string {
	return config.Environment
}

func (config RadixEnvironmentConfig) GetSecretRefs() RadixSecretRefs {
	return config.SecretRefs
}

func (config RadixEnvironmentConfig) GetVariables() EnvVarsMap {
	return config.Variables
}

func (config RadixEnvironmentConfig) GetResources() ResourceRequirements {
	return config.Resources
}

func (config RadixEnvironmentConfig) GetNode() RadixNode {
	return config.Node
}

func (config RadixEnvironmentConfig) GetImageTagName() string {
	return config.ImageTagName
}

func (config RadixEnvironmentConfig) GetHorizontalScaling() *RadixHorizontalScaling {
	return config.HorizontalScaling
}

func (config RadixEnvironmentConfig) GetReplicas() *int {
	return config.Replicas
}

func (config RadixEnvironmentConfig) GetIdentity() *Identity {
	return config.Identity
}

func (config RadixEnvironmentConfig) GetEnabled() *bool {
	return config.Enabled
}

func (config RadixJobComponentEnvironmentConfig) GetEnvironment() string {
	return config.Environment
}

func (config RadixJobComponentEnvironmentConfig) GetSecretRefs() RadixSecretRefs {
	return config.SecretRefs
}

func (config RadixJobComponentEnvironmentConfig) GetVariables() EnvVarsMap {
	return config.Variables
}

func (config RadixJobComponentEnvironmentConfig) GetResources() ResourceRequirements {
	return config.Resources
}

func (config RadixJobComponentEnvironmentConfig) GetNode() RadixNode {
	return config.Node
}

func (config RadixJobComponentEnvironmentConfig) GetImageTagName() string {
	return config.ImageTagName
}

func (config RadixJobComponentEnvironmentConfig) GetHorizontalScaling() *RadixHorizontalScaling {
	return nil
}

func (config RadixJobComponentEnvironmentConfig) GetReplicas() *int {
	return numbers.IntPtr(1)
}

func (config RadixJobComponentEnvironmentConfig) GetIdentity() *Identity {
	return config.Identity
}

// GetNotifications Get job component notifications
func (config RadixJobComponentEnvironmentConfig) GetNotifications() *Notifications {
	return config.Notifications
}

func (config RadixJobComponentEnvironmentConfig) GetEnabled() *bool {
	return config.Enabled
}

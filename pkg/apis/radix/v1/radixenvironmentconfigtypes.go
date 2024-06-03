package v1

import (
	"github.com/equinor/radix-common/utils/numbers"
)

type RadixCommonEnvironmentConfig interface {
	GetEnvironment() string
	GetImage() string
	// GetDockerfileName Gets component docker file name
	GetDockerfileName() string
	// GetSourceFolder Gets component source folder
	GetSourceFolder() string
	GetVariables() EnvVarsMap
	GetSecretRefs() RadixSecretRefs
	GetResources() ResourceRequirements
	GetNode() RadixNode
	GetImageTagName() string
	GetHorizontalScaling() *RadixHorizontalScaling
	GetReplicas() *int
	GetIdentity() *Identity
	GetReadOnlyFileSystem() *bool
	// GetMonitoring Gets monitoring setting
	GetMonitoring() *bool
	// GetVolumeMounts Get volume mounts configurations
	GetVolumeMounts() []RadixVolumeMount
	// GetRuntime Gets environment specific target runtime requirements
	GetRuntime() *Runtime
	getEnabled() *bool
}

func (config *RadixEnvironmentConfig) GetEnvironment() string {
	return config.Environment
}

func (config *RadixEnvironmentConfig) GetImage() string {
	return config.Image
}

func (config *RadixEnvironmentConfig) GetDockerfileName() string {
	return config.DockerfileName
}

func (config *RadixEnvironmentConfig) GetSourceFolder() string {
	return config.SourceFolder
}

func (config *RadixEnvironmentConfig) GetSecretRefs() RadixSecretRefs {
	return config.SecretRefs
}

func (config *RadixEnvironmentConfig) GetVariables() EnvVarsMap {
	return config.Variables
}

func (config *RadixEnvironmentConfig) GetResources() ResourceRequirements {
	return config.Resources
}

func (config *RadixEnvironmentConfig) GetNode() RadixNode {
	return config.Node
}

func (config *RadixEnvironmentConfig) GetImageTagName() string {
	return config.ImageTagName
}

func (config *RadixEnvironmentConfig) GetHorizontalScaling() *RadixHorizontalScaling {
	return config.HorizontalScaling
}

func (config *RadixEnvironmentConfig) GetReplicas() *int {
	return config.Replicas
}

func (config *RadixEnvironmentConfig) GetIdentity() *Identity {
	return config.Identity
}

func (config *RadixEnvironmentConfig) GetReadOnlyFileSystem() *bool {
	return config.ReadOnlyFileSystem
}

func (config *RadixEnvironmentConfig) GetMonitoring() *bool {
	return config.Monitoring
}

func (config *RadixEnvironmentConfig) GetVolumeMounts() []RadixVolumeMount {
	return config.VolumeMounts
}

func (config *RadixEnvironmentConfig) GetRuntime() *Runtime {
	return config.Runtime
}

func (config *RadixEnvironmentConfig) getEnabled() *bool {
	return config.Enabled
}

func (config *RadixJobComponentEnvironmentConfig) GetEnvironment() string {
	return config.Environment
}

func (config *RadixJobComponentEnvironmentConfig) GetImage() string {
	return config.Image
}

func (config *RadixJobComponentEnvironmentConfig) GetDockerfileName() string {
	return config.DockerfileName
}

func (config *RadixJobComponentEnvironmentConfig) GetSourceFolder() string {
	return config.SourceFolder
}

func (config *RadixJobComponentEnvironmentConfig) GetSecretRefs() RadixSecretRefs {
	return config.SecretRefs
}

func (config *RadixJobComponentEnvironmentConfig) GetVariables() EnvVarsMap {
	return config.Variables
}

func (config *RadixJobComponentEnvironmentConfig) GetResources() ResourceRequirements {
	return config.Resources
}

func (config *RadixJobComponentEnvironmentConfig) GetNode() RadixNode {
	return config.Node
}

func (config *RadixJobComponentEnvironmentConfig) GetImageTagName() string {
	return config.ImageTagName
}

func (config *RadixJobComponentEnvironmentConfig) GetHorizontalScaling() *RadixHorizontalScaling {
	return nil
}

func (config *RadixJobComponentEnvironmentConfig) GetReplicas() *int {
	return numbers.IntPtr(1)
}

func (config *RadixJobComponentEnvironmentConfig) GetIdentity() *Identity {
	return config.Identity
}

// GetNotifications Get job component notifications
func (config *RadixJobComponentEnvironmentConfig) GetNotifications() *Notifications {
	return config.Notifications
}

func (config *RadixJobComponentEnvironmentConfig) GetReadOnlyFileSystem() *bool {
	return config.ReadOnlyFileSystem
}

func (config *RadixJobComponentEnvironmentConfig) GetMonitoring() *bool {
	return config.Monitoring
}

func (config *RadixJobComponentEnvironmentConfig) GetVolumeMounts() []RadixVolumeMount {
	return config.VolumeMounts
}

func (config *RadixJobComponentEnvironmentConfig) GetRuntime() *Runtime {
	return config.Runtime
}

func (config *RadixJobComponentEnvironmentConfig) getEnabled() *bool {
	return config.Enabled
}

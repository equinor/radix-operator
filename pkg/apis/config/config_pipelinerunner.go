package config

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

type PipelineRunnerConfig struct {
	ContainerRegistry      string         `json:"containerRegistry" required:"true"`
	CacheContainerRegistry string         `json:"cacheContainerRegistry" required:"true"`
	Builder                BuilderConfig  `json:"builder" required:"true"`
	GitCloneImage          ContainerImage `json:"gitCloneImage" required:"true"`
}

type BuilderConfig struct {
	Image                          ContainerImage `json:"image" required:"true"`
	SeccompProfileLocalhostProfile string         `json:"seccompProfileLocalhostProfile" required:"true"`
	Resources                      Resources      `json:"resources" required:"true" validate:"compareQuantity(self.limits.memory, self.requests.memory) >= 0 && compareQuantity(self.limits.cpu, self.requests.cpu) >= 0"`
}

type Resources struct {
	Requests ResourceRequirements `json:"requests" required:"true"`
	Limits   ResourceRequirements `json:"limits" required:"true"`
}
type ResourceRequirements struct {
	Memory *resource.Quantity `json:"memory" required:"true"`
	CPU    *resource.Quantity `json:"cpu" required:"true"`
}

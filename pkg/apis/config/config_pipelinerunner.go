package config

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

type PipelineRunnerConfig struct {
	ContainerRegistry      string `json:"containerRegistry" required:"true"`
	CacheContainerRegistry string `json:"cacheContainerRegistry" required:"true"`
}

type Resources struct {
	Requests ResourceRequirements `json:"requests" required:"true"`
	Limits   ResourceRequirements `json:"limits" required:"true"`
}
type ResourceRequirements struct {
	Memory resource.Quantity `json:"memory" required:"true"`
	CPU    resource.Quantity `json:"cpu" required:"true"`
}

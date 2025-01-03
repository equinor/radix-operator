package deployment

import (
	"github.com/equinor/radix-operator/pkg/apis/utils/resources"
	"k8s.io/api/core/v1"
)

func getJobAuxResources() v1.ResourceRequirements {
	return resources.New(resources.WithCPUMilli(1), resources.WithMemoryMega(20))
}

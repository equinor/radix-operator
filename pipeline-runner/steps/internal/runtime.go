package internal

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/pointers"
)

// GetRuntimeForEnvironment Returns Runtime configuration by combining Runtime from the
// specified environment with Runtime configuration on the component level.
func GetRuntimeForEnvironment(radixComponent v1.RadixCommonComponent, environment string) *v1.Runtime {
	environmentSpecificConfig := radixComponent.GetEnvironmentConfigByName(environment)
	if !pointers.IsNil(environmentSpecificConfig) {
		if runtime := environmentSpecificConfig.GetRuntime(); runtime != nil {
			return runtime
		}
	}
	return radixComponent.GetRuntime()
}

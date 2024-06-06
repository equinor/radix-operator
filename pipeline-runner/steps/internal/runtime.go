package internal

import (
	"reflect"

	commonutils "github.com/equinor/radix-common/utils"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// GetRuntimeForEnvironment Returns Runtime configuration by combining Runtime from the
// specified environment with Runtime configuration on the component level.
func GetRuntimeForEnvironment(radixComponent v1.RadixCommonComponent, environment string) *v1.Runtime {
	var finalRuntime v1.Runtime

	if rt := radixComponent.GetRuntime(); rt != nil {
		finalRuntime.Architecture = rt.Architecture
	}

	environmentSpecificConfig := radixComponent.GetEnvironmentConfigByName(environment)

	if !commonutils.IsNil(environmentSpecificConfig) && environmentSpecificConfig.GetRuntime() != nil {
		if arch := environmentSpecificConfig.GetRuntime().Architecture; len(arch) > 0 {
			finalRuntime.Architecture = arch
		}
	}

	if reflect.ValueOf(finalRuntime).IsZero() {
		return nil
	}
	return &finalRuntime
}

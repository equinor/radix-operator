package utils

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// GetArchitectureFromRuntime returns architecture from Runtime.
// If Runtime is nil or Runtime.Architecture is empty then defaults.DefaultNodeSelectorArchitecture is returned
func GetArchitectureFromRuntime(runtime *radixv1.Runtime) string {
	if runtime != nil && len(runtime.Architecture) > 0 {
		return string(runtime.Architecture)
	}
	return defaults.DefaultNodeSelectorArchitecture
}

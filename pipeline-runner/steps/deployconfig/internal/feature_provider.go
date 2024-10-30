package internal

import (
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// FeatureProvider provides methods for checking and mutating Radix features
type FeatureProvider interface {
	// Check if feature is enabled for the specified environment by inspecting RadixApplication and active RadixDeployment (if set)
	IsEnabledForEnvironment(envName string, ra radixv1.RadixApplication, activeRd radixv1.RadixDeployment) bool

	// Mutates target with fields from source
	Mutate(target, source radixv1.RadixDeployment) error
}

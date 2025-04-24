package internal

import (
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// FeatureProvider provides methods for checking and mutating Radix features
type FeatureProvider interface {
	// IsEnabledForEnvironment Check if feature is enabled for the specified environment by inspecting RadixApplication and active RadixDeployment (if set)
	IsEnabledForEnvironment(envName string, ra radixv1.RadixApplication, activeRd radixv1.RadixDeployment) bool

	// Mutate Mutates target with fields from source
	Mutate(source, target *radixv1.RadixDeployment) error
}

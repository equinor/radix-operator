package common

// +kubebuilder:object:generate=true

import "maps"

import "github.com/equinor/radix-operator/pkg/apis/radix/v1"

// EnvVars Map of environment variables in the form '<envvarname>: <value>'
type EnvVars map[string]string

// MapToRadixEnvVarsMap maps the object to a RadixV1 EnvVarsMap
func (envVars EnvVars) MapToRadixEnvVarsMap() v1.EnvVarsMap {
	if envVars == nil {
		return nil
	}
	radixEnvVarMap := make(v1.EnvVarsMap, len(envVars))
	maps.Copy(radixEnvVarMap, envVars)
	return radixEnvVarMap
}

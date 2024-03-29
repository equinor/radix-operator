package utils

import (
	"fmt"
	"os"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"sigs.k8s.io/yaml"
)

// GetRadixApplicationFromFile Reads radix config from file
func GetRadixApplicationFromFile(filename string) (*v1.RadixApplication, error) {
	radixApp := &v1.RadixApplication{}
	err := getFromFile(filename, radixApp)
	return radixApp, err
}

// GetRadixRegistrationFromFile Reads radix registration from file
func GetRadixRegistrationFromFile(filename string) (*v1.RadixRegistration, error) {
	reg := &v1.RadixRegistration{}
	err := getFromFile(filename, reg)
	return reg, err
}

// GetRadixDeployFromFile Reads radix deployment from file
func GetRadixDeployFromFile(filename string) (*v1.RadixDeployment, error) {
	radixDeploy := &v1.RadixDeployment{}
	err := getFromFile(filename, radixDeploy)
	return radixDeploy, err
}

// GetRadixEnvironmentFromFile reads RadixEnvironment configuration from a yaml file
func GetRadixEnvironmentFromFile(filename string) (*v1.RadixEnvironment, error) {
	env := &v1.RadixEnvironment{}
	err := getFromFile(filename, env)
	return env, err
}

func getFromFile(filename string, objRef interface{}) error {
	raw, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file %v Error: %v ", filename, err)
	}
	return yaml.Unmarshal(raw, objRef)
}

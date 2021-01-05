package utils

import (
	"fmt"
	"io/ioutil"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// GetRadixApplication Reads radix config from file
func GetRadixApplication(filename string) (*v1.RadixApplication, error) {
	log.Debugf("get radix application yaml from %s", filename)
	radixApp := &v1.RadixApplication{}
	err := getFromFile(filename, radixApp)
	return radixApp, err
}

// GetRadixRegistrationFromFile Reads radix registration from file
func GetRadixRegistrationFromFile(filename string) (*v1.RadixRegistration, error) {
	log.Debugf("get radix registration yaml from %s", filename)
	reg := &v1.RadixRegistration{}
	err := getFromFile(filename, reg)
	return reg, err
}

// GetRadixDeploy Reads radix deployment from file
func GetRadixDeploy(filename string) (*v1.RadixDeployment, error) {
	log.Debugf("get radix deploy yaml from %s", filename)
	radixDeploy := &v1.RadixDeployment{}
	err := getFromFile(filename, radixDeploy)
	return radixDeploy, err
}

// GetRadixEnvironmentFromFile reads RadixEnvironment configuration from a yaml file
func GetRadixEnvironmentFromFile(filename string) (*v1.RadixEnvironment, error) {
	log.Debugf("get radix environment yaml from %s", filename)
	env := &v1.RadixEnvironment{}
	err := getFromFile(filename, env)
	return env, err
}

func getFromFile(filename string, objRef interface{}) error {
	raw, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("Failed to read file %v Error: %v ", filename, err)
	}
	return yaml.Unmarshal(raw, objRef)
}

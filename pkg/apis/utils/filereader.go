package utils

import (
	"fmt"
	"io/ioutil"

	log "github.com/sirupsen/logrus"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	yaml "gopkg.in/yaml.v2"
)

func GetRadixApplication(filename string) (*v1.RadixApplication, error) {
	log.Infof("get radix application yaml from %s", filename)
	radixApp := v1.RadixApplication{}
	yamlFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("Failed to read file %v Error:  %v ", filename, err)
	}
	err = yaml.Unmarshal(yamlFile, &radixApp)
	if err != nil {
		return nil, fmt.Errorf("Unmarshal: %v", err)
	}

	return &radixApp, nil
}

func GetRadixRegistrationFromFile(file string) (*v1.RadixRegistration, error) {
	raw, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	reg := &v1.RadixRegistration{}
	err = yaml.Unmarshal(raw, reg)
	if err != nil {
		return nil, err
	}
	return reg, nil
}

func GetRadixDeploy(filename string) (*v1.RadixDeployment, error) {
	log.Infof("get radix deploy yaml from %s", filename)
	radixDeploy := v1.RadixDeployment{}
	yamlFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("Failed to read file %v Error:  %v ", filename, err)
	}
	err = yaml.Unmarshal(yamlFile, &radixDeploy)
	if err != nil {
		return nil, fmt.Errorf("Unmarshal: %v", err)
	}

	return &radixDeploy, nil
}

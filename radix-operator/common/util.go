package common

import (
	"io/ioutil"

	radix_v1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
	"gopkg.in/yaml.v2"
)

func GetRadixAppFromFile(file string) (*radix_v1.RadixApplication, error) {
	raw, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	app := &radix_v1.RadixApplication{}
	err = yaml.Unmarshal(raw, app)
	if err != nil {
		return nil, err
	}
	return app, nil
}

func GetRadixRegistrationFromFile(file string) (*radix_v1.RadixRegistration, error) {
	raw, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	reg := &radix_v1.RadixRegistration{}
	err = yaml.Unmarshal(raw, reg)
	if err != nil {
		return nil, err
	}
	return reg, nil
}

package utils

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetAuxiliaryComponentDeploymentName(t *testing.T) {
	assert.Equal(t, "component-suffix", GetAuxiliaryComponentDeploymentName("component", "suffix"))
}

func Test_GetAuxiliaryComponentServiceName(t *testing.T) {
	assert.Equal(t, "component-suffix", defaults.defaults.GetAuxiliaryComponentServiceName("component", "suffix"))
}

func Test_GetAuxiliaryComponentSecretName(t *testing.T) {
	assert.Equal(t, "component-suffix-"+strings.ToLower(RandStringStrSeed(8, "component-suffix")), GetAuxiliaryComponentSecretName("component", "suffix"))
}

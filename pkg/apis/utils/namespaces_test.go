package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetAuxiliaryComponentDeploymentName(t *testing.T) {
	assert.Equal(t, "component-suffix", GetAuxiliaryComponentDeploymentName("component", "suffix"))
}

func Test_GetAuxOAuthProxyComponentServiceName(t *testing.T) {
	assert.Equal(t, "component-aux-oauth", GetAuxOAuthProxyComponentServiceName("component"))
}

func Test_GetAuxOAuthRedisComponentServiceName(t *testing.T) {
	assert.Equal(t, "component-aux-oauth-redis", GetAuxOAuthRedisServiceName("component"))
}

func Test_GetAuxiliaryComponentSecretName(t *testing.T) {
	assert.Equal(t, "component-suffix-"+strings.ToLower(RandStringStrSeed(8, "component-suffix")), GetAuxiliaryComponentSecretName("component", "suffix"))
}

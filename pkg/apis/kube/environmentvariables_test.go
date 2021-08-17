package kube

import (
	corev1 "k8s.io/api/core/v1"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_order_of_env_variables(t *testing.T) {
	radixConfigEnvVariableMap := map[string]string{
		"d_key": "4",
		"a_key": "1",
		"c_key": "3",
		"g_key": "6",
		"b_key": "2",
		"q_key": "7",
		"e_key": "5",
	}

	envVarsConfigMap := &corev1.ConfigMap{Data: map[string]string{}}
	envVars := getEnvVarsFromRadixConfig(envVarsConfigMap)
	assert.Len(t, envVars, len(radixConfigEnvVariableMap))
	assert.Equal(t, "a_key", envVars[0].Name)
	assert.Equal(t, "b_key", envVars[1].Name)
	assert.Equal(t, "c_key", envVars[2].Name)
	assert.Equal(t, "d_key", envVars[3].Name)
	assert.Equal(t, "e_key", envVars[4].Name)
	assert.Equal(t, "g_key", envVars[5].Name)
	assert.Equal(t, "q_key", envVars[6].Name)
	for _, envVar := range envVars {
		assert.Equal(t, radixConfigEnvVariableMap[envVar.Name], envVarsConfigMap.Data[envVar.Name])
	}
}

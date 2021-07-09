package deployment

import (
	corev1 "k8s.io/api/core/v1"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_order_of_env_variables(t *testing.T) {
	envVariableMap := map[string]string{
		"d_key": "4",
		"a_key": "1",
		"c_key": "3",
		"g_key": "6",
		"b_key": "2",
		"q_key": "7",
		"e_key": "5",
	}

	configMap := &corev1.ConfigMap{Data: map[string]string{}}
	envVar1 := getEnvVarsFromRadixConfig(envVariableMap, configMap)
	for i := 0; i < 100; i++ {
		envVar2 := getEnvVarsFromRadixConfig(envVariableMap, configMap)
		for i, val := range envVar1 {
			assert.Equal(t, val, envVar2[i])
		}
	}

}

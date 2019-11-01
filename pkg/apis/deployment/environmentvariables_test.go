package deployment

import (
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

	envVar1 := appendAppEnvVariables("adeployment", envVariableMap)
	for i := 0; i < 100; i++ {
		envVar2 := appendAppEnvVariables("adeployment", envVariableMap)
		for i, val := range envVar1 {
			assert.Equal(t, val, envVar2[i])
		}
	}

}

package v1_test

import (
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
)

func Test_RadixComponent_GetRuntimeForEnvironment(t *testing.T) {
	tests := map[string]struct {
		runtime  *v1.Runtime
		env      []v1.RadixEnvironmentConfig
		getEnv   string
		expected *v1.Runtime
	}{
		"all nil": {
			runtime:  nil,
			env:      nil,
			getEnv:   "dev",
			expected: nil,
		},
		"common empty, env nil": {
			runtime:  &v1.Runtime{Architecture: ""},
			env:      nil,
			getEnv:   "dev",
			expected: nil,
		},
		"common set, env nil": {
			runtime:  &v1.Runtime{Architecture: "commonarch"},
			env:      nil,
			getEnv:   "dev",
			expected: &v1.Runtime{Architecture: "commonarch"},
		},
		"common set, env empty": {
			runtime:  &v1.Runtime{Architecture: "commonarch"},
			env:      []v1.RadixEnvironmentConfig{{Environment: "dev", Runtime: &v1.Runtime{Architecture: ""}}},
			getEnv:   "dev",
			expected: &v1.Runtime{Architecture: "commonarch"},
		},
		"common nil, env set": {
			runtime:  nil,
			env:      []v1.RadixEnvironmentConfig{{Environment: "dev", Runtime: &v1.Runtime{Architecture: "devarch"}}},
			getEnv:   "dev",
			expected: &v1.Runtime{Architecture: "devarch"},
		},
		"common empty, env set": {
			runtime:  &v1.Runtime{Architecture: ""},
			env:      []v1.RadixEnvironmentConfig{{Environment: "dev", Runtime: &v1.Runtime{Architecture: "devarch"}}},
			getEnv:   "dev",
			expected: &v1.Runtime{Architecture: "devarch"},
		},
		"common set, env set": {
			runtime:  &v1.Runtime{Architecture: "commonarch"},
			env:      []v1.RadixEnvironmentConfig{{Environment: "dev", Runtime: &v1.Runtime{Architecture: "devarch"}}},
			getEnv:   "dev",
			expected: &v1.Runtime{Architecture: "devarch"},
		},
		"common set, other env set": {
			runtime:  &v1.Runtime{Architecture: "commonarch"},
			env:      []v1.RadixEnvironmentConfig{{Environment: "other", Runtime: &v1.Runtime{Architecture: "devarch"}}},
			getEnv:   "dev",
			expected: &v1.Runtime{Architecture: "commonarch"},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			comp := v1.RadixComponent{Runtime: test.runtime, EnvironmentConfig: test.env}
			actual := comp.GetRuntimeForEnvironment(test.getEnv)
			if test.expected == nil {
				assert.Nil(t, actual)
			} else {
				assert.Equal(t, test.expected, actual)
			}
		})
	}

}

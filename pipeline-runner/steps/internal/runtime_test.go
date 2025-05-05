package internal_test

import (
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
)

func Test_GetRuntimeForEnvironment(t *testing.T) {
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
		"fills missing Architecture and NodeType": {
			runtime: &v1.Runtime{Architecture: v1.RuntimeArchitectureAmd64},
			env:     []v1.RadixEnvironmentConfig{{Environment: "dev", Runtime: &v1.Runtime{}}},
			getEnv:  "dev",
			expected: &v1.Runtime{
				Architecture: v1.RuntimeArchitectureAmd64,
			},
		},
		"preserves existing NodeType": {
			runtime: &v1.Runtime{
				Architecture: v1.RuntimeArchitectureAmd64,
			},
			env: []v1.RadixEnvironmentConfig{{Environment: "dev", Runtime: &v1.Runtime{
				NodeType: pointers.Ptr("gpu-nodes"),
			}}},
			getEnv: "dev",
			expected: &v1.Runtime{
				NodeType: pointers.Ptr("gpu-nodes"),
				// Architecture should be cleared
			},
		},
		"preserves existing Architecture if NodeType not set": {
			runtime: &v1.Runtime{},
			env: []v1.RadixEnvironmentConfig{{Environment: "dev", Runtime: &v1.Runtime{
				Architecture: v1.RuntimeArchitectureArm64,
			}}},
			getEnv: "dev",
			expected: &v1.Runtime{
				Architecture: v1.RuntimeArchitectureArm64,
			},
		},
		"preserves job Architecture if NodeType not set": {
			runtime: &v1.Runtime{
				Architecture: v1.RuntimeArchitectureAmd64,
			},
			env: []v1.RadixEnvironmentConfig{{Environment: "dev", Runtime: &v1.Runtime{
				Architecture: v1.RuntimeArchitectureArm64,
			}}},
			getEnv: "dev",
			expected: &v1.Runtime{
				Architecture: v1.RuntimeArchitectureArm64,
			},
		},
		"preserves job Architecture when there is no default runtime": {
			runtime: &v1.Runtime{},
			env: []v1.RadixEnvironmentConfig{{Environment: "dev", Runtime: &v1.Runtime{
				Architecture: v1.RuntimeArchitectureArm64,
			}}},
			getEnv: "dev",
			expected: &v1.Runtime{
				Architecture: v1.RuntimeArchitectureArm64,
			},
		},
		"sets job Architecture by default runtime": {
			runtime: &v1.Runtime{
				Architecture: v1.RuntimeArchitectureArm64,
			},
			env:    []v1.RadixEnvironmentConfig{},
			getEnv: "dev",
			expected: &v1.Runtime{
				Architecture: v1.RuntimeArchitectureArm64,
			},
		},
		"no job Architecture if no default runtime and job runtime": {
			runtime: &v1.Runtime{
				Architecture: v1.RuntimeArchitectureArm64,
			},
			env: []v1.RadixEnvironmentConfig{},
			expected: &v1.Runtime{
				Architecture: v1.RuntimeArchitectureArm64,
			},
		},
		"clears Architecture if both present after merge": {
			runtime: &v1.Runtime{
				Architecture: v1.RuntimeArchitectureAmd64,
			},
			env: []v1.RadixEnvironmentConfig{{Environment: "dev", Runtime: &v1.Runtime{
				NodeType: pointers.Ptr("edge-nodes"),
			}}},
			getEnv: "dev",
			expected: &v1.Runtime{
				NodeType: pointers.Ptr("edge-nodes"),
				// Architecture must be cleared
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			comp := v1.RadixComponent{Runtime: test.runtime, EnvironmentConfig: test.env}
			actual := internal.GetRuntimeForEnvironment(&comp, test.getEnv)
			if test.expected == nil {
				assert.Nil(t, actual)
			} else {
				assert.Equal(t, test.expected, actual)
			}
		})
	}

}

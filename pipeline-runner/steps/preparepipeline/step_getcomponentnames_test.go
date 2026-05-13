package preparepipeline

import (
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
)

func Test_getComponentNames(t *testing.T) {
	tests := []struct {
		name     string
		ra       *radixv1.RadixApplication
		expected model.EnvironmentComponentNames
	}{
		{
			name:     "nil RadixApplication returns nil",
			ra:       nil,
			expected: nil,
		},
		{
			name: "no components and no jobs returns nil",
			ra: &radixv1.RadixApplication{
				Spec: radixv1.RadixApplicationSpec{
					Environments: []radixv1.Environment{{Name: "dev"}},
				},
			},
			expected: nil,
		},
		{
			name: "no environments with components returns empty map",
			ra: &radixv1.RadixApplication{
				Spec: radixv1.RadixApplicationSpec{
					Components: []radixv1.RadixComponent{{Name: "frontend"}},
				},
			},
			expected: model.EnvironmentComponentNames{},
		},
		{
			name: "single component single environment",
			ra: &radixv1.RadixApplication{
				Spec: radixv1.RadixApplicationSpec{
					Components:   []radixv1.RadixComponent{{Name: "frontend"}},
					Environments: []radixv1.Environment{{Name: "dev"}},
				},
			},
			expected: model.EnvironmentComponentNames{
				"dev": {"frontend"},
			},
		},
		{
			name: "multiple components multiple environments",
			ra: &radixv1.RadixApplication{
				Spec: radixv1.RadixApplicationSpec{
					Components:   []radixv1.RadixComponent{{Name: "frontend"}, {Name: "backend"}},
					Environments: []radixv1.Environment{{Name: "dev"}, {Name: "prod"}},
				},
			},
			expected: model.EnvironmentComponentNames{
				"dev":  {"frontend", "backend"},
				"prod": {"frontend", "backend"},
			},
		},
		{
			name: "only job components",
			ra: &radixv1.RadixApplication{
				Spec: radixv1.RadixApplicationSpec{
					Jobs:         []radixv1.RadixJobComponent{{Name: "compute"}},
					Environments: []radixv1.Environment{{Name: "dev"}},
				},
			},
			expected: model.EnvironmentComponentNames{
				"dev": {"compute"},
			},
		},
		{
			name: "components and job components are merged",
			ra: &radixv1.RadixApplication{
				Spec: radixv1.RadixApplicationSpec{
					Components:   []radixv1.RadixComponent{{Name: "frontend"}},
					Jobs:         []radixv1.RadixJobComponent{{Name: "compute"}},
					Environments: []radixv1.Environment{{Name: "dev"}, {Name: "prod"}},
				},
			},
			expected: model.EnvironmentComponentNames{
				"dev":  {"frontend", "compute"},
				"prod": {"frontend", "compute"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getComponentNames(tt.ra)
			assert.Equal(t, tt.expected, result)
		})
	}
}

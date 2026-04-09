package subpipeline_test

import (
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/internal/subpipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

func TestObjectParamSpec(t *testing.T) {
	type validParams struct {
		GitSSHURL string `propname:"gitSSHUrl"`
		GitCommit string `propname:"gitCommit"`
		Other     string
	}

	type invalidParams struct {
		Count int `propname:"count"`
	}

	type validPointerParams struct {
		Key string `propname:"key"`
	}

	tests := map[string]struct {
		paramName           string
		obj                 any
		expectedSpec        pipelinev1.ParamSpec
		expectedErrorString string
	}{
		"returns object param spec for tagged string fields": {
			paramName: "radix",
			obj: validParams{
				GitSSHURL: "git@example.com:repo.git",
				GitCommit: "abc123",
				Other:     "ignored",
			},
			expectedSpec: pipelinev1.ParamSpec{
				Name: "radix",
				Type: pipelinev1.ParamTypeObject,
				Properties: map[string]pipelinev1.PropertySpec{
					"gitSSHUrl": {Type: pipelinev1.ParamTypeString},
					"gitCommit": {Type: pipelinev1.ParamTypeString},
				},
			},
		},
		"accepts pointer to struct": {
			paramName: "metadata",
			obj: &validPointerParams{
				Key: "value",
			},
			expectedSpec: pipelinev1.ParamSpec{
				Name: "metadata",
				Type: pipelinev1.ParamTypeObject,
				Properties: map[string]pipelinev1.PropertySpec{
					"key": {Type: pipelinev1.ParamTypeString},
				},
			},
		},
		"returns error for nil input": {
			paramName:           "radix",
			obj:                 nil,
			expectedErrorString: "input cannot be nil",
		},
		"returns error for nil pointer input": {
			paramName:           "radix",
			obj:                 (*validParams)(nil),
			expectedErrorString: "input cannot be nil pointer",
		},
		"returns error for non-struct input": {
			paramName:           "radix",
			obj:                 "not-a-struct",
			expectedErrorString: "input must be struct or pointer to struct",
		},
		"returns error when tagged field is not string": {
			paramName:           "radix",
			obj:                 invalidParams{Count: 3},
			expectedErrorString: "field Count has prop tag but is not string",
		},
		"returns empty properties when no prop tags exist": {
			paramName: "radix",
			obj: struct {
				Value string
			}{Value: "x"},
			expectedSpec: pipelinev1.ParamSpec{
				Name:       "radix",
				Type:       pipelinev1.ParamTypeObject,
				Properties: map[string]pipelinev1.PropertySpec{},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			actualSpec, err := subpipeline.ObjectParamSpec(tt.paramName, tt.obj)

			if tt.expectedErrorString != "" {
				require.Error(t, err)
				assert.EqualError(t, err, tt.expectedErrorString)
				assert.Equal(t, pipelinev1.ParamSpec{}, actualSpec)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedSpec, actualSpec)
		})
	}
}

func TestObjectParamReference(t *testing.T) {
	type validParams struct {
		GitSSHURL string `propname:"gitSSHUrl"`
		GitCommit string `propname:"gitCommit"`
		Other     string
	}

	type invalidParams struct {
		Count int `propname:"count"`
	}

	type pointerParams struct {
		Key string `propname:"key"`
	}

	tests := map[string]struct {
		paramName           string
		obj                 any
		reference           pipelinev1.ParamSpec
		expectedParam       pipelinev1.Param
		expectedErrorString string
	}{
		"returns error when reference is not object type": {
			paramName: "source",
			obj: validParams{
				GitSSHURL: "git@example.com:repo.git",
				GitCommit: "abc123",
			},
			reference: pipelinev1.ParamSpec{
				Name: "radix",
				Type: pipelinev1.ParamTypeString,
			},
			expectedErrorString: "reference param must be of type object",
		},
		"returns error for nil input": {
			paramName: "source",
			obj:       nil,
			reference: pipelinev1.ParamSpec{
				Name: "radix",
				Type: pipelinev1.ParamTypeObject,
				Properties: map[string]pipelinev1.PropertySpec{
					"gitSSHUrl": {Type: pipelinev1.ParamTypeString},
				},
			},
			expectedErrorString: "input cannot be nil",
		},
		"returns error when tagged field is not string": {
			paramName: "source",
			obj:       invalidParams{Count: 1},
			reference: pipelinev1.ParamSpec{
				Name: "radix",
				Type: pipelinev1.ParamTypeObject,
				Properties: map[string]pipelinev1.PropertySpec{
					"count": {Type: pipelinev1.ParamTypeString},
				},
			},
			expectedErrorString: "field Count has prop tag but is not string",
		},
		"returns object param with references for matching properties only": {
			paramName: "source",
			obj: validParams{
				GitSSHURL: "git@example.com:repo.git",
				GitCommit: "abc123",
				Other:     "ignored",
			},
			reference: pipelinev1.ParamSpec{
				Name: "radix",
				Type: pipelinev1.ParamTypeObject,
				Properties: map[string]pipelinev1.PropertySpec{
					"gitSSHUrl": {Type: pipelinev1.ParamTypeString},
					"gitCommit": {Type: pipelinev1.ParamTypeString},
					"notUsed":   {Type: pipelinev1.ParamTypeString},
				},
			},
			expectedParam: pipelinev1.Param{
				Name:  "source",
				Value: *pipelinev1.NewObject(map[string]string{"gitSSHUrl": "$(params.radix.gitSSHUrl)", "gitCommit": "$(params.radix.gitCommit)"}),
			},
		},
		"accepts pointer to struct": {
			paramName: "source",
			obj: &pointerParams{
				Key: "value",
			},
			reference: pipelinev1.ParamSpec{
				Name: "metadata",
				Type: pipelinev1.ParamTypeObject,
				Properties: map[string]pipelinev1.PropertySpec{
					"key": {Type: pipelinev1.ParamTypeString},
				},
			},
			expectedParam: pipelinev1.Param{
				Name:  "source",
				Value: *pipelinev1.NewObject(map[string]string{"key": "$(params.metadata.key)"}),
			},
		},
		"returns empty object when no tagged properties match reference": {
			paramName: "source",
			obj: validParams{
				GitSSHURL: "git@example.com:repo.git",
				GitCommit: "abc123",
			},
			reference: pipelinev1.ParamSpec{
				Name: "radix",
				Type: pipelinev1.ParamTypeObject,
				Properties: map[string]pipelinev1.PropertySpec{
					"another": {Type: pipelinev1.ParamTypeString},
				},
			},
			expectedParam: pipelinev1.Param{
				Name:  "source",
				Value: *pipelinev1.NewObject(map[string]string{}),
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			actualParam, err := subpipeline.ObjectParamReference(tt.paramName, tt.obj, tt.reference)

			if tt.expectedErrorString != "" {
				require.Error(t, err)
				assert.EqualError(t, err, tt.expectedErrorString)
				assert.Equal(t, pipelinev1.Param{}, actualParam)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedParam, actualParam)
		})
	}
}

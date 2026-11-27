package internal_test

import (
	"errors"
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/steps/preparepipeline/internal"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/stretchr/testify/require"
)

const (
	sampleAppRadixConfigFileName = "/radixconfig.yaml"
	sampleAppWorkspace           = "../../internal/test/testdata"
)

func Test_LoadRadixApplication(t *testing.T) {
	const (
		appName = "test-app"
	)
	type scenario struct {
		name              string
		registeredAppName string
		expectedError     error
	}
	scenarios := []scenario{
		{
			name:              "RadixApplication loaded",
			registeredAppName: appName,
			expectedError:     nil,
		},
		{
			name:              "RadixApplication not loaded",
			registeredAppName: "not-matching-app-name",
			expectedError:     errors.New("the application name test-app in the radixconfig file does not match the registered application name not-matching-app-name"),
		},
	}

	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			pipelineInfo := &model.PipelineInfo{
				PipelineArguments: model.PipelineArguments{
					AppName:         ts.registeredAppName,
					RadixConfigFile: sampleAppRadixConfigFileName,
					GitWorkspace:    sampleAppWorkspace,
				},
			}
			radixConfigReader := internal.NewRadixConfigReader()
			_, err := radixConfigReader.Read(pipelineInfo)
			if ts.expectedError == nil {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, ts.expectedError.Error())
			}
		})
	}
}

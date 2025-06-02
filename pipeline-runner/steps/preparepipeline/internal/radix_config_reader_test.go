package internal_test

import (
	"context"
	"errors"
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/steps/preparepipeline/internal"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclientfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			rxClient := radixclientfake.NewSimpleClientset()
			_, err := rxClient.RadixV1().RadixRegistrations().Create(context.Background(), &radixv1.RadixRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: appName},
			}, metav1.CreateOptions{})
			require.NoError(t, err)
			pipelineInfo := &model.PipelineInfo{
				PipelineArguments: model.PipelineArguments{
					AppName:         ts.registeredAppName,
					RadixConfigFile: sampleAppRadixConfigFileName,
					GitWorkspace:    sampleAppWorkspace,
				},
			}
			radixConfigReader := internal.NewRadixConfigReader(rxClient)
			_, err = radixConfigReader.Read(pipelineInfo)
			if ts.expectedError == nil {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, ts.expectedError.Error())
			}
		})
	}
}

package preparepipeline

import (
	"context"
	"errors"
	"github.com/equinor/radix-operator/pipeline-runner/steps/runpipeline"
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/internal/wait"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/utils/test"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	sampleApp = "./testdata/radixconfig.yaml"
)

func Test_LoadRadixApplication(t *testing.T) {
	type scenario struct {
		name              string
		registeredAppName string
		expectedError     error
	}
	scenarios := []scenario{
		{
			name:              "RadixApplication loaded",
			registeredAppName: sampleApp,
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
			mockCtrl := gomock.NewController(t)
			kubeClient, rxClient, tknClient := test.Setup()
			completionWaiter := wait.NewMockPipelineRunsCompletionWaiter(mockCtrl)
			completionWaiter.EXPECT().Wait(gomock.Any(), gomock.Any()).AnyTimes()
			_, err := rxClient.RadixV1().RadixRegistrations().Create(context.Background(), &radixv1.RadixRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: sampleApp},
			}, metav1.CreateOptions{})
			require.NoError(t, err)
			pipelineInfo := &model.PipelineInfo{
				PipelineArguments: model.PipelineArguments{
					AppName: sampleApp,
				},
			}
			ctx := NewPipelineContext(kubeClient, rxClient, tknClient, pipelineInfo, WithPipelineRunsWaiter(completionWaiter))

			err = ctx.ProcessRadixAppConfig()
			if ts.expectedError == nil {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, ts.expectedError.Error())
			}
		})
	}
}

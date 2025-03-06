package pipeline

import (
	"context"
	"errors"
	"testing"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-tekton/pkg/internal/wait"
	internalTest "github.com/equinor/radix-tekton/pkg/pipeline/internal/test"
	"github.com/equinor/radix-tekton/pkg/utils/test"
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
			registeredAppName: internalTest.AppName,
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
			envMock := internalTest.MockEnv(mockCtrl, ts.registeredAppName)
			envMock.EXPECT().GetRadixConfigFileName().Return(sampleApp).AnyTimes()
			kubeClient, rxClient, tknClient := test.Setup()
			completionWaiter := wait.NewMockPipelineRunsCompletionWaiter(mockCtrl)
			completionWaiter.EXPECT().Wait(gomock.Any(), gomock.Any()).AnyTimes()
			_, err := rxClient.RadixV1().RadixRegistrations().Create(context.Background(), &radixv1.RadixRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: internalTest.AppName},
			}, metav1.CreateOptions{})
			require.NoError(t, err)

			ctx := NewPipelineContext(kubeClient, rxClient, tknClient, envMock, WithPipelineRunsWaiter(completionWaiter))

			err = ctx.ProcessRadixAppConfig()
			if ts.expectedError == nil {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, ts.expectedError.Error())
			}
		})
	}
}

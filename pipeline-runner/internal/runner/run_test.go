package runner

import (
	"context"
	"fmt"
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	modelmock "github.com/equinor/radix-operator/pipeline-runner/model/mock"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commonTest "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func createRadixJob(appName, jobName string) *v1.RadixJob {
	return &v1.RadixJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: utils.GetAppNamespace(appName),
		},
		Spec: v1.RadixJobSpec{
			AppName:      appName,
			PipeLineType: v1.Deploy,
		},
	}
}

func getPipelineRunStatus(t *testing.T, fakeClient client.Client, appName, jobName string) *v1.RadixJobPipelineRunStatus {
	t.Helper()
	rj := &v1.RadixJob{}
	err := fakeClient.Get(context.Background(), client.ObjectKey{Namespace: utils.GetAppNamespace(appName), Name: jobName}, rj)
	require.NoError(t, err)
	return rj.Status.PipelineRunStatus
}

func TestRun_PipelineRunStatusUpdated(t *testing.T) {
	useBuildCache := true

	tests := []struct {
		name               string
		stepRunFunc        func(context.Context, *model.PipelineInfo) error
		pipelineArgs       model.PipelineArguments
		expectRunError     bool
		expectedStatus     v1.RadixJobCondition
		expectedBuildKit   bool
		expectedBuildCache bool
	}{
		{
			name:               "step succeeds",
			stepRunFunc:        func(_ context.Context, _ *model.PipelineInfo) error { return nil },
			expectedStatus:     v1.JobSucceeded,
			expectedBuildKit:   true,
			expectedBuildCache: true,
		},
		{
			name:               "step fails",
			stepRunFunc:        func(_ context.Context, _ *model.PipelineInfo) error { return fmt.Errorf("build error") },
			expectRunError:     true,
			expectedStatus:     v1.JobFailed,
			expectedBuildKit:   true,
			expectedBuildCache: true,
		},
		{
			name: "stop pipeline",
			stepRunFunc: func(_ context.Context, info *model.PipelineInfo) error {
				info.StopPipeline = true
				info.StopPipelineMessage = "no changes"
				return nil
			},
			expectedStatus:     v1.JobStoppedNoChanges,
			expectedBuildKit:   true,
			expectedBuildCache: true,
		},
		{
			name:               "buildkit and cache info",
			stepRunFunc:        func(_ context.Context, _ *model.PipelineInfo) error { return nil },
			pipelineArgs:       model.PipelineArguments{OverrideUseBuildCache: &useBuildCache},
			expectedStatus:     v1.JobSucceeded,
			expectedBuildKit:   true,
			expectedBuildCache: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const appName, jobName = "myapp", "myjob"
			ctrl := gomock.NewController(t)
			fakeClient := commonTest.CreateClient(createRadixJob(appName, jobName))

			step := modelmock.NewMockStep(ctrl)
			step.EXPECT().Run(gomock.Any(), gomock.Any()).DoAndReturn(tt.stepRunFunc)
			step.EXPECT().SucceededMsg().Return("ok").AnyTimes()
			step.EXPECT().ErrorMsg(gomock.Any()).Return("error").AnyTimes()

			tt.pipelineArgs.JobName = jobName
			tt.pipelineArgs.AppName = appName

			cli := PipelineRunner{
				dynamicClient: fakeClient,
				appName:       appName,
				pipelineInfo: &model.PipelineInfo{
					Definition:        &pipeline.Definition{Type: v1.Deploy},
					PipelineArguments: tt.pipelineArgs,
					Steps:             []model.Step{step},
				},
			}

			err := cli.Run(context.Background())
			if tt.expectRunError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			status := getPipelineRunStatus(t, fakeClient, appName, jobName)
			require.NotNil(t, status)
			assert.Equal(t, tt.expectedStatus, status.Status)
			assert.Equal(t, tt.expectedBuildKit, status.UsedBuildKit)
			assert.Equal(t, tt.expectedBuildCache, status.UsedBuildCache)
		})
	}
}

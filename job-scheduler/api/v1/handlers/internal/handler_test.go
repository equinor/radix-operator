package internal

import (
	"context"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/job-scheduler/internal/test"
	"github.com/equinor/radix-operator/job-scheduler/models"
	"github.com/equinor/radix-operator/job-scheduler/models/common"
	v1 "github.com/equinor/radix-operator/job-scheduler/models/v1"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_CreateBatch(t *testing.T) {
	type scenario struct {
		name                   string
		batchDescription       common.BatchScheduleDescription
		expectedBatchType      kube.RadixBatchType
		expectedError          bool
		expectedRadixBatchSpec *radixv1.RadixBatchSpec
	}
	scenarios := []scenario{
		{
			name: "batch with multiple jobs",
			batchDescription: common.BatchScheduleDescription{JobScheduleDescriptions: []common.JobScheduleDescription{
				{
					JobId:                   "job1",
					Payload:                 "{}",
					RadixJobComponentConfig: common.RadixJobComponentConfig{},
				},
				{
					JobId:                   "job2",
					Payload:                 "{}",
					RadixJobComponentConfig: common.RadixJobComponentConfig{},
				},
			}},
			expectedBatchType: kube.RadixBatchTypeBatch,
			expectedError:     false,
			expectedRadixBatchSpec: &radixv1.RadixBatchSpec{
				Jobs: []radixv1.RadixBatchJob{
					{
						JobId: "job1",
					},
					{
						JobId: "job2",
					},
				},
			},
		},
		{
			name: "batch with one job",
			batchDescription: common.BatchScheduleDescription{JobScheduleDescriptions: []common.JobScheduleDescription{
				{
					JobId:                   "job1",
					Payload:                 "{}",
					RadixJobComponentConfig: common.RadixJobComponentConfig{},
				},
			}},
			expectedBatchType: kube.RadixBatchTypeBatch,
			expectedError:     false,
			expectedRadixBatchSpec: &radixv1.RadixBatchSpec{
				Jobs: []radixv1.RadixBatchJob{
					{
						JobId: "job1",
					},
				},
			},
		},
		{
			name: "batch with one job, no payload",
			batchDescription: common.BatchScheduleDescription{JobScheduleDescriptions: []common.JobScheduleDescription{
				{
					JobId:                   "job1",
					RadixJobComponentConfig: common.RadixJobComponentConfig{},
				},
			}},
			expectedBatchType: kube.RadixBatchTypeBatch,
			expectedError:     false,
			expectedRadixBatchSpec: &radixv1.RadixBatchSpec{
				Jobs: []radixv1.RadixBatchJob{
					{
						JobId: "job1",
					},
				},
			},
		},
		{
			name:              "batch with no job failed",
			batchDescription:  common.BatchScheduleDescription{JobScheduleDescriptions: []common.JobScheduleDescription{}},
			expectedBatchType: kube.RadixBatchTypeBatch,
			expectedError:     true,
		},
		{
			name: "single job",
			batchDescription: common.BatchScheduleDescription{JobScheduleDescriptions: []common.JobScheduleDescription{
				{
					JobId:                   "job1",
					Payload:                 "{}",
					RadixJobComponentConfig: common.RadixJobComponentConfig{},
				},
			}},
			expectedBatchType: kube.RadixBatchTypeJob,
			expectedError:     false,
			expectedRadixBatchSpec: &radixv1.RadixBatchSpec{
				Jobs: []radixv1.RadixBatchJob{
					{
						JobId: "job1",
					},
				},
			},
		},
		{
			name:              "single job with no job failed",
			batchDescription:  common.BatchScheduleDescription{JobScheduleDescriptions: []common.JobScheduleDescription{}},
			expectedBatchType: kube.RadixBatchTypeJob,
			expectedError:     true,
		},
		{
			name: "batch with commands and args",
			batchDescription: common.BatchScheduleDescription{
				JobScheduleDescriptions: []common.JobScheduleDescription{
					{
						JobId: "job1",
						RadixJobComponentConfig: common.RadixJobComponentConfig{
							Command: pointers.Ptr([]string{"some-command", "arg0"}),
						},
					},
					{
						JobId: "job2",
						RadixJobComponentConfig: common.RadixJobComponentConfig{
							Args: pointers.Ptr([]string{"job-arg1", "job-arg2"}),
						},
					},
					{
						JobId: "job3",
						RadixJobComponentConfig: common.RadixJobComponentConfig{
							Command: pointers.Ptr([]string{}),
							Args:    pointers.Ptr([]string{}),
						},
					},
				},
				DefaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
					Command: pointers.Ptr([]string{"some-command", "def-arg0"}),
					Args:    pointers.Ptr([]string{"def-arg1", "def-arg2"}),
				},
			},
			expectedBatchType: kube.RadixBatchTypeBatch,
			expectedError:     false,
			expectedRadixBatchSpec: &radixv1.RadixBatchSpec{
				Jobs: []radixv1.RadixBatchJob{
					{
						JobId:   "job1",
						Command: pointers.Ptr([]string{"some-command", "arg0"}),
						Args:    pointers.Ptr([]string{"def-arg1", "def-arg2"}),
					},
					{
						JobId:   "job2",
						Command: pointers.Ptr([]string{"some-command", "def-arg0"}),
						Args:    pointers.Ptr([]string{"job-arg1", "job-arg2"}),
					},
					{
						JobId:   "job3",
						Command: pointers.Ptr([]string{}),
						Args:    pointers.Ptr([]string{}),
					},
				},
			},
		},
		{
			name: "batch with runAsUser and a jobs, jobs without runAsUser should use batch, jobs with runAsUser should take precedence",
			batchDescription: common.BatchScheduleDescription{
				JobScheduleDescriptions: []common.JobScheduleDescription{
					{
						JobId: "job1",
						RadixJobComponentConfig: common.RadixJobComponentConfig{
							RunAsUser: pointers.Ptr(int64(1001)),
						},
					},
					{
						JobId: "job2",
						RadixJobComponentConfig: common.RadixJobComponentConfig{
							RunAsUser: pointers.Ptr(int64(1002)),
						},
					},
					{
						JobId:                   "job3",
						RadixJobComponentConfig: common.RadixJobComponentConfig{},
					},
					{
						JobId:                   "job4",
						RadixJobComponentConfig: common.RadixJobComponentConfig{},
					},
				},
				DefaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
					RunAsUser: pointers.Ptr(int64(1000)),
				},
			},
			expectedBatchType: kube.RadixBatchTypeBatch,
			expectedError:     false,
			expectedRadixBatchSpec: &radixv1.RadixBatchSpec{
				Jobs: []radixv1.RadixBatchJob{
					{
						JobId:     "job1",
						RunAsUser: pointers.Ptr(int64(1001)),
					},
					{
						JobId:     "job2",
						RunAsUser: pointers.Ptr(int64(1002)),
					},
					{
						JobId:     "job3",
						RunAsUser: pointers.Ptr(int64(1000)),
					},
					{
						JobId:     "job4",
						RunAsUser: pointers.Ptr(int64(1000)),
					},
				},
			},
		},
	}

	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			appJobComponent := "compute"
			radixDeployJobComponent := utils.NewDeployJobComponentBuilder().WithName(appJobComponent).BuildJobComponent()
			defer test.Cleanup(t)
			_, _, kubeUtil := test.SetupTest(t, "app", "qa", appJobComponent, "app-deploy-1", 1)
			env := models.NewEnv()

			h := &Handler{
				kubeUtil:                kubeUtil,
				env:                     env,
				radixDeployJobComponent: &radixDeployJobComponent,
			}
			params := test.GetTestParams()
			rd := params.ApplyRd(kubeUtil)
			assert.NotNil(t, rd)

			var err error
			var createdRadixBatch *v1.BatchStatus
			if ts.expectedBatchType == kube.RadixBatchTypeBatch {
				createdRadixBatch, err = h.CreateBatch(context.TODO(), &ts.batchDescription)
			} else {
				var jobScheduleDescription *common.JobScheduleDescription
				if len(ts.batchDescription.JobScheduleDescriptions) > 0 {
					jobScheduleDescription = &ts.batchDescription.JobScheduleDescriptions[0]
				}
				createdRadixBatch, err = h.CreateRadixBatchSingleJob(context.TODO(), jobScheduleDescription)
			}
			if ts.expectedError {
				assert.NotNil(t, err)
				return
			}
			assert.Nil(t, err)
			assert.NotNil(t, createdRadixBatch)

			scheduledBatchList, err := h.kubeUtil.RadixClient().RadixV1().RadixBatches(rd.Namespace).List(context.TODO(),
				metav1.ListOptions{})
			assert.Nil(t, err)

			assert.Len(t, scheduledBatchList.Items, 1)
			scheduledBatch := scheduledBatchList.Items[0]
			assert.Equal(t, params.JobComponentName,
				scheduledBatch.ObjectMeta.Labels[kube.RadixComponentLabel])
			assert.Equal(t, params.AppName,
				scheduledBatch.ObjectMeta.Labels[kube.RadixAppLabel])
			assert.Equal(t, len(ts.batchDescription.JobScheduleDescriptions), len(scheduledBatch.Spec.Jobs))
			assert.Equal(t, string(ts.expectedBatchType),
				scheduledBatch.ObjectMeta.Labels[kube.RadixBatchTypeLabel])
			assert.Len(t, scheduledBatch.Spec.Jobs, len(ts.expectedRadixBatchSpec.Jobs), "expected number of jobs in batch")
			for i, job := range ts.expectedRadixBatchSpec.Jobs {
				assert.Equal(t, job.JobId, scheduledBatch.Spec.Jobs[i].JobId, "expected job id in batch job #%d", i)
				if len(ts.batchDescription.JobScheduleDescriptions[i].Payload) > 0 {
					assert.NotEmpty(t, scheduledBatch.Spec.Jobs[i].PayloadSecretRef, "expected payload secret ref in batch job #%d", i)
				} else {
					assert.Empty(t, scheduledBatch.Spec.Jobs[i].PayloadSecretRef, "expected no payload secret ref in batch job #%d", i)
				}
				if ts.expectedRadixBatchSpec.Jobs[i].Command != nil {
					if assert.NotNil(t, scheduledBatch.Spec.Jobs[i].Command, "expected command in batch job #%d", i) {
						assert.ElementsMatch(t, *ts.expectedRadixBatchSpec.Jobs[i].Command, *scheduledBatch.Spec.Jobs[i].Command, "mismatched command in batch job #%d", i)
					}
				} else {
					assert.Nil(t, scheduledBatch.Spec.Jobs[i].Command, "expected no command in batch job #%d", i)
				}
				if ts.expectedRadixBatchSpec.Jobs[i].Args != nil {
					if assert.NotNil(t, scheduledBatch.Spec.Jobs[i].Args, "expected args in batch job #%d", i) {
						assert.ElementsMatch(t, *ts.expectedRadixBatchSpec.Jobs[i].Args, *scheduledBatch.Spec.Jobs[i].Args, "mismatched args in batch job #%d", i)
					}
				} else {
					assert.Nil(t, scheduledBatch.Spec.Jobs[i].Args, "expected no args in batch job #%d", i)
				}
				if ts.expectedRadixBatchSpec.Jobs[i].RunAsUser != nil {
					if assert.NotNil(t, scheduledBatch.Spec.Jobs[i].RunAsUser, "expected runAsUser in batch job #%d", i) {
						assert.Equal(t, *ts.expectedRadixBatchSpec.Jobs[i].RunAsUser, *scheduledBatch.Spec.Jobs[i].RunAsUser, "mismatched runAsUser in batch job #%d", i)
					}
				} else {
					assert.Nil(t, scheduledBatch.Spec.Jobs[i].RunAsUser, "expected no runAsUser in batch job #%d", i)
				}
			}
		})
	}
}

func Test_MergeJobDescriptionWithDefaultJobDescription(t *testing.T) {
	tests := map[string]struct {
		defaultRadixJobComponentConfig  *common.RadixJobComponentConfig
		jobScheduleDescription          *common.JobScheduleDescription
		expectedRadixJobComponentConfig *common.RadixJobComponentConfig
	}{
		"Resources merged from job and default spec": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Resources: &common.Resources{
					Limits: common.ResourceList{
						"cpu":    "20m",
						"memory": "20M",
					},
					Requests: common.ResourceList{
						"cpu": "10m",
					},
				},
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					Resources: &common.Resources{
						Limits: common.ResourceList{
							"memory": "21M",
						},
						Requests: common.ResourceList{
							"memory": "10M",
							"cpu":    "11m",
						},
					},
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Resources: &common.Resources{
					Limits: common.ResourceList{
						"cpu":    "20m",
						"memory": "21M",
					},
					Requests: common.ResourceList{
						"cpu":    "11m",
						"memory": "10M",
					},
				},
			},
		},
		"Resources from job spec only": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					Resources: &common.Resources{
						Limits: common.ResourceList{
							"memory": "20M",
						},
						Requests: common.ResourceList{
							"memory": "10M",
						},
					},
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Resources: &common.Resources{
					Limits: common.ResourceList{
						"memory": "20M",
					},
					Requests: common.ResourceList{
						"memory": "10M",
					},
				},
			},
		},
		"Resources from default spec only": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Resources: &common.Resources{
					Limits: common.ResourceList{
						"memory": "20M",
					},
					Requests: common.ResourceList{
						"memory": "10M",
					},
				},
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Resources: &common.Resources{
					Limits: common.ResourceList{
						"memory": "20M",
					},
					Requests: common.ResourceList{
						"memory": "10M",
					},
				},
			},
		},
		"BackoffLimit from job spec": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				BackoffLimit: pointers.Ptr[int32](2000),
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					BackoffLimit: pointers.Ptr[int32](1000),
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				BackoffLimit: pointers.Ptr[int32](1000),
			},
		},
		"BackoffLimit from default spec": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				BackoffLimit: pointers.Ptr[int32](2000),
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				BackoffLimit: pointers.Ptr[int32](2000),
			},
		},
		"TimeLimitSeconds from job spec": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				TimeLimitSeconds: pointers.Ptr[int64](2000),
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					TimeLimitSeconds: pointers.Ptr[int64](1000),
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				TimeLimitSeconds: pointers.Ptr[int64](1000),
			},
		},
		"TimeLimitSeconds from default spec": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				TimeLimitSeconds: pointers.Ptr[int64](2000),
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				TimeLimitSeconds: pointers.Ptr[int64](2000),
			},
		},
		"FailurePolicy from job spec only": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				FailurePolicy: &common.FailurePolicy{
					Rules: []common.FailurePolicyRule{
						{
							Action: common.FailurePolicyRuleActionCount,
							OnExitCodes: common.FailurePolicyRuleOnExitCodes{
								Operator: common.FailurePolicyRuleOnExitCodesOpIn,
								Values:   []int32{1, 2, 3},
							},
						},
					},
				},
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					FailurePolicy: &common.FailurePolicy{
						Rules: []common.FailurePolicyRule{
							{
								Action: common.FailurePolicyRuleActionFailJob,
								OnExitCodes: common.FailurePolicyRuleOnExitCodes{
									Operator: common.FailurePolicyRuleOnExitCodesOpNotIn,
									Values:   []int32{0, 1},
								},
							},
						},
					},
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				FailurePolicy: &common.FailurePolicy{
					Rules: []common.FailurePolicyRule{
						{
							Action: common.FailurePolicyRuleActionFailJob,
							OnExitCodes: common.FailurePolicyRuleOnExitCodes{
								Operator: common.FailurePolicyRuleOnExitCodesOpNotIn,
								Values:   []int32{0, 1},
							},
						},
					},
				},
			},
		},
		"FailurePolicy from default spec": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				FailurePolicy: &common.FailurePolicy{
					Rules: []common.FailurePolicyRule{
						{
							Action: common.FailurePolicyRuleActionCount,
							OnExitCodes: common.FailurePolicyRuleOnExitCodes{
								Operator: common.FailurePolicyRuleOnExitCodesOpIn,
								Values:   []int32{1, 2, 3},
							},
						},
					},
				},
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				FailurePolicy: &common.FailurePolicy{
					Rules: []common.FailurePolicyRule{
						{
							Action: common.FailurePolicyRuleActionCount,
							OnExitCodes: common.FailurePolicyRuleOnExitCodes{
								Operator: common.FailurePolicyRuleOnExitCodesOpIn,
								Values:   []int32{1, 2, 3},
							},
						},
					},
				},
			},
		},
		"Image from job spec": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Image: "my-default-image:latest",
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					Image: "my-job-image:latest",
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Image: "my-job-image:latest",
			},
		},
		"No default image, image from job spec": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					Image: "my-job-image:latest",
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Image: "my-job-image:latest",
			},
		},
		"Image from default spec": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Image: "my-default-image:latest",
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Image: "my-default-image:latest",
			},
		},
		"Command and Args from job spec": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					Command: pointers.Ptr([]string{"sh", "-c"}),
					Args:    pointers.Ptr([]string{"echo hello"}),
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Command: pointers.Ptr([]string{"sh", "-c"}),
				Args:    pointers.Ptr([]string{"echo hello"}),
			},
		},
		"Command and Args from default spec": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Command: pointers.Ptr([]string{"sh", "-c"}),
				Args:    pointers.Ptr([]string{"echo hello"}),
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Command: pointers.Ptr([]string{"sh", "-c"}),
				Args:    pointers.Ptr([]string{"echo hello"}),
			},
		},
		"job command and args are not set": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{},
		},
		"job single command is set": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					Command: pointers.Ptr([]string{"bash"}),
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Command: pointers.Ptr([]string{"bash"}),
			},
		},
		"job command with arguments is set": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					Command: pointers.Ptr([]string{"sh", "-c", "echo hello"}),
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Command: pointers.Ptr([]string{"sh", "-c", "echo hello"}),
			},
		},
		"job command is set and args are set": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					Command: pointers.Ptr([]string{"sh", "-c"}),
					Args:    pointers.Ptr([]string{"echo hello"}),
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Command: pointers.Ptr([]string{"sh", "-c"}),
				Args:    pointers.Ptr([]string{"echo hello"}),
			},
		},
		"job only args are set": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					Args: pointers.Ptr([]string{"--verbose", "--output=json"}),
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Args: pointers.Ptr([]string{"--verbose", "--output=json"}),
			},
		},
		"job and component command and args are set, job takes precedence": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Command: pointers.Ptr([]string{"comp-cmd"}),
				Args:    pointers.Ptr([]string{"comp-arg1", "comp-arg2"}),
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					Command: pointers.Ptr([]string{"job-cmd"}),
					Args:    pointers.Ptr([]string{"job-arg1", "job-arg2"}),
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Command: pointers.Ptr([]string{"job-cmd"}),
				Args:    pointers.Ptr([]string{"job-arg1", "job-arg2"}),
			},
		},
		"job command set, component args set, job command takes precedence, args from component": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Args: pointers.Ptr([]string{"comp-arg1", "comp-arg2"}),
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					Command: pointers.Ptr([]string{"job-cmd"}),
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Command: pointers.Ptr([]string{"job-cmd"}),
				Args:    pointers.Ptr([]string{"comp-arg1", "comp-arg2"}),
			},
		},
		"job args set, component command set, job args take precedence, command from component": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Command: pointers.Ptr([]string{"comp-cmd"}),
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					Args: pointers.Ptr([]string{"job-arg1", "job-arg2"}),
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Command: pointers.Ptr([]string{"comp-cmd"}),
				Args:    pointers.Ptr([]string{"job-arg1", "job-arg2"}),
			},
		},
		"only component command and args set": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Command: pointers.Ptr([]string{"comp-cmd"}),
				Args:    pointers.Ptr([]string{"comp-arg1", "comp-arg2"}),
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Command: pointers.Ptr([]string{"comp-cmd"}),
				Args:    pointers.Ptr([]string{"comp-arg1", "comp-arg2"}),
			},
		},
		"job command set, component command and args set, job command takes precedence, args from component": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Command: pointers.Ptr([]string{"comp-cmd"}),
				Args:    pointers.Ptr([]string{"comp-arg1", "comp-arg2"}),
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					Command: pointers.Ptr([]string{"job-cmd"}),
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Command: pointers.Ptr([]string{"job-cmd"}),
				Args:    pointers.Ptr([]string{"comp-arg1", "comp-arg2"}),
			},
		},
		"job args set, component command and args set, job args take precedence, command from component": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Command: pointers.Ptr([]string{"comp-cmd"}),
				Args:    pointers.Ptr([]string{"comp-arg1", "comp-arg2"}),
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					Args: pointers.Ptr([]string{"job-arg1", "job-arg2"}),
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Command: pointers.Ptr([]string{"comp-cmd"}),
				Args:    pointers.Ptr([]string{"job-arg1", "job-arg2"}),
			},
		},
		"job command is empty array, default is set and should be empty": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Command: pointers.Ptr([]string{"default-cmd"}),
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					Command: pointers.Ptr([]string{}),
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Command: pointers.Ptr([]string{}),
			},
		},
		"job args is empty array, default is set and should be empty": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Args: pointers.Ptr([]string{"default-arg1", "default-arg2"}),
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					Args: pointers.Ptr([]string{}),
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Args: pointers.Ptr([]string{}),
			},
		},
		"job command and args are empty arrays, default is set and should be empty": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Command: pointers.Ptr([]string{"default-cmd"}),
				Args:    pointers.Ptr([]string{"default-arg1", "default-arg2"}),
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					Command: pointers.Ptr([]string{}),
					Args:    pointers.Ptr([]string{}),
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Command: pointers.Ptr([]string{}),
				Args:    pointers.Ptr([]string{}),
			},
		},
		"job runAsUser is set, component runAsUser is set, job should take precedence": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				RunAsUser: pointers.Ptr(int64(1002)),
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					RunAsUser: pointers.Ptr(int64(1003)),
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				RunAsUser: pointers.Ptr(int64(1003)),
			},
		},
		"job runAsUser is nil, component runAsUser is set, component should be used": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				RunAsUser: pointers.Ptr(int64(1002)),
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					RunAsUser: nil,
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				RunAsUser: pointers.Ptr(int64(1002)),
			},
		},
	}
	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			err := applyDefaultJobDescriptionProperties(test.jobScheduleDescription, test.defaultRadixJobComponentConfig)
			require.NoError(t, err)
			assert.EqualValues(t, *test.expectedRadixJobComponentConfig, test.jobScheduleDescription.RadixJobComponentConfig)
		})
	}
}

func Test_MergeRuntime(t *testing.T) {
	scenarios := map[string]struct {
		radixJobComponentConfig         common.RadixJobComponentConfig
		defaultRadixJobComponentConfig  common.RadixJobComponentConfig
		expectedRadixJobComponentConfig common.RadixJobComponentConfig
	}{
		"clear job Architecture when empty job runtime": {
			defaultRadixJobComponentConfig:  common.RadixJobComponentConfig{Runtime: &common.Runtime{Architecture: string(radixv1.RuntimeArchitectureAmd64)}},
			radixJobComponentConfig:         common.RadixJobComponentConfig{Runtime: &common.Runtime{}},
			expectedRadixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{}},
		},
		"clear job NodeType when empty job runtime": {
			defaultRadixJobComponentConfig:  common.RadixJobComponentConfig{Runtime: &common.Runtime{NodeType: pointers.Ptr("some-node-type")}},
			radixJobComponentConfig:         common.RadixJobComponentConfig{Runtime: &common.Runtime{}},
			expectedRadixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{}},
		},
		"preserves existing NodeType": {
			defaultRadixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureAmd64),
			}},
			radixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				NodeType: pointers.Ptr("gpu-nodes"),
			}},
			expectedRadixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				NodeType: pointers.Ptr("gpu-nodes"),
				// Architecture should be cleared
			}},
		},
		"preserves existing Architecture if NodeType not set": {
			defaultRadixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{}},
			radixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureArm64),
			}},
			expectedRadixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureArm64),
			}},
		},
		"preserves job Architecture if NodeType not set": {
			defaultRadixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureAmd64),
			}},
			radixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureArm64),
			}},
			expectedRadixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureArm64),
			}},
		},
		"preserves job Architecture when there is no default runtime": {
			defaultRadixJobComponentConfig: common.RadixJobComponentConfig{},
			radixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureArm64),
			}},
			expectedRadixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureArm64),
			}},
		},
		"sets job Architecture by default runtime": {
			defaultRadixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureArm64),
			}},
			radixJobComponentConfig: common.RadixJobComponentConfig{},
			expectedRadixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureArm64),
			}},
		},
		"no job Architecture if no default runtime and job runtime": {
			defaultRadixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureArm64),
			}},
			radixJobComponentConfig: common.RadixJobComponentConfig{},
			expectedRadixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureArm64),
			}},
		},
		"clears Architecture if both present after merge": {
			defaultRadixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureAmd64),
			}},
			radixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				NodeType: pointers.Ptr("edge-nodes"),
			}},
			expectedRadixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				NodeType: pointers.Ptr("edge-nodes"),
				// Architecture must be cleared
			}},
		},
		"clears default Architecture and NodeType if empty Runtime in job": {
			defaultRadixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureAmd64),
			}},
			radixJobComponentConfig:         common.RadixJobComponentConfig{Runtime: &common.Runtime{}},
			expectedRadixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{}},
		},
		"keeps default Architecture if nil Runtime in job": {
			defaultRadixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureArm64),
			}},
			radixJobComponentConfig: common.RadixJobComponentConfig{Runtime: nil},
			expectedRadixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureArm64),
			}},
		},
		"keeps default NodeType if nil Runtime in job": {
			defaultRadixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				NodeType: pointers.Ptr("some-node-type"),
			}},
			radixJobComponentConfig: common.RadixJobComponentConfig{Runtime: nil},
			expectedRadixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				NodeType: pointers.Ptr("some-node-type"),
			}},
		},
		"keeps job Architecture if nil default Runtime": {
			defaultRadixJobComponentConfig: common.RadixJobComponentConfig{Runtime: nil},
			radixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureArm64),
			}},
			expectedRadixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureArm64),
			}},
		},
		"keeps job NodeType if nil default Runtime": {
			defaultRadixJobComponentConfig: common.RadixJobComponentConfig{Runtime: nil},
			radixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				NodeType: pointers.Ptr("some-node-type"),
			}},
			expectedRadixJobComponentConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				NodeType: pointers.Ptr("some-node-type"),
			}},
		},
		"keeps nil job Runtime if nil default and job Runtime": {
			defaultRadixJobComponentConfig:  common.RadixJobComponentConfig{Runtime: nil},
			radixJobComponentConfig:         common.RadixJobComponentConfig{Runtime: nil},
			expectedRadixJobComponentConfig: common.RadixJobComponentConfig{Runtime: nil},
		},
	}

	for name, tt := range scenarios {
		t.Run(name, func(t *testing.T) {
			jobScheduleDescription := &common.JobScheduleDescription{RadixJobComponentConfig: tt.radixJobComponentConfig}
			err := applyDefaultJobDescriptionProperties(jobScheduleDescription,
				&tt.defaultRadixJobComponentConfig)
			require.NoError(t, err)
			assert.EqualValues(t, tt.expectedRadixJobComponentConfig, jobScheduleDescription.RadixJobComponentConfig)
		})
	}
}

func Test_MergeEnvVars(t *testing.T) {
	scenarios := map[string]struct {
		radixJobComponentConfig         common.RadixJobComponentConfig
		defaultRadixJobComponentConfig  common.RadixJobComponentConfig
		expectedRadixJobComponentConfig common.RadixJobComponentConfig
	}{
		"no default env-vars, no job env-vars": {
			defaultRadixJobComponentConfig:  common.RadixJobComponentConfig{},
			radixJobComponentConfig:         common.RadixJobComponentConfig{},
			expectedRadixJobComponentConfig: common.RadixJobComponentConfig{},
		},
		"no default env-vars, job env-vars": {
			defaultRadixJobComponentConfig:  common.RadixJobComponentConfig{},
			radixJobComponentConfig:         common.RadixJobComponentConfig{Variables: common.EnvVars{"VAR1": "value1", "VAR2": "value2"}},
			expectedRadixJobComponentConfig: common.RadixJobComponentConfig{Variables: common.EnvVars{"VAR1": "value1", "VAR2": "value2"}},
		},
		"default env-vars, no job env-vars": {
			defaultRadixJobComponentConfig:  common.RadixJobComponentConfig{Variables: common.EnvVars{"VAR1": "value1", "VAR2": "value2"}},
			radixJobComponentConfig:         common.RadixJobComponentConfig{},
			expectedRadixJobComponentConfig: common.RadixJobComponentConfig{Variables: common.EnvVars{"VAR1": "value1", "VAR2": "value2"}},
		},
		"default env-vars, job env-vars": {
			defaultRadixJobComponentConfig:  common.RadixJobComponentConfig{Variables: common.EnvVars{"VAR1": "value1", "VAR2": "value2"}},
			radixJobComponentConfig:         common.RadixJobComponentConfig{Variables: common.EnvVars{"VAR2": "value22", "VAR3": "value3"}},
			expectedRadixJobComponentConfig: common.RadixJobComponentConfig{Variables: common.EnvVars{"VAR1": "value1", "VAR2": "value22", "VAR3": "value3"}},
		},
	}

	for name, tt := range scenarios {
		t.Run(name, func(t *testing.T) {
			jobScheduleDescription := &common.JobScheduleDescription{RadixJobComponentConfig: tt.radixJobComponentConfig}
			err := applyDefaultJobDescriptionProperties(jobScheduleDescription,
				&tt.defaultRadixJobComponentConfig)
			require.NoError(t, err)
			if assert.Len(t, jobScheduleDescription.RadixJobComponentConfig.Variables, len(tt.expectedRadixJobComponentConfig.Variables), "should return expected number of environment variables") {
				for varName, varValue := range jobScheduleDescription.RadixJobComponentConfig.Variables {
					assert.Equal(t, tt.expectedRadixJobComponentConfig.Variables[varName], varValue, "should return expected environment variable for key %s", varName)
				}
			}

			assert.EqualValues(t, tt.expectedRadixJobComponentConfig, jobScheduleDescription.RadixJobComponentConfig)
		})
	}
}

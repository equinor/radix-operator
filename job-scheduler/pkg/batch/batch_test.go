package batch

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/equinor/radix-common/utils/numbers"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	testUtil "github.com/equinor/radix-operator/job-scheduler/internal/test"
	v1 "github.com/equinor/radix-operator/job-scheduler/models/v1"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type testProps struct {
	appName               string
	envName               string
	radixJobComponentName string
}

type jobStatusPhase map[string]radixv1.RadixBatchJobPhase
type testArgs struct {
	radixBatch          *radixv1.RadixBatch
	batchRadixDeploy    operatorUtils.DeploymentBuilder
	activeRadixDeploy   *operatorUtils.DeploymentBuilder
	expectedBatchStatus string
}

const (
	batchName1           = "batch1"
	batchId1             = "batchId1"
	batchName2           = "batch2"
	batchId2             = "batchId2"
	batchName3           = "batch3"
	batchId3             = "batchId3"
	jobName1             = "job1"
	jobName2             = "job2"
	jobName3             = "job3"
	jobName4             = "job4"
	radixDeploymentName1 = "any-deployment1"
	radixDeploymentName2 = "any-deployment2"
	radixDeploymentName3 = "any-deployment3"
)

var (
	now       = time.Now()
	yesterday = now.Add(time.Hour * -20)
	props     = testProps{
		appName:               "any-app",
		envName:               "any-env",
		radixJobComponentName: "any-job",
	}
)

func TestCopyRadixBatchOrJob(t *testing.T) {
	tests := []struct {
		name    string
		args    testArgs
		want    *v1.BatchStatus
		wantErr bool
	}{
		{
			name: "only deployment has no rules, no job statuses",
			args: testArgs{
				radixBatch: createRadixBatch(batchName1, batchId1, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
					radixv1.BatchConditionTypeActive, nil),
				batchRadixDeploy:    createRadixDeployJobComponent(radixDeploymentName1, props),
				expectedBatchStatus: "Waiting",
			},
		},
		{
			name: "only deployment has no rules, job statuses waiting, active",
			args: testArgs{
				radixBatch: createRadixBatch(batchName1, batchId1, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
					radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseWaiting, jobName2: radixv1.BatchJobPhaseActive}),
				batchRadixDeploy:    createRadixDeployJobComponent(radixDeploymentName1, props),
				expectedBatchStatus: "Waiting",
			},
		},
		{
			name: "only deployment has no rules, job statuses failed, succeeded",
			args: testArgs{
				radixBatch: createRadixBatch(batchName1, batchId1, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
					radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseFailed, jobName2: radixv1.BatchJobPhaseSucceeded}),
				batchRadixDeploy:    createRadixDeployJobComponent(radixDeploymentName1, props),
				expectedBatchStatus: "Waiting",
			},
		},
		{
			name: "only deployment, with rules, job statuses failed, succeeded",
			args: testArgs{
				radixBatch: createRadixBatch(batchName1, batchId1, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
					radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseFailed, jobName2: radixv1.BatchJobPhaseSucceeded}),
				batchRadixDeploy: createRadixDeployJobComponent(radixDeploymentName1, props,
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusFailed, radixv1.ConditionAny, radixv1.OperatorIn, radixv1.BatchJobPhaseFailed)),
				expectedBatchStatus: "Waiting",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer testUtil.Cleanup(t)
			radixClient, _, _ := testUtil.SetupTest(t, props.appName, props.envName, props.radixJobComponentName, radixDeploymentName1, 1)
			tt.args.batchRadixDeploy.WithActiveFrom(yesterday)
			var activeRadixDeployment *radixv1.RadixDeployment
			if tt.args.activeRadixDeploy != nil {
				tt.args.batchRadixDeploy.WithActiveTo(now)
				tt.args.batchRadixDeploy.WithCondition(radixv1.DeploymentInactive)
				activeRadixDeployment = (*tt.args.activeRadixDeploy).WithActiveFrom(now).WithCondition(radixv1.DeploymentActive).BuildRD()
				_, err := radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(props.appName, props.envName)).
					Create(context.Background(), activeRadixDeployment, metav1.CreateOptions{})
				require.NoError(t, err)
			} else {
				tt.args.batchRadixDeploy.WithCondition(radixv1.DeploymentActive)
			}
			batchRadixDeploy, err := radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(props.appName, props.envName)).
				Create(context.Background(), tt.args.batchRadixDeploy.BuildRD(), metav1.CreateOptions{})
			require.NoError(t, err)
			if activeRadixDeployment == nil {
				activeRadixDeployment = batchRadixDeploy
			}
			radixDeployJobComponent, ok := slice.FindFirst(activeRadixDeployment.Spec.Jobs, func(component radixv1.RadixDeployJobComponent) bool {
				return component.Name == props.radixJobComponentName
			})
			require.True(t, ok)

			createdRadixBatchStatus, err := CopyRadixBatchOrJob(context.Background(), radixClient, tt.args.radixBatch, "", &radixDeployJobComponent, radixDeploymentName1)
			if (err != nil) != tt.wantErr {
				t.Errorf("CopyRadixBatchOrJob() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.NotNil(t, createdRadixBatchStatus, "Status is nil")
			assert.Equal(t, tt.args.expectedBatchStatus, createdRadixBatchStatus.Status, "Status is not as expected")
		})
	}
}

func TestGetRadixBatchStatus(t *testing.T) {
	tests := []struct {
		name    string
		args    testArgs
		want    *v1.BatchStatus
		wantErr bool
	}{
		{
			name: "only deployment has no rules, no job statuses",
			args: testArgs{
				radixBatch: createRadixBatch(batchName1, batchId1, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
					radixv1.BatchConditionTypeWaiting, nil),
				batchRadixDeploy:    createRadixDeployJobComponent(radixDeploymentName1, props),
				expectedBatchStatus: "Waiting",
			},
		},
		{
			name: "only deployment has no rules, job statuses waiting, active",
			args: testArgs{
				radixBatch: createRadixBatch(batchName1, batchId1, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
					radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseWaiting, jobName2: radixv1.BatchJobPhaseActive}),
				batchRadixDeploy:    createRadixDeployJobComponent(radixDeploymentName1, props),
				expectedBatchStatus: "Active",
			},
		},
		{
			name: "only deployment has no rules, job statuses failed, succeeded",
			args: testArgs{
				radixBatch: createRadixBatch(batchName1, batchId1, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
					radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseFailed, jobName2: radixv1.BatchJobPhaseSucceeded}),
				batchRadixDeploy:    createRadixDeployJobComponent(radixDeploymentName1, props),
				expectedBatchStatus: "Active",
			},
		},
		{
			name: "only deployment, with only rule does not match, job statuses failed, succeeded",
			args: testArgs{
				radixBatch: createRadixBatch(batchName1, batchId1, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
					radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseWaiting, jobName2: radixv1.BatchJobPhaseSucceeded}),
				batchRadixDeploy: createRadixDeployJobComponent(radixDeploymentName1, props,
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusFailed, radixv1.ConditionAny, radixv1.OperatorIn, radixv1.BatchJobPhaseFailed)),
				expectedBatchStatus: "Active",
			},
		},
		{
			name: "only deployment, second rule matches",
			args: testArgs{
				radixBatch: createRadixBatch(batchName1, batchId1, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
					radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseStopped, jobName2: radixv1.BatchJobPhaseSucceeded}),
				batchRadixDeploy: createRadixDeployJobComponent(radixDeploymentName1, props,
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusFailed, radixv1.ConditionAny, radixv1.OperatorIn, radixv1.BatchJobPhaseFailed),
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusSucceeded, radixv1.ConditionAll, radixv1.OperatorIn, radixv1.BatchJobPhaseSucceeded, radixv1.BatchJobPhaseStopped)),
				expectedBatchStatus: "Succeeded",
			},
		},
		{
			name: "only deployment, with only rule any in matches, job statuses failed, succeeded",
			args: testArgs{
				radixBatch: createRadixBatch(batchName1, batchId1, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
					radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseFailed, jobName2: radixv1.BatchJobPhaseSucceeded}),
				batchRadixDeploy: createRadixDeployJobComponent(radixDeploymentName1, props,
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusFailed, radixv1.ConditionAny, radixv1.OperatorIn, radixv1.BatchJobPhaseFailed)),
				expectedBatchStatus: "Failed",
			},
		},
		{
			name: "only deployment, with rule all not-in matches",
			args: testArgs{
				radixBatch: createRadixBatch(batchName1, batchId1, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2, jobName3},
					radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseRunning, jobName2: radixv1.BatchJobPhaseActive}),
				batchRadixDeploy: createRadixDeployJobComponent(radixDeploymentName1, props,
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusRunning, radixv1.ConditionAll, radixv1.OperatorNotIn, radixv1.BatchJobPhaseWaiting, radixv1.BatchJobPhaseStopped, radixv1.BatchJobPhaseSucceeded, radixv1.BatchJobPhaseFailed),
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusWaiting, radixv1.ConditionAll, radixv1.OperatorNotIn, radixv1.BatchJobPhaseActive, radixv1.BatchJobPhaseRunning, radixv1.BatchJobPhaseStopped, radixv1.BatchJobPhaseSucceeded, radixv1.BatchJobPhaseFailed),
				),
				expectedBatchStatus: "Running",
			},
		},
		{
			name: "only deployment, with second rule all not-in matches",
			args: testArgs{
				radixBatch: createRadixBatch(batchName1, batchId1, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2, jobName3},
					radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseRunning, jobName2: radixv1.BatchJobPhaseActive}),
				batchRadixDeploy: createRadixDeployJobComponent(radixDeploymentName1, props,
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusWaiting, radixv1.ConditionAll, radixv1.OperatorNotIn, radixv1.BatchJobPhaseActive, radixv1.BatchJobPhaseRunning, radixv1.BatchJobPhaseStopped, radixv1.BatchJobPhaseSucceeded, radixv1.BatchJobPhaseFailed),
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusRunning, radixv1.ConditionAll, radixv1.OperatorNotIn, radixv1.BatchJobPhaseWaiting, radixv1.BatchJobPhaseStopped, radixv1.BatchJobPhaseSucceeded, radixv1.BatchJobPhaseFailed),
				),
				expectedBatchStatus: "Running",
			},
		},
		{
			name: "only deployment, with none of rules all not-in matches",
			args: testArgs{
				radixBatch: createRadixBatch(batchName1, batchId1, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2, jobName3},
					radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseRunning, jobName2: radixv1.BatchJobPhaseActive, jobName3: radixv1.BatchJobPhaseFailed}),
				batchRadixDeploy: createRadixDeployJobComponent(radixDeploymentName1, props,
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusWaiting, radixv1.ConditionAll, radixv1.OperatorNotIn, radixv1.BatchJobPhaseActive, radixv1.BatchJobPhaseRunning, radixv1.BatchJobPhaseStopped, radixv1.BatchJobPhaseSucceeded, radixv1.BatchJobPhaseFailed),
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusRunning, radixv1.ConditionAll, radixv1.OperatorNotIn, radixv1.BatchJobPhaseWaiting, radixv1.BatchJobPhaseStopped, radixv1.BatchJobPhaseSucceeded, radixv1.BatchJobPhaseFailed),
				),
				expectedBatchStatus: "Active",
			},
		},
		{
			name: "two deployments, with rule from active applied",
			args: testArgs{
				radixBatch: createRadixBatch(batchName1, batchId1, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2, jobName3},
					radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseRunning, jobName2: radixv1.BatchJobPhaseActive, jobName3: radixv1.BatchJobPhaseFailed, jobName4: radixv1.BatchJobPhaseSucceeded}),
				batchRadixDeploy: createRadixDeployJobComponent(radixDeploymentName1, props,
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusFailed, radixv1.ConditionAny, radixv1.OperatorIn, radixv1.BatchJobPhaseFailed)),
				activeRadixDeploy: pointers.Ptr(createRadixDeployJobComponent(radixDeploymentName2, props,
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusFailed, radixv1.ConditionAll, radixv1.OperatorIn, radixv1.BatchJobPhaseFailed),
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusRunning, radixv1.ConditionAny, radixv1.OperatorIn, radixv1.BatchJobPhaseRunning))),
				expectedBatchStatus: "Running",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer testUtil.Cleanup(t)
			radixClient, _, _ := testUtil.SetupTest(t, props.appName, props.envName, props.radixJobComponentName, radixDeploymentName1, 1)
			tt.args.batchRadixDeploy.WithActiveFrom(yesterday)
			var activeRadixDeployment *radixv1.RadixDeployment
			if tt.args.activeRadixDeploy != nil {
				tt.args.batchRadixDeploy.WithActiveTo(now)
				tt.args.batchRadixDeploy.WithCondition(radixv1.DeploymentInactive)
				activeRadixDeployment = (*tt.args.activeRadixDeploy).WithActiveFrom(now).WithCondition(radixv1.DeploymentActive).BuildRD()
				_, err := radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(props.appName, props.envName)).
					Create(context.Background(), activeRadixDeployment, metav1.CreateOptions{})
				require.NoError(t, err)
			} else {
				tt.args.batchRadixDeploy.WithCondition(radixv1.DeploymentActive)
			}
			batchRadixDeploy, err := radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(props.appName, props.envName)).
				Create(context.Background(), tt.args.batchRadixDeploy.BuildRD(), metav1.CreateOptions{})
			require.NoError(t, err)
			if activeRadixDeployment == nil {
				activeRadixDeployment = batchRadixDeploy
			}
			radixDeployJobComponent, ok := slice.FindFirst(activeRadixDeployment.Spec.Jobs, func(component radixv1.RadixDeployJobComponent) bool {
				return component.Name == props.radixJobComponentName
			})
			require.True(t, ok)

			actualBatchStatus := GetRadixBatchStatus(tt.args.radixBatch, &radixDeployJobComponent)
			assert.Equal(t, tt.args.expectedBatchStatus, actualBatchStatus.Status, "Status is not as expected")
		})
	}
}

func TestGetRadixBatchStatuses(t *testing.T) {
	type multiBatchArgs struct {
		radixBatches                 []*radixv1.RadixBatch
		batchRadixDeploymentBuilders map[string]operatorUtils.DeploymentBuilder
		activeRadixDeploymentName    string
		expectedBatchStatuses        map[string]string
	}
	tests := []struct {
		name        string
		batchesArgs multiBatchArgs
		want        *v1.BatchStatus
		wantErr     bool
	}{
		{
			name: "only deployment has no rules, no job statuses",
			batchesArgs: multiBatchArgs{
				radixBatches: []*radixv1.RadixBatch{
					createRadixBatch(batchName1, batchId1, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
						radixv1.BatchConditionTypeWaiting, nil),
					createRadixBatch(batchName2, batchId2, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
						radixv1.BatchConditionTypeWaiting, nil),
				},
				batchRadixDeploymentBuilders: map[string]operatorUtils.DeploymentBuilder{radixDeploymentName1: createRadixDeployJobComponent(radixDeploymentName1, props)},
				activeRadixDeploymentName:    radixDeploymentName1,
				expectedBatchStatuses: map[string]string{
					batchName1: "Waiting",
					batchName2: "Waiting"},
			},
		},
		{
			name: "only deployment has no rules, batch status is default",
			batchesArgs: multiBatchArgs{
				radixBatches: []*radixv1.RadixBatch{
					createRadixBatch(batchName1, batchId1, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
						radixv1.BatchConditionTypeWaiting, jobStatusPhase{jobName1: radixv1.BatchJobPhaseWaiting, jobName2: radixv1.BatchJobPhaseActive}),
					createRadixBatch(batchName2, batchId2, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
						radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseSucceeded, jobName2: radixv1.BatchJobPhaseRunning}),
					createRadixBatch(batchName3, batchId3, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
						radixv1.BatchConditionTypeCompleted, jobStatusPhase{jobName1: radixv1.BatchJobPhaseSucceeded, jobName2: radixv1.BatchJobPhaseFailed}),
				},
				batchRadixDeploymentBuilders: map[string]operatorUtils.DeploymentBuilder{radixDeploymentName1: createRadixDeployJobComponent(radixDeploymentName1, props)},
				activeRadixDeploymentName:    radixDeploymentName1,
				expectedBatchStatuses: map[string]string{
					batchName1: "Waiting",
					batchName2: "Active",
					batchName3: "Completed",
				},
			},
		},
		{
			name: "only deployment, with only rule does not match, batch status is default",
			batchesArgs: multiBatchArgs{
				radixBatches: []*radixv1.RadixBatch{
					createRadixBatch(batchName1, batchId1, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
						radixv1.BatchConditionTypeWaiting, jobStatusPhase{jobName1: radixv1.BatchJobPhaseWaiting, jobName2: radixv1.BatchJobPhaseWaiting}),
					createRadixBatch(batchName2, batchId2, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
						radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseSucceeded, jobName2: radixv1.BatchJobPhaseRunning}),
					createRadixBatch(batchName3, batchId3, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
						radixv1.BatchConditionTypeCompleted, jobStatusPhase{jobName1: radixv1.BatchJobPhaseSucceeded, jobName2: radixv1.BatchJobPhaseStopped}),
				},
				batchRadixDeploymentBuilders: map[string]operatorUtils.DeploymentBuilder{radixDeploymentName1: createRadixDeployJobComponent(radixDeploymentName1, props,
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusFailed, radixv1.ConditionAny, radixv1.OperatorIn, radixv1.BatchJobPhaseFailed))},
				activeRadixDeploymentName: radixDeploymentName1,
				expectedBatchStatuses: map[string]string{
					batchName1: "Waiting",
					batchName2: "Active",
					batchName3: "Completed",
				},
			},
		},
		{
			name: "only deployment, with only rule and it matches on two batches, third batch status is default",
			batchesArgs: multiBatchArgs{
				radixBatches: []*radixv1.RadixBatch{
					createRadixBatch(batchName1, batchId1, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
						radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseWaiting, jobName2: radixv1.BatchJobPhaseRunning, jobName3: radixv1.BatchJobPhaseSucceeded}),
					createRadixBatch(batchName2, batchId2, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
						radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseStopped, jobName2: radixv1.BatchJobPhaseRunning}),
					createRadixBatch(batchName3, batchId3, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
						radixv1.BatchConditionTypeCompleted, jobStatusPhase{jobName1: radixv1.BatchJobPhaseSucceeded, jobName2: radixv1.BatchJobPhaseFailed}),
				},
				batchRadixDeploymentBuilders: map[string]operatorUtils.DeploymentBuilder{radixDeploymentName1: createRadixDeployJobComponent(radixDeploymentName1, props,
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusRunning, radixv1.ConditionAny, radixv1.OperatorIn, radixv1.BatchJobPhaseRunning))},
				activeRadixDeploymentName: radixDeploymentName1,
				expectedBatchStatuses: map[string]string{
					batchName1: "Running",
					batchName2: "Running",
					batchName3: "Completed",
				},
			},
		},
		{
			name: "only deployment, multiple rules, first matching rule applied on two batches, third batch status is default",
			batchesArgs: multiBatchArgs{
				radixBatches: []*radixv1.RadixBatch{
					createRadixBatch(batchName1, batchId1, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
						radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseStopped, jobName2: radixv1.BatchJobPhaseRunning, jobName3: radixv1.BatchJobPhaseSucceeded}),
					createRadixBatch(batchName2, batchId2, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
						radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseStopped, jobName2: radixv1.BatchJobPhaseWaiting, jobName3: radixv1.BatchJobPhaseFailed}),
					createRadixBatch(batchName3, batchId3, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
						radixv1.BatchConditionTypeCompleted, jobStatusPhase{jobName1: radixv1.BatchJobPhaseSucceeded, jobName2: radixv1.BatchJobPhaseFailed}),
				},
				batchRadixDeploymentBuilders: map[string]operatorUtils.DeploymentBuilder{radixDeploymentName1: createRadixDeployJobComponent(radixDeploymentName1, props,
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusRunning, radixv1.ConditionAny, radixv1.OperatorIn, radixv1.BatchJobPhaseRunning),
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusStopped, radixv1.ConditionAny, radixv1.OperatorIn, radixv1.BatchJobPhaseStopped),
				)},
				activeRadixDeploymentName: radixDeploymentName1,
				expectedBatchStatuses: map[string]string{
					batchName1: "Running",
					batchName2: "Stopped",
					batchName3: "Completed",
				},
			},
		},
		{
			name: "only deployment, multiple rules, only first matching rule not-in applied on two batches, third batch status is default",
			batchesArgs: multiBatchArgs{
				radixBatches: []*radixv1.RadixBatch{
					createRadixBatch(batchName1, batchId1, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
						radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseWaiting, jobName2: radixv1.BatchJobPhaseRunning, jobName3: radixv1.BatchJobPhaseActive}),
					createRadixBatch(batchName2, batchId2, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
						radixv1.BatchConditionTypeCompleted, jobStatusPhase{jobName1: radixv1.BatchJobPhaseStopped, jobName2: radixv1.BatchJobPhaseSucceeded, jobName3: radixv1.BatchJobPhaseRunning, jobName4: radixv1.BatchJobPhaseWaiting}),
					createRadixBatch(batchName3, batchId3, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
						radixv1.BatchConditionTypeCompleted, jobStatusPhase{jobName1: radixv1.BatchJobPhaseFailed, jobName2: radixv1.BatchJobPhaseFailed}),
				},
				batchRadixDeploymentBuilders: map[string]operatorUtils.DeploymentBuilder{radixDeploymentName1: createRadixDeployJobComponent(radixDeploymentName1, props,
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusRunning, radixv1.ConditionAll, radixv1.OperatorNotIn, radixv1.BatchJobPhaseStopped, radixv1.BatchJobPhaseFailed, radixv1.BatchJobPhaseSucceeded),
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusSucceeded, radixv1.ConditionAny, radixv1.OperatorNotIn, radixv1.BatchJobPhaseFailed),
				)},
				activeRadixDeploymentName: radixDeploymentName1,
				expectedBatchStatuses: map[string]string{
					batchName1: "Running",
					batchName2: "Succeeded",
					batchName3: "Completed",
				},
			},
		},
		{
			name: "multiple deployments, used rules from active deployment",
			batchesArgs: multiBatchArgs{
				radixBatches: []*radixv1.RadixBatch{
					createRadixBatch(batchName1, batchId1, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
						radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseWaiting, jobName2: radixv1.BatchJobPhaseRunning, jobName3: radixv1.BatchJobPhaseActive}),
					createRadixBatch(batchName2, batchId2, props, kube.RadixBatchTypeBatch, radixDeploymentName2, []string{jobName1, jobName2},
						radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseWaiting, jobName2: radixv1.BatchJobPhaseRunning, jobName3: radixv1.BatchJobPhaseActive}),
					createRadixBatch(batchName3, batchId3, props, kube.RadixBatchTypeBatch, radixDeploymentName3, []string{jobName1, jobName2},
						radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseWaiting, jobName2: radixv1.BatchJobPhaseRunning, jobName3: radixv1.BatchJobPhaseActive}),
				},
				batchRadixDeploymentBuilders: map[string]operatorUtils.DeploymentBuilder{
					radixDeploymentName1: createRadixDeployJobComponent(radixDeploymentName1, props,
						createBatchStatusRule(radixv1.RadixBatchJobApiStatusActive, radixv1.ConditionAny, radixv1.OperatorIn, radixv1.BatchJobPhaseRunning, radixv1.BatchJobPhaseActive),
						createBatchStatusRule(radixv1.RadixBatchJobApiStatusFailed, radixv1.ConditionAny, radixv1.OperatorNotIn, radixv1.BatchJobPhaseRunning),
					),
					radixDeploymentName2: createRadixDeployJobComponent(radixDeploymentName2, props), // no rules
					radixDeploymentName3: createRadixDeployJobComponent(radixDeploymentName3, props,
						createBatchStatusRule(radixv1.RadixBatchJobApiStatusRunning, radixv1.ConditionAny, radixv1.OperatorIn, radixv1.BatchJobPhaseRunning, radixv1.BatchJobPhaseActive),
						createBatchStatusRule(radixv1.RadixBatchJobApiStatusFailed, radixv1.ConditionAny, radixv1.OperatorIn, radixv1.BatchJobPhaseFailed),
					),
				},
				activeRadixDeploymentName: radixDeploymentName3,
				expectedBatchStatuses: map[string]string{
					batchName1: "Running",
					batchName2: "Running",
					batchName3: "Running",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			someTime := now.Add(time.Hour * -20)
			defer testUtil.Cleanup(t)
			radixClient, _, _ := testUtil.SetupTest(t, props.appName, props.envName, props.radixJobComponentName, radixDeploymentName1, 1)
			var activeRadixDeployment *radixv1.RadixDeployment
			for radixDeploymentName, deploymentBuilder := range tt.batchesArgs.batchRadixDeploymentBuilders {
				deploymentBuilder.WithActiveFrom(someTime)
				if radixDeploymentName != tt.batchesArgs.activeRadixDeploymentName {
					someTime = someTime.Add(time.Hour)
					deploymentBuilder.WithActiveTo(someTime)
					deploymentBuilder.WithCondition(radixv1.DeploymentInactive)
				} else {
					deploymentBuilder.WithCondition(radixv1.DeploymentActive)
				}
				radixDeployment := deploymentBuilder.BuildRD()
				if radixDeploymentName == tt.batchesArgs.activeRadixDeploymentName {
					activeRadixDeployment = radixDeployment
				}
				_, err := radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(props.appName, props.envName)).
					Create(context.Background(), radixDeployment, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			require.NotNil(t, activeRadixDeployment, "active radix deployment is not set")
			radixDeployJobComponent, ok := slice.FindFirst(activeRadixDeployment.Spec.Jobs, func(component radixv1.RadixDeployJobComponent) bool {
				return component.Name == props.radixJobComponentName
			})
			require.True(t, ok)

			actualBatchStatuses := GetRadixBatchStatuses(tt.batchesArgs.radixBatches, &radixDeployJobComponent)
			batchStatusesMap := slice.Reduce(actualBatchStatuses, map[string]v1.BatchStatus{}, func(acc map[string]v1.BatchStatus, batchStatus v1.BatchStatus) map[string]v1.BatchStatus {
				acc[batchStatus.Name] = batchStatus
				return acc
			})
			radixBatchMap := slice.Reduce(tt.batchesArgs.radixBatches, make(map[string]*radixv1.RadixBatch), func(acc map[string]*radixv1.RadixBatch, batch *radixv1.RadixBatch) map[string]*radixv1.RadixBatch {
				acc[batch.Name] = batch
				return acc
			})
			for batchName, actualBatchStatus := range batchStatusesMap {
				assert.Equal(t, tt.batchesArgs.expectedBatchStatuses[batchName], actualBatchStatus.Status, "Status is not as expected for the batch %s", batchName)
				assert.Equal(t, radixBatchMap[batchName].Spec.BatchId, actualBatchStatus.BatchId, "Invalid or missing BatchId in the batch %s status", batchName)
			}
		})
	}
}

func TestDeleteRadixBatch(t *testing.T) {
	type deletionTestArgs struct {
		name               string
		existingRadixBatch *radixv1.RadixBatch
		radixBatchToDelete string
		expectedError      error
	}
	tests := []deletionTestArgs{
		{
			name:               "Radix batch exists",
			existingRadixBatch: createRadixBatch(batchName1, batchId1, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2}, radixv1.BatchConditionTypeWaiting, nil),
			radixBatchToDelete: batchName1,
		},
		{
			name:               "Radix batch does not exist, no error",
			existingRadixBatch: createRadixBatch(batchName1, batchId1, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2}, radixv1.BatchConditionTypeWaiting, nil),
			radixBatchToDelete: batchName2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer testUtil.Cleanup(t)
			radixClient, _, _ := testUtil.SetupTest(t, props.appName, props.envName, props.radixJobComponentName, radixDeploymentName1, 1)
			_, err := radixClient.RadixV1().RadixBatches(utils.GetEnvironmentNamespace(props.appName, props.envName)).Create(context.Background(), tt.existingRadixBatch, metav1.CreateOptions{})
			require.NoError(t, err)
			err = DeleteRadixBatch(context.Background(), radixClient, &radixv1.RadixBatch{ObjectMeta: metav1.ObjectMeta{Name: tt.radixBatchToDelete, Namespace: utils.GetEnvironmentNamespace(props.appName, props.envName)}})
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRestartRadixBatch(t *testing.T) {
	type deletionTestArgs struct {
		name                string
		existingRadixBatch  *radixv1.RadixBatch
		radixBatchToRestart *radixv1.RadixBatch
		expectedError       error
	}
	radixBatch1 := createRadixBatch(batchName1, batchId1, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2}, radixv1.BatchConditionTypeWaiting, nil)
	radixBatch2 := createRadixBatch(batchName2, batchId2, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2}, radixv1.BatchConditionTypeWaiting, nil)
	tests := []deletionTestArgs{
		{
			name:                "Radix batch exists",
			existingRadixBatch:  radixBatch1,
			radixBatchToRestart: radixBatch1,
		},
		{
			name:                "Radix batch does not exist, no error",
			existingRadixBatch:  radixBatch1,
			radixBatchToRestart: radixBatch2,
			expectedError:       errors.New("radixbatches.radix.equinor.com \"batch2\" not found"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer testUtil.Cleanup(t)
			radixClient, _, _ := testUtil.SetupTest(t, props.appName, props.envName, props.radixJobComponentName, radixDeploymentName1, 1)
			_, err := radixClient.RadixV1().RadixBatches(utils.GetEnvironmentNamespace(props.appName, props.envName)).Create(context.Background(), tt.existingRadixBatch, metav1.CreateOptions{})
			require.NoError(t, err)
			err = RestartRadixBatch(context.Background(), radixClient, tt.radixBatchToRestart)
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRestartRadixBatchJob(t *testing.T) {
	type deletionTestArgs struct {
		name                   string
		existingRadixBatch     *radixv1.RadixBatch
		radixBatchToRestart    *radixv1.RadixBatch
		expectedError          error
		radixBatchJobToRestart string
	}
	radixBatch1 := createRadixBatch(batchName1, batchId1, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2}, radixv1.BatchConditionTypeWaiting, nil)
	radixBatch2 := createRadixBatch(batchName2, batchId2, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2}, radixv1.BatchConditionTypeWaiting, nil)
	tests := []deletionTestArgs{
		{
			name:                   "Radix batch exists",
			existingRadixBatch:     radixBatch1,
			radixBatchToRestart:    radixBatch1,
			radixBatchJobToRestart: jobName1,
		},
		{
			name:                   "Radix batch does not exist",
			existingRadixBatch:     radixBatch1,
			radixBatchToRestart:    radixBatch2,
			radixBatchJobToRestart: jobName1,
			expectedError:          errors.New("radixbatches.radix.equinor.com \"batch2\" not found"),
		},
		{
			name:                   "Radix batch job does not exists",
			existingRadixBatch:     radixBatch1,
			radixBatchToRestart:    radixBatch1,
			radixBatchJobToRestart: jobName4,
			expectedError:          errors.New("job job4 not found"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer testUtil.Cleanup(t)
			radixClient, _, _ := testUtil.SetupTest(t, props.appName, props.envName, props.radixJobComponentName, radixDeploymentName1, 1)
			_, err := radixClient.RadixV1().RadixBatches(utils.GetEnvironmentNamespace(props.appName, props.envName)).Create(context.Background(), tt.existingRadixBatch, metav1.CreateOptions{})
			require.NoError(t, err)
			err = RestartRadixBatchJob(context.Background(), radixClient, tt.radixBatchToRestart, tt.radixBatchJobToRestart)
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStopRadixBatch(t *testing.T) {
	type deletionTestArgs struct {
		name               string
		existingRadixBatch *radixv1.RadixBatch
		radixBatchToStop   *radixv1.RadixBatch
		expectedError      error
	}
	radixBatch1 := createRadixBatch(batchName1, batchId1, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2}, radixv1.BatchConditionTypeWaiting, nil)
	radixBatch2 := createRadixBatch(batchName2, batchId2, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2}, radixv1.BatchConditionTypeWaiting, nil)
	tests := []deletionTestArgs{
		{
			name:               "Radix batch exists",
			existingRadixBatch: radixBatch1,
			radixBatchToStop:   radixBatch1,
		},
		{
			name:               "Radix batch does not exist",
			existingRadixBatch: radixBatch1,
			radixBatchToStop:   radixBatch2,
			expectedError:      errors.New("radixbatches.radix.equinor.com \"batch2\" not found"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer testUtil.Cleanup(t)
			radixClient, _, _ := testUtil.SetupTest(t, props.appName, props.envName, props.radixJobComponentName, radixDeploymentName1, 1)
			_, err := radixClient.RadixV1().RadixBatches(utils.GetEnvironmentNamespace(props.appName, props.envName)).Create(context.Background(), tt.existingRadixBatch, metav1.CreateOptions{})
			require.NoError(t, err)
			err = StopRadixBatch(context.Background(), radixClient, props.appName, props.envName, props.radixJobComponentName, tt.radixBatchToStop.GetName())
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStopRadixBatchJob(t *testing.T) {
	type deletionTestArgs struct {
		name                string
		existingRadixBatch  *radixv1.RadixBatch
		radixBatchToStop    *radixv1.RadixBatch
		expectedError       error
		radixBatchJobToStop string
	}
	radixBatch1 := createRadixBatch(batchName1, batchId1, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2}, radixv1.BatchConditionTypeWaiting, map[string]radixv1.RadixBatchJobPhase{jobName1: radixv1.BatchJobPhaseWaiting, jobName2: radixv1.BatchJobPhaseRunning})
	radixBatch2 := createRadixBatch(batchName2, batchId2, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2}, radixv1.BatchConditionTypeWaiting, nil)
	radixBatch3 := createRadixBatch(batchName2, batchId2, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2}, radixv1.BatchConditionTypeWaiting, map[string]radixv1.RadixBatchJobPhase{jobName1: radixv1.BatchJobPhaseStopped, jobName2: radixv1.BatchJobPhaseRunning})
	radixBatch4 := createRadixBatch(batchName2, batchId2, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2}, radixv1.BatchConditionTypeWaiting, map[string]radixv1.RadixBatchJobPhase{jobName1: radixv1.BatchJobPhaseSucceeded, jobName2: radixv1.BatchJobPhaseRunning})
	radixBatch5 := createRadixBatch(batchName2, batchId2, props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2}, radixv1.BatchConditionTypeWaiting, map[string]radixv1.RadixBatchJobPhase{jobName1: radixv1.BatchJobPhaseFailed, jobName2: radixv1.BatchJobPhaseRunning})
	tests := []deletionTestArgs{
		{
			name:                "Radix batch exists",
			existingRadixBatch:  radixBatch1,
			radixBatchToStop:    radixBatch1,
			radixBatchJobToStop: jobName1,
		},
		{
			name:                "Radix batch does not exist",
			existingRadixBatch:  radixBatch1,
			radixBatchToStop:    radixBatch2,
			radixBatchJobToStop: jobName1,
			expectedError:       errors.New("radixbatches.radix.equinor.com \"batch2\" not found"),
		},
		{
			name:                "Radix batch job does not exists",
			existingRadixBatch:  radixBatch1,
			radixBatchToStop:    radixBatch1,
			radixBatchJobToStop: jobName4,
			expectedError:       errors.New("radixbatches.job.radix.equinor.com \"job4\" not found"),
		},
		{
			name:                "Radix batch job has stopped status",
			existingRadixBatch:  radixBatch3,
			radixBatchToStop:    radixBatch3,
			radixBatchJobToStop: jobName1,
			expectedError:       errors.New("cannot stop the job job1 with the status Stopped in the batch batch2"),
		},
		{
			name:                "Radix batch job has succeeded status",
			existingRadixBatch:  radixBatch4,
			radixBatchToStop:    radixBatch4,
			radixBatchJobToStop: jobName1,
			expectedError:       errors.New("cannot stop the job job1 with the status Succeeded in the batch batch2"),
		},
		{
			name:                "Radix batch job has failed status",
			existingRadixBatch:  radixBatch5,
			radixBatchToStop:    radixBatch5,
			radixBatchJobToStop: jobName1,
			expectedError:       errors.New("cannot stop the job job1 with the status Failed in the batch batch2"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer testUtil.Cleanup(t)
			radixClient, _, _ := testUtil.SetupTest(t, props.appName, props.envName, props.radixJobComponentName, radixDeploymentName1, 1)
			_, err := radixClient.RadixV1().RadixBatches(utils.GetEnvironmentNamespace(props.appName, props.envName)).Create(context.Background(), tt.existingRadixBatch, metav1.CreateOptions{})
			require.NoError(t, err)
			err = StopRadixBatchJob(context.Background(), radixClient, props.appName, props.envName, props.radixJobComponentName, tt.radixBatchToStop.GetName(), tt.radixBatchJobToStop)
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func aRadixDeploymentWithComponentModifier(props testProps, radixDeploymentName string, m func(builder operatorUtils.DeployJobComponentBuilder) operatorUtils.DeployJobComponentBuilder) operatorUtils.DeploymentBuilder {
	builder := operatorUtils.NewDeploymentBuilder().
		WithAppName(props.appName).
		WithDeploymentName(radixDeploymentName).
		WithImageTag("imagetag").
		WithEnvironment(props.envName).
		WithJobComponent(m(operatorUtils.NewDeployJobComponentBuilder().
			WithName(props.radixJobComponentName).
			WithImage("radixdev.azurecr.io/job:imagetag").
			WithSchedulerPort(numbers.Int32Ptr(8080))))
	return builder
}

func createBatchStatusRule(batchStatus radixv1.RadixBatchJobApiStatus, condition radixv1.Condition, operator radixv1.Operator, jobPhases ...radixv1.RadixBatchJobPhase) radixv1.BatchStatusRule {
	return radixv1.BatchStatusRule{Condition: condition, BatchStatus: batchStatus, Operator: operator, JobStatuses: jobPhases}
}

func createRadixDeployJobComponent(radixDeploymentName string, props testProps, rules ...radixv1.BatchStatusRule) operatorUtils.DeploymentBuilder {
	return aRadixDeploymentWithComponentModifier(props, radixDeploymentName, func(builder operatorUtils.DeployJobComponentBuilder) operatorUtils.DeployJobComponentBuilder {
		return builder.WithBatchStatusRules(rules...)
	})
}

func createRadixBatch(batchName, batchId string, props testProps, radixBatchType kube.RadixBatchType, radixDeploymentName string, jobNames []string, batchStatus radixv1.RadixBatchConditionType, jobStatuses jobStatusPhase) *radixv1.RadixBatch {
	radixBatch := radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{
			Name:      batchName,
			Namespace: utils.GetEnvironmentNamespace(props.appName, props.envName),
			Labels: radixLabels.Merge(
				radixLabels.ForApplicationName(props.appName),
				radixLabels.ForComponentName(props.radixJobComponentName),
				radixLabels.ForBatchType(radixBatchType),
			),
		},
		Spec: radixv1.RadixBatchSpec{
			BatchId: batchId,
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: radixDeploymentName},
				Job:                  props.radixJobComponentName,
			},
		},
		Status: radixv1.RadixBatchStatus{
			Condition: radixv1.RadixBatchCondition{
				Type: batchStatus,
			},
		},
	}
	for _, jobName := range jobNames {
		radixBatch.Spec.Jobs = append(radixBatch.Spec.Jobs, radixv1.RadixBatchJob{
			Name: jobName,
		})
	}
	if jobStatuses != nil {
		for _, jobName := range jobNames {
			if jobPhase, ok := jobStatuses[jobName]; ok {
				radixBatch.Status.JobStatuses = append(radixBatch.Status.JobStatuses, radixv1.RadixBatchJobStatus{
					Name:  jobName,
					Phase: jobPhase,
				})
			}
		}
	}
	return &radixBatch
}

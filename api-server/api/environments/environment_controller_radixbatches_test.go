package environments

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/numbers"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/api-server/api/deployments/models"
	environmentModels "github.com/equinor/radix-operator/api-server/api/environments/models"
	"github.com/equinor/radix-operator/api-server/api/test"
	apiUtils "github.com/equinor/radix-operator/api-server/api/utils"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_GetJobs(t *testing.T) {
	batch1Name, batch2Name := "batch1", "batch2"
	namespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(utils.NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		utils.NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithJobComponents(utils.NewDeployJobComponentBuilder().WithName(anyJobName)).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	// Insert test data
	testData := []v1.RadixBatch{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batch1Name,
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
			},
			Spec: v1.RadixBatchSpec{Jobs: []v1.RadixBatchJob{{Name: "job1"}}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batch2Name,
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
			},
			Spec: v1.RadixBatchSpec{Jobs: []v1.RadixBatchJob{{Name: "job2"}}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "otherjobbatch",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName("othercomponent"), labels.ForBatchType(kube.RadixBatchTypeJob)),
			},
			Spec: v1.RadixBatchSpec{Jobs: []v1.RadixBatchJob{{Name: "anyjob"}}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "anybatch",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Spec: v1.RadixBatchSpec{Jobs: []v1.RadixBatchJob{{Name: "job3"}}},
		},
	}
	for _, rb := range testData {
		_, err := radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &rb, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	// Test get jobs for jobComponent1Name
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs", anyAppName, anyEnvironment, anyJobName))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	var actual []models.ScheduledJobSummary
	err = test.GetResponseBody(response, &actual)
	require.NoError(t, err)
	require.NoError(t, err)
	require.Len(t, actual, 2)
	actualMapped := slice.Map(actual, func(job models.ScheduledJobSummary) string {
		return job.Name
	})
	expected := []string{batch1Name + "-job1", batch2Name + "-job2"}
	assert.ElementsMatch(t, expected, actualMapped)
}

func Test_GetJobs_Status(t *testing.T) {
	namespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)
	type scenario struct {
		name           string
		jobStatus      *v1.RadixBatchJobStatus
		expectedStatus models.ScheduledBatchJobStatus
	}
	scenarios := []scenario{
		{
			name:           "no job status",
			expectedStatus: v1.RadixBatchJobApiStatusWaiting,
		},
		{
			name: "pod is pending, no phase",
			jobStatus: &v1.RadixBatchJobStatus{
				Name:                     "no1",
				RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{CreationTime: &metav1.Time{Time: time.Now()}, Phase: v1.PodPending}},
			},
			expectedStatus: v1.RadixBatchJobApiStatusWaiting,
		},
		{
			name: "pod is pending, phase is waiting",
			jobStatus: &v1.RadixBatchJobStatus{
				Name:                     "no1",
				Phase:                    v1.BatchJobPhaseWaiting,
				RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{CreationTime: &metav1.Time{Time: time.Now()}, Phase: v1.PodPending}},
			},
			expectedStatus: v1.RadixBatchJobApiStatusWaiting,
		},
		{
			name: "pod is running, phase is active",
			jobStatus: &v1.RadixBatchJobStatus{
				Name:                     "no1",
				Phase:                    v1.BatchJobPhaseActive,
				RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{CreationTime: &metav1.Time{Time: time.Now()}, Phase: v1.PodRunning}},
			},
			expectedStatus: v1.RadixBatchJobApiStatusActive,
		},
		{
			name: "pod is succeeded, phase is succeeded",
			jobStatus: &v1.RadixBatchJobStatus{
				Name:                     "no1",
				Phase:                    v1.BatchJobPhaseSucceeded,
				RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{CreationTime: &metav1.Time{Time: time.Now()}, Phase: v1.PodSucceeded}},
			},
			expectedStatus: v1.RadixBatchJobApiStatusSucceeded,
		},
		{
			name: "pod is failed, phase is failed",
			jobStatus: &v1.RadixBatchJobStatus{
				Name:                     "no1",
				Phase:                    v1.BatchJobPhaseFailed,
				RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{CreationTime: &metav1.Time{Time: time.Now()}, Phase: v1.PodFailed}},
			},
			expectedStatus: v1.RadixBatchJobApiStatusFailed,
		},
		{
			name: "pod is succeeded, phase is stopped",
			jobStatus: &v1.RadixBatchJobStatus{
				Name:                     "no1",
				Phase:                    v1.BatchJobPhaseStopped,
				RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{CreationTime: &metav1.Time{Time: time.Now()}, Phase: v1.PodSucceeded}},
			},
			expectedStatus: v1.RadixBatchJobApiStatusStopped,
		},
		{
			name:           "no pod status, phase is not defined",
			jobStatus:      &v1.RadixBatchJobStatus{Name: "not-defined"},
			expectedStatus: v1.RadixBatchJobApiStatusWaiting,
		},
	}
	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			// Setup
			commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, []EnvironmentHandlerOptions{})
			_, err := commonTestUtils.ApplyRegistration(utils.NewRegistrationBuilder().
				WithName(anyAppName))
			require.NoError(t, err)
			_, err = commonTestUtils.ApplyApplication(utils.NewRadixApplicationBuilder().
				WithAppName(anyAppName))
			require.NoError(t, err)
			_, err = commonTestUtils.ApplyDeployment(
				context.Background(),
				utils.NewDeploymentBuilder().
					WithAppName(anyAppName).
					WithEnvironment(anyEnvironment).
					WithJobComponents(utils.NewDeployJobComponentBuilder().WithName(anyJobName)).
					WithActiveFrom(time.Now()))
			require.NoError(t, err)

			// Insert test data
			batch := v1.RadixBatch{
				ObjectMeta: metav1.ObjectMeta{
					Name:   anyBatchName,
					Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
				},
				Spec: v1.RadixBatchSpec{
					Jobs: []v1.RadixBatchJob{{Name: "no1"}}},
				Status: v1.RadixBatchStatus{},
			}
			if ts.jobStatus != nil {
				batch.Status.JobStatuses = append(batch.Status.JobStatuses, *ts.jobStatus)
			}
			_, err = radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &batch, metav1.CreateOptions{})
			require.NoError(t, err)

			// Test get jobs for jobComponent1Name
			responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs", anyAppName, anyEnvironment, anyJobName))
			response := <-responseChannel
			assert.Equal(t, http.StatusOK, response.Code)
			var actual []models.ScheduledJobSummary
			err = test.GetResponseBody(response, &actual)
			require.NoError(t, err)
			assert.Len(t, actual, 1)
			assert.Equal(t, ts.expectedStatus, actual[0].Status)
		})
	}

}

func Test_GetBatch_JobsListStatus_StopIsTrue(t *testing.T) {
	namespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(utils.NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		utils.NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithJobComponents(utils.NewDeployJobComponentBuilder().WithName(anyJobName)).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	// Insert test data
	testData := []v1.RadixBatch{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   anyBatchName,
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Spec: v1.RadixBatchSpec{
				Jobs: []v1.RadixBatchJob{
					{Name: "no1", Stop: commonUtils.BoolPtr(true)},
					{Name: "no2", Stop: commonUtils.BoolPtr(true)},
					{Name: "no3", Stop: commonUtils.BoolPtr(true)},
					{Name: "no4", Stop: commonUtils.BoolPtr(true)},
					{Name: "no5", Stop: commonUtils.BoolPtr(true)},
					{Name: "no6", Stop: commonUtils.BoolPtr(true)},
					{Name: "no7", Stop: commonUtils.BoolPtr(true)},
				},
			},
			Status: v1.RadixBatchStatus{
				JobStatuses: []v1.RadixBatchJobStatus{
					{Name: "no2"},
					{Name: "no3", Phase: v1.BatchJobPhaseWaiting},
					{Name: "no4", Phase: v1.BatchJobPhaseActive},
					{Name: "no5", Phase: v1.BatchJobPhaseSucceeded},
					{Name: "no6", Phase: v1.BatchJobPhaseFailed},
					{Name: "no7", Phase: v1.BatchJobPhaseStopped},
					{Name: "not-defined"},
				},
			},
		},
	}
	for _, rb := range testData {
		_, err := radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &rb, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	// Test get jobs for jobComponent1Name
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/batches", anyAppName, anyEnvironment, anyJobName))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	var actual []models.ScheduledBatchSummary
	err = test.GetResponseBody(response, &actual)
	require.NoError(t, err)
	require.Len(t, actual, 1)
	type assertMapped struct {
		Name   string
		Status models.ScheduledBatchJobStatus
	}
	actualMapped := slice.Map(actual[0].JobList, func(job models.ScheduledJobSummary) assertMapped {
		return assertMapped{Name: job.Name, Status: job.Status}
	})
	expected := []assertMapped{
		{Name: anyBatchName + "-no1", Status: apiUtils.GetBatchJobStatusByJobApiStatus(v1.RadixBatchJobApiStatusStopping)},
		{Name: anyBatchName + "-no2", Status: apiUtils.GetBatchJobStatusByJobApiStatus(v1.RadixBatchJobApiStatusStopping)},
		{Name: anyBatchName + "-no3", Status: apiUtils.GetBatchJobStatusByJobApiStatus(v1.RadixBatchJobApiStatusStopping)},
		{Name: anyBatchName + "-no4", Status: apiUtils.GetBatchJobStatusByJobApiStatus(v1.RadixBatchJobApiStatusStopping)},
		{Name: anyBatchName + "-no5", Status: apiUtils.GetBatchJobStatusByJobApiStatus(v1.RadixBatchJobApiStatusSucceeded)},
		{Name: anyBatchName + "-no6", Status: apiUtils.GetBatchJobStatusByJobApiStatus(v1.RadixBatchJobApiStatusFailed)},
		{Name: anyBatchName + "-no7", Status: apiUtils.GetBatchJobStatusByJobApiStatus(v1.RadixBatchJobApiStatusStopped)},
	}
	assert.ElementsMatch(t, expected, actualMapped)
}

func Test_GetSingleJobs_Status_StopIsTrue(t *testing.T) {
	namespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)
	type scenario struct {
		name            string
		batchStatusType v1.RadixBatchConditionType
		jobStatus       *v1.RadixBatchJobStatus
		expectedStatus  models.ScheduledBatchJobStatus
	}
	scenarios := []scenario{
		{name: "No status",
			jobStatus:      nil,
			expectedStatus: v1.RadixBatchJobApiStatusStopping,
		},
		{
			name:            "No phase",
			batchStatusType: v1.BatchConditionTypeWaiting,
			jobStatus:       &v1.RadixBatchJobStatus{Name: "no1"},
			expectedStatus:  models.ScheduledBatchJobStatusStopping,
		},
		{
			name:            "In waiting",
			batchStatusType: v1.BatchConditionTypeWaiting,
			jobStatus:       &v1.RadixBatchJobStatus{Name: "no1", Phase: v1.BatchJobPhaseWaiting},
			expectedStatus:  models.ScheduledBatchJobStatusStopping,
		},
		{
			name:            "Active",
			batchStatusType: v1.BatchConditionTypeActive,
			jobStatus:       &v1.RadixBatchJobStatus{Name: "no1", Phase: v1.BatchJobPhaseActive},
			expectedStatus:  models.ScheduledBatchJobStatusStopping,
		},
		{
			name:            "Succeeded",
			batchStatusType: v1.BatchConditionTypeCompleted,
			jobStatus:       &v1.RadixBatchJobStatus{Name: "no1", Phase: v1.BatchJobPhaseSucceeded},
			expectedStatus:  models.ScheduledBatchJobStatusSucceeded,
		},
		{
			name:            "Failed",
			batchStatusType: v1.BatchConditionTypeCompleted,
			jobStatus:       &v1.RadixBatchJobStatus{Name: "no1", Phase: v1.BatchJobPhaseFailed},
			expectedStatus:  models.ScheduledBatchJobStatusFailed,
		},
		{
			name:            "Stopped",
			batchStatusType: v1.BatchConditionTypeCompleted,
			jobStatus:       &v1.RadixBatchJobStatus{Name: "no1", Phase: v1.BatchJobPhaseStopped},
			expectedStatus:  models.ScheduledBatchJobStatusStopped,
		},
	}
	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			// Setup
			commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
			_, err := commonTestUtils.ApplyRegistration(utils.NewRegistrationBuilder().
				WithName(anyAppName))
			require.NoError(t, err)
			_, err = commonTestUtils.ApplyApplication(utils.NewRadixApplicationBuilder().
				WithAppName(anyAppName))
			require.NoError(t, err)
			_, err = commonTestUtils.ApplyDeployment(
				context.Background(),
				utils.NewDeploymentBuilder().
					WithAppName(anyAppName).
					WithEnvironment(anyEnvironment).
					WithJobComponents(utils.NewDeployJobComponentBuilder().WithName(anyJobName)).
					WithActiveFrom(time.Now()))
			require.NoError(t, err)

			// Insert test data
			batch := v1.RadixBatch{
				ObjectMeta: metav1.ObjectMeta{
					Name:   anyBatchName,
					Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
				},
				Spec: v1.RadixBatchSpec{
					Jobs: []v1.RadixBatchJob{
						{Name: "no1", Stop: commonUtils.BoolPtr(true)},
					},
				},
				Status: v1.RadixBatchStatus{
					Condition: v1.RadixBatchCondition{
						Type: v1.BatchConditionTypeActive,
					},
				},
			}
			if ts.jobStatus != nil {
				batch.Status.JobStatuses = append(batch.Status.JobStatuses, *ts.jobStatus)
			}
			_, err = radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &batch, metav1.CreateOptions{})
			require.NoError(t, err)

			// Test get jobs for jobComponent1Name
			responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs", anyAppName, anyEnvironment, anyJobName))
			response := <-responseChannel
			assert.Equal(t, http.StatusOK, response.Code)
			var actual []models.ScheduledJobSummary
			err = test.GetResponseBody(response, &actual)
			require.NoError(t, err)
			assert.Len(t, actual, 1)
			assert.Equal(t, ts.expectedStatus, actual[0].Status)
		})

	}
}

func Test_GetJob(t *testing.T) {
	namespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(utils.NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		utils.NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithJobComponents(utils.NewDeployJobComponentBuilder().WithName(anyJobName)).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	// Insert test data
	testData := []v1.RadixBatch{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "job-batch1",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
			},
			Spec: v1.RadixBatchSpec{
				Jobs: []v1.RadixBatchJob{{Name: "job1"}, {Name: "job2"}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "job-batch2",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Spec: v1.RadixBatchSpec{
				Jobs: []v1.RadixBatchJob{{Name: "job1"}, {Name: "job2"}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "other-batch1",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName("other-component"), labels.ForBatchType(kube.RadixBatchTypeJob)),
			},
			Spec: v1.RadixBatchSpec{
				Jobs: []v1.RadixBatchJob{{Name: "job1"}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "other-batch2",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName("other-component"), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Spec: v1.RadixBatchSpec{
				Jobs: []v1.RadixBatchJob{{Name: "job1"}},
			},
		},
	}
	for _, rb := range testData {
		_, err := radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &rb, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	type scenarioSpec struct {
		Name    string
		JobName string
		Success bool
	}

	scenarios := []scenarioSpec{
		{Name: "get existing job1 from existing batch of type job", JobName: "job-batch1-job1", Success: true},
		{Name: "get existing job2 from existing batch of type job", JobName: "job-batch1-job2", Success: true},
		{Name: "get non-existing job3 from existing batch of type job", JobName: "job-batch1-job3", Success: false},
		{Name: "get existing job from existing batch of type job for other jobcomponent", JobName: "other-batch1-job1", Success: false},
		{Name: "get existing job1 from existing batch of type batch", JobName: "job-batch2-job1", Success: true},
		{Name: "get existing job2 from existing batch of type batch", JobName: "job-batch2-job2", Success: true},
		{Name: "get non-existing job3 from existing batch of type batch", JobName: "job-batch2-job3", Success: false},
		{Name: "get existing job from existing batch of type batch for other jobcomponent", JobName: "other-batch2-job1", Success: false},
		{Name: "get job from non-existing batch", JobName: "non-existing-batch-anyjob", Success: false},
	}

	for _, scenario := range scenarios {
		scenario := scenario
		t.Run(scenario.Name, func(t *testing.T) {
			responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/%s", anyAppName, anyEnvironment, anyJobName, scenario.JobName))
			response := <-responseChannel
			assert.Equal(t, scenario.Success, response.Code == http.StatusOK)
		})
	}
}

func Test_GetJob_AllProps(t *testing.T) {
	namespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)
	creationTime := metav1.NewTime(time.Date(2022, 1, 2, 3, 4, 5, 0, time.UTC))
	startTime := metav1.NewTime(time.Date(2022, 1, 2, 3, 4, 10, 0, time.UTC))
	podCreationTime := metav1.NewTime(time.Date(2022, 1, 2, 3, 4, 15, 0, time.UTC))
	endTime := metav1.NewTime(time.Date(2022, 1, 2, 3, 4, 15, 0, time.UTC))
	defaultBackoffLimit := numbers.Int32Ptr(3)

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(utils.NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		utils.NewDeploymentBuilder().
			WithDeploymentName(anyDeployment).
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithJobComponents(utils.NewDeployJobComponentBuilder().
				WithName(anyJobName).
				WithTimeLimitSeconds(numbers.Int64Ptr(123)).
				WithNodeGpu("gpu1").
				WithNodeGpuCount("2").
				WithRuntime(&v1.Runtime{Architecture: v1.RuntimeArchitectureArm64}).
				WithResource(map[string]string{"cpu": "50Mi", "memory": "250M"}, map[string]string{"cpu": "100Mi", "memory": "500M"}),
				utils.NewDeployJobComponentBuilder().
					WithName(anyJobName2).
					WithRuntime(&v1.Runtime{NodeType: pointers.Ptr(nodeType1)})).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	// HACK: Missing WithBackoffLimit in DeploymentBuild, so we have to update the RD manually
	rd, _ := radixClient.RadixV1().RadixDeployments(namespace).Get(context.Background(), anyDeployment, metav1.GetOptions{})
	rd.Spec.Jobs[0].BackoffLimit = defaultBackoffLimit
	_, err = radixClient.RadixV1().RadixDeployments(namespace).Update(context.Background(), rd, metav1.UpdateOptions{})
	require.NoError(t, err)

	// Insert test data
	testData := []v1.RadixBatch{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "job-batch1",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
			},
			Spec: v1.RadixBatchSpec{
				Jobs: []v1.RadixBatchJob{
					{
						Name: "job1",
					},
					{
						Name:             "job2",
						JobId:            "anyjobid",
						BackoffLimit:     numbers.Int32Ptr(5),
						TimeLimitSeconds: numbers.Int64Ptr(999),
						Resources: &v1.ResourceRequirements{
							Limits:   v1.ResourceList{"cpu": "101Mi", "memory": "501M"},
							Requests: v1.ResourceList{"cpu": "51Mi", "memory": "251M"},
						},
						Node: &v1.RadixNode{
							Gpu:      "gpu2",
							GpuCount: "3",
						},
					},
				},
				RadixDeploymentJobRef: v1.RadixDeploymentJobComponentSelector{
					Job:                  anyJobName,
					LocalObjectReference: v1.LocalObjectReference{Name: anyDeployment},
				},
			},
			Status: v1.RadixBatchStatus{
				Condition: v1.RadixBatchCondition{
					Type: v1.BatchConditionTypeCompleted,
				},
				JobStatuses: []v1.RadixBatchJobStatus{
					{
						Name:         "job1",
						Phase:        v1.BatchJobPhaseSucceeded,
						Message:      "anymessage",
						CreationTime: &creationTime,
						StartTime:    &startTime,
						EndTime:      &endTime,
						RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{
							CreationTime: &podCreationTime,
							Phase:        v1.PodSucceeded,
						}},
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "job3-batch1",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName2), labels.ForBatchType(kube.RadixBatchTypeJob)),
			},
			Spec: v1.RadixBatchSpec{
				Jobs: []v1.RadixBatchJob{
					{
						Name: "job6",
					},
					{
						Name:             "job7",
						JobId:            "anyjobid2",
						BackoffLimit:     numbers.Int32Ptr(5),
						TimeLimitSeconds: numbers.Int64Ptr(999),
						Runtime: &v1.Runtime{
							NodeType: pointers.Ptr(nodeType1),
						},
					},
				},
				RadixDeploymentJobRef: v1.RadixDeploymentJobComponentSelector{
					Job:                  anyJobName2,
					LocalObjectReference: v1.LocalObjectReference{Name: anyDeployment},
				},
			},
			Status: v1.RadixBatchStatus{
				Condition: v1.RadixBatchCondition{
					Type: v1.BatchConditionTypeCompleted,
				},
				JobStatuses: []v1.RadixBatchJobStatus{
					{
						Name:         "job7",
						Phase:        v1.BatchJobPhaseSucceeded,
						Message:      "anymessage",
						CreationTime: &creationTime,
						StartTime:    &startTime,
						EndTime:      &endTime,
						RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{
							CreationTime: &podCreationTime,
							Phase:        v1.PodSucceeded,
						}},
					},
				},
			},
		},
	}
	for _, rb := range testData {
		_, err := radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &rb, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	// Test job1 props - props from RD jobComponent
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/%s", anyAppName, anyEnvironment, anyJobName, "job-batch1-job1"))
	response := <-responseChannel
	var actual models.ScheduledJobSummary
	err = test.GetResponseBody(response, &actual)
	require.NoError(t, err)
	assert.Equal(t, models.ScheduledJobSummary{
		Name:             "job-batch1-job1",
		Created:          &creationTime.Time,
		Started:          &startTime.Time,
		Ended:            &endTime.Time,
		Status:           models.ScheduledBatchJobStatusSucceeded,
		Message:          "anymessage",
		BackoffLimit:     *defaultBackoffLimit,
		TimeLimitSeconds: numbers.Int64Ptr(123),
		Resources: models.ResourceRequirements{
			Limits:   models.Resources{CPU: "100Mi", Memory: "500M"},
			Requests: models.Resources{CPU: "50Mi", Memory: "250M"},
		},
		Node:           &models.Node{Gpu: "gpu1", GpuCount: "2"},
		DeploymentName: anyDeployment,
		ReplicaList: []models.ReplicaSummary{{
			Created: podCreationTime.Time,
			Status:  models.ReplicaStatus{Status: models.Succeeded},
		}},
		Runtime: &models.Runtime{
			Architecture: "amd64",
		},
	}, actual)

	// Test job2 props - override props from RD jobComponent
	responseChannel = environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/%s", anyAppName, anyEnvironment, anyJobName, "job-batch1-job2"))
	response = <-responseChannel
	actual = models.ScheduledJobSummary{}
	err = test.GetResponseBody(response, &actual)
	require.NoError(t, err)
	assert.Equal(t, models.ScheduledJobSummary{
		Created:          nil,
		Name:             "job-batch1-job2",
		JobId:            "anyjobid",
		Status:           models.ScheduledBatchJobStatusWaiting,
		BackoffLimit:     5,
		TimeLimitSeconds: numbers.Int64Ptr(999),
		Resources: models.ResourceRequirements{
			Limits:   models.Resources{CPU: "101Mi", Memory: "501M"},
			Requests: models.Resources{CPU: "51Mi", Memory: "251M"},
		},
		Node:           &models.Node{Gpu: "gpu2", GpuCount: "3"},
		DeploymentName: anyDeployment,
		Runtime: &models.Runtime{
			Architecture: "amd64",
		},
	}, actual)
	// Test job3 props
	responseChannel = environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/%s", anyAppName, anyEnvironment, anyJobName2, "job3-batch1-job7"))
	response = <-responseChannel
	actual = models.ScheduledJobSummary{}
	err = test.GetResponseBody(response, &actual)
	require.NoError(t, err)
	assert.Equal(t, models.ScheduledJobSummary{
		Created:          &creationTime.Time,
		Started:          &startTime.Time,
		Ended:            &endTime.Time,
		Name:             "job3-batch1-job7",
		JobId:            "anyjobid2",
		Message:          "anymessage",
		Status:           models.ScheduledBatchJobStatusSucceeded,
		BackoffLimit:     5,
		TimeLimitSeconds: numbers.Int64Ptr(999),
		DeploymentName:   anyDeployment,
		ReplicaList: []models.ReplicaSummary{{
			Created: podCreationTime.Time,
			Status:  models.ReplicaStatus{Status: models.Succeeded},
		}},
		Runtime: &models.Runtime{
			Architecture: "amd64",
			NodeType:     nodeType1,
		},
	}, actual)
}

func Test_GetJobPayload(t *testing.T) {
	namespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, kubeClient, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(utils.NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		utils.NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithJobComponents(utils.NewDeployJobComponentBuilder().
				WithName(anyJobName)).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	// Insert test data
	rb := v1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "job-batch1",
			Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
		},
		Spec: v1.RadixBatchSpec{
			Jobs: []v1.RadixBatchJob{
				{Name: "job1"},
				{Name: "job2", PayloadSecretRef: &v1.PayloadSecretKeySelector{
					Key:                  "payload1",
					LocalObjectReference: v1.LocalObjectReference{Name: anySecretName},
				}},
				{Name: "job3", PayloadSecretRef: &v1.PayloadSecretKeySelector{
					Key:                  "missingpayloadkey",
					LocalObjectReference: v1.LocalObjectReference{Name: anySecretName},
				}},
				{Name: "job4", PayloadSecretRef: &v1.PayloadSecretKeySelector{
					Key:                  "payload1",
					LocalObjectReference: v1.LocalObjectReference{Name: "otherSecret"},
				}},
			},
		}}
	_, err = radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &rb, metav1.CreateOptions{})
	require.NoError(t, err)

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: anySecretName},
		Data: map[string][]byte{
			"payload1": []byte("job1payload"),
		},
	}
	_, err = kubeClient.CoreV1().Secrets(namespace).Create(context.Background(), &secret, metav1.CreateOptions{})
	require.NoError(t, err)

	// Test job1 payload
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/%s/payload", anyAppName, anyEnvironment, anyJobName, "job-batch1-job1"))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Empty(t, response.Body.Bytes())

	// Test job2 payload
	responseChannel = environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/%s/payload", anyAppName, anyEnvironment, anyJobName, "job-batch1-job2"))
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "job1payload", response.Body.String())

	// Test job3 payload
	responseChannel = environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/%s/payload", anyAppName, anyEnvironment, anyJobName, "job-batch1-job3"))
	response = <-responseChannel
	assert.Equal(t, http.StatusNotFound, response.Code)
	errorResponse, _ := test.GetErrorResponse(response)
	assert.Equal(t, environmentModels.ScheduledJobPayloadNotFoundError(anyAppName, "job-batch1-job3"), errorResponse)

	// Test job4 payload
	responseChannel = environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/%s/payload", anyAppName, anyEnvironment, anyJobName, "job-batch1-job4"))
	response = <-responseChannel
	assert.Equal(t, http.StatusNotFound, response.Code)
	errorResponse, _ = test.GetErrorResponse(response)
	assert.Equal(t, environmentModels.ScheduledJobPayloadNotFoundError(anyAppName, "job-batch1-job4"), errorResponse)

}

func Test_GetBatch_JobList(t *testing.T) {
	namespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(utils.NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		utils.NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithJobComponents(utils.NewDeployJobComponentBuilder().WithName(anyJobName)).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	// Insert test data
	testData := []v1.RadixBatch{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   anyBatchName,
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Spec: v1.RadixBatchSpec{
				Jobs: []v1.RadixBatchJob{{Name: "no1"}, {Name: "no2"}, {Name: "no3"}, {Name: "no4"}, {Name: "no5"}, {Name: "no6"}, {Name: "no7"}, {Name: "no8"}}},
			Status: v1.RadixBatchStatus{
				JobStatuses: []v1.RadixBatchJobStatus{
					{Name: "no2"},
					{Name: "no3", Phase: v1.BatchJobPhaseWaiting},
					{Name: "no4", Phase: v1.BatchJobPhaseActive},
					{Name: "no5", Phase: v1.BatchJobPhaseRunning},
					{Name: "no6", Phase: v1.BatchJobPhaseSucceeded},
					{Name: "no7", Phase: v1.BatchJobPhaseFailed},
					{Name: "no8", Phase: v1.BatchJobPhaseStopped},
					{Name: "not-defined"},
				},
			},
		},
	}
	for _, rb := range testData {
		_, err := radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &rb, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	// Test get jobs for jobComponent1Name
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/batches/%s", anyAppName, anyEnvironment, anyJobName, anyBatchName))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	var actual models.ScheduledBatchSummary
	err = test.GetResponseBody(response, &actual)
	require.NoError(t, err)
	require.Len(t, actual.JobList, 8)
	type assertMapped struct {
		Name   string
		Status models.ScheduledBatchJobStatus
	}
	actualMapped := slice.Map(actual.JobList, func(job models.ScheduledJobSummary) assertMapped {
		return assertMapped{Name: job.Name, Status: job.Status}
	})
	expected := []assertMapped{
		{Name: anyBatchName + "-no1", Status: models.ScheduledBatchJobStatusWaiting},
		{Name: anyBatchName + "-no2", Status: models.ScheduledBatchJobStatusWaiting},
		{Name: anyBatchName + "-no3", Status: models.ScheduledBatchJobStatusWaiting},
		{Name: anyBatchName + "-no4", Status: models.ScheduledBatchJobStatusActive},
		{Name: anyBatchName + "-no5", Status: models.ScheduledBatchJobStatusRunning},
		{Name: anyBatchName + "-no6", Status: models.ScheduledBatchJobStatusSucceeded},
		{Name: anyBatchName + "-no7", Status: models.ScheduledBatchJobStatusFailed},
		{Name: anyBatchName + "-no8", Status: models.ScheduledBatchJobStatusStopped},
	}
	assert.ElementsMatch(t, expected, actualMapped)
}

func Test_GetBatch_JobList_StopFlag(t *testing.T) {
	namespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(utils.NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		utils.NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithJobComponents(utils.NewDeployJobComponentBuilder().WithName(anyJobName)).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	// Insert test data
	testData := []v1.RadixBatch{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   anyBatchName,
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Spec: v1.RadixBatchSpec{
				Jobs: []v1.RadixBatchJob{{Name: "no1", Stop: commonUtils.BoolPtr(true)}, {Name: "no2", Stop: commonUtils.BoolPtr(true)}, {Name: "no3", Stop: commonUtils.BoolPtr(true)}, {Name: "no4", Stop: commonUtils.BoolPtr(true)}, {Name: "no5", Stop: commonUtils.BoolPtr(true)}, {Name: "no6", Stop: commonUtils.BoolPtr(true)}, {Name: "no7", Stop: commonUtils.BoolPtr(true)}}},
			Status: v1.RadixBatchStatus{
				JobStatuses: []v1.RadixBatchJobStatus{
					{Name: "no2"},
					{Name: "no3", Phase: v1.BatchJobPhaseWaiting},
					{Name: "no4", Phase: v1.BatchJobPhaseActive},
					{Name: "no5", Phase: v1.BatchJobPhaseSucceeded},
					{Name: "no6", Phase: v1.BatchJobPhaseFailed},
					{Name: "no7", Phase: v1.BatchJobPhaseStopped},
					{Name: "not-defined"},
				},
			},
		},
	}
	for _, rb := range testData {
		_, err := radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &rb, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	// Test get jobs for jobComponent1Name
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/batches/%s", anyAppName, anyEnvironment, anyJobName, anyBatchName))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	var actual models.ScheduledBatchSummary
	err = test.GetResponseBody(response, &actual)
	require.NoError(t, err)
	require.Len(t, actual.JobList, 7)
	type assertMapped struct {
		Name   string
		Status models.ScheduledBatchJobStatus
	}
	actualMapped := slice.Map(actual.JobList, func(job models.ScheduledJobSummary) assertMapped {
		return assertMapped{Name: job.Name, Status: job.Status}
	})
	expected := []assertMapped{
		{Name: anyBatchName + "-no1", Status: models.ScheduledBatchJobStatusStopping},
		{Name: anyBatchName + "-no2", Status: models.ScheduledBatchJobStatusStopping},
		{Name: anyBatchName + "-no3", Status: models.ScheduledBatchJobStatusStopping},
		{Name: anyBatchName + "-no4", Status: models.ScheduledBatchJobStatusStopping},
		{Name: anyBatchName + "-no5", Status: models.ScheduledBatchJobStatusSucceeded},
		{Name: anyBatchName + "-no6", Status: models.ScheduledBatchJobStatusFailed},
		{Name: anyBatchName + "-no7", Status: models.ScheduledBatchJobStatusStopped},
	}
	assert.ElementsMatch(t, expected, actualMapped)
}

func Test_GetBatches_Status(t *testing.T) {
	namespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(utils.NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		utils.NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithJobComponents(utils.NewDeployJobComponentBuilder().WithName(anyJobName)).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	// Insert test data
	testData := []v1.RadixBatch{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "batch-job1",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "batch-job2",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Status: v1.RadixBatchStatus{
				Condition: v1.RadixBatchCondition{Type: v1.BatchConditionTypeWaiting},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "batch-job3",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Status: v1.RadixBatchStatus{
				JobStatuses: []v1.RadixBatchJobStatus{
					{Name: "j1"},
					{
						Name:  "j2",
						Phase: v1.BatchJobPhaseActive,
						RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{
							Phase:        v1.PodRunning,
							CreationTime: &metav1.Time{Time: time.Now()},
							StartTime:    &metav1.Time{Time: time.Now()},
						}},
					},
					{
						Name:  "j3",
						Phase: v1.BatchJobPhaseWaiting,
						RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{
							Phase:        v1.PodPending,
							CreationTime: &metav1.Time{Time: time.Now()},
						}},
					},
				},
				Condition: v1.RadixBatchCondition{Type: v1.BatchConditionTypeActive},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "batch-job4",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Status: v1.RadixBatchStatus{
				JobStatuses: []v1.RadixBatchJobStatus{
					{Name: "j1"},
					{
						Name:  "j2",
						Phase: v1.BatchJobPhaseRunning,
						RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{
							Phase:        v1.PodRunning,
							CreationTime: &metav1.Time{Time: time.Now()},
							StartTime:    &metav1.Time{Time: time.Now()},
						}},
					},
					{
						Name:  "j3",
						Phase: v1.BatchJobPhaseSucceeded,
						RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{
							Phase:        v1.PodSucceeded,
							CreationTime: &metav1.Time{Time: time.Now()},
							StartTime:    &metav1.Time{Time: time.Now()},
							EndTime:      &metav1.Time{Time: time.Now()},
						}},
					},
				},
				Condition: v1.RadixBatchCondition{Type: v1.BatchConditionTypeActive},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "batch-job5",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Status: v1.RadixBatchStatus{
				Condition: v1.RadixBatchCondition{Type: v1.BatchConditionTypeCompleted},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "batch-job6",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Status: v1.RadixBatchStatus{
				Condition: v1.RadixBatchCondition{Type: v1.BatchConditionTypeCompleted},
				JobStatuses: []v1.RadixBatchJobStatus{
					{
						Name:    "j1",
						Phase:   v1.BatchJobPhaseFailed,
						EndTime: &metav1.Time{Time: time.Now()},
						Failed:  1,
						RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{
							Phase:        v1.PodFailed,
							CreationTime: &metav1.Time{Time: time.Now()},
							StartTime:    &metav1.Time{Time: time.Now()},
							EndTime:      &metav1.Time{Time: time.Now()},
						}},
					},
					{
						Name:    "j2",
						Phase:   v1.BatchJobPhaseFailed,
						EndTime: &metav1.Time{Time: time.Now()},
						Failed:  1,
						RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{
							Phase:        v1.PodFailed,
							CreationTime: &metav1.Time{Time: time.Now()},
							StartTime:    &metav1.Time{Time: time.Now()},
							EndTime:      &metav1.Time{Time: time.Now()},
						}},
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "batch-job7",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Status: v1.RadixBatchStatus{
				Condition: v1.RadixBatchCondition{Type: v1.BatchConditionTypeCompleted},
				JobStatuses: []v1.RadixBatchJobStatus{
					{
						Name:    "j1",
						Phase:   v1.BatchJobPhaseFailed,
						EndTime: &metav1.Time{Time: time.Now()},
						Failed:  1,
						RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{
							Phase:        v1.PodFailed,
							CreationTime: &metav1.Time{Time: time.Now()},
							StartTime:    &metav1.Time{Time: time.Now()},
							EndTime:      &metav1.Time{Time: time.Now()},
						}},
					},
					{
						Name:    "j2",
						Phase:   v1.BatchJobPhaseSucceeded,
						EndTime: &metav1.Time{Time: time.Now()},
						RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{
							Phase:        v1.PodSucceeded,
							CreationTime: &metav1.Time{Time: time.Now()},
							StartTime:    &metav1.Time{Time: time.Now()},
							EndTime:      &metav1.Time{Time: time.Now()},
						}},
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "batch-compute1",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName("other-component"), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "jobtype-job1",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
			},
		},
	}
	for _, rb := range testData {
		_, err := radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &rb, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/batches", anyAppName, anyEnvironment, anyJobName))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	var actual []models.ScheduledBatchSummary
	err = test.GetResponseBody(response, &actual)
	require.NoError(t, err)
	type assertMapped struct {
		Name   string
		Status models.ScheduledBatchJobStatus
	}
	actualMapped := slice.Map(actual, func(b models.ScheduledBatchSummary) assertMapped {
		return assertMapped{Name: b.Name, Status: b.Status}
	})
	expected := []assertMapped{
		{Name: "batch-job1", Status: models.ScheduledBatchJobStatusWaiting},
		{Name: "batch-job2", Status: models.ScheduledBatchJobStatusWaiting},
		{Name: "batch-job3", Status: models.ScheduledBatchJobStatusActive},
		{Name: "batch-job4", Status: models.ScheduledBatchJobStatusActive},
		{Name: "batch-job5", Status: models.ScheduledBatchJobStatusCompleted},
		{Name: "batch-job6", Status: models.ScheduledBatchJobStatusCompleted},
		{Name: "batch-job7", Status: models.ScheduledBatchJobStatusCompleted},
	}
	assert.ElementsMatch(t, expected, actualMapped)
}

func Test_GetBatches_JobListShouldHaveJobWithStatusWaiting(t *testing.T) {
	namespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(utils.NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		utils.NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithJobComponents(utils.NewDeployJobComponentBuilder().WithName(anyJobName)).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	// Insert test data
	testData := []v1.RadixBatch{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "batch-job1",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Spec: v1.RadixBatchSpec{
				Jobs: []v1.RadixBatchJob{{Name: "j1"}},
			},
		},
	}
	for _, rb := range testData {
		_, err := radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &rb, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/batches", anyAppName, anyEnvironment, anyJobName))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	var actual []models.ScheduledBatchSummary
	err = test.GetResponseBody(response, &actual)
	require.NoError(t, err)
	require.Len(t, actual, 1)
	assert.Len(t, actual[0].JobList, 1)
	assert.Equal(t, models.ScheduledBatchJobStatusWaiting, actual[0].JobList[0].Status)
}

func Test_StopJob(t *testing.T) {
	type JobTestData struct {
		name      string
		jobStatus v1.RadixBatchJobStatus
	}

	batchTypeBatchName, batchTypeJobName := "batchBatch", "jobBatch"
	namespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	validJobs := []JobTestData{
		{name: "validJob1"},
		{name: "validJob2", jobStatus: v1.RadixBatchJobStatus{Name: "validJob2", Phase: ""}},
		{name: "validJob3", jobStatus: v1.RadixBatchJobStatus{Name: "validJob3", Phase: v1.BatchJobPhaseWaiting}},
		{name: "validJob4", jobStatus: v1.RadixBatchJobStatus{Name: "validJob4", Phase: v1.BatchJobPhaseActive}},
		{name: "validJob5", jobStatus: v1.RadixBatchJobStatus{Name: "validJob5", Phase: v1.BatchJobPhaseRunning}},
	}
	invalidJobs := []JobTestData{
		{name: "invalidJob1", jobStatus: v1.RadixBatchJobStatus{Name: "invalidJob1", Phase: v1.BatchJobPhaseSucceeded}},
		{name: "invalidJob2", jobStatus: v1.RadixBatchJobStatus{Name: "invalidJob2", Phase: v1.BatchJobPhaseFailed}},
		{name: "invalidJob3", jobStatus: v1.RadixBatchJobStatus{Name: "invalidJob3", Phase: v1.BatchJobPhaseStopped}},
	}
	nonExistentJobs := []JobTestData{
		{name: "noJob"},
	}

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(utils.NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		utils.NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithJobComponents(utils.NewDeployJobComponentBuilder().WithName(anyJobName)).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	jobSpecList := append(
		slice.Map(validJobs, func(j JobTestData) v1.RadixBatchJob { return v1.RadixBatchJob{Name: j.name} }),
		slice.Map(invalidJobs, func(j JobTestData) v1.RadixBatchJob { return v1.RadixBatchJob{Name: j.name} })...,
	)
	jobStatuses := append(
		slice.Map(validJobs, func(j JobTestData) v1.RadixBatchJobStatus { return j.jobStatus }),
		slice.Map(invalidJobs, func(j JobTestData) v1.RadixBatchJobStatus { return j.jobStatus })...,
	)
	// Insert test data
	testData := []v1.RadixBatch{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batchTypeBatchName,
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Spec: v1.RadixBatchSpec{Jobs: jobSpecList},
			Status: v1.RadixBatchStatus{
				JobStatuses: jobStatuses,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batchTypeJobName,
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
			},
			Spec: v1.RadixBatchSpec{Jobs: jobSpecList},
			Status: v1.RadixBatchStatus{
				JobStatuses: jobStatuses,
			},
		},
	}
	for _, rb := range testData {
		_, err := radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &rb, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	// jobs by name that can be stopped
	validJobNames := slice.Reduce(validJobs, []string{}, func(obj []string, job JobTestData) []string { return append(obj, job.name) })

	// Test both batches
	for _, batchName := range []string{batchTypeBatchName, batchTypeJobName} {
		// Test valid jobs
		for _, v := range validJobs {
			responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/%s/stop", anyAppName, anyEnvironment, anyJobName, batchName+"-"+v.name))
			response := <-responseChannel
			assert.Equal(t, http.StatusNoContent, response.Code)
			assert.Empty(t, response.Body.Bytes())
		}

		// Test invalid jobs
		for _, v := range invalidJobs {
			responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/%s/stop", anyAppName, anyEnvironment, anyJobName, batchName+"-"+v.name))
			response := <-responseChannel
			assert.Equal(t, http.StatusBadRequest, response.Code)
			assert.NotEmpty(t, response.Body.Bytes())
		}

		// Test non-existent jobs
		for _, v := range nonExistentJobs {
			responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/%s/stop", anyAppName, anyEnvironment, anyJobName, batchName+"-"+v.name))
			response := <-responseChannel
			assert.Equal(t, http.StatusNotFound, response.Code)
			assert.NotEmpty(t, response.Body.Bytes())
		}

		// Check that stoppable jobs are stopped
		assertBatchJobStoppedStates(t, radixClient, namespace, batchName, validJobNames)
	}
}

func Test_DeleteJob(t *testing.T) {
	type JobTestData struct {
		name      string
		jobStatus v1.RadixBatchJobStatus
	}

	batchTypeBatchName, batchTypeJobNames := "batchBatch", []string{"jobBatch1", "jobBatch2", "jobBatch3"}
	namespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	jobs := []JobTestData{
		{name: "validJob1"},
		{name: "validJob2", jobStatus: v1.RadixBatchJobStatus{Name: "validJob2", Phase: ""}},
		{name: "validJob3", jobStatus: v1.RadixBatchJobStatus{Name: "validJob3", Phase: v1.BatchJobPhaseWaiting}},
		{name: "validJob4", jobStatus: v1.RadixBatchJobStatus{Name: "validJob4", Phase: v1.BatchJobPhaseActive}},
	}

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(utils.NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		utils.NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithJobComponents(utils.NewDeployJobComponentBuilder().WithName(anyJobName)).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	// Insert test data
	testData := []v1.RadixBatch{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batchTypeBatchName,
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Spec:   v1.RadixBatchSpec{Jobs: []v1.RadixBatchJob{{Name: jobs[0].name}, {Name: jobs[1].name}, {Name: jobs[2].name}, {Name: jobs[3].name}}},
			Status: v1.RadixBatchStatus{JobStatuses: []v1.RadixBatchJobStatus{jobs[0].jobStatus, jobs[1].jobStatus, jobs[2].jobStatus, jobs[3].jobStatus}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batchTypeJobNames[0],
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
			},
			Spec:   v1.RadixBatchSpec{Jobs: []v1.RadixBatchJob{{Name: jobs[0].name}}},
			Status: v1.RadixBatchStatus{JobStatuses: []v1.RadixBatchJobStatus{jobs[0].jobStatus}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batchTypeJobNames[1],
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
			},
			Spec:   v1.RadixBatchSpec{Jobs: []v1.RadixBatchJob{{Name: jobs[1].name}}},
			Status: v1.RadixBatchStatus{JobStatuses: []v1.RadixBatchJobStatus{jobs[1].jobStatus}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batchTypeJobNames[2],
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
			},
			Spec:   v1.RadixBatchSpec{Jobs: []v1.RadixBatchJob{{Name: jobs[2].name}}},
			Status: v1.RadixBatchStatus{JobStatuses: []v1.RadixBatchJobStatus{jobs[2].jobStatus}},
		},
	}
	for _, rb := range testData {
		_, err := radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &rb, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	// deletable jobs
	deletableJobs := []string{batchTypeJobNames[0], batchTypeJobNames[2]} // selected jobs to delete
	for _, batchName := range deletableJobs {
		jobs := testData[slice.FindIndex(testData, func(batch v1.RadixBatch) bool { return batch.Name == batchName })].Spec.Jobs
		responseChannel := environmentControllerTestUtils.ExecuteRequest("DELETE", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/%s", anyAppName, anyEnvironment, anyJobName, batchName+"-"+jobs[0].Name))
		response := <-responseChannel
		assert.Equal(t, http.StatusNoContent, response.Code)
		assert.Empty(t, response.Body.Bytes())
	}

	// non-deletable jobs
	nonDeletableJobs := []string{batchTypeBatchName}
	for _, batchName := range nonDeletableJobs {
		jobs := testData[slice.FindIndex(testData, func(batch v1.RadixBatch) bool { return batch.Name == batchName })].Spec.Jobs
		jobNames := slice.Reduce(jobs, []string{}, func(names []string, job v1.RadixBatchJob) []string { return append(names, job.Name) })
		for _, jobName := range jobNames {
			responseChannel := environmentControllerTestUtils.ExecuteRequest("DELETE", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/%s", anyAppName, anyEnvironment, anyJobName, batchName+"-"+jobName))
			response := <-responseChannel
			assert.Equal(t, http.StatusNotFound, response.Code)
			assert.NotEmpty(t, response.Body.Bytes())
		}
	}

	// non-existent jobs
	nonExistentJobs := []string{"noBatch"}
	for _, batchName := range nonExistentJobs {
		jobName := "noJob"
		responseChannel := environmentControllerTestUtils.ExecuteRequest("DELETE", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/%s", anyAppName, anyEnvironment, anyJobName, batchName+"-"+jobName))
		response := <-responseChannel
		assert.Equal(t, http.StatusNotFound, response.Code)
		assert.NotEmpty(t, response.Body.Bytes())
	}

	// assert only deletable jobs are deleted/gone
	for _, batchName := range append(batchTypeJobNames, batchTypeBatchName) {
		assertBatchDeleted(t, radixClient, namespace, batchName, deletableJobs)
	}
}

func Test_StopBatch(t *testing.T) {
	type JobTestData struct {
		name      string
		jobStatus v1.RadixBatchJobStatus
	}

	batchTypeBatchName, batchTypeJobName, nonExistentBatch := "batchBatch", "jobBatch", "noBatch"
	namespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	validJobs := []JobTestData{
		{name: "validJob1"},
		{name: "validJob2", jobStatus: v1.RadixBatchJobStatus{Name: "validJob2", Phase: ""}},
		{name: "validJob3", jobStatus: v1.RadixBatchJobStatus{Name: "validJob3", Phase: v1.BatchJobPhaseWaiting}},
		{name: "validJob4", jobStatus: v1.RadixBatchJobStatus{Name: "validJob4", Phase: v1.BatchJobPhaseActive}},
	}
	invalidJobs := []JobTestData{
		{name: "invalidJob1", jobStatus: v1.RadixBatchJobStatus{Name: "invalidJob1", Phase: v1.BatchJobPhaseSucceeded}},
		{name: "invalidJob2", jobStatus: v1.RadixBatchJobStatus{Name: "invalidJob2", Phase: v1.BatchJobPhaseFailed}},
		{name: "invalidJob3", jobStatus: v1.RadixBatchJobStatus{Name: "invalidJob3", Phase: v1.BatchJobPhaseStopped}},
	}

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(utils.NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		utils.NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithJobComponents(utils.NewDeployJobComponentBuilder().WithName(anyJobName)).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	// Insert test data
	testData := []v1.RadixBatch{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batchTypeBatchName,
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Spec: v1.RadixBatchSpec{
				Jobs: []v1.RadixBatchJob{
					{Name: validJobs[0].name}, {Name: validJobs[1].name}, {Name: validJobs[2].name}, {Name: validJobs[3].name},
					{Name: invalidJobs[0].name}, {Name: invalidJobs[1].name}, {Name: invalidJobs[2].name},
				}},
			Status: v1.RadixBatchStatus{
				Condition: v1.RadixBatchCondition{Type: v1.BatchConditionTypeActive},
				JobStatuses: []v1.RadixBatchJobStatus{
					validJobs[0].jobStatus, validJobs[1].jobStatus, validJobs[2].jobStatus, validJobs[3].jobStatus,
					invalidJobs[0].jobStatus, invalidJobs[1].jobStatus, invalidJobs[2].jobStatus,
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batchTypeJobName,
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
			},
			Spec: v1.RadixBatchSpec{
				Jobs: []v1.RadixBatchJob{
					{Name: validJobs[0].name}, {Name: validJobs[1].name}, {Name: validJobs[2].name}, {Name: validJobs[3].name},
					{Name: invalidJobs[0].name}, {Name: invalidJobs[1].name}, {Name: invalidJobs[2].name},
				}},
			Status: v1.RadixBatchStatus{
				Condition: v1.RadixBatchCondition{Type: v1.BatchConditionTypeActive},
				JobStatuses: []v1.RadixBatchJobStatus{
					validJobs[0].jobStatus, validJobs[1].jobStatus, validJobs[2].jobStatus, validJobs[3].jobStatus,
					invalidJobs[0].jobStatus, invalidJobs[1].jobStatus, invalidJobs[2].jobStatus,
				},
			},
		},
	}
	for _, rb := range testData {
		_, err := radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &rb, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	// Test valid batch
	responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/batches/%s/stop", anyAppName, anyEnvironment, anyJobName, batchTypeBatchName))
	response := <-responseChannel
	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.Empty(t, response.Body.Bytes())

	// Test invalid batch type
	responseChannel = environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/batches/%s/stop", anyAppName, anyEnvironment, anyJobName, batchTypeJobName))
	response = <-responseChannel
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.NotEmpty(t, response.Body.Bytes())

	// Test non-existent batch
	responseChannel = environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/batches/%s/stop", anyAppName, anyEnvironment, anyJobName, nonExistentBatch))
	response = <-responseChannel
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.NotEmpty(t, response.Body.Bytes())

	// jobs by name that can be stopped
	validJobNames := slice.Reduce(validJobs, []string{}, func(obj []string, job JobTestData) []string { return append(obj, job.name) })

	// Check that stoppable jobs are stopped
	assertBatchJobStoppedStates(t, radixClient, namespace, batchTypeBatchName, validJobNames)
	assertBatchJobStoppedStates(t, radixClient, namespace, batchTypeJobName, []string{}) // invalid batch type, no jobs should be stopped
}

func Test_DeleteBatch(t *testing.T) {
	type JobTestData struct {
		name      string
		jobStatus v1.RadixBatchJobStatus
	}

	batchTypeBatchNames, batchTypeJobName := []string{"batchBatch1", "batchBatch2", "batchBatch3"}, "jobBatch"
	namespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	jobs := []JobTestData{
		{name: "validJob1"},
		{name: "validJob2", jobStatus: v1.RadixBatchJobStatus{Name: "validJob2", Phase: ""}},
		{name: "validJob3", jobStatus: v1.RadixBatchJobStatus{Name: "validJob3", Phase: v1.BatchJobPhaseWaiting}},
		{name: "validJob4", jobStatus: v1.RadixBatchJobStatus{Name: "validJob4", Phase: v1.BatchJobPhaseActive}},
	}

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(utils.NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		utils.NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithJobComponents(utils.NewDeployJobComponentBuilder().WithName(anyJobName)).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	// Insert test data
	testData := []v1.RadixBatch{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batchTypeBatchNames[0],
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Spec:   v1.RadixBatchSpec{Jobs: []v1.RadixBatchJob{{Name: jobs[0].name}, {Name: jobs[1].name}, {Name: jobs[2].name}, {Name: jobs[3].name}}},
			Status: v1.RadixBatchStatus{JobStatuses: []v1.RadixBatchJobStatus{jobs[0].jobStatus, jobs[1].jobStatus, jobs[2].jobStatus, jobs[3].jobStatus}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batchTypeBatchNames[1],
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Spec:   v1.RadixBatchSpec{Jobs: []v1.RadixBatchJob{{Name: jobs[0].name}, {Name: jobs[1].name}}},
			Status: v1.RadixBatchStatus{JobStatuses: []v1.RadixBatchJobStatus{jobs[0].jobStatus, jobs[1].jobStatus}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batchTypeBatchNames[2],
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Spec:   v1.RadixBatchSpec{Jobs: []v1.RadixBatchJob{{Name: jobs[2].name}, {Name: jobs[3].name}}},
			Status: v1.RadixBatchStatus{JobStatuses: []v1.RadixBatchJobStatus{jobs[2].jobStatus, jobs[3].jobStatus}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batchTypeJobName,
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
			},
			Spec:   v1.RadixBatchSpec{Jobs: []v1.RadixBatchJob{{Name: jobs[0].name}}},
			Status: v1.RadixBatchStatus{JobStatuses: []v1.RadixBatchJobStatus{jobs[0].jobStatus}},
		},
	}
	for _, rb := range testData {
		_, err := radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &rb, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	// deletable batches
	deletableBatches := []string{batchTypeBatchNames[0], batchTypeBatchNames[2]} // selected jobs to delete
	for _, batchName := range deletableBatches {
		responseChannel := environmentControllerTestUtils.ExecuteRequest("DELETE", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/batches/%s", anyAppName, anyEnvironment, anyJobName, batchName))
		response := <-responseChannel
		assert.Equal(t, http.StatusNoContent, response.Code)
		assert.Empty(t, response.Body.Bytes())
	}

	// non-deletable batches
	nonDeletableJobs := []string{batchTypeJobName}
	for _, batchName := range nonDeletableJobs {
		responseChannel := environmentControllerTestUtils.ExecuteRequest("DELETE", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/batches/%s", anyAppName, anyEnvironment, anyJobName, batchName))
		response := <-responseChannel
		assert.Equal(t, http.StatusNotFound, response.Code)
		assert.NotEmpty(t, response.Body.Bytes())
	}

	// non-existent batches
	nonExistentJobs := []string{"noBatch"}
	for _, batchName := range nonExistentJobs {
		responseChannel := environmentControllerTestUtils.ExecuteRequest("DELETE", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/batches/%s", anyAppName, anyEnvironment, anyJobName, batchName))
		response := <-responseChannel
		assert.Equal(t, http.StatusNotFound, response.Code)
		assert.NotEmpty(t, response.Body.Bytes())
	}

	// assert only deletable batches are deleted/gone
	for _, batchName := range append(batchTypeBatchNames, batchTypeJobName) {
		assertBatchDeleted(t, radixClient, namespace, batchName, deletableBatches)
	}
}

func Test_StopAllJobComponentJobs(t *testing.T) {
	const (
		envName1      = "qa"
		envName2      = "dev"
		jobComponent1 = "compute1"
		jobComponent2 = "compute2"
	)

	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(utils.NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)

	envNamespace1 := utils.GetEnvironmentNamespace(anyAppName, envName1)
	envNamespace2 := utils.GetEnvironmentNamespace(anyAppName, envName2)
	addRadixBatchWithStatus(radixClient, "test-batch1-job1", jobComponent1, kube.RadixBatchTypeJob, envNamespace1, v1.BatchConditionTypeCompleted)
	addRadixBatchWithStatus(radixClient, "test-batch2-job1", jobComponent1, kube.RadixBatchTypeJob, envNamespace1, v1.BatchConditionTypeActive)
	addRadixBatchWithStatus(radixClient, "test-batch3-job1", jobComponent2, kube.RadixBatchTypeJob, envNamespace1, v1.BatchConditionTypeActive)
	addRadixBatchWithStatus(radixClient, "test-batch4-job1", jobComponent1, kube.RadixBatchTypeBatch, envNamespace1, v1.BatchConditionTypeActive)
	addRadixBatch(radixClient, "test-batch5-job1", jobComponent1, kube.RadixBatchTypeJob, envNamespace1)
	addRadixBatchWithStatus(radixClient, "test-batch6-job1", jobComponent1, kube.RadixBatchTypeJob, envNamespace1, v1.BatchConditionTypeWaiting)
	addRadixBatch(radixClient, "test-batch7-job1", jobComponent1, kube.RadixBatchTypeJob, envNamespace2)

	responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/stop", anyAppName, envName1, jobComponent1))
	response := <-responseChannel
	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.Empty(t, response.Body.Bytes())

	batchList, err := radixClient.RadixV1().RadixBatches(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	assert.NoError(t, err)

	rb1, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch1" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch1 should be found")
	assert.Nil(t, rb1.Spec.Jobs[0].Stop, "test-batch1 job should not be stopped")

	rb2, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch2" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch2 should be found")
	assert.Equal(t, pointers.Ptr(true), rb2.Spec.Jobs[0].Stop, "test-batch2 job should be stopped")

	rb3, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch3" && batch.GetNamespace() == envNamespace1 && batch.GetLabels()[kube.RadixComponentLabel] == jobComponent2
	})
	assert.True(t, ok, "test-batch3 should be found")
	assert.Nil(t, rb3.Spec.Jobs[0].Stop, "test-batch3 job should not be stopped")

	rb4, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch4" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch4 should be found")
	assert.Nil(t, rb4.Spec.Jobs[0].Stop, "test-batch4 single job should not be stopped")

	rb5, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch5" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch5 should be found")
	assert.Equal(t, pointers.Ptr(true), rb5.Spec.Jobs[0].Stop, "test-batch5 job should be stopped")

	rb6, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch6" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch6 should be found")
	assert.Equal(t, pointers.Ptr(true), rb6.Spec.Jobs[0].Stop, "test-batch6 job should be stopped")

	rb7, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch7" && batch.GetNamespace() == envNamespace2
	})
	assert.True(t, ok, "test-batch7 should be found")
	assert.Nil(t, rb7.Spec.Jobs[0].Stop, "test-batch7 job should not be stopped")
}

func Test_StopAllJobComponentBatches(t *testing.T) {
	const (
		envName1      = "qa"
		envName2      = "dev"
		jobComponent1 = "compute1"
		jobComponent2 = "compute2"
	)

	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(utils.NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)

	envNamespace1 := utils.GetEnvironmentNamespace(anyAppName, envName1)
	envNamespace2 := utils.GetEnvironmentNamespace(anyAppName, envName2)
	addRadixBatchWithStatus(radixClient, "test-batch1-job1", jobComponent1, kube.RadixBatchTypeBatch, envNamespace1, v1.BatchConditionTypeCompleted)
	addRadixBatchWithStatus(radixClient, "test-batch2-job1", jobComponent1, kube.RadixBatchTypeBatch, envNamespace1, v1.BatchConditionTypeActive)
	addRadixBatchWithStatus(radixClient, "test-batch3-job1", jobComponent2, kube.RadixBatchTypeBatch, envNamespace1, v1.BatchConditionTypeActive)
	addRadixBatchWithStatus(radixClient, "test-batch4-job1", jobComponent1, kube.RadixBatchTypeJob, envNamespace1, v1.BatchConditionTypeActive)
	addRadixBatch(radixClient, "test-batch5-job1", jobComponent1, kube.RadixBatchTypeBatch, envNamespace1)
	addRadixBatchWithStatus(radixClient, "test-batch6-job1", jobComponent1, kube.RadixBatchTypeBatch, envNamespace1, v1.BatchConditionTypeWaiting)
	addRadixBatch(radixClient, "test-batch7-job1", jobComponent1, kube.RadixBatchTypeBatch, envNamespace2)

	responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/batches/stop", anyAppName, envName1, jobComponent1))
	response := <-responseChannel
	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.Empty(t, response.Body.Bytes())

	batchList, err := radixClient.RadixV1().RadixBatches(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	assert.NoError(t, err)

	rb1, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch1" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch1 should be found")
	assert.Nil(t, rb1.Spec.Jobs[0].Stop, "test-batch1 job should not be stopped")

	rb2, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch2" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch2 should be found")
	assert.Equal(t, pointers.Ptr(true), rb2.Spec.Jobs[0].Stop, "test-batch2 job should be stopped")

	rb3, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch3" && batch.GetNamespace() == envNamespace1 && batch.GetLabels()[kube.RadixComponentLabel] == jobComponent2
	})
	assert.True(t, ok, "test-batch3 should be found")
	assert.Nil(t, rb3.Spec.Jobs[0].Stop, "test-batch3 job should not be stopped")

	rb4, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch4" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch4 should be found")
	assert.Nil(t, rb4.Spec.Jobs[0].Stop, "test-batch4 single job should not be stopped")

	rb5, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch5" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch5 should be found")
	assert.Equal(t, pointers.Ptr(true), rb5.Spec.Jobs[0].Stop, "test-batch5 job should be stopped")

	rb6, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch6" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch6 should be found")
	assert.Equal(t, pointers.Ptr(true), rb6.Spec.Jobs[0].Stop, "test-batch6 job should not be stopped")

	rb7, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch7" && batch.GetNamespace() == envNamespace2
	})
	assert.True(t, ok, "test-batch7 should be found")
	assert.Nil(t, rb7.Spec.Jobs[0].Stop, "test-batch7 job should not be stopped")
}

func Test_StopAllJobComponentBatchesAndJobs(t *testing.T) {
	const (
		envName1      = "qa"
		envName2      = "dev"
		jobComponent1 = "compute1"
		jobComponent2 = "compute2"
	)

	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(utils.NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(context.Background(), utils.NewDeploymentBuilder().
		WithAppName(anyAppName).WithEnvironment(envName1).
		WithJobComponent(utils.NewDeployJobComponentBuilder().WithName(jobComponent1).WithSchedulerPort(8001)).
		WithJobComponent(utils.NewDeployJobComponentBuilder().WithName(jobComponent2).WithSchedulerPort(8002)))
	require.NoError(t, err)

	envNamespace1 := utils.GetEnvironmentNamespace(anyAppName, envName1)
	envNamespace2 := utils.GetEnvironmentNamespace(anyAppName, envName2)
	addRadixBatchWithStatus(radixClient, "test-batch1-job1", jobComponent1, kube.RadixBatchTypeBatch, envNamespace1, v1.BatchConditionTypeCompleted)
	addRadixBatchWithStatus(radixClient, "test-batch2-job1", jobComponent1, kube.RadixBatchTypeBatch, envNamespace1, v1.BatchConditionTypeActive) //to be stopped
	addRadixBatchWithStatus(radixClient, "test-batch3-job1", jobComponent2, kube.RadixBatchTypeBatch, envNamespace1, v1.BatchConditionTypeActive)
	addRadixBatchWithStatus(radixClient, "test-batch4-job1", jobComponent1, kube.RadixBatchTypeJob, envNamespace1, v1.BatchConditionTypeCompleted)
	addRadixBatchWithStatus(radixClient, "test-batch5-job1", jobComponent1, kube.RadixBatchTypeJob, envNamespace1, v1.BatchConditionTypeActive) //to be stopped
	addRadixBatchWithStatus(radixClient, "test-batch6-job1", jobComponent2, kube.RadixBatchTypeJob, envNamespace1, v1.BatchConditionTypeActive)
	addRadixBatch(radixClient, "test-batch7-job1", jobComponent1, kube.RadixBatchTypeBatch, envNamespace1)                                         //to be stopped
	addRadixBatchWithStatus(radixClient, "test-batch8-job1", jobComponent1, kube.RadixBatchTypeBatch, envNamespace1, v1.BatchConditionTypeWaiting) //to be stopped
	addRadixBatch(radixClient, "test-batch9-job1", jobComponent1, kube.RadixBatchTypeBatch, envNamespace2)
	addRadixBatch(radixClient, "test-batch10-job1", jobComponent1, kube.RadixBatchTypeJob, envNamespace1)                                         //to be stopped
	addRadixBatchWithStatus(radixClient, "test-batch11-job1", jobComponent1, kube.RadixBatchTypeJob, envNamespace1, v1.BatchConditionTypeWaiting) //to be stopped
	addRadixBatch(radixClient, "test-batch12-job1", jobComponent1, kube.RadixBatchTypeJob, envNamespace2)

	responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/stop", anyAppName, envName1, jobComponent1))
	response := <-responseChannel
	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.Empty(t, response.Body.Bytes())
	batchList, err := radixClient.RadixV1().RadixBatches(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	assert.NoError(t, err)

	rb1, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch1" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch1 should be found")
	assert.Nil(t, rb1.Spec.Jobs[0].Stop, "test-batch1 job should not be stopped")

	rb2, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch2" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch2 should be found")
	assert.Equal(t, pointers.Ptr(true), rb2.Spec.Jobs[0].Stop, "test-batch2 job should be stopped")

	rb3, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch3" && batch.GetNamespace() == envNamespace1 && batch.GetLabels()[kube.RadixComponentLabel] == jobComponent2
	})
	assert.True(t, ok, "test-batch3 should be found")
	assert.Nil(t, rb3.Spec.Jobs[0].Stop, "test-batch3 job should not be stopped")

	rb4, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch4" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch4 should be found")
	assert.Nil(t, rb4.Spec.Jobs[0].Stop, "test-batch4 single job should not be stopped")

	rb5, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch5" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch5 should be found")
	assert.Equal(t, pointers.Ptr(true), rb5.Spec.Jobs[0].Stop, "test-batch5 job should be stopped")

	rb6, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch6" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch6 should be found")
	assert.Nil(t, rb6.Spec.Jobs[0].Stop, "test-batch6 job should not be stopped")

	rb7, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch7" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch7 should be found")
	assert.Equal(t, pointers.Ptr(true), rb7.Spec.Jobs[0].Stop, "test-batch7 job should be stopped")

	rb8, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch8" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch8 should be found")
	assert.Equal(t, pointers.Ptr(true), rb8.Spec.Jobs[0].Stop, "test-batch8 job should be stopped")

	rb9, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch9" && batch.GetNamespace() == envNamespace2
	})
	assert.True(t, ok, "test-batch9 should be found")
	assert.Nil(t, rb9.Spec.Jobs[0].Stop, "test-batch9 job should not be stopped")

	rb10, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch10" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch10 should be found")
	assert.Equal(t, pointers.Ptr(true), rb10.Spec.Jobs[0].Stop, "test-batch10 job should be stopped")

	rb11, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch11" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch11 should be found")
	assert.Equal(t, pointers.Ptr(true), rb11.Spec.Jobs[0].Stop, "test-batch11 job should be stopped")

	rb12, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch12" && batch.GetNamespace() == envNamespace2
	})
	assert.True(t, ok, "test-batch12 should be found")
	assert.Nil(t, rb12.Spec.Jobs[0].Stop, "test-batch12 job should not be stopped")
}

func Test_StopAllEnvironmentBatchesAndJobs(t *testing.T) {
	const (
		envName1      = "qa"
		envName2      = "dev"
		jobComponent1 = "compute1"
		jobComponent2 = "compute2"
	)

	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(utils.NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(context.Background(), utils.NewDeploymentBuilder().
		WithAppName(anyAppName).WithEnvironment(envName1).
		WithJobComponent(utils.NewDeployJobComponentBuilder().WithName(jobComponent1).WithSchedulerPort(8001)).
		WithJobComponent(utils.NewDeployJobComponentBuilder().WithName(jobComponent2).WithSchedulerPort(8002)))
	require.NoError(t, err)

	envNamespace1 := utils.GetEnvironmentNamespace(anyAppName, envName1)
	envNamespace2 := utils.GetEnvironmentNamespace(anyAppName, envName2)
	addRadixBatchWithStatus(radixClient, "test-batch1-job1", jobComponent1, kube.RadixBatchTypeBatch, envNamespace1, v1.BatchConditionTypeCompleted)
	addRadixBatchWithStatus(radixClient, "test-batch2-job1", jobComponent1, kube.RadixBatchTypeBatch, envNamespace1, v1.BatchConditionTypeActive) //to be stopped
	addRadixBatchWithStatus(radixClient, "test-batch3-job1", jobComponent2, kube.RadixBatchTypeBatch, envNamespace1, v1.BatchConditionTypeActive)
	addRadixBatchWithStatus(radixClient, "test-batch4-job1", jobComponent1, kube.RadixBatchTypeJob, envNamespace1, v1.BatchConditionTypeCompleted)
	addRadixBatchWithStatus(radixClient, "test-batch5-job1", jobComponent1, kube.RadixBatchTypeJob, envNamespace1, v1.BatchConditionTypeActive) //to be stopped
	addRadixBatchWithStatus(radixClient, "test-batch6-job1", jobComponent2, kube.RadixBatchTypeJob, envNamespace1, v1.BatchConditionTypeActive)
	addRadixBatch(radixClient, "test-batch7-job1", jobComponent1, kube.RadixBatchTypeBatch, envNamespace1)                                         //to be stopped
	addRadixBatchWithStatus(radixClient, "test-batch8-job1", jobComponent1, kube.RadixBatchTypeBatch, envNamespace1, v1.BatchConditionTypeWaiting) //to be stopped
	addRadixBatch(radixClient, "test-batch9-job1", jobComponent1, kube.RadixBatchTypeBatch, envNamespace2)
	addRadixBatch(radixClient, "test-batch10-job1", jobComponent1, kube.RadixBatchTypeJob, envNamespace1)                                         //to be stopped
	addRadixBatchWithStatus(radixClient, "test-batch11-job1", jobComponent1, kube.RadixBatchTypeJob, envNamespace1, v1.BatchConditionTypeWaiting) //to be stopped
	addRadixBatch(radixClient, "test-batch12-job1", jobComponent1, kube.RadixBatchTypeJob, envNamespace2)

	responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/stop", anyAppName, envName1))
	response := <-responseChannel
	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.Empty(t, response.Body.Bytes())
	batchList, err := radixClient.RadixV1().RadixBatches(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	assert.NoError(t, err)

	rb1, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch1" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch1 should be found")
	assert.Nil(t, rb1.Spec.Jobs[0].Stop, "test-batch1 job should not be stopped")

	rb2, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch2" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch2 should be found")
	assert.Equal(t, pointers.Ptr(true), rb2.Spec.Jobs[0].Stop, "test-batch2 job should be stopped")

	rb3, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch3" && batch.GetNamespace() == envNamespace1 && batch.GetLabels()[kube.RadixComponentLabel] == jobComponent2
	})
	assert.True(t, ok, "test-batch3 should be found")
	assert.Equal(t, pointers.Ptr(true), rb3.Spec.Jobs[0].Stop, "test-batch3 job should be stopped")

	rb4, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch4" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch4 should be found")
	assert.Nil(t, rb4.Spec.Jobs[0].Stop, "test-batch4 single job should not be stopped")

	rb5, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch5" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch5 should be found")
	assert.Equal(t, pointers.Ptr(true), rb5.Spec.Jobs[0].Stop, "test-batch5 job should be stopped")

	rb6, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch6" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch6 should be found")
	assert.Equal(t, pointers.Ptr(true), rb6.Spec.Jobs[0].Stop, "test-batch6 job should be stopped")

	rb7, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch7" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch7 should be found")
	assert.Equal(t, pointers.Ptr(true), rb7.Spec.Jobs[0].Stop, "test-batch7 job should be stopped")

	rb8, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch8" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch8 should be found")
	assert.Equal(t, pointers.Ptr(true), rb8.Spec.Jobs[0].Stop, "test-batch8 job should be stopped")

	rb9, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch9" && batch.GetNamespace() == envNamespace2
	})
	assert.True(t, ok, "test-batch9 should be found")
	assert.Nil(t, rb9.Spec.Jobs[0].Stop, "test-batch9 job should not be stopped")

	rb10, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch10" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch10 should be found")
	assert.Equal(t, pointers.Ptr(true), rb10.Spec.Jobs[0].Stop, "test-batch10 job should be stopped")

	rb11, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch11" && batch.GetNamespace() == envNamespace1
	})
	assert.True(t, ok, "test-batch11 should be found")
	assert.Equal(t, pointers.Ptr(true), rb11.Spec.Jobs[0].Stop, "test-batch11 job should be stopped")

	rb12, ok := slice.FindFirst(batchList.Items, func(batch v1.RadixBatch) bool {
		return batch.GetName() == "test-batch12" && batch.GetNamespace() == envNamespace2
	})
	assert.True(t, ok, "test-batch12 should be found")
	assert.Nil(t, rb12.Spec.Jobs[0].Stop, "test-batch12 job should not be stopped")
}

func assertBatchDeleted(t *testing.T, rc radixclient.Interface, ns, batchName string, deletableBatches []string) {
	updatedBatch, err := rc.RadixV1().RadixBatches(ns).Get(context.Background(), batchName, metav1.GetOptions{})
	if slice.FindIndex(deletableBatches, func(name string) bool { return name == batchName }) == -1 {
		// deletable
		require.NotNil(t, updatedBatch)
		require.Nil(t, err)
	} else {
		// not deletable
		require.Error(t, err)
	}
}

func assertBatchJobStoppedStates(t *testing.T, rc radixclient.Interface, ns, batchName string, stoppableJobs []string) {
	updatedBatch, err := rc.RadixV1().RadixBatches(ns).Get(context.Background(), batchName, metav1.GetOptions{})
	require.Nil(t, err)

	isStopped := func(job *v1.RadixBatchJob) bool {
		if job == nil || job.Stop == nil {
			return false
		}
		return *job.Stop
	}

	for _, job := range updatedBatch.Spec.Jobs {
		if slice.FindIndex(stoppableJobs, func(name string) bool { return name == job.Name }) != -1 {
			assert.True(t, isStopped(&job))
		} else {
			assert.False(t, isStopped(&job))
		}
	}
}

func addRadixBatch(radixClient radixclient.Interface, jobName, componentName string, batchJobType kube.RadixBatchType, namespace string) *v1.RadixBatch {
	return addRadixBatchWithStatus(radixClient, jobName, componentName, batchJobType, namespace, "")
}

func addRadixBatchWithStatus(radixClient radixclient.Interface, jobName, componentName string, batchJobType kube.RadixBatchType, namespace string, batchStatusConditionType v1.RadixBatchConditionType) *v1.RadixBatch {
	rbLabels := make(map[string]string)

	if len(strings.TrimSpace(componentName)) > 0 {
		rbLabels[kube.RadixComponentLabel] = componentName
	}
	rbLabels[kube.RadixBatchTypeLabel] = string(batchJobType)

	batchName, batchJobName, ok := parseBatchAndJobNameFromScheduledJobName(jobName)
	if !ok {
		panic(fmt.Sprintf("invalid job name %s", jobName))
	}
	batch := v1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{
			Name:      batchName,
			Namespace: namespace,
			Labels:    rbLabels,
		},
		Spec: v1.RadixBatchSpec{
			Jobs: []v1.RadixBatchJob{
				{
					Name: batchJobName,
					PayloadSecretRef: &v1.PayloadSecretKeySelector{
						LocalObjectReference: v1.LocalObjectReference{Name: jobName},
						Key:                  jobName,
					},
				},
			},
		},
	}
	if len(batchStatusConditionType) != 0 {
		batch.Status = v1.RadixBatchStatus{
			Condition: v1.RadixBatchCondition{
				Type: batchStatusConditionType,
			},
		}
	}
	radixBatch, err := radixClient.RadixV1().RadixBatches(namespace).Create(
		context.TODO(),
		&batch,
		metav1.CreateOptions{},
	)
	if err != nil {
		panic(err)
	}
	return radixBatch
}

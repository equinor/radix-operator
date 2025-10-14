package jobs

import (
	"context"
	"fmt"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/job-scheduler/internal"
	"github.com/equinor/radix-operator/job-scheduler/internal/test"
	modelsEnv "github.com/equinor/radix-operator/job-scheduler/models"
	models "github.com/equinor/radix-operator/job-scheduler/models/common"
	apiErrors "github.com/equinor/radix-operator/job-scheduler/pkg/errors"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewHandler(t *testing.T) {
	appJobComponent := "compute"
	radixDeployJobComponent := utils.NewDeployJobComponentBuilder().WithName(appJobComponent).BuildJobComponent()
	defer test.Cleanup(t)
	radixClient, kubeClient, kubeUtil := test.SetupTest(t, "app", "qa", appJobComponent, "app-deploy-1", 1)
	env := modelsEnv.NewEnv()

	h := New(kubeUtil, env, &radixDeployJobComponent)
	assert.IsType(t, &jobHandler{}, h)
	actualHandler := h.(*jobHandler)

	assert.Equal(t, kubeUtil, actualHandler.GetKubeUtil())
	assert.Equal(t, env, actualHandler.GetEnv())
	assert.Equal(t, kubeClient, actualHandler.GetKubeUtil().KubeClient())
	assert.Equal(t, radixClient, actualHandler.GetKubeUtil().RadixClient())
}

func TestGetJobs(t *testing.T) {
	appJobComponent := "compute"
	radixDeployJobComponent := utils.NewDeployJobComponentBuilder().WithName(appJobComponent).BuildJobComponent()
	appName, appEnvironment, appComponent, appDeployment := "app", "qa", appJobComponent, "app-deploy-1"
	appNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
	defer test.Cleanup(t)
	radixClient, _, kubeUtil := test.SetupTest(t, appName, appEnvironment, appComponent, appDeployment, 1)
	test.AddRadixBatch(radixClient, "testbatch1-job1", appComponent, kube.RadixBatchTypeJob, appNamespace)
	test.AddRadixBatch(radixClient, "testbatch2-job2", appComponent, kube.RadixBatchTypeJob, appNamespace)
	test.AddRadixBatch(radixClient, "testbatch3-job3", "other-component", kube.RadixBatchTypeJob, appNamespace)
	test.AddRadixBatch(radixClient, "testbatch4-job4", appComponent, "other-type", appNamespace)
	test.AddRadixBatch(radixClient, "testbatch5-job5", appComponent, kube.RadixBatchTypeJob, "app-other")

	handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
	jobs, err := handler.GetJobs(context.TODO())
	assert.Nil(t, err)
	assert.Len(t, jobs, 2)
	job1 := test.GetJobStatusByNameForTest(jobs, "job1")
	assert.NotNil(t, job1)
	assert.Equal(t, "testbatch1-job1", job1.Name)
	assert.Equal(t, "", job1.BatchName)
	job2 := test.GetJobStatusByNameForTest(jobs, "job2")
	assert.NotNil(t, job2)
	assert.Equal(t, "testbatch2-job2", job2.Name)
	assert.Equal(t, "", job2.BatchName)
}

func TestGetJob(t *testing.T) {
	appJobComponent := "compute"
	radixDeployJobComponent := utils.NewDeployJobComponentBuilder().WithName(appJobComponent).BuildJobComponent()

	t.Run("get existing job", func(t *testing.T) {
		appName, appEnvironment, appComponent, appDeployment := "app", "qa", appJobComponent, "app-deploy-1"
		appNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		defer test.Cleanup(t)
		radixClient, _, kubeUtil := test.SetupTest(t, appName, appEnvironment, appComponent, appDeployment, 1)
		test.AddRadixBatch(radixClient, "testbatch1-job1", appComponent, kube.RadixBatchTypeJob, appNamespace)
		test.AddRadixBatch(radixClient, "testbatch2-job1", appComponent, kube.RadixBatchTypeJob, appNamespace)

		radixDeployJobComponent := utils.NewDeployJobComponentBuilder().WithName(appComponent).BuildJobComponent()
		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		job1, err := handler.GetJob(context.TODO(), "testbatch1-job1")
		assert.Nil(t, err)
		assert.Equal(t, "testbatch1-job1", job1.Name)
		assert.Equal(t, "", job1.BatchName)
		job2, err := handler.GetJob(context.TODO(), "testbatch2-job1")
		assert.Nil(t, err)
		assert.Equal(t, "testbatch2-job1", job2.Name)
		assert.Equal(t, "", job2.BatchName)
	})

	t.Run("job in different app namespace", func(t *testing.T) {
		appName, appEnvironment, appComponent, appDeployment, jobName := "app", "qa", appJobComponent, "app-deploy-1", "a-job"
		defer test.Cleanup(t)
		radixClient, _, kubeUtil := test.SetupTest(t, appName, appEnvironment, appComponent, appDeployment, 1)
		test.AddRadixBatch(radixClient, jobName, appComponent, kube.RadixBatchTypeJob, "app-other")

		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		job, err := handler.GetJob(context.TODO(), jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
		assert.Nil(t, job)
	})
}

func TestCreateJob(t *testing.T) {
	appJobComponent := "compute"
	radixDeployJobComponent := utils.NewDeployJobComponentBuilder().WithName(appJobComponent).BuildJobComponent()

	// RD job runAsNonRoot (security context)
	t.Run("job static configuration", func(t *testing.T) {
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", appJobComponent, "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		rd := utils.ARadixDeployment().
			WithDeploymentName(appDeployment).
			WithAppName(appName).
			WithEnvironment(appEnvironment).
			WithComponents().
			WithJobComponents(utils.NewDeployJobComponentBuilder().WithName(appJobComponent)).
			BuildRD()

		defer test.Cleanup(t)
		radixClient, _, kubeUtil := test.SetupTest(t, appName, appEnvironment, appJobComponent, appDeployment, 1)
		_, err := radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		require.NoError(t, err)
		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		jobStatus, err := handler.CreateJob(context.TODO(), &models.JobScheduleDescription{})
		require.Nil(t, err)
		assert.NotNil(t, jobStatus)
		batchName, batchJobName, ok := internal.ParseBatchAndJobNameFromScheduledJobName(jobStatus.Name)
		require.True(t, ok)
		assert.Equal(t, "", jobStatus.BatchName)
		radixBatch, err := radixClient.RadixV1().RadixBatches(envNamespace).Get(context.TODO(), batchName, metav1.GetOptions{})
		require.NoError(t, err)
		assert.Len(t, radixBatch.Labels, 3)
		assert.Equal(t, appName, radixBatch.Labels[kube.RadixAppLabel])
		assert.Equal(t, appJobComponent, radixBatch.Labels[kube.RadixComponentLabel])
		assert.Equal(t, string(kube.RadixBatchTypeJob), radixBatch.Labels[kube.RadixBatchTypeLabel])
		require.Len(t, radixBatch.Spec.Jobs, 1)
		expectedJob := radixv1.RadixBatchJob{Name: batchJobName}
		assert.Equal(t, expectedJob, radixBatch.Spec.Jobs[0])
	})

	t.Run("job with payload path", func(t *testing.T) {
		appName, appEnvironment, appJobComponent, appDeployment, payloadPath, payloadString := "app", "qa", appJobComponent, "app-deploy-1", "path/payload", "the_payload"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		rd := utils.ARadixDeployment().
			WithDeploymentName(appDeployment).
			WithAppName(appName).
			WithEnvironment(appEnvironment).
			WithComponents().
			WithJobComponents(utils.NewDeployJobComponentBuilder().WithName(appJobComponent).WithPayloadPath(&payloadPath)).
			BuildRD()

		defer test.Cleanup(t)
		radixClient, kubeClient, kubeUtil := test.SetupTest(t, appName, appEnvironment, appJobComponent, appDeployment, 1)
		_, err := radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		require.NoError(t, err)
		env := modelsEnv.NewEnv()
		handler := New(kubeUtil, env, &radixDeployJobComponent)
		jobStatus, err := handler.CreateJob(context.TODO(), &models.JobScheduleDescription{Payload: payloadString})
		require.Nil(t, err)
		assert.NotNil(t, jobStatus)
		// Test secret spec
		batchName, jobName, ok := internal.ParseBatchAndJobNameFromScheduledJobName(jobStatus.Name)
		require.True(t, ok)
		radixBatch, err := radixClient.RadixV1().RadixBatches(env.RadixDeploymentNamespace).Get(context.TODO(), batchName, metav1.GetOptions{})
		require.NoError(t, err)
		require.Len(t, radixBatch.Spec.Jobs, 1)
		batchJob := radixBatch.Spec.Jobs[0]
		expectedBatchJob := radixv1.RadixBatchJob{
			Name: jobName,
			PayloadSecretRef: &radixv1.PayloadSecretKeySelector{
				Key: jobName,
				LocalObjectReference: radixv1.LocalObjectReference{
					Name: fmt.Sprintf("%s-payloads-0", batchName),
				},
			},
		}
		assert.Equal(t, expectedBatchJob, batchJob)
		secretName := batchJob.PayloadSecretRef.Name
		secret, err := kubeClient.CoreV1().Secrets(envNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
		require.NoError(t, err)
		assert.Len(t, secret.Labels, 5)
		assert.Equal(t, appName, secret.Labels[kube.RadixAppLabel])
		assert.Equal(t, appJobComponent, secret.Labels[kube.RadixComponentLabel])
		assert.Equal(t, batchName, secret.Labels[kube.RadixBatchNameLabel])
		assert.Equal(t, kube.RadixJobTypeJobSchedule, secret.Labels[kube.RadixJobTypeLabel])
		assert.Equal(t, string(kube.RadixSecretJobPayload), secret.Labels[kube.RadixSecretTypeLabel])
		assert.Equal(t, payloadString, string(secret.Data[batchJob.PayloadSecretRef.Key]))
	})

	t.Run("job without payload path should fail when payload set in request", func(t *testing.T) {
		appName, appEnvironment, appJobComponent, appDeployment, payloadString := "app", "qa", appJobComponent, "app-deploy-1", "the_payload"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		rd := utils.ARadixDeployment().
			WithDeploymentName(appDeployment).
			WithAppName(appName).
			WithEnvironment(appEnvironment).
			WithComponents().
			WithJobComponents(utils.NewDeployJobComponentBuilder().WithName(appJobComponent)).
			BuildRD()

		defer test.Cleanup(t)
		radixClient, _, kubeUtil := test.SetupTest(t, appName, appEnvironment, appJobComponent, appDeployment, 1)
		_, err := radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		require.NoError(t, err)
		env := modelsEnv.NewEnv()
		handler := New(kubeUtil, env, &radixDeployJobComponent)
		_, err = handler.CreateJob(context.TODO(), &models.JobScheduleDescription{Payload: payloadString})
		assert.Error(t, err)
		assert.Equal(t, models.StatusReasonUnknown, apiErrors.ReasonForError(err))
		assert.Contains(t, err.Error(), "missing an expected payload path, but there is a payload in the job")
	})

	t.Run("job resources set in request", func(t *testing.T) {
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", appJobComponent, "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		rd := utils.ARadixDeployment().
			WithDeploymentName(appDeployment).
			WithAppName(appName).
			WithEnvironment(appEnvironment).
			WithComponents().
			WithJobComponents(utils.NewDeployJobComponentBuilder().WithName(appJobComponent)).
			BuildRD()

		defer test.Cleanup(t)
		radixClient, _, kubeUtil := test.SetupTest(t, appName, appEnvironment, appJobComponent, appDeployment, 1)
		_, err := radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		require.NoError(t, err)
		env := modelsEnv.NewEnv()
		handler := New(kubeUtil, env, &radixDeployJobComponent)

		jobRequestConfig := models.JobScheduleDescription{
			RadixJobComponentConfig: models.RadixJobComponentConfig{
				Resources: &models.Resources{
					Requests: models.ResourceList{
						"cpu":    "50m",
						"memory": "60M",
					},
					Limits: models.ResourceList{
						"cpu":    "100m",
						"memory": "120M",
					},
				},
			},
		}
		jobStatus, err := handler.CreateJob(context.TODO(), &jobRequestConfig)
		require.NoError(t, err)
		assert.NotNil(t, jobStatus)

		// Test resources defined
		batchName, jobName, ok := internal.ParseBatchAndJobNameFromScheduledJobName(jobStatus.Name)
		require.True(t, ok)
		radixBatch, err := radixClient.RadixV1().RadixBatches(env.RadixDeploymentNamespace).Get(context.TODO(), batchName, metav1.GetOptions{})
		require.NoError(t, err)
		require.Len(t, radixBatch.Spec.Jobs, 1)
		// Test CPU resource set by request
		expectedJob := radixv1.RadixBatchJob{
			Name: jobName,
			Resources: &radixv1.ResourceRequirements{
				Requests: radixv1.ResourceList{
					"cpu":    "50m",
					"memory": "60M",
				},
				Limits: radixv1.ResourceList{
					"cpu":    "100m",
					"memory": "120M",
				},
			},
		}
		assert.Equal(t, expectedJob, radixBatch.Spec.Jobs[0])
	})

	t.Run("job not defined in RadixDeployment", func(t *testing.T) {
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", appJobComponent, "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		rd := utils.ARadixDeployment().
			WithDeploymentName(appDeployment).
			WithAppName(appName).
			WithEnvironment(appEnvironment).
			WithComponents().
			WithJobComponents(utils.NewDeployJobComponentBuilder().WithName("anotherjob")).
			BuildRD()

		defer test.Cleanup(t)
		radixClient, _, kubeUtil := test.SetupTest(t, appName, appEnvironment, appJobComponent, appDeployment, 1)
		_, err := radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		require.NoError(t, err)
		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		_, err = handler.CreateJob(context.TODO(), &models.JobScheduleDescription{})
		require.Error(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
		assert.Equal(t, apiErrors.NotFoundMessage("job component", appJobComponent), err.Error())
	})

	t.Run("RadixDeployment does not exist", func(t *testing.T) {
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", appJobComponent, "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		rd := utils.ARadixDeployment().
			WithDeploymentName("another-deployment").
			WithAppName(appName).
			WithEnvironment(appEnvironment).
			WithComponents().
			WithJobComponents(utils.NewDeployJobComponentBuilder().WithName(appJobComponent)).BuildRD()

		defer test.Cleanup(t)
		radixClient, _, kubeUtil := test.SetupTest(t, appName, appEnvironment, appJobComponent, appDeployment, 1)
		_, err := radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		require.NoError(t, err)
		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		_, err = handler.CreateJob(context.TODO(), &models.JobScheduleDescription{})
		require.Error(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
		assert.Equal(t, apiErrors.NotFoundMessage("radix deployment", appDeployment), err.Error())
	})

	t.Run("job with gpu set in request", func(t *testing.T) {
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", appJobComponent, "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		rd := utils.ARadixDeployment().
			WithDeploymentName(appDeployment).
			WithAppName(appName).
			WithEnvironment(appEnvironment).
			WithComponents().
			WithJobComponents(utils.NewDeployJobComponentBuilder().WithName(appJobComponent)).
			BuildRD()

		defer test.Cleanup(t)
		radixClient, _, kubeUtil := test.SetupTest(t, appName, appEnvironment, appJobComponent, appDeployment, 1)
		_, err := radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		require.NoError(t, err)
		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		jobStatus, err := handler.CreateJob(context.TODO(), &models.JobScheduleDescription{
			RadixJobComponentConfig: models.RadixJobComponentConfig{
				Node: &models.Node{
					Gpu:      "gpu1, gpu2",
					GpuCount: "2",
				},
			},
		})
		require.NoError(t, err)

		batchName, jobName, ok := internal.ParseBatchAndJobNameFromScheduledJobName(jobStatus.Name)
		require.True(t, ok)
		radixBatch, err := radixClient.RadixV1().RadixBatches(envNamespace).Get(context.TODO(), batchName, metav1.GetOptions{})
		require.NoError(t, err)
		assert.Len(t, radixBatch.Spec.Jobs, 1)
		expectedJob := radixv1.RadixBatchJob{
			Name: jobName,
			Node: &radixv1.RadixNode{
				Gpu:      "gpu1, gpu2",
				GpuCount: "2",
			},
		}
		assert.Equal(t, expectedJob, radixBatch.Spec.Jobs[0])

	})

	t.Run("job with failurePolicy", func(t *testing.T) {
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", appJobComponent, "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		rd := utils.ARadixDeployment().
			WithDeploymentName(appDeployment).
			WithAppName(appName).
			WithEnvironment(appEnvironment).
			WithComponents().
			WithJobComponents(utils.NewDeployJobComponentBuilder().WithName(appJobComponent)).
			BuildRD()

		defer test.Cleanup(t)
		radixClient, _, kubeUtil := test.SetupTest(t, appName, appEnvironment, appJobComponent, appDeployment, 1)
		_, err := radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		require.NoError(t, err)
		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		jobStatus, err := handler.CreateJob(context.TODO(), &models.JobScheduleDescription{
			RadixJobComponentConfig: models.RadixJobComponentConfig{
				FailurePolicy: &models.FailurePolicy{
					Rules: []models.FailurePolicyRule{
						{
							Action: models.FailurePolicyRuleActionFailJob,
							OnExitCodes: models.FailurePolicyRuleOnExitCodes{
								Operator: models.FailurePolicyRuleOnExitCodesOpIn,
								Values:   []int32{42},
							},
						},
					},
				},
			},
		})
		require.NoError(t, err)

		batchName, jobName, ok := internal.ParseBatchAndJobNameFromScheduledJobName(jobStatus.Name)
		require.True(t, ok)
		radixBatch, err := radixClient.RadixV1().RadixBatches(envNamespace).Get(context.TODO(), batchName, metav1.GetOptions{})
		require.NoError(t, err)
		require.Len(t, radixBatch.Spec.Jobs, 1)
		expectedJob := radixv1.RadixBatchJob{
			Name: jobName,
			FailurePolicy: &radixv1.RadixJobComponentFailurePolicy{
				Rules: []radixv1.RadixJobComponentFailurePolicyRule{
					{
						Action: radixv1.RadixJobComponentFailurePolicyActionFailJob,
						OnExitCodes: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes{
							Operator: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOpIn,
							Values:   []int32{42},
						},
					},
				},
			},
		}
		assert.Equal(t, expectedJob, radixBatch.Spec.Jobs[0])
	})

}

func TestDeleteJob(t *testing.T) {
	appJobComponent := "compute"
	radixDeployJobComponent := utils.NewDeployJobComponentBuilder().WithName(appJobComponent).BuildJobComponent()
	t.Run("delete job - cleanup resources for job", func(t *testing.T) {
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", appJobComponent, "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		defer test.Cleanup(t)
		radixClient, kubeClient, kubeUtil := test.SetupTest(t, appName, appEnvironment, appJobComponent, appDeployment, 1)
		radixBatch1 := test.AddRadixBatch(radixClient, "test-batch1-job1", appJobComponent, kube.RadixBatchTypeJob, envNamespace)
		radixBatch2 := test.AddRadixBatch(radixClient, "test-batch2-job1", appJobComponent, kube.RadixBatchTypeJob, envNamespace)
		test.CreateSecretForTest(appName, radixBatch1.Spec.Jobs[0].PayloadSecretRef.Name, "test-batch1-job1", appJobComponent, envNamespace, kubeClient)
		test.CreateSecretForTest(appName, radixBatch2.Spec.Jobs[0].PayloadSecretRef.Name, "test-batch2-job1", appJobComponent, envNamespace, kubeClient)
		test.CreateSecretForTest(appName, "secret3", "test-batch3-job1", appJobComponent, envNamespace, kubeClient)
		test.CreateSecretForTest(appName, "secret4", "test-batch4-job1", "other-job-component", envNamespace, kubeClient)
		test.CreateSecretForTest(appName, "secret5", "test-batch5-job1", appJobComponent, "other-ns", kubeClient)

		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		err := handler.DeleteJob(context.TODO(), "test-batch1-job1")
		assert.Nil(t, err)
		radixBatchList, _ := radixClient.RadixV1().RadixBatches("").List(context.TODO(), metav1.ListOptions{})
		assert.Len(t, radixBatchList.Items, 1)
		assert.NotNil(t, test.GetRadixBatchByNameForTest(radixBatchList.Items, "test-batch2-job1"))
		secrets, _ := kubeClient.CoreV1().Secrets("").List(context.TODO(), metav1.ListOptions{})
		assert.Len(t, secrets.Items, 3)
		assert.NotNil(t, test.GetSecretByNameForTest(secrets.Items, radixBatch2.Spec.Jobs[0].PayloadSecretRef.Name), "remaining test-batch2-job1")
		assert.NotNil(t, test.GetSecretByNameForTest(secrets.Items, "secret4"), "other-job-component")
		assert.NotNil(t, test.GetSecretByNameForTest(secrets.Items, "secret5"), "other-ns")
	})

	t.Run("delete job - job name does not exist", func(t *testing.T) {
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", appJobComponent, "app-deploy-1", "test-batch-job1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		defer test.Cleanup(t)
		radixClient, _, kubeUtil := test.SetupTest(t, appName, appEnvironment, appJobComponent, appDeployment, 1)
		test.AddRadixBatch(radixClient, "test-batch-anotherjob", appJobComponent, kube.RadixBatchTypeJob, envNamespace)

		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		err := handler.DeleteJob(context.TODO(), jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})

	t.Run("delete job - another job component name", func(t *testing.T) {
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", appJobComponent, "app-deploy-1", "test-batch-job1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		defer test.Cleanup(t)
		radixClient, _, kubeUtil := test.SetupTest(t, appName, appEnvironment, appJobComponent, appDeployment, 1)
		test.AddRadixBatch(radixClient, "test-batch-anotherjob", "anotherjob-component", kube.RadixBatchTypeJob, envNamespace)

		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		err := handler.DeleteJob(context.TODO(), jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})

	t.Run("delete job - another job type", func(t *testing.T) {
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", appJobComponent, "app-deploy-1", "test-batch-job1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		defer test.Cleanup(t)
		radixClient, _, kubeUtil := test.SetupTest(t, appName, appEnvironment, appJobComponent, appDeployment, 1)
		test.AddRadixBatch(radixClient, "test-batch-anotherjob", appJobComponent, kube.RadixBatchTypeBatch, envNamespace)

		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		err := handler.DeleteJob(context.TODO(), jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonInvalid, apiErrors.ReasonForError(err))
	})

	t.Run("delete job - another namespace", func(t *testing.T) {
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", appJobComponent, "app-deploy-1", "test-batch-job1"
		defer test.Cleanup(t)
		radixClient, _, kubeUtil := test.SetupTest(t, appName, appEnvironment, appJobComponent, appDeployment, 1)
		test.AddRadixBatch(radixClient, jobName, appJobComponent, kube.RadixBatchTypeJob, "another-ns")

		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		err := handler.DeleteJob(context.TODO(), jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})
}
func TestStopJob(t *testing.T) {
	const (
		appName       = "app"
		envName1      = "qa"
		envName2      = "dev"
		jobComponent1 = "compute1"
		jobComponent2 = "compute2"
		appDeployment = "app-deploy-1"
	)
	radixDeployJobComponent := utils.NewDeployJobComponentBuilder().WithName(jobComponent1).BuildJobComponent()
	t.Run("stop job - cleanup resources for job", func(t *testing.T) {
		envNamespace := utils.GetEnvironmentNamespace(appName, envName1)
		defer test.Cleanup(t)
		radixClient, kubeClient, kubeUtil := test.SetupTest(t, appName, envName1, jobComponent1, appDeployment, 1)
		radixBatch1 := test.AddRadixBatch(radixClient, "test-batch1-job1", jobComponent1, kube.RadixBatchTypeJob, envNamespace)
		radixBatch2 := test.AddRadixBatch(radixClient, "test-batch2-job1", jobComponent1, kube.RadixBatchTypeJob, envNamespace)
		test.CreateSecretForTest(appName, radixBatch1.Spec.Jobs[0].PayloadSecretRef.Name, "test-batch1-job1", jobComponent1, envNamespace, kubeClient)
		test.CreateSecretForTest(appName, radixBatch2.Spec.Jobs[0].PayloadSecretRef.Name, "test-batch2-job1", jobComponent1, envNamespace, kubeClient)
		test.CreateSecretForTest(appName, "secret3", "test-batch3-job1", jobComponent1, envNamespace, kubeClient)
		test.CreateSecretForTest(appName, "secret4", "test-batch4-job1", "other-job-component", envNamespace, kubeClient)
		test.CreateSecretForTest(appName, "secret5", "test-batch5-job1", jobComponent1, "other-ns", kubeClient)

		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		err := handler.StopJob(context.TODO(), "test-batch1-job1")
		assert.Nil(t, err)
		radixBatchList, _ := radixClient.RadixV1().RadixBatches("").List(context.TODO(), metav1.ListOptions{})
		assert.Len(t, radixBatchList.Items, 2)
		radixBatch1 = test.GetRadixBatchByNameForTest(radixBatchList.Items, "test-batch1-job1")
		assert.NotNil(t, radixBatch1)
		assert.NotNil(t, test.GetRadixBatchByNameForTest(radixBatchList.Items, "test-batch2-job1"))
		for _, jobStatus := range radixBatch1.Status.JobStatuses {
			assert.Equal(t, radixv1.BatchJobPhaseStopped, jobStatus.Phase)
		}
		for _, job := range radixBatch1.Spec.Jobs {
			assert.NotNil(t, job.Stop)
			assert.Equal(t, pointers.Ptr(true), job.Stop)
		}
		secrets, _ := kubeClient.CoreV1().Secrets("").List(context.TODO(), metav1.ListOptions{})
		assert.Len(t, secrets.Items, 5)
		assert.NotNil(t, test.GetSecretByNameForTest(secrets.Items, radixBatch1.Spec.Jobs[0].PayloadSecretRef.Name))
		assert.NotNil(t, test.GetSecretByNameForTest(secrets.Items, radixBatch2.Spec.Jobs[0].PayloadSecretRef.Name))
		assert.NotNil(t, test.GetSecretByNameForTest(secrets.Items, "secret3"))
		assert.NotNil(t, test.GetSecretByNameForTest(secrets.Items, "secret4"))
		assert.NotNil(t, test.GetSecretByNameForTest(secrets.Items, "secret5"))
	})

	t.Run("stop job - job name does not exist", func(t *testing.T) {
		envNamespace := utils.GetEnvironmentNamespace(appName, envName1)
		defer test.Cleanup(t)
		radixClient, _, kubeUtil := test.SetupTest(t, appName, envName1, jobComponent1, appDeployment, 1)
		test.AddRadixBatch(radixClient, "test-batch-anotherjob", jobComponent1, kube.RadixBatchTypeJob, envNamespace)

		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		err := handler.StopJob(context.TODO(), "test-batch-job1")
		assert.Error(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})

	t.Run("stop job - another job component name", func(t *testing.T) {
		envNamespace := utils.GetEnvironmentNamespace(appName, envName1)
		defer test.Cleanup(t)
		radixClient, _, kubeUtil := test.SetupTest(t, appName, envName1, jobComponent1, appDeployment, 1)
		test.AddRadixBatch(radixClient, "test-batch-anotherjob", "anotherjob-component", kube.RadixBatchTypeJob, envNamespace)

		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		err := handler.StopJob(context.TODO(), "test-batch-job1")
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})

	t.Run("stop job - another job type", func(t *testing.T) {
		envNamespace := utils.GetEnvironmentNamespace(appName, envName1)
		defer test.Cleanup(t)
		radixClient, _, kubeUtil := test.SetupTest(t, appName, envName1, jobComponent1, appDeployment, 1)
		test.AddRadixBatch(radixClient, "test-batch-anotherjob", jobComponent1, kube.RadixBatchTypeBatch, envNamespace)

		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		err := handler.StopJob(context.TODO(), "test-batch-job1")
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})

	t.Run("stop job - another namespace", func(t *testing.T) {
		defer test.Cleanup(t)
		radixClient, _, kubeUtil := test.SetupTest(t, appName, envName1, jobComponent1, appDeployment, 1)
		test.AddRadixBatch(radixClient, "test-batch-job1", jobComponent1, kube.RadixBatchTypeJob, "another-ns")

		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		err := handler.StopJob(context.TODO(), "test-batch-job1")
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})

	t.Run("stop all batches", func(t *testing.T) {
		envNamespace1 := utils.GetEnvironmentNamespace(appName, envName1)
		envNamespace2 := utils.GetEnvironmentNamespace(appName, envName2)
		defer test.Cleanup(t)
		radixClient, _, kubeUtil := test.SetupTest(t, appName, envName1, jobComponent1, appDeployment, 1)
		test.AddRadixBatchWithStatus(radixClient, "test-batch1-job1", jobComponent1, kube.RadixBatchTypeJob, envNamespace1, radixv1.BatchConditionTypeCompleted)
		test.AddRadixBatchWithStatus(radixClient, "test-batch2-job1", jobComponent1, kube.RadixBatchTypeJob, envNamespace1, radixv1.BatchConditionTypeActive)
		test.AddRadixBatchWithStatus(radixClient, "test-batch3-job1", jobComponent2, kube.RadixBatchTypeJob, envNamespace1, radixv1.BatchConditionTypeActive)
		test.AddRadixBatchWithStatus(radixClient, "test-batch4-job1", jobComponent1, kube.RadixBatchTypeBatch, envNamespace1, radixv1.BatchConditionTypeActive)
		test.AddRadixBatch(radixClient, "test-batch5-job1", jobComponent1, kube.RadixBatchTypeJob, envNamespace1)
		test.AddRadixBatchWithStatus(radixClient, "test-batch6-job1", jobComponent1, kube.RadixBatchTypeJob, envNamespace1, radixv1.BatchConditionTypeWaiting)
		test.AddRadixBatch(radixClient, "test-batch1-job1", jobComponent1, kube.RadixBatchTypeJob, envNamespace2)

		env := modelsEnv.NewEnv()
		env.RadixAppName = appName
		env.RadixEnvironmentName = envName1
		env.RadixComponentName = jobComponent1
		handler := New(kubeUtil, env, &radixDeployJobComponent)
		err := handler.StopAllJobs(context.TODO())
		assert.NoError(t, err)
		batchList, err := radixClient.RadixV1().RadixBatches(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
		assert.NoError(t, err)
		rb1, ok := slice.FindFirst(batchList.Items, func(batch radixv1.RadixBatch) bool {
			return batch.GetName() == "test-batch1" && batch.GetNamespace() == envNamespace1
		})
		assert.True(t, ok, "test-batch2 should be found")
		assert.Nil(t, rb1.Spec.Jobs[0].Stop, "test-batch1 job should not be stopped")
		rb2, ok := slice.FindFirst(batchList.Items, func(batch radixv1.RadixBatch) bool {
			return batch.GetName() == "test-batch2" && batch.GetNamespace() == envNamespace1
		})
		assert.True(t, ok, "test-batch2 should be found")
		assert.Equal(t, pointers.Ptr(true), rb2.Spec.Jobs[0].Stop, "test-batch2 job should be stopped")
		rb3, ok := slice.FindFirst(batchList.Items, func(batch radixv1.RadixBatch) bool {
			return batch.GetName() == "test-batch3" && batch.GetNamespace() == envNamespace1 && batch.GetLabels()[kube.RadixComponentLabel] == jobComponent2
		})
		assert.True(t, ok, "test-batch3 should be found")
		assert.Nil(t, rb3.Spec.Jobs[0].Stop, "test-batch3 job should not be stopped")
		rb4, ok := slice.FindFirst(batchList.Items, func(batch radixv1.RadixBatch) bool {
			return batch.GetName() == "test-batch4" && batch.GetNamespace() == envNamespace1
		})
		assert.True(t, ok, "test-batch4 should be found")
		assert.Nil(t, rb4.Spec.Jobs[0].Stop, "test-batch4 single job should not be stopped")
		rb5, ok := slice.FindFirst(batchList.Items, func(batch radixv1.RadixBatch) bool {
			return batch.GetName() == "test-batch5" && batch.GetNamespace() == envNamespace1
		})
		assert.True(t, ok, "test-batch5 should be found")
		assert.Equal(t, pointers.Ptr(true), rb5.Spec.Jobs[0].Stop, "test-batch5 job should be stopped")
		rb6, ok := slice.FindFirst(batchList.Items, func(batch radixv1.RadixBatch) bool {
			return batch.GetName() == "test-batch1" && batch.GetNamespace() == envNamespace2
		})
		assert.True(t, ok, "test-batch1 should be found")
		assert.Nil(t, rb6.Spec.Jobs[0].Stop, "test-batch6 job should not be stopped")
	})
}

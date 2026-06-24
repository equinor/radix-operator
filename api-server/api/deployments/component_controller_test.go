package deployments

import (
	"context"
	"fmt"
	"strings"
	"testing"

	radixhttp "github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	deploymentModels "github.com/equinor/radix-operator/api-server/api/deployments/models"
	controllertest "github.com/equinor/radix-operator/api-server/api/test"
	"github.com/equinor/radix-operator/api-server/api/utils"
	"github.com/equinor/radix-operator/api-server/api/utils/labelselector"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func createGetComponentsEndpoint(appName, deployName string) string {
	return fmt.Sprintf("/api/v1/applications/%s/deployments/%s/components", appName, deployName)
}

func TestGetComponents_non_existing_app(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _, _, _ := setupTest(t)

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 404, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, controllertest.AppNotFoundErrorMsg(anyAppName), errorResponse.Message)
}

func TestGetComponents_non_existing_deployment(t *testing.T) {
	commonTestUtils, controllerTestUtils, _, _, _, _, _, _ := setupTest(t)
	_, err := commonTestUtils.ApplyApplication(operatorUtils.
		ARadixApplication().
		WithAppName(anyAppName))
	require.NoError(t, err)

	endpoint := createGetComponentsEndpoint(anyAppName, "any-non-existing-deployment")

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 404, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := deploymentModels.NonExistingDeployment(nil, "any-non-existing-deployment")

	assert.Equal(t, (expectedError.(*radixhttp.Error)).Message, errorResponse.Message)
}

func TestGetComponents_active_deployment(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _, _, _, _, _ := setupTest(t)
	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorUtils.
			ARadixDeployment().
			WithJobComponents(
				operatorUtils.NewDeployJobComponentBuilder().WithName("job")).
			WithComponents(
				operatorUtils.NewDeployComponentBuilder().WithName("app")).
			WithAppName(anyAppName).
			WithEnvironment("dev").
			WithDeploymentName(anyDeployName))
	require.NoError(t, err)

	err = createComponentPod(kubeclient, "pod1", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "app")
	require.NoError(t, err)
	err = createComponentPod(kubeclient, "pod2", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "app")
	require.NoError(t, err)
	err = createComponentPod(kubeclient, "pod3", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "job")
	require.NoError(t, err)

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	err = controllertest.GetResponseBody(response, &components)
	require.NoError(t, err)

	assert.Equal(t, 2, len(components))
	app := getComponentByName("app", components)
	assert.Equal(t, 2, len(app.Replicas)) // nolint:staticcheck // SA1019: Ignore linting deprecated fields
	job := getComponentByName("job", components)
	assert.Equal(t, 1, len(job.Replicas)) // nolint:staticcheck // SA1019: Ignore linting deprecated fields
}

func TestGetComponents_WithVolumeMount_ContainsVolumeMountSecrets(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient, kedaClient, dynamicClient, secretProviderClient, certClient := setupTest(t)
	err := utils.ApplyDeploymentWithSync(client, radixclient, kedaClient, dynamicClient, commonTestUtils, secretProviderClient, certClient, operatorUtils.ARadixDeployment().
		WithAppName("any-app").
		WithEnvironment("prod").
		WithDeploymentName(anyDeployName).
		WithJobComponents(
			operatorUtils.NewDeployJobComponentBuilder().
				WithName("job").
				WithVolumeMounts(
					v1.RadixVolumeMount{
						Name: "jobvol",
						Path: "jobpath",
						BlobFuse2: &v1.RadixBlobFuse2VolumeMount{
							Container: "jobcont",
						},
					},
				),
		).
		WithComponents(
			operatorUtils.NewDeployComponentBuilder().
				WithName("frontend").
				WithPort("http", 8080).
				WithPublicPort("http").
				WithVolumeMounts(
					v1.RadixVolumeMount{
						Name: "somevolumename",
						Path: "some-path",
						BlobFuse2: &v1.RadixBlobFuse2VolumeMount{
							Container: "some-container",
						},
					},
				)))
	require.NoError(t, err)

	// Test
	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	err = controllertest.GetResponseBody(response, &components)
	require.NoError(t, err)

	frontend := getComponentByName("frontend", components)
	secrets := frontend.Secrets
	assert.Equal(t, 2, len(secrets))
	assert.Contains(t, secrets, "frontend-somevolumename-csiazurecreds-accountkey")
	assert.Contains(t, secrets, "frontend-somevolumename-csiazurecreds-accountname")

	job := getComponentByName("job", components)
	secrets = job.Secrets
	assert.Equal(t, 2, len(secrets))
	assert.Contains(t, secrets, "job-jobvol-csiazurecreds-accountkey")
	assert.Contains(t, secrets, "job-jobvol-csiazurecreds-accountname")
}

func TestGetComponents_WithTwoVolumeMounts_ContainsTwoVolumeMountSecrets(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient, kedaClient, dynamicClient, secretProviderClient, certClient := setupTest(t)
	err := utils.ApplyDeploymentWithSync(client, radixclient, kedaClient, dynamicClient, commonTestUtils, secretProviderClient, certClient, operatorUtils.ARadixDeployment().
		WithAppName("any-app").
		WithEnvironment("prod").
		WithDeploymentName(anyDeployName).
		WithJobComponents().
		WithComponents(
			operatorUtils.NewDeployComponentBuilder().
				WithName("frontend").
				WithPort("http", 8080).
				WithPublicPort("http").
				WithVolumeMounts(
					v1.RadixVolumeMount{
						Name: "somevolumename1",
						Path: "some-path1",
						BlobFuse2: &v1.RadixBlobFuse2VolumeMount{
							Container: "some-container1",
						},
					},
					v1.RadixVolumeMount{
						Name: "somevolumename2",
						Path: "some-path2",
						BlobFuse2: &v1.RadixBlobFuse2VolumeMount{
							Container: "some-container2",
						},
					},
				)))
	require.NoError(t, err)

	// Test
	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	err = controllertest.GetResponseBody(response, &components)
	require.NoError(t, err)

	secrets := components[0].Secrets
	assert.Equal(t, 4, len(secrets))
	assert.Contains(t, secrets, "frontend-somevolumename1-csiazurecreds-accountkey")
	assert.Contains(t, secrets, "frontend-somevolumename1-csiazurecreds-accountname")
	assert.Contains(t, secrets, "frontend-somevolumename2-csiazurecreds-accountkey")
	assert.Contains(t, secrets, "frontend-somevolumename2-csiazurecreds-accountname")
}

func TestGetComponents_inactive_deployment(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _, _, _, _, _ := setupTest(t)

	initialDeploymentCreated, _ := radixutils.ParseTimestamp("2018-11-12T11:45:26Z")
	activeDeploymentCreated, _ := radixutils.ParseTimestamp("2018-11-14T11:45:26Z")

	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorUtils.
			ARadixDeployment().
			WithAppName(anyAppName).
			WithEnvironment("dev").
			WithDeploymentName("initial-deployment").
			WithComponents(
				operatorUtils.NewDeployComponentBuilder().WithName("app"),
			).
			WithJobComponents(
				operatorUtils.NewDeployJobComponentBuilder().WithName("job"),
			).
			WithCreated(initialDeploymentCreated).
			WithCondition(v1.DeploymentInactive).
			WithActiveFrom(initialDeploymentCreated).
			WithActiveTo(activeDeploymentCreated))
	require.NoError(t, err)

	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorUtils.
			ARadixDeployment().
			WithAppName(anyAppName).
			WithEnvironment("dev").
			WithDeploymentName("active-deployment").
			WithComponents(
				operatorUtils.NewDeployComponentBuilder().WithName("app"),
			).
			WithJobComponents(
				operatorUtils.NewDeployJobComponentBuilder().WithName("job"),
			).
			WithCreated(activeDeploymentCreated).
			WithCondition(v1.DeploymentActive).
			WithActiveFrom(activeDeploymentCreated))
	require.NoError(t, err)

	err = createComponentPod(kubeclient, "pod1", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "app")
	require.NoError(t, err)
	err = createComponentPod(kubeclient, "pod2", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "job")
	require.NoError(t, err)

	endpoint := createGetComponentsEndpoint(anyAppName, "initial-deployment")

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	err = controllertest.GetResponseBody(response, &components)
	require.NoError(t, err)

	assert.Equal(t, 2, len(components))
	app := getComponentByName("app", components)
	assert.Equal(t, 0, len(app.Replicas)) // nolint:staticcheck // SA1019: Ignore linting deprecated fields
	job := getComponentByName("job", components)
	assert.Equal(t, 0, len(job.Replicas)) // nolint:staticcheck // SA1019: Ignore linting deprecated fields
}

func createComponentPod(kubeclient kubernetes.Interface, podName, namespace, radixAppLabel, radixComponentLabel string) error {
	podSpec := getPodSpec(podName, radixAppLabel, radixComponentLabel, "")
	_, err := kubeclient.CoreV1().Pods(namespace).Create(context.Background(), podSpec, metav1.CreateOptions{})
	return err
}

func getPodSpec(podName, appName, componentName, image string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				kube.RadixComponentLabel: componentName,
				kube.RadixAppLabel:       appName,
			},
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: componentName, Image: image}}},
	}
}

func TestGetComponents_success(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _, _, _, _, _ := setupTest(t)
	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorUtils.
			ARadixDeployment().
			WithAppName(anyAppName).
			WithDeploymentName(anyDeployName))
	require.NoError(t, err)

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	err = controllertest.GetResponseBody(response, &components)
	require.NoError(t, err)

	assert.Equal(t, 2, len(components))
	assert.Nil(t, components[0].HorizontalScalingSummary)
	assert.Nil(t, components[1].HorizontalScalingSummary)
}

func TestGetComponents_ReplicaStatus_Failing(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _, _, _, _, _ := setupTest(t)
	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorUtils.
			ARadixDeployment().
			WithAppName(anyAppName).
			WithEnvironment("dev").
			WithDeploymentName(anyDeployName).
			WithComponents(
				operatorUtils.NewDeployComponentBuilder().WithName("app")).
			WithJobComponents(
				operatorUtils.NewDeployJobComponentBuilder().WithName("job")))
	require.NoError(t, err)

	message1 := "Couldn't find key TEST_SECRET in Secret radix-demo-hello-nodejs-dev/www"
	err = createComponentPodWithContainerState(kubeclient, "pod1", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "app", "some-image:abc", message1, deploymentModels.Failing, true)
	require.NoError(t, err)
	err = createComponentPodWithContainerState(kubeclient, "pod2", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "app", "some-image:abc", message1, deploymentModels.Failing, true)
	require.NoError(t, err)
	message2 := "Couldn't find key TEST_SECRET in Secret radix-demo-hello-nodejs-dev/job"
	err = createComponentPodWithContainerState(kubeclient, "pod3", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "job", "some-image:abc", message2, deploymentModels.Failing, true)
	require.NoError(t, err)

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	err = controllertest.GetResponseBody(response, &components)
	require.NoError(t, err)

	assert.Equal(t, 2, len(components))
	app := getComponentByName("app", components)
	require.Equal(t, 2, len(app.ReplicaList))
	assert.Equal(t, deploymentModels.Failing, app.ReplicaList[0].Status.Status)
	assert.Equal(t, message1, app.ReplicaList[0].StatusMessage)

	job := getComponentByName("job", components)
	require.Equal(t, 1, len(job.ReplicaList))
	assert.Equal(t, deploymentModels.Failing, job.ReplicaList[0].Status.Status)
	assert.Equal(t, message2, job.ReplicaList[0].StatusMessage)
}

func TestGetComponents_ReplicaStatus_Running(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _, _, _, _, _ := setupTest(t)
	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorUtils.
			ARadixDeployment().
			WithAppName(anyAppName).
			WithEnvironment("dev").
			WithDeploymentName(anyDeployName).
			WithComponents(
				operatorUtils.NewDeployComponentBuilder().WithName("app")).
			WithJobComponents(
				operatorUtils.NewDeployJobComponentBuilder().WithName("job")))
	require.NoError(t, err)

	message := ""
	err = createComponentPodWithContainerState(kubeclient, "pod1", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "app", "some-image:abc", message, deploymentModels.Running, true)
	require.NoError(t, err)
	err = createComponentPodWithContainerState(kubeclient, "pod2", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "app", "some-image:abc", message, deploymentModels.Running, true)
	require.NoError(t, err)
	err = createComponentPodWithContainerState(kubeclient, "pod3", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "job", "some-image:abc", message, deploymentModels.Running, true)
	require.NoError(t, err)

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	err = controllertest.GetResponseBody(response, &components)
	require.NoError(t, err)

	assert.Equal(t, 2, len(components))
	app := getComponentByName("app", components)
	assert.Equal(t, 2, len(app.ReplicaList))
	assert.Equal(t, deploymentModels.Running, app.ReplicaList[0].Status.Status)
	assert.Equal(t, message, app.ReplicaList[0].StatusMessage)

	job := getComponentByName("job", components)
	assert.Equal(t, 1, len(job.ReplicaList))
	assert.Equal(t, deploymentModels.Running, job.ReplicaList[0].Status.Status)
	assert.Equal(t, message, job.ReplicaList[0].StatusMessage)
}

func TestGetComponents_ReplicaStatus_Starting(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _, _, _, _, _ := setupTest(t)
	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorUtils.
			ARadixDeployment().
			WithAppName(anyAppName).
			WithEnvironment("dev").
			WithDeploymentName(anyDeployName).
			WithComponents(
				operatorUtils.NewDeployComponentBuilder().WithName("app")).
			WithJobComponents(
				operatorUtils.NewDeployJobComponentBuilder().WithName("job")))
	require.NoError(t, err)

	message := ""
	err = createComponentPodWithContainerState(kubeclient, "pod1", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "app", "some-image:abc", message, deploymentModels.Running, false)
	require.NoError(t, err)
	err = createComponentPodWithContainerState(kubeclient, "pod2", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "app", "some-image:abc", message, deploymentModels.Running, false)
	require.NoError(t, err)
	err = createComponentPodWithContainerState(kubeclient, "pod3", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "job", "some-image:abc", message, deploymentModels.Running, false)
	require.NoError(t, err)

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	err = controllertest.GetResponseBody(response, &components)
	require.NoError(t, err)

	assert.Equal(t, 2, len(components))
	app := getComponentByName("app", components)
	assert.Equal(t, 2, len(app.ReplicaList))
	assert.Equal(t, deploymentModels.Starting, app.ReplicaList[0].Status.Status)
	assert.Equal(t, message, app.ReplicaList[0].StatusMessage)

	job := getComponentByName("job", components)
	assert.Equal(t, 1, len(job.ReplicaList))
	assert.Equal(t, deploymentModels.Starting, job.ReplicaList[0].Status.Status)
	assert.Equal(t, message, job.ReplicaList[0].StatusMessage)
}

func TestGetComponents_ReplicaStatus_Pending(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _, _, _, _, _ := setupTest(t)
	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorUtils.
			ARadixDeployment().
			WithAppName(anyAppName).
			WithEnvironment("dev").
			WithDeploymentName(anyDeployName).
			WithComponents(
				operatorUtils.NewDeployComponentBuilder().WithName("app")).
			WithJobComponents(
				operatorUtils.NewDeployJobComponentBuilder().WithName("job")))
	require.NoError(t, err)

	message := ""
	err = createComponentPodWithContainerState(kubeclient, "pod1", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "app", "some-image:abc", message, deploymentModels.Pending, true)
	require.NoError(t, err)
	err = createComponentPodWithContainerState(kubeclient, "pod2", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "app", "some-image:abc", message, deploymentModels.Pending, true)
	require.NoError(t, err)
	err = createComponentPodWithContainerState(kubeclient, "pod3", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "job", "some-image:abc", message, deploymentModels.Pending, true)
	require.NoError(t, err)

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	err = controllertest.GetResponseBody(response, &components)
	require.NoError(t, err)

	assert.Equal(t, 2, len(components))
	app := getComponentByName("app", components)
	assert.Equal(t, 2, len(app.ReplicaList))
	assert.Equal(t, deploymentModels.Pending, app.ReplicaList[0].Status.Status)
	assert.Equal(t, message, app.ReplicaList[0].StatusMessage)

	job := getComponentByName("job", components)
	assert.Equal(t, 1, len(job.ReplicaList))
	assert.Equal(t, deploymentModels.Pending, job.ReplicaList[0].Status.Status)
	assert.Equal(t, message, job.ReplicaList[0].StatusMessage)
}

func Test_GetComponents_HorizontalScaling_Utilization(t *testing.T) {
	type testScenario struct {
		createScaledObject       bool
		createHPA                bool
		mutateScaledObject       func(*v1alpha1.ScaledObject)
		mutateHPA                func(*v2.HorizontalPodAutoscaler)
		expectCurrentUtilization bool
		expectCurrentReplicas    bool
		expectErrorByType        map[string]string
	}

	testScenarios := map[string]testScenario{
		"valid scaledobject and hpa": {
			createScaledObject:       true,
			createHPA:                true,
			expectCurrentUtilization: true,
			expectCurrentReplicas:    true,
		},
		"missing scaledobject and hpa": {
			createScaledObject:       false,
			createHPA:                false,
			expectCurrentUtilization: false,
			expectCurrentReplicas:    false,
		},
		"valid scaledobject without hpa": {
			createScaledObject:       true,
			createHPA:                false,
			expectCurrentUtilization: false,
			expectCurrentReplicas:    false,
		},
		"scaledobject points to non-existing hpa": {
			createScaledObject: true,
			createHPA:          true,
			mutateScaledObject: func(so *v1alpha1.ScaledObject) {
				so.Status.HpaName = "hpa-does-not-exist"
			},
			expectCurrentUtilization: false,
			expectCurrentReplicas:    false,
		},
		"invalid scaledobject trigger mismatch": {
			createScaledObject: true,
			createHPA:          true,
			mutateScaledObject: func(so *v1alpha1.ScaledObject) {
				so.Spec.Triggers[0].Name = "invalid-trigger"
			},
			expectCurrentUtilization: false,
			expectCurrentReplicas:    true,
		},
		"invalid scaledobject metric names count mismatch": {
			createScaledObject: true,
			createHPA:          true,
			mutateScaledObject: func(so *v1alpha1.ScaledObject) {
				so.Status.ExternalMetricNames = append(so.Status.ExternalMetricNames, "extra")
			},
			expectCurrentUtilization: false,
			expectCurrentReplicas:    true,
		},
		"invalid hpa metric count mismatch": {
			createScaledObject: true,
			createHPA:          true,
			mutateHPA: func(hpa *v2.HorizontalPodAutoscaler) {
				hpa.Status.CurrentMetrics = hpa.Status.CurrentMetrics[:len(hpa.Status.CurrentMetrics)-1]
			},
			expectCurrentUtilization: false,
			expectCurrentReplicas:    true,
		},
		"invalid hpa metric type": {
			createScaledObject: true,
			createHPA:          true,
			mutateHPA: func(hpa *v2.HorizontalPodAutoscaler) {
				hpa.Status.CurrentMetrics[0].Type = v2.ResourceMetricSourceType
			},
			expectCurrentUtilization: false,
			expectCurrentReplicas:    true,
		},
		"external scaler health has failures": {
			createScaledObject: true,
			createHPA:          true,
			mutateScaledObject: func(so *v1alpha1.ScaledObject) {
				for metricName, status := range so.Status.Health {
					if strings.Contains(metricName, "-azure-servicebus-") {
						status.Status = "Failing"
						status.NumberOfFailures = pointers.Ptr[int32](3)
						so.Status.Health[metricName] = status
					}
				}
			},
			expectCurrentUtilization: true,
			expectCurrentReplicas:    true,
			expectErrorByType: map[string]string{
				"azure-servicebus": "Number of failures: 3",
			},
		},
	}

	for name, scenario := range testScenarios {
		t.Run(name, func(t *testing.T) {
			horizontalScaling := createHorizontalScalingConfig()
			commonTestUtils, controllerTestUtils, client, radixclient, kedaClient, dynamicClient, secretProviderClient, certClient := setupTest(t)
			err := utils.ApplyDeploymentWithSync(client, radixclient, kedaClient, dynamicClient, commonTestUtils, secretProviderClient, certClient, operatorUtils.ARadixDeployment().
				WithAppName(anyAppName).
				WithEnvironment("prod").
				WithDeploymentName("dep-hpa").
				WithJobComponents().
				WithComponents(
					operatorUtils.NewDeployComponentBuilder().
						WithName("frontend").
						WithPort("http", 8080).
						WithPublicPort("http").
						WithHorizontalScaling(horizontalScaling)))
			require.NoError(t, err)

			ns := operatorUtils.GetEnvironmentNamespace(anyAppName, "prod")

			// ApplyDeploymentWithSync creates autoscaling objects for the configured component.
			// Remove them to make each subtest deterministic for ScaledObject/HPA presence and validity.
			scaledObjects, err := kedaClient.KedaV1alpha1().ScaledObjects(ns).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			for _, scaledObject := range scaledObjects.Items {
				err = kedaClient.KedaV1alpha1().ScaledObjects(ns).Delete(context.Background(), scaledObject.Name, metav1.DeleteOptions{})
				require.NoError(t, err)
			}

			hpas, err := client.AutoscalingV2().HorizontalPodAutoscalers(ns).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			for _, existingHPA := range hpas.Items {
				err = client.AutoscalingV2().HorizontalPodAutoscalers(ns).Delete(context.Background(), existingHPA.Name, metav1.DeleteOptions{})
				require.NoError(t, err)
			}

			scaledObject, hpa := createHorizontalScalingObjects("frontend", horizontalScaling)

			if scenario.mutateScaledObject != nil {
				scenario.mutateScaledObject(&scaledObject)
			}
			if scenario.mutateHPA != nil {
				scenario.mutateHPA(&hpa)
			}

			if scenario.createScaledObject {
				_, err = kedaClient.KedaV1alpha1().ScaledObjects(ns).Create(context.Background(), &scaledObject, metav1.CreateOptions{})
				require.NoError(t, err)
			}

			if scenario.createHPA {
				_, err = client.AutoscalingV2().HorizontalPodAutoscalers(ns).Create(context.Background(), &hpa, metav1.CreateOptions{})
				require.NoError(t, err)
			}

			endpoint := createGetComponentsEndpoint(anyAppName, "dep-hpa")
			responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
			response := <-responseChannel

			assert.Equal(t, 200, response.Code)

			var components []deploymentModels.Component
			err = controllertest.GetResponseBody(response, &components)
			require.NoError(t, err)
			require.NotEmpty(t, components)

			frontend := getComponentByName("frontend", components)
			require.NotNil(t, frontend)
			require.NotNil(t, frontend.HorizontalScalingSummary)

			summary := frontend.HorizontalScalingSummary

			if scenario.expectCurrentReplicas {
				assert.EqualValues(t, 2, summary.CurrentReplicas)
				assert.EqualValues(t, 4, summary.DesiredReplicas)
			} else {
				assert.Zero(t, summary.CurrentReplicas)
				assert.Zero(t, summary.DesiredReplicas)
			}

			currentByType := map[string]string{}
			errorByType := map[string]string{}
			for _, trigger := range summary.Triggers {
				currentByType[trigger.Type] = trigger.CurrentUtilization
				errorByType[trigger.Type] = trigger.Error
			}

			if scenario.expectCurrentUtilization {
				assert.Equal(t, "60", currentByType["cpu"])
				assert.Equal(t, "75", currentByType["memory"])
				assert.Equal(t, "5", currentByType["cron"])
				assert.Equal(t, "15", currentByType["azure-servicebus"])
				assert.Equal(t, "20", currentByType["azure-eventhub"])
			} else {
				for _, trigger := range summary.Triggers {
					assert.Empty(t, trigger.CurrentUtilization)
				}
			}

			for _, trigger := range summary.Triggers {
				expectedError := ""
				if scenario.expectErrorByType != nil {
					expectedError = scenario.expectErrorByType[trigger.Type]
				}

				assert.Equal(t, expectedError, errorByType[trigger.Type])
			}
		})
	}
}

func createHorizontalScalingConfig() *v1.RadixHorizontalScaling {
	return &v1.RadixHorizontalScaling{
		MinReplicas: pointers.Ptr[int32](2),
		MaxReplicas: 6,
		Triggers: []v1.RadixHorizontalScalingTrigger{
			{Name: "cpu", Cpu: &v1.RadixHorizontalScalingCPUTrigger{Value: 60}},
			{Name: "memory", Memory: &v1.RadixHorizontalScalingMemoryTrigger{Value: 75}},
			{Name: "cron", Cron: &v1.RadixHorizontalScalingCronTrigger{Start: "0 8 * * 1-5", End: "0 16 * * 1-5", Timezone: "Europe/Oslo", DesiredReplicas: 5}},
			{Name: "servicebus", AzureServiceBus: &v1.RadixHorizontalScalingAzureServiceBusTrigger{Namespace: "ns-prod", QueueName: "orders", MessageCount: pointers.Ptr(15), Authentication: v1.RadixHorizontalScalingAuthentication{Identity: v1.RadixHorizontalScalingRequiredIdentity{Azure: v1.AzureIdentity{ClientId: "service-bus-client-id"}}}}},
			{Name: "eventhub", AzureEventHub: &v1.RadixHorizontalScalingAzureEventHubTrigger{UnprocessedEventThreshold: pointers.Ptr(20), EventHubNamespace: "ehns", EventHubName: "orders", StorageAccount: "storage", Container: "container", Authentication: &v1.RadixHorizontalScalingAuthentication{Identity: v1.RadixHorizontalScalingRequiredIdentity{Azure: v1.AzureIdentity{ClientId: "event-hub-client-id"}}}}},
		},
	}
}

func createHorizontalScalingObjects(name string, scaling *v1.RadixHorizontalScaling) (v1alpha1.ScaledObject, v2.HorizontalPodAutoscaler) {
	var triggers []v1alpha1.ScaleTriggers
	resourceMetricNames := []string{}
	externalMetricNames := []string{}
	health := map[string]v1alpha1.HealthStatus{}
	externalMetricStatus := []v2.MetricStatus{}
	resourceMetricStatus := []v2.MetricStatus{}

	for triggerIndex, trigger := range scaling.Triggers {
		scaleTrigger := v1alpha1.ScaleTriggers{
			Type: trigger.Type(),
			Name: trigger.Name,
		}

		switch trigger.Type() {
		case "cpu":
			value := int32(trigger.Cpu.Value)
			scaleTrigger.Metadata = map[string]string{"value": fmt.Sprintf("%d", trigger.Cpu.Value)}
			scaleTrigger.MetricType = "Utilization"
			resourceMetricNames = append(resourceMetricNames, "cpu")
			resourceMetricStatus = append(resourceMetricStatus, v2.MetricStatus{
				Type: v2.ResourceMetricSourceType,
				Resource: &v2.ResourceMetricStatus{
					Name: "cpu",
					Current: v2.MetricValueStatus{
						AverageUtilization: &value,
					},
				},
			})
		case "memory":
			value := int32(trigger.Memory.Value)
			scaleTrigger.Metadata = map[string]string{"value": fmt.Sprintf("%d", trigger.Memory.Value)}
			scaleTrigger.MetricType = "Utilization"
			resourceMetricNames = append(resourceMetricNames, "memory")
			resourceMetricStatus = append(resourceMetricStatus, v2.MetricStatus{
				Type: v2.ResourceMetricSourceType,
				Resource: &v2.ResourceMetricStatus{
					Name: "memory",
					Current: v2.MetricValueStatus{
						AverageUtilization: &value,
					},
				},
			})
		case "cron":
			externalMetricName := fmt.Sprintf("s%d-cron-Europe-Oslo-08xx1-5-016xx1-5", triggerIndex)
			scaleTrigger.Metadata = map[string]string{
				"end":             trigger.Cron.End,
				"start":           trigger.Cron.Start,
				"timezone":        trigger.Cron.Timezone,
				"desiredReplicas": fmt.Sprintf("%d", trigger.Cron.DesiredReplicas),
			}
			externalMetricNames = append(externalMetricNames, externalMetricName)
			health[externalMetricName] = v1alpha1.HealthStatus{NumberOfFailures: pointers.Ptr[int32](0), Status: "Happy"}
			externalMetricStatus = append(externalMetricStatus, v2.MetricStatus{
				Type: v2.ExternalMetricSourceType,
				External: &v2.ExternalMetricStatus{
					Current: v2.MetricValueStatus{AverageValue: resource.NewQuantity(int64(trigger.Cron.DesiredReplicas), resource.DecimalSI)},
					Metric:  v2.MetricIdentifier{Name: externalMetricName},
				},
			})
		case "azure-servicebus":
			messageCount := 5
			if trigger.AzureServiceBus.MessageCount != nil {
				messageCount = *trigger.AzureServiceBus.MessageCount
			}
			externalMetricName := fmt.Sprintf("s%d-azure-servicebus-orders", triggerIndex)
			scaleTrigger.Metadata = map[string]string{"messageCount": fmt.Sprintf("%d", messageCount)}
			externalMetricNames = append(externalMetricNames, externalMetricName)
			health[externalMetricName] = v1alpha1.HealthStatus{NumberOfFailures: pointers.Ptr[int32](0), Status: "Happy"}
			externalMetricStatus = append(externalMetricStatus, v2.MetricStatus{
				Type: v2.ExternalMetricSourceType,
				External: &v2.ExternalMetricStatus{
					Current: v2.MetricValueStatus{AverageValue: resource.NewQuantity(int64(messageCount), resource.DecimalSI)},
					Metric:  v2.MetricIdentifier{Name: externalMetricName},
				},
			})
		case "azure-eventhub":
			threshold := 64
			if trigger.AzureEventHub.UnprocessedEventThreshold != nil {
				threshold = *trigger.AzureEventHub.UnprocessedEventThreshold
			}
			externalMetricName := fmt.Sprintf("s%d-azure-eventhub-orders", triggerIndex)
			scaleTrigger.Metadata = map[string]string{"unprocessedEventThreshold": fmt.Sprintf("%d", threshold)}
			externalMetricNames = append(externalMetricNames, externalMetricName)
			health[externalMetricName] = v1alpha1.HealthStatus{NumberOfFailures: pointers.Ptr[int32](0), Status: "Happy"}
			externalMetricStatus = append(externalMetricStatus, v2.MetricStatus{
				Type: v2.ExternalMetricSourceType,
				External: &v2.ExternalMetricStatus{
					Current: v2.MetricValueStatus{AverageValue: resource.NewQuantity(int64(threshold), resource.DecimalSI)},
					Metric:  v2.MetricIdentifier{Name: externalMetricName},
				},
			})
		}

		triggers = append(triggers, scaleTrigger)
	}

	metricStatus := append(externalMetricStatus, resourceMetricStatus...)

	scaler := v1alpha1.ScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labelselector.ForComponent(anyAppName, "frontend"),
		},
		Spec: v1alpha1.ScaledObjectSpec{
			MinReplicaCount: scaling.MinReplicas,
			MaxReplicaCount: &scaling.MaxReplicas,
			Triggers:        triggers,
		},
		Status: v1alpha1.ScaledObjectStatus{
			HpaName:             fmt.Sprintf("hpa-%s", name),
			Health:              health,
			ResourceMetricNames: resourceMetricNames,
			ExternalMetricNames: externalMetricNames,
		},
	}

	hpa := v2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:   fmt.Sprintf("hpa-%s", name),
			Labels: labelselector.ForComponent(anyAppName, "frontend"),
		},
		Spec: v2.HorizontalPodAutoscalerSpec{
			MinReplicas: scaling.MinReplicas,
			MaxReplicas: scaling.MaxReplicas,
		},
		Status: v2.HorizontalPodAutoscalerStatus{
			CurrentMetrics:  metricStatus,
			CurrentReplicas: 2,
			DesiredReplicas: 4,
		},
	}

	return scaler, hpa
}

func TestGetComponents_WithIdentity(t *testing.T) {
	// Setup
	const (
		appName = "any-app"
		envName = "prod"
	)

	commonTestUtils, controllerTestUtils, client, radixclient, kedaClient, dynamicClient, secretProviderClient, certClient := setupTest(t)

	err := utils.ApplyDeploymentWithSync(client, radixclient, kedaClient, dynamicClient, commonTestUtils, secretProviderClient, certClient, operatorUtils.ARadixDeployment().
		WithAppName(appName).
		WithEnvironment(envName).
		WithDeploymentName(anyDeployName).
		WithJobComponents(
			operatorUtils.NewDeployJobComponentBuilder().
				WithName("job1").
				WithIdentity(&v1.Identity{Azure: &v1.AzureIdentity{ClientId: "job-clientid"}}).
				WithSecretRefs(v1.RadixSecretRefs{AzureKeyVaults: []v1.RadixAzureKeyVault{{Name: "job-key-vault1", Items: []v1.RadixAzureKeyVaultItem{{Name: "secret1"}}}}}).
				WithSecretRefs(v1.RadixSecretRefs{AzureKeyVaults: []v1.RadixAzureKeyVault{{Name: "job-key-vault2", Items: []v1.RadixAzureKeyVaultItem{{Name: "secret2"}}, UseAzureIdentity: pointers.Ptr(false)}}}).
				WithSecretRefs(v1.RadixSecretRefs{AzureKeyVaults: []v1.RadixAzureKeyVault{{Name: "job-key-vault3", Items: []v1.RadixAzureKeyVaultItem{{Name: "secret3"}}, UseAzureIdentity: pointers.Ptr(true)}}}),
			operatorUtils.NewDeployJobComponentBuilder().WithName("job2"),
		).
		WithComponents(
			operatorUtils.NewDeployComponentBuilder().
				WithName("comp1").
				WithIdentity(&v1.Identity{Azure: &v1.AzureIdentity{ClientId: "comp-clientid"}}).
				WithSecretRefs(v1.RadixSecretRefs{AzureKeyVaults: []v1.RadixAzureKeyVault{{Name: "comp-key-vault1", Items: []v1.RadixAzureKeyVaultItem{{Name: "secret1"}}}}}).
				WithSecretRefs(v1.RadixSecretRefs{AzureKeyVaults: []v1.RadixAzureKeyVault{{Name: "comp-key-vault2", Items: []v1.RadixAzureKeyVaultItem{{Name: "secret2"}}, UseAzureIdentity: pointers.Ptr(false)}}}).
				WithSecretRefs(v1.RadixSecretRefs{AzureKeyVaults: []v1.RadixAzureKeyVault{{Name: "comp-key-vault3", Items: []v1.RadixAzureKeyVaultItem{{Name: "secret3"}}, UseAzureIdentity: pointers.Ptr(true)}}}),
			operatorUtils.NewDeployComponentBuilder().WithName("comp2"),
		))
	require.NoError(t, err)

	// Test
	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	err = controllertest.GetResponseBody(response, &components)
	require.NoError(t, err)

	assert.Equal(t,
		&deploymentModels.Identity{Azure: &deploymentModels.AzureIdentity{
			ClientId:           "job-clientid",
			ServiceAccountName: operatorUtils.GetComponentServiceAccountName("job1"),
			Namespace:          operatorUtils.GetEnvironmentNamespace(appName, envName),
			AzureKeyVaults:     []string{"job-key-vault3"}}},
		getComponentByName("job1", components).Identity)
	assert.Nil(t, getComponentByName("job2", components).Identity)
	assert.Equal(t,
		&deploymentModels.Identity{Azure: &deploymentModels.AzureIdentity{
			ClientId:           "comp-clientid",
			ServiceAccountName: operatorUtils.GetComponentServiceAccountName("comp1"),
			Namespace:          operatorUtils.GetEnvironmentNamespace(appName, envName),
			AzureKeyVaults:     []string{"comp-key-vault3"}}},
		getComponentByName("comp1", components).Identity)
	assert.Nil(t, getComponentByName("comp2", components).Identity)
}

func TestGetComponents_ReplicaStatus_WithImageFromSpec(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _, _, _, _, _ := setupTest(t)
	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorUtils.
			ARadixDeployment().
			WithAppName(anyAppName).
			WithEnvironment("dev").
			WithDeploymentName(anyDeployName).
			WithComponents(
				operatorUtils.NewDeployComponentBuilder().WithName("app")).
			WithJobComponents(
				operatorUtils.NewDeployJobComponentBuilder().WithName("job")))
	require.NoError(t, err)

	podSpec := getPodSpec("pod1", anyAppName, "app", "some-image:abc")
	containerState := getContainerState("", deploymentModels.Running)
	podStatus := corev1.PodStatus{
		ContainerStatuses: []corev1.ContainerStatus{
			{
				State:   containerState,
				Ready:   true,
				Image:   "some-image:abc-updated",
				ImageID: "some-image-id",
			},
		},
	}
	podSpec.Status = podStatus
	namespace := operatorUtils.GetEnvironmentNamespace(anyAppName, "dev")
	_, err = kubeclient.CoreV1().Pods(namespace).Create(context.Background(), podSpec, metav1.CreateOptions{})
	require.NoError(t, err)

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	err = controllertest.GetResponseBody(response, &components)
	require.NoError(t, err)

	app := getComponentByName("app", components)
	assert.NotNil(t, app, "App component should be present")
	assert.Equal(t, 1, len(app.ReplicaList), "There should be one replica for the app component")
	assert.Equal(t, "some-image:abc", app.ReplicaList[0].Image, "Image should be from spec, not pod status")
	assert.Equal(t, "some-image-id", app.ReplicaList[0].ImageId, "ImageId should be from pod status")
}

func createComponentPodWithContainerState(kubeclient kubernetes.Interface, podName, namespace, appName, componentName, image, message string, status deploymentModels.ContainerStatus, ready bool) error {
	podSpec := getPodSpec(podName, appName, componentName, image)
	containerState := getContainerState(message, status)
	podStatus := corev1.PodStatus{
		ContainerStatuses: []corev1.ContainerStatus{
			{
				State: containerState,
				Ready: ready,
			},
		},
	}
	podSpec.Status = podStatus

	_, err := kubeclient.CoreV1().Pods(namespace).Create(context.Background(), podSpec, metav1.CreateOptions{})
	return err
}

func getContainerState(message string, status deploymentModels.ContainerStatus) corev1.ContainerState {
	var containerState corev1.ContainerState

	if status == deploymentModels.Failing {
		containerState = corev1.ContainerState{
			Waiting: &corev1.ContainerStateWaiting{
				Message: message,
				Reason:  "",
			},
		}
	}
	if status == deploymentModels.Pending {
		containerState = corev1.ContainerState{
			Waiting: &corev1.ContainerStateWaiting{
				Message: message,
				Reason:  "ContainerCreating",
			},
		}
	}
	if status == deploymentModels.Running {
		containerState = corev1.ContainerState{
			Running: &corev1.ContainerStateRunning{},
		}
	}
	if status == deploymentModels.Terminated {
		containerState = corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				Message: message,
			},
		}
	}

	return containerState
}

func Test_HorizontalScalingSummary_Identity(t *testing.T) {
	type testScenario struct {
		triggers           []v1.RadixHorizontalScalingTrigger
		expectedIdentities map[string]*deploymentModels.Identity // key is trigger name
	}

	testScenarios := map[string]testScenario{
		"cpu trigger has no identity": {
			triggers: []v1.RadixHorizontalScalingTrigger{
				{Name: "cpu", Cpu: &v1.RadixHorizontalScalingCPUTrigger{Value: 60}},
			},
			expectedIdentities: map[string]*deploymentModels.Identity{
				"cpu": nil,
			},
		},
		"memory trigger has no identity": {
			triggers: []v1.RadixHorizontalScalingTrigger{
				{Name: "memory", Memory: &v1.RadixHorizontalScalingMemoryTrigger{Value: 75}},
			},
			expectedIdentities: map[string]*deploymentModels.Identity{
				"memory": nil,
			},
		},
		"cron trigger has no identity": {
			triggers: []v1.RadixHorizontalScalingTrigger{
				{Name: "cron", Cron: &v1.RadixHorizontalScalingCronTrigger{Start: "0 8 * * 1-5", End: "0 16 * * 1-5", Timezone: "Europe/Oslo", DesiredReplicas: 5}},
			},
			expectedIdentities: map[string]*deploymentModels.Identity{
				"cron": nil,
			},
		},
		"azure-servicebus trigger with identity": {
			triggers: []v1.RadixHorizontalScalingTrigger{
				{Name: "servicebus", AzureServiceBus: &v1.RadixHorizontalScalingAzureServiceBusTrigger{
					Namespace:    "ns-prod",
					QueueName:    "orders",
					MessageCount: pointers.Ptr(15),
					Authentication: v1.RadixHorizontalScalingAuthentication{
						Identity: v1.RadixHorizontalScalingRequiredIdentity{
							Azure: v1.AzureIdentity{ClientId: "service-bus-client-id"},
						},
					},
				}},
			},
			expectedIdentities: map[string]*deploymentModels.Identity{
				"servicebus": {
					Azure: &deploymentModels.AzureIdentity{
						ClientId:           "service-bus-client-id",
						ServiceAccountName: "keda-operator",
						Namespace:          "keda",
					},
				},
			},
		},
		"azure-servicebus trigger without identity": {
			triggers: []v1.RadixHorizontalScalingTrigger{
				{Name: "servicebus", AzureServiceBus: &v1.RadixHorizontalScalingAzureServiceBusTrigger{
					Namespace:    "ns-prod",
					QueueName:    "orders",
					MessageCount: pointers.Ptr(15),
				}},
			},
			expectedIdentities: map[string]*deploymentModels.Identity{
				"servicebus": nil,
			},
		},
		"azure-eventhub trigger with identity": {
			triggers: []v1.RadixHorizontalScalingTrigger{
				{Name: "eventhub", AzureEventHub: &v1.RadixHorizontalScalingAzureEventHubTrigger{
					UnprocessedEventThreshold: pointers.Ptr(20),
					EventHubNamespace:         "ehns",
					EventHubName:              "orders",
					StorageAccount:            "storage",
					Container:                 "container",
					Authentication: &v1.RadixHorizontalScalingAuthentication{
						Identity: v1.RadixHorizontalScalingRequiredIdentity{
							Azure: v1.AzureIdentity{ClientId: "event-hub-client-id"},
						},
					},
				}},
			},
			expectedIdentities: map[string]*deploymentModels.Identity{
				"eventhub": {
					Azure: &deploymentModels.AzureIdentity{
						ClientId:           "event-hub-client-id",
						ServiceAccountName: "keda-operator",
						Namespace:          "keda",
					},
				},
			},
		},
		"azure-eventhub trigger without identity": {
			triggers: []v1.RadixHorizontalScalingTrigger{
				{Name: "eventhub", AzureEventHub: &v1.RadixHorizontalScalingAzureEventHubTrigger{
					UnprocessedEventThreshold: pointers.Ptr(20),
					EventHubNamespace:         "ehns",
					EventHubName:              "orders",
					StorageAccount:            "storage",
					Container:                 "container",
				}},
			},
			expectedIdentities: map[string]*deploymentModels.Identity{
				"eventhub": nil,
			},
		},
		"multiple triggers with mixed identities": {
			triggers: []v1.RadixHorizontalScalingTrigger{
				{Name: "cpu", Cpu: &v1.RadixHorizontalScalingCPUTrigger{Value: 60}},
				{Name: "servicebus", AzureServiceBus: &v1.RadixHorizontalScalingAzureServiceBusTrigger{
					Namespace:    "ns-prod",
					QueueName:    "orders",
					MessageCount: pointers.Ptr(15),
					Authentication: v1.RadixHorizontalScalingAuthentication{
						Identity: v1.RadixHorizontalScalingRequiredIdentity{
							Azure: v1.AzureIdentity{ClientId: "service-bus-client-id"},
						},
					},
				}},
				{Name: "eventhub", AzureEventHub: &v1.RadixHorizontalScalingAzureEventHubTrigger{
					UnprocessedEventThreshold: pointers.Ptr(20),
					EventHubNamespace:         "ehns",
					EventHubName:              "orders",
					StorageAccount:            "storage",
					Container:                 "container",
				}},
			},
			expectedIdentities: map[string]*deploymentModels.Identity{
				"cpu": nil,
				"servicebus": {
					Azure: &deploymentModels.AzureIdentity{
						ClientId:           "service-bus-client-id",
						ServiceAccountName: "keda-operator",
						Namespace:          "keda",
					},
				},
				"eventhub": nil,
			},
		},
	}

	for name, scenario := range testScenarios {
		t.Run(name, func(t *testing.T) {
			commonTestUtils, controllerTestUtils, client, radixclient, kedaClient, dynamicClient, secretProviderClient, certClient := setupTest(t)
			horizontalScaling := &v1.RadixHorizontalScaling{
				MinReplicas: pointers.Ptr[int32](1),
				MaxReplicas: 5,
				Triggers:    scenario.triggers,
			}

			err := utils.ApplyDeploymentWithSync(client, radixclient, kedaClient, dynamicClient, commonTestUtils, secretProviderClient, certClient, operatorUtils.ARadixDeployment().
				WithAppName(anyAppName).
				WithEnvironment("dev").
				WithDeploymentName("dep-identity").
				WithJobComponents().
				WithComponents(
					operatorUtils.NewDeployComponentBuilder().
						WithName("component").
						WithHorizontalScaling(horizontalScaling)))
			require.NoError(t, err)

			endpoint := createGetComponentsEndpoint(anyAppName, "dep-identity")
			responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
			response := <-responseChannel

			assert.Equal(t, 200, response.Code)

			var components []deploymentModels.Component
			err = controllertest.GetResponseBody(response, &components)
			require.NoError(t, err)
			require.NotEmpty(t, components)

			component := getComponentByName("component", components)
			require.NotNil(t, component)
			require.NotNil(t, component.HorizontalScalingSummary)

			summary := component.HorizontalScalingSummary

			// Verify all triggers are present
			assert.Equal(t, len(scenario.triggers), len(summary.Triggers))

			// Verify identity for each trigger
			for _, trigger := range summary.Triggers {
				expectedIdentity, hasExpected := scenario.expectedIdentities[trigger.Name]
				require.True(t, hasExpected, "trigger name %s not in expected identities", trigger.Name)

				if expectedIdentity == nil {
					assert.Nil(t, trigger.Identity, "trigger %s should not have identity", trigger.Name)
				} else {
					require.NotNil(t, trigger.Identity, "trigger %s should have identity", trigger.Name)
					require.NotNil(t, trigger.Identity.Azure, "trigger %s should have Azure identity", trigger.Name)
					assert.Equal(t, expectedIdentity.Azure.ClientId, trigger.Identity.Azure.ClientId)
					assert.Equal(t, expectedIdentity.Azure.ServiceAccountName, trigger.Identity.Azure.ServiceAccountName)
					assert.Equal(t, expectedIdentity.Azure.Namespace, trigger.Identity.Azure.Namespace)
				}
			}
		})
	}
}

func Test_HorizontalScalingSummary_Properties(t *testing.T) {
	type testScenario struct {
		horizontalScaling *v1.RadixHorizontalScaling
		expectedValue     *deploymentModels.HorizontalScalingSummary
	}

	testScenarios := map[string]testScenario{
		"verify min and max replicas": {
			horizontalScaling: &v1.RadixHorizontalScaling{
				MinReplicas: pointers.Ptr[int32](2),
				MaxReplicas: 10,
				Triggers: []v1.RadixHorizontalScalingTrigger{
					{Name: "cpu", Cpu: &v1.RadixHorizontalScalingCPUTrigger{Value: 60}},
				},
			},
			expectedValue: &deploymentModels.HorizontalScalingSummary{
				MinReplicas: pointers.Ptr[int32](2),
				MaxReplicas: pointers.Ptr[int32](10),
				Triggers: []deploymentModels.HorizontalScalingSummaryTrigger{
					{Name: "cpu", Type: "cpu", TargetUtilization: "60"},
				},
			},
		},
		"verify cooldown and polling intervals": {
			horizontalScaling: &v1.RadixHorizontalScaling{
				MinReplicas:     pointers.Ptr[int32](1),
				MaxReplicas:     5,
				CooldownPeriod:  pointers.Ptr[int32](120),
				PollingInterval: pointers.Ptr[int32](45),
				Triggers: []v1.RadixHorizontalScalingTrigger{
					{Name: "memory", Memory: &v1.RadixHorizontalScalingMemoryTrigger{Value: 75}},
				},
			},
			expectedValue: &deploymentModels.HorizontalScalingSummary{
				MinReplicas:     pointers.Ptr[int32](1),
				MaxReplicas:     pointers.Ptr[int32](5),
				CooldownPeriod:  pointers.Ptr[int32](120),
				PollingInterval: pointers.Ptr[int32](45),
				Triggers: []deploymentModels.HorizontalScalingSummaryTrigger{
					{Name: "memory", Type: "memory", TargetUtilization: "75"},
				},
			},
		},
		"verify multiple triggers are set": {
			horizontalScaling: &v1.RadixHorizontalScaling{
				MinReplicas: pointers.Ptr[int32](1),
				MaxReplicas: 8,
				Triggers: []v1.RadixHorizontalScalingTrigger{
					{Name: "cpu", Cpu: &v1.RadixHorizontalScalingCPUTrigger{Value: 60}},
					{Name: "memory", Memory: &v1.RadixHorizontalScalingMemoryTrigger{Value: 75}},
					{Name: "cron", Cron: &v1.RadixHorizontalScalingCronTrigger{Start: "0 8 * * 1-5", End: "0 16 * * 1-5", Timezone: "Europe/Oslo", DesiredReplicas: 5}},
				},
			},
			expectedValue: &deploymentModels.HorizontalScalingSummary{
				MinReplicas: pointers.Ptr[int32](1),
				MaxReplicas: pointers.Ptr[int32](8),
				Triggers: []deploymentModels.HorizontalScalingSummaryTrigger{
					{Name: "cpu", Type: "cpu", TargetUtilization: "60"},
					{Name: "memory", Type: "memory", TargetUtilization: "75"},
					{Name: "cron", Type: "cron", TargetUtilization: "5"},
				},
			},
		},
		"verify deprecated resources are present without triggers": {
			horizontalScaling: &v1.RadixHorizontalScaling{
				MinReplicas: pointers.Ptr[int32](2),
				MaxReplicas: 9,
				RadixHorizontalScalingResources: &v1.RadixHorizontalScalingResources{
					Cpu:    &v1.RadixHorizontalScalingResource{AverageUtilization: pointers.Ptr[int32](55)},
					Memory: &v1.RadixHorizontalScalingResource{AverageUtilization: pointers.Ptr[int32](70)},
				},
			},
			expectedValue: &deploymentModels.HorizontalScalingSummary{
				MinReplicas: pointers.Ptr[int32](2),
				MaxReplicas: pointers.Ptr[int32](9),
				Triggers:    []deploymentModels.HorizontalScalingSummaryTrigger{},
			},
		},
		"verify all properties together": {
			horizontalScaling: &v1.RadixHorizontalScaling{
				MinReplicas:     pointers.Ptr[int32](3),
				MaxReplicas:     12,
				CooldownPeriod:  pointers.Ptr[int32](300),
				PollingInterval: pointers.Ptr[int32](30),
				Triggers: []v1.RadixHorizontalScalingTrigger{
					{Name: "cpu", Cpu: &v1.RadixHorizontalScalingCPUTrigger{Value: 80}},
					{Name: "memory", Memory: &v1.RadixHorizontalScalingMemoryTrigger{Value: 85}},
				},
			},
			expectedValue: &deploymentModels.HorizontalScalingSummary{
				MinReplicas:     pointers.Ptr[int32](3),
				MaxReplicas:     pointers.Ptr[int32](12),
				CooldownPeriod:  pointers.Ptr[int32](300),
				PollingInterval: pointers.Ptr[int32](30),
				Triggers: []deploymentModels.HorizontalScalingSummaryTrigger{
					{Name: "cpu", Type: "cpu", TargetUtilization: "80"},
					{Name: "memory", Type: "memory", TargetUtilization: "85"},
				},
			},
		},
	}

	for name, scenario := range testScenarios {
		t.Run(name, func(t *testing.T) {
			commonTestUtils, controllerTestUtils, client, radixclient, kedaClient, dynamicClient, secretProviderClient, certClient := setupTest(t)

			err := utils.ApplyDeploymentWithSync(client, radixclient, kedaClient, dynamicClient, commonTestUtils, secretProviderClient, certClient, operatorUtils.ARadixDeployment().
				WithAppName(anyAppName).
				WithEnvironment("dev").
				WithDeploymentName("dep-props").
				WithJobComponents().
				WithComponents(
					operatorUtils.NewDeployComponentBuilder().
						WithName("component").
						WithHorizontalScaling(scenario.horizontalScaling)))
			require.NoError(t, err)

			endpoint := createGetComponentsEndpoint(anyAppName, "dep-props")
			responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
			response := <-responseChannel

			assert.Equal(t, 200, response.Code)

			var components []deploymentModels.Component
			err = controllertest.GetResponseBody(response, &components)
			require.NoError(t, err)
			require.NotEmpty(t, components)

			component := getComponentByName("component", components)
			require.NotNil(t, component)
			require.NotNil(t, component.HorizontalScalingSummary)

			assert.Equal(t, scenario.expectedValue, component.HorizontalScalingSummary)
		})
	}
}

func getComponentByName(name string, components []deploymentModels.Component) *deploymentModels.Component {
	for _, comp := range components {
		if strings.EqualFold(name, comp.Name) {
			return &comp
		}
	}
	return nil
}

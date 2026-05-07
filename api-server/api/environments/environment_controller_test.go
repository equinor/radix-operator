package environments

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	certclientfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	radixhttp "github.com/equinor/radix-common/net/http"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	deploymentModels "github.com/equinor/radix-operator/api-server/api/deployments/models"
	environmentModels "github.com/equinor/radix-operator/api-server/api/environments/models"
	eventModels "github.com/equinor/radix-operator/api-server/api/events/models"
	"github.com/equinor/radix-operator/api-server/api/secrets"
	secretModels "github.com/equinor/radix-operator/api-server/api/secrets/models"
	"github.com/equinor/radix-operator/api-server/api/secrets/suffix"
	controllertest "github.com/equinor/radix-operator/api-server/api/test"
	"github.com/equinor/radix-operator/api-server/api/utils"
	authnmock "github.com/equinor/radix-operator/api-server/api/utils/token/mock"
	operatordefaults "github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedav2 "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	authorizationapiv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	testing2 "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	secretsstorevclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

const (
	clusterName      = "AnyClusterName"
	anyAppName       = "any-app"
	anyComponentName = "component1"
	anyJobName       = "job1"
	anyJobName2      = "job2"
	anyBatchName     = "batch1"
	anyDeployment    = "deployment1"
	anyEnvironment   = "dev1"
	anySecretName    = "TEST_SECRET"
	subscriptionId   = "12347718-c8f8-4995-bfbb-02655ff1f89c"
	nodeType1        = "some-node-type1"
)

func setupTest(t *testing.T, envHandlerOpts []EnvironmentHandlerOptions) (*commontest.Utils, *controllertest.Utils, *controllertest.Utils, *kubefake.Clientset, radixclient.Interface, kedav2.Interface, client.Client, secretsstorevclient.Interface, *certclientfake.Clientset) {
	// Setup
	kubeclient := kubefake.NewClientset()
	radixClient := radixfake.NewSimpleClientset() //nolint:staticcheck
	kedaClient := kedafake.NewSimpleClientset()
	dynamicClient := commontest.CreateClient()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	certClient := certclientfake.NewSimpleClientset()

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeclient, radixClient, kedaClient, secretproviderclient)
	err := commonTestUtils.CreateClusterPrerequisites(clusterName, subscriptionId)
	require.NoError(t, err)

	mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
	mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(true), nil)
	// secretControllerTestUtils is used for issuing HTTP request and processing responses
	secretControllerTestUtils := controllertest.NewTestUtils(kubeclient, radixClient, kedaClient, secretproviderclient, certClient, nil, mockValidator, secrets.NewSecretController(nil))
	// controllerTestUtils is used for issuing HTTP request and processing responses
	environmentControllerTestUtils := controllertest.NewTestUtils(kubeclient, radixClient, kedaClient, secretproviderclient, certClient, nil, mockValidator, NewEnvironmentController(NewEnvironmentHandlerFactory(envHandlerOpts...)))

	return &commonTestUtils, &environmentControllerTestUtils, &secretControllerTestUtils, kubeclient, radixClient, kedaClient, dynamicClient, secretproviderclient, certClient
}

func TestGetEnvironmentDeployments_SortedWithFromTo(t *testing.T) {
	deploymentOneImage := "abcdef"
	deploymentTwoImage := "ghijkl"
	deploymentThreeImage := "mnopqr"
	layout := "2006-01-02T15:04:05.000Z"
	deploymentOneCreated, _ := time.Parse(layout, "2018-11-12T11:45:26.371Z")
	deploymentTwoCreated, _ := time.Parse(layout, "2018-11-12T12:30:14.000Z")
	deploymentThreeCreated, _ := time.Parse(layout, "2018-11-20T09:00:00.000Z")

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	setupGetDeploymentsTest(t, commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, anyEnvironment)

	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/deployments", anyAppName, anyEnvironment))
	response := <-responseChannel

	deployments := make([]*deploymentModels.DeploymentSummary, 0)
	err := controllertest.GetResponseBody(response, &deployments)
	require.NoError(t, err)
	assert.Equal(t, 3, len(deployments))

	assert.Equal(t, deploymentThreeImage, deployments[0].Name)
	assert.Equal(t, deploymentThreeCreated, deployments[0].ActiveFrom)
	assert.Nil(t, deployments[0].ActiveTo)

	assert.Equal(t, deploymentTwoImage, deployments[1].Name)
	assert.Equal(t, deploymentTwoCreated, deployments[1].ActiveFrom)
	assert.Equal(t, &deploymentThreeCreated, deployments[1].ActiveTo)

	assert.Equal(t, deploymentOneImage, deployments[2].Name)
	assert.Equal(t, deploymentOneCreated, deployments[2].ActiveFrom)
	assert.Equal(t, &deploymentTwoCreated, deployments[2].ActiveTo)
}

func TestGetEnvironmentDeployments_Latest(t *testing.T) {
	deploymentOneImage := "abcdef"
	deploymentTwoImage := "ghijkl"
	deploymentThreeImage := "mnopqr"
	layout := "2006-01-02T15:04:05.000Z"
	deploymentOneCreated, _ := time.Parse(layout, "2018-11-12T11:45:26.371Z")
	deploymentTwoCreated, _ := time.Parse(layout, "2018-11-12T12:30:14.000Z")
	deploymentThreeCreated, _ := time.Parse(layout, "2018-11-20T09:00:00.000Z")

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	setupGetDeploymentsTest(t, commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, anyEnvironment)

	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/deployments?latest=true", anyAppName, anyEnvironment))
	response := <-responseChannel

	deployments := make([]*deploymentModels.DeploymentSummary, 0)
	err := controllertest.GetResponseBody(response, &deployments)
	require.NoError(t, err)
	assert.Equal(t, 1, len(deployments))

	assert.Equal(t, deploymentThreeImage, deployments[0].Name)
	assert.Equal(t, deploymentThreeCreated, deployments[0].ActiveFrom)
	assert.Nil(t, deployments[0].ActiveTo)
}

func TestGetEnvironmentSummary_ApplicationWithNoDeployments_EnvironmentPending(t *testing.T) {
	envName1, envName2 := "dev", "master"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithRadixRegistration(operatorutils.ARadixRegistration()).
		WithAppName(anyAppName).
		WithEnvironment(envName1, envName2))
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	err = controllertest.GetResponseBody(response, &environments)
	require.NoError(t, err)
	assert.Equal(t, 1, len(environments))

	assert.Equal(t, envName1, environments[0].Name)
	assert.Equal(t, environmentModels.Pending.String(), environments[0].Status)
	assert.Equal(t, envName2, environments[0].BranchMapping)
	assert.Nil(t, environments[0].ActiveDeployment)
}

func TestGetEnvironmentSummary_ApplicationWithDeployment_EnvironmentConsistent(t *testing.T) {
	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			ARadixDeployment().
			WithRadixApplication(operatorutils.
				NewRadixApplicationBuilder().
				WithRadixRegistration(operatorutils.ARadixRegistration()).
				WithAppName(anyAppName).
				WithEnvironment(anyEnvironment, "master")).
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment))
	require.NoError(t, err)

	re, err := radixClient.RadixV1().RadixEnvironments().Get(context.Background(), operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment), metav1.GetOptions{})
	require.NoError(t, err)
	re.Status.Reconciled = metav1.Now()
	_, err = radixClient.RadixV1().RadixEnvironments().UpdateStatus(context.Background(), re, metav1.UpdateOptions{})
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	err = controllertest.GetResponseBody(response, &environments)
	require.NoError(t, err)

	assert.Equal(t, environmentModels.Consistent.String(), environments[0].Status)
	assert.NotNil(t, environments[0].ActiveDeployment)
}

func TestGetEnvironmentSummary_RemoveEnvironmentFromConfig_OrphanedEnvironment(t *testing.T) {
	envName1, envName2 := "dev", "master"
	anyOrphanedEnvironment := "feature-1"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(envName1, envName2).
		WithEnvironment(anyOrphanedEnvironment, "feature"))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(envName1).
			WithImageTag("someimageindev"))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyOrphanedEnvironment).
			WithImageTag("someimageinfeature"))
	require.NoError(t, err)

	// Remove feature environment from application config
	_, err = commonTestUtils.ApplyApplicationUpdate(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(envName1, envName2))
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	err = controllertest.GetResponseBody(response, &environments)
	require.NoError(t, err)

	for _, environment := range environments {
		if strings.EqualFold(environment.Name, anyOrphanedEnvironment) {
			assert.Equal(t, environmentModels.Orphan.String(), environment.Status)
			assert.NotNil(t, environment.ActiveDeployment)
		}
	}
}

func TestGetEnvironmentSummary_OrphanedEnvironmentWithDash_OrphanedEnvironmentIsListedOk(t *testing.T) {
	anyOrphanedEnvironment := "feature-1"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	rr, err := commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment, "master"))
	require.NoError(t, err)
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyEnvironment(operatorutils.
		NewEnvironmentBuilder().
		WithAppLabel().
		WithAppName(anyAppName).
		WithEnvironmentName(anyOrphanedEnvironment).
		WithRegistrationOwner(rr).
		WithOrphaned(true))
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	err = controllertest.GetResponseBody(response, &environments)
	require.NoError(t, err)
	require.NoError(t, err)

	environmentListed := false
	for _, environment := range environments {
		if strings.EqualFold(environment.Name, anyOrphanedEnvironment) {
			assert.Equal(t, environmentModels.Orphan.String(), environment.Status)
			environmentListed = true
		}
	}

	assert.True(t, environmentListed)
}

func TestDeleteEnvironment_OneOrphanedEnvironment_OnlyOrphanedCanBeDeleted(t *testing.T) {
	anyNonOrphanedEnvironment := "dev-1"
	anyOrphanedEnvironment := "feature-1"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(anyNonOrphanedEnvironment, "master").
		WithRadixRegistration(operatorutils.
			NewRegistrationBuilder().
			WithName(anyAppName)))
	require.NoError(t, err)
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyEnvironment(operatorutils.
		NewEnvironmentBuilder().
		WithAppLabel().
		WithAppName(anyAppName).
		WithEnvironmentName(anyOrphanedEnvironment).
		WithOrphaned(true))
	require.NoError(t, err)

	// Test
	// Start with two environments
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	err = controllertest.GetResponseBody(response, &environments)
	require.NoError(t, err)
	require.NoError(t, err)
	assert.Equal(t, 2, len(environments))

	// Orphaned environment can be deleted
	responseChannel = environmentControllerTestUtils.ExecuteRequest("DELETE", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyOrphanedEnvironment))
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	// Non-orphaned cannot
	responseChannel = environmentControllerTestUtils.ExecuteRequest("DELETE", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyNonOrphanedEnvironment))
	response = <-responseChannel
	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := environmentModels.CannotDeleteNonOrphanedEnvironment(anyAppName, anyNonOrphanedEnvironment)
	assert.Equal(t, (expectedError.(*radixhttp.Error)).Message, errorResponse.Message)

	// Only one remaining environment after delete
	responseChannel = environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response = <-responseChannel
	environments = make([]*environmentModels.EnvironmentSummary, 0)
	err = controllertest.GetResponseBody(response, &environments)
	require.NoError(t, err)
	require.NoError(t, err)
	assert.Equal(t, 1, len(environments))
}

func TestGetEnvironment_NoExistingEnvironment_ReturnsAnError(t *testing.T) {
	anyNonExistingEnvironment := "non-existing-environment"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyApplication(operatorutils.
		ARadixApplication().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment, "master"))
	require.NoError(t, err)
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyNonExistingEnvironment))
	response := <-responseChannel
	assert.Equal(t, http.StatusNotFound, response.Code)

	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := environmentModels.NonExistingEnvironment(nil, anyAppName, anyNonExistingEnvironment)
	assert.Equal(t, (expectedError.(*radixhttp.Error)).Message, errorResponse.Message)
}

func TestGetEnvironment_ExistingEnvironmentInConfig_ReturnsAPendingEnvironment(t *testing.T) {
	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyApplication(operatorutils.
		ARadixApplication().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment, "master"))
	require.NoError(t, err)
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyEnvironment))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	environment := environmentModels.Environment{}
	err = controllertest.GetResponseBody(response, &environment)
	require.NoError(t, err)
	require.NoError(t, err)
	assert.Equal(t, anyEnvironment, environment.Name)
	assert.Equal(t, environmentModels.Pending.String(), environment.Status)
}

func setupGetDeploymentsTest(t *testing.T, commonTestUtils *commontest.Utils, appName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage string, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated time.Time, environment string) {
	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			ARadixDeployment().
			WithDeploymentName(deploymentOneImage).
			WithAppName(appName).
			WithEnvironment(environment).
			WithImageTag(deploymentOneImage).
			WithCreated(deploymentOneCreated).
			WithCondition(v1.DeploymentInactive).
			WithActiveFrom(deploymentOneCreated).
			WithActiveTo(deploymentTwoCreated))
	require.NoError(t, err)
	require.NoError(t, err)

	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			ARadixDeployment().
			WithDeploymentName(deploymentTwoImage).
			WithAppName(appName).
			WithEnvironment(environment).
			WithImageTag(deploymentTwoImage).
			WithCreated(deploymentTwoCreated).
			WithCondition(v1.DeploymentInactive).
			WithActiveFrom(deploymentTwoCreated).
			WithActiveTo(deploymentThreeCreated))
	require.NoError(t, err)
	require.NoError(t, err)

	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			ARadixDeployment().
			WithDeploymentName(deploymentThreeImage).
			WithAppName(appName).
			WithEnvironment(environment).
			WithImageTag(deploymentThreeImage).
			WithCreated(deploymentThreeCreated).
			WithCondition(v1.DeploymentActive).
			WithActiveFrom(deploymentThreeCreated))
	require.NoError(t, err)
	require.NoError(t, err)
}

func TestComponentStatusActions(t *testing.T) {

	scenarios := []ComponentCreatorStruct{
		{scenarioName: "Stop unstopped component", number: 1, name: "comp1", action: "stop", status: deploymentModels.ConsistentComponent, expectedStatus: http.StatusOK},
		{scenarioName: "Stop stopped component", number: 1, name: "comp2", action: "stop", status: deploymentModels.StoppedComponent, expectedStatus: http.StatusBadRequest},
		{scenarioName: "Start stopped component", number: 1, name: "comp3", action: "start", status: deploymentModels.StoppedComponent, expectedStatus: http.StatusOK},
		{scenarioName: "Start started component", number: 1, name: "comp4", action: "start", status: deploymentModels.ConsistentComponent, expectedStatus: http.StatusOK},
		{scenarioName: "Restart started component", number: 1, name: "comp5", action: "restart", status: deploymentModels.ConsistentComponent, expectedStatus: http.StatusOK},
		{scenarioName: "Restart stopped component", number: 1, name: "comp6", action: "restart", status: deploymentModels.StoppedComponent, expectedStatus: http.StatusBadRequest},
		{scenarioName: "Reset manually scaled component", number: 1, name: "comp7", action: "reset-scale", status: deploymentModels.StoppedComponent, expectedStatus: http.StatusOK},
	}

	// Mock Status
	statuser := func(component v1.RadixCommonDeployComponent, kd *appsv1.Deployment, rd *v1.RadixDeployment) deploymentModels.ComponentStatus {
		for _, scenario := range scenarios {
			if scenario.name == component.GetName() {
				return scenario.status
			}
		}

		panic("unknown component! ")
	}

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, []EnvironmentHandlerOptions{WithComponentStatuserFunc(statuser)})
	_, _ = createRadixDeploymentWithReplicas(commonTestUtils, anyAppName, anyEnvironment, scenarios)

	for _, scenario := range scenarios {
		t.Run(scenario.scenarioName, func(t *testing.T) {
			responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/%s", anyAppName, anyEnvironment, scenario.name, scenario.action))
			response := <-responseChannel
			assert.Equal(t, scenario.expectedStatus, response.Code)
		})
	}
}

func TestRestartEnvrionment_ApplicationWithDeployment_EnvironmentConsistent(t *testing.T) {
	zeroReplicas := 0

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixclient, _, _, _, _ := setupTest(t, nil)

	// Test
	t.Run("Restart Environment", func(t *testing.T) {
		envName := "fullyRunningEnv"
		rd, _ := createRadixDeploymentWithReplicas(commonTestUtils, anyAppName, envName, []ComponentCreatorStruct{
			{name: "runningComponent1", number: 1},
			{name: "runningComponent2", number: 2},
		})
		for _, comp := range rd.Spec.Components {
			assert.True(t, *comp.Replicas != zeroReplicas)
		}

		responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/restart", anyAppName, envName))
		response := <-responseChannel
		assert.Equal(t, http.StatusOK, response.Code)

		updatedRd, _ := radixclient.RadixV1().RadixDeployments(rd.GetNamespace()).Get(context.Background(), rd.GetName(), metav1.GetOptions{})
		for _, comp := range updatedRd.Spec.Components {
			assert.True(t, *comp.Replicas > zeroReplicas)
		}
	})

	t.Run("Restart Environment with stopped component", func(t *testing.T) {
		envName := "partiallyRunningEnv"
		rd, _ := createRadixDeploymentWithReplicas(commonTestUtils, anyAppName, envName, []ComponentCreatorStruct{
			{name: "stoppedComponent", number: 0},
			{name: "runningComponent", number: 7},
		})
		replicaCount := 0
		for _, comp := range rd.Spec.Components {
			replicaCount += *comp.Replicas
		}
		assert.True(t, replicaCount > zeroReplicas)

		responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/restart", anyAppName, envName))
		response := <-responseChannel
		assert.Equal(t, http.StatusOK, response.Code)

		errorResponse, _ := controllertest.GetErrorResponse(response)
		assert.Nil(t, errorResponse)

		updatedRd, _ := radixclient.RadixV1().RadixDeployments(rd.GetNamespace()).Get(context.Background(), rd.GetName(), metav1.GetOptions{})
		updatedReplicaCount := 0
		for _, comp := range updatedRd.Spec.Components {
			updatedReplicaCount += *comp.Replicas
		}
		assert.True(t, updatedReplicaCount == replicaCount)
	})
}

func TestCreateEnvironment(t *testing.T) {
	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyApplication(operatorutils.
		ARadixApplication().
		WithAppName(anyAppName))
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyEnvironment))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
}

func Test_GetEnvironmentEvents_Controller(t *testing.T) {
	envName := "dev"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, kubeClient, _, _, _, _, _ := setupTest(t, nil)
	createEvent := func(namespace, eventName string) {
		_, err := kubeClient.CoreV1().Events(namespace).CreateWithEventNamespaceWithContext(context.Background(), &corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Name: eventName,
			},
		})
		require.NoError(t, err)
	}
	createEvent(operatorutils.GetEnvironmentNamespace(anyAppName, envName), "ev1")
	createEvent(operatorutils.GetEnvironmentNamespace(anyAppName, envName), "ev2")
	_, err := commonTestUtils.ApplyApplication(operatorutils.
		ARadixApplication().
		WithAppName(anyAppName).
		WithEnvironment(envName, "master"))
	require.NoError(t, err)

	t.Run("Get events for dev environment", func(t *testing.T) {
		responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/events", anyAppName, envName))
		response := <-responseChannel
		assert.Equal(t, http.StatusOK, response.Code)
		events := make([]eventModels.Event, 0)
		err = controllertest.GetResponseBody(response, &events)
		require.NoError(t, err)
		assert.Len(t, events, 2)
	})

	t.Run("Get events for non-existing environment", func(t *testing.T) {
		responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/events", anyAppName, "prod"))
		response := <-responseChannel
		assert.Equal(t, http.StatusNotFound, response.Code)
		errResponse, _ := controllertest.GetErrorResponse(response)
		assert.Equal(
			t,
			environmentModels.NonExistingEnvironment(nil, anyAppName, "prod").Error(),
			errResponse.Message,
		)
	})

	t.Run("Get events for non-existing application", func(t *testing.T) {
		responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/events", "noapp", envName))
		response := <-responseChannel
		assert.Equal(t, http.StatusNotFound, response.Code)
		errResponse, _ := controllertest.GetErrorResponse(response)
		assert.Equal(
			t,
			controllertest.AppNotFoundErrorMsg("noapp"),
			errResponse.Message,
		)
	})
}

func TestUpdateSecret_AccountSecretForComponentVolumeMount_UpdatedOk(t *testing.T) {
	// Setup
	commonTestUtils, environmentControllerTestUtils, controllerTestUtils, client, radixclient, kedaClient, promclient, secretProviderClient, certClient := setupTest(t, nil)
	err := utils.ApplyDeploymentWithSync(client, radixclient, kedaClient, promclient, commonTestUtils, secretProviderClient, certClient, operatorutils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithRadixApplication(operatorutils.ARadixApplication().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment, "master")).
		WithComponents(
			operatorutils.NewDeployComponentBuilder().
				WithName(anyComponentName).
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
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyEnvironment))
	response := <-responseChannel

	environment := environmentModels.Environment{}
	err = controllertest.GetResponseBody(response, &environment)
	require.NoError(t, err)
	assert.Equal(t, 2, len(environment.Secrets))
	assert.True(t, contains(environment.Secrets, fmt.Sprintf("%v-somevolumename-csiazurecreds-accountkey", anyComponentName)))
	assert.True(t, contains(environment.Secrets, fmt.Sprintf("%v-somevolumename-csiazurecreds-accountname", anyComponentName)))

	parameters := secretModels.SecretParameters{SecretValue: "anyValue"}
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/secrets/%s", anyAppName, anyEnvironment, anyComponentName, environment.Secrets[0].Name), parameters)
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	responseChannel = environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyEnvironment))
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	environment = environmentModels.Environment{}
	err = controllertest.GetResponseBody(response, &environment)
	require.NoError(t, err)

	accountKeySecret, ok := slice.FindFirst(environment.Secrets, func(secret secretModels.Secret) bool {
		return secret.ID == secretModels.SecretIdAccountKey
	})
	assert.True(t, ok)
	assert.WithinDuration(t, time.Now(), pointers.Val(accountKeySecret.Updated), 1*time.Second)
	accountNameSecret, ok := slice.FindFirst(environment.Secrets, func(secret secretModels.Secret) bool {
		return secret.ID == secretModels.SecretIdAccountName
	})
	assert.True(t, ok)
	assert.Nil(t, accountNameSecret.Updated)
}

func TestUpdateSecret_AccountSecretForJobVolumeMount_UpdatedOk(t *testing.T) {
	// Setup
	commonTestUtils, environmentControllerTestUtils, controllerTestUtils, client, radixclient, kedaClient, promclient, secretProviderClient, certClient := setupTest(t, nil)
	err := utils.ApplyDeploymentWithSync(client, radixclient, kedaClient, promclient, commonTestUtils, secretProviderClient, certClient, operatorutils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithRadixApplication(operatorutils.ARadixApplication().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment, "master")).
		WithJobComponents(
			operatorutils.NewDeployJobComponentBuilder().
				WithName(anyJobName).
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
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyEnvironment))
	response := <-responseChannel
	require.Equal(t, http.StatusOK, response.Code)

	environment := environmentModels.Environment{}
	err = controllertest.GetResponseBody(response, &environment)
	require.NoError(t, err)
	assert.Equal(t, 2, len(environment.Secrets))
	assert.True(t, contains(environment.Secrets, fmt.Sprintf("%v-somevolumename-csiazurecreds-accountkey", anyJobName)))
	assert.True(t, contains(environment.Secrets, fmt.Sprintf("%v-somevolumename-csiazurecreds-accountname", anyJobName)))

	parameters := secretModels.SecretParameters{SecretValue: "anyValue"}
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/secrets/%s", anyAppName, anyEnvironment, anyJobName, environment.Secrets[0].Name), parameters)
	response = <-responseChannel
	require.Equal(t, http.StatusOK, response.Code)

	responseChannel = environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyEnvironment))
	response = <-responseChannel
	require.Equal(t, http.StatusOK, response.Code)
	environment = environmentModels.Environment{}
	err = controllertest.GetResponseBody(response, &environment)
	require.NoError(t, err)
	require.NotNil(t, environment.Secrets)

	secret, ok := slice.FindFirst(environment.Secrets, func(secret secretModels.Secret) bool {
		return secret.ID == secretModels.SecretIdAccountKey
	})
	assert.True(t, ok)
	assert.WithinDuration(t, time.Now(), pointers.Val(secret.Updated), 1*time.Second)

	secret, ok = slice.FindFirst(environment.Secrets, func(secret secretModels.Secret) bool {
		return secret.ID == secretModels.SecretIdAccountName
	})
	assert.True(t, ok)
	assert.Nil(t, secret.Updated)
}

func TestUpdateSecret_OAuth2_UpdatedOk(t *testing.T) {
	// Setup
	envNs := operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment)
	commonTestUtils, environmentControllerTestUtils, controllerTestUtils, client, radixclient, kedaClient, promclient, secretProviderClient, certClient := setupTest(t, nil)
	err := utils.ApplyDeploymentWithSync(client, radixclient, kedaClient, promclient, commonTestUtils, secretProviderClient, certClient, operatorutils.NewDeploymentBuilder().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithRadixApplication(operatorutils.ARadixApplication().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment, "master")).
		WithComponents(
			operatorutils.NewDeployComponentBuilder().WithName(anyComponentName).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{SessionStoreType: v1.SessionStoreRedis}}),
		))
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyEnvironment))
	response := <-responseChannel

	environment := environmentModels.Environment{}
	err = controllertest.GetResponseBody(response, &environment)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{anyComponentName + suffix.OAuth2ClientSecret, anyComponentName + suffix.OAuth2CookieSecret, anyComponentName + suffix.OAuth2RedisPassword}, environment.ActiveDeployment.Components[0].Secrets)

	// Update secret when k8s secret object is missing should return 404
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters(
		"PUT",
		fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/secrets/%s", anyAppName, anyEnvironment, anyComponentName, anyComponentName+suffix.OAuth2ClientSecret),
		secretModels.SecretParameters{SecretValue: "clientsecret"},
	)
	response = <-responseChannel
	assert.Equal(t, http.StatusNotFound, response.Code)

	// Update client secret when k8s secret exists should set Data
	secretName := operatorutils.GetAuxiliaryComponentSecretName(anyComponentName, v1.OAuthProxyAuxiliaryComponentSuffix)
	_, err = client.CoreV1().Secrets(envNs).Create(context.Background(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName}}, metav1.CreateOptions{})
	require.NoError(t, err)

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters(
		"PUT",
		fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/secrets/%s", anyAppName, anyEnvironment, anyComponentName, anyComponentName+suffix.OAuth2ClientSecret),
		secretModels.SecretParameters{SecretValue: "clientsecret"},
	)
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	actualSecret, _ := client.CoreV1().Secrets(envNs).Get(context.Background(), secretName, metav1.GetOptions{})
	assert.Equal(t, actualSecret.Data, map[string][]byte{operatordefaults.OAuthClientSecretKeyName: []byte("clientsecret")})

	// Update client secret when k8s secret exists should set Data
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters(
		"PUT",
		fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/secrets/%s", anyAppName, anyEnvironment, anyComponentName, anyComponentName+suffix.OAuth2CookieSecret),
		secretModels.SecretParameters{SecretValue: "cookiesecret"},
	)
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	actualSecret, _ = client.CoreV1().Secrets(envNs).Get(context.Background(), secretName, metav1.GetOptions{})
	assert.Equal(t, actualSecret.Data, map[string][]byte{operatordefaults.OAuthClientSecretKeyName: []byte("clientsecret"), operatordefaults.OAuthCookieSecretKeyName: []byte("cookiesecret")})

	// Update client secret when k8s secret exists should set Data
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters(
		"PUT",
		fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/secrets/%s", anyAppName, anyEnvironment, anyComponentName, anyComponentName+suffix.OAuth2RedisPassword),
		secretModels.SecretParameters{SecretValue: "redispassword"},
	)
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	actualSecret, _ = client.CoreV1().Secrets(envNs).Get(context.Background(), secretName, metav1.GetOptions{})
	assert.Equal(t, actualSecret.Data, map[string][]byte{operatordefaults.OAuthClientSecretKeyName: []byte("clientsecret"), operatordefaults.OAuthCookieSecretKeyName: []byte("cookiesecret"), operatordefaults.OAuthRedisPasswordKeyName: []byte("redispassword")})
}

func TestGetSecretDeployments_SortedWithFromTo(t *testing.T) {
	deploymentOneImage := "abcdef"
	deploymentTwoImage := "ghijkl"
	deploymentThreeImage := "mnopqr"
	layout := "2006-01-02T15:04:05.000Z"
	deploymentOneCreated, _ := time.Parse(layout, "2018-11-12T11:45:26.371Z")
	deploymentTwoCreated, _ := time.Parse(layout, "2018-11-12T12:30:14.000Z")
	deploymentThreeCreated, _ := time.Parse(layout, "2018-11-20T09:00:00.000Z")

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	setupGetDeploymentsTest(t, commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, anyEnvironment)

	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/deployments", anyAppName, anyEnvironment))
	response := <-responseChannel

	deployments := make([]*deploymentModels.DeploymentSummary, 0)
	err := controllertest.GetResponseBody(response, &deployments)
	require.NoError(t, err)
	assert.Equal(t, 3, len(deployments))

	assert.Equal(t, deploymentThreeImage, deployments[0].Name)
	assert.Equal(t, deploymentThreeCreated, deployments[0].ActiveFrom)
	assert.Nil(t, deployments[0].ActiveTo)

	assert.Equal(t, deploymentTwoImage, deployments[1].Name)
	assert.Equal(t, deploymentTwoCreated, deployments[1].ActiveFrom)
	assert.Equal(t, &deploymentThreeCreated, deployments[1].ActiveTo)

	assert.Equal(t, deploymentOneImage, deployments[2].Name)
	assert.Equal(t, deploymentOneCreated, deployments[2].ActiveFrom)
	assert.Equal(t, &deploymentTwoCreated, deployments[2].ActiveTo)
}

func TestGetSecretDeployments_Latest(t *testing.T) {
	deploymentOneImage := "abcdef"
	deploymentTwoImage := "ghijkl"
	deploymentThreeImage := "mnopqr"
	layout := "2006-01-02T15:04:05.000Z"
	deploymentOneCreated, _ := time.Parse(layout, "2018-11-12T11:45:26.371Z")
	deploymentTwoCreated, _ := time.Parse(layout, "2018-11-12T12:30:14.000Z")
	deploymentThreeCreated, _ := time.Parse(layout, "2018-11-20T09:00:00.000Z")

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	setupGetDeploymentsTest(t, commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, anyEnvironment)

	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/deployments?latest=true", anyAppName, anyEnvironment))
	response := <-responseChannel

	deployments := make([]*deploymentModels.DeploymentSummary, 0)
	err := controllertest.GetResponseBody(response, &deployments)
	require.NoError(t, err)
	assert.Equal(t, 1, len(deployments))

	assert.Equal(t, deploymentThreeImage, deployments[0].Name)
	assert.Equal(t, deploymentThreeCreated, deployments[0].ActiveFrom)
	assert.Nil(t, deployments[0].ActiveTo)
}

func TestGetEnvironmentSummary_ApplicationWithNoDeployments_SecretPending(t *testing.T) {
	envName1, envName2 := "dev", "master"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithRadixRegistration(operatorutils.ARadixRegistration()).
		WithAppName(anyAppName).
		WithEnvironment(envName1, envName2))
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	err = controllertest.GetResponseBody(response, &environments)
	require.NoError(t, err)

	assert.Equal(t, 1, len(environments))
	assert.Equal(t, envName1, environments[0].Name)
	assert.Equal(t, environmentModels.Pending.String(), environments[0].Status)
	assert.Equal(t, envName2, environments[0].BranchMapping)
	assert.Nil(t, environments[0].ActiveDeployment)
}

func TestGetEnvironmentSummary_RemoveSecretFromConfig_OrphanedSecret(t *testing.T) {
	envName1, envName2 := "dev", "master"
	orphanedEnvironment := "feature-1"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(envName1, envName2).
		WithEnvironment(orphanedEnvironment, "feature"))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(envName1).
			WithImageTag("someimageindev"))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(orphanedEnvironment).
			WithImageTag("someimageinfeature"))
	require.NoError(t, err)

	// Remove feature environment from application config
	_, err = commonTestUtils.ApplyApplicationUpdate(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(envName1, envName2))
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	err = controllertest.GetResponseBody(response, &environments)
	require.NoError(t, err)

	for _, environment := range environments {
		if strings.EqualFold(environment.Name, orphanedEnvironment) {
			assert.Equal(t, environmentModels.Orphan.String(), environment.Status)
			assert.NotNil(t, environment.ActiveDeployment)
		}
	}
}

func TestGetEnvironmentSummary_OrphanedSecretWithDash_OrphanedSecretIsListedOk(t *testing.T) {
	orphanedEnvironment := "feature-1"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	rr, err := commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment, "master"))
	require.NoError(t, err)
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyEnvironment(operatorutils.
		NewEnvironmentBuilder().
		WithAppLabel().
		WithAppName(anyAppName).
		WithEnvironmentName(orphanedEnvironment).
		WithRegistrationOwner(rr).
		WithOrphaned(true))
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	err = controllertest.GetResponseBody(response, &environments)
	require.NoError(t, err)

	environmentListed := false
	for _, environment := range environments {
		if strings.EqualFold(environment.Name, orphanedEnvironment) {
			assert.Equal(t, environmentModels.Orphan.String(), environment.Status)
			environmentListed = true
		}
	}

	assert.True(t, environmentListed)
}

func TestGetSecret_ExistingSecretInConfig_ReturnsAPendingSecret(t *testing.T) {
	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyApplication(operatorutils.
		ARadixApplication().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment, "master"))
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyEnvironment))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	environment := environmentModels.Environment{}
	err = controllertest.GetResponseBody(response, &environment)
	require.NoError(t, err)
	assert.Nil(t, err)
	assert.Equal(t, anyEnvironment, environment.Name)
	assert.Equal(t, environmentModels.Pending.String(), environment.Status)
}

func TestCreateSecret(t *testing.T) {
	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyApplication(operatorutils.
		ARadixApplication().
		WithAppName(anyAppName))
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyEnvironment))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestRestartAuxiliaryResource(t *testing.T) {
	auxType := "oauth"
	called := 0

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, kubeClient, _, _, _, _, _ := setupTest(t, nil)
	kubeClient.PrependReactor("create", "*", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
		createAction, ok := action.DeepCopy().(testing2.CreateAction)
		if !ok {
			return false, nil, nil
		}

		review, ok := createAction.GetObject().(*authorizationapiv1.SelfSubjectAccessReview)
		if !ok {
			return false, nil, nil
		}

		called++

		if review.Spec.ResourceAttributes.Name != anyAppName {
			return true, review, nil
		}

		assert.Equal(t, review.Spec.ResourceAttributes.Name, anyAppName)
		assert.Equal(t, review.Spec.ResourceAttributes.Resource, v1.ResourceRadixRegistrations)
		assert.Equal(t, review.Spec.ResourceAttributes.Verb, "patch")

		review.Status.Allowed = true
		return true, review, nil
	})
	_, err := commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName("forbidden"))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment, "master"))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithComponent(operatorutils.
				NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}})).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	envNs := operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment)
	_, err = kubeClient.AppsV1().Deployments(envNs).Create(context.Background(), &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "comp1-aux-resource",
			Labels: map[string]string{kube.RadixAppLabel: anyAppName, kube.RadixAuxiliaryComponentLabel: anyComponentName, kube.RadixAuxiliaryComponentTypeLabel: auxType},
		},
		Spec:   appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{}}},
		Status: appsv1.DeploymentStatus{Replicas: 1},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/aux/%s/restart", anyAppName, anyEnvironment, anyComponentName, auxType))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, 1, called)

	kubeDeploy, _ := kubeClient.AppsV1().Deployments(envNs).Get(context.Background(), "comp1-aux-resource", metav1.GetOptions{})
	assert.NotEmpty(t, kubeDeploy.Spec.Template.Annotations[restartedAtAnnotation])

	// Test Forbidden for other app names

	responseChannel = environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/aux/%s/restart", "forbidden", anyEnvironment, anyComponentName, auxType))
	response = <-responseChannel
	assert.Equal(t, http.StatusForbidden, response.Code)
	assert.Equal(t, 2, called)
}

type ComponentCreatorStruct struct {
	name           string
	number         int
	action         string
	status         deploymentModels.ComponentStatus
	expectedStatus int
	scenarioName   string
}

func createRadixDeploymentWithReplicas(tu *commontest.Utils, appName, envName string, components []ComponentCreatorStruct) (*v1.RadixDeployment, error) {
	var comps []operatorutils.DeployComponentBuilder
	for _, component := range components {
		comps = append(
			comps,
			operatorutils.
				NewDeployComponentBuilder().
				WithName(component.name).
				WithReplicas(pointers.Ptr(component.number)).
				WithReplicasOverride(pointers.Ptr(component.number)),
		)
	}

	rd, err := tu.ApplyDeployment(
		context.Background(),
		operatorutils.
			ARadixDeployment().
			WithComponents(comps...).
			WithAppName(appName).
			WithAnnotations(make(map[string]string)).
			WithEnvironment(envName),
	)

	return rd, err
}

func contains(secrets []secretModels.Secret, name string) bool {
	for _, secret := range secrets {
		if secret.Name == name {
			return true
		}
	}
	return false
}

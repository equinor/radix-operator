package deployments

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	certfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	authnmock "github.com/equinor/radix-operator/api-server/api/utils/token/mock"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	kedav2 "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"k8s.io/client-go/kubernetes"

	radixhttp "github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
	deploymentModels "github.com/equinor/radix-operator/api-server/api/deployments/models"
	controllertest "github.com/equinor/radix-operator/api-server/api/test"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	secretsstorevclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

const (
	anyAppName    = "any-app"
	anyPodName    = "any-pod"
	anyDeployName = "any-deploy"
)

func createGetLogEndpoint(appName, podName string) string {
	return fmt.Sprintf("/api/v1/applications/%s/deployments/any/components/any/replicas/%s/logs", appName, podName)
}

func setupTest(t *testing.T) (*commontest.Utils, *controllertest.Utils, kubernetes.Interface, radixclient.Interface, kedav2.Interface, client.Client, secretsstorevclient.Interface, *certfake.Clientset) {
	const (
		clusterName    = "AnyClusterName"
		subscriptionId = "bd9f9eaa-2703-47c6-b5e0-faf4e058df73"
	)
	kubeClient := kubefake.NewClientset()         //nolint:staticcheck
	radixClient := radixfake.NewSimpleClientset() //nolint:staticcheck
	kedaClient := kedafake.NewSimpleClientset()
	secretProviderClient := secretproviderfake.NewSimpleClientset()
	certClient := certfake.NewSimpleClientset()
	dynamicClient := commontest.CreateClient()
	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeClient, radixClient, kedaClient, secretProviderClient)
	err := commonTestUtils.CreateClusterPrerequisites(clusterName, subscriptionId)
	require.NoError(t, err)

	// controllerTestUtils is used for issuing HTTP request and processing responses
	mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
	mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(true), nil)
	controllerTestUtils := controllertest.NewTestUtils(kubeClient, radixClient, kedaClient, secretProviderClient, certClient, nil, mockValidator, NewDeploymentController())
	return &commonTestUtils, &controllerTestUtils, kubeClient, radixClient, kedaClient, dynamicClient, secretProviderClient, certClient
}
func TestGetPodLog_no_radixconfig(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _, _, _ := setupTest(t)

	endpoint := createGetLogEndpoint(anyAppName, anyPodName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 404, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, controllertest.AppNotFoundErrorMsg(anyAppName), errorResponse.Message)

}

func TestGetPodLog_No_Pod(t *testing.T) {
	commonTestUtils, controllerTestUtils, _, _, _, _, _, _ := setupTest(t)
	endpoint := createGetLogEndpoint(anyAppName, anyPodName)

	_, err := commonTestUtils.ApplyApplication(builders.
		ARadixApplication().
		WithAppName(anyAppName))
	require.NoError(t, err)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 404, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := deploymentModels.NonExistingPod(anyAppName, anyPodName)

	assert.Equal(t, (expectedError.(*radixhttp.Error)).Error(), errorResponse.Error())

}

func TestGetDeployments_Filter_FilterIsApplied(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _, _, _, _, _ := setupTest(t)

	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		builders.
			ARadixDeployment().
			WithAppName("any-app-1").
			WithEnvironment("prod").
			WithImageTag("abcdef").
			WithCondition(v1.DeploymentInactive))
	require.NoError(t, err)

	// Ensure the second image is considered the latest version
	firstDeploymentActiveFrom := time.Now()
	secondDeploymentActiveFrom := time.Now().AddDate(0, 0, 1)

	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		builders.
			ARadixDeployment().
			WithAppName("any-app-2").
			WithEnvironment("dev").
			WithImageTag("ghijklm").
			WithCreated(firstDeploymentActiveFrom).
			WithCondition(v1.DeploymentInactive).
			WithActiveFrom(firstDeploymentActiveFrom).
			WithActiveTo(secondDeploymentActiveFrom))
	require.NoError(t, err)

	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		builders.
			ARadixDeployment().
			WithAppName("any-app-2").
			WithEnvironment("dev").
			WithImageTag("nopqrst").
			WithCondition(v1.DeploymentActive).
			WithActiveFrom(secondDeploymentActiveFrom))
	require.NoError(t, err)

	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		builders.
			ARadixDeployment().
			WithAppName("any-app-2").
			WithEnvironment("prod").
			WithImageTag("uvwxyza"))
	require.NoError(t, err)

	// Test
	var testScenarios = []struct {
		name                   string
		appName                string
		environment            string
		latestOnly             bool
		numDeploymentsExpected int
	}{
		{"list all accross all environments", "any-app-2", "", false, 3},
		{"list all for environment", "any-app-2", "dev", false, 2},
		{"only list latest in environment", "any-app-2", "dev", true, 1},
		{"only list latest for app in all environments", "any-app-2", "", true, 2},
		{"non existing app should lead to empty list", "any-app-3", "", false, 0},
		{"non existing environment should lead to empty list", "any-app-2", "qa", false, 0},
	}

	for _, scenario := range testScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			endpoint := fmt.Sprintf("/api/v1/applications/%s/deployments", scenario.appName)
			queryParameters := ""

			if scenario.environment != "" {
				queryParameters = fmt.Sprintf("?environment=%s", scenario.environment)
			}

			if scenario.latestOnly {
				if queryParameters != "" {
					queryParameters = queryParameters + "&"
				} else {
					queryParameters = "?"
				}

				queryParameters = queryParameters + fmt.Sprintf("latest=%v", scenario.latestOnly)
			}

			responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint+queryParameters)
			response := <-responseChannel

			deployments := make([]*deploymentModels.DeploymentSummary, 0)
			_ = controllertest.GetResponseBody(response, &deployments)
			assert.Equal(t, scenario.numDeploymentsExpected, len(deployments))
		})
	}
}

func TestGetDeployments_NoApplicationRegistered(t *testing.T) {
	_, controllerTestUtils, _, _, _, _, _, _ := setupTest(t)
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deployments", anyAppName))
	response := <-responseChannel

	assert.Equal(t, 404, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, controllertest.AppNotFoundErrorMsg(anyAppName), errorResponse.Message)
}

func TestGetDeployments_OneEnvironment_SortedWithFromTo(t *testing.T) {
	deploymentOneImage := "abcdef"
	deploymentTwoImage := "ghijkl"
	deploymentThreeImage := "mnopqr"
	layout := "2006-01-02T15:04:05.000Z"
	deploymentOneCreated, _ := time.Parse(layout, "2018-11-12T11:45:26.371Z")
	deploymentTwoCreated, _ := time.Parse(layout, "2018-11-12T12:30:14.000Z")
	deploymentThreeCreated, _ := time.Parse(layout, "2018-11-20T09:00:00.000Z")
	gitTags := "some tags go here"
	gitCommitHash := "gfsjrgnsdkfgnlnfgdsMYCOMMIT"

	// Setup
	annotations := make(map[string]string)
	annotations[kube.RadixGitTagsAnnotation] = gitTags
	annotations[kube.RadixCommitAnnotation] = gitCommitHash
	commonTestUtils, controllerTestUtils, _, _, _, _, _, _ := setupTest(t)
	err := setupGetDeploymentsTest(commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, []string{"dev"}, annotations)
	require.NoError(t, err)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deployments", anyAppName))
	response := <-responseChannel

	deployments := make([]*deploymentModels.DeploymentSummary, 0)
	err = controllertest.GetResponseBody(response, &deployments)
	require.NoError(t, err)
	assert.Equal(t, 3, len(deployments))

	assert.Equal(t, deploymentThreeImage, deployments[0].Name)
	assert.Equal(t, deploymentThreeCreated, deployments[0].ActiveFrom)
	assert.Nil(t, deployments[0].ActiveTo)
	assert.Equal(t, gitCommitHash, deployments[0].GitCommitHash)
	assert.Equal(t, gitTags, deployments[0].GitTags)

	assert.Equal(t, deploymentTwoImage, deployments[1].Name)
	assert.Equal(t, deploymentTwoCreated, deployments[1].ActiveFrom)
	assert.Equal(t, &deploymentThreeCreated, deployments[1].ActiveTo)

	assert.Equal(t, deploymentOneImage, deployments[2].Name)
	assert.Equal(t, deploymentOneCreated, deployments[2].ActiveFrom)
	assert.Equal(t, &deploymentTwoCreated, deployments[2].ActiveTo)
}

func TestGetDeployments_OneEnvironment_Latest(t *testing.T) {
	deploymentOneImage := "abcdef"
	deploymentTwoImage := "ghijkl"
	deploymentThreeImage := "mnopqr"
	layout := "2006-01-02T15:04:05.000Z"
	deploymentOneCreated, _ := time.Parse(layout, "2018-11-12T11:45:26.371Z")
	deploymentTwoCreated, _ := time.Parse(layout, "2018-11-12T12:30:14.000Z")
	deploymentThreeCreated, _ := time.Parse(layout, "2018-11-20T09:00:00.000Z")

	// Setup
	annotations := make(map[string]string)
	annotations[kube.RadixGitTagsAnnotation] = "some tags go here"
	annotations[kube.RadixCommitAnnotation] = "gfsjrgnsdkfgnlnfgdsMYCOMMIT"
	commonTestUtils, controllerTestUtils, _, _, _, _, _, _ := setupTest(t)
	err := setupGetDeploymentsTest(commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, []string{"dev"}, annotations)
	require.NoError(t, err)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deployments?latest=true", anyAppName))
	response := <-responseChannel

	deployments := make([]*deploymentModels.DeploymentSummary, 0)
	err = controllertest.GetResponseBody(response, &deployments)
	require.NoError(t, err)
	assert.Equal(t, 1, len(deployments))

	assert.Equal(t, deploymentThreeImage, deployments[0].Name)
	assert.Equal(t, deploymentThreeCreated, deployments[0].ActiveFrom)
	assert.Nil(t, deployments[0].ActiveTo)
}

func TestGetDeployments_TwoEnvironments_SortedWithFromTo(t *testing.T) {
	deploymentOneImage := "abcdef"
	deploymentTwoImage := "ghijkl"
	deploymentThreeImage := "mnopqr"
	layout := "2006-01-02T15:04:05.000Z"
	deploymentOneCreated, _ := time.Parse(layout, "2018-11-12T11:45:26.371Z")
	deploymentTwoCreated, _ := time.Parse(layout, "2018-11-12T12:30:14.000Z")
	deploymentThreeCreated, _ := time.Parse(layout, "2018-11-20T09:00:00.000Z")

	// Setup
	annotations := make(map[string]string)
	annotations[kube.RadixGitTagsAnnotation] = "some tags go here"
	annotations[kube.RadixCommitAnnotation] = "gfsjrgnsdkfgnlnfgdsMYCOMMIT"
	commonTestUtils, controllerTestUtils, _, _, _, _, _, _ := setupTest(t)
	err := setupGetDeploymentsTest(commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, []string{"dev", "prod"}, annotations)
	require.NoError(t, err)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deployments", anyAppName))
	response := <-responseChannel

	deployments := make([]*deploymentModels.DeploymentSummary, 0)
	err = controllertest.GetResponseBody(response, &deployments)
	require.NoError(t, err)
	assert.Equal(t, 3, len(deployments))

	assert.Equal(t, deploymentThreeImage, deployments[0].Name)
	assert.Equal(t, deploymentThreeCreated, deployments[0].ActiveFrom)
	assert.Nil(t, deployments[0].ActiveTo)

	assert.Equal(t, deploymentTwoImage, deployments[1].Name)
	assert.Equal(t, deploymentTwoCreated, deployments[1].ActiveFrom)
	assert.Nil(t, deployments[1].ActiveTo)

	assert.Equal(t, deploymentOneImage, deployments[2].Name)
	assert.Equal(t, deploymentOneCreated, deployments[2].ActiveFrom)
	assert.Equal(t, &deploymentThreeCreated, deployments[2].ActiveTo)
}

func TestGetDeployments_TwoEnvironments_Latest(t *testing.T) {
	deploymentOneImage := "abcdef"
	deploymentTwoImage := "ghijkl"
	deploymentThreeImage := "mnopqr"
	layout := "2006-01-02T15:04:05.000Z"
	deploymentOneCreated, _ := time.Parse(layout, "2018-11-12T11:45:26.371Z")
	deploymentTwoCreated, _ := time.Parse(layout, "2018-11-12T12:30:14.000Z")
	deploymentThreeCreated, _ := time.Parse(layout, "2018-11-20T09:00:00.000Z")

	// Setup
	annotations := make(map[string]string)
	annotations[kube.RadixGitTagsAnnotation] = "some tags go here"
	annotations[kube.RadixCommitAnnotation] = "gfsjrgnsdkfgnlnfgdsMYCOMMIT"
	commonTestUtils, controllerTestUtils, _, _, _, _, _, _ := setupTest(t)
	err := setupGetDeploymentsTest(commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, []string{"dev", "prod"}, annotations)
	require.NoError(t, err)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deployments?latest=true", anyAppName))
	response := <-responseChannel

	deployments := make([]*deploymentModels.DeploymentSummary, 0)
	err = controllertest.GetResponseBody(response, &deployments)
	require.NoError(t, err)
	assert.Equal(t, 2, len(deployments))

	assert.Equal(t, deploymentThreeImage, deployments[0].Name)
	assert.Equal(t, deploymentThreeCreated, deployments[0].ActiveFrom)
	assert.Nil(t, deployments[0].ActiveTo)

	assert.Equal(t, deploymentTwoImage, deployments[1].Name)
	assert.Equal(t, deploymentTwoCreated, deployments[1].ActiveFrom)
	assert.Nil(t, deployments[1].ActiveTo)
}

func TestGetDeployment_NoApplicationRegistered(t *testing.T) {
	_, controllerTestUtils, _, _, _, _, _, _ := setupTest(t)
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deployments/%s", anyAppName, anyDeployName))
	response := <-responseChannel

	assert.Equal(t, 404, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, controllertest.AppNotFoundErrorMsg(anyAppName), errorResponse.Message)
}

func TestGetDeployment_TwoDeploymentsFirstDeployment_ReturnsDeploymentWithComponents(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _, _, _, _, _ := setupTest(t)
	anyAppName := "any-app"
	anyEnvironment := "dev"
	anyDeployment1Name := "abcdef"
	anyDeployment2Name := "ghijkl"
	appDeployment1Created, _ := radixutils.ParseTimestamp("2018-11-12T12:00:00Z")
	appDeployment2Created, _ := radixutils.ParseTimestamp("2018-11-14T12:00:00Z")
	jobName1, jobName2 := "rj1", "rj2"
	commitID1 := "commit1"

	_, err := commonTestUtils.ApplyJob(builders.ARadixBuildDeployJob().WithAppName(anyAppName).WithJobName(jobName1).WithCommitID(commitID1))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		builders.
			NewDeploymentBuilder().
			WithRadixApplication(
				builders.ARadixApplication().
					WithAppName(anyAppName)).
			WithAppName(anyAppName).
			WithLabel(kube.RadixJobNameLabel, jobName1).
			WithDeploymentName(anyDeployment1Name).
			WithCreated(appDeployment1Created).
			WithCondition(v1.DeploymentInactive).
			WithActiveFrom(appDeployment1Created).
			WithActiveTo(appDeployment2Created).
			WithEnvironment(anyEnvironment).
			WithImageTag(anyDeployment1Name).
			WithJobComponents(
				builders.NewDeployJobComponentBuilder().WithName("job1"),
				builders.NewDeployJobComponentBuilder().WithName("job2"),
			).
			WithComponents(
				builders.NewDeployComponentBuilder().
					WithImage("radixdev.azurecr.io/some-image:imagetag").
					WithName("frontend").
					WithPort("http", 8080).
					WithReplicas(commontest.IntPtr(1)),
				builders.NewDeployComponentBuilder().
					WithImage("radixdev.azurecr.io/another-image:imagetag").
					WithName("backend").
					WithReplicas(commontest.IntPtr(1))))
	require.NoError(t, err)

	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		builders.
			NewDeploymentBuilder().
			WithRadixApplication(
				builders.ARadixApplication().
					WithAppName(anyAppName)).
			WithAppName(anyAppName).
			WithLabel(kube.RadixJobNameLabel, jobName2).
			WithDeploymentName(anyDeployment2Name).
			WithCreated(appDeployment2Created).
			WithCondition(v1.DeploymentActive).
			WithActiveFrom(appDeployment2Created).
			WithEnvironment(anyEnvironment).
			WithImageTag(anyDeployment2Name).
			WithJobComponents(
				builders.NewDeployJobComponentBuilder().WithName("job1"),
			).
			WithComponents(
				builders.NewDeployComponentBuilder().
					WithImage("radixdev.azurecr.io/another-second-image:imagetag").
					WithName("backend").
					WithReplicas(commontest.IntPtr(1))))
	require.NoError(t, err)

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deployments/%s", anyAppName, anyDeployment1Name))
	response := <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)

	deployment := deploymentModels.Deployment{}
	err = controllertest.GetResponseBody(response, &deployment)
	require.NoError(t, err)

	assert.Equal(t, anyDeployment1Name, deployment.Name)
	assert.Equal(t, appDeployment1Created, deployment.ActiveFrom)
	assert.Equal(t, &appDeployment2Created, deployment.ActiveTo)
	assert.Equal(t, 4, len(deployment.Components))

}

func setupGetDeploymentsTest(commonTestUtils *commontest.Utils, appName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage string, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated time.Time, environments []string, annotations map[string]string) error {
	var environmentOne, environmentTwo string
	var deploymentOneActiveTo, deploymentTwoActiveTo time.Time
	var deploymentTwoCondition v1.RadixDeployCondition

	if len(environments) == 1 {
		environmentOne = environments[0]
		environmentTwo = environments[0]
		deploymentOneActiveTo = deploymentTwoCreated
		deploymentTwoActiveTo = deploymentThreeCreated
		deploymentTwoCondition = v1.DeploymentInactive

	} else {
		environmentOne = environments[0]
		environmentTwo = environments[1]
		deploymentOneActiveTo = deploymentThreeCreated
		deploymentTwoCondition = v1.DeploymentActive
	}

	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		builders.
			ARadixDeployment().
			WithAnnotations(annotations).
			WithDeploymentName(deploymentOneImage).
			WithAppName(appName).
			WithEnvironment(environmentOne).
			WithImageTag(deploymentOneImage).
			WithCreated(deploymentOneCreated).
			WithCondition(v1.DeploymentInactive).
			WithActiveFrom(deploymentOneCreated).
			WithActiveTo(deploymentOneActiveTo))
	if err != nil {
		return err
	}

	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		builders.
			ARadixDeployment().
			WithAnnotations(annotations).
			WithDeploymentName(deploymentTwoImage).
			WithAppName(appName).
			WithEnvironment(environmentTwo).
			WithImageTag(deploymentTwoImage).
			WithCreated(deploymentTwoCreated).
			WithCondition(deploymentTwoCondition).
			WithActiveFrom(deploymentTwoCreated).
			WithActiveTo(deploymentTwoActiveTo))
	if err != nil {
		return err
	}

	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		builders.
			ARadixDeployment().
			WithAnnotations(annotations).
			WithDeploymentName(deploymentThreeImage).
			WithAppName(appName).
			WithEnvironment(environmentOne).
			WithImageTag(deploymentThreeImage).
			WithCreated(deploymentThreeCreated).
			WithCondition(v1.DeploymentActive).
			WithActiveFrom(deploymentThreeCreated))
	if err != nil {
		return err
	}
	return nil
}

package pipelineservice

import (
	"testing"

	radixhttp "github.com/equinor/radix-common/net/http"
	applicationModels "github.com/equinor/radix-operator/api-server/api/applications/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testAppName      = "my-app"
	testConfigBranch = "main"
)

func setupTest(t *testing.T) (*PipelineService, *radixfake.Clientset) {
	t.Helper()
	radixClient := radixfake.NewSimpleClientset() //nolint:staticcheck
	svc := New(radixClient)
	return svc, radixClient
}

func registerTestApp(t *testing.T, radixClient radixclient.Interface) {
	t.Helper()
	rr := operatorutils.NewRegistrationBuilder().
		WithName(testAppName).
		WithCloneURL("git@github.com:equinor/my-app.git").
		WithConfigBranch(testConfigBranch).
		BuildRR()
	_, err := radixClient.RadixV1().RadixRegistrations().Create(t.Context(), rr, metav1.CreateOptions{})
	require.NoError(t, err)
}

// assertValidationErrorMessage asserts that err is a radixhttp validation error with the expected message.
func assertValidationErrorMessage(t *testing.T, err error, expectedMessage string) {
	t.Helper()
	var httpErr *radixhttp.Error
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, expectedMessage, httpErr.Message)
}

func TestNew(t *testing.T) {
	radixClient := radixfake.NewSimpleClientset() //nolint:staticcheck
	svc := New(radixClient)
	require.NotNil(t, svc)
	assert.Equal(t, radixClient, svc.RadixClient)
}

func TestTriggerPipelinePromote(t *testing.T) {
	t.Run("missing required parameters returns validation error", func(t *testing.T) {
		svc, radixClient := setupTest(t)
		registerTestApp(t, radixClient)

		cases := []applicationModels.PipelineParametersPromote{
			{DeploymentName: "", FromEnvironment: "dev", ToEnvironment: "prod"},
			{DeploymentName: "dev-abc-123", FromEnvironment: "", ToEnvironment: "prod"},
			{DeploymentName: "dev-abc-123", FromEnvironment: "dev", ToEnvironment: ""},
		}
		for _, params := range cases {
			jobSummary, err := svc.TriggerPipelinePromote(t.Context(), testAppName, false, params)
			assert.Nil(t, jobSummary)
			assertValidationErrorMessage(t, err, "Deployment name, from environment and to environment are required for \"promote\" pipeline")
		}
	})

	t.Run("deployment not found returns error", func(t *testing.T) {
		svc, radixClient := setupTest(t)
		registerTestApp(t, radixClient)

		params := applicationModels.PipelineParametersPromote{
			DeploymentName:  "non-existing",
			FromEnvironment: "dev",
			ToEnvironment:   "prod",
		}
		jobSummary, err := svc.TriggerPipelinePromote(t.Context(), testAppName, false, params)
		assert.Nil(t, jobSummary)
		assert.EqualError(t, err, "invalid or not existing deployment name")
	})

	t.Run("success creates promote job", func(t *testing.T) {
		svc, radixClient := setupTest(t)
		registerTestApp(t, radixClient)

		const deploymentName = "dev-abc-123"
		const commitID = "4faca8595c5283a9d0f17a623b9255a0d9866a2e"
		rd := operatorutils.NewDeploymentBuilder().
			WithAppName(testAppName).
			WithEnvironment("dev").
			WithDeploymentName(deploymentName).
			WithLabel(kube.RadixCommitLabel, commitID).
			BuildRD()
		_, err := radixClient.RadixV1().RadixDeployments(operatorutils.GetEnvironmentNamespace(testAppName, "dev")).
			Create(t.Context(), rd, metav1.CreateOptions{})
		require.NoError(t, err)

		params := applicationModels.PipelineParametersPromote{
			DeploymentName:  deploymentName,
			FromEnvironment: "dev",
			ToEnvironment:   "prod",
			TriggeredBy:     "a_user@equinor.com",
		}
		jobSummary, err := svc.TriggerPipelinePromote(t.Context(), testAppName, false, params)
		require.NoError(t, err)
		require.NotNil(t, jobSummary)
		assert.Equal(t, testAppName, jobSummary.AppName)
		assert.Equal(t, string(radixv1.Promote), jobSummary.Pipeline)
		assert.Equal(t, deploymentName, jobSummary.PromotedFromDeployment)
		assert.Equal(t, "dev", jobSummary.PromotedFromEnvironment)
		assert.Equal(t, "prod", jobSummary.PromotedToEnvironment)
		assert.Equal(t, commitID, jobSummary.CommitID)
		assert.Equal(t, "a_user@equinor.com", jobSummary.TriggeredBy)

		jobs, err := radixClient.RadixV1().RadixJobs(operatorutils.GetAppNamespace(testAppName)).List(t.Context(), metav1.ListOptions{})
		require.NoError(t, err)
		assert.Len(t, jobs.Items, 1)
	})

	t.Run("success when deployment name is a suffix", func(t *testing.T) {
		svc, radixClient := setupTest(t)
		registerTestApp(t, radixClient)

		const fullName = "dev-abc-123"
		const suffix = "123"
		rd := operatorutils.NewDeploymentBuilder().
			WithAppName(testAppName).
			WithEnvironment("dev").
			WithDeploymentName(fullName).
			BuildRD()
		_, err := radixClient.RadixV1().RadixDeployments(operatorutils.GetEnvironmentNamespace(testAppName, "dev")).
			Create(t.Context(), rd, metav1.CreateOptions{})
		require.NoError(t, err)

		params := applicationModels.PipelineParametersPromote{
			DeploymentName:  suffix,
			FromEnvironment: "dev",
			ToEnvironment:   "prod",
		}
		jobSummary, err := svc.TriggerPipelinePromote(t.Context(), testAppName, false, params)
		require.NoError(t, err)
		require.NotNil(t, jobSummary)
		assert.Equal(t, fullName, jobSummary.PromotedFromDeployment)
	})
}

func TestTriggerPipelineDeploy(t *testing.T) {
	t.Run("missing toEnvironment returns validation error", func(t *testing.T) {
		svc, radixClient := setupTest(t)
		registerTestApp(t, radixClient)

		jobSummary, err := svc.TriggerPipelineDeploy(t.Context(), testAppName, false, applicationModels.PipelineParametersDeploy{})
		assert.Nil(t, jobSummary)
		assertValidationErrorMessage(t, err, "To environment is required for \"deploy\" pipeline")
	})

	t.Run("success creates deploy job", func(t *testing.T) {
		svc, radixClient := setupTest(t)
		registerTestApp(t, radixClient)

		params := applicationModels.PipelineParametersDeploy{
			ToEnvironment:      "prod",
			CommitID:           "4faca8595c5283a9d0f17a623b9255a0d9866a2e",
			ImageTagNames:      map[string]string{"component1": "tag1"},
			ComponentsToDeploy: []string{"component1"},
			TriggeredBy:        "a_user@equinor.com",
		}
		jobSummary, err := svc.TriggerPipelineDeploy(t.Context(), testAppName, false, params)
		require.NoError(t, err)
		require.NotNil(t, jobSummary)
		assert.Equal(t, testAppName, jobSummary.AppName)
		assert.Equal(t, string(radixv1.Deploy), jobSummary.Pipeline)
		assert.Equal(t, params.CommitID, jobSummary.CommitID)
		assert.Equal(t, params.ImageTagNames, jobSummary.ImageTagNames)
		assert.Equal(t, "a_user@equinor.com", jobSummary.TriggeredBy)
	})
}

func TestTriggerPipelineApplyConfig(t *testing.T) {
	t.Run("success creates apply-config job", func(t *testing.T) {
		svc, radixClient := setupTest(t)
		registerTestApp(t, radixClient)

		deployExternalDNS := true
		params := applicationModels.PipelineParametersApplyConfig{
			TriggeredBy:       "a_user@equinor.com",
			DeployExternalDNS: &deployExternalDNS,
		}
		jobSummary, err := svc.TriggerPipelineApplyConfig(t.Context(), testAppName, false, params)
		require.NoError(t, err)
		require.NotNil(t, jobSummary)
		assert.Equal(t, testAppName, jobSummary.AppName)
		assert.Equal(t, string(radixv1.ApplyConfig), jobSummary.Pipeline)
		require.NotNil(t, jobSummary.DeployExternalDNS)
		assert.True(t, *jobSummary.DeployExternalDNS)
	})
}

func TestTriggerPipelineBuild(t *testing.T) {
	t.Run("missing gitRef returns validation error", func(t *testing.T) {
		svc, radixClient := setupTest(t)
		registerTestApp(t, radixClient)

		jobSummary, err := svc.TriggerPipelineBuild(t.Context(), testAppName, false, applicationModels.PipelineParametersBuild{})
		assert.Nil(t, jobSummary)
		assertValidationErrorMessage(t, err, "App name and branch are required")
	})

	t.Run("success on config branch creates build job", func(t *testing.T) {
		svc, radixClient := setupTest(t)
		registerTestApp(t, radixClient)

		params := applicationModels.PipelineParametersBuild{
			GitRef:      testConfigBranch,
			CommitID:    "4faca8595c5283a9d0f17a623b9255a0d9866a2e",
			PushImage:   "true",
			TriggeredBy: "a_user@equinor.com",
		}
		jobSummary, err := svc.TriggerPipelineBuild(t.Context(), testAppName, false, params)
		require.NoError(t, err)
		require.NotNil(t, jobSummary)
		assert.Equal(t, testAppName, jobSummary.AppName)
		assert.Equal(t, string(radixv1.Build), jobSummary.Pipeline)
		assert.Equal(t, testConfigBranch, jobSummary.GitRef)
		assert.Equal(t, params.CommitID, jobSummary.CommitID)
	})

	t.Run("unmatched branch returns error", func(t *testing.T) {
		svc, radixClient := setupTest(t)
		registerTestApp(t, radixClient)

		ra := operatorutils.NewRadixApplicationBuilder().
			WithAppName(testAppName).
			WithEnvironment("prod", "release").
			BuildRA()
		_, err := radixClient.RadixV1().RadixApplications(operatorutils.GetAppNamespace(testAppName)).
			Create(t.Context(), ra, metav1.CreateOptions{})
		require.NoError(t, err)

		params := applicationModels.PipelineParametersBuild{
			GitRef:   "feature-branch",
			CommitID: "4faca8595c5283a9d0f17a623b9255a0d9866a2e",
		}
		jobSummary, err := svc.TriggerPipelineBuild(t.Context(), testAppName, false, params)
		assert.Nil(t, jobSummary)
		assertValidationErrorMessage(t, err, "Failed to match environment to branch: feature-branch")
	})
}

func TestTriggerPipelineBuildDeploy(t *testing.T) {
	t.Run("missing gitRef returns validation error", func(t *testing.T) {
		svc, radixClient := setupTest(t)
		registerTestApp(t, radixClient)

		jobSummary, err := svc.TriggerPipelineBuildDeploy(t.Context(), testAppName, false, applicationModels.PipelineParametersBuild{})
		assert.Nil(t, jobSummary)
		assertValidationErrorMessage(t, err, "App name and branch are required")
	})

	t.Run("success creates build-deploy job for mapped environment", func(t *testing.T) {
		svc, radixClient := setupTest(t)
		registerTestApp(t, radixClient)

		ra := operatorutils.NewRadixApplicationBuilder().
			WithAppName(testAppName).
			WithEnvironment("dev", "feature-branch").
			BuildRA()
		_, err := radixClient.RadixV1().RadixApplications(operatorutils.GetAppNamespace(testAppName)).
			Create(t.Context(), ra, metav1.CreateOptions{})
		require.NoError(t, err)

		params := applicationModels.PipelineParametersBuild{
			GitRef:        "feature-branch",
			ToEnvironment: "dev",
			CommitID:      "4faca8595c5283a9d0f17a623b9255a0d9866a2e",
		}
		jobSummary, err := svc.TriggerPipelineBuildDeploy(t.Context(), testAppName, false, params)
		require.NoError(t, err)
		require.NotNil(t, jobSummary)
		assert.Equal(t, string(radixv1.BuildDeploy), jobSummary.Pipeline)
	})

	t.Run("environment not mapped to branch returns error", func(t *testing.T) {
		svc, radixClient := setupTest(t)
		registerTestApp(t, radixClient)

		ra := operatorutils.NewRadixApplicationBuilder().
			WithAppName(testAppName).
			WithEnvironment("dev", "feature-branch").
			BuildRA()
		_, err := radixClient.RadixV1().RadixApplications(operatorutils.GetAppNamespace(testAppName)).
			Create(t.Context(), ra, metav1.CreateOptions{})
		require.NoError(t, err)

		params := applicationModels.PipelineParametersBuild{
			GitRef:        "feature-branch",
			ToEnvironment: "prod",
			CommitID:      "4faca8595c5283a9d0f17a623b9255a0d9866a2e",
		}
		jobSummary, err := svc.TriggerPipelineBuildDeploy(t.Context(), testAppName, false, params)
		assert.Nil(t, jobSummary)
		assertValidationErrorMessage(t, err, "Failed to match environment prod to branch: feature-branch")
	})
}

func TestTriggerPipeline_AppNotRegistered(t *testing.T) {
	svc, _ := setupTest(t)

	params := applicationModels.PipelineParametersDeploy{ToEnvironment: "prod"}
	jobSummary, err := svc.TriggerPipelineDeploy(t.Context(), testAppName, false, params)
	assert.Error(t, err)
	assert.Nil(t, jobSummary)
}

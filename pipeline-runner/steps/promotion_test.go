package steps

import (
	"fmt"
	"strings"
	"testing"

	"github.com/coreos/prometheus-operator/pkg/client/monitoring"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/test"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
)

func TestPromote_ErrorScenarios_ErrorIsReturned(t *testing.T) {
	anyApp1 := "any-app-1"
	anyApp2 := "any-app-2"
	anyDeployment1 := "1"
	anyDeployment2 := "2"
	anyDeployment3 := "3"
	anyProdEnvironment := "prod"
	anyDevEnvironment := "dev"
	anyQAEnvironment := "qa"
	anyImageTag := "abcdef"
	anyJobName := "radix-pipeline-abcdef"

	// Setup
	kubeclient, kube, radixclient, commonTestUtils := setupTest()

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithDeploymentName(anyDeployment1).
		WithAppName(anyApp1).
		WithEnvironment(anyProdEnvironment).
		WithImageTag(anyImageTag))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithDeploymentName(anyDeployment2).
		WithAppName(anyApp1).
		WithEnvironment(anyDevEnvironment).
		WithImageTag(anyImageTag))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithDeploymentName(anyDeployment3).
		WithAppName(anyApp2).
		WithEnvironment(anyDevEnvironment).
		WithImageTag(anyImageTag))

	test.CreateEnvNamespace(kubeclient, anyApp2, anyProdEnvironment)

	var testScenarios = []struct {
		name            string
		appName         string
		fromEnvironment string
		imageTag        string
		jobName         string
		toEnvironment   string
		deploymentName  string
		expectedError   error
	}{
		{"empty from environment", anyApp1, "", anyImageTag, anyJobName, anyProdEnvironment, anyDeployment2, EmptyArgument("From environment")},
		{"empty to environment", anyApp1, anyDevEnvironment, anyImageTag, anyJobName, "", anyDeployment2, EmptyArgument("To environment")},
		{"empty image tag", anyApp1, anyDevEnvironment, "", anyJobName, anyProdEnvironment, anyDeployment2, EmptyArgument("Image tag")},
		{"empty job name", anyApp1, anyDevEnvironment, anyImageTag, "", anyProdEnvironment, anyDeployment2, EmptyArgument("Job name")},
		{"empty deployment name", anyApp1, anyDevEnvironment, anyImageTag, anyJobName, anyProdEnvironment, "", EmptyArgument("Deployment name")},
		{"promote from non-existing environment", anyApp1, anyQAEnvironment, anyImageTag, anyJobName, anyProdEnvironment, anyDeployment2, NonExistingFromEnvironment(anyQAEnvironment)},
		{"promote to non-existing environment", anyApp1, anyDevEnvironment, anyImageTag, anyJobName, anyQAEnvironment, anyDeployment2, NonExistingToEnvironment(anyQAEnvironment)},
		{"promote non-existing deployment", anyApp2, anyDevEnvironment, "nopqrst", anyJobName, anyProdEnvironment, "non-existing", NonExistingDeployment("non-existing")},
	}

	for _, scenario := range testScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			rr, _ := radixclient.RadixV1().RadixRegistrations().Get(scenario.appName, metav1.GetOptions{})
			ra, _ := radixclient.RadixV1().RadixApplications(utils.GetAppNamespace(scenario.appName)).Get(scenario.appName, metav1.GetOptions{})

			cli := NewPromoteStep()
			cli.Init(kubeclient, radixclient, kube, &monitoring.Clientset{})

			pipelineInfo := &model.PipelineInfo{
				RadixRegistration: rr,
				RadixApplication:  ra,
				PipelineArguments: model.PipelineArguments{
					FromEnvironment: scenario.fromEnvironment,
					ToEnvironment:   scenario.toEnvironment,
					DeploymentName:  scenario.deploymentName,
					JobName:         scenario.jobName,
					ImageTag:        scenario.imageTag,
					CommitID:        anyCommitID,
				},
			}

			err := cli.Run(pipelineInfo)
			assert.Error(t, err)

			if scenario.expectedError != nil {
				assert.Equal(t, scenario.expectedError.Error(), err.Error())
			}
		})
	}
}

func TestPromote_PromoteToOtherEnvironment_NewStateIsExpected(t *testing.T) {
	anyApp := "any-app"
	anyDeploymentName := "deployment-1"
	anyImageTag := "abcdef"
	anyBuildDeployJobName := "any-build-deploy-job"
	anyPromoteJobName := "any-promote-job"
	anyProdEnvironment := "prod"
	anyDevEnvironment := "dev"

	// Setup
	kubeclient, kubeUtil, radixclient, commonTestUtils := setupTest()

	commonTestUtils.ApplyDeployment(
		builders.ARadixDeployment().
			WithRadixApplication(
				utils.NewRadixApplicationBuilder().
					WithRadixRegistration(
						utils.ARadixRegistration().
							WithName(anyApp)).
					WithAppName(anyApp).
					WithEnvironment(anyDevEnvironment, "master").
					WithEnvironment(anyProdEnvironment, "").
					WithComponents(
						utils.AnApplicationComponent().
							WithName("app").
							WithEnvironmentConfigs(
								utils.AnEnvironmentConfig().
									WithEnvironment(anyDevEnvironment).
									WithReplicas(2).
									WithEnvironmentVariable("DB_HOST", "db-dev").
									WithEnvironmentVariable("DB_PORT", "1234"),
								utils.AnEnvironmentConfig().
									WithEnvironment(anyProdEnvironment).
									WithReplicas(4).
									WithEnvironmentVariable("DB_HOST", "db-prod").
									WithEnvironmentVariable("DB_PORT", "5678")))).
			WithAppName(anyApp).
			WithDeploymentName(anyDeploymentName).
			WithEnvironment(anyDevEnvironment).
			WithImageTag(anyImageTag).
			WithLabel(kube.RadixJobNameLabel, anyBuildDeployJobName))

	// Create prod environment without any deployments
	test.CreateEnvNamespace(kubeclient, anyApp, anyProdEnvironment)

	rr, _ := radixclient.RadixV1().RadixRegistrations().Get(anyApp, metav1.GetOptions{})
	ra, _ := radixclient.RadixV1().RadixApplications(utils.GetAppNamespace(anyApp)).Get(anyApp, metav1.GetOptions{})

	cli := NewPromoteStep()
	cli.Init(kubeclient, radixclient, kubeUtil, &monitoring.Clientset{})

	pipelineInfo := &model.PipelineInfo{
		RadixRegistration: rr,
		RadixApplication:  ra,
		PipelineArguments: model.PipelineArguments{
			FromEnvironment: anyDevEnvironment,
			ToEnvironment:   anyProdEnvironment,
			DeploymentName:  anyDeploymentName,
			JobName:         anyPromoteJobName,
			ImageTag:        anyImageTag,
			CommitID:        anyCommitID,
		},
	}

	err := cli.Run(pipelineInfo)
	assert.NoError(t, err)

	rds, _ := radixclient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(anyApp, anyProdEnvironment)).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(rds.Items))
	assert.True(t, strings.HasPrefix(rds.Items[0].Name, fmt.Sprintf("%s-%s-", anyProdEnvironment, anyImageTag)))
	assert.Equal(t, anyProdEnvironment, rds.Items[0].Labels[kube.RadixEnvLabel])
	assert.Equal(t, anyImageTag, rds.Items[0].Labels[kube.RadixImageTagLabel])
	assert.Equal(t, anyPromoteJobName, rds.Items[0].Labels[kube.RadixJobNameLabel])
	assert.Equal(t, 4, rds.Items[0].Spec.Components[0].Replicas)
	assert.Equal(t, "db-prod", rds.Items[0].Spec.Components[0].EnvironmentVariables["DB_HOST"])
	assert.Equal(t, "5678", rds.Items[0].Spec.Components[0].EnvironmentVariables["DB_PORT"])
}

func TestPromote_PromoteToSameEnvironment_NewStateIsExpected(t *testing.T) {
	anyApp := "any-app"
	anyDeploymentName := "deployment-1"
	anyImageTag := "abcdef"
	anyBuildDeployJobName := "any-build-deploy-job"
	anyPromoteJobName := "any-promote-job"
	anyDevEnvironment := "dev"

	// Setup
	kubeclient, kubeUtil, radixclient, commonTestUtils := setupTest()

	commonTestUtils.ApplyDeployment(
		builders.ARadixDeployment().
			WithAppName(anyApp).
			WithDeploymentName(anyDeploymentName).
			WithEnvironment(anyDevEnvironment).
			WithImageTag(anyImageTag).
			WithLabel(kube.RadixJobNameLabel, anyBuildDeployJobName))

	rr, _ := radixclient.RadixV1().RadixRegistrations().Get(anyApp, metav1.GetOptions{})
	ra, _ := radixclient.RadixV1().RadixApplications(utils.GetAppNamespace(anyApp)).Get(anyApp, metav1.GetOptions{})

	cli := NewPromoteStep()
	cli.Init(kubeclient, radixclient, kubeUtil, &monitoring.Clientset{})

	pipelineInfo := &model.PipelineInfo{
		RadixRegistration: rr,
		RadixApplication:  ra,
		PipelineArguments: model.PipelineArguments{
			FromEnvironment: anyDevEnvironment,
			ToEnvironment:   anyDevEnvironment,
			DeploymentName:  anyDeploymentName,
			JobName:         anyPromoteJobName,
			ImageTag:        anyImageTag,
			CommitID:        anyCommitID,
		},
	}

	err := cli.Run(pipelineInfo)
	assert.NoError(t, err)

	rds, _ := radixclient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(anyApp, anyDevEnvironment)).List(metav1.ListOptions{})
	assert.Equal(t, 2, len(rds.Items))
}

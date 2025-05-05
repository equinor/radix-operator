// nolint:staticcheck // SA1019: Ignore linting deprecated fields
package promote_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	commonslice "github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/promote"
	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commonTest "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

const (
	anyAppName  = "any-app"
	anyJobName  = "any-job-name"
	anyImageTag = "anytag"
	anyCommitID = "4faca8595c5283a9d0f17a623b9255a0d9866a2e"
	anyGitTags  = "some tags go here"
)

func setupTest(t *testing.T) (*kubernetes.Clientset, *kube.Kube, *radix.Clientset, commonTest.Utils) {
	// Setup
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()
	kedaClient := kedafake.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	testUtils := commonTest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient)
	err := testUtils.CreateClusterPrerequisites("AnyClusterName", "0.0.0.0", "anysubid")
	require.NoError(t, err)
	kubeUtil, _ := kube.New(kubeclient, radixclient, kedaClient, secretproviderclient)

	return kubeclient, kubeUtil, radixclient, testUtils
}

func TestPromote_ErrorScenarios_ErrorIsReturned(t *testing.T) {
	anyApp1 := "any-app-1"
	anyApp2 := "any-app-2"
	anyApp4 := "any-app-4"
	anyApp5 := "any-app-5"
	anyDeployment1 := "1"
	anyDeployment2 := "2"
	anyDeployment3 := "3"
	anyDeployment4 := "4"
	anyDeployment5 := "5"
	anyProdEnvironment := "prod"
	anyDevEnvironment := "dev"
	anyQAEnvironment := "qa"
	anyImageTag := "abcdef"
	anyJobName := "radix-pipeline-abcdef"
	nonExistingComponent := "non-existing-comp"
	nonExistingJobComponent := "non-existing-job"

	// Setup
	kubeclient, kube, radixclient, commonTestUtils := setupTest(t)

	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		utils.ARadixDeployment().
			WithDeploymentName(anyDeployment1).
			WithAppName(anyApp1).
			WithEnvironment(anyProdEnvironment).
			WithImageTag(anyImageTag))
	require.NoError(t, err)

	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		utils.
			ARadixDeployment().
			WithDeploymentName(anyDeployment2).
			WithAppName(anyApp1).
			WithEnvironment(anyDevEnvironment).
			WithImageTag(anyImageTag))
	require.NoError(t, err)

	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		utils.
			ARadixDeployment().
			WithDeploymentName(anyDeployment3).
			WithAppName(anyApp2).
			WithEnvironment(anyDevEnvironment).
			WithImageTag(anyImageTag))
	require.NoError(t, err)

	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		utils.
			ARadixDeployment().
			WithDeploymentName(anyDeployment4).
			WithAppName(anyApp4).
			WithEnvironment(anyDevEnvironment).
			WithImageTag(anyImageTag).
			WithComponent(utils.
				NewDeployComponentBuilder().
				WithName(nonExistingComponent)))
	require.NoError(t, err)

	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		utils.
			ARadixDeployment().
			WithDeploymentName(anyDeployment5).
			WithAppName(anyApp5).
			WithEnvironment(anyDevEnvironment).
			WithImageTag(anyImageTag).
			WithJobComponent(utils.
				NewDeployJobComponentBuilder().
				WithName(nonExistingJobComponent)))
	require.NoError(t, err)

	commonTest.CreateEnvNamespace(kubeclient, anyApp2, anyProdEnvironment)

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
		{"empty from environment", anyApp1, "", anyImageTag, anyJobName, anyProdEnvironment, anyDeployment2, promote.EmptyArgument("From environment")},
		{"empty to environment", anyApp1, anyDevEnvironment, anyImageTag, anyJobName, "", anyDeployment2, promote.EmptyArgument("To environment")},
		{"empty image tag", anyApp1, anyDevEnvironment, "", anyJobName, anyProdEnvironment, anyDeployment2, promote.EmptyArgument("Image tag")},
		{"empty job name", anyApp1, anyDevEnvironment, anyImageTag, "", anyProdEnvironment, anyDeployment2, promote.EmptyArgument("Job name")},
		{"empty deployment name", anyApp1, anyDevEnvironment, anyImageTag, anyJobName, anyProdEnvironment, "", promote.EmptyArgument("Deployment name")},
		{"promote from non-existing environment", anyApp1, anyQAEnvironment, anyImageTag, anyJobName, anyProdEnvironment, anyDeployment2, promote.NonExistingFromEnvironment(anyQAEnvironment)},
		{"promote to non-existing environment", anyApp1, anyDevEnvironment, anyImageTag, anyJobName, anyQAEnvironment, anyDeployment2, promote.NonExistingToEnvironment(anyQAEnvironment)},
		{"promote non-existing deployment", anyApp2, anyDevEnvironment, "nopqrst", anyJobName, anyProdEnvironment, "non-existing", promote.NonExistingDeployment("non-existing")},
		{"promote deployment with non-existing component", anyApp4, anyDevEnvironment, anyImageTag, anyJobName, anyDevEnvironment, anyDeployment4, promote.NonExistingComponentName(anyApp4, nonExistingComponent)},
		{"promote deployment with non-existing job component", anyApp5, anyDevEnvironment, anyImageTag, anyJobName, anyDevEnvironment, anyDeployment5, promote.NonExistingComponentName(anyApp5, nonExistingJobComponent)},
	}

	for _, scenario := range testScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			rr, _ := radixclient.RadixV1().RadixRegistrations().Get(context.Background(), scenario.appName, metav1.GetOptions{})

			cli := promote.NewPromoteStep()
			cli.Init(context.Background(), kubeclient, radixclient, kube, &monitoring.Clientset{}, nil, rr)

			pipelineInfo := &model.PipelineInfo{
				PipelineArguments: model.PipelineArguments{
					FromEnvironment: scenario.fromEnvironment,
					ToEnvironment:   scenario.toEnvironment,
					DeploymentName:  scenario.deploymentName,
					JobName:         scenario.jobName,
					ImageTag:        scenario.imageTag,
					CommitID:        anyCommitID,
				},
			}

			err = cli.Run(context.Background(), pipelineInfo)

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
	anyDNSAlias1 := "a-dns-alias1"
	anyDNSAlias2 := "a-dns-alias2"
	anyExternalDNSAlias := "an-external-dns-alias"
	prodNode := v1.RadixNode{Gpu: "prod-gpu", GpuCount: "2"}

	// Setup
	kubeclient, kubeUtil, radixclient, commonTestUtils := setupTest(t)

	secretType := v1.RadixAzureKeyVaultObjectTypeSecret
	keyType := v1.RadixAzureKeyVaultObjectTypeKey
	componentName1 := "app"
	dnsAlias1 := v1.DNSAlias{
		Alias:       anyDNSAlias1,
		Environment: anyProdEnvironment,
		Component:   componentName1,
	}
	dnsAlias2 := v1.DNSAlias{
		Alias:       anyDNSAlias2,
		Environment: anyProdEnvironment,
		Component:   componentName1,
	}
	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		utils.NewDeploymentBuilder().
			WithComponent(
				utils.NewDeployComponentBuilder().
					WithName("app").
					WithSecrets([]string{"DEPLOYAPPSECRET"}).
					WithNodeGpu("dev-gpu").
					WithNodeGpuCount("1"),
			).
			WithJobComponent(
				utils.NewDeployJobComponentBuilder().
					WithName("job").
					WithSecrets([]string{"DEPLOYJOBSECRET"}),
			).
			WithRadixApplication(
				utils.NewRadixApplicationBuilder().
					WithRadixRegistration(
						utils.ARadixRegistration().
							WithName(anyApp)).
					WithAppName(anyApp).
					WithEnvironment(anyDevEnvironment, "master").
					WithEnvironment(anyProdEnvironment, "").
					WithDNSAppAlias(anyProdEnvironment, "app").
					WithDNSExternalAlias(anyExternalDNSAlias, anyProdEnvironment, "app", false).
					WithDNSAlias(dnsAlias1, dnsAlias2).
					WithComponents(
						utils.AnApplicationComponent().
							WithName("app").
							WithSecrets("APPSECRET1", "APPSECRET2").
							WithCommonEnvironmentVariable("DB_TYPE", "mysql").
							WithCommonEnvironmentVariable("DB_NAME", "my-db").
							WithSecretRefs(v1.RadixSecretRefs{AzureKeyVaults: []v1.RadixAzureKeyVault{{
								Name: "TestKeyVault2",
								Items: []v1.RadixAzureKeyVaultItem{
									{
										Name:   "Secret2",
										EnvVar: "SECRET_2",
										Type:   &secretType,
									},
									{
										Name:   "Key2",
										EnvVar: "KEY_2",
										Type:   &keyType,
									},
								},
							}}}).
							WithEnvironmentConfigs(
								utils.AnEnvironmentConfig().
									WithEnvironment(anyDevEnvironment).
									WithReplicas(commonTest.IntPtr(2)).
									WithEnvironmentVariable("DB_HOST", "db-dev").
									WithEnvironmentVariable("DB_PORT", "1234"),
								utils.AnEnvironmentConfig().
									WithEnvironment(anyProdEnvironment).
									WithReplicas(commonTest.IntPtr(4)).
									WithEnvironmentVariable("DB_HOST", "db-prod").
									WithEnvironmentVariable("DB_PORT", "5678").
									WithEnvironmentVariable("DB_NAME", "my-db-prod").
									WithNode(prodNode))).
					WithJobComponents(
						utils.AnApplicationJobComponent().
							WithName("job").
							WithSchedulerPort(numbers.Int32Ptr(8888)).
							WithPayloadPath(utils.StringPtr("/path")).
							WithSecrets("JOBSECRET1", "JOBSECRET2").
							WithCommonEnvironmentVariable("COMMON1", "common1").
							WithCommonEnvironmentVariable("COMMON2", "common2").
							WithSecretRefs(v1.RadixSecretRefs{AzureKeyVaults: []v1.RadixAzureKeyVault{{
								Name: "TestKeyVault",
								Items: []v1.RadixAzureKeyVaultItem{
									{
										Name:   "Secret1",
										EnvVar: "SECRET_1",
										Type:   &secretType,
									},
									{
										Name:   "Key1",
										EnvVar: "KEY_1",
										Type:   &keyType,
									},
								},
							}}}).
							WithEnvironmentConfigs(
								utils.AJobComponentEnvironmentConfig().
									WithEnvironment(anyDevEnvironment).
									WithEnvironmentVariable("COMMON1", "dev1").
									WithEnvironmentVariable("COMMON2", "dev2"),
								utils.AJobComponentEnvironmentConfig().
									WithEnvironment(anyProdEnvironment).
									WithEnvironmentVariable("COMMON1", "prod1").
									WithEnvironmentVariable("PROD3", "prod3"),
							),
					)).
			WithAppName(anyApp).
			WithDeploymentName(anyDeploymentName).
			WithEnvironment(anyDevEnvironment).
			WithImageTag(anyImageTag).
			WithLabel(kube.RadixJobNameLabel, anyBuildDeployJobName))
	require.NoError(t, err)

	// Create prod environment without any deployments
	commonTest.CreateEnvNamespace(kubeclient, anyApp, anyProdEnvironment)

	rr, _ := radixclient.RadixV1().RadixRegistrations().Get(context.Background(), anyApp, metav1.GetOptions{})
	ra, _ := radixclient.RadixV1().RadixApplications(utils.GetAppNamespace(anyApp)).Get(context.Background(), anyApp, metav1.GetOptions{})

	cli := promote.NewPromoteStep()
	cli.Init(context.Background(), kubeclient, radixclient, kubeUtil, &monitoring.Clientset{}, nil, rr)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			FromEnvironment: anyDevEnvironment,
			ToEnvironment:   anyProdEnvironment,
			DeploymentName:  anyDeploymentName,
			JobName:         anyPromoteJobName,
			ImageTag:        anyImageTag,
			CommitID:        anyCommitID,
		},
	}

	applicationConfig := application.NewApplicationConfig(kubeclient, kubeUtil, radixclient, rr, ra, nil)
	gitCommitHash := pipelineInfo.GitCommitHash
	gitTags := pipelineInfo.GitTags
	pipelineInfo.SetApplicationConfig(applicationConfig)
	pipelineInfo.SetGitAttributes(gitCommitHash, gitTags)
	err = cli.Run(context.Background(), pipelineInfo)
	require.NoError(t, err)

	rds, _ := radixclient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(anyApp, anyProdEnvironment)).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, 1, len(rds.Items))
	assert.True(t, strings.HasPrefix(rds.Items[0].Name, fmt.Sprintf("%s-%s-", anyProdEnvironment, anyImageTag)))
	assert.Equal(t, anyProdEnvironment, rds.Items[0].Labels[kube.RadixEnvLabel])
	assert.Equal(t, anyPromoteJobName, rds.Items[0].Labels[kube.RadixJobNameLabel])
	assert.Equal(t, 4, *rds.Items[0].Spec.Components[0].Replicas)
	assert.Equal(t, "db-prod", rds.Items[0].Spec.Components[0].EnvironmentVariables["DB_HOST"])
	assert.Equal(t, "5678", rds.Items[0].Spec.Components[0].EnvironmentVariables["DB_PORT"])
	assert.Equal(t, "mysql", rds.Items[0].Spec.Components[0].EnvironmentVariables["DB_TYPE"])
	assert.Equal(t, "my-db-prod", rds.Items[0].Spec.Components[0].EnvironmentVariables["DB_NAME"])
	assert.ElementsMatch(t, []v1.RadixDeployExternalDNS{{FQDN: anyExternalDNSAlias, UseCertificateAutomation: false}}, rds.Items[0].Spec.Components[0].ExternalDNS)
	assert.True(t, rds.Items[0].Spec.Components[0].DNSAppAlias)
	assert.Len(t, rds.Items[0].Spec.Components[0].Secrets, 1)
	assert.Equal(t, "DEPLOYAPPSECRET", rds.Items[0].Spec.Components[0].Secrets[0])
	assert.Len(t, rds.Items[0].Spec.Components[0].SecretRefs.AzureKeyVaults, 1)
	assert.Len(t, rds.Items[0].Spec.Components[0].SecretRefs.AzureKeyVaults[0].Items, 2)
	assert.Equal(t, "TestKeyVault2", rds.Items[0].Spec.Components[0].SecretRefs.AzureKeyVaults[0].Name)
	assert.Equal(t, "Secret2", rds.Items[0].Spec.Components[0].SecretRefs.AzureKeyVaults[0].Items[0].Name)
	assert.Equal(t, "SECRET_2", rds.Items[0].Spec.Components[0].SecretRefs.AzureKeyVaults[0].Items[0].EnvVar)
	assert.Equal(t, "secret", string(*rds.Items[0].Spec.Components[0].SecretRefs.AzureKeyVaults[0].Items[0].Type))
	assert.Equal(t, "Key2", rds.Items[0].Spec.Components[0].SecretRefs.AzureKeyVaults[0].Items[1].Name)
	assert.Equal(t, "KEY_2", rds.Items[0].Spec.Components[0].SecretRefs.AzureKeyVaults[0].Items[1].EnvVar)
	assert.Equal(t, "key", string(*rds.Items[0].Spec.Components[0].SecretRefs.AzureKeyVaults[0].Items[1].Type))
	assert.Equal(t, prodNode, rds.Items[0].Spec.Components[0].Node)

	assert.Equal(t, 1, len(rds.Items[0].Spec.Jobs))
	assert.Equal(t, 3, len(rds.Items[0].Spec.Jobs[0].EnvironmentVariables))
	assert.Equal(t, "prod1", rds.Items[0].Spec.Jobs[0].EnvironmentVariables["COMMON1"])
	assert.Equal(t, "common2", rds.Items[0].Spec.Jobs[0].EnvironmentVariables["COMMON2"])
	assert.Equal(t, "prod3", rds.Items[0].Spec.Jobs[0].EnvironmentVariables["PROD3"])
	assert.Equal(t, numbers.Int32Ptr(8888), rds.Items[0].Spec.Jobs[0].SchedulerPort)
	assert.Equal(t, "/path", rds.Items[0].Spec.Jobs[0].Payload.Path)
	assert.Len(t, rds.Items[0].Spec.Jobs[0].Secrets, 1)
	assert.Equal(t, "DEPLOYJOBSECRET", rds.Items[0].Spec.Jobs[0].Secrets[0])
	assert.Len(t, rds.Items[0].Spec.Jobs[0].SecretRefs.AzureKeyVaults, 1)
	assert.Len(t, rds.Items[0].Spec.Jobs[0].SecretRefs.AzureKeyVaults[0].Items, 2)
	assert.Equal(t, "TestKeyVault", rds.Items[0].Spec.Jobs[0].SecretRefs.AzureKeyVaults[0].Name)
	assert.Equal(t, "Secret1", rds.Items[0].Spec.Jobs[0].SecretRefs.AzureKeyVaults[0].Items[0].Name)
	assert.Equal(t, "SECRET_1", rds.Items[0].Spec.Jobs[0].SecretRefs.AzureKeyVaults[0].Items[0].EnvVar)
	assert.Equal(t, "secret", string(*rds.Items[0].Spec.Jobs[0].SecretRefs.AzureKeyVaults[0].Items[0].Type))
	assert.Equal(t, "Key1", rds.Items[0].Spec.Jobs[0].SecretRefs.AzureKeyVaults[0].Items[1].Name)
	assert.Equal(t, "KEY_1", rds.Items[0].Spec.Jobs[0].SecretRefs.AzureKeyVaults[0].Items[1].EnvVar)
	assert.Equal(t, "key", string(*rds.Items[0].Spec.Jobs[0].SecretRefs.AzureKeyVaults[0].Items[1].Type))
}

func TestPromote_PromoteToOtherEnvironment_Resources_NoOverride(t *testing.T) {
	anyApp := "any-app"
	anyDeploymentName := "deployment-1"
	anyImageTag := "abcdef"
	anyBuildDeployJobName := "any-build-deploy-job"
	anyPromoteJobName := "any-promote-job"
	anyProdEnvironment := "prod"
	anyDevEnvironment := "dev"

	// Setup
	kubeclient, kubeUtil, radixclient, commonTestUtils := setupTest(t)

	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		utils.ARadixDeployment().
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
							WithCommonResource(map[string]string{
								"memory": "64Mi",
								"cpu":    "250m",
							}, map[string]string{
								"memory": "128Mi",
								"cpu":    "500m",
							})).
					WithJobComponents(
						utils.AnApplicationJobComponent().
							WithName("job").
							WithSchedulerPort(numbers.Int32Ptr(8888)).
							WithCommonResource(map[string]string{
								"memory": "11Mi",
								"cpu":    "22m",
							}, map[string]string{
								"memory": "33Mi",
								"cpu":    "44m",
							}),
					)).
			WithAppName(anyApp).
			WithDeploymentName(anyDeploymentName).
			WithEnvironment(anyDevEnvironment).
			WithImageTag(anyImageTag).
			WithLabel(kube.RadixJobNameLabel, anyBuildDeployJobName))
	require.NoError(t, err)

	// Create prod environment without any deployments
	commonTest.CreateEnvNamespace(kubeclient, anyApp, anyProdEnvironment)

	rr, _ := radixclient.RadixV1().RadixRegistrations().Get(context.Background(), anyApp, metav1.GetOptions{})
	ra, _ := radixclient.RadixV1().RadixApplications(utils.GetAppNamespace(anyApp)).Get(context.Background(), anyApp, metav1.GetOptions{})

	cli := promote.NewPromoteStep()
	cli.Init(context.Background(), kubeclient, radixclient, kubeUtil, &monitoring.Clientset{}, nil, rr)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			FromEnvironment: anyDevEnvironment,
			ToEnvironment:   anyProdEnvironment,
			DeploymentName:  anyDeploymentName,
			JobName:         anyPromoteJobName,
			ImageTag:        anyImageTag,
			CommitID:        anyCommitID,
		},
	}

	applicationConfig := application.NewApplicationConfig(kubeclient, kubeUtil, radixclient, rr, ra, nil)
	gitCommitHash := pipelineInfo.GitCommitHash
	gitTags := pipelineInfo.GitTags
	pipelineInfo.SetApplicationConfig(applicationConfig)
	pipelineInfo.SetGitAttributes(gitCommitHash, gitTags)
	err = cli.Run(context.Background(), pipelineInfo)
	require.NoError(t, err)

	rds, _ := radixclient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(anyApp, anyProdEnvironment)).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, 1, len(rds.Items))
	assert.True(t, strings.HasPrefix(rds.Items[0].Name, fmt.Sprintf("%s-%s-", anyProdEnvironment, anyImageTag)))
	assert.Equal(t, anyProdEnvironment, rds.Items[0].Labels[kube.RadixEnvLabel])
	assert.Equal(t, anyPromoteJobName, rds.Items[0].Labels[kube.RadixJobNameLabel])
	assert.Equal(t, "250m", rds.Items[0].Spec.Components[0].Resources.Requests["cpu"])
	assert.Equal(t, "64Mi", rds.Items[0].Spec.Components[0].Resources.Requests["memory"])
	assert.Equal(t, "500m", rds.Items[0].Spec.Components[0].Resources.Limits["cpu"])
	assert.Equal(t, "128Mi", rds.Items[0].Spec.Components[0].Resources.Limits["memory"])
	assert.Equal(t, "22m", rds.Items[0].Spec.Jobs[0].Resources.Requests["cpu"])
	assert.Equal(t, "11Mi", rds.Items[0].Spec.Jobs[0].Resources.Requests["memory"])
	assert.Equal(t, "44m", rds.Items[0].Spec.Jobs[0].Resources.Limits["cpu"])
	assert.Equal(t, "33Mi", rds.Items[0].Spec.Jobs[0].Resources.Limits["memory"])
}

func TestPromote_PromoteToOtherEnvironment_Authentication(t *testing.T) {
	anyApp := "any-app"
	anyDeploymentName := "deployment-1"
	anyImageTag := "abcdef"
	anyBuildDeployJobName := "any-build-deploy-job"
	anyPromoteJobName := "any-promote-job"
	anyProdEnvironment := "prod"
	anyDevEnvironment := "dev"

	// Setup
	kubeclient, kubeUtil, radixclient, commonTestUtils := setupTest(t)

	verification := v1.VerificationTypeOptional
	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		utils.NewDeploymentBuilder().
			WithAppName(anyApp).
			WithDeploymentName(anyDeploymentName).
			WithEnvironment(anyDevEnvironment).
			WithComponents(
				utils.NewDeployComponentBuilder().WithName("app").WithAuthentication(nil),
			).
			WithLabel(kube.RadixJobNameLabel, anyBuildDeployJobName).
			WithRadixApplication(
				utils.NewRadixApplicationBuilder().
					WithRadixRegistration(utils.ARadixRegistration().WithName(anyApp)).
					WithAppName(anyApp).
					WithEnvironment(anyProdEnvironment, "").
					WithComponents(
						utils.AnApplicationComponent().
							WithName("app").
							WithAuthentication(
								&v1.Authentication{
									ClientCertificate: &v1.ClientCertificate{
										PassCertificateToUpstream: pointers.Ptr(true),
									},
								},
							).
							WithEnvironmentConfigs(
								utils.NewComponentEnvironmentBuilder().WithEnvironment(anyProdEnvironment).WithAuthentication(
									&v1.Authentication{
										ClientCertificate: &v1.ClientCertificate{
											Verification: &verification,
										},
									},
								),
							),
					)))
	require.NoError(t, err)

	// Create environments
	commonTest.CreateEnvNamespace(kubeclient, anyApp, anyProdEnvironment)
	commonTest.CreateEnvNamespace(kubeclient, anyApp, anyDevEnvironment)

	rr, _ := radixclient.RadixV1().RadixRegistrations().Get(context.Background(), anyApp, metav1.GetOptions{})
	ra, _ := radixclient.RadixV1().RadixApplications(utils.GetAppNamespace(anyApp)).Get(context.Background(), anyApp, metav1.GetOptions{})

	cli := promote.NewPromoteStep()
	cli.Init(context.Background(), kubeclient, radixclient, kubeUtil, &monitoring.Clientset{}, nil, rr)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			FromEnvironment: anyDevEnvironment,
			ToEnvironment:   anyProdEnvironment,
			DeploymentName:  anyDeploymentName,
			JobName:         anyPromoteJobName,
			ImageTag:        anyImageTag,
			CommitID:        anyCommitID,
		},
	}

	applicationConfig := application.NewApplicationConfig(kubeclient, kubeUtil, radixclient, rr, ra, nil)
	gitCommitHash := pipelineInfo.GitCommitHash
	gitTags := pipelineInfo.GitTags
	pipelineInfo.SetApplicationConfig(applicationConfig)
	pipelineInfo.SetGitAttributes(gitCommitHash, gitTags)
	err = cli.Run(context.Background(), pipelineInfo)
	require.NoError(t, err)

	rds, _ := radixclient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(anyApp, anyProdEnvironment)).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, 1, len(rds.Items))

	x0 := &v1.Authentication{
		ClientCertificate: &v1.ClientCertificate{
			Verification:              &verification,
			PassCertificateToUpstream: pointers.Ptr(true),
		},
	}
	assert.NotNil(t, rds.Items[0].Spec.Components[0].Authentication)
	assert.NotNil(t, rds.Items[0].Spec.Components[0].Authentication.ClientCertificate)
	assert.Equal(t, x0, rds.Items[0].Spec.Components[0].Authentication)
}

func TestPromote_PromoteToOtherEnvironment_Resources_WithOverride(t *testing.T) {
	anyApp := "any-app"
	anyDeploymentName := "deployment-1"
	anyImageTag := "abcdef"
	anyBuildDeployJobName := "any-build-deploy-job"
	anyPromoteJobName := "any-promote-job"
	anyProdEnvironment := "prod"
	anyDevEnvironment := "dev"

	// Setup
	kubeclient, kubeUtil, radixclient, commonTestUtils := setupTest(t)

	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		utils.ARadixDeployment().
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
							WithCommonResource(map[string]string{
								"memory": "64Mi",
								"cpu":    "250m",
							}, map[string]string{
								"memory": "128Mi",
								"cpu":    "500m",
							}).
							WithEnvironmentConfigs(
								utils.AnEnvironmentConfig().
									WithEnvironment(anyProdEnvironment).
									WithResource(
										map[string]string{
											"memory": "128Mi",
											"cpu":    "500m",
										}, map[string]string{
											"memory": "256Mi",
											"cpu":    "750m",
										}))).
					WithJobComponents(
						utils.AnApplicationJobComponent().
							WithName("job").
							WithSchedulerPort(numbers.Int32Ptr(8888)).
							WithCommonResource(
								map[string]string{
									"memory": "11Mi",
									"cpu":    "22m",
								}, map[string]string{
									"memory": "33Mi",
									"cpu":    "44m",
								}).
							WithEnvironmentConfigs(
								utils.AJobComponentEnvironmentConfig().
									WithEnvironment(anyProdEnvironment).
									WithResource(
										map[string]string{
											"memory": "111Mi",
											"cpu":    "222m",
										}, map[string]string{
											"memory": "333Mi",
											"cpu":    "444m",
										})),
					)).
			WithAppName(anyApp).
			WithDeploymentName(anyDeploymentName).
			WithEnvironment(anyDevEnvironment).
			WithImageTag(anyImageTag).
			WithLabel(kube.RadixJobNameLabel, anyBuildDeployJobName))
	require.NoError(t, err)

	// Create prod environment without any deployments
	commonTest.CreateEnvNamespace(kubeclient, anyApp, anyProdEnvironment)

	rr, _ := radixclient.RadixV1().RadixRegistrations().Get(context.Background(), anyApp, metav1.GetOptions{})
	ra, _ := radixclient.RadixV1().RadixApplications(utils.GetAppNamespace(anyApp)).Get(context.Background(), anyApp, metav1.GetOptions{})

	cli := promote.NewPromoteStep()
	cli.Init(context.Background(), kubeclient, radixclient, kubeUtil, &monitoring.Clientset{}, nil, rr)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			FromEnvironment: anyDevEnvironment,
			ToEnvironment:   anyProdEnvironment,
			DeploymentName:  anyDeploymentName,
			JobName:         anyPromoteJobName,
			ImageTag:        anyImageTag,
			CommitID:        anyCommitID,
		},
	}

	applicationConfig := application.NewApplicationConfig(kubeclient, kubeUtil, radixclient, rr, ra, nil)
	gitCommitHash := pipelineInfo.GitCommitHash
	gitTags := pipelineInfo.GitTags
	pipelineInfo.SetApplicationConfig(applicationConfig)
	pipelineInfo.SetGitAttributes(gitCommitHash, gitTags)
	err = cli.Run(context.Background(), pipelineInfo)
	require.NoError(t, err)

	rds, _ := radixclient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(anyApp, anyProdEnvironment)).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, 1, len(rds.Items))
	assert.True(t, strings.HasPrefix(rds.Items[0].Name, fmt.Sprintf("%s-%s-", anyProdEnvironment, anyImageTag)))
	assert.Equal(t, anyProdEnvironment, rds.Items[0].Labels[kube.RadixEnvLabel])
	assert.Equal(t, anyPromoteJobName, rds.Items[0].Labels[kube.RadixJobNameLabel])
	assert.Equal(t, "500m", rds.Items[0].Spec.Components[0].Resources.Requests["cpu"])
	assert.Equal(t, "128Mi", rds.Items[0].Spec.Components[0].Resources.Requests["memory"])
	assert.Equal(t, "750m", rds.Items[0].Spec.Components[0].Resources.Limits["cpu"])
	assert.Equal(t, "256Mi", rds.Items[0].Spec.Components[0].Resources.Limits["memory"])
	assert.Equal(t, "222m", rds.Items[0].Spec.Jobs[0].Resources.Requests["cpu"])
	assert.Equal(t, "111Mi", rds.Items[0].Spec.Jobs[0].Resources.Requests["memory"])
	assert.Equal(t, "444m", rds.Items[0].Spec.Jobs[0].Resources.Limits["cpu"])
	assert.Equal(t, "333Mi", rds.Items[0].Spec.Jobs[0].Resources.Limits["memory"])
}

func TestPromote_PromoteToSameEnvironment_NewStateIsExpected(t *testing.T) {
	anyApp := "any-app"
	anyDeploymentName := "deployment-1"
	anyImageTag := "abcdef"
	anyBuildDeployJobName := "any-build-deploy-job"
	anyPromoteJobName := "any-promote-job"
	anyDevEnvironment := "dev"

	// Setup
	kubeclient, kubeUtil, radixclient, commonTestUtils := setupTest(t)

	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		utils.ARadixDeployment().
			WithAppName(anyApp).
			WithDeploymentName(anyDeploymentName).
			WithEnvironment(anyDevEnvironment).
			WithImageTag(anyImageTag).
			WithLabel(kube.RadixJobNameLabel, anyBuildDeployJobName))
	require.NoError(t, err)

	rr, _ := radixclient.RadixV1().RadixRegistrations().Get(context.Background(), anyApp, metav1.GetOptions{})
	ra, _ := radixclient.RadixV1().RadixApplications(utils.GetAppNamespace(anyApp)).Get(context.Background(), anyApp, metav1.GetOptions{})

	cli := promote.NewPromoteStep()
	cli.Init(context.Background(), kubeclient, radixclient, kubeUtil, &monitoring.Clientset{}, nil, rr)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			FromEnvironment: anyDevEnvironment,
			ToEnvironment:   anyDevEnvironment,
			DeploymentName:  anyDeploymentName,
			JobName:         anyPromoteJobName,
			ImageTag:        anyImageTag,
			CommitID:        anyCommitID,
		},
	}

	applicationConfig := application.NewApplicationConfig(kubeclient, kubeUtil, radixclient, rr, ra, nil)
	gitCommitHash := pipelineInfo.GitCommitHash
	gitTags := pipelineInfo.GitTags
	pipelineInfo.SetApplicationConfig(applicationConfig)
	pipelineInfo.SetGitAttributes(gitCommitHash, gitTags)
	err = cli.Run(context.Background(), pipelineInfo)
	require.NoError(t, err)

	rds, _ := radixclient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(anyApp, anyDevEnvironment)).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, 2, len(rds.Items))
}

func TestPromote_PromoteToOtherEnvironment_Identity(t *testing.T) {
	anyApp := "any-app"
	anyDeploymentName := "deployment-1"
	anyImageTag := "abcdef"
	anyBuildDeployJobName := "any-build-deploy-job"
	anyPromoteJobName := "any-promote-job"
	anyProdEnvironment := "prod"
	anyDevEnvironment := "dev"
	currentRdIdentity := &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "any-current-identity-123"}}

	type scenarioSpec struct {
		name                 string
		commonConfig         *v1.Identity
		configureEnvironment bool
		environmentConfig    *v1.Identity
		expected             *v1.Identity
	}

	scenarios := []scenarioSpec{
		{name: "nil when commonConfig and environmentConfig is empty", commonConfig: &v1.Identity{}, configureEnvironment: true, environmentConfig: &v1.Identity{}, expected: nil},
		{name: "nil when commonConfig is nil and environmentConfig is empty", commonConfig: nil, configureEnvironment: true, environmentConfig: &v1.Identity{}, expected: nil},
		{name: "nil when commonConfig is empty and environmentConfig is nil", commonConfig: &v1.Identity{}, configureEnvironment: true, environmentConfig: nil, expected: nil},
		{name: "nil when commonConfig is nil and environmentConfig is not set", commonConfig: nil, configureEnvironment: false, environmentConfig: nil, expected: nil},
		{name: "nil when commonConfig is empty and environmentConfig is not set", commonConfig: &v1.Identity{}, configureEnvironment: false, environmentConfig: nil, expected: nil},
		{name: "use commonConfig when environmentConfig is empty", commonConfig: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}, configureEnvironment: true, environmentConfig: &v1.Identity{}, expected: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}},
		{name: "use commonConfig when environmentConfig.Azure is empty", commonConfig: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}, configureEnvironment: true, environmentConfig: &v1.Identity{Azure: &v1.AzureIdentity{}}, expected: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}},
		{name: "override non-empty commonConfig with environmentConfig.Azure", commonConfig: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}, configureEnvironment: true, environmentConfig: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}, expected: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}},
		{name: "override empty commonConfig with environmentConfig", commonConfig: &v1.Identity{}, configureEnvironment: true, environmentConfig: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}, expected: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}},
		{name: "override empty commonConfig.Azure with environmentConfig", commonConfig: &v1.Identity{Azure: &v1.AzureIdentity{}}, configureEnvironment: true, environmentConfig: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}, expected: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}},
		{name: "transform clientId with curly to standard format", commonConfig: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "{11111111-2222-3333-4444-555555555555}"}}, configureEnvironment: false, environmentConfig: nil, expected: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}},
		{name: "transform clientId with urn:uuid to standard format", commonConfig: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "urn:uuid:11111111-2222-3333-4444-555555555555"}}, configureEnvironment: false, environmentConfig: nil, expected: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}},
		{name: "transform clientId without dashes to standard format", commonConfig: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "11111111222233334444555555555555"}}, configureEnvironment: false, environmentConfig: nil, expected: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			kubeclient, kubeUtil, radixclient, commonTestUtils := setupTest(t)
			var componentEnvironmentConfigs []utils.RadixEnvironmentConfigBuilder
			var jobEnvironmentConfigs []utils.RadixJobComponentEnvironmentConfigBuilder

			if scenario.configureEnvironment {
				componentEnvironmentConfigs = append(componentEnvironmentConfigs, utils.AnEnvironmentConfig().WithEnvironment(anyProdEnvironment).WithIdentity(scenario.environmentConfig))
				jobEnvironmentConfigs = append(jobEnvironmentConfigs, utils.AJobComponentEnvironmentConfig().WithEnvironment(anyProdEnvironment).WithIdentity(scenario.environmentConfig))
			}

			_, err := commonTestUtils.ApplyDeployment(
				context.Background(),
				utils.NewDeploymentBuilder().
					WithComponents(
						utils.NewDeployComponentBuilder().
							WithName("comp1").
							WithIdentity(currentRdIdentity),
					).
					WithJobComponents(
						utils.NewDeployJobComponentBuilder().
							WithName("job1").
							WithIdentity(currentRdIdentity),
					).
					WithAppName(anyApp).
					WithDeploymentName(anyDeploymentName).
					WithEnvironment(anyDevEnvironment).
					WithImageTag(anyImageTag).
					WithLabel(kube.RadixJobNameLabel, anyBuildDeployJobName).
					WithRadixApplication(
						utils.NewRadixApplicationBuilder().
							WithRadixRegistration(
								utils.ARadixRegistration().
									WithName(anyApp)).
							WithAppName(anyApp).
							WithEnvironment(anyDevEnvironment, "").
							WithEnvironment(anyProdEnvironment, "").
							WithComponents(
								utils.AnApplicationComponent().
									WithName("comp1").
									WithIdentity(scenario.commonConfig).
									WithEnvironmentConfigs(componentEnvironmentConfigs...)).
							WithJobComponents(
								utils.AnApplicationJobComponent().
									WithName("job1").
									WithIdentity(scenario.commonConfig).
									WithSchedulerPort(numbers.Int32Ptr(8888)).
									WithPayloadPath(utils.StringPtr("/path")).
									WithEnvironmentConfigs(jobEnvironmentConfigs...),
							)),
			)
			require.NoError(t, err)

			// Create prod environment without any deployments
			commonTest.CreateEnvNamespace(kubeclient, anyApp, anyProdEnvironment)

			rr, _ := radixclient.RadixV1().RadixRegistrations().Get(context.Background(), anyApp, metav1.GetOptions{})
			ra, _ := radixclient.RadixV1().RadixApplications(utils.GetAppNamespace(anyApp)).Get(context.Background(), anyApp, metav1.GetOptions{})

			cli := promote.NewPromoteStep()
			cli.Init(context.Background(), kubeclient, radixclient, kubeUtil, &monitoring.Clientset{}, nil, rr)

			pipelineInfo := &model.PipelineInfo{
				PipelineArguments: model.PipelineArguments{
					FromEnvironment: anyDevEnvironment,
					ToEnvironment:   anyProdEnvironment,
					DeploymentName:  anyDeploymentName,
					JobName:         anyPromoteJobName,
					ImageTag:        anyImageTag,
					CommitID:        anyCommitID,
				},
			}

			applicationConfig := application.NewApplicationConfig(kubeclient, kubeUtil, radixclient, rr, ra, nil)
			pipelineInfo.SetApplicationConfig(applicationConfig)
			err = cli.Run(context.Background(), pipelineInfo)
			require.NoError(t, err)

			rds, _ := radixclient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(anyApp, anyProdEnvironment)).List(context.Background(), metav1.ListOptions{})
			require.Equal(t, 1, len(rds.Items))
			assert.Equal(t, scenario.expected, rds.Items[0].Spec.Components[0].Identity)
			assert.Equal(t, scenario.expected, rds.Items[0].Spec.Jobs[0].Identity)
		})

	}
}

func TestPromote_AnnotatedBySourceDeploymentAttributes(t *testing.T) {
	srcDeploymentName := "deployment-1"
	anyImageTag := "abcdef"
	anyPromoteJobName := "any-promote-job"
	dstEnv := "test"
	srcEnv := "dev"
	srcDeploymentCommitID := "222ca8595c5283a9d0f17a623b9255a0d9866a2e"
	srcRadixConfigHash := "sha256-a5d1565b32252be05910e459eb7551fd0fd6e0d513f7728c54ca5507c9b11387"

	// Setup
	kubeclient, kubeUtil, radixclient, commonTestUtils := setupTest(t)

	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		utils.NewDeploymentBuilder().
			WithRadixApplication(
				utils.NewRadixApplicationBuilder().
					WithRadixRegistration(utils.ARadixRegistration()).
					WithAppName(anyAppName).
					WithEnvironment(srcEnv, "dev-branch").
					WithEnvironment(dstEnv, "test-branch").
					WithComponent(utils.NewApplicationComponentBuilder().WithName("comp1"))).
			WithAppName(anyAppName).
			WithDeploymentName(srcDeploymentName).
			WithEnvironment(srcEnv).
			WithImageTag(anyImageTag).
			WithLabel(kube.RadixCommitLabel, srcDeploymentCommitID).
			WithAnnotations(map[string]string{kube.RadixConfigHash: srcRadixConfigHash}))

	require.NoError(t, err)

	rr, _ := radixclient.RadixV1().RadixRegistrations().Get(context.Background(), anyAppName, metav1.GetOptions{})
	ra, _ := radixclient.RadixV1().RadixApplications(utils.GetAppNamespace(anyAppName)).Get(context.Background(), anyAppName, metav1.GetOptions{})

	cli := promote.NewPromoteStep()
	cli.Init(context.Background(), kubeclient, radixclient, kubeUtil, &monitoring.Clientset{}, nil, rr)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			FromEnvironment: srcEnv,
			ToEnvironment:   dstEnv,
			DeploymentName:  srcDeploymentName,
			JobName:         anyPromoteJobName,
			ImageTag:        anyImageTag,
			CommitID:        anyCommitID,
		},
	}

	applicationConfig := application.NewApplicationConfig(kubeclient, kubeUtil, radixclient, rr, ra, nil)
	gitCommitHash := pipelineInfo.GitCommitHash
	gitTags := pipelineInfo.GitTags
	pipelineInfo.SetApplicationConfig(applicationConfig)
	pipelineInfo.SetGitAttributes(gitCommitHash, gitTags)
	err = cli.Run(context.Background(), pipelineInfo)
	require.NoError(t, err)

	rds, err := radixclient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(anyAppName, dstEnv)).List(context.Background(), metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, rds.Items, 1)
	promotedRD := rds.Items[0]
	assert.Equal(t, srcEnv, promotedRD.GetAnnotations()[kube.RadixDeploymentPromotedFromEnvironmentAnnotation])
	assert.Equal(t, srcDeploymentName, promotedRD.GetAnnotations()[kube.RadixDeploymentPromotedFromDeploymentAnnotation])
	assert.Equal(t, srcRadixConfigHash, promotedRD.GetAnnotations()[kube.RadixConfigHash])
	assert.Equal(t, srcDeploymentCommitID, promotedRD.GetLabels()[kube.RadixCommitLabel])
}

func TestPromote_Runtime_KeepFromSourceRD(t *testing.T) {
	var (
		appName         string = "anyapp"
		comp1, comp2    string = "comp1", "comp2"
		job1, job2      string = "job1", "job2"
		envDev, envProd string = "dev", "prod"
		rdSource        string = "rdsource"
		schedulerPort   int32  = 9999
	)

	// Setup
	kubeclient, kubeUtil, radixclient, commonTestUtils := setupTest(t)
	rr := utils.NewRegistrationBuilder().WithName(appName)
	ra := utils.NewRadixApplicationBuilder().WithRadixRegistration(rr).WithAppName(appName).
		WithEnvironmentNoBranch(envDev).WithEnvironmentNoBranch(envProd).
		WithComponents(
			utils.NewApplicationComponentBuilder().WithName(comp1).WithRuntime(&v1.Runtime{Architecture: "commonarch"}),
			utils.NewApplicationComponentBuilder().WithName(comp2).WithRuntime(&v1.Runtime{Architecture: "commonarch"}).WithEnvironmentConfig(utils.NewComponentEnvironmentBuilder().WithEnvironment(envProd).WithRuntime(&v1.Runtime{Architecture: "prodarch"})),
		).
		WithJobComponents(
			utils.NewApplicationJobComponentBuilder().WithName(job1).WithSchedulerPort(&schedulerPort).WithRuntime(&v1.Runtime{Architecture: "commonarch"}),
			utils.NewApplicationJobComponentBuilder().WithName(job2).WithSchedulerPort(&schedulerPort).WithRuntime(&v1.Runtime{Architecture: "commonarch"}).WithEnvironmentConfig(utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envProd).WithRuntime(&v1.Runtime{Architecture: "prodarch"})),
		)

	_, err := commonTestUtils.ApplyApplication(ra)
	require.NoError(t, err)
	commonTest.CreateEnvNamespace(kubeclient, appName, envDev)
	commonTest.CreateEnvNamespace(kubeclient, appName, envProd)

	rd := utils.NewDeploymentBuilder().
		WithDeploymentName(rdSource).
		WithAppName(appName).
		WithEnvironment(envDev).
		WithComponents(
			utils.NewDeployComponentBuilder().WithName(comp1).WithRuntime(nil),
			utils.NewDeployComponentBuilder().WithName(comp2).WithRuntime(&v1.Runtime{Architecture: "comp2arch"}),
		).
		WithJobComponents(
			utils.NewDeployJobComponentBuilder().WithName(job1).WithRuntime(nil),
			utils.NewDeployJobComponentBuilder().WithName(job2).WithRuntime(&v1.Runtime{Architecture: "job2arch"}),
		)

	_, err = commonTestUtils.ApplyDeployment(context.Background(), rd)
	require.NoError(t, err)

	cli := promote.NewPromoteStep()
	cli.Init(context.Background(), kubeclient, radixclient, kubeUtil, &monitoring.Clientset{}, nil, rr.BuildRR())

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			DeploymentName:  rdSource,
			FromEnvironment: envDev,
			ToEnvironment:   envProd,
			JobName:         "ajob",
			ImageTag:        "atag",
		},
	}

	err = cli.Run(context.Background(), pipelineInfo)
	require.NoError(t, err)

	rdListProd, err := radixclient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(appName, envProd)).List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, rdListProd.Items, 1)
	rdProd := rdListProd.Items[0]
	comp1Prod, found := commonslice.FindFirst(rdProd.Spec.Components, func(c v1.RadixDeployComponent) bool { return c.Name == comp1 })
	require.True(t, found)
	comp2Prod, found := commonslice.FindFirst(rdProd.Spec.Components, func(c v1.RadixDeployComponent) bool { return c.Name == comp2 })
	require.True(t, found)
	job1Prod, found := commonslice.FindFirst(rdProd.Spec.Jobs, func(c v1.RadixDeployJobComponent) bool { return c.Name == job1 })
	require.True(t, found)
	job2Prod, found := commonslice.FindFirst(rdProd.Spec.Jobs, func(c v1.RadixDeployJobComponent) bool { return c.Name == job2 })
	require.True(t, found)
	assert.Nil(t, comp1Prod.Runtime, "%s should use Runtime from source RD", comp1Prod.Name)
	assert.Equal(t, &v1.Runtime{Architecture: "comp2arch"}, comp2Prod.Runtime, "%s should use Runtime from source RD", comp2Prod.Name)
	assert.Nil(t, job1Prod.Runtime, "%s should use Runtime from source RD", job1Prod.Name)
	assert.Equal(t, &v1.Runtime{Architecture: "job2arch"}, job2Prod.Runtime, "%s should use Runtime from source RD", job2Prod.Name)

}

package steps_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commonTest "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

func Test_RunPromoteTestSuite(t *testing.T) {
	suite.Run(t, new(promoteTestSuite))
}

type promoteTestSuite struct {
	suite.Suite
	kubeClient  *kubefake.Clientset
	radixClient *radixfake.Clientset
	promClient  *prometheusfake.Clientset
	kubeUtil    *kube.Kube
	testUtils   commonTest.Utils
	ctrl        *gomock.Controller
}

func (s *promoteTestSuite) SetupTest() {
	s.setupTest()
}

func (s *promoteTestSuite) SetupSubTest() {
	s.setupTest()
}

func (s *promoteTestSuite) setupTest() {
	s.kubeClient = kubefake.NewSimpleClientset()
	s.radixClient = radixfake.NewSimpleClientset()
	s.promClient = prometheusfake.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, nil)
	s.ctrl = gomock.NewController(s.T())
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	s.testUtils = commonTest.NewTestUtils(s.kubeClient, s.radixClient, secretproviderclient)
}

func (s *promoteTestSuite) TestPromote_ErrorScenarios_ErrorIsReturned() {
	app1 := "any-app-1"
	app2 := "any-app-2"
	app4 := "any-app-4"
	app5 := "any-app-5"
	app1Deployment := "1"
	app2Deployment := "2"
	app3Deployment := "3"
	app4Deployment := "4"
	app5Deployment := "5"
	prodEnvName := "prod"
	devEnvName := "dev"
	qaEnvName := "qa"
	imageTag := "abcdef"
	jobName := "radix-pipeline-abcdef"
	nonExistingComponent := "non-existing-comp"
	nonExistingJobComponent := "non-existing-job"

	initTestResources := func() {
		_, err := s.testUtils.ApplyDeployment(utils.
			ARadixDeployment().
			WithDeploymentName(app1Deployment).
			WithAppName(app1).
			WithEnvironment(prodEnvName).
			WithImageTag(imageTag))
		s.Require().NoError(err)

		_, err = s.testUtils.ApplyDeployment(utils.
			ARadixDeployment().
			WithDeploymentName(app2Deployment).
			WithAppName(app1).
			WithEnvironment(devEnvName).
			WithImageTag(imageTag))
		s.Require().NoError(err)

		_, err = s.testUtils.ApplyDeployment(utils.
			ARadixDeployment().
			WithDeploymentName(app3Deployment).
			WithAppName(app2).
			WithEnvironment(devEnvName).
			WithImageTag(imageTag))
		s.Require().NoError(err)

		_, err = s.testUtils.ApplyDeployment(utils.
			ARadixDeployment().
			WithDeploymentName(app4Deployment).
			WithAppName(app4).
			WithEnvironment(devEnvName).
			WithImageTag(imageTag).
			WithComponent(utils.
				NewDeployComponentBuilder().
				WithName(nonExistingComponent)))
		s.Require().NoError(err)

		_, err = s.testUtils.ApplyDeployment(utils.
			ARadixDeployment().
			WithDeploymentName(app5Deployment).
			WithAppName(app5).
			WithEnvironment(devEnvName).
			WithImageTag(imageTag).
			WithJobComponent(utils.
				NewDeployJobComponentBuilder().
				WithName(nonExistingJobComponent)))
		s.Require().NoError(err)

		commonTest.CreateEnvNamespace(s.kubeClient, app2, prodEnvName)
	}

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
		{"empty from environment", app1, "", imageTag, jobName, prodEnvName, app2Deployment, steps.EmptyArgument("From environment")},
		{"empty to environment", app1, devEnvName, imageTag, jobName, "", app2Deployment, steps.EmptyArgument("To environment")},
		{"empty image tag", app1, devEnvName, "", jobName, prodEnvName, app2Deployment, steps.EmptyArgument("Image tag")},
		{"empty job name", app1, devEnvName, imageTag, "", prodEnvName, app2Deployment, steps.EmptyArgument("Job name")},
		{"empty deployment name", app1, devEnvName, imageTag, jobName, prodEnvName, "", steps.EmptyArgument("Deployment name")},
		{"promote from non-existing environment", app1, qaEnvName, imageTag, jobName, prodEnvName, app2Deployment, steps.NonExistingFromEnvironment(qaEnvName)},
		{"promote to non-existing environment", app1, devEnvName, imageTag, jobName, qaEnvName, app2Deployment, steps.NonExistingToEnvironment(qaEnvName)},
		{"promote non-existing deployment", app2, devEnvName, "nopqrst", jobName, prodEnvName, "non-existing", steps.NonExistingDeployment("non-existing")},
		{"promote deployment with non-existing component", app4, devEnvName, imageTag, jobName, devEnvName, app4Deployment, steps.NonExistingComponentName(app4, nonExistingComponent)},
		{"promote deployment with non-existing job component", app5, devEnvName, imageTag, jobName, devEnvName, app5Deployment, steps.NonExistingComponentName(app5, nonExistingJobComponent)},
	}

	for _, scenario := range testScenarios {
		s.Run(scenario.name, func() {
			initTestResources()
			rr, _ := s.radixClient.RadixV1().RadixRegistrations().Get(context.TODO(), scenario.appName, metav1.GetOptions{})

			cli := steps.NewPromoteStep()
			cli.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)

			pipelineInfo := &model.PipelineInfo{
				PipelineArguments: model.PipelineArguments{
					FromEnvironment: scenario.fromEnvironment,
					ToEnvironment:   scenario.toEnvironment,
					DeploymentName:  scenario.deploymentName,
					JobName:         scenario.jobName,
					ImageTag:        scenario.imageTag,
					CommitID:        "anyCommitID",
				},
			}

			err := cli.Run(pipelineInfo)
			if scenario.expectedError != nil {
				s.Require().Error(err)
				s.Equal(scenario.expectedError.Error(), err.Error())
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *promoteTestSuite) TestPromote_PromoteToOtherEnvironment_NewStateIsExpected() {
	appName := "any-app"
	deploymentName := "deployment-1"
	imageTag := "abcdef"
	buildDeployJobName := "any-build-deploy-job"
	promoteJobName := "any-promote-job"
	provEnvName := "prod"
	qaEnvName := "dev"
	dnsAlias := "a-dns-alias"
	prodNode := radixv1.RadixNode{Gpu: "prod-gpu", GpuCount: "2"}

	secretType := radixv1.RadixAzureKeyVaultObjectTypeSecret
	keyType := radixv1.RadixAzureKeyVaultObjectTypeKey
	_, err := s.testUtils.ApplyDeployment(
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
							WithName(appName)).
					WithAppName(appName).
					WithEnvironment(qaEnvName, "master").
					WithEnvironment(provEnvName, "").
					WithDNSAppAlias(provEnvName, "app").
					WithDNSExternalAlias(dnsAlias, provEnvName, "app").
					WithComponents(
						utils.AnApplicationComponent().
							WithName("app").
							WithSecrets("APPSECRET1", "APPSECRET2").
							WithCommonEnvironmentVariable("DB_TYPE", "mysql").
							WithCommonEnvironmentVariable("DB_NAME", "my-db").
							WithSecretRefs(radixv1.RadixSecretRefs{AzureKeyVaults: []radixv1.RadixAzureKeyVault{{
								Name: "TestKeyVault2",
								Items: []radixv1.RadixAzureKeyVaultItem{
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
									WithEnvironment(qaEnvName).
									WithReplicas(pointers.Ptr(2)).
									WithEnvironmentVariable("DB_HOST", "db-dev").
									WithEnvironmentVariable("DB_PORT", "1234"),
								utils.AnEnvironmentConfig().
									WithEnvironment(provEnvName).
									WithReplicas(pointers.Ptr(4)).
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
							WithSecretRefs(radixv1.RadixSecretRefs{AzureKeyVaults: []radixv1.RadixAzureKeyVault{{
								Name: "TestKeyVault",
								Items: []radixv1.RadixAzureKeyVaultItem{
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
									WithEnvironment(qaEnvName).
									WithEnvironmentVariable("COMMON1", "dev1").
									WithEnvironmentVariable("COMMON2", "dev2"),
								utils.AJobComponentEnvironmentConfig().
									WithEnvironment(provEnvName).
									WithEnvironmentVariable("COMMON1", "prod1").
									WithEnvironmentVariable("PROD3", "prod3"),
							),
					)).
			WithAppName(appName).
			WithDeploymentName(deploymentName).
			WithEnvironment(qaEnvName).
			WithImageTag(imageTag).
			WithLabel(kube.RadixJobNameLabel, buildDeployJobName))
	s.Require().NoError(err)

	// Create prod environment without any deployments
	commonTest.CreateEnvNamespace(s.kubeClient, appName, provEnvName)

	rr, _ := s.radixClient.RadixV1().RadixRegistrations().Get(context.TODO(), appName, metav1.GetOptions{})
	ra, _ := s.radixClient.RadixV1().RadixApplications(utils.GetAppNamespace(appName)).Get(context.TODO(), appName, metav1.GetOptions{})

	cli := steps.NewPromoteStep()
	cli.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			FromEnvironment: qaEnvName,
			ToEnvironment:   provEnvName,
			DeploymentName:  deploymentName,
			JobName:         promoteJobName,
			ImageTag:        imageTag,
			CommitID:        "anyCommitID",
		},
	}

	gitCommitHash := pipelineInfo.GitCommitHash
	gitTags := pipelineInfo.GitTags
	pipelineInfo.SetApplicationConfig(ra)
	pipelineInfo.SetGitAttributes(gitCommitHash, gitTags)
	err = cli.Run(pipelineInfo)
	s.Require().NoError(err)

	rds, _ := s.radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(appName, provEnvName)).List(context.TODO(), metav1.ListOptions{})
	s.Len(rds.Items, 1)
	s.True(strings.HasPrefix(rds.Items[0].Name, fmt.Sprintf("%s-%s-", provEnvName, imageTag)))
	s.Equal(provEnvName, rds.Items[0].Labels[kube.RadixEnvLabel])
	s.Equal(imageTag, rds.Items[0].Labels[kube.RadixImageTagLabel])
	s.Equal(promoteJobName, rds.Items[0].Labels[kube.RadixJobNameLabel])
	s.Equal(4, *rds.Items[0].Spec.Components[0].Replicas)
	s.Equal("db-prod", rds.Items[0].Spec.Components[0].EnvironmentVariables["DB_HOST"])
	s.Equal("5678", rds.Items[0].Spec.Components[0].EnvironmentVariables["DB_PORT"])
	s.Equal("mysql", rds.Items[0].Spec.Components[0].EnvironmentVariables["DB_TYPE"])
	s.Equal("my-db-prod", rds.Items[0].Spec.Components[0].EnvironmentVariables["DB_NAME"])
	s.Equal(dnsAlias, rds.Items[0].Spec.Components[0].DNSExternalAlias[0])
	s.True(rds.Items[0].Spec.Components[0].DNSAppAlias)
	s.Len(rds.Items[0].Spec.Components[0].Secrets, 1)
	s.Equal("DEPLOYAPPSECRET", rds.Items[0].Spec.Components[0].Secrets[0])
	s.Len(rds.Items[0].Spec.Components[0].SecretRefs.AzureKeyVaults, 1)
	s.Len(rds.Items[0].Spec.Components[0].SecretRefs.AzureKeyVaults[0].Items, 2)
	s.Equal("TestKeyVault2", rds.Items[0].Spec.Components[0].SecretRefs.AzureKeyVaults[0].Name)
	s.Equal("Secret2", rds.Items[0].Spec.Components[0].SecretRefs.AzureKeyVaults[0].Items[0].Name)
	s.Equal("SECRET_2", rds.Items[0].Spec.Components[0].SecretRefs.AzureKeyVaults[0].Items[0].EnvVar)
	s.Equal("secret", string(*rds.Items[0].Spec.Components[0].SecretRefs.AzureKeyVaults[0].Items[0].Type))
	s.Equal("Key2", rds.Items[0].Spec.Components[0].SecretRefs.AzureKeyVaults[0].Items[1].Name)
	s.Equal("KEY_2", rds.Items[0].Spec.Components[0].SecretRefs.AzureKeyVaults[0].Items[1].EnvVar)
	s.Equal("key", string(*rds.Items[0].Spec.Components[0].SecretRefs.AzureKeyVaults[0].Items[1].Type))
	s.Equal(prodNode, rds.Items[0].Spec.Components[0].Node)
	s.Len(rds.Items[0].Spec.Jobs, 1)
	s.Len(rds.Items[0].Spec.Jobs[0].EnvironmentVariables, 3)
	s.Equal("prod1", rds.Items[0].Spec.Jobs[0].EnvironmentVariables["COMMON1"])
	s.Equal("common2", rds.Items[0].Spec.Jobs[0].EnvironmentVariables["COMMON2"])
	s.Equal("prod3", rds.Items[0].Spec.Jobs[0].EnvironmentVariables["PROD3"])
	s.Equal(numbers.Int32Ptr(8888), rds.Items[0].Spec.Jobs[0].SchedulerPort)
	s.Equal("/path", rds.Items[0].Spec.Jobs[0].Payload.Path)
	s.Len(rds.Items[0].Spec.Jobs[0].Secrets, 1)
	s.Equal("DEPLOYJOBSECRET", rds.Items[0].Spec.Jobs[0].Secrets[0])
	s.Len(rds.Items[0].Spec.Jobs[0].SecretRefs.AzureKeyVaults, 1)
	s.Len(rds.Items[0].Spec.Jobs[0].SecretRefs.AzureKeyVaults[0].Items, 2)
	s.Equal("TestKeyVault", rds.Items[0].Spec.Jobs[0].SecretRefs.AzureKeyVaults[0].Name)
	s.Equal("Secret1", rds.Items[0].Spec.Jobs[0].SecretRefs.AzureKeyVaults[0].Items[0].Name)
	s.Equal("SECRET_1", rds.Items[0].Spec.Jobs[0].SecretRefs.AzureKeyVaults[0].Items[0].EnvVar)
	s.Equal("secret", string(*rds.Items[0].Spec.Jobs[0].SecretRefs.AzureKeyVaults[0].Items[0].Type))
	s.Equal("Key1", rds.Items[0].Spec.Jobs[0].SecretRefs.AzureKeyVaults[0].Items[1].Name)
	s.Equal("KEY_1", rds.Items[0].Spec.Jobs[0].SecretRefs.AzureKeyVaults[0].Items[1].EnvVar)
	s.Equal("key", string(*rds.Items[0].Spec.Jobs[0].SecretRefs.AzureKeyVaults[0].Items[1].Type))
}

func (s *promoteTestSuite) TestPromote_PromoteToOtherEnvironment_Resources_NoOverride() {
	appName := "any-app"
	deploymentName := "deployment-1"
	imageTag := "abcdef"
	buildDeployJobName := "any-build-deploy-job"
	promoteJobName := "any-promote-job"
	prodEnvName := "prod"
	devEnvName := "dev"

	_, err := s.testUtils.ApplyDeployment(
		utils.ARadixDeployment().
			WithRadixApplication(
				utils.NewRadixApplicationBuilder().
					WithRadixRegistration(
						utils.ARadixRegistration().
							WithName(appName)).
					WithAppName(appName).
					WithEnvironment(devEnvName, "master").
					WithEnvironment(prodEnvName, "").
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
			WithAppName(appName).
			WithDeploymentName(deploymentName).
			WithEnvironment(devEnvName).
			WithImageTag(imageTag).
			WithLabel(kube.RadixJobNameLabel, buildDeployJobName))
	s.Require().NoError(err)

	// Create prod environment without any deployments
	commonTest.CreateEnvNamespace(s.kubeClient, appName, prodEnvName)

	rr, _ := s.radixClient.RadixV1().RadixRegistrations().Get(context.TODO(), appName, metav1.GetOptions{})
	ra, _ := s.radixClient.RadixV1().RadixApplications(utils.GetAppNamespace(appName)).Get(context.TODO(), appName, metav1.GetOptions{})

	cli := steps.NewPromoteStep()
	cli.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			FromEnvironment: devEnvName,
			ToEnvironment:   prodEnvName,
			DeploymentName:  deploymentName,
			JobName:         promoteJobName,
			ImageTag:        imageTag,
			CommitID:        "anyCommitID",
		},
	}

	gitCommitHash := pipelineInfo.GitCommitHash
	gitTags := pipelineInfo.GitTags
	pipelineInfo.SetApplicationConfig(ra)
	pipelineInfo.SetGitAttributes(gitCommitHash, gitTags)
	err = cli.Run(pipelineInfo)
	s.Require().NoError(err)

	rds, _ := s.radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(appName, prodEnvName)).List(context.TODO(), metav1.ListOptions{})
	s.Equal(1, len(rds.Items))
	s.True(strings.HasPrefix(rds.Items[0].Name, fmt.Sprintf("%s-%s-", prodEnvName, imageTag)))
	s.Equal(prodEnvName, rds.Items[0].Labels[kube.RadixEnvLabel])
	s.Equal(imageTag, rds.Items[0].Labels[kube.RadixImageTagLabel])
	s.Equal(promoteJobName, rds.Items[0].Labels[kube.RadixJobNameLabel])
	s.Equal("250m", rds.Items[0].Spec.Components[0].Resources.Requests["cpu"])
	s.Equal("64Mi", rds.Items[0].Spec.Components[0].Resources.Requests["memory"])
	s.Equal("500m", rds.Items[0].Spec.Components[0].Resources.Limits["cpu"])
	s.Equal("128Mi", rds.Items[0].Spec.Components[0].Resources.Limits["memory"])
	s.Equal("22m", rds.Items[0].Spec.Jobs[0].Resources.Requests["cpu"])
	s.Equal("11Mi", rds.Items[0].Spec.Jobs[0].Resources.Requests["memory"])
	s.Equal("44m", rds.Items[0].Spec.Jobs[0].Resources.Limits["cpu"])
	s.Equal("33Mi", rds.Items[0].Spec.Jobs[0].Resources.Limits["memory"])
}

func (s *promoteTestSuite) TestPromote_PromoteToOtherEnvironment_Authentication() {
	appName := "any-app"
	deploymentName := "deployment-1"
	imageTag := "abcdef"
	buildDeployJobName := "any-build-deploy-job"
	promoteJobName := "any-promote-job"
	prodEnvName := "prod"
	devEnvName := "dev"

	verification := radixv1.VerificationTypeOptional
	_, err := s.testUtils.ApplyDeployment(
		utils.NewDeploymentBuilder().
			WithAppName(appName).
			WithDeploymentName(deploymentName).
			WithEnvironment(devEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().WithName("app").WithAuthentication(nil),
			).
			WithLabel(kube.RadixJobNameLabel, buildDeployJobName).
			WithRadixApplication(
				utils.NewRadixApplicationBuilder().
					WithRadixRegistration(utils.ARadixRegistration().WithName(appName)).
					WithAppName(appName).
					WithEnvironment(prodEnvName, "").
					WithComponents(
						utils.AnApplicationComponent().
							WithName("app").
							WithAuthentication(
								&radixv1.Authentication{
									ClientCertificate: &radixv1.ClientCertificate{
										PassCertificateToUpstream: utils.BoolPtr(true),
									},
								},
							).
							WithEnvironmentConfigs(
								utils.NewComponentEnvironmentBuilder().WithEnvironment(prodEnvName).WithAuthentication(
									&radixv1.Authentication{
										ClientCertificate: &radixv1.ClientCertificate{
											Verification: &verification,
										},
									},
								),
							),
					)))
	s.Require().NoError(err)

	// Create environments
	commonTest.CreateEnvNamespace(s.kubeClient, appName, prodEnvName)
	commonTest.CreateEnvNamespace(s.kubeClient, appName, devEnvName)

	rr, _ := s.radixClient.RadixV1().RadixRegistrations().Get(context.TODO(), appName, metav1.GetOptions{})
	ra, _ := s.radixClient.RadixV1().RadixApplications(utils.GetAppNamespace(appName)).Get(context.TODO(), appName, metav1.GetOptions{})

	cli := steps.NewPromoteStep()
	cli.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			FromEnvironment: devEnvName,
			ToEnvironment:   prodEnvName,
			DeploymentName:  deploymentName,
			JobName:         promoteJobName,
			ImageTag:        imageTag,
			CommitID:        "anyCommitID",
		},
	}

	gitCommitHash := pipelineInfo.GitCommitHash
	gitTags := pipelineInfo.GitTags
	pipelineInfo.SetApplicationConfig(ra)
	pipelineInfo.SetGitAttributes(gitCommitHash, gitTags)
	err = cli.Run(pipelineInfo)
	s.Require().NoError(err)

	rds, _ := s.radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(appName, prodEnvName)).List(context.TODO(), metav1.ListOptions{})
	s.Equal(1, len(rds.Items))

	expectedAuth := &radixv1.Authentication{
		ClientCertificate: &radixv1.ClientCertificate{
			Verification:              &verification,
			PassCertificateToUpstream: utils.BoolPtr(true),
		},
	}
	s.NotNil(rds.Items[0].Spec.Components[0].Authentication)
	s.NotNil(rds.Items[0].Spec.Components[0].Authentication.ClientCertificate)
	s.Equal(expectedAuth, rds.Items[0].Spec.Components[0].Authentication)
}

func (s *promoteTestSuite) TestPromote_PromoteToOtherEnvironment_Resources_WithOverride() {
	appName := "any-app"
	deploymentName := "deployment-1"
	imageTag := "abcdef"
	buildDeployJobName := "any-build-deploy-job"
	promoteJobName := "any-promote-job"
	prodEnvName := "prod"
	devEnvName := "dev"

	_, err := s.testUtils.ApplyDeployment(
		utils.ARadixDeployment().
			WithRadixApplication(
				utils.NewRadixApplicationBuilder().
					WithRadixRegistration(
						utils.ARadixRegistration().
							WithName(appName)).
					WithAppName(appName).
					WithEnvironment(devEnvName, "master").
					WithEnvironment(prodEnvName, "").
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
									WithEnvironment(prodEnvName).
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
									WithEnvironment(prodEnvName).
									WithResource(
										map[string]string{
											"memory": "111Mi",
											"cpu":    "222m",
										}, map[string]string{
											"memory": "333Mi",
											"cpu":    "444m",
										})),
					)).
			WithAppName(appName).
			WithDeploymentName(deploymentName).
			WithEnvironment(devEnvName).
			WithImageTag(imageTag).
			WithLabel(kube.RadixJobNameLabel, buildDeployJobName))
	s.Require().NoError(err)

	// Create prod environment without any deployments
	commonTest.CreateEnvNamespace(s.kubeClient, appName, prodEnvName)

	rr, _ := s.radixClient.RadixV1().RadixRegistrations().Get(context.TODO(), appName, metav1.GetOptions{})
	ra, _ := s.radixClient.RadixV1().RadixApplications(utils.GetAppNamespace(appName)).Get(context.TODO(), appName, metav1.GetOptions{})

	cli := steps.NewPromoteStep()
	cli.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			FromEnvironment: devEnvName,
			ToEnvironment:   prodEnvName,
			DeploymentName:  deploymentName,
			JobName:         promoteJobName,
			ImageTag:        imageTag,
			CommitID:        "anyCommitID",
		},
	}

	gitCommitHash := pipelineInfo.GitCommitHash
	gitTags := pipelineInfo.GitTags
	pipelineInfo.SetApplicationConfig(ra)
	pipelineInfo.SetGitAttributes(gitCommitHash, gitTags)
	err = cli.Run(pipelineInfo)
	s.Require().NoError(err)

	rds, _ := s.radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(appName, prodEnvName)).List(context.TODO(), metav1.ListOptions{})
	s.Equal(1, len(rds.Items))
	s.True(strings.HasPrefix(rds.Items[0].Name, fmt.Sprintf("%s-%s-", prodEnvName, imageTag)))
	s.Equal(prodEnvName, rds.Items[0].Labels[kube.RadixEnvLabel])
	s.Equal(imageTag, rds.Items[0].Labels[kube.RadixImageTagLabel])
	s.Equal(promoteJobName, rds.Items[0].Labels[kube.RadixJobNameLabel])
	s.Equal("500m", rds.Items[0].Spec.Components[0].Resources.Requests["cpu"])
	s.Equal("128Mi", rds.Items[0].Spec.Components[0].Resources.Requests["memory"])
	s.Equal("750m", rds.Items[0].Spec.Components[0].Resources.Limits["cpu"])
	s.Equal("256Mi", rds.Items[0].Spec.Components[0].Resources.Limits["memory"])
	s.Equal("222m", rds.Items[0].Spec.Jobs[0].Resources.Requests["cpu"])
	s.Equal("111Mi", rds.Items[0].Spec.Jobs[0].Resources.Requests["memory"])
	s.Equal("444m", rds.Items[0].Spec.Jobs[0].Resources.Limits["cpu"])
	s.Equal("333Mi", rds.Items[0].Spec.Jobs[0].Resources.Limits["memory"])
}

func (s *promoteTestSuite) TestPromote_PromoteToSameEnvironment_NewStateIsExpected() {
	appName := "any-app"
	deploymentName := "deployment-1"
	imageTag := "abcdef"
	buildDeployJobName := "any-build-deploy-job"
	promoteJobName := "any-promote-job"
	devEnvName := "dev"

	_, err := s.testUtils.ApplyDeployment(
		utils.ARadixDeployment().
			WithAppName(appName).
			WithDeploymentName(deploymentName).
			WithEnvironment(devEnvName).
			WithImageTag(imageTag).
			WithLabel(kube.RadixJobNameLabel, buildDeployJobName))
	s.Require().NoError(err)

	rr, _ := s.radixClient.RadixV1().RadixRegistrations().Get(context.TODO(), appName, metav1.GetOptions{})
	ra, _ := s.radixClient.RadixV1().RadixApplications(utils.GetAppNamespace(appName)).Get(context.TODO(), appName, metav1.GetOptions{})

	cli := steps.NewPromoteStep()
	cli.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			FromEnvironment: devEnvName,
			ToEnvironment:   devEnvName,
			DeploymentName:  deploymentName,
			JobName:         promoteJobName,
			ImageTag:        imageTag,
			CommitID:        "anyCommitID",
		},
	}

	gitCommitHash := pipelineInfo.GitCommitHash
	gitTags := pipelineInfo.GitTags
	pipelineInfo.SetApplicationConfig(ra)
	pipelineInfo.SetGitAttributes(gitCommitHash, gitTags)
	err = cli.Run(pipelineInfo)
	s.Require().NoError(err)

	rds, _ := s.radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(appName, devEnvName)).List(context.Background(), metav1.ListOptions{})
	s.Equal(2, len(rds.Items))
}

func (s *promoteTestSuite) TestPromote_PromoteToOtherEnvironment_Identity() {
	appName := "any-app"
	deploymentName := "deployment-1"
	imageTag := "abcdef"
	buildDeployJobName := "any-build-deploy-job"
	promoteJobName := "any-promote-job"
	prodEnvName := "prod"
	devEnvName := "dev"
	currentRdIdentity := &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "any-current-identity-123"}}

	type scenarioSpec struct {
		name                 string
		commonConfig         *radixv1.Identity
		configureEnvironment bool
		environmentConfig    *radixv1.Identity
		expected             *radixv1.Identity
	}

	scenarios := []scenarioSpec{
		{name: "nil when commonConfig and environmentConfig is empty", commonConfig: &radixv1.Identity{}, configureEnvironment: true, environmentConfig: &radixv1.Identity{}, expected: nil},
		{name: "nil when commonConfig is nil and environmentConfig is empty", commonConfig: nil, configureEnvironment: true, environmentConfig: &radixv1.Identity{}, expected: nil},
		{name: "nil when commonConfig is empty and environmentConfig is nil", commonConfig: &radixv1.Identity{}, configureEnvironment: true, environmentConfig: nil, expected: nil},
		{name: "nil when commonConfig is nil and environmentConfig is not set", commonConfig: nil, configureEnvironment: false, environmentConfig: nil, expected: nil},
		{name: "nil when commonConfig is empty and environmentConfig is not set", commonConfig: &radixv1.Identity{}, configureEnvironment: false, environmentConfig: nil, expected: nil},
		{name: "use commonConfig when environmentConfig is empty", commonConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}, configureEnvironment: true, environmentConfig: &radixv1.Identity{}, expected: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}},
		{name: "use commonConfig when environmentConfig.Azure is empty", commonConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}, configureEnvironment: true, environmentConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{}}, expected: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}},
		{name: "override non-empty commonConfig with environmentConfig.Azure", commonConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}, configureEnvironment: true, environmentConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}, expected: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}},
		{name: "override empty commonConfig with environmentConfig", commonConfig: &radixv1.Identity{}, configureEnvironment: true, environmentConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}, expected: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}},
		{name: "override empty commonConfig.Azure with environmentConfig", commonConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{}}, configureEnvironment: true, environmentConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}, expected: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}},
		{name: "transform clientId with curly to standard format", commonConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "{11111111-2222-3333-4444-555555555555}"}}, configureEnvironment: false, environmentConfig: nil, expected: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}},
		{name: "transform clientId with urn:uuid to standard format", commonConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "urn:uuid:11111111-2222-3333-4444-555555555555"}}, configureEnvironment: false, environmentConfig: nil, expected: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}},
		{name: "transform clientId without dashes to standard format", commonConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "11111111222233334444555555555555"}}, configureEnvironment: false, environmentConfig: nil, expected: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			var componentEnvironmentConfigs []utils.RadixEnvironmentConfigBuilder
			var jobEnvironmentConfigs []utils.RadixJobComponentEnvironmentConfigBuilder

			if scenario.configureEnvironment {
				componentEnvironmentConfigs = append(componentEnvironmentConfigs, utils.AnEnvironmentConfig().WithEnvironment(prodEnvName).WithIdentity(scenario.environmentConfig))
				jobEnvironmentConfigs = append(jobEnvironmentConfigs, utils.AJobComponentEnvironmentConfig().WithEnvironment(prodEnvName).WithIdentity(scenario.environmentConfig))
			}

			_, err := s.testUtils.ApplyDeployment(
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
					WithAppName(appName).
					WithDeploymentName(deploymentName).
					WithEnvironment(devEnvName).
					WithImageTag(imageTag).
					WithLabel(kube.RadixJobNameLabel, buildDeployJobName).
					WithRadixApplication(
						utils.NewRadixApplicationBuilder().
							WithRadixRegistration(
								utils.ARadixRegistration().
									WithName(appName)).
							WithAppName(appName).
							WithEnvironment(devEnvName, "").
							WithEnvironment(prodEnvName, "").
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
			s.Require().NoError(err)

			// Create prod environment without any deployments
			commonTest.CreateEnvNamespace(s.kubeClient, appName, prodEnvName)

			rr, _ := s.radixClient.RadixV1().RadixRegistrations().Get(context.TODO(), appName, metav1.GetOptions{})
			ra, _ := s.radixClient.RadixV1().RadixApplications(utils.GetAppNamespace(appName)).Get(context.TODO(), appName, metav1.GetOptions{})

			cli := steps.NewPromoteStep()
			cli.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)

			pipelineInfo := &model.PipelineInfo{
				PipelineArguments: model.PipelineArguments{
					FromEnvironment: devEnvName,
					ToEnvironment:   prodEnvName,
					DeploymentName:  deploymentName,
					JobName:         promoteJobName,
					ImageTag:        imageTag,
					CommitID:        "anyCommitID",
				},
			}

			pipelineInfo.SetApplicationConfig(ra)
			err = cli.Run(pipelineInfo)
			s.Require().NoError(err)

			rds, _ := s.radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(appName, prodEnvName)).List(context.TODO(), metav1.ListOptions{})
			s.Require().Equal(1, len(rds.Items))
			s.Equal(scenario.expected, rds.Items[0].Spec.Components[0].Identity)
			s.Equal(scenario.expected, rds.Items[0].Spec.Jobs[0].Identity)
		})

	}
}

func (s *promoteTestSuite) TestPromote_AnnotatedBySourceDeploymentAttributes() {
	anyAppName := "any-app"
	srcDeploymentName := "deployment-1"
	anyImageTag := "abcdef"
	anyPromoteJobName := "any-promote-job"
	dstEnv := "test"
	srcEnv := "dev"
	srcDeploymentCommitID := "222ca8595c5283a9d0f17a623b9255a0d9866a2e"
	srcRadixConfigHash := "sha256-a5d1565b32252be05910e459eb7551fd0fd6e0d513f7728c54ca5507c9b11387"

	_, err := s.testUtils.ApplyDeployment(
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

	s.Require().NoError(err)

	rr, _ := s.radixClient.RadixV1().RadixRegistrations().Get(context.TODO(), anyAppName, metav1.GetOptions{})
	ra, _ := s.radixClient.RadixV1().RadixApplications(utils.GetAppNamespace(anyAppName)).Get(context.TODO(), anyAppName, metav1.GetOptions{})

	cli := steps.NewPromoteStep()
	cli.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			FromEnvironment: srcEnv,
			ToEnvironment:   dstEnv,
			DeploymentName:  srcDeploymentName,
			JobName:         anyPromoteJobName,
			ImageTag:        anyImageTag,
			CommitID:        "anyCommitID",
		},
	}

	gitCommitHash := pipelineInfo.GitCommitHash
	gitTags := pipelineInfo.GitTags
	pipelineInfo.SetApplicationConfig(ra)
	pipelineInfo.SetGitAttributes(gitCommitHash, gitTags)
	err = cli.Run(pipelineInfo)
	s.Require().NoError(err)

	rds, err := s.radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(anyAppName, dstEnv)).List(context.TODO(), metav1.ListOptions{})
	s.NoError(err)
	s.Require().Len(rds.Items, 1)
	promotedRD := rds.Items[0]
	s.Equal(srcEnv, promotedRD.GetAnnotations()[kube.RadixDeploymentPromotedFromEnvironmentAnnotation])
	s.Equal(srcDeploymentName, promotedRD.GetAnnotations()[kube.RadixDeploymentPromotedFromDeploymentAnnotation])
	s.Equal(srcRadixConfigHash, promotedRD.GetAnnotations()[kube.RadixConfigHash])
	s.Equal(srcDeploymentCommitID, promotedRD.GetLabels()[kube.RadixCommitLabel])
}

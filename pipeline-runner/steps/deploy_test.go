package steps_test

import (
	"context"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	internaltest "github.com/equinor/radix-operator/pipeline-runner/internal/test"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps"
	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func Test_RunDeployTestSuite(t *testing.T) {
	suite.Run(t, new(deployTestSuite))
}

type deployTestSuite struct {
	suite.Suite
	kubeClient  *kubefake.Clientset
	radixClient *radixfake.Clientset
	promClient  *prometheusfake.Clientset
	kubeUtil    *kube.Kube
	ctrl        *gomock.Controller
}

func (s *deployTestSuite) SetupTest() {
	s.setupTest()
}

func (s *deployTestSuite) SetupSubTest() {
	s.setupTest()
}

func (s *deployTestSuite) setupTest() {
	s.kubeClient = kubefake.NewSimpleClientset()
	s.radixClient = radixfake.NewSimpleClientset()
	s.promClient = prometheusfake.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, nil)
	s.ctrl = gomock.NewController(s.T())
}

func (s *deployTestSuite) TestDeploy_BranchIsNotMapped_ShouldSkip() {
	appName := "any-app"
	mappedBranch := "master"
	noMappedBranch := "feature"

	rr := utils.ARadixRegistration().
		WithName(appName).
		BuildRR()

	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithEnvironment("anyEnvironment", mappedBranch).
		WithComponents(
			utils.AnApplicationComponent().
				WithName("anyComponentName")).
		BuildRA()

	deployStep := steps.NewDeployStep(internaltest.FakeNamespaceWatcher{})
	deployStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)

	targetEnvs := application.GetTargetEnvironments(noMappedBranch, ra)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			JobName:  "anyjob",
			ImageTag: "anyimagetag",
			Branch:   noMappedBranch,
			CommitID: "anycommit",
		},
		TargetEnvironments: targetEnvs,
	}

	err := deployStep.Run(pipelineInfo)
	s.Require().NoError(err)
	radixJobList, err := s.radixClient.RadixV1().RadixJobs(utils.GetAppNamespace(appName)).List(context.Background(), metav1.ListOptions{})
	s.Require().NoError(err)
	s.Empty(radixJobList.Items)
}

func (s *deployTestSuite) TestDeploy_PromotionSetup_ShouldCreateNamespacesForAllBranchesIfNotExists() {
	appName := "any-app"
	jobName := "any-job-name"
	commitID := "4faca8595c5283a9d0f17a623b9255a0d9866a2e"
	gitTags := "some tags go here"

	rr := utils.ARadixRegistration().
		WithName(appName).
		BuildRR()

	certificateVerification := radixv1.VerificationTypeOptional

	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithEnvironment("dev", "master").
		WithEnvironment("prod", "").
		WithDNSAppAlias("dev", "app").
		WithComponents(
			utils.AnApplicationComponent().
				WithName("app").
				WithPublicPort("http").
				WithPort("http", 8080).
				WithAuthentication(
					&radixv1.Authentication{
						ClientCertificate: &radixv1.ClientCertificate{
							PassCertificateToUpstream: utils.BoolPtr(true),
						},
					},
				).
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("prod").
						WithReplicas(pointers.Ptr(4)),
					utils.AnEnvironmentConfig().
						WithEnvironment("dev").
						WithAuthentication(
							&radixv1.Authentication{
								ClientCertificate: &radixv1.ClientCertificate{
									Verification:              &certificateVerification,
									PassCertificateToUpstream: utils.BoolPtr(false),
								},
							},
						).
						WithReplicas(pointers.Ptr(4))),
			utils.AnApplicationComponent().
				WithName("redis").
				WithPublicPort("").
				WithPort("http", 6379).
				WithAuthentication(
					&radixv1.Authentication{
						ClientCertificate: &radixv1.ClientCertificate{
							PassCertificateToUpstream: utils.BoolPtr(true),
						},
					},
				).
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("dev").
						WithEnvironmentVariable("DB_HOST", "db-dev").
						WithEnvironmentVariable("DB_PORT", "1234").
						WithResource(map[string]string{
							"memory": "64Mi",
							"cpu":    "250m",
						}, map[string]string{
							"memory": "128Mi",
							"cpu":    "500m",
						}),
					utils.AnEnvironmentConfig().
						WithEnvironment("prod").
						WithEnvironmentVariable("DB_HOST", "db-prod").
						WithEnvironmentVariable("DB_PORT", "9876").
						WithResource(map[string]string{
							"memory": "64Mi",
							"cpu":    "250m",
						}, map[string]string{
							"memory": "128Mi",
							"cpu":    "500m",
						}),
					utils.AnEnvironmentConfig().
						WithEnvironment("no-existing-env").
						WithEnvironmentVariable("DB_HOST", "db-prod").
						WithEnvironmentVariable("DB_PORT", "9876"))).
		BuildRA()

	// Prometheus doesn´t contain any fake
	cli := steps.NewDeployStep(internaltest.FakeNamespaceWatcher{})
	cli.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)

	targetEnvs := application.GetTargetEnvironments("master", ra)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			JobName:  jobName,
			ImageTag: "anyImageTag",
			Branch:   "master",
			CommitID: commitID,
		},
		TargetEnvironments: targetEnvs,
		GitCommitHash:      commitID,
		GitTags:            gitTags,
	}

	gitCommitHash := pipelineInfo.GitCommitHash

	pipelineInfo.SetApplicationConfig(ra)
	pipelineInfo.SetGitAttributes(gitCommitHash, gitTags)
	err := cli.Run(pipelineInfo)
	s.Require().NoError(err)
	rds, _ := s.radixClient.RadixV1().RadixDeployments(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	s.Require().Len(rds.Items, 1)
	rdDev := rds.Items[0]
	s.Require().Equal("any-app-dev", rdDev.Namespace)

	// validate deployment environment variables
	s.Len(rdDev.Spec.Components, 2)
	s.Len(rdDev.Spec.Components[1].EnvironmentVariables, 4)
	s.Equal("db-dev", rdDev.Spec.Components[1].EnvironmentVariables["DB_HOST"])
	s.Equal("1234", rdDev.Spec.Components[1].EnvironmentVariables["DB_PORT"])
	s.Equal(commitID, rdDev.Spec.Components[1].EnvironmentVariables[defaults.RadixCommitHashEnvironmentVariable])
	s.Equal(gitTags, rdDev.Spec.Components[1].EnvironmentVariables[defaults.RadixGitTagsEnvironmentVariable])
	s.NotEmpty(rdDev.Annotations[kube.RadixBranchAnnotation])
	s.NotEmpty(rdDev.Labels[kube.RadixCommitLabel])
	s.NotEmpty(rdDev.Labels["radix-job-name"])
	s.Equal("master", rdDev.Annotations[kube.RadixBranchAnnotation])
	s.Equal(commitID, rdDev.Labels[kube.RadixCommitLabel])
	s.Equal(jobName, rdDev.Labels["radix-job-name"])

	// validate authentication variable
	expectedAuthComp1 := &radixv1.Authentication{
		ClientCertificate: &radixv1.ClientCertificate{
			Verification:              &certificateVerification,
			PassCertificateToUpstream: utils.BoolPtr(false),
		},
	}
	expectedAuthComp2 := &radixv1.Authentication{
		ClientCertificate: &radixv1.ClientCertificate{
			PassCertificateToUpstream: utils.BoolPtr(true),
		},
	}
	s.NotNil(rdDev.Spec.Components[0].Authentication)
	s.NotNil(rdDev.Spec.Components[0].Authentication.ClientCertificate)
	s.Equal(expectedAuthComp1, rdDev.Spec.Components[0].Authentication)
	s.NotNil(rdDev.Spec.Components[1].Authentication)
	s.NotNil(rdDev.Spec.Components[1].Authentication.ClientCertificate)
	s.Equal(expectedAuthComp2, rdDev.Spec.Components[1].Authentication)

	// validate dns app alias
	s.True(rdDev.Spec.Components[0].DNSAppAlias)
	s.False(rdDev.Spec.Components[1].DNSAppAlias)

	// validate resources
	s.NotNil(rdDev.Spec.Components[1].Resources)
	s.Equal("128Mi", rdDev.Spec.Components[1].Resources.Limits["memory"])
	s.Equal("500m", rdDev.Spec.Components[1].Resources.Limits["cpu"])
}

func (s *deployTestSuite) TestDeploy_SetCommitID_whenSet() {
	appName := "any-app"

	rr := utils.ARadixRegistration().
		WithName(appName).
		BuildRR()
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithEnvironment("dev", "master").
		WithComponents(utils.AnApplicationComponent().WithName("app")).
		BuildRA()
	const commitID = "222ca8595c5283a9d0f17a623b9255a0d9866a2e"

	cli := steps.NewDeployStep(internaltest.FakeNamespaceWatcher{})
	cli.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			JobName:  "anyjob",
			ImageTag: "anyimagetag",
			Branch:   "master",
			CommitID: "anycommit",
		},
		TargetEnvironments: []string{"master"},
		GitCommitHash:      commitID,
		GitTags:            "",
	}

	gitCommitHash := pipelineInfo.GitCommitHash
	gitTags := pipelineInfo.GitTags

	pipelineInfo.SetApplicationConfig(ra)
	pipelineInfo.SetGitAttributes(gitCommitHash, gitTags)
	err := cli.Run(pipelineInfo)
	s.Require().NoError(err)

	rds, err := s.radixClient.RadixV1().RadixDeployments("any-app-dev").List(context.TODO(), metav1.ListOptions{})
	s.Require().NoError(err)
	s.Len(rds.Items, 1)
	rd := rds.Items[0]
	s.Equal(commitID, rd.ObjectMeta.Labels[kube.RadixCommitLabel])
}

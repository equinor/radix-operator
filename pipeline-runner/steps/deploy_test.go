package steps_test

import (
	"context"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pipeline-runner/internal/hash"
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

func (s *deployTestSuite) Test_DeployOnly_DeploymentConsistent() {
	appName := "any-app"
	jobName := "any-job-name"
	commitID := "4faca8595c5283a9d0f17a623b9255a0d9866a2e"
	prepareConfigMapName := "preparecm"
	gitConfigMapName := "gitcm"
	certificateVerification := radixv1.VerificationTypeOptional
	rr := utils.ARadixRegistration().WithName(appName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithEnvironment("dev", "master").
		WithEnvironment("prod", "").
		WithDNSAppAlias("dev", "app").
		WithComponents(
			utils.NewApplicationComponentBuilder().
				WithName("app").
				WithImage("app:1").
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
				WithImage("redis:1").
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
				)).
		BuildRA()

	s.Require().NoError(internaltest.CreatePreparePipelineConfigMapResponse(s.kubeClient, prepareConfigMapName, appName, ra, nil))
	s.Require().NoError(internaltest.CreateGitInfoConfigMapResponse(s.kubeClient, gitConfigMapName, appName, "someothercommitid", "anytags"))
	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			PipelineType:  string(radixv1.Deploy),
			ToEnvironment: "dev",
			JobName:       jobName,
			CommitID:      commitID,
		},
		RadixConfigMapName: prepareConfigMapName,
		GitConfigMapName:   gitConfigMapName,
	}

	applyStep := steps.NewApplyConfigStep()
	applyStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	deployStep := steps.NewDeployStep(internaltest.FakeNamespaceWatcher{})
	deployStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)

	s.Require().NoError(applyStep.Run(pipelineInfo))
	s.Require().NoError(deployStep.Run(pipelineInfo))
	rds, _ := s.radixClient.RadixV1().RadixDeployments(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	s.Require().Len(rds.Items, 1)
	rdDev := rds.Items[0]
	s.Require().Equal("any-app-dev", rdDev.Namespace)

	// validate deployment environment variables
	s.NotEmpty(rdDev.Labels[kube.RadixCommitLabel])
	s.Equal(jobName, rdDev.Labels["radix-job-name"])
	expectedRaHash, _ := hash.ToHashString(hash.SHA256, ra.Spec)
	s.Equal(expectedRaHash, rdDev.GetAnnotations()[kube.RadixConfigHash])
	s.Equal(internaltest.GetBuildSecretHash(nil), rdDev.GetAnnotations()[kube.RadixBuildSecretHash])
	s.Empty(rdDev.GetAnnotations()[kube.RadixBranchAnnotation])
	s.Empty(rdDev.GetAnnotations()[kube.RadixGitTagsAnnotation])
	s.Equal(commitID, rdDev.GetAnnotations()[kube.RadixCommitAnnotation])
	s.Len(rdDev.Spec.Components, 2)
	s.Len(rdDev.Spec.Components[1].EnvironmentVariables, 4)
	s.Equal("db-dev", rdDev.Spec.Components[1].EnvironmentVariables["DB_HOST"])
	s.Equal("1234", rdDev.Spec.Components[1].EnvironmentVariables["DB_PORT"])
	s.Equal(commitID, rdDev.Spec.Components[1].EnvironmentVariables[defaults.RadixCommitHashEnvironmentVariable])
	s.Empty(rdDev.Spec.Components[1].EnvironmentVariables[defaults.RadixGitTagsEnvironmentVariable])

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

func (s *deployTestSuite) Test_DeployOnly_IsDeployable() {
	appName := "any-app"
	jobName := "any-job-name"
	prepareConfigMapName := "preparecm"
	rr := utils.ARadixRegistration().WithName(appName).BuildRR()
	pipelineArgs := model.PipelineArguments{
		PipelineType:  string(radixv1.Deploy),
		ToEnvironment: "dev",
		JobName:       jobName,
	}

	type testSpec struct {
		name          string
		components    []utils.RadixApplicationComponentBuilder
		jobs          []utils.RadixApplicationJobComponentBuilder
		expectedError error
	}

	tests := []testSpec{
		{
			name: "component and job with image should succeed",
			components: []utils.RadixApplicationComponentBuilder{
				utils.NewApplicationComponentBuilder().WithName("c1").WithPort("any", 8000).WithImage("img:1"),
			},
			jobs: []utils.RadixApplicationJobComponentBuilder{
				utils.NewApplicationJobComponentBuilder().WithName("j1").WithSchedulerPort(pointers.Ptr[int32](8000)).WithImage("img:1"),
			},
		},
		{
			name: "component without image should fail",
			components: []utils.RadixApplicationComponentBuilder{
				utils.NewApplicationComponentBuilder().WithName("c1").WithPort("any", 8000).WithImage("img:1"),
				utils.NewApplicationComponentBuilder().WithName("c2").WithPort("any", 8000),
			},
			expectedError: steps.ErrDeployOnlyPipelineDoesNotSupportBuild,
		},
		{
			name: "job without image should fail",
			jobs: []utils.RadixApplicationJobComponentBuilder{
				utils.NewApplicationJobComponentBuilder().WithName("j1").WithSchedulerPort(pointers.Ptr[int32](8000)).WithImage("img:1"),
				utils.NewApplicationJobComponentBuilder().WithName("j2").WithSchedulerPort(pointers.Ptr[int32](8000)),
			},
			expectedError: steps.ErrDeployOnlyPipelineDoesNotSupportBuild,
		},
	}

	for _, test := range tests {
		s.Run(test.name, func() {
			_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
			ra := utils.NewRadixApplicationBuilder().
				WithAppName(appName).
				WithEnvironment("dev", "master").
				WithComponents(test.components...).
				WithJobComponents(test.jobs...).
				BuildRA()
			s.Require().NoError(internaltest.CreatePreparePipelineConfigMapResponse(s.kubeClient, prepareConfigMapName, appName, ra, nil))

			pipelineInfo := &model.PipelineInfo{
				PipelineArguments:  pipelineArgs,
				RadixConfigMapName: prepareConfigMapName,
			}
			applyStep := steps.NewApplyConfigStep()
			applyStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
			deployStep := steps.NewDeployStep(internaltest.FakeNamespaceWatcher{})
			deployStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)

			err := applyStep.Run(pipelineInfo)
			if test.expectedError != nil {
				s.ErrorIs(err, test.expectedError)
				return
			} else {
				s.NoError(err)
			}
			s.NoError(deployStep.Run(pipelineInfo))
		})
	}
}

func (s *deployTestSuite) Test_Deploy_ImageTagNames() {
	appName, envName, rjName, buildBranch, jobPort := "anyapp", "dev", "anyrj", "anybranch", pointers.Ptr[int32](9999)
	prepareConfigMapName := "preparecm"

	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	_, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	rj := utils.ARadixBuildDeployJob().WithJobName(rjName).WithAppName(appName).BuildRJ()
	_, _ = s.radixClient.RadixV1().RadixJobs(utils.GetAppNamespace(appName)).Create(context.Background(), rj, metav1.CreateOptions{})
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithEnvironment(envName, buildBranch).
		WithComponents(
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("comp1").WithImage("comp1img:{imageTagName}").
				WithEnvironmentConfig(utils.NewComponentEnvironmentBuilder().WithEnvironment(envName).WithImageTagName("comp1envtag")),
			utils.NewApplicationComponentBuilder().WithPort("any", 8080).WithName("comp2").WithImage("comp2img:{imageTagName}").
				WithEnvironmentConfig(utils.NewComponentEnvironmentBuilder().WithEnvironment(envName).WithImageTagName("comp2envtag")),
		).
		WithJobComponents(
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("job1").WithImage("job1img:{imageTagName}").
				WithEnvironmentConfig(utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName).WithImageTagName("job1envtag")),
			utils.NewApplicationJobComponentBuilder().WithSchedulerPort(jobPort).WithName("job2").WithImage("job2img:{imageTagName}").
				WithEnvironmentConfig(utils.NewJobComponentEnvironmentBuilder().WithEnvironment(envName).WithImageTagName("job2envtag")),
		).
		BuildRA()
	s.Require().NoError(internaltest.CreatePreparePipelineConfigMapResponse(s.kubeClient, prepareConfigMapName, appName, ra, nil))
	pipeline := model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			PipelineType:  string(radixv1.Deploy),
			ToEnvironment: envName,
			JobName:       rjName,
			ImageTagNames: map[string]string{"comp1": "comp1customtag", "job1": "job1customtag"},
		},
		RadixConfigMapName: prepareConfigMapName,
	}

	applyStep := steps.NewApplyConfigStep()
	applyStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	deployStep := steps.NewDeployStep(internaltest.FakeNamespaceWatcher{})
	deployStep.Init(s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)

	s.Require().NoError(applyStep.Run(&pipeline))
	s.Require().NoError(deployStep.Run(&pipeline))

	// Check RadixDeployment component and job images
	rds, _ := s.radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(appName, envName)).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(rds.Items, 1)
	rd := rds.Items[0]
	type deployComponentSpec struct {
		Name  string
		Image string
	}
	expectedDeployComponents := []deployComponentSpec{
		{Name: "comp1", Image: "comp1img:comp1customtag"},
		{Name: "comp2", Image: "comp2img:comp2envtag"},
	}
	actualDeployComponents := slice.Map(rd.Spec.Components, func(c radixv1.RadixDeployComponent) deployComponentSpec {
		return deployComponentSpec{Name: c.Name, Image: c.Image}
	})
	s.ElementsMatch(expectedDeployComponents, actualDeployComponents)
	expectedJobComponents := []deployComponentSpec{
		{Name: "job1", Image: "job1img:job1customtag"},
		{Name: "job2", Image: "job2img:job2envtag"},
	}
	actualJobComponents := slice.Map(rd.Spec.Jobs, func(c radixv1.RadixDeployJobComponent) deployComponentSpec {
		return deployComponentSpec{Name: c.Name, Image: c.Image}
	})
	s.ElementsMatch(expectedJobComponents, actualJobComponents)
}

package deploy_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	internaltest "github.com/equinor/radix-operator/pipeline-runner/internal/test"
	"github.com/equinor/radix-operator/pipeline-runner/internal/watcher"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/deploy"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commonTest "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func Test_RunDeployTestSuite(t *testing.T) {
	suite.Run(t, new(deployTestSuite))
}

type deployTestSuite struct {
	suite.Suite
	kubeClient  *kubefake.Clientset
	radixClient *radix.Clientset
	kedaClient  *kedafake.Clientset
	promClient  *prometheusfake.Clientset
	kubeUtil    *kube.Kube
	ctrl        *gomock.Controller
}

func (s *deployTestSuite) SetupTest() {
	s.kubeClient = kubefake.NewSimpleClientset()
	s.radixClient = radix.NewSimpleClientset()
	s.kedaClient = kedafake.NewSimpleClientset()
	s.promClient = prometheusfake.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, s.kedaClient, nil)
	s.ctrl = gomock.NewController(s.T())
}

func (s *deployTestSuite) Test_EmptyTargetEnvironments_SkipDeployment() {
	appName := "anyappname"
	rr := utils.ARadixRegistration().WithName(appName).BuildRR()
	pipelineInfo := &model.PipelineInfo{
		TargetEnvironments: []string{},
	}
	namespaceWatcher := watcher.NewMockNamespaceWatcher(s.ctrl)
	namespaceWatcher.EXPECT().WaitFor(gomock.Any(), gomock.Any()).Times(0)
	radixDeploymentWatcher := watcher.NewMockRadixDeploymentWatcher(s.ctrl)
	radixDeploymentWatcher.EXPECT().WaitForActive(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	cli := deploy.NewDeployStep(namespaceWatcher, radixDeploymentWatcher)
	cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	err := cli.Run(context.Background(), pipelineInfo)
	s.Require().NoError(err)
}

func (s *deployTestSuite) TestDeploy_PromotionSetup_ShouldCreateNamespacesForAllBranchesIfNotExists() {
	appName, envName, branch, jobName, imageTag, commitId, gitTags := "anyapp", "dev", "master", "anyjobname", "anyimagetag", "anycommit", "gittags"
	rr := utils.ARadixRegistration().
		WithName(appName).
		BuildRR()

	certificateVerification := radixv1.VerificationTypeOptional

	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithEnvironment(envName, branch).
		WithEnvironment("prod", "").
		WithDNSAppAlias(envName, "app").
		WithComponents(
			utils.AnApplicationComponent().
				WithName("app").
				WithPublicPort("http").
				WithPort("http", 8080).
				WithAuthentication(
					&radixv1.Authentication{
						ClientCertificate: &radixv1.ClientCertificate{
							PassCertificateToUpstream: pointers.Ptr(true),
						},
					},
				).
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("prod").
						WithReplicas(commonTest.IntPtr(4)),
					utils.AnEnvironmentConfig().
						WithEnvironment(envName).
						WithAuthentication(
							&radixv1.Authentication{
								ClientCertificate: &radixv1.ClientCertificate{
									Verification:              &certificateVerification,
									PassCertificateToUpstream: pointers.Ptr(false),
								},
							},
						).
						WithReplicas(commonTest.IntPtr(4))),
			utils.AnApplicationComponent().
				WithName("redis").
				WithPublicPort("").
				WithPort("http", 6379).
				WithAuthentication(
					&radixv1.Authentication{
						ClientCertificate: &radixv1.ClientCertificate{
							PassCertificateToUpstream: pointers.Ptr(true),
						},
					},
				).
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment(envName).
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

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			JobName:  jobName,
			ImageTag: imageTag,
			Branch:   branch,
		},
		RadixApplication:   ra,
		TargetEnvironments: []string{envName},
		GitCommitHash:      commitId,
		GitTags:            gitTags,
	}

	namespaceWatcher := watcher.NewMockNamespaceWatcher(s.ctrl)
	namespaceWatcher.EXPECT().WaitFor(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	radixDeploymentWatcher := watcher.NewMockRadixDeploymentWatcher(s.ctrl)
	radixDeploymentWatcher.EXPECT().WaitForActive(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	cli := deploy.NewDeployStep(namespaceWatcher, radixDeploymentWatcher)
	cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
	err := cli.Run(context.Background(), pipelineInfo)
	s.Require().NoError(err)

	rds, _ := s.radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(appName, envName)).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(rds.Items, 1)
	rdDev := rds.Items[0]

	s.Run("validate deployment environment variables", func() {
		s.Equal(2, len(rdDev.Spec.Components))
		s.Equal(4, len(rdDev.Spec.Components[1].EnvironmentVariables))
		s.Equal("db-dev", rdDev.Spec.Components[1].EnvironmentVariables["DB_HOST"])
		s.Equal("1234", rdDev.Spec.Components[1].EnvironmentVariables["DB_PORT"])
		s.Equal(commitId, rdDev.Spec.Components[1].EnvironmentVariables[defaults.RadixCommitHashEnvironmentVariable])
		s.Equal(gitTags, rdDev.Spec.Components[1].EnvironmentVariables[defaults.RadixGitTagsEnvironmentVariable])
		s.NotEmpty(rdDev.Annotations[kube.RadixBranchAnnotation])
		s.NotEmpty(rdDev.Labels[kube.RadixCommitLabel])
		s.NotEmpty(rdDev.Labels["radix-job-name"])
		s.Equal(branch, rdDev.Annotations[kube.RadixBranchAnnotation])
		s.Equal(commitId, rdDev.Labels[kube.RadixCommitLabel])
		s.Equal(jobName, rdDev.Labels["radix-job-name"])
	})

	s.Run("validate authentication variable", func() {
		x0 := &radixv1.Authentication{
			ClientCertificate: &radixv1.ClientCertificate{
				Verification:              &certificateVerification,
				PassCertificateToUpstream: pointers.Ptr(false),
			},
		}

		x1 := &radixv1.Authentication{
			ClientCertificate: &radixv1.ClientCertificate{
				PassCertificateToUpstream: pointers.Ptr(true),
			},
		}

		s.NotNil(rdDev.Spec.Components[0].Authentication)
		s.NotNil(rdDev.Spec.Components[0].Authentication.ClientCertificate)
		s.Equal(x0, rdDev.Spec.Components[0].Authentication)

		s.NotNil(rdDev.Spec.Components[1].Authentication)
		s.NotNil(rdDev.Spec.Components[1].Authentication.ClientCertificate)
		s.Equal(x1, rdDev.Spec.Components[1].Authentication)
	})

	s.Run("validate dns app alias", func() {
		s.True(rdDev.Spec.Components[0].DNSAppAlias)
		s.False(rdDev.Spec.Components[1].DNSAppAlias)
	})

	s.Run("validate resources", func() {
		s.NotNil(rdDev.Spec.Components[1].Resources)
		s.Equal("128Mi", rdDev.Spec.Components[1].Resources.Limits["memory"])
		s.Equal("500m", rdDev.Spec.Components[1].Resources.Limits["cpu"])
	})
}

func (s *deployTestSuite) Test_RadixConfigHashAnnotation() {
	anyAppName := "anyapp"
	rr := utils.ARadixRegistration().
		WithName(anyAppName).
		BuildRR()

	ra := utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment("dev", "master").
		WithComponents(utils.AnApplicationComponent().WithName("app")).
		BuildRA()

	namespaceWatcher := watcher.NewMockNamespaceWatcher(s.ctrl)
	namespaceWatcher.EXPECT().WaitFor(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	radixDeploymentWatcher := watcher.NewMockRadixDeploymentWatcher(s.ctrl)
	radixDeploymentWatcher.EXPECT().WaitForActive(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	cli := deploy.NewDeployStep(namespaceWatcher, radixDeploymentWatcher)
	cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, &monitoring.Clientset{}, rr)

	pipelineInfo := &model.PipelineInfo{
		RadixApplication:   ra,
		TargetEnvironments: []string{"dev"},
	}

	err := cli.Run(context.Background(), pipelineInfo)
	s.Require().NoError(err)
	rds, _ := s.radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(anyAppName, "dev")).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(rds.Items, 1)
	rd := rds.Items[0]
	expectedHash := internaltest.GetRadixApplicationHash(ra)
	s.Equal(expectedHash, rd.Annotations[kube.RadixConfigHash])
}

func (s *deployTestSuite) Test_RadixBuildSecretHashAnnotation_BuildSecretSet() {
	anyAppName := "anyapp"
	rr := utils.ARadixRegistration().
		WithName(anyAppName).
		BuildRR()

	ra := utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment("dev", "master").
		WithComponents(utils.AnApplicationComponent().WithName("app")).
		BuildRA()

	secret := &corev1.Secret{Data: map[string][]byte{"anykey": []byte("anydata")}}

	namespaceWatcher := watcher.NewMockNamespaceWatcher(s.ctrl)
	namespaceWatcher.EXPECT().WaitFor(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	radixDeploymentWatcher := watcher.NewMockRadixDeploymentWatcher(s.ctrl)
	radixDeploymentWatcher.EXPECT().WaitForActive(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	cli := deploy.NewDeployStep(namespaceWatcher, radixDeploymentWatcher)
	cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, &monitoring.Clientset{}, rr)

	pipelineInfo := &model.PipelineInfo{
		RadixApplication:   ra,
		BuildSecret:        secret,
		TargetEnvironments: []string{"dev"},
	}

	err := cli.Run(context.Background(), pipelineInfo)
	s.Require().NoError(err)
	rds, _ := s.radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(anyAppName, "dev")).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(rds.Items, 1)
	rd := rds.Items[0]
	expectedHash := internaltest.GetBuildSecretHash(secret)
	s.Equal(expectedHash, rd.Annotations[kube.RadixBuildSecretHash])
}

func (s *deployTestSuite) Test_RadixBuildSecretHashAnnotation_BuildSecretNot() {
	anyAppName := "anyapp"
	rr := utils.ARadixRegistration().
		WithName(anyAppName).
		BuildRR()

	ra := utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment("dev", "master").
		WithComponents(utils.AnApplicationComponent().WithName("app")).
		BuildRA()

	namespaceWatcher := watcher.NewMockNamespaceWatcher(s.ctrl)
	namespaceWatcher.EXPECT().WaitFor(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	radixDeploymentWatcher := watcher.NewMockRadixDeploymentWatcher(s.ctrl)
	radixDeploymentWatcher.EXPECT().WaitForActive(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	cli := deploy.NewDeployStep(namespaceWatcher, radixDeploymentWatcher)
	cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, &monitoring.Clientset{}, rr)

	pipelineInfo := &model.PipelineInfo{
		RadixApplication:   ra,
		TargetEnvironments: []string{"dev"},
	}

	err := cli.Run(context.Background(), pipelineInfo)
	s.Require().NoError(err)
	rds, _ := s.radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(anyAppName, "dev")).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(rds.Items, 1)
	rd := rds.Items[0]
	expectedHash := internaltest.GetBuildSecretHash(nil)
	s.Equal(expectedHash, rd.Annotations[kube.RadixBuildSecretHash])
}

func (s *deployTestSuite) TestDeploy_RadixCommitLabel_FromGitCommitHashIfSet() {
	anyAppName, anyJobName, anyImageTag := "anyapp", "anyjobname", "anyimagetag"

	rr := utils.ARadixRegistration().
		WithName(anyAppName).
		BuildRR()

	ra := utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment("dev", "master").
		WithComponents(utils.AnApplicationComponent().WithName("app")).
		BuildRA()

	namespaceWatcher := watcher.NewMockNamespaceWatcher(s.ctrl)
	namespaceWatcher.EXPECT().WaitFor(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	radixDeploymentWatcher := watcher.NewMockRadixDeploymentWatcher(s.ctrl)
	radixDeploymentWatcher.EXPECT().WaitForActive(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	cli := deploy.NewDeployStep(namespaceWatcher, radixDeploymentWatcher)
	cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, &monitoring.Clientset{}, rr)

	const commitID = "222ca8595c5283a9d0f17a623b9255a0d9866a2e"

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			JobName:  anyJobName,
			ImageTag: anyImageTag,
			Branch:   "master",
			CommitID: "anycommitid",
		},
		RadixApplication:   ra,
		TargetEnvironments: []string{"dev"},
		GitCommitHash:      commitID,
		GitTags:            "",
	}

	err := cli.Run(context.Background(), pipelineInfo)
	s.Require().NoError(err)
	rds, _ := s.radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(anyAppName, "dev")).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(rds.Items, 1)
	rd := rds.Items[0]
	s.Equal(commitID, rd.ObjectMeta.Labels[kube.RadixCommitLabel])
}

func (s *deployTestSuite) TestDeploy_RadixCommitLabel_FromCommtIdIfGitCommitHashEmpty() {
	anyAppName, anyJobName, anyImageTag := "anyapp", "anyjobname", "anyimagetag"

	rr := utils.ARadixRegistration().
		WithName(anyAppName).
		BuildRR()

	ra := utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment("dev", "master").
		WithComponents(utils.AnApplicationComponent().WithName("app")).
		BuildRA()

	namespaceWatcher := watcher.NewMockNamespaceWatcher(s.ctrl)
	namespaceWatcher.EXPECT().WaitFor(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	radixDeploymentWatcher := watcher.NewMockRadixDeploymentWatcher(s.ctrl)
	radixDeploymentWatcher.EXPECT().WaitForActive(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	cli := deploy.NewDeployStep(namespaceWatcher, radixDeploymentWatcher)
	cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, &monitoring.Clientset{}, rr)

	const commitID = "222ca8595c5283a9d0f17a623b9255a0d9866a2e"

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			JobName:  anyJobName,
			ImageTag: anyImageTag,
			Branch:   "master",
			CommitID: commitID,
		},
		RadixApplication:   ra,
		TargetEnvironments: []string{"dev"},
		GitCommitHash:      "",
		GitTags:            "",
	}

	err := cli.Run(context.Background(), pipelineInfo)
	s.Require().NoError(err)
	rds, _ := s.radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(anyAppName, "dev")).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(rds.Items, 1)
	rd := rds.Items[0]
	s.Equal(commitID, rd.ObjectMeta.Labels[kube.RadixCommitLabel])
}

func (s *deployTestSuite) TestDeploy_WaitActiveDeployment() {
	anyAppName, anyJobName, anyImageTag := "anyapp", "anyjobname", "anyimagetag"

	rr := utils.ARadixRegistration().
		WithName(anyAppName).
		BuildRR()

	envName := "dev"
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(envName, "master").
		WithComponents(utils.AnApplicationComponent().WithName("app")).
		BuildRA()

	type scenario struct {
		name                     string
		watcherError             error
		expectedRadixDeployments int
	}
	scenarios := []scenario{
		{name: "No fail", watcherError: nil, expectedRadixDeployments: 1},
		{name: "Watch fails", watcherError: errors.New("some error"), expectedRadixDeployments: 0},
	}
	for _, ts := range scenarios {
		s.Run(ts.name, func() {
			s.SetupTest()
			namespace := utils.GetEnvironmentNamespace(anyAppName, envName)
			namespaceWatcher := watcher.NewMockNamespaceWatcher(s.ctrl)
			namespaceWatcher.EXPECT().WaitFor(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			radixDeploymentWatcher := watcher.NewMockRadixDeploymentWatcher(s.ctrl)
			radixDeploymentWatcher.EXPECT().WaitForActive(context.Background(), namespace, radixDeploymentNameMatcher{envName: envName, imageTag: anyImageTag}).Return(ts.watcherError)
			cli := deploy.NewDeployStep(namespaceWatcher, radixDeploymentWatcher)
			cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, &monitoring.Clientset{}, rr)

			pipelineInfo := &model.PipelineInfo{
				PipelineArguments: model.PipelineArguments{
					JobName:  anyJobName,
					ImageTag: anyImageTag,
					Branch:   "master",
				},
				TargetEnvironments: []string{envName},
				RadixApplication:   ra,
			}

			err := cli.Run(context.Background(), pipelineInfo)
			if ts.watcherError == nil {
				s.NoError(err)
			} else {
				s.EqualError(err, ts.watcherError.Error())
			}
			rdList, err := s.radixClient.RadixV1().RadixDeployments(namespace).List(context.Background(), metav1.ListOptions{})
			s.Require().NoError(err)
			s.Len(rdList.Items, ts.expectedRadixDeployments, "Invalid expected RadixDeployment-s count")
		})
	}
}

type radixDeploymentNameMatcher struct {
	envName  string
	imageTag string
}

func (m radixDeploymentNameMatcher) Matches(name interface{}) bool {
	rdName, ok := name.(string)
	return ok && strings.HasPrefix(rdName, m.String())
}
func (m radixDeploymentNameMatcher) String() string {
	return fmt.Sprintf("%s-%s", m.envName, m.imageTag)
}

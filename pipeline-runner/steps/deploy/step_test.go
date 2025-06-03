package deploy_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pipeline-runner/internal/watcher"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/deploy"
	internaltest "github.com/equinor/radix-operator/pipeline-runner/steps/internal/test"
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
	"github.com/stretchr/testify/assert"
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
		TargetEnvironments: []model.TargetEnvironment{},
	}
	namespaceWatcher := watcher.NewMockNamespaceWatcher(s.ctrl)
	namespaceWatcher.EXPECT().WaitFor(gomock.Any(), gomock.Any()).Times(0)
	radixDeploymentWatcher := watcher.NewMockRadixDeploymentWatcher(s.ctrl)
	radixDeploymentWatcher.EXPECT().WaitForActive(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	cli := deploy.NewDeployStep(namespaceWatcher, radixDeploymentWatcher)
	cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, nil, rr)
	err := cli.Run(context.Background(), pipelineInfo)
	s.Require().NoError(err)
}

func (s *deployTestSuite) TestDeploy_PromotionSetup_ShouldCreateNamespacesForAllBranchesIfNotExists() {
	appName, envName, branch, jobName, imageTag, commitId, gitTags, gitRef, gitRefType := "anyapp", "dev", "master", "anyjobname", "anyimagetag", "anycommit", "gittags", "anytag", "tag"
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
			JobName:    jobName,
			ImageTag:   imageTag,
			GitRef:     gitRef,
			GitRefType: gitRefType,
		},
		RadixApplication:   ra,
		TargetEnvironments: []model.TargetEnvironment{{Environment: envName}},
		GitCommitHash:      commitId,
		GitTags:            gitTags,
	}

	namespaceWatcher := watcher.NewMockNamespaceWatcher(s.ctrl)
	namespaceWatcher.EXPECT().WaitFor(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	radixDeploymentWatcher := watcher.NewMockRadixDeploymentWatcher(s.ctrl)
	radixDeploymentWatcher.EXPECT().WaitForActive(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	cli := deploy.NewDeployStep(namespaceWatcher, radixDeploymentWatcher)
	cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, nil, rr)
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
		s.Empty(rdDev.Annotations[kube.RadixBranchAnnotation])
		s.NotEmpty(rdDev.Annotations[kube.RadixGitRefAnnotation])
		s.NotEmpty(rdDev.Annotations[kube.RadixGitRefTypeAnnotation])
		s.NotEmpty(rdDev.Labels[kube.RadixCommitLabel])
		s.NotEmpty(rdDev.Labels["radix-job-name"])
		s.Empty(rdDev.Annotations[kube.RadixBranchAnnotation])
		s.Equal(gitRef, rdDev.Annotations[kube.RadixGitRefAnnotation])
		s.Equal(gitRefType, rdDev.Annotations[kube.RadixGitRefTypeAnnotation])
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
	cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, &monitoring.Clientset{}, nil, rr)

	pipelineInfo := &model.PipelineInfo{
		RadixApplication:   ra,
		TargetEnvironments: []model.TargetEnvironment{{Environment: "dev"}},
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
	cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, &monitoring.Clientset{}, nil, rr)

	pipelineInfo := &model.PipelineInfo{
		RadixApplication:   ra,
		BuildSecret:        secret,
		TargetEnvironments: []model.TargetEnvironment{{Environment: "dev"}},
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
	cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, &monitoring.Clientset{}, nil, rr)

	pipelineInfo := &model.PipelineInfo{
		RadixApplication:   ra,
		TargetEnvironments: []model.TargetEnvironment{{Environment: "dev"}},
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
	cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, &monitoring.Clientset{}, nil, rr)

	const commitID = "222ca8595c5283a9d0f17a623b9255a0d9866a2e"

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			JobName:    anyJobName,
			ImageTag:   anyImageTag,
			GitRef:     "master",
			GitRefType: string(radixv1.GitRefBranch),
			CommitID:   "anycommitid",
		},
		RadixApplication:   ra,
		TargetEnvironments: []model.TargetEnvironment{{Environment: "dev"}},
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
	cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, &monitoring.Clientset{}, nil, rr)

	const commitID = "222ca8595c5283a9d0f17a623b9255a0d9866a2e"

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			JobName:    anyJobName,
			ImageTag:   anyImageTag,
			GitRef:     "master",
			GitRefType: string(radixv1.GitRefBranch),
			CommitID:   commitID,
		},
		RadixApplication:   ra,
		TargetEnvironments: []model.TargetEnvironment{{Environment: "dev"}},
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
			cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, &monitoring.Clientset{}, nil, rr)

			pipelineInfo := &model.PipelineInfo{
				PipelineArguments: model.PipelineArguments{
					JobName:    anyJobName,
					ImageTag:   anyImageTag,
					GitRef:     "master",
					GitRefType: string(radixv1.GitRefBranch),
				},
				TargetEnvironments: []model.TargetEnvironment{{Environment: envName}},
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

func (s *deployTestSuite) TestDeploy_CommandAndArgs() {
	const (
		anyAppName = "anyapp"
		env1       = "env1"
	)
	type scenario struct {
		command          []string
		args             []string
		envConfigCommand *[]string
		envConfigArgs    *[]string
		expectedCommand  []string
		expectedArgs     []string
	}
	scenarios := map[string]scenario{
		"command and args are not set": {
			command:         nil,
			args:            nil,
			expectedCommand: nil,
			expectedArgs:    nil,
		},
		"single command is set": {
			command:         []string{"bash"},
			args:            nil,
			expectedCommand: []string{"bash"},
			expectedArgs:    nil,
		},
		"command with arguments is set": {
			command:         []string{"sh", "-c", "echo hello"},
			args:            nil,
			expectedCommand: []string{"sh", "-c", "echo hello"},
			expectedArgs:    nil,
		},
		"command is set and args are set": {
			command:         []string{"sh", "-c"},
			args:            []string{"echo hello"},
			expectedCommand: []string{"sh", "-c"},
			expectedArgs:    []string{"echo hello"},
		},
		"only args are set": {
			command:         nil,
			args:            []string{"--verbose", "--output=json"},
			expectedCommand: nil,
			expectedArgs:    []string{"--verbose", "--output=json"},
		},
		"single command is set to env-config": {
			envConfigCommand: &[]string{"bash"},
			envConfigArgs:    nil,
			expectedCommand:  []string{"bash"},
			expectedArgs:     nil,
		},
		"command with arguments is set to env-config": {
			envConfigCommand: &[]string{"sh", "-c", "echo hello"},
			envConfigArgs:    nil,
			expectedCommand:  []string{"sh", "-c", "echo hello"},
			expectedArgs:     nil,
		},
		"command is set and args are set to env-config": {
			envConfigCommand: &[]string{"sh", "-c"},
			envConfigArgs:    &[]string{"echo hello"},
			expectedCommand:  []string{"sh", "-c"},
			expectedArgs:     []string{"echo hello"},
		},
		"only args are set to env-config": {
			envConfigCommand: nil,
			envConfigArgs:    &[]string{"--verbose", "--output=json"},
			expectedCommand:  nil,
			expectedArgs:     []string{"--verbose", "--output=json"},
		},
		"override by single command is set to env-config": {
			command:          []string{"sh"},
			envConfigCommand: &[]string{"bash"},
			envConfigArgs:    nil,
			expectedCommand:  []string{"bash"},
			expectedArgs:     nil,
		},
		"override by command with arguments is set to env-config": {
			command:          []string{"ls", "-ls", "/"},
			envConfigCommand: &[]string{"sh", "-c", "echo hello"},
			envConfigArgs:    nil,
			expectedCommand:  []string{"sh", "-c", "echo hello"},
			expectedArgs:     nil,
		},
		"override by command is set and args are set to env-config": {
			command:          []string{"ls", "-ls"},
			args:             []string{"/"},
			envConfigCommand: &[]string{"sh", "-c"},
			envConfigArgs:    &[]string{"echo hello"},
			expectedCommand:  []string{"sh", "-c"},
			expectedArgs:     []string{"echo hello"},
		},
		"override by only args are set to env-config": {
			command:          nil,
			args:             []string{"--format", "yaml"},
			envConfigCommand: nil,
			envConfigArgs:    &[]string{"--verbose", "--output=json"},
			expectedCommand:  nil,
			expectedArgs:     []string{"--verbose", "--output=json"},
		},
		"combine args and set command in env-config": {
			command:          nil,
			args:             []string{"echo hello"},
			envConfigCommand: &[]string{"sh", "-c"},
			envConfigArgs:    nil,
			expectedCommand:  []string{"sh", "-c"},
			expectedArgs:     []string{"echo hello"},
		},
		"combine command and set args in env-config": {
			command:          []string{"sh", "-c"},
			args:             nil,
			envConfigCommand: nil,
			envConfigArgs:    &[]string{"echo hello"},
			expectedCommand:  []string{"sh", "-c"},
			expectedArgs:     []string{"echo hello"},
		},
	}

	for name, ts := range scenarios {
		s.T().Run(name, func(t *testing.T) {
			s.SetupTest()
			rr := utils.ARadixRegistration().WithName(anyAppName).BuildRR()

			comp1Builder := utils.AnApplicationComponent().WithName("comp1").WithCommand(ts.command).WithArgs(ts.args)
			job1Builder := utils.AnApplicationJobComponent().WithName("job1").WithCommand(ts.command).WithArgs(ts.args)
			if ts.envConfigCommand != nil || ts.envConfigArgs != nil {
				component1EnvBuilder := utils.NewComponentEnvironmentBuilder().WithEnvironment(env1)
				job1EnvBuilder := utils.NewJobComponentEnvironmentBuilder().WithEnvironment(env1)
				if ts.envConfigCommand != nil {
					component1EnvBuilder = component1EnvBuilder.WithCommand(*ts.envConfigCommand)
					job1EnvBuilder = job1EnvBuilder.WithCommand(*ts.envConfigCommand)
				}
				if ts.envConfigArgs != nil {
					component1EnvBuilder = component1EnvBuilder.WithArgs(*ts.envConfigArgs)
					job1EnvBuilder = job1EnvBuilder.WithArgs(*ts.envConfigArgs)
				}
				comp1Builder = comp1Builder.WithEnvironmentConfigs(component1EnvBuilder)
				job1Builder = job1Builder.WithEnvironmentConfigs(job1EnvBuilder)
			}
			ra := utils.NewRadixApplicationBuilder().WithAppName(anyAppName).
				WithEnvironment(env1, "master").
				WithComponents(
					comp1Builder,
					utils.AnApplicationComponent().WithName("comp2"),
				).
				WithJobComponents(
					job1Builder,
					utils.AnApplicationJobComponent().WithName("job2"),
				).
				BuildRA()

			namespaceWatcher := watcher.NewMockNamespaceWatcher(s.ctrl)
			namespaceWatcher.EXPECT().WaitFor(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			radixDeploymentWatcher := watcher.NewMockRadixDeploymentWatcher(s.ctrl)
			radixDeploymentWatcher.EXPECT().WaitForActive(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
			cli := deploy.NewDeployStep(namespaceWatcher, radixDeploymentWatcher)
			cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, &monitoring.Clientset{}, nil, rr)

			pipelineInfo := &model.PipelineInfo{
				PipelineArguments: model.PipelineArguments{
					JobName:    "any-job-name",
					ImageTag:   "any-image-tag",
					GitRef:     "master",
					GitRefType: string(radixv1.GitRefBranch),
				},
				RadixApplication:   ra,
				TargetEnvironments: []model.TargetEnvironment{{Environment: env1}},
			}

			err := cli.Run(context.Background(), pipelineInfo)
			s.Require().NoError(err)
			rds, _ := s.radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(anyAppName, env1)).List(context.Background(), metav1.ListOptions{})
			s.Require().Len(rds.Items, 1)

			rd := rds.Items[0]

			component1 := rd.GetComponentByName("comp1")
			assert.Equal(t, ts.expectedCommand, component1.GetCommand(), "command in component1 should match in RadixDeployment")
			assert.Equal(t, ts.expectedArgs, component1.GetArgs(), "args in component1 should match in RadixDeployment")

			component2 := rd.GetComponentByName("comp2")
			assert.Empty(t, component2.GetCommand(), "command in component2 should be empty")
			assert.Empty(t, component2.GetArgs(), "args in component2 should be empty")

			job1 := rd.GetJobComponentByName("job1")
			assert.Equal(t, ts.expectedCommand, job1.GetCommand(), "command in job 1 should match in RadixDeployment")
			assert.Equal(t, ts.expectedArgs, job1.GetArgs(), "args in job 1 should match in RadixDeployment")

			job2 := rd.GetJobComponentByName("job2")
			assert.Empty(t, job2.GetCommand(), "command in job 2 should be empty")
			assert.Empty(t, job2.GetArgs(), "args in job 2 should be empty")
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

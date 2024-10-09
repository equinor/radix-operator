package deployconfig_test

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/internal/watcher"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/deploy"
	"github.com/equinor/radix-operator/pipeline-runner/steps/deployconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func Test_RunDeployConfigTestSuite(t *testing.T) {
	suite.Run(t, new(deployConfigTestSuite))
}

type deployConfigTestSuite struct {
	suite.Suite
	kubeClient  *kubefake.Clientset
	radixClient *radix.Clientset
	kedaClient  *kedafake.Clientset
	promClient  *prometheusfake.Clientset
	kubeUtil    *kube.Kube
	ctrl        *gomock.Controller
}

func (s *deployConfigTestSuite) SetupTest() {
	s.kubeClient = kubefake.NewSimpleClientset()
	s.radixClient = radix.NewSimpleClientset()
	s.kedaClient = kedafake.NewSimpleClientset()
	s.promClient = prometheusfake.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, s.kedaClient, nil)
	s.ctrl = gomock.NewController(s.T())
}

func (s *deployConfigTestSuite) Test_EmptyTargetEnvironments_SkipDeployment() {
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

func (s *deployConfigTestSuite) TestDeploy_() {
	const appName, envName, branch, jobName, imageTag, commitId, gitTags = "anyapp", "dev", "master", "anyjobname", "anyimagetag", "anycommit", "gittags"
	rr := utils.ARadixRegistration().
		WithName(appName).
		BuildRR()

	ra := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithEnvironment(envName, branch).
		WithEnvironment("prod", "").
		WithDNSExternalAlias("alias1", envName, "app", false).
		WithComponents(
			utils.AnApplicationComponent().WithName("app"),
		).
		BuildRA()

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			JobName: jobName,
		},
		RadixApplication: ra,
		GitCommitHash:    commitId,
		GitTags:          gitTags,
	}

	namespaceWatcher := watcher.NewMockNamespaceWatcher(s.ctrl)
	namespaceWatcher.EXPECT().WaitFor(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	radixDeploymentWatcher := watcher.NewMockRadixDeploymentWatcher(s.ctrl)
	radixDeploymentWatcher.EXPECT().WaitForActive(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	cli := deployconfig.NewDeployConfigStep(namespaceWatcher, radixDeploymentWatcher)
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

	s.Run("validate dns app alias", func() {
		s.True(rdDev.Spec.Components[0].DNSAppAlias)
		s.False(rdDev.Spec.Components[1].DNSAppAlias)
	})
}

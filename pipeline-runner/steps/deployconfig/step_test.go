package deployconfig_test

import (
	"context"
	"testing"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pipeline-runner/internal/watcher"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/deploy"
	"github.com/equinor/radix-operator/pipeline-runner/steps/deployconfig"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
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
	radixClient *radixfake.Clientset
	kedaClient  *kedafake.Clientset
	promClient  *prometheusfake.Clientset
	kubeUtil    *kube.Kube
	ctrl        *gomock.Controller
}

func (s *deployConfigTestSuite) SetupTest() {
	s.kubeClient = kubefake.NewSimpleClientset()
	s.radixClient = radixfake.NewSimpleClientset()
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

type scenario struct {
	name                            string
	raBuilder                       utils.ApplicationBuilder
	existingRadixDeploymentBuilders []utils.DeploymentBuilder
	expectedRadixDeploymentBuilders []utils.DeploymentBuilder
}

func (s *deployConfigTestSuite) TestDeployConfig() {
	const (
		appName    = "anyapp"
		env1       = "env1"
		env2       = "env2"
		component1 = "component1"
		component2 = "component2"
		alias1     = "alias1"
		alias2     = "alias2"
		jobName    = "anyjobname"
		commitId   = "anycommit"
		gitTags    = "gittags"
	)
	rr := utils.ARadixRegistration().WithName(appName).BuildRR()

	scenarios := []scenario{
		func() scenario {
			ts := scenario{
				name: "No active deployments",
				raBuilder: utils.NewRadixApplicationBuilder().WithAppName(appName).
					WithEnvironment(env1, "main").
					WithEnvironment(env2, "").
					WithComponents(
						utils.AnApplicationComponent().WithName(component1),
					).
					WithDNSExternalAlias(alias1, env1, component1, false),
			}
			ts.existingRadixDeploymentBuilders = nil
			ts.expectedRadixDeploymentBuilders = nil
			return ts
		}(),
	}

	for _, ts := range scenarios {
		s.T().Run(ts.name, func(t *testing.T) {
			ra := ts.raBuilder.BuildRA()
			pipelineInfo := &model.PipelineInfo{
				PipelineArguments: model.PipelineArguments{
					JobName: jobName,
				},
				RadixApplication: ra,
				GitCommitHash:    commitId,
				GitTags:          gitTags,
			}

			s.SetupTest()

			for _, radixDeploymentBuilder := range ts.existingRadixDeploymentBuilders {
				rd := radixDeploymentBuilder.BuildRD()
				_, err := s.radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(rd.Spec.AppName, rd.Spec.Environment)).Create(context.Background(), rd, metav1.CreateOptions{})
				s.Require().NoError(err)
			}

			namespaceWatcher := watcher.NewMockNamespaceWatcher(s.ctrl)
			namespaceWatcher.EXPECT().WaitFor(gomock.Any(), gomock.Any()).Return(nil).Times(len(ra.Spec.Environments))
			radixDeploymentWatcher := watcher.NewMockRadixDeploymentWatcher(s.ctrl)
			radixDeploymentWatcher.EXPECT().WaitForActive(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(len(ts.expectedRadixDeploymentBuilders))

			cli := deployconfig.NewDeployConfigStep(namespaceWatcher, radixDeploymentWatcher)
			cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
			err := cli.Run(context.Background(), pipelineInfo)
			s.Require().NoError(err)

			rds, _ := s.radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(appName, env1)).List(context.Background(), metav1.ListOptions{})
			if !s.Assert().Len(rds.Items, len(ts.expectedRadixDeploymentBuilders)) {
				return
			}
			rdsByEnvMap := slice.Reduce(rds.Items, make(map[string]radixv1.RadixDeployment), func(acc map[string]radixv1.RadixDeployment, rd radixv1.RadixDeployment) map[string]radixv1.RadixDeployment {
				acc[rd.Spec.Environment] = rd
				return acc
			})

			for _, builder := range ts.expectedRadixDeploymentBuilders {
				expectedRd := builder.BuildRD()
				actualRd, ok := rdsByEnvMap[expectedRd.Spec.Environment]
				if !s.True(ok, "Expected RadixDeployment for environment %s not found", expectedRd.Spec.Environment) {
					continue
				}
				if s.Len(actualRd.Spec.Components, len(expectedRd.Spec.Components), "Invalid number of components") {
					for i := 0; i < len(expectedRd.Spec.Components); i++ {
						s.Equal(expectedRd.Spec.Components[i].Name, actualRd.Spec.Components[i].Name)
						for envVarName, envVarValue := range expectedRd.Spec.Components[i].EnvironmentVariables {
							s.Equal(envVarValue, actualRd.Spec.Components[i].EnvironmentVariables[envVarName], "Invalid or missing an environment variable %s for the env %s and component %s", envVarName, expectedRd.Spec.Environment, expectedRd.Spec.Components[i].Name)
						}
						for j := 0; j < len(expectedRd.Spec.Components[i].ExternalDNS); j++ {
							s.Equal(expectedRd.Spec.Components[i].ExternalDNS[j].UseCertificateAutomation, actualRd.Spec.Components[i].ExternalDNS[j].UseCertificateAutomation, "Invalid external UseCertificateAutomation for the env %s and component %s", expectedRd.Spec.Environment, expectedRd.Spec.Components[i].Name)
							s.Equal(expectedRd.Spec.Components[i].ExternalDNS[j].FQDN, actualRd.Spec.Components[i].ExternalDNS[j].FQDN, "Invalid external FQDN for the env %s and component %s", expectedRd.Spec.Environment, expectedRd.Spec.Components[i].Name)
						}
					}
				}
				expectedAnnotations := expectedRd.GetAnnotations()
				actualAnnotations := actualRd.GetAnnotations()
				if s.Len(actualAnnotations, len(expectedAnnotations), "Invalid number of annotations") {
					for key, value := range expectedAnnotations {
						s.Equal(expectedAnnotations[key], actualAnnotations[value], "Invalid or missing an annotation %s or value %s", key, value)
					}
				}
				expectedLabels := expectedRd.GetLabels()
				actualLabels := actualRd.GetLabels()
				if s.Len(actualLabels, len(expectedLabels), "Invalid number of labels") {
					for key, value := range expectedLabels {
						s.Equal(expectedLabels[key], actualLabels[value], "Invalid or missing an label %s or value %s", key, value)
					}
				}
				// TODO check other props
			}
		})
	}
}

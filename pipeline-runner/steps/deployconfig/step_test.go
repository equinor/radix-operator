package deployconfig_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pipeline-runner/internal/watcher"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/deployconfig"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
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

type scenario struct {
	name                                  string
	existingRaBuilder                     utils.ApplicationBuilder
	applyingRaBuilder                     utils.ApplicationBuilder
	existingActiveRadixDeploymentBuilders []utils.DeploymentBuilder
	expectedActiveRadixDeploymentBuilders []utils.DeploymentBuilder
	affectedEnvs                          []string
}

func (s *deployConfigTestSuite) TestDeployConfig() {
	const (
		appName          = "anyapp"
		env1             = "env1"
		env2             = "env2"
		component1       = "component1"
		component2       = "component2"
		alias1           = "alias1"
		alias2           = "alias2"
		alias3           = "alias3"
		jobName          = "anyjobname"
		commitId         = "anycommit"
		gitTags          = "gittags"
		branch1          = "branch1"
		existingImageTag = "existingImageTag"
		appliedImageTag  = "appliedImageTag"
	)
	rr := utils.ARadixRegistration().WithName(appName).BuildRR()
	timeNow := time.Now()
	scenarios := []scenario{
		{
			name: "No active deployments, no RA changes",
			existingRaBuilder: utils.NewRadixApplicationBuilder().WithAppName(appName).
				WithEnvironment(env1, branch1).
				WithEnvironment(env2, "").
				WithComponents(
					utils.AnApplicationComponent().WithName(component1),
				).
				WithDNSExternalAlias(alias1, env1, component1, false),
			applyingRaBuilder: utils.NewRadixApplicationBuilder().WithAppName(appName).
				WithEnvironment(env1, branch1).
				WithEnvironment(env2, "").
				WithComponents(
					utils.AnApplicationComponent().WithName(component1),
				).
				WithDNSExternalAlias(alias1, env1, component1, false),

			existingActiveRadixDeploymentBuilders: nil,
			expectedActiveRadixDeploymentBuilders: nil,
			affectedEnvs:                          nil,
		},
		{
			name: "Deleted environment",
			existingRaBuilder: utils.NewRadixApplicationBuilder().WithAppName(appName).
				WithEnvironment(env1, branch1).
				WithEnvironment(env2, "").
				WithComponents(
					utils.AnApplicationComponent().WithName(component1).WithImageTagName(existingImageTag),
				).
				WithDNSExternalAlias(alias1, env1, component1, false).
				WithDNSExternalAlias(alias3, env2, component1, false),
			applyingRaBuilder: utils.NewRadixApplicationBuilder().WithAppName(appName).
				WithEnvironment(env1, "").
				WithComponents(
					utils.AnApplicationComponent().WithName(component1),
				).
				WithDNSExternalAlias(alias2, env1, component1, false),

			existingActiveRadixDeploymentBuilders: []utils.DeploymentBuilder{
				utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(env1).WithImageTag(existingImageTag).WithComponent(
					utils.NewDeployComponentBuilder().WithName(component1).WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: alias1, UseCertificateAutomation: false})).
					WithActiveFrom(timeNow.Add(-time.Hour * 24 * 7)),
				utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(env2).WithImageTag(existingImageTag).WithComponent(
					utils.NewDeployComponentBuilder().WithName(component1).WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: alias3, UseCertificateAutomation: false})).
					WithActiveFrom(timeNow.Add(-time.Hour * 24 * 7)),
			},
			expectedActiveRadixDeploymentBuilders: []utils.DeploymentBuilder{
				utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(env1).WithImageTag(existingImageTag).WithComponent(
					utils.NewDeployComponentBuilder().WithName(component1).WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: alias1, UseCertificateAutomation: false})).
					WithActiveFrom(timeNow.Add(-time.Hour * 24 * 7)),
				utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(env2).WithImageTag(existingImageTag).WithComponent(
					utils.NewDeployComponentBuilder().WithName(component1).WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: alias3, UseCertificateAutomation: false})).
					WithActiveFrom(timeNow.Add(-time.Hour * 24 * 7)),
				utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(env1).WithImageTag(appliedImageTag).WithComponent(
					utils.NewDeployComponentBuilder().WithName(component1).WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: alias2, UseCertificateAutomation: false})),
			},
			affectedEnvs: []string{env1},
		},
		{
			name: "Orphaned environment, no active deployments, no new deployments",
			existingRaBuilder: utils.NewRadixApplicationBuilder().WithAppName(appName).
				WithEnvironment(env1, branch1).
				WithEnvironment(env2, "").
				WithComponents(
					utils.AnApplicationComponent().WithName(component1),
				).
				WithDNSExternalAlias(alias1, env1, component1, false),
			applyingRaBuilder: utils.NewRadixApplicationBuilder().WithAppName(appName).
				WithEnvironment(env2, "").
				WithComponents(
					utils.AnApplicationComponent().WithName(component1),
				).
				WithDNSExternalAlias(alias1, env1, component1, false),
			existingActiveRadixDeploymentBuilders: nil,
			expectedActiveRadixDeploymentBuilders: nil,
			affectedEnvs:                          nil,
		},
	}

	for _, ts := range scenarios {
		s.T().Run(ts.name, func(t *testing.T) {
			t.Logf("Running test case '%s'", ts.name)
			existingRa := ts.existingRaBuilder.BuildRA()
			_, err := s.radixClient.RadixV1().RadixApplications(utils.GetAppNamespace(existingRa.Name)).Create(context.Background(), existingRa, metav1.CreateOptions{})
			s.Require().NoError(err)
			pipelineInfo := &model.PipelineInfo{
				PipelineArguments: model.PipelineArguments{
					JobName:  jobName,
					ImageTag: appliedImageTag,
				},
				RadixApplication: ts.applyingRaBuilder.BuildRA(),
				GitCommitHash:    commitId,
				GitTags:          gitTags,
			}

			s.SetupTest()

			for _, radixDeploymentBuilder := range ts.existingActiveRadixDeploymentBuilders {
				rd := radixDeploymentBuilder.BuildRD()
				namespace := utils.GetEnvironmentNamespace(rd.Spec.AppName, rd.Spec.Environment)
				_, err := s.kubeUtil.KubeClient().CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}, metav1.CreateOptions{})
				s.Require().NoError(err)
				_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
				s.Require().NoError(err)
			}

			namespaceWatcher := watcher.NewMockNamespaceWatcher(s.ctrl)
			radixDeploymentWatcher := watcher.NewMockRadixDeploymentWatcher(s.ctrl)
			namespaceWatcher.EXPECT().WaitFor(gomock.Any(), gomock.Any()).Return(nil).Times(len(ts.affectedEnvs))
			radixDeploymentWatcher.EXPECT().WaitForActive(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(len(ts.affectedEnvs))

			cli := deployconfig.NewDeployConfigStep(namespaceWatcher, radixDeploymentWatcher)
			cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
			if err = cli.Run(context.Background(), pipelineInfo); err != nil {
				t.Logf("Error: %v", err)
				s.Require().NoError(err)
			}

			rdsList, _ := s.radixClient.RadixV1().RadixDeployments("").List(context.Background(), metav1.ListOptions{})
			if !s.Assert().Len(rdsList.Items, len(ts.expectedActiveRadixDeploymentBuilders)) {
				return
			}
			rdsByEnvMap := slice.Reduce(rdsList.Items, make(map[string]radixv1.RadixDeployment), func(acc map[string]radixv1.RadixDeployment, rd radixv1.RadixDeployment) map[string]radixv1.RadixDeployment {
				acc[rd.Spec.Environment] = rd
				return acc
			})

			for _, builder := range ts.expectedActiveRadixDeploymentBuilders {
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
				s.ElementsMatch(strings.Split(expectedRd.GetName(), "-")[:2], strings.Split(actualRd.GetName(), "-")[:2], "Invalid name prefix parts")
				expectedAnnotations := expectedRd.GetAnnotations()
				actualAnnotations := actualRd.GetAnnotations()
				if s.Len(actualAnnotations, len(expectedAnnotations), "Invalid number of annotations") {
					for key, value := range expectedAnnotations {
						s.Equal(expectedAnnotations[key], actualAnnotations[key], "Invalid or missing an annotation %s or value %s", key, value)
					}
				}
				expectedLabels := expectedRd.GetLabels()
				actualLabels := actualRd.GetLabels()
				if s.Len(actualLabels, len(expectedLabels), "Invalid number of labels") {
					for key, value := range expectedLabels {
						s.Equal(expectedLabels[key], actualLabels[key], "Invalid or missing an label %s or value %s", key, value)
					}
				}
				// TODO check other props
			}
		})
	}
}

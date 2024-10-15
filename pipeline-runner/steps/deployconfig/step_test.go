package deployconfig_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/equinor/radix-common/utils/maps"
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
	name                                string
	existingRaProps                     raProps
	applyingRaProps                     raProps
	existingRadixDeploymentBuilderProps []radixDeploymentBuildersProps
	expectedRadixDeploymentBuilderProps []radixDeploymentBuildersProps
	affectedEnvs                        []string
}

type raProps struct {
	envs               []string
	componentNames     []string
	dnsExternalAliases []dnsExternalAlias
}

type dnsExternalAlias struct {
	alias                    string
	envName                  string
	componentName            string
	useCertificateAutomation bool
}

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

type radixDeploymentBuildersProps struct {
	envName      string
	imageTag     string
	activeFrom   time.Time
	externalDNSs map[string][]externalDNS
}

type externalDNS struct {
	fqdn                     string
	useCertificateAutomation bool
}

func (s *deployConfigTestSuite) TestDeployConfig() {
	rr := utils.ARadixRegistration().WithName(appName).BuildRR()
	timeNow := time.Now()
	inPast := timeNow.Add(-time.Hour * 24 * 7)
	scenarios := []scenario{
		{
			name: "No active deployments, no RA changes",
			existingRaProps: raProps{
				envs:           []string{env1, env2},
				componentNames: []string{component1},
				dnsExternalAliases: []dnsExternalAlias{
					{alias: alias1, envName: env1, componentName: component1, useCertificateAutomation: false},
				},
			},
			applyingRaProps: raProps{
				envs:           []string{env1, env2},
				componentNames: []string{component1},
				dnsExternalAliases: []dnsExternalAlias{
					{alias: alias1, envName: env1, componentName: component1, useCertificateAutomation: false},
				},
			},
		},
		{
			name: "Deleted environment",
			existingRaProps: raProps{
				envs:           []string{env1, env2},
				componentNames: []string{component1},
				dnsExternalAliases: []dnsExternalAlias{
					{alias: alias1, envName: env1, componentName: component1, useCertificateAutomation: false},
					{alias: alias3, envName: env2, componentName: component1, useCertificateAutomation: false},
				},
			},
			applyingRaProps: raProps{
				envs:           []string{env1},
				componentNames: []string{component1},
				dnsExternalAliases: []dnsExternalAlias{
					{alias: alias1, envName: env1, componentName: component1, useCertificateAutomation: false},
				},
			},
			existingRadixDeploymentBuilderProps: []radixDeploymentBuildersProps{
				{
					envName: env1, imageTag: existingImageTag, activeFrom: inPast,
					externalDNSs: map[string][]externalDNS{
						component1: {
							{fqdn: alias1, useCertificateAutomation: false},
						},
					},
				},
				{
					envName: env2, imageTag: existingImageTag, activeFrom: inPast,
					externalDNSs: map[string][]externalDNS{
						component1: {
							{fqdn: alias3, useCertificateAutomation: false},
						},
					},
				},
			},
			expectedRadixDeploymentBuilderProps: []radixDeploymentBuildersProps{
				{
					envName: env1, imageTag: existingImageTag, activeFrom: inPast,
					externalDNSs: map[string][]externalDNS{
						component1: {
							{fqdn: alias1, useCertificateAutomation: false},
						},
					},
				},
				{
					envName: env2, imageTag: existingImageTag, activeFrom: inPast,
					externalDNSs: map[string][]externalDNS{
						component1: {
							{fqdn: alias3, useCertificateAutomation: false},
						},
					},
				},
			},
		},
		{
			name: "Changed alias",
			existingRaProps: raProps{
				envs:           []string{env1, env2},
				componentNames: []string{component1},
				dnsExternalAliases: []dnsExternalAlias{
					{alias: alias1, envName: env1, componentName: component1, useCertificateAutomation: false},
					{alias: alias3, envName: env2, componentName: component1, useCertificateAutomation: false},
				},
			},
			applyingRaProps: raProps{
				envs:           []string{env1, env2},
				componentNames: []string{component1},
				dnsExternalAliases: []dnsExternalAlias{
					{alias: alias2, envName: env1, componentName: component1, useCertificateAutomation: false},
				},
			},
			existingRadixDeploymentBuilderProps: []radixDeploymentBuildersProps{
				{
					envName: env1, imageTag: existingImageTag, activeFrom: inPast,
					externalDNSs: map[string][]externalDNS{
						component1: {
							{fqdn: alias1, useCertificateAutomation: false},
						},
					},
				},
				{
					envName: env2, imageTag: existingImageTag, activeFrom: inPast,
					externalDNSs: map[string][]externalDNS{
						component1: {
							{fqdn: alias3, useCertificateAutomation: false},
						},
					},
				},
			},
			expectedRadixDeploymentBuilderProps: []radixDeploymentBuildersProps{
				{
					envName: env1, imageTag: existingImageTag, activeFrom: inPast,
					externalDNSs: map[string][]externalDNS{
						component1: {
							{fqdn: alias1, useCertificateAutomation: false},
						},
					},
				},
				{
					envName: env2, imageTag: existingImageTag, activeFrom: inPast,
					externalDNSs: map[string][]externalDNS{
						component1: {
							{fqdn: alias3, useCertificateAutomation: false},
						},
					},
				},
				{
					envName: env2, imageTag: appliedImageTag, activeFrom: inPast,
					externalDNSs: map[string][]externalDNS{
						component1: {
							{fqdn: alias2, useCertificateAutomation: false},
						},
					},
				},
			},
			affectedEnvs: []string{env1},
		},
		{
			name: "Deleted environment, no active deployments, no new deployments",
			existingRaProps: raProps{
				envs:           []string{env1, env2},
				componentNames: []string{component1},
				dnsExternalAliases: []dnsExternalAlias{
					{alias: alias1, envName: env1, componentName: component1, useCertificateAutomation: false},
				},
			},
			applyingRaProps: raProps{
				envs:           []string{env2},
				componentNames: []string{component1},
				dnsExternalAliases: []dnsExternalAlias{
					{alias: alias1, envName: env1, componentName: component1, useCertificateAutomation: false},
				},
			},
		},
	}

	for _, ts := range scenarios {
		s.T().Run(ts.name, func(t *testing.T) {
			t.Logf("Running test case '%s'", ts.name)
			s.createRadixApplication(ts.existingRaProps)
			pipelineInfo := &model.PipelineInfo{
				PipelineArguments: model.PipelineArguments{
					JobName:  jobName,
					ImageTag: appliedImageTag,
				},
				RadixApplication: buildRadixApplication(ts.applyingRaProps),
				GitCommitHash:    commitId,
				GitTags:          gitTags,
			}

			s.SetupTest()
			s.createUsedNamespaces(ts)
			s.createRadixDeployments(ts.existingRadixDeploymentBuilderProps)

			namespaceWatcher := watcher.NewMockNamespaceWatcher(s.ctrl)
			radixDeploymentWatcher := watcher.NewMockRadixDeploymentWatcher(s.ctrl)
			namespaceWatcher.EXPECT().WaitFor(gomock.Any(), gomock.Any()).Return(nil).Times(len(ts.affectedEnvs))
			radixDeploymentWatcher.EXPECT().WaitForActive(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(len(ts.affectedEnvs))

			cli := deployconfig.NewDeployConfigStep(namespaceWatcher, radixDeploymentWatcher)
			cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
			if err := cli.Run(context.Background(), pipelineInfo); err != nil {
				t.Logf("Error: %v", err)
				s.Require().NoError(err)
			}

			actualRdsByEnvMap, ok := s.getActualRadixDeploymentsByEnvMap(ts)
			if !ok {
				return
			}

			expectedRdList := s.buildRadixDeployments(ts.expectedRadixDeploymentBuilderProps)
			for _, expectedRd := range expectedRdList {
				actualRd, ok := actualRdsByEnvMap[expectedRd.Spec.Environment]
				if !s.True(ok, "Expected RadixDeployment for environment %s not found", expectedRd.Spec.Environment) {
					continue
				}
				if !s.Len(actualRd.Spec.Components, len(expectedRd.Spec.Components), "Invalid number of components") {
					continue
				}
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

func (s *deployConfigTestSuite) getActualRadixDeploymentsByEnvMap(ts scenario) (map[string]radixv1.RadixDeployment, bool) {
	rdsList, _ := s.radixClient.RadixV1().RadixDeployments("").List(context.Background(), metav1.ListOptions{})
	if !s.Assert().Len(rdsList.Items, len(ts.expectedRadixDeploymentBuilderProps)) {
		return nil, false
	}
	return slice.Reduce(rdsList.Items, make(map[string]radixv1.RadixDeployment), func(acc map[string]radixv1.RadixDeployment, rd radixv1.RadixDeployment) map[string]radixv1.RadixDeployment {
		acc[rd.Spec.Environment] = rd
		return acc
	}), true
}

func (s *deployConfigTestSuite) createRadixApplication(props raProps) {
	existingRa := buildRadixApplication(props)
	_, err := s.radixClient.RadixV1().RadixApplications(utils.GetAppNamespace(existingRa.Name)).Create(context.Background(), existingRa, metav1.CreateOptions{})
	s.Require().NoError(err)
}

func (s *deployConfigTestSuite) createRadixDeployments(deploymentBuildersProps []radixDeploymentBuildersProps) {
	rdList := s.buildRadixDeployments(deploymentBuildersProps)
	for _, rd := range rdList {
		_, err := s.radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(rd.Spec.AppName, rd.Spec.Environment)).Create(context.Background(), rd, metav1.CreateOptions{})
		s.Require().NoError(err)
	}
}

func (s *deployConfigTestSuite) buildRadixDeployments(deploymentBuildersProps []radixDeploymentBuildersProps) []*radixv1.RadixDeployment {
	var rdList []*radixv1.RadixDeployment
	for _, rdProps := range deploymentBuildersProps {
		builder := utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(rdProps.envName).WithImageTag(rdProps.imageTag).WithActiveFrom(rdProps.activeFrom)
		for componentName, externalDNSs := range rdProps.externalDNSs {
			componentBuilder := utils.NewDeployComponentBuilder().WithName(componentName)
			for _, compExternalDNS := range externalDNSs {
				componentBuilder = componentBuilder.WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: compExternalDNS.fqdn, UseCertificateAutomation: compExternalDNS.useCertificateAutomation})
			}
			builder = builder.WithComponent(componentBuilder)
		}
		rdList = append(rdList, builder.BuildRD())
	}
	return rdList
}

func (s *deployConfigTestSuite) createUsedNamespaces(ts scenario) {
	usedNamespacesSet := make(map[string]struct{})
	for _, envName := range ts.existingRaProps.envs {
		usedNamespacesSet[envName] = struct{}{}
	}
	for _, envName := range ts.applyingRaProps.envs {
		usedNamespacesSet[envName] = struct{}{}
	}
	usedNamespaces := maps.GetKeysFromMap(usedNamespacesSet)
	for _, namespace := range usedNamespaces {
		_, err := s.kubeClient.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: utils.GetEnvironmentNamespace(appName, namespace)}}, metav1.CreateOptions{})
		s.Require().NoError(err)
	}
}

func buildRadixApplication(props raProps) *radixv1.RadixApplication {
	existingRaBuilder := utils.NewRadixApplicationBuilder().WithAppName(appName)
	for _, env := range props.envs {
		existingRaBuilder.WithEnvironment(env, branch1)
	}
	for _, componentName := range props.componentNames {
		existingRaBuilder.WithComponents(utils.AnApplicationComponent().WithName(componentName).WithImageTagName(existingImageTag))
	}
	for _, item := range props.dnsExternalAliases {
		existingRaBuilder.WithDNSExternalAlias(item.alias, item.envName, item.componentName, item.useCertificateAutomation)
	}
	existingRa := existingRaBuilder.BuildRA()
	return existingRa
}

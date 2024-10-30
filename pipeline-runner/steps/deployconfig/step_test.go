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
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal"
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
	name                                   string
	existingRaProps                        raProps
	applyingRaProps                        raProps
	existingRadixDeploymentBuilderProps    []radixDeploymentBuildersProps
	expectedNewRadixDeploymentBuilderProps []radixDeploymentBuildersProps
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
	env3             = "env3"
	component1       = "component1"
	component2       = "component2"
	alias1           = "alias1"
	alias2           = "alias2"
	alias3           = "alias3"
	alias4           = "alias4"
	alias5           = "alias5"
	alias6           = "alias6"
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
	timeInPast := timeNow.Add(-time.Hour * 24 * 7)
	zeroTime := time.Time{}
	scenarios := []scenario{
		{
			name: "No DNSes changes, no deployments",
			existingRaProps: raProps{
				envs:           []string{env1, env2},
				componentNames: []string{component1},
				dnsExternalAliases: []dnsExternalAlias{
					{alias: alias1, envName: env1, componentName: component1, useCertificateAutomation: false},
					{alias: alias2, envName: env2, componentName: component2, useCertificateAutomation: true},
				},
			},
			applyingRaProps: raProps{
				envs:           []string{env1, env2},
				componentNames: []string{component1},
				dnsExternalAliases: []dnsExternalAlias{
					{alias: alias1, envName: env1, componentName: component1, useCertificateAutomation: false},
					{alias: alias2, envName: env2, componentName: component2, useCertificateAutomation: true},
				},
			},
			existingRadixDeploymentBuilderProps: []radixDeploymentBuildersProps{
				{
					envName: env1, imageTag: existingImageTag, activeFrom: timeInPast,
					externalDNSs: map[string][]externalDNS{component1: {{fqdn: alias1, useCertificateAutomation: false}}},
				},
			},
		},
		{
			name: "DNS exists for env, which has no active deployments, ignore this env",
			existingRaProps: raProps{
				envs:               []string{env1, env2},
				componentNames:     []string{component1},
				dnsExternalAliases: []dnsExternalAlias{},
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
			name: "DNSes added for two envs, one has no active deployments, deploy only second one",
			existingRaProps: raProps{
				envs:               []string{env1, env2},
				componentNames:     []string{component1},
				dnsExternalAliases: []dnsExternalAlias{},
			},
			applyingRaProps: raProps{
				envs:           []string{env1, env2},
				componentNames: []string{component1},
				dnsExternalAliases: []dnsExternalAlias{
					{alias: alias1, envName: env1, componentName: component1, useCertificateAutomation: false},
					{alias: alias2, envName: env2, componentName: component1, useCertificateAutomation: false},
				},
			},
			existingRadixDeploymentBuilderProps: []radixDeploymentBuildersProps{
				{
					envName: env2, imageTag: existingImageTag, activeFrom: timeInPast,
					externalDNSs: map[string][]externalDNS{},
				},
			},
			expectedNewRadixDeploymentBuilderProps: []radixDeploymentBuildersProps{
				{
					envName: env2, imageTag: appliedImageTag, activeFrom: zeroTime,
					externalDNSs: map[string][]externalDNS{component1: {{fqdn: alias2, useCertificateAutomation: false}}},
				},
			},
		},
		{
			name: "DNS changed in both of two envs, one has no active deployments, deploy only second one with change",
			existingRaProps: raProps{
				envs:           []string{env1, env2},
				componentNames: []string{component1},
				dnsExternalAliases: []dnsExternalAlias{
					{alias: alias1, envName: env1, componentName: component1, useCertificateAutomation: false},
					{alias: alias2, envName: env2, componentName: component1, useCertificateAutomation: false},
				},
			},
			applyingRaProps: raProps{
				envs:           []string{env1, env2},
				componentNames: []string{component1},
				dnsExternalAliases: []dnsExternalAlias{
					{alias: alias3, envName: env1, componentName: component1, useCertificateAutomation: false},
					{alias: alias4, envName: env2, componentName: component1, useCertificateAutomation: false},
				},
			},
			existingRadixDeploymentBuilderProps: []radixDeploymentBuildersProps{
				{
					envName: env1, imageTag: existingImageTag, activeFrom: timeInPast,
					externalDNSs: map[string][]externalDNS{component1: {{fqdn: alias1, useCertificateAutomation: false}}},
				},
			},
			expectedNewRadixDeploymentBuilderProps: []radixDeploymentBuildersProps{
				{
					envName: env1, imageTag: appliedImageTag, activeFrom: zeroTime,
					externalDNSs: map[string][]externalDNS{component1: {{fqdn: alias3, useCertificateAutomation: false}}},
				},
			},
		},
		{
			name: "DNSes deleted from two envs, one has no active deployments, deploy only second one",
			existingRaProps: raProps{
				envs:           []string{env1, env2},
				componentNames: []string{component1},
				dnsExternalAliases: []dnsExternalAlias{
					{alias: alias1, envName: env1, componentName: component1, useCertificateAutomation: false},
					{alias: alias2, envName: env2, componentName: component1, useCertificateAutomation: false},
				},
			},
			applyingRaProps: raProps{
				envs:               []string{env1, env2},
				componentNames:     []string{component1},
				dnsExternalAliases: []dnsExternalAlias{},
			},
			existingRadixDeploymentBuilderProps: []radixDeploymentBuildersProps{
				{
					envName: env2, imageTag: existingImageTag, activeFrom: timeInPast,
					externalDNSs: map[string][]externalDNS{component1: {{fqdn: alias2, useCertificateAutomation: false}}},
				},
			},
			expectedNewRadixDeploymentBuilderProps: []radixDeploymentBuildersProps{
				{
					envName: env2, imageTag: appliedImageTag, activeFrom: zeroTime,
					externalDNSs: map[string][]externalDNS{},
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
					envName: env1, imageTag: existingImageTag, activeFrom: timeInPast,
					externalDNSs: map[string][]externalDNS{component1: {{fqdn: alias1, useCertificateAutomation: false}}},
				},
				{
					envName: env2, imageTag: existingImageTag, activeFrom: timeInPast,
					externalDNSs: map[string][]externalDNS{component1: {{fqdn: alias3, useCertificateAutomation: false}}},
				},
			},
		},
		{
			name: "Added alias",
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
					{alias: alias1, envName: env1, componentName: component1, useCertificateAutomation: false}, // added alias
					{alias: alias2, envName: env1, componentName: component1, useCertificateAutomation: true},
					{alias: alias3, envName: env2, componentName: component1, useCertificateAutomation: false},
				},
			},
			existingRadixDeploymentBuilderProps: []radixDeploymentBuildersProps{
				{
					envName: env1, imageTag: existingImageTag, activeFrom: timeInPast,
					externalDNSs: map[string][]externalDNS{component1: {{fqdn: alias1, useCertificateAutomation: false}}},
				},
				{
					envName: env2, imageTag: existingImageTag, activeFrom: timeInPast,
					externalDNSs: map[string][]externalDNS{component1: {{fqdn: alias3, useCertificateAutomation: false}}},
				},
			},
			expectedNewRadixDeploymentBuilderProps: []radixDeploymentBuildersProps{
				{
					envName: env1, imageTag: appliedImageTag, activeFrom: zeroTime,
					externalDNSs: map[string][]externalDNS{component1: {
						{fqdn: alias1, useCertificateAutomation: false},
						{fqdn: alias2, useCertificateAutomation: true},
					}},
				},
			},
		},
		{
			name: "Changed alias in one environment",
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
					{alias: alias2, envName: env1, componentName: component1, useCertificateAutomation: false}, // renamed alias
					{alias: alias3, envName: env2, componentName: component1, useCertificateAutomation: false},
				},
			},
			existingRadixDeploymentBuilderProps: []radixDeploymentBuildersProps{
				{
					envName: env1, imageTag: existingImageTag, activeFrom: timeInPast,
					externalDNSs: map[string][]externalDNS{component1: {{fqdn: alias1, useCertificateAutomation: false}}},
				},
				{
					envName: env2, imageTag: existingImageTag, activeFrom: timeInPast,
					externalDNSs: map[string][]externalDNS{component1: {{fqdn: alias3, useCertificateAutomation: false}}},
				},
			},
			expectedNewRadixDeploymentBuilderProps: []radixDeploymentBuildersProps{
				{
					envName: env1, imageTag: appliedImageTag, activeFrom: zeroTime,
					externalDNSs: map[string][]externalDNS{component1: {{fqdn: alias2, useCertificateAutomation: false}}},
				},
			},
		},
		{
			name: "Changed alias in multiple environments",
			existingRaProps: raProps{
				envs:           []string{env1, env2, env3},
				componentNames: []string{component1},
				dnsExternalAliases: []dnsExternalAlias{
					{alias: alias1, envName: env1, componentName: component1, useCertificateAutomation: false},
					{alias: alias3, envName: env2, componentName: component1, useCertificateAutomation: false},
					{alias: alias5, envName: env3, componentName: component1, useCertificateAutomation: true},
				},
			},
			applyingRaProps: raProps{
				envs:           []string{env1, env2, env3},
				componentNames: []string{component1},
				dnsExternalAliases: []dnsExternalAlias{
					{alias: alias2, envName: env1, componentName: component1, useCertificateAutomation: false}, // renamed alias
					{alias: alias3, envName: env2, componentName: component1, useCertificateAutomation: false},
					{alias: alias6, envName: env3, componentName: component1, useCertificateAutomation: true}, // renamed alias
				},
			},
			existingRadixDeploymentBuilderProps: []radixDeploymentBuildersProps{
				{
					envName: env1, imageTag: existingImageTag, activeFrom: timeInPast,
					externalDNSs: map[string][]externalDNS{component1: {{fqdn: alias1, useCertificateAutomation: false}}},
				},
				{
					envName: env2, imageTag: existingImageTag, activeFrom: timeInPast,
					externalDNSs: map[string][]externalDNS{component1: {{fqdn: alias3, useCertificateAutomation: false}}},
				},
				{
					envName: env3, imageTag: existingImageTag, activeFrom: timeInPast,
					externalDNSs: map[string][]externalDNS{component1: {{fqdn: alias5, useCertificateAutomation: true}}},
				},
			},
			expectedNewRadixDeploymentBuilderProps: []radixDeploymentBuildersProps{
				{
					envName: env1, imageTag: appliedImageTag, activeFrom: zeroTime,
					externalDNSs: map[string][]externalDNS{component1: {{fqdn: alias2, useCertificateAutomation: false}}},
				},
				{
					envName: env3, imageTag: appliedImageTag, activeFrom: zeroTime,
					externalDNSs: map[string][]externalDNS{component1: {{fqdn: alias6, useCertificateAutomation: true}}},
				},
			},
		},
		{
			name: "Changed component in DNS",
			existingRaProps: raProps{
				envs:           []string{env1, env2},
				componentNames: []string{component1, component2},
				dnsExternalAliases: []dnsExternalAlias{
					{alias: alias1, envName: env1, componentName: component1, useCertificateAutomation: false},
					{alias: alias3, envName: env2, componentName: component1, useCertificateAutomation: false},
				},
			},
			applyingRaProps: raProps{
				envs:           []string{env1, env2},
				componentNames: []string{component1, component2},
				dnsExternalAliases: []dnsExternalAlias{
					{alias: alias1, envName: env1, componentName: component1, useCertificateAutomation: false},
					{alias: alias3, envName: env2, componentName: component2, useCertificateAutomation: false}, // changed component
				},
			},
			existingRadixDeploymentBuilderProps: []radixDeploymentBuildersProps{
				{
					envName: env1, imageTag: existingImageTag, activeFrom: timeInPast,
					externalDNSs: map[string][]externalDNS{component1: {{fqdn: alias1, useCertificateAutomation: false}}},
				},
				{
					envName: env2, imageTag: existingImageTag, activeFrom: timeInPast,
					externalDNSs: map[string][]externalDNS{component1: {{fqdn: alias3, useCertificateAutomation: false}}},
				},
			},
			expectedNewRadixDeploymentBuilderProps: []radixDeploymentBuildersProps{
				{
					envName: env2, imageTag: appliedImageTag, activeFrom: zeroTime,
					externalDNSs: map[string][]externalDNS{component2: {{fqdn: alias3, useCertificateAutomation: false}}},
				},
			},
		},
		{
			name: "Delete DNS",
			existingRaProps: raProps{
				envs:           []string{env1, env2},
				componentNames: []string{component1},
				dnsExternalAliases: []dnsExternalAlias{
					{alias: alias1, envName: env1, componentName: component1, useCertificateAutomation: false},
					{alias: alias3, envName: env2, componentName: component1, useCertificateAutomation: false}, // deleted
				},
			},
			applyingRaProps: raProps{
				envs:           []string{env1, env2},
				componentNames: []string{component1},
				dnsExternalAliases: []dnsExternalAlias{
					{alias: alias1, envName: env1, componentName: component1, useCertificateAutomation: false},
				},
			},
			existingRadixDeploymentBuilderProps: []radixDeploymentBuildersProps{
				{
					envName: env1, imageTag: existingImageTag, activeFrom: timeInPast,
					externalDNSs: map[string][]externalDNS{component1: {{fqdn: alias1, useCertificateAutomation: false}}},
				},
				{
					envName: env2, imageTag: existingImageTag, activeFrom: timeInPast,
					externalDNSs: map[string][]externalDNS{component1: {{fqdn: alias3, useCertificateAutomation: false}}},
				},
			},
			expectedNewRadixDeploymentBuilderProps: []radixDeploymentBuildersProps{
				{
					envName: env2, imageTag: appliedImageTag, activeFrom: zeroTime,
					externalDNSs: map[string][]externalDNS{},
				},
			},
		},
		{
			name: "Delete multiple DNSes",
			existingRaProps: raProps{
				envs:           []string{env1, env2},
				componentNames: []string{component1, component2},
				dnsExternalAliases: []dnsExternalAlias{
					{alias: alias1, envName: env1, componentName: component1, useCertificateAutomation: false},
					{alias: alias2, envName: env2, componentName: component1, useCertificateAutomation: false}, // deleted
					{alias: alias3, envName: env2, componentName: component2, useCertificateAutomation: false},
					{alias: alias4, envName: env2, componentName: component2, useCertificateAutomation: false}, // deleted
				},
			},
			applyingRaProps: raProps{
				envs:           []string{env1, env2},
				componentNames: []string{component1, component2},
				dnsExternalAliases: []dnsExternalAlias{
					{alias: alias1, envName: env1, componentName: component1, useCertificateAutomation: false}, // renamed alias
					{alias: alias3, envName: env2, componentName: component2, useCertificateAutomation: false},
				},
			},
			existingRadixDeploymentBuilderProps: []radixDeploymentBuildersProps{
				{
					envName: env1, imageTag: existingImageTag, activeFrom: timeInPast,
					externalDNSs: map[string][]externalDNS{component1: {{fqdn: alias1, useCertificateAutomation: false}}},
				},
				{
					envName: env2, imageTag: existingImageTag, activeFrom: timeInPast,
					externalDNSs: map[string][]externalDNS{
						component1: {
							{fqdn: alias2, useCertificateAutomation: false},
						},
						component2: {
							{fqdn: alias3, useCertificateAutomation: false},
							{fqdn: alias4, useCertificateAutomation: false},
						},
					},
				},
			},
			expectedNewRadixDeploymentBuilderProps: []radixDeploymentBuildersProps{
				{
					envName: env2, imageTag: appliedImageTag, activeFrom: zeroTime,
					externalDNSs: map[string][]externalDNS{component2: {{fqdn: alias3, useCertificateAutomation: false}}},
				},
			},
		},
		{
			name: "Deleted environment without alias",
			existingRaProps: raProps{
				envs:           []string{env1, env2},
				componentNames: []string{component1},
				dnsExternalAliases: []dnsExternalAlias{
					{alias: alias1, envName: env1, componentName: component1, useCertificateAutomation: false},
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
					envName: env1, imageTag: existingImageTag, activeFrom: timeInPast,
					externalDNSs: map[string][]externalDNS{component1: {{fqdn: alias1, useCertificateAutomation: false}}},
				},
				{
					envName: env2, imageTag: existingImageTag, activeFrom: timeInPast,
					externalDNSs: map[string][]externalDNS{component1: {{fqdn: alias3, useCertificateAutomation: false}}},
				},
			},
		},
		{
			name: "Deleted environment with alias",
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
					envName: env1, imageTag: existingImageTag, activeFrom: timeInPast,
					externalDNSs: map[string][]externalDNS{component1: {{fqdn: alias1, useCertificateAutomation: false}}},
				},
				{
					envName: env2, imageTag: existingImageTag, activeFrom: timeInPast,
					externalDNSs: map[string][]externalDNS{component1: {{fqdn: alias3, useCertificateAutomation: false}}},
				},
			},
		},
	}

	for _, ts := range scenarios {
		s.T().Run(ts.name, func(t *testing.T) {
			t.Logf("Running test case '%s'", ts.name)
			existingRa := s.createRadixApplication(ts.existingRaProps)
			pipelineInfo := &model.PipelineInfo{
				PipelineArguments: model.PipelineArguments{
					JobName:  jobName,
					ImageTag: appliedImageTag,
					ApplyConfigOptions: model.ApplyConfigOptions{
						DeployExternalDNS: true,
					},
				},
				RadixApplication: buildRadixApplication(ts.applyingRaProps),
				GitCommitHash:    commitId,
				GitTags:          gitTags,
			}

			s.SetupTest()
			s.createUsedNamespaces(ts)
			existingRdByEnvMap := s.createRadixDeployments(ts.existingRadixDeploymentBuilderProps, existingRa)
			expectedRdByEnvMap := getRadixDeploymentsByEnvMap(s.buildRadixDeployments(ts.expectedNewRadixDeploymentBuilderProps, pipelineInfo.RadixApplication, existingRdByEnvMap))
			affectedEnvs := maps.GetKeysFromMap(expectedRdByEnvMap)

			radixDeploymentWatcher := watcher.NewMockRadixDeploymentWatcher(s.ctrl)
			radixDeploymentWatcher.EXPECT().WaitForActive(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(len(affectedEnvs))

			cli := deployconfig.NewDeployConfigStep(radixDeploymentWatcher)
			cli.Init(context.Background(), s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, rr)
			if err := cli.Run(context.Background(), pipelineInfo); err != nil {
				t.Logf("Error: %v", err)
				s.Require().NoError(err)
			}

			actualNewRdsByEnvMap, ok := s.getActualNewRadixDeploymentsByEnvMap(ts)
			if !ok {
				return
			}

			for envName, expectedRdList := range expectedRdByEnvMap {
				actualRdList, ok := actualNewRdsByEnvMap[envName]
				if !ok {
					t.Logf("Error: No Radix Deployments found for environment %s", envName)
					continue
				}
				if !s.Len(actualRdList, len(expectedRdList), "Invalid number of Radix Deployments for environment %s", envName) {
					continue
				}
				for i := 0; i < len(expectedRdList); i++ {
					expectedRd := expectedRdList[i]
					actualRd := actualRdList[i]
					if !s.Len(actualRd.Spec.Components, len(expectedRd.Spec.Components), "Invalid number of components") {
						continue
					}
					for i := 0; i < len(expectedRd.Spec.Components); i++ {
						expectedComponent := expectedRd.Spec.Components[i]
						actualComponent := actualRd.Spec.Components[i]
						s.Equal(expectedComponent.Name, actualComponent.Name)
						for envVarName, envVarValue := range expectedComponent.EnvironmentVariables {
							s.Equal(envVarValue, actualComponent.EnvironmentVariables[envVarName], "Invalid or missing an environment variable %s for the env %s and component %s", envVarName, expectedRd.Spec.Environment, expectedComponent.Name)
						}
						if !s.Len(actualComponent.ExternalDNS, len(expectedComponent.ExternalDNS), "Invalid number of external DNS") {
							continue
						}
						for j := 0; j < len(expectedComponent.ExternalDNS); j++ {
							expectedExternalDNS := expectedComponent.ExternalDNS[j]
							actualExternalDNS := actualComponent.ExternalDNS[j]
							s.Equal(expectedExternalDNS.UseCertificateAutomation, actualExternalDNS.UseCertificateAutomation, "Invalid external UseCertificateAutomation for the env %s and component %s", expectedRd.Spec.Environment, expectedComponent.Name)
							s.Equal(expectedExternalDNS.FQDN, actualExternalDNS.FQDN, "Invalid external FQDN for the env %s and component %s", expectedRd.Spec.Environment, expectedComponent.Name)
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
			}
		})
	}
}

func (s *deployConfigTestSuite) getActualNewRadixDeploymentsByEnvMap(ts scenario) (map[string][]radixv1.RadixDeployment, bool) {
	rdsList, err := s.radixClient.RadixV1().RadixDeployments("").List(context.Background(), metav1.ListOptions{})
	s.Require().NoError(err)
	radixDeployments := slice.FindAll(rdsList.Items, func(rd radixv1.RadixDeployment) bool { return rd.Status.ActiveFrom.IsZero() })
	if !s.Assert().Len(radixDeployments, len(ts.expectedNewRadixDeploymentBuilderProps)) {
		return nil, false
	}
	return getRadixDeploymentsByEnvMap(radixDeployments), true
}

func getRadixDeploymentsByEnvMap(radixDeployments []radixv1.RadixDeployment) map[string][]radixv1.RadixDeployment {
	return slice.Reduce(radixDeployments, make(map[string][]radixv1.RadixDeployment), func(acc map[string][]radixv1.RadixDeployment, rd radixv1.RadixDeployment) map[string][]radixv1.RadixDeployment {
		acc[rd.Spec.Environment] = append(acc[rd.Spec.Environment], rd)
		return acc
	})
}

func (s *deployConfigTestSuite) createRadixApplication(props raProps) *radixv1.RadixApplication {
	radixApplication := buildRadixApplication(props)
	_, err := s.radixClient.RadixV1().RadixApplications(utils.GetAppNamespace(radixApplication.Name)).Create(context.Background(), radixApplication, metav1.CreateOptions{})
	s.Require().NoError(err)
	return radixApplication
}

func (s *deployConfigTestSuite) createRadixDeployments(deploymentBuildersProps []radixDeploymentBuildersProps, ra *radixv1.RadixApplication) map[string]radixv1.RadixDeployment {
	rdList := s.buildRadixDeployments(deploymentBuildersProps, ra, nil)
	for _, rd := range rdList {
		_, err := s.radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(rd.Spec.AppName, rd.Spec.Environment)).Create(context.Background(), &rd, metav1.CreateOptions{})
		s.Require().NoError(err)
	}
	return slice.Reduce(rdList, make(map[string]radixv1.RadixDeployment), func(acc map[string]radixv1.RadixDeployment, rd radixv1.RadixDeployment) map[string]radixv1.RadixDeployment {
		acc[rd.Spec.Environment] = rd
		return acc
	})
}

func (s *deployConfigTestSuite) buildRadixDeployments(deploymentBuildersProps []radixDeploymentBuildersProps, ra *radixv1.RadixApplication, sourceEnvMap map[string]radixv1.RadixDeployment) []radixv1.RadixDeployment {
	radixConfigHash, _ := internal.CreateRadixApplicationHash(ra)
	buildSecretHash, _ := internal.CreateBuildSecretHash(nil)
	var rdList []radixv1.RadixDeployment
	for _, rdProps := range deploymentBuildersProps {
		if sourceRd, ok := sourceEnvMap[rdProps.envName]; ok {
			if sourceRdRadixConfigHash, ok := sourceRd.GetAnnotations()[kube.RadixConfigHash]; ok && len(sourceRdRadixConfigHash) > 0 {
				radixConfigHash = sourceRdRadixConfigHash
			}
			if sourceBuildSecretHash, ok := sourceRd.GetAnnotations()[kube.RadixBuildSecretHash]; ok && len(sourceBuildSecretHash) > 0 {
				buildSecretHash = sourceBuildSecretHash
			}
		}
		deploymentBuilder := utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(rdProps.envName).WithImageTag(rdProps.imageTag).
			WithActiveFrom(rdProps.activeFrom).WithLabel(kube.RadixCommitLabel, commitId).WithLabel(kube.RadixJobNameLabel, jobName).
			WithAnnotations(map[string]string{
				kube.RadixGitTagsAnnotation: gitTags,
				kube.RadixCommitAnnotation:  commitId,
				kube.RadixBranchAnnotation:  "",
				kube.RadixBuildSecretHash:   buildSecretHash,
				kube.RadixConfigHash:        radixConfigHash,
			})
		for _, component := range ra.Spec.Components {
			componentBuilder := utils.NewDeployComponentBuilder().WithName(component.Name)
			for varName, varValue := range component.GetVariables() {
				componentBuilder = componentBuilder.WithEnvironmentVariable(varName, varValue)
			}
			if externalDNSs, ok := rdProps.externalDNSs[component.Name]; ok {
				componentBuilder = componentBuilder.WithExternalDNS(
					slice.Map(externalDNSs, func(compExternalDNS externalDNS) radixv1.RadixDeployExternalDNS {
						return radixv1.RadixDeployExternalDNS{FQDN: compExternalDNS.fqdn, UseCertificateAutomation: compExternalDNS.useCertificateAutomation}
					})...)
			}
			// set other props if needed
			deploymentBuilder = deploymentBuilder.WithComponent(componentBuilder)
		}
		rd := deploymentBuilder.BuildRD()
		rdList = append(rdList, *rd)
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
		existingRaBuilder.WithComponent(utils.AnApplicationComponent().WithName(componentName).WithImageTagName(existingImageTag))
	}
	for _, item := range props.dnsExternalAliases {
		existingRaBuilder.WithDNSExternalAlias(item.alias, item.envName, item.componentName, item.useCertificateAutomation)
	}
	return existingRaBuilder.BuildRA()
}

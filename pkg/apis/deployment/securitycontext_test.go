package deployment

import (
	"fmt"
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
)

func Test_NewSecurityContextBuilder(t *testing.T) {
	sut := NewSecurityContextBuilder(true)
	assert.True(t, sut.forceRunAsNonRoot)

	sut = NewSecurityContextBuilder(false)
	assert.False(t, sut.forceRunAsNonRoot)
}

type securityContextTestScenario struct {
	forceRunAsNonRoot     bool
	componentRunAsNonRoot bool
	expected              bool
}

type SecurityContextTestSuite struct {
	suite.Suite
	scenarios []securityContextTestScenario
}

func (s *SecurityContextTestSuite) SetupSuite() {
	s.scenarios = []securityContextTestScenario{
		{forceRunAsNonRoot: false, componentRunAsNonRoot: false, expected: false},
		{forceRunAsNonRoot: false, componentRunAsNonRoot: true, expected: true},
		{forceRunAsNonRoot: true, componentRunAsNonRoot: false, expected: true},
		{forceRunAsNonRoot: true, componentRunAsNonRoot: true, expected: true},
	}
}

func (s *SecurityContextTestSuite) TestBuildPodSecurityContext() {
	s.T().Parallel()
	for _, scenario := range s.scenarios {
		s.Run(
			fmt.Sprintf("test with forceRunAsNonRoot=%v and componentRunAsNonRoot=%v", scenario.forceRunAsNonRoot, scenario.componentRunAsNonRoot),
			func() {
				sut := securityContextBuilder{forceRunAsNonRoot: scenario.forceRunAsNonRoot}
				expected := corev1.PodSecurityContext{RunAsNonRoot: &scenario.expected}
				component := s.getTestComponent(scenario.componentRunAsNonRoot)
				actual := sut.BuildPodSecurityContext(component)
				s.Equal(expected, *actual)
				job := s.getTestJobComponent(scenario.componentRunAsNonRoot)
				actual = sut.BuildPodSecurityContext(job)
				s.Equal(expected, *actual)
			},
		)
	}
}

func (s *SecurityContextTestSuite) TestBuildContainerSecurityContext() {
	s.T().Parallel()
	for _, scenario := range s.scenarios {
		s.Run(
			fmt.Sprintf("test with forceRunAsNonRoot=%v and componentRunAsNonRoot=%v", scenario.forceRunAsNonRoot, scenario.componentRunAsNonRoot),
			func() {
				sut := securityContextBuilder{forceRunAsNonRoot: scenario.forceRunAsNonRoot}
				expected := corev1.SecurityContext{
					RunAsNonRoot:             &scenario.expected,
					AllowPrivilegeEscalation: utils.BoolPtr(false),
					Privileged:               utils.BoolPtr(false),
				}
				component := s.getTestComponent(scenario.componentRunAsNonRoot)
				actual := sut.BuildContainerSecurityContext(component)
				s.Equal(expected, *actual)
				job := s.getTestJobComponent(scenario.componentRunAsNonRoot)
				actual = sut.BuildContainerSecurityContext(job)
				s.Equal(expected, *actual)
			},
		)
	}
}

func (s *SecurityContextTestSuite) getTestComponent(runAsNonRoot bool) *v1.RadixDeployComponent {
	return &v1.RadixDeployComponent{
		RunAsNonRoot: runAsNonRoot,
	}
}

func (s *SecurityContextTestSuite) getTestJobComponent(runAsNonRoot bool) *v1.RadixDeployJobComponent {
	return &v1.RadixDeployJobComponent{
		RunAsNonRoot: runAsNonRoot,
	}
}

func Test_Run_SecurityContextTestSuite(t *testing.T) {
	suite.Run(t, new(SecurityContextTestSuite))
}

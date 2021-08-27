package deployment

import (
	commonUtils "github.com/equinor/radix-common/utils"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
)

type SecurityContextBuilder interface {
	BuildPodSecurityContext(v1.RadixCommonDeployComponent) *corev1.PodSecurityContext
	BuildContainerSecurityContext(v1.RadixCommonDeployComponent) *corev1.SecurityContext
}

type securityContextBuilder struct {
	forceRunAsNonRoot bool
}

func NewSecurityContextBuilder(forceRunAsNonRoot bool) *securityContextBuilder {
	return &securityContextBuilder{forceRunAsNonRoot: forceRunAsNonRoot}
}

func (s *securityContextBuilder) BuildPodSecurityContext(component v1.RadixCommonDeployComponent) *corev1.PodSecurityContext {
	runAsNonRoot := s.runAsNonRoot(component)
	return &corev1.PodSecurityContext{
		RunAsNonRoot: commonUtils.BoolPtr(runAsNonRoot),
	}
}

func (s *securityContextBuilder) BuildContainerSecurityContext(component v1.RadixCommonDeployComponent) *corev1.SecurityContext {

	runAsNonRoot := s.runAsNonRoot(component)
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: commonUtils.BoolPtr(ALLOW_PRIVILEGE_ESCALATION),
		Privileged:               commonUtils.BoolPtr(PRIVILEGED_CONTAINER),
		RunAsNonRoot:             commonUtils.BoolPtr(runAsNonRoot),
	}
}

func (s *securityContextBuilder) runAsNonRoot(component v1.RadixCommonDeployComponent) bool {
	return s.forceRunAsNonRoot || component.GetRunAsNonRoot()
}

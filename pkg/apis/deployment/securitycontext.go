package deployment

import (
	commonUtils "github.com/equinor/radix-common/utils"
	corev1 "k8s.io/api/core/v1"
)

type SecurityContextBuilder interface {
	BuildPodSecurityContext() *corev1.PodSecurityContext
	BuildContainerSecurityContext() *corev1.SecurityContext
}

type securityContextBuilder struct {
	forceRunAsNonRoot bool
}

func NewSecurityContextBuilder() *securityContextBuilder {
	return &securityContextBuilder{forceRunAsNonRoot: true}
}

func (s *securityContextBuilder) BuildPodSecurityContext() *corev1.PodSecurityContext {
	return &corev1.PodSecurityContext{
		RunAsNonRoot: commonUtils.BoolPtr(true),
	}
}

func (s *securityContextBuilder) BuildContainerSecurityContext() *corev1.SecurityContext {

	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: commonUtils.BoolPtr(ALLOW_PRIVILEGE_ESCALATION),
		Privileged:               commonUtils.BoolPtr(PRIVILEGED_CONTAINER),
		RunAsNonRoot:             commonUtils.BoolPtr(true),
	}
}

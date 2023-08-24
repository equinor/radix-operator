package securitycontext

import (
	commonUtils "github.com/equinor/radix-common/utils"
	corev1 "k8s.io/api/core/v1"
)

type ContainerOption func(securityContext *corev1.SecurityContext)

func WithContainerDropAllCapabilities() ContainerOption {
	return func(securityContext *corev1.SecurityContext) {
		if securityContext.Capabilities == nil {
			securityContext.Capabilities = &corev1.Capabilities{}
		}

		securityContext.Capabilities.Drop = []corev1.Capability{"ALL"}
	}
}

func WithContainerSeccompProfile(secCompProfile corev1.SeccompProfileType) ContainerOption {
	return func(securityContext *corev1.SecurityContext) {
		securityContext.SeccompProfile = &corev1.SeccompProfile{
			Type: secCompProfile,
		}
	}
}

func WithContainerRunAsUser(userId int64) ContainerOption {
	return func(securityContext *corev1.SecurityContext) {
		securityContext.RunAsUser = &userId
	}
}

func WithContainerRunAsGroup(groupId int64) ContainerOption {
	return func(securityContext *corev1.SecurityContext) {
		securityContext.RunAsGroup = &groupId
	}
}

func WithContainerRunAsNonRoot(runAsNonRoot *bool) ContainerOption {
	return func(securityContext *corev1.SecurityContext) {
		securityContext.RunAsNonRoot = runAsNonRoot
	}
}

func WithContainerCapabilities(capabilities []corev1.Capability) ContainerOption {
	return func(securityContext *corev1.SecurityContext) {
		securityContext.Capabilities = &corev1.Capabilities{
			Add: capabilities,
		}
	}
}

func Container(options ...ContainerOption) *corev1.SecurityContext {
	securityContext := &corev1.SecurityContext{
		AllowPrivilegeEscalation: commonUtils.BoolPtr(false),
		Privileged:               commonUtils.BoolPtr(false),
		RunAsNonRoot:             commonUtils.BoolPtr(true),
	}

	for _, o := range options {
		o(securityContext)
	}

	return securityContext
}

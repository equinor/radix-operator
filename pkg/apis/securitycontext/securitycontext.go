package securitycontext

import (
	commonUtils "github.com/equinor/radix-common/utils"
	corev1 "k8s.io/api/core/v1"
)

func PodSecurityContext() *corev1.PodSecurityContext {
	return &corev1.PodSecurityContext{
		RunAsNonRoot: commonUtils.BoolPtr(true),
	}
}

func ContainerSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: commonUtils.BoolPtr(false),
		Privileged:               commonUtils.BoolPtr(false),
		RunAsNonRoot:             commonUtils.BoolPtr(true),
	}
}

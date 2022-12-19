package securitycontext

import (
	commonUtils "github.com/equinor/radix-common/utils"
	corev1 "k8s.io/api/core/v1"
)

type PodOption func(securityContext *corev1.PodSecurityContext)

func WithPodFSGroup(fsGroup int64) PodOption {
	return func(securityContext *corev1.PodSecurityContext) {
		securityContext.FSGroup = &fsGroup
	}
}

func WithPodSeccompProfile(secCompProfile corev1.SeccompProfileType) PodOption {
	return func(securityContext *corev1.PodSecurityContext) {
		securityContext.SeccompProfile = &corev1.SeccompProfile{
			Type: secCompProfile,
		}
	}
}

func Pod(options ...PodOption) *corev1.PodSecurityContext {
	securityContext := &corev1.PodSecurityContext{
		RunAsNonRoot: commonUtils.BoolPtr(true),
	}

	for _, o := range options {
		o(securityContext)
	}

	return securityContext
}

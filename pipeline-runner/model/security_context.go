package model

import (
	corev1 "k8s.io/api/core/v1"
)

func GetContainerSecurityContext(privilegedContainer, allowPrivEsc bool, runAsGroup, runAsUser int64) *corev1.SecurityContext {
	return &corev1.SecurityContext{
		Privileged:               &privilegedContainer,
		AllowPrivilegeEscalation: &allowPrivEsc,
		RunAsUser:                &runAsUser,
		RunAsGroup:               &runAsGroup,
	}
}

func GetPodSecurityContext(runAsNonRoot bool, fsgroup int64) *corev1.PodSecurityContext {

	return &corev1.PodSecurityContext{
		RunAsNonRoot: &runAsNonRoot,
		FSGroup:      &fsgroup,
	}
}

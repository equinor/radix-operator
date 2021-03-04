package model

import (
	corev1 "k8s.io/api/core/v1"
)

// Security context default values for secure deployment
const (
	PRIVILEGED_CONTAINER       = false
	ALLOW_PRIVILEGE_ESCALATION = false
	RUN_AS_NON_ROOT            = true
	RUN_AS_USER                = 1000
	RUN_AS_GROUP               = 1000
	FS_GROUP                   = 1000
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

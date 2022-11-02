package securitycontext

import (
	commonUtils "github.com/equinor/radix-common/utils"
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
	SECCOMP_PROFILE_TYPE       = "RuntimeDefault"
)

// GetRadixPipelineContainerSecurityContext returns default security context for containers running in steps of radix-pipeline jobs
func GetRadixPipelineContainerSecurityContext(privilegedContainer, allowPrivEsc bool, runAsGroup, runAsUser int64, seccompProfileType string) *corev1.SecurityContext {
	return &corev1.SecurityContext{
		Privileged:               &privilegedContainer,
		AllowPrivilegeEscalation: &allowPrivEsc,
		RunAsUser:                &runAsUser,
		RunAsGroup:               &runAsGroup,
		SeccompProfile: &corev1.SeccompProfile{
			Type:             corev1.SeccompProfileType(seccompProfileType),
			LocalhostProfile: nil,
		},
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{
				"ALL",
			},
			Add: nil,
		},
	}
}

// GetRadixPipelinePodSecurityContext returns default security context for pods running in steps of radix-pipeline jobs
func GetRadixPipelinePodSecurityContext(runAsNonRoot bool, fsgroup int64) *corev1.PodSecurityContext {

	return &corev1.PodSecurityContext{
		RunAsNonRoot: &runAsNonRoot,
		FSGroup:      &fsgroup,
	}
}

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

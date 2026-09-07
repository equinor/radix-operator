package labels

import (
	"github.com/equinor/radix-operator/pkg/apis/config2"
)

// PodSecurityStandardFromConfig builds pod security standard labels for a namespace.
func PodSecurityStandardFromConfig(cfg config2.PodSecurityStandardPolicyConfig) map[string]string {
	labels := make(map[string]string)
	if cfg.Enforce.Level != "" && cfg.Enforce.Version != "" {
		labels["pod-security.kubernetes.io/enforce"] = cfg.Enforce.Level
		labels["pod-security.kubernetes.io/enforce-version"] = cfg.Enforce.Version
	}
	if cfg.Audit.Level != "" && cfg.Audit.Version != "" {
		labels["pod-security.kubernetes.io/audit"] = cfg.Audit.Level
		labels["pod-security.kubernetes.io/audit-version"] = cfg.Audit.Version
	}
	if cfg.Warn.Level != "" && cfg.Warn.Version != "" {
		labels["pod-security.kubernetes.io/warn"] = cfg.Warn.Level
		labels["pod-security.kubernetes.io/warn-version"] = cfg.Warn.Version
	}
	return labels
}

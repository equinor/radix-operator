package kube

import (
	"github.com/equinor/radix-operator/pkg/apis/config2"
)

type podSecurityMode string
type PodSecurityLevel string

const (
	modeEnforce podSecurityMode = "enforce"
	modeAudit   podSecurityMode = "audit"
	modeWarn    podSecurityMode = "warn"

	PrivilegedLevel PodSecurityLevel = "privileged"
	BaselineLevel   PodSecurityLevel = "baseline"
	RestrictedLevel PodSecurityLevel = "restricted"

	podSecurityStandardLabelPrefix = "pod-security.kubernetes.io/"
)

// NewAppNamespacePodSecurityStandardFromEnv builds pod security standard for app namespaces from environment variables
func NewPodSecurityStandardFromConfig(cfg config2.PodSecurityStandardPolicyConfig) *PodSecurityStandard {
	pss := &PodSecurityStandard{}

	setMode := func(mode config2.PodSecurityStandardModeConfig, modeFunc func(level PodSecurityLevel, version string)) {
		if len(mode.Level) == 0 || len(mode.Version) == 0 {
			return
		}
		modeFunc(PodSecurityLevel(mode.Level), mode.Version)
	}

	setMode(cfg.Enforce, pss.Enforce)
	setMode(cfg.Audit, pss.Audit)
	setMode(cfg.Warn, pss.Warn)

	return pss
}

type podSecurityModeSpec struct {
	version string
	level   PodSecurityLevel
}

// PodSecurityStandard defines methods to build pod security standard labels. See https://kubernetes.io/docs/concepts/security/pod-security-standards/
type PodSecurityStandard struct {
	mode map[podSecurityMode]podSecurityModeSpec
}

// Enforce policy
// Policy violations will cause the pod to be rejected.
func (pss *PodSecurityStandard) Enforce(level PodSecurityLevel, version string) {
	pss.setMode(modeEnforce, level, version)
}

// Audit pod policy violations.
// Policy violations will trigger the addition of an audit annotation to the event recorded in the audit log, but are otherwise allowed.
func (pss *PodSecurityStandard) Audit(level PodSecurityLevel, version string) {
	pss.setMode(modeAudit, level, version)
}

// Warn triggers a user-facing warning (e.g. kubectl) when a pod violates the policy
func (pss *PodSecurityStandard) Warn(level PodSecurityLevel, version string) {
	pss.setMode(modeWarn, level, version)
}

// Labels returns labels that will enforce pod security standard when applied on a namespace
func (pss *PodSecurityStandard) Labels() map[string]string {
	labels := make(map[string]string)
	for mode, spec := range pss.mode {
		labels[podSecurityStandardLabelPrefix+string(mode)] = string(spec.level)
		labels[podSecurityStandardLabelPrefix+string(mode)+"-version"] = spec.version
	}
	return labels
}

func (pss *PodSecurityStandard) setMode(mode podSecurityMode, level PodSecurityLevel, version string) {
	if pss.mode == nil {
		pss.mode = make(map[podSecurityMode]podSecurityModeSpec)
	}
	pss.mode[mode] = podSecurityModeSpec{level: level, version: version}
}

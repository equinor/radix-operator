package kube_test

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/config2"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	_ "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/stretchr/testify/assert"
)

func Test_NewPodSecurityStandardFromConfig(t *testing.T) {
	cfg := config2.PodSecurityStandardPolicyConfig{
		Enforce: config2.PodSecurityStandardModeConfig{Level: "enforcelevel", Version: "enforceversion"},
		Audit:   config2.PodSecurityStandardModeConfig{Level: "auditlevel", Version: "auditversion"},
		Warn:    config2.PodSecurityStandardModeConfig{Level: "warnlevel", Version: "warnversion"},
	}

	sut := kube.NewPodSecurityStandardFromConfig(cfg)
	actual := sut.Labels()
	expected := map[string]string{
		"pod-security.kubernetes.io/enforce":         cfg.Enforce.Level,
		"pod-security.kubernetes.io/enforce-version": cfg.Enforce.Version,
		"pod-security.kubernetes.io/audit":           cfg.Audit.Level,
		"pod-security.kubernetes.io/audit-version":   cfg.Audit.Version,
		"pod-security.kubernetes.io/warn":            cfg.Warn.Level,
		"pod-security.kubernetes.io/warn-version":    cfg.Warn.Version,
	}
	assert.Equal(t, expected, actual)
}

func Test_PodSecurityStandard_Enforce(t *testing.T) {
	version := "anyversion"
	type scenario struct {
		profile  kube.PodSecurityLevel
		expected string
	}
	scenarios := []scenario{
		{profile: kube.RestrictedLevel, expected: "restricted"},
		{profile: kube.BaselineLevel, expected: "baseline"},
		{profile: kube.PrivilegedLevel, expected: "privileged"}}

	for _, s := range scenarios {
		sut := kube.PodSecurityStandard{}
		sut.Enforce(s.profile, version)
		actual := sut.Labels()
		expected := map[string]string{
			"pod-security.kubernetes.io/enforce":         s.expected,
			"pod-security.kubernetes.io/enforce-version": version,
		}
		assert.Equal(t, expected, actual)
	}
}

func Test_PodSecurityStandard_Audit(t *testing.T) {
	version := "anyversion"
	type scenario struct {
		profile  kube.PodSecurityLevel
		expected string
	}
	scenarios := []scenario{
		{profile: kube.RestrictedLevel, expected: "restricted"},
		{profile: kube.BaselineLevel, expected: "baseline"},
		{profile: kube.PrivilegedLevel, expected: "privileged"}}

	for _, s := range scenarios {
		sut := kube.PodSecurityStandard{}
		sut.Audit(s.profile, version)
		actual := sut.Labels()
		expected := map[string]string{
			"pod-security.kubernetes.io/audit":         s.expected,
			"pod-security.kubernetes.io/audit-version": version,
		}
		assert.Equal(t, expected, actual)
	}
}

func Test_PodSecurityStandard_Warn(t *testing.T) {
	version := "anyversion"
	type scenario struct {
		profile  kube.PodSecurityLevel
		expected string
	}
	scenarios := []scenario{
		{profile: kube.RestrictedLevel, expected: "restricted"},
		{profile: kube.BaselineLevel, expected: "baseline"},
		{profile: kube.PrivilegedLevel, expected: "privileged"}}

	for _, s := range scenarios {
		sut := kube.PodSecurityStandard{}
		sut.Warn(s.profile, version)
		actual := sut.Labels()
		expected := map[string]string{
			"pod-security.kubernetes.io/warn":         s.expected,
			"pod-security.kubernetes.io/warn-version": version,
		}
		assert.Equal(t, expected, actual)
	}
}

func Test_PodSecurityStandard_MultipleModes(t *testing.T) {
	sut := kube.PodSecurityStandard{}
	sut.Enforce(kube.BaselineLevel, "v1")
	sut.Audit(kube.RestrictedLevel, "v2")
	sut.Warn(kube.PrivilegedLevel, "v3")
	actual := sut.Labels()
	expected := map[string]string{
		"pod-security.kubernetes.io/enforce":         "baseline",
		"pod-security.kubernetes.io/enforce-version": "v1",
		"pod-security.kubernetes.io/audit":           "restricted",
		"pod-security.kubernetes.io/audit-version":   "v2",
		"pod-security.kubernetes.io/warn":            "privileged",
		"pod-security.kubernetes.io/warn-version":    "v3",
	}
	assert.Equal(t, expected, actual)
}

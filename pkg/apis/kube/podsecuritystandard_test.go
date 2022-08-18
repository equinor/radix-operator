package kube

import (
	"os"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/stretchr/testify/assert"
)

func Test_NewPodSecurityStandardFromEnv_Enforce(t *testing.T) {
	os.Setenv(defaults.PodSecurityStandardEnforceLevelEnvironmentVariable, "anylevel")
	os.Setenv(defaults.PodSecurityStandardEnforceVersionEnvironmentVariable, "anyversion")
	defer os.Clearenv()

	sut := NewPodSecurityStandardFromEnv()
	actual := sut.Labels()
	expected := map[string]string{
		"pod-security.kubernetes.io/enforce":         "anylevel",
		"pod-security.kubernetes.io/enforce-version": "anyversion",
	}
	assert.Equal(t, expected, actual)
}

func Test_NewPodSecurityStandardFromEnv_Audit(t *testing.T) {
	os.Setenv(defaults.PodSecurityStandardAuditLevelEnvironmentVariable, "anylevel")
	os.Setenv(defaults.PodSecurityStandardAuditVersionEnvironmentVariable, "anyversion")
	defer os.Clearenv()

	sut := NewPodSecurityStandardFromEnv()
	actual := sut.Labels()
	expected := map[string]string{
		"pod-security.kubernetes.io/audit":         "anylevel",
		"pod-security.kubernetes.io/audit-version": "anyversion",
	}
	assert.Equal(t, expected, actual)
}

func Test_NewPodSecurityStandardFromEnv_Warn(t *testing.T) {
	os.Setenv(defaults.PodSecurityStandardWarnLevelEnvironmentVariable, "anylevel")
	os.Setenv(defaults.PodSecurityStandardWarnVersionEnvironmentVariable, "anyversion")
	defer os.Clearenv()

	sut := NewPodSecurityStandardFromEnv()
	actual := sut.Labels()
	expected := map[string]string{
		"pod-security.kubernetes.io/warn":         "anylevel",
		"pod-security.kubernetes.io/warn-version": "anyversion",
	}
	assert.Equal(t, expected, actual)
}

func Test_NewPodSecurityStandardFromEnv_All(t *testing.T) {
	os.Setenv(defaults.PodSecurityStandardEnforceLevelEnvironmentVariable, "anylevel1")
	os.Setenv(defaults.PodSecurityStandardEnforceVersionEnvironmentVariable, "anyversion1")
	os.Setenv(defaults.PodSecurityStandardAuditLevelEnvironmentVariable, "anylevel2")
	os.Setenv(defaults.PodSecurityStandardAuditVersionEnvironmentVariable, "anyversion2")
	os.Setenv(defaults.PodSecurityStandardWarnLevelEnvironmentVariable, "anylevel3")
	os.Setenv(defaults.PodSecurityStandardWarnVersionEnvironmentVariable, "anyversion3")
	defer os.Clearenv()

	sut := NewPodSecurityStandardFromEnv()
	actual := sut.Labels()
	expected := map[string]string{
		"pod-security.kubernetes.io/enforce":         "anylevel1",
		"pod-security.kubernetes.io/enforce-version": "anyversion1",
		"pod-security.kubernetes.io/audit":           "anylevel2",
		"pod-security.kubernetes.io/audit-version":   "anyversion2",
		"pod-security.kubernetes.io/warn":            "anylevel3",
		"pod-security.kubernetes.io/warn-version":    "anyversion3",
	}
	assert.Equal(t, expected, actual)
}

func Test_PodSecurityStandard_Enforce(t *testing.T) {
	version := "anyversion"
	type scenario struct {
		profile  PodSecurityLevel
		expected string
	}
	scenarios := []scenario{{profile: RestrictedLevel, expected: "restricted"}, {profile: BaselineLevel, expected: "baseline"}, {profile: PrivilegedLevel, expected: "privileged"}}

	for _, s := range scenarios {
		sut := PodSecurityStandard{}
		actual := sut.Enforce(s.profile, version).Labels()
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
		profile  PodSecurityLevel
		expected string
	}
	scenarios := []scenario{{profile: RestrictedLevel, expected: "restricted"}, {profile: BaselineLevel, expected: "baseline"}, {profile: PrivilegedLevel, expected: "privileged"}}

	for _, s := range scenarios {
		sut := PodSecurityStandard{}
		actual := sut.Audit(s.profile, version).Labels()
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
		profile  PodSecurityLevel
		expected string
	}
	scenarios := []scenario{{profile: RestrictedLevel, expected: "restricted"}, {profile: BaselineLevel, expected: "baseline"}, {profile: PrivilegedLevel, expected: "privileged"}}

	for _, s := range scenarios {
		sut := PodSecurityStandard{}
		actual := sut.Warn(s.profile, version).Labels()
		expected := map[string]string{
			"pod-security.kubernetes.io/warn":         s.expected,
			"pod-security.kubernetes.io/warn-version": version,
		}
		assert.Equal(t, expected, actual)
	}
}

func Test_PodSecurityStandard_MultipleModesAndEmptyVersion(t *testing.T) {
	sut := PodSecurityStandard{}
	actual := sut.Enforce(BaselineLevel, "").Audit(RestrictedLevel, "").Warn(PrivilegedLevel, "").Labels()
	expected := map[string]string{
		"pod-security.kubernetes.io/enforce":         "baseline",
		"pod-security.kubernetes.io/enforce-version": "latest",
		"pod-security.kubernetes.io/audit":           "restricted",
		"pod-security.kubernetes.io/audit-version":   "latest",
		"pod-security.kubernetes.io/warn":            "privileged",
		"pod-security.kubernetes.io/warn-version":    "latest",
	}
	assert.Equal(t, expected, actual)
}

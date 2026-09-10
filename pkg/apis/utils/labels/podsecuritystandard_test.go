package labels_test

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/stretchr/testify/assert"
	kubelabels "k8s.io/apimachinery/pkg/labels"
)

func Test_PodSecurityStandardFromConfig(t *testing.T) {
	tests := map[string]struct {
		cfg      config.PodSecurityStandardPolicyConfig
		expected kubelabels.Set
	}{
		"all modes": {
			cfg: config.PodSecurityStandardPolicyConfig{
				Enforce: config.PodSecurityStandardModeConfig{Level: "enforcelevel", Version: "enforceversion"},
				Audit:   config.PodSecurityStandardModeConfig{Level: "auditlevel", Version: "auditversion"},
				Warn:    config.PodSecurityStandardModeConfig{Level: "warnlevel", Version: "warnversion"},
			},
			expected: map[string]string{
				"pod-security.kubernetes.io/enforce":         "enforcelevel",
				"pod-security.kubernetes.io/enforce-version": "enforceversion",
				"pod-security.kubernetes.io/audit":           "auditlevel",
				"pod-security.kubernetes.io/audit-version":   "auditversion",
				"pod-security.kubernetes.io/warn":            "warnlevel",
				"pod-security.kubernetes.io/warn-version":    "warnversion",
			},
		},
		"incomplete modes are omitted": {
			cfg: config.PodSecurityStandardPolicyConfig{
				Enforce: config.PodSecurityStandardModeConfig{Level: "enforcelevel"},
				Audit:   config.PodSecurityStandardModeConfig{Version: "auditversion"},
				Warn:    config.PodSecurityStandardModeConfig{Level: "warnlevel", Version: "warnversion"},
			},
			expected: map[string]string{
				"pod-security.kubernetes.io/warn":         "warnlevel",
				"pod-security.kubernetes.io/warn-version": "warnversion",
			},
		},
		"empty config returns empty labels": {
			expected: map[string]string{},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			actual := labels.PodSecurityStandardFromConfig(test.cfg)
			assert.Equal(t, test.expected, actual)
		})
	}
}

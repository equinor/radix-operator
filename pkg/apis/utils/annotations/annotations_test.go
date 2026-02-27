package annotations

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/stretchr/testify/assert"
)

func Test_Merge(t *testing.T) {
	actual := Merge(
		map[string]string{"a": "a", "b": "b", "c": "c1"},
		map[string]string{"a": "a", "c": "c2", "d": "d"},
	)
	expected := map[string]string{"a": "a", "b": "b", "c": "c2", "d": "d"}
	assert.Equal(t, expected, actual)
}

func Test_ForRadixBranch(t *testing.T) {
	actual := ForRadixBranch("anybranch")
	expected := map[string]string{kube.RadixBranchAnnotation: "anybranch"}
	assert.Equal(t, expected, actual)
}

func Test_ForRadixDeploymentName(t *testing.T) {
	actual := ForRadixDeploymentName("anydeployment")
	expected := map[string]string{kube.RadixDeploymentNameAnnotation: "anydeployment"}
	assert.Equal(t, expected, actual)
}

func Test_ForServiceAccountWithRadixIdentity(t *testing.T) {
	actual := ForServiceAccountWithRadixIdentityClientId("")
	assert.Equal(t, map[string]string(nil), actual)

	actual = ForServiceAccountWithRadixIdentityClientId("anyclientid")
	expected := map[string]string{"azure.workload.identity/client-id": "anyclientid"}
	assert.Equal(t, expected, actual)
}

func Test_ForClusterAutoscalerSafeToEvict(t *testing.T) {
	actual := ForClusterAutoscalerSafeToEvict(false)
	expected := map[string]string{"cluster-autoscaler.kubernetes.io/safe-to-evict": "false"}
	assert.Equal(t, expected, actual)

	actual = ForClusterAutoscalerSafeToEvict(true)
	expected = map[string]string{"cluster-autoscaler.kubernetes.io/safe-to-evict": "true"}
	assert.Equal(t, expected, actual)
}

func Test_OAuth2ProxyModeEnabledForEnvironment(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		currentEnv  string
		expected    bool
	}{
		{name: "nil annotations", annotations: nil, currentEnv: "dev", expected: false},
		{name: "empty annotations", annotations: map[string]string{}, currentEnv: "dev", expected: false},
		{name: "annotation missing", annotations: map[string]string{"other": "dev"}, currentEnv: "dev", expected: false},
		{name: "wildcard matches any env", annotations: map[string]string{PreviewOAuth2ProxyModeAnnotation: "*"}, currentEnv: "qa", expected: true},
		{name: "exact match single env", annotations: map[string]string{PreviewOAuth2ProxyModeAnnotation: "dev"}, currentEnv: "dev", expected: true},
		{name: "no match single env", annotations: map[string]string{PreviewOAuth2ProxyModeAnnotation: "dev"}, currentEnv: "qa", expected: false},
		{name: "match first in list", annotations: map[string]string{PreviewOAuth2ProxyModeAnnotation: "dev,qa"}, currentEnv: "dev", expected: true},
		{name: "match last in list", annotations: map[string]string{PreviewOAuth2ProxyModeAnnotation: "dev,qa"}, currentEnv: "qa", expected: true},
		{name: "no match in list", annotations: map[string]string{PreviewOAuth2ProxyModeAnnotation: "dev,qa"}, currentEnv: "prod", expected: false},
		{name: "case sensitive mismatch", annotations: map[string]string{PreviewOAuth2ProxyModeAnnotation: "dev,qa"}, currentEnv: "Qa", expected: false},
		{name: "spaces around env trimmed", annotations: map[string]string{PreviewOAuth2ProxyModeAnnotation: "dev, qa , prod"}, currentEnv: "qa", expected: true},
		{name: "empty value", annotations: map[string]string{PreviewOAuth2ProxyModeAnnotation: ""}, currentEnv: "dev", expected: false},
		{name: "empty currentEnv with empty entry", annotations: map[string]string{PreviewOAuth2ProxyModeAnnotation: ""}, currentEnv: "", expected: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, OAuth2ProxyModeEnabledForEnvironment(tt.annotations, tt.currentEnv))
		})
	}
}

func Test_GatewayAPIEnabledForEnvironment(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		currentEnv  string
		expected    bool
	}{
		{name: "nil annotations", annotations: nil, currentEnv: "dev", expected: false},
		{name: "empty annotations", annotations: map[string]string{}, currentEnv: "dev", expected: false},
		{name: "annotation missing", annotations: map[string]string{"other": "dev"}, currentEnv: "dev", expected: false},
		{name: "wildcard matches any env", annotations: map[string]string{PreviewGatewayModeAnnotation: "*"}, currentEnv: "qa", expected: true},
		{name: "exact match single env", annotations: map[string]string{PreviewGatewayModeAnnotation: "dev"}, currentEnv: "dev", expected: true},
		{name: "no match single env", annotations: map[string]string{PreviewGatewayModeAnnotation: "dev"}, currentEnv: "qa", expected: false},
		{name: "match first in list", annotations: map[string]string{PreviewGatewayModeAnnotation: "dev,qa"}, currentEnv: "dev", expected: true},
		{name: "match last in list", annotations: map[string]string{PreviewGatewayModeAnnotation: "dev,qa"}, currentEnv: "qa", expected: true},
		{name: "no match in list", annotations: map[string]string{PreviewGatewayModeAnnotation: "dev,qa"}, currentEnv: "prod", expected: false},
		{name: "case sensitive mismatch", annotations: map[string]string{PreviewGatewayModeAnnotation: "dev,qa"}, currentEnv: "Qa", expected: false},
		{name: "spaces around env trimmed", annotations: map[string]string{PreviewGatewayModeAnnotation: "dev, qa , prod"}, currentEnv: "qa", expected: true},
		{name: "empty value", annotations: map[string]string{PreviewGatewayModeAnnotation: ""}, currentEnv: "dev", expected: false},
		{name: "empty currentEnv with empty entry", annotations: map[string]string{PreviewGatewayModeAnnotation: ""}, currentEnv: "", expected: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, GatewayAPIEnabledForEnvironment(tt.annotations, tt.currentEnv))
		})
	}
}

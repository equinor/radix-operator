package annotations

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func Test_OAuth2ProxyMode_IsEnabled_WhenDeploymentAnnotation_IsDevQa_AndCurrentEnv_IsDev(t *testing.T) {
	rd := radixv1.RadixDeployment{
		ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{PreviewOAuth2ProxyModeAnnotation: "dev,qa"}},
		Spec:       radixv1.RadixDeploymentSpec{Environment: "dev"},
	}

	assert.True(t, OAuth2ProxyModeEnabledForEnvironment(&rd, nil))
}

func Test_OAuth2ProxyMode_IsEnabled_WhenDeploymentAnnotation_IsDevQa_AndCurrentEnv_IsQa(t *testing.T) {
	rd := radixv1.RadixDeployment{
		ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{PreviewOAuth2ProxyModeAnnotation: "dev,qa"}},
		Spec:       radixv1.RadixDeploymentSpec{Environment: "qa"},
	}

	assert.True(t, OAuth2ProxyModeEnabledForEnvironment(&rd, nil))
}

func Test_OAuth2ProxyMode_IsNotEnabled_WhenDeploymentAnnotation_IsDevQa_AndCurrentEnv_IsProd(t *testing.T) {
	rd := radixv1.RadixDeployment{
		ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{PreviewOAuth2ProxyModeAnnotation: "dev,qa"}},
		Spec:       radixv1.RadixDeploymentSpec{Environment: "prod"},
	}

	assert.False(t, OAuth2ProxyModeEnabledForEnvironment(&rd, nil))
}

func Test_OAuth2ProxyMode_IsEnabled_WhenRegistrationAnnotation_IsDevQa_AndCurrentEnv_IsQa(t *testing.T) {
	rd := radixv1.RadixDeployment{
		Spec: radixv1.RadixDeploymentSpec{Environment: "qa"},
	}
	rr := radixv1.RadixRegistration{
		ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{PreviewOAuth2ProxyModeAnnotation: "dev,qa"}},
	}

	assert.True(t, OAuth2ProxyModeEnabledForEnvironment(&rd, &rr))
}

func Test_OAuth2ProxyMode_IsNotEnabled_WhenRegistrationAnnotation_IsDevQa_AndCurrentEnv_IsProd(t *testing.T) {
	rd := radixv1.RadixDeployment{
		Spec: radixv1.RadixDeploymentSpec{Environment: "prod"},
	}
	rr := radixv1.RadixRegistration{
		ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{PreviewOAuth2ProxyModeAnnotation: "dev,qa"}},
	}

	assert.False(t, OAuth2ProxyModeEnabledForEnvironment(&rd, &rr))
}

func Test_OAuth2ProxyMode_IsEnabled_WhenDeploymentAnnotation_IsWildcard_AndCurrentEnv_IsDevQa(t *testing.T) {
	rd := radixv1.RadixDeployment{
		ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{PreviewOAuth2ProxyModeAnnotation: "*"}},
		Spec:       radixv1.RadixDeploymentSpec{Environment: "dev,qa"},
	}

	assert.True(t, OAuth2ProxyModeEnabledForEnvironment(&rd, nil))
}

func Test_OAuth2ProxyMode_IsEnabled_WhenDeploymentAnnotation_IsWildcard_AndCurrentEnv_IsEmpty(t *testing.T) {
	rd := radixv1.RadixDeployment{
		ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{PreviewOAuth2ProxyModeAnnotation: "*"}},
		Spec:       radixv1.RadixDeploymentSpec{Environment: ""},
	}

	assert.True(t, OAuth2ProxyModeEnabledForEnvironment(&rd, nil))
}

func Test_OAuth2ProxyMode_IsEnabled_WhenDeploymentAnnotation_IsDevQa_AndRegistrationAnnotation_IsWildcard_AndCurrentEnv_IsProd(t *testing.T) {
	rd := radixv1.RadixDeployment{
		ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{PreviewOAuth2ProxyModeAnnotation: "dev,qa"}},
		Spec:       radixv1.RadixDeploymentSpec{Environment: "prod"},
	}

	rr := radixv1.RadixRegistration{
		ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{PreviewOAuth2ProxyModeAnnotation: "*"}},
	}

	assert.True(t, OAuth2ProxyModeEnabledForEnvironment(&rd, &rr))
}

func Test_OAuth2ProxyMode_IsNotEnabled_WhenDeploymentAnnotation_IsWildcard_AndRegistrationAnnotation_IsDevQa_AndCurrentEnv_IsProd(t *testing.T) {
	rd := radixv1.RadixDeployment{
		ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{PreviewOAuth2ProxyModeAnnotation: "*"}},
		Spec:       radixv1.RadixDeploymentSpec{Environment: "prod"},
	}

	rr := radixv1.RadixRegistration{
		ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{PreviewOAuth2ProxyModeAnnotation: "dev,qa"}},
	}

	assert.False(t, OAuth2ProxyModeEnabledForEnvironment(&rd, &rr))
}

func Test_OAuth2ProxyMode_IsNotEnabled_WhenDeploymentAnnotation_IsDevQa_AndRegistrationAnnotation_IsDev_AndCurrentEnv_IsQa(t *testing.T) {
	rd := radixv1.RadixDeployment{
		ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{PreviewOAuth2ProxyModeAnnotation: "dev,qa"}},
		Spec:       radixv1.RadixDeploymentSpec{Environment: "qa"},
	}

	rr := radixv1.RadixRegistration{
		ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{PreviewOAuth2ProxyModeAnnotation: "dev"}},
	}

	assert.False(t, OAuth2ProxyModeEnabledForEnvironment(&rd, &rr))
}

func Test_OAuth2ProxyMode_IsEnabled_WhenDeploymentAnnotation_IsDevQa_AndRegistrationAnnotation_IsProd_AndCurrentEnv_IsProd(t *testing.T) {
	rd := radixv1.RadixDeployment{
		ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{PreviewOAuth2ProxyModeAnnotation: "dev,qa"}},
		Spec:       radixv1.RadixDeploymentSpec{Environment: "prod"},
	}

	rr := radixv1.RadixRegistration{
		ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{PreviewOAuth2ProxyModeAnnotation: "dev,qa,prod"}},
	}

	assert.True(t, OAuth2ProxyModeEnabledForEnvironment(&rd, &rr))
}

func Test_OAuth2ProxyMode_IsNotEnabled_WhenDeploymentAnnotation_IsWrong(t *testing.T) {
	rd := radixv1.RadixDeployment{
		ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{"some-other-annotation": "dev,qa"}},
		Spec:       radixv1.RadixDeploymentSpec{Environment: "dev"},
	}

	assert.False(t, OAuth2ProxyModeEnabledForEnvironment(&rd, nil))
}

func Test_OAuth2ProxyMode_IsNotEnabled_WhenRegistrationAnnotation_IsWrong(t *testing.T) {
	rd := radixv1.RadixDeployment{
		Spec: radixv1.RadixDeploymentSpec{Environment: "dev"},
	}
	rr := radixv1.RadixRegistration{
		ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{"some-other-annotation": "dev,qa,prod"}},
	}

	assert.False(t, OAuth2ProxyModeEnabledForEnvironment(&rd, &rr))
}

func Test_OAuth2ProxyMode_IsNotEnabled_WhenDeployment_IsNil(t *testing.T) {
	assert.False(t, OAuth2ProxyModeEnabledForEnvironment(nil, nil))
}

func Test_OAuth2ProxyMode_IsNotEnabled_WhenCasingMismatch(t *testing.T) {
	rd := radixv1.RadixDeployment{
		ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{PreviewOAuth2ProxyModeAnnotation: "dev,qa"}},
		Spec:       radixv1.RadixDeploymentSpec{Environment: "Qa"},
	}

	assert.False(t, OAuth2ProxyModeEnabledForEnvironment(&rd, nil))
}

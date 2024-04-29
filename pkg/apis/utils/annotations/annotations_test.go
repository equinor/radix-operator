package annotations

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	_ "github.com/equinor/radix-operator/pkg/apis/test"
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
	actual := ForServiceAccountWithRadixIdentity(nil)
	assert.Equal(t, map[string]string(nil), actual)

	actual = ForServiceAccountWithRadixIdentity(&v1.Identity{})
	assert.Equal(t, map[string]string(nil), actual)

	actual = ForServiceAccountWithRadixIdentity(&v1.Identity{Azure: &v1.AzureIdentity{ClientId: "anyclientid"}})
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

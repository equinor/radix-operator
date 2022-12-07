package labels

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	kubelabels "k8s.io/apimachinery/pkg/labels"
)

func Test_Merge(t *testing.T) {
	actual := Merge(
		kubelabels.Set{"a": "a", "b": "b", "c": "c1"},
		kubelabels.Set{"a": "a", "c": "c2", "d": "d"},
	)
	expected := kubelabels.Set{"a": "a", "b": "b", "c": "c2", "d": "d"}
	assert.Equal(t, expected, actual)
}

func Test_ForApplicationName(t *testing.T) {
	actual := ForApplicationName("anyappname")
	expected := kubelabels.Set{kube.RadixAppLabel: "anyappname"}
	assert.Equal(t, expected, actual)
}

func Test_ForComponentName(t *testing.T) {
	actual := ForComponentName("anycomponentname")
	expected := kubelabels.Set{kube.RadixComponentLabel: "anycomponentname"}
	assert.Equal(t, expected, actual)
}

func Test_ForCommitId(t *testing.T) {
	actual := ForCommitId("anycommit")
	expected := kubelabels.Set{kube.RadixCommitLabel: "anycommit"}
	assert.Equal(t, expected, actual)
}

func Test_ForPodIsJobScheduler(t *testing.T) {
	actual := ForPodIsJobScheduler()
	expected := kubelabels.Set{kube.RadixPodIsJobSchedulerLabel: "true"}
	assert.Equal(t, expected, actual)
}

func Test_ForServiceAccountWithRadixIdentity(t *testing.T) {
	actual := ForServiceAccountWithRadixIdentity(nil)
	assert.Equal(t, kubelabels.Set(nil), actual)

	actual = ForServiceAccountWithRadixIdentity(&v1.Identity{})
	assert.Equal(t, kubelabels.Set(nil), actual)

	actual = ForServiceAccountWithRadixIdentity(&v1.Identity{Azure: &v1.AzureIdentity{ClientId: "any"}})
	expected := kubelabels.Set{"azure.workload.identity/use": "true"}
	assert.Equal(t, expected, actual)
}

func Test_ForPodWithRadixIdentity(t *testing.T) {
	actual := ForPodWithRadixIdentity(nil)
	assert.Equal(t, kubelabels.Set(nil), actual)

	actual = ForPodWithRadixIdentity(&v1.Identity{})
	assert.Equal(t, kubelabels.Set(nil), actual)

	actual = ForPodWithRadixIdentity(&v1.Identity{Azure: &v1.AzureIdentity{ClientId: "any"}})
	expected := kubelabels.Set{"azure.workload.identity/use": "true"}
	assert.Equal(t, expected, actual)
}

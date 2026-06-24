package kubequery

import (
	"context"
	"testing"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_GetRadixDNSAliases(t *testing.T) {
	matched1 := radixv1.RadixDNSAlias{
		ObjectMeta: metav1.ObjectMeta{Name: "matched1"},
		Spec: radixv1.RadixDNSAliasSpec{
			AppName: "app1", Environment: "env1", Component: "comp1",
		},
		Status: radixv1.RadixDNSAliasStatus{
			ReconcileStatus: radixv1.RadixDNSAliasReconcileSucceeded,
			Message:         "",
		},
	}
	matched2 := radixv1.RadixDNSAlias{ObjectMeta: metav1.ObjectMeta{Name: "matched2"}, Spec: radixv1.RadixDNSAliasSpec{
		AppName: "app1", Environment: "env1", Component: "comp1",
	},
		Status: radixv1.RadixDNSAliasStatus{
			ReconcileStatus: radixv1.RadixDNSAliasReconcileFailed,
			Message:         "Some error",
		},
	}
	unmatched := radixv1.RadixDNSAlias{ObjectMeta: metav1.ObjectMeta{Name: "unmatched"}, Spec: radixv1.RadixDNSAliasSpec{
		AppName: "app2", Environment: "env1", Component: "comp1",
	}}
	client := radixfake.NewSimpleClientset(&matched1, &matched2, &unmatched) //nolint:staticcheck
	ra := &radixv1.RadixApplication{ObjectMeta: metav1.ObjectMeta{Name: "app1"},
		Spec: radixv1.RadixApplicationSpec{
			DNSAlias: []radixv1.DNSAlias{
				{Alias: "matched1", Environment: "env1", Component: "comp1"},
				{Alias: "matched2", Environment: "env1", Component: "comp2"},
			}},
	}
	actual := GetDNSAliases(context.Background(), client, ra)
	require.Len(t, actual, 2, "unexpected amount of actual DNS aliases")
	assert.ElementsMatch(t, []radixv1.RadixDNSAlias{matched1, matched2}, actual)
}

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

func Test_GetRadixDeploymentsForEnvironment(t *testing.T) {
	matched1 := radixv1.RadixDeployment{ObjectMeta: metav1.ObjectMeta{Name: "matched1", Namespace: "app1-env1"}}
	matched2 := radixv1.RadixDeployment{ObjectMeta: metav1.ObjectMeta{Name: "matched2", Namespace: "app1-env1"}}
	unmatched1 := radixv1.RadixDeployment{ObjectMeta: metav1.ObjectMeta{Name: "unmatched1", Namespace: "app1-env2"}}
	client := radixfake.NewSimpleClientset(&matched1, &matched2, &unmatched1) //nolint:staticcheck
	expected := []radixv1.RadixDeployment{matched1, matched2}
	actual, err := GetRadixDeploymentsForEnvironment(context.Background(), client, "app1", "env1")
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, actual)
}

func Test_GetRadixDeploymentsForEnvironments(t *testing.T) {
	matched1 := radixv1.RadixDeployment{ObjectMeta: metav1.ObjectMeta{Name: "matched1", Namespace: "app1-env1"}}
	matched2 := radixv1.RadixDeployment{ObjectMeta: metav1.ObjectMeta{Name: "matched2", Namespace: "app1-env1"}}
	matched3 := radixv1.RadixDeployment{ObjectMeta: metav1.ObjectMeta{Name: "matched3", Namespace: "app1-env2"}}
	unmatched1 := radixv1.RadixDeployment{ObjectMeta: metav1.ObjectMeta{Name: "unmatched1", Namespace: "app1-env3"}}
	client := radixfake.NewSimpleClientset(&matched1, &matched2, &matched3, &unmatched1) //nolint:staticcheck
	expected := []radixv1.RadixDeployment{matched1, matched2, matched3}
	actual, err := GetRadixDeploymentsForEnvironments(context.Background(), client, "app1", []string{"env1", "env2"}, 10)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, actual)
}

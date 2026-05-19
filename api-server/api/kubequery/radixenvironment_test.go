package kubequery

import (
	"context"
	"testing"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_GetRadixEnvironment(t *testing.T) {
	matched := radixv1.RadixEnvironment{ObjectMeta: metav1.ObjectMeta{Name: "app1-env1"}}
	unmatched := radixv1.RadixEnvironment{ObjectMeta: metav1.ObjectMeta{Name: "app1-any"}}
	client := radixfake.NewSimpleClientset(&matched, &unmatched) //nolint:staticcheck

	// Get existing RE
	actual, err := GetRadixEnvironment(context.Background(), client, "app1", "env1")
	require.NoError(t, err)
	assert.Equal(t, &matched, actual)

	// Get non-existing RE
	actual, err = GetRadixEnvironment(context.Background(), client, "app2", "env2")
	assert.True(t, errors.IsNotFound(err))
	assert.Nil(t, actual)
}

func Test_GetRadixEnvironments(t *testing.T) {
	matched1 := radixv1.RadixEnvironment{ObjectMeta: metav1.ObjectMeta{Name: "matched1", Labels: labels.ForApplicationName("app1")}}
	matched2 := radixv1.RadixEnvironment{ObjectMeta: metav1.ObjectMeta{Name: "matched2", Labels: labels.ForApplicationName("app1")}}
	unmatched1 := radixv1.RadixEnvironment{ObjectMeta: metav1.ObjectMeta{Name: "unmatched1", Labels: labels.ForApplicationName("app2")}}
	client := radixfake.NewSimpleClientset(&matched1, &matched2, &unmatched1) //nolint:staticcheck
	expected := []radixv1.RadixEnvironment{matched1, matched2}
	actual, err := GetRadixEnvironments(context.Background(), client, "app1")
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, actual)
}

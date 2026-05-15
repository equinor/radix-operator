package kubequery

import (
	"context"
	"testing"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_GetRadixJobs(t *testing.T) {
	matched1 := radixv1.RadixJob{ObjectMeta: metav1.ObjectMeta{Name: "matched1", Namespace: "app1-app"}}
	matched2 := radixv1.RadixJob{ObjectMeta: metav1.ObjectMeta{Name: "matched2", Namespace: "app1-app"}}
	unmatched1 := radixv1.RadixJob{ObjectMeta: metav1.ObjectMeta{Name: "unmatched1", Namespace: "app1-any"}}
	unmatched2 := radixv1.RadixJob{ObjectMeta: metav1.ObjectMeta{Name: "unmatched2", Namespace: "app2-app"}}

	client := radixfake.NewSimpleClientset(&matched1, &matched2, &unmatched1, &unmatched2) //nolint:staticcheck
	expected := []radixv1.RadixJob{matched1, matched2}
	actual, err := GetRadixJobs(context.Background(), client, "app1")
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, actual)
}

func Test_GetRadixJob(t *testing.T) {
	matched := radixv1.RadixJob{ObjectMeta: metav1.ObjectMeta{Name: "matched", Namespace: "app1-app"}}
	unmatched := radixv1.RadixJob{ObjectMeta: metav1.ObjectMeta{Name: "unmatched", Namespace: "app2-any"}}

	client := radixfake.NewSimpleClientset(&matched, &unmatched) //nolint:staticcheck

	// Get existing RJ
	actual, err := GetRadixJob(context.Background(), client, "app1", "matched")
	require.NoError(t, err)
	assert.Equal(t, &matched, actual)

	// Get non-existing RJ
	_, err = GetRadixJob(context.Background(), client, "app2", "unmatched")
	assert.True(t, errors.IsNotFound(err))
}

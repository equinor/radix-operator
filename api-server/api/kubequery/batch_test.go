package kubequery_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/equinor/radix-operator/api-server/api/kubequery"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_GetRadixBatchesForJobComponent(t *testing.T) {
	app, env, comp := "app1", "env1", "c1"

	ns := func(app, env string) string { return fmt.Sprintf("%s-%s", app, env) }
	matchjob1 := radixv1.RadixBatch{ObjectMeta: metav1.ObjectMeta{
		Name:      "matchjob1",
		Namespace: ns(app, env),
		Labels:    radixlabels.Merge(radixlabels.ForApplicationName(app), radixlabels.ForComponentName(comp), radixlabels.ForBatchType(kube.RadixBatchTypeJob)),
	}}
	matchjob2 := radixv1.RadixBatch{ObjectMeta: metav1.ObjectMeta{
		Name:      "matchjob2",
		Namespace: ns(app, env),
		Labels:    radixlabels.Merge(radixlabels.ForApplicationName(app), radixlabels.ForComponentName(comp), radixlabels.ForBatchType(kube.RadixBatchTypeJob)),
	}}
	matchbatch1 := radixv1.RadixBatch{ObjectMeta: metav1.ObjectMeta{
		Name:      "matchbatch1",
		Namespace: ns(app, env),
		Labels:    radixlabels.Merge(radixlabels.ForApplicationName(app), radixlabels.ForComponentName(comp), radixlabels.ForBatchType(kube.RadixBatchTypeBatch)),
	}}
	unmatched1 := radixv1.RadixBatch{ObjectMeta: metav1.ObjectMeta{
		Name:      "unmatched1",
		Namespace: ns(app, env),
		Labels:    radixlabels.Merge(radixlabels.ForApplicationName(app), radixlabels.ForComponentName("othercomp"), radixlabels.ForBatchType(kube.RadixBatchTypeJob)),
	}}
	unmatched2 := radixv1.RadixBatch{ObjectMeta: metav1.ObjectMeta{
		Name:      "unmatched2",
		Namespace: ns(app, env),
		Labels:    radixlabels.Merge(radixlabels.ForComponentName(comp), radixlabels.ForBatchType(kube.RadixBatchTypeJob)),
	}}
	unmatched3 := radixv1.RadixBatch{ObjectMeta: metav1.ObjectMeta{
		Name:      "unmatched3",
		Namespace: ns(app, env),
		Labels:    radixlabels.Merge(radixlabels.ForApplicationName(app), radixlabels.ForBatchType(kube.RadixBatchTypeJob)),
	}}
	unmatched4 := radixv1.RadixBatch{ObjectMeta: metav1.ObjectMeta{
		Name:      "unmatched4",
		Namespace: ns(app, "otherenv"),
		Labels:    radixlabels.Merge(radixlabels.ForApplicationName(app), radixlabels.ForComponentName(comp), radixlabels.ForBatchType(kube.RadixBatchTypeJob)),
	}}
	unmatched5 := radixv1.RadixBatch{ObjectMeta: metav1.ObjectMeta{
		Name:      "unmatched5",
		Namespace: ns(app, env),
		Labels:    radixlabels.Merge(radixlabels.ForApplicationName(app), radixlabels.ForComponentName(comp)),
	}}

	client := radixfake.NewSimpleClientset() //nolint:staticcheck
	applyRb := func(rb *radixv1.RadixBatch) error {
		_, err := client.RadixV1().RadixBatches(rb.Namespace).Create(context.Background(), rb, metav1.CreateOptions{})
		return err
	}
	require.NoError(t, applyRb(&matchjob1))
	require.NoError(t, applyRb(&matchjob2))
	require.NoError(t, applyRb(&matchbatch1))
	require.NoError(t, applyRb(&unmatched1))
	require.NoError(t, applyRb(&unmatched2))
	require.NoError(t, applyRb(&unmatched3))
	require.NoError(t, applyRb(&unmatched4))
	require.NoError(t, applyRb(&unmatched5))

	// Get batches of type job
	actual, err := kubequery.GetRadixBatchesForJobComponent(context.Background(), client, app, env, comp, kube.RadixBatchTypeJob)
	require.NoError(t, err)
	expected := []radixv1.RadixBatch{matchjob1, matchjob2}
	assert.ElementsMatch(t, expected, actual)

	// Get batches of type batch
	actual, err = kubequery.GetRadixBatchesForJobComponent(context.Background(), client, app, env, comp, kube.RadixBatchTypeBatch)
	require.NoError(t, err)
	expected = []radixv1.RadixBatch{matchbatch1}
	assert.ElementsMatch(t, expected, actual)
}

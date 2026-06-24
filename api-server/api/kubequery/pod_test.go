package kubequery

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func Test_GetPodsForEnvironmentComponents(t *testing.T) {
	matched1 := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "matched1", Namespace: "app1-env1", Labels: labels.ForApplicationName("app1")}}
	matched2 := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "matched2", Namespace: "app1-env1", Labels: labels.ForApplicationName("app1")}}
	unmatched1 := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "unmatched1", Namespace: "app1-env1", Labels: labels.ForApplicationName("app2")}}
	unmatched2 := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "unmatched2", Namespace: "app1-env2", Labels: labels.ForApplicationName("app1")}}
	unmatched3 := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "unmatched3", Namespace: "app1-env1"}}
	unmatched4 := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "unmatched4", Namespace: "app1-env1", Labels: labels.Merge(labels.ForApplicationName("app1"), map[string]string{kube.RadixBatchNameLabel: "any"})}}
	unmatched5 := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "unmatched5", Namespace: "app1-env1", Labels: labels.Merge(labels.ForApplicationName("app1"), map[string]string{kube.RadixJobTypeLabel: "any"})}}
	client := kubefake.NewSimpleClientset(&matched1, &matched2, &unmatched1, &unmatched2, &unmatched3, &unmatched4, &unmatched5) //nolint:staticcheck
	expected := []corev1.Pod{matched1, matched2}
	actual, err := GetPodsForEnvironmentComponents(context.Background(), client, "app1", "env1")
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, actual)
}

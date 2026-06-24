package kubequery

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func Test_GetHorizontalPodAutoscalersForEnvironment(t *testing.T) {
	matched1 := autoscalingv2.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "matched1", Namespace: "app1-env1", Labels: labels.ForApplicationName("app1")}}
	matched2 := autoscalingv2.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "matched2", Namespace: "app1-env1", Labels: labels.ForApplicationName("app1")}}
	unmatched1 := autoscalingv2.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "unmatched1", Namespace: "app1-env1"}}
	unmatched2 := autoscalingv2.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "unmatched2", Namespace: "app1-env1", Labels: labels.ForApplicationName("app2")}}
	unmatched3 := autoscalingv2.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "unmatched3", Namespace: "app1-env2", Labels: labels.ForApplicationName("app1")}}
	client := kubefake.NewSimpleClientset(&matched1, &matched2, &unmatched1, &unmatched2, &unmatched3) //nolint:staticcheck
	expected := []autoscalingv2.HorizontalPodAutoscaler{matched1, matched2}
	actual, err := GetHorizontalPodAutoscalersForEnvironment(context.Background(), client, "app1", "env1")
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, actual)
}

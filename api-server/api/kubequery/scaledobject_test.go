package kubequery_test

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/api-server/api/kubequery"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_GetScaledObjectsForEnvironment(t *testing.T) {
	matched1 := v1alpha1.ScaledObject{ObjectMeta: metav1.ObjectMeta{Name: "matched1", Namespace: "app1-env1", Labels: labels.ForApplicationName("app1")}}
	matched2 := v1alpha1.ScaledObject{ObjectMeta: metav1.ObjectMeta{Name: "matched2", Namespace: "app1-env1", Labels: labels.ForApplicationName("app1")}}
	unmatched1 := v1alpha1.ScaledObject{ObjectMeta: metav1.ObjectMeta{Name: "unmatched1", Namespace: "app1-env1"}}
	unmatched2 := v1alpha1.ScaledObject{ObjectMeta: metav1.ObjectMeta{Name: "unmatched2", Namespace: "app1-env1", Labels: labels.ForApplicationName("app2")}}
	unmatched3 := v1alpha1.ScaledObject{ObjectMeta: metav1.ObjectMeta{Name: "unmatched3", Namespace: "app1-env2", Labels: labels.ForApplicationName("app1")}}
	client := kedafake.NewSimpleClientset(&matched1, &matched2, &unmatched1, &unmatched2, &unmatched3)
	expected := []v1alpha1.ScaledObject{matched1, matched2}
	actual, err := kubequery.GetScaledObjectsForEnvironment(context.Background(), client, "app1", "env1")
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, actual)
}

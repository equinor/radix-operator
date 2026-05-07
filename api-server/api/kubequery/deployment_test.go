package kubequery

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func Test_GetDeploymentsForEnvironment(t *testing.T) {
	matched1 := appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "matched1", Namespace: "app1-env1", Labels: labels.ForApplicationName("app1")}}
	matched2 := appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "matched2", Namespace: "app1-env1", Labels: labels.ForApplicationName("app1")}}
	unmatched1 := appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "unmatched1", Namespace: "app1-env1"}}
	unmatched2 := appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "unmatched2", Namespace: "app1-env1", Labels: labels.ForApplicationName("app2")}}
	unmatched3 := appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "unmatched3", Namespace: "app1-env2", Labels: labels.ForApplicationName("app1")}}
	client := kubefake.NewSimpleClientset(&matched1, &matched2, &unmatched1, &unmatched2, &unmatched3) //nolint:staticcheck
	expected := []appsv1.Deployment{matched1, matched2}
	actual, err := GetDeploymentsForEnvironment(context.Background(), client, "app1", "env1")
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, actual)
}

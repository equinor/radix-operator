package kubequery

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func Test_GetSecretsForEnvironment(t *testing.T) {
	matched1 := corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "matched1", Namespace: "app1-env1"}}
	matched2 := corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "matched2", Namespace: "app1-env1"}}
	unmatched := corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "unmatched", Namespace: "app2-env1"}}
	jobPayload := corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "payload", Namespace: "app1-env1", Labels: map[string]string{kube.RadixSecretTypeLabel: string(kube.RadixSecretJobPayload)}}}
	client := kubefake.NewSimpleClientset(&matched1, &matched2, &unmatched, &jobPayload) //nolint:staticcheck

	expected := []corev1.Secret{matched1, matched2}
	noJobPayloadReq, err := labels.NewRequirement(kube.RadixSecretTypeLabel, selection.NotEquals, []string{string(kube.RadixSecretJobPayload)})
	require.NoError(t, err)

	actual, err := GetSecretsForEnvironment(context.Background(), client, "app1", "env1", *noJobPayloadReq)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, actual)
}

package kubequery

import (
	"context"
	"testing"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certclientfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_GetCertificateRequestsForEnvironment(t *testing.T) {
	matched1 := cmv1.CertificateRequest{ObjectMeta: metav1.ObjectMeta{Name: "matched1", Namespace: "app1-env1"}}
	matched2 := cmv1.CertificateRequest{ObjectMeta: metav1.ObjectMeta{Name: "matched2", Namespace: "app1-env1"}}
	unmatched1 := cmv1.CertificateRequest{ObjectMeta: metav1.ObjectMeta{Name: "unmatched1", Namespace: "app1-env2"}}
	unmatched2 := cmv1.CertificateRequest{ObjectMeta: metav1.ObjectMeta{Name: "unmatched2", Namespace: "app1-env3"}}
	unmatched3 := cmv1.CertificateRequest{ObjectMeta: metav1.ObjectMeta{Name: "unmatched3", Namespace: "app1-env4"}}
	client := certclientfake.NewSimpleClientset(&matched1, &matched2, &unmatched1, &unmatched2, &unmatched3)
	expected := []cmv1.CertificateRequest{matched1, matched2}
	actual, err := GetCertificateRequestsForEnvironment(context.Background(), client, "app1", "env1")
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, actual)
}

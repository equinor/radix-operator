package kubequery

import (
	"context"
	"testing"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certclientfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetCertificatesForEnvironment(t *testing.T) {
	anyExternalDNS := v1.RadixDeployExternalDNS{FQDN: "any.domain.com"}
	matched1 := cmv1.Certificate{Name: "matched1", Namespace: "app1-env1", Labels: labels.ForExternalDNSResource("app1", anyExternalDNS)}
	matched2 := cmv1.Certificate{Name: "matched2", Namespace: "app1-env1", Labels: labels.ForExternalDNSResource("app1", anyExternalDNS)}
	unmatched1 := cmv1.Certificate{Name: "unmatched1", Namespace: "app1-env1", Labels: labels.ForExternalDNSResource("app2", anyExternalDNS)}
	unmatched2 := cmv1.Certificate{Name: "unmatched2", Namespace: "app1-env2", Labels: labels.ForExternalDNSResource("app1", anyExternalDNS)}
	unmatched3 := cmv1.Certificate{Name: "unmatched3", Namespace: "app1-env1"}
	client := certclientfake.NewSimpleClientset(&matched1, &matched2, &unmatched1, &unmatched2, &unmatched3)
	expected := []cmv1.Certificate{matched1, matched2}
	actual, err := GetCertificatesForEnvironment(context.Background(), client, "app1", "env1")
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, actual)
}

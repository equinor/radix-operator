package deployment

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func TestLoadIngressConfigFromMap(t *testing.T) {
	ingressConfig := `
configuration:
  - name: foo
    annotations:
      foo1: x
  - name: bar
    annotations:
      bar1: x
      bar2: y
`
	ingressCm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: ingressConfigurationMap, Namespace: corev1.NamespaceDefault},
		Data:       map[string]string{"ingressConfiguration": ingressConfig},
	}
	kubeutil, _ := kube.New(kubefake.NewSimpleClientset(&ingressCm), nil)
	expected := []AnnotationConfiguration{
		{Name: "foo", Annotations: map[string]string{"foo1": "x"}},
		{Name: "bar", Annotations: map[string]string{"bar1": "x", "bar2": "y"}},
	}
	actual, _ := loadIngressConfigFromMap(kubeutil)
	assert.ElementsMatch(t, expected, actual.AnnotationConfigurations)

}
func TestGetAnnotationsFromConfigurations_ReturnsCorrectConfig(t *testing.T) {
	config := IngressConfiguration{
		AnnotationConfigurations: []AnnotationConfiguration{
			{Name: "ewma", Annotations: map[string]string{"ewma1": "x", "ewma2": "y"}},
			{Name: "socket", Annotations: map[string]string{"socket1": "x", "socket2": "y", "socket3": "z"}},
			{Name: "round-robin", Annotations: map[string]string{"round-robin1": "1"}},
		},
	}
	componentIngress := componentIngressConfigurationAnnotations{config: config}

	annotations := componentIngress.GetAnnotations(&v1.RadixDeployComponent{IngressConfiguration: []string{"socket"}})
	assert.Equal(t, 3, len(annotations))

	annotations = componentIngress.GetAnnotations(&v1.RadixDeployComponent{IngressConfiguration: []string{"socket", "round-robin"}})
	assert.Equal(t, 4, len(annotations))

	annotations = componentIngress.GetAnnotations(&v1.RadixDeployComponent{IngressConfiguration: []string{"non-existing"}})
	assert.Equal(t, 0, len(annotations))
}

func TestGetAuthenticationAnnotationsFromConfiguration(t *testing.T) {
	verification := v1.VerificationTypeOptional

	expect1 := make(map[string]string)
	expect1["nginx.ingress.kubernetes.io/auth-tls-pass-certificate-to-upstream"] = "true"
	expect1["nginx.ingress.kubernetes.io/auth-tls-verify-client"] = string(v1.VerificationTypeOff)
	expect1["nginx.ingress.kubernetes.io/auth-tls-secret"] = utils.GetComponentClientCertificateSecretName("ns/name")

	expect2 := make(map[string]string)
	expect2["nginx.ingress.kubernetes.io/auth-tls-pass-certificate-to-upstream"] = "false"
	expect2["nginx.ingress.kubernetes.io/auth-tls-verify-client"] = string(v1.VerificationTypeOff)

	expect3 := make(map[string]string)
	expect3["nginx.ingress.kubernetes.io/auth-tls-pass-certificate-to-upstream"] = "false"
	expect3["nginx.ingress.kubernetes.io/auth-tls-verify-client"] = string(verification)
	expect3["nginx.ingress.kubernetes.io/auth-tls-secret"] = utils.GetComponentClientCertificateSecretName("ns/name")

	config1 := &v1.Authentication{
		ClientCertificate: &v1.ClientCertificate{
			PassCertificateToUpstream: utils.BoolPtr(true),
		},
	}

	config2 := &v1.Authentication{
		ClientCertificate: &v1.ClientCertificate{
			PassCertificateToUpstream: utils.BoolPtr(false),
		},
	}

	config3 := &v1.Authentication{
		ClientCertificate: &v1.ClientCertificate{
			Verification: &verification,
		},
	}

	ingressAnnotations := clientCertificateAnnotations{namespace: "ns"}
	result := ingressAnnotations.GetAnnotations(&v1.RadixDeployComponent{Name: "name", Authentication: config1})
	assert.Equal(t, expect1, result)

	result = ingressAnnotations.GetAnnotations(&v1.RadixDeployComponent{Name: "name", Authentication: config2})
	assert.Equal(t, expect2, result)

	result = ingressAnnotations.GetAnnotations(&v1.RadixDeployComponent{Name: "name", Authentication: config3})
	assert.Equal(t, expect3, result)

	result = ingressAnnotations.GetAnnotations(&v1.RadixDeployComponent{Name: "name"})
	assert.Empty(t, result, "Expected Annotations to be empty")
}

package deployment

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
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

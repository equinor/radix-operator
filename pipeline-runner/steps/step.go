package steps

import (
	"github.com/coreos/prometheus-operator/pkg/client/monitoring"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

// RadixStepHandler Instance variables
type RadixStepHandler struct {
	kubeclient               kubernetes.Interface
	radixclient              radixclient.Interface
	prometheusOperatorClient monitoring.Interface
	kubeutil                 *kube.Kube
}

// Init constructor
func Init(kubeclient kubernetes.Interface, radixclient radixclient.Interface, prometheusOperatorClient monitoring.Interface) (RadixStepHandler, error) {
	kube, err := kube.New(kubeclient)
	if err != nil {
		return RadixStepHandler{}, err
	}

	handler := RadixStepHandler{
		kubeclient:               kubeclient,
		radixclient:              radixclient,
		prometheusOperatorClient: prometheusOperatorClient,
		kubeutil:                 kube,
	}

	return handler, nil
}

package internal

import (
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

func setupTest() *kube.Kube {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixClient := radixfake.NewSimpleClientset()
	secretProviderClient := secretproviderfake.NewSimpleClientset()
	kubeUtil, _ := kube.New(
		kubeclient,
		radixClient,
		secretProviderClient,
	)
	return kubeUtil
}

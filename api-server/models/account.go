package models

import (
	certclient "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	kedav2 "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	secretProviderClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
)

// Account Holds kubernetes account sessions
type Account struct {
	Client               kubernetes.Interface
	RadixClient          radixclient.Interface
	SecretProviderClient secretProviderClient.Interface
	TektonClient         tektonclient.Interface
	CertManagerClient    certclient.Interface
	KedaClient           kedav2.Interface
}

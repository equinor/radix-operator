package models

import (
	kedav2 "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"

	certclient "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	secretProviderClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
)

// NewAccounts creates a new Accounts struct
func NewAccounts(inClusterClient kubernetes.Interface, inClusterRadixClient radixclient.Interface, inClusterKedaClient kedav2.Interface, inClusterSecretProviderClient secretProviderClient.Interface, inClusterTektonClient tektonclient.Interface, inClusterCertManagerClient certclient.Interface, outClusterClient kubernetes.Interface, outClusterRadixClient radixclient.Interface, outClusterKedaClient kedav2.Interface, outClusterSecretProviderClient secretProviderClient.Interface, outClusterTektonClient tektonclient.Interface, outClusterCertManagerClient certclient.Interface) Accounts {

	return Accounts{
		UserAccount: Account{
			Client:               outClusterClient,
			RadixClient:          outClusterRadixClient,
			KedaClient:           outClusterKedaClient,
			SecretProviderClient: outClusterSecretProviderClient,
			TektonClient:         outClusterTektonClient,
			CertManagerClient:    outClusterCertManagerClient,
		},
		ServiceAccount: Account{
			Client:               inClusterClient,
			RadixClient:          inClusterRadixClient,
			KedaClient:           inClusterKedaClient,
			SecretProviderClient: inClusterSecretProviderClient,
			TektonClient:         inClusterTektonClient,
			CertManagerClient:    inClusterCertManagerClient,
		},
	}
}

func NewServiceAccount(inClusterClient kubernetes.Interface, inClusterRadixClient radixclient.Interface, inClusterKedaClient kedav2.Interface, inClusterSecretProviderClient secretProviderClient.Interface, inClusterTektonClient tektonclient.Interface, inClusterCertManagerClient certclient.Interface) Account {
	return Account{
		Client:               inClusterClient,
		RadixClient:          inClusterRadixClient,
		SecretProviderClient: inClusterSecretProviderClient,
		TektonClient:         inClusterTektonClient,
		CertManagerClient:    inClusterCertManagerClient,
		KedaClient:           inClusterKedaClient,
	}
}

// Accounts contains accounts for accessing k8s API.
type Accounts struct {
	UserAccount    Account
	ServiceAccount Account
}

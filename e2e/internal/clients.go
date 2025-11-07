package internal

import (
	"fmt"

	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Clients holds all Kubernetes clients needed for testing
type Clients struct {
	KubeClient  kubernetes.Interface
	RadixClient radixclient.Interface
	Config      *rest.Config
}

// NewClients creates a new set of clients from a rest.Config
func NewClients(config *rest.Config) (*Clients, error) {
	// Create kubernetes client
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Create radix client
	radixClient, err := radixclient.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create radix client: %w", err)
	}

	return &Clients{
		KubeClient:  kubeClient,
		RadixClient: radixClient,
		Config:      config,
	}, nil
}

package internal

import (
	"fmt"

	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Clients holds all Kubernetes clients needed for testing
type Clients struct {
	Client      client.Client         // Controller-runtime client from manager
	RadixClient radixclient.Interface // Typed Radix client for CRD operations
	Config      *rest.Config
}

// NewClients creates a new set of clients from a manager client and rest.Config
func NewClients(c client.Client, config *rest.Config) (*Clients, error) {
	// Create radix client
	radixClient, err := radixclient.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create radix client: %w", err)
	}

	return &Clients{
		Client:      c,
		RadixClient: radixClient,
		Config:      config,
	}, nil
}

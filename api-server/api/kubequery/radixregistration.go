package kubequery

import (
	"context"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetRadixRegistration returns the RadixRegistration for the specified application name.
func GetRadixRegistration(ctx context.Context, client radixclient.Interface, appName string) (*radixv1.RadixRegistration, error) {
	return client.RadixV1().RadixRegistrations().Get(ctx, appName, metav1.GetOptions{})
}

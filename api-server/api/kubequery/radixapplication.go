package kubequery

import (
	"context"

	"github.com/equinor/radix-operator/api-server/api/utils/access"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	authorizationapi "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetRadixApplication returns the RadixApplication for the specified application name.
func GetRadixApplication(ctx context.Context, client radixclient.Interface, appName string) (*radixv1.RadixApplication, error) {
	ns := operatorUtils.GetAppNamespace(appName)
	return client.RadixV1().RadixApplications(ns).Get(ctx, appName, metav1.GetOptions{})
}

func IsRadixApplicationAdmin(ctx context.Context, kubeClient kubernetes.Interface, appName string) (bool, error) {
	return access.HasAccess(ctx, kubeClient, &authorizationapi.ResourceAttributes{
		Verb:     "patch",
		Group:    radixv1.GroupName,
		Resource: radixv1.ResourceRadixRegistrations,
		Version:  "*",
		Name:     appName,
	})
}

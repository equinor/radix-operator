package dnsalias

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/utils"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CreateRadixDNSAliasIngress Create an Ingress for a RadixDNSAlias
func CreateRadixDNSAliasIngress(kubeClient kubernetes.Interface, appName, envName string, ingress *networkingv1.Ingress) (*networkingv1.Ingress, error) {
	return kubeClient.NetworkingV1().Ingresses(utils.GetEnvironmentNamespace(appName, envName)).Create(context.Background(), ingress, metav1.CreateOptions{})
}

// GetDNSAliasIngressName Gets name of the ingress for the custom DNS alias
func GetDNSAliasIngressName(alias string) string {
	return fmt.Sprintf("%s.custom-alias", alias)
}

// GetDNSAliasHost Gets DNS alias host.
// Example for the alias "my-app" and the cluster "Playground": my-app.playground.radix.equinor.com
func GetDNSAliasHost(alias, dnsZone string) string {
	return fmt.Sprintf("%s.%s", alias, dnsZone)
}

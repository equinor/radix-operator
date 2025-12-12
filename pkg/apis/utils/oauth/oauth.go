package oauth

import (
	"fmt"
	"net/url"
	"path"
	"strings"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// SanitizePathPrefix adds a leading "/" and strips away any trailing "/"
// Each path element in the input is path escaped
func SanitizePathPrefix(prefix string) string {
	// Add "/" to make prefix path absolute. Join calls Clean internally to find the shortest path
	// See path.Clean for details
	sanitizedPath := path.Join("/", prefix)
	parts := strings.Split(sanitizedPath, "/")
	var escapedParts []string
	for _, p := range parts {
		escapedParts = append(escapedParts, url.PathEscape(p))
	}

	return strings.Join(escapedParts, "/")
}

// GetAuxOAuthProxyIngressName Get an ingress name for the auxiliary OAuth proxy component
func GetAuxOAuthProxyIngressName(sourceIngressName string) string {
	return fmt.Sprintf("%s-%s", sourceIngressName, radixv1.OAuthProxyAuxiliaryComponentSuffix)
}

// MergeAuxOAuthProxyComponentResourceLabels  Merge labels for object and aux OAuth proxy
func MergeAuxOAuthProxyComponentResourceLabels(object metav1.Object, appName string, component radixv1.RadixCommonDeployComponent) {
	object.SetLabels(labels.Merge(object.GetLabels(), radixlabels.ForAuxOAuthProxyComponent(appName, component)))
}

// MergeAuxOAuthRedisComponentResourceLabels  Merge labels for object and aux Redis
func MergeAuxOAuthRedisComponentResourceLabels(object metav1.Object, appName string, component radixv1.RadixCommonDeployComponent) {
	object.SetLabels(labels.Merge(object.GetLabels(), radixlabels.ForAuxOAuthRedisComponent(appName, component)))
}

// MergeAuxComponentDefaultAliasIngressLabels  Merge labels for default alias ingress and aux OAuth proxy
func MergeAuxComponentDefaultAliasIngressLabels(auxIngress *networkingv1.Ingress, appName string, component radixv1.RadixCommonDeployComponent) {
	auxIngress.SetLabels(labels.Merge(auxIngress.GetLabels(), radixlabels.ForAuxComponentDefaultIngress(appName, component)))
}

// MergeAuxComponentActiveClusterAliasIngressLabels  Merge labels for active cluster alias ingress and aux OAuth proxy
func MergeAuxComponentActiveClusterAliasIngressLabels(auxIngress *networkingv1.Ingress, appName string, component radixv1.RadixCommonDeployComponent) {
	auxIngress.SetLabels(labels.Merge(auxIngress.GetLabels(), radixlabels.ForAuxComponentActiveClusterAliasIngress(appName, component)))
}

// MergeAuxComponentAppAliasIngressLabels  Merge labels for app alias ingress and aux OAuth proxy
func MergeAuxComponentAppAliasIngressLabels(auxIngress *networkingv1.Ingress, appName string, component radixv1.RadixCommonDeployComponent) {
	auxIngress.SetLabels(labels.Merge(auxIngress.GetLabels(), radixlabels.ForAuxComponentAppAliasIngress(appName, component)))
}

// MergeAuxComponentExternalAliasIngressLabels  Merge labels for external alias ingress and aux OAuth proxy
func MergeAuxComponentExternalAliasIngressLabels(auxIngress *networkingv1.Ingress, appName string, component radixv1.RadixCommonDeployComponent) {
	auxIngress.SetLabels(labels.Merge(auxIngress.GetLabels(), radixlabels.ForAuxComponentExternalAliasIngress(appName, component)))
}

// MergeAuxComponentDNSAliasIngressResourceLabels  Merge labels for ingress and aux DNS alias OAuth proxy
func MergeAuxComponentDNSAliasIngressResourceLabels(auxIngress *networkingv1.Ingress, appName string, component radixv1.RadixCommonDeployComponent, dnsAlias string) {
	auxIngress.SetLabels(labels.Merge(auxIngress.GetLabels(), radixlabels.ForAuxComponentDNSAliasIngress(appName, component, dnsAlias)))
}

package annotations

import (
	"fmt"
	"strconv"
	"strings"

	maputils "github.com/equinor/radix-common/utils/maps"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

const (
	azureWorkloadIdentityClientIdAnnotation = "azure.workload.identity/client-id"
	clusterAutoscaleSafeToEvictAnnotation   = "cluster-autoscaler.kubernetes.io/safe-to-evict"

	// PreviewOAuth2ProxyModeAnnotation is used to indicate if the preview OAuth2 proxy mode is enabled.
	// Comma separated list of target environments where its enabled.
	PreviewOAuth2ProxyModeAnnotation = "radix.equinor.com/preview-oauth2-proxy-mode"
)

// Merge multiple maps into one
func Merge(annotations ...map[string]string) map[string]string {
	return maputils.MergeMaps(annotations...)
}

// ForRadixBranch returns annotations describing a branch name
func ForRadixBranch(branch string) map[string]string {
	return map[string]string{kube.RadixBranchAnnotation: branch}
}

// ForRadixDeploymentName returns annotations describing a Radix deployment name
func ForRadixDeploymentName(deploymentName string) map[string]string {
	return map[string]string{kube.RadixDeploymentNameAnnotation: deploymentName}
}

// ForKubernetesDeploymentObservedGeneration returns annotations describing which RadixDeployment version was used while syncing
func ForKubernetesDeploymentObservedGeneration(rd *radixv1.RadixDeployment) map[string]string {
	if rd == nil {
		return map[string]string{}
	}

	return map[string]string{kube.RadixDeploymentObservedGeneration: strconv.Itoa(int(rd.ObjectMeta.Generation))}
}

// ForServiceAccountWithRadixIdentityClientId returns annotations for configuring a ServiceAccount with external identities,
// e.g. for Azure Workload Identity: "azure.workload.identity/client-id": "11111111-2222-3333-4444-555555555555"
func ForServiceAccountWithRadixIdentityClientId(identityClientId string) map[string]string {
	if len(identityClientId) == 0 {
		return nil
	}
	return forAzureWorkloadIdentityClientId(identityClientId)
}

// ForClusterAutoscalerSafeToEvict returns annotation used by cluster autoscaler to determine if a Pod
// can be evicted from a node in case of scale down.
// safeToEvict=false: The pod can not be evicted, and the Pod's node cannot be removed until Pod is completed
// safeToEvict=true: The pod can be evicted, and the Pod's node can be removed in scale-down scenario
// Cluster Autoscaler docs: https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/FAQ.md#what-are-the-parameters-to-ca
func ForClusterAutoscalerSafeToEvict(safeToEvict bool) map[string]string {
	return map[string]string{clusterAutoscaleSafeToEvictAnnotation: fmt.Sprint(safeToEvict)}
}

func forAzureWorkloadIdentityClientId(clientId string) map[string]string {
	return map[string]string{azureWorkloadIdentityClientIdAnnotation: clientId}
}

// Oauth2PreviewModeEnabledForEnvironment checks if the preview OAuth2 proxy mode is enabled for a given environment
func Oauth2PreviewModeEnabledForEnvironment(annotations map[string]string, currentEnv string) bool {
	if annotations == nil {
		return false
	}
	modes, exists := annotations[PreviewOAuth2ProxyModeAnnotation]
	if !exists {
		return false
	}

	for _, targetEnv := range strings.Split(modes, ",") {
		if strings.TrimSpace(targetEnv) == currentEnv {
			return true
		}
	}
	return false
}

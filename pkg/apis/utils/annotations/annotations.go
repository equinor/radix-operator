package annotations

import (
	"fmt"

	maputils "github.com/equinor/radix-common/utils/maps"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

const (
	azureWorkloadIdentityClientIdAnnotation = "azure.workload.identity/client-id"
	clusterAutoscaleSafeToEvictAnnotation   = "cluster-autoscaler.kubernetes.io/safe-to-evict"
)

// Merge multiple maps into one
func Merge(labels ...map[string]string) map[string]string {
	return maputils.MergeMaps(labels...)
}

// ForRadixBranch returns annotations describing a branch name
func ForRadixBranch(branch string) map[string]string {
	return map[string]string{kube.RadixBranchAnnotation: branch}
}

// ForRadixDeploymentName returns annotations describing a Radix deployment name
func ForRadixDeploymentName(deploymentName string) map[string]string {
	return map[string]string{kube.RadixDeploymentNameAnnotation: deploymentName}
}

// ForServiceAccountWithRadixIdentity returns annotations for configuring a ServiceAccount with external identities,
// e.g. for Azure Workload Identity: "azure.workload.identity/client-id": "11111111-2222-3333-4444-555555555555"
func ForServiceAccountWithRadixIdentity(identity *radixv1.Identity) map[string]string {
	if identity == nil {
		return nil
	}

	var annotations map[string]string

	if identity.Azure != nil {
		annotations = Merge(annotations, forAzureWorkloadIdentityClientId(identity.Azure.ClientId))
	}

	return annotations
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

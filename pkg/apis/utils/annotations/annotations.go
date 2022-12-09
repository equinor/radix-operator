package annotations

import (
	maputils "github.com/equinor/radix-common/utils/maps"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

const (
	podAppArmorAnnotation                   = "apparmor.security.beta.kubernetes.io/pod"
	azureWorkloadIdentityClientIdAnnotation = "azure.workload.identity/client-id"
)

// Merge multiple maps into one
func Merge(labels ...map[string]string) map[string]string {
	return maputils.MergeMaps(labels...)
}

// ForPodAppArmorRuntimeDefault returns annotations for configuring a pod
// to run with AppArmor profile "runtime/default"
func ForPodAppArmorRuntimeDefault() map[string]string {
	return map[string]string{podAppArmorAnnotation: "runtime/default"}
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

func forAzureWorkloadIdentityClientId(clientId string) map[string]string {
	return map[string]string{azureWorkloadIdentityClientIdAnnotation: clientId}
}

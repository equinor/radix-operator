package predicate

import (
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"k8s.io/apimachinery/pkg/labels"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

func IsSecretProviderClassForDeployment(deploymentName string) func(spc secretsstorev1.SecretProviderClass) bool {
	selector := labels.Set{kube.RadixDeploymentLabel: deploymentName}.AsSelector()
	return func(spc secretsstorev1.SecretProviderClass) bool {
		return selector.Matches(labels.Set(spc.Labels))
	}
}

package predicate

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/stretchr/testify/assert"
	secretstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

func Test_IsSecretProviderClassForDeployment(t *testing.T) {
	sut := IsSecretProviderClassForDeployment("deploy1")
	assert.True(t, sut(secretstorev1.SecretProviderClass{Labels: map[string]string{kube.RadixDeploymentLabel: "deploy1"}}))
	assert.False(t, sut(secretstorev1.SecretProviderClass{Labels: map[string]string{kube.RadixDeploymentLabel: "deploy2"}}))
	assert.False(t, sut(secretstorev1.SecretProviderClass{}))
}

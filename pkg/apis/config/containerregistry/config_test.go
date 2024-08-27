package containerregistry_test

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/config/containerregistry"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func Test_ImagePullSecretsFromDefaultAuth(t *testing.T) {
	cfg := containerregistry.Config{DefaultAuthSecret: ""}
	assert.Len(t, cfg.ImagePullSecretsFromDefaultAuth(), 0)

	secretName := "a-secret"
	cfg = containerregistry.Config{DefaultAuthSecret: secretName}
	expected := []corev1.LocalObjectReference{{Name: secretName}}
	assert.ElementsMatch(t, expected, cfg.ImagePullSecretsFromDefaultAuth())
}

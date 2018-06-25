package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_RetrieveDockerConfig(t *testing.T) {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "radix-docker",
			Namespace: "default",
		},
		Data: map[string][]byte{
			".dockerconfigjson": []byte("{\"auths\":{\"radixdev.azurecr.io\":{\"username\":\"testuser\",\"password\":\"mysecretpassword\",\"email\":\"frode.hus@outlook.com\",\"auth\":\"asdKJfdfTlU=\"}}}"),
		},
	}

	fakeClient := fake.NewSimpleClientset(secret)
	kubeutil, _ := New(fakeClient)

	creds, err := kubeutil.RetrieveContainerRegistryCredentials()
	assert.NoError(t, err)
	assert.Equal(t, "radixdev.azurecr.io", creds.Server)
	assert.Equal(t, "testuser", creds.User)
	assert.Equal(t, "mysecretpassword", creds.Password)
}

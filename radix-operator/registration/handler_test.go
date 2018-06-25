package registration

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/statoil/radix-operator/radix-operator/common"

	log "github.com/Sirupsen/logrus"

	"github.com/stretchr/testify/assert"

	fakeradix "github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	kubetest "k8s.io/client-go/testing"
)

func init() {
	log.SetOutput(ioutil.Discard)
}

func Test_RadixRegistrationHandler(t *testing.T) {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "radix-docker",
			Namespace: "default",
		},
		Data: map[string][]byte{
			".dockerconfigjson": []byte("{\"auths\":{\"radixdev.azurecr.io\":{\"username\":\"testuser\",\"password\":\"mysecretpassword\",\"email\":\"frode.hus@outlook.com\",\"auth\":\"asdKJfdfTlU=\"}}}"),
		},
	}

	client := fake.NewSimpleClientset(secret)
	radixClient := fakeradix.NewSimpleClientset()
	registration, _ := common.GetRadixRegistrationFromFile("testdata/sampleregistration.yaml")
	handler := NewRegistrationHandler(client)
	radixClient.PrependReactor("get", "radixregistrations", func(action kubetest.Action) (handled bool, ret runtime.Object, err error) {
		return true, registration, nil
	})

	t.Run("It creates a registration", func(t *testing.T) {
		err := handler.ObjectCreated(registration)
		assert.NoError(t, err)
		ns, err := client.CoreV1().Namespaces().Get(fmt.Sprintf("%s-app", registration.Name), metav1.GetOptions{})
		assert.NoError(t, err)
		assert.NotNil(t, ns)
	})
	t.Run("It updates a registration", func(t *testing.T) {
		err := handler.ObjectUpdated(nil, registration)
		assert.NoError(t, err)
	})
}

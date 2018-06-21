package application

import (
	"io/ioutil"
	"testing"

	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	"github.com/statoil/radix-operator/radix-operator/common"

	log "github.com/Sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/stretchr/testify/assert"

	fakeradix "github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	kubetest "k8s.io/client-go/testing"
)

func init() {
	log.SetOutput(ioutil.Discard)
}

func Test_RadixAppHandler(t *testing.T) {
	client := fake.NewSimpleClientset()
	radixClient := fakeradix.NewSimpleClientset()
	radixApp, _ := common.GetRadixAppFromFile("testdata/radixconfig.yaml")
	registration, _ := common.GetRadixRegistrationFromFile("../registration/testdata/sampleregistration.yaml")
	handler := NewApplicationHandler(client, radixClient)
	radixClient.PrependReactor("get", "radixregistrations", func(action kubetest.Action) (handled bool, ret runtime.Object, err error) {
		return true, registration, nil
	})

	t.Run("It creates environments", func(t *testing.T) {
		handler.ObjectCreated(radixApp)
		ns, err := client.CoreV1().Namespaces().Get("testapp-dev", metav1.GetOptions{})
		assert.NoError(t, err)
		assert.NotNil(t, ns)
	})

	t.Run("It creates additional environments when updated", func(t *testing.T) {
		newEnv := v1.Environment{
			Name: "test",
		}
		radixApp.Spec.Environments = append(radixApp.Spec.Environments, newEnv)
		handler.ObjectUpdated(nil, radixApp)
		ns, err := client.CoreV1().Namespaces().Get("testapp-test", metav1.GetOptions{})
		assert.NoError(t, err)
		assert.NotNil(t, ns)
	})

	t.Run("It removes deleted environments", func(t *testing.T) {
		radixApp.Spec.Environments = []v1.Environment{}
		handler.ObjectUpdated(nil, radixApp)
		ns, err := client.CoreV1().Namespaces().Get("testapp-dev", metav1.GetOptions{})
		assert.True(t, errors.IsNotFound(err))
		assert.Nil(t, ns)
	})
}

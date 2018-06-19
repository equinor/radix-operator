package registration

import (
	"io/ioutil"
	"testing"

	log "github.com/Sirupsen/logrus"

	fakeradix "github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/statoil/radix-operator/radix-operator/common"
	"github.com/stretchr/testify/assert"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type FakeHandler struct {
	operation chan string
}

func (h *FakeHandler) Init() error {
	return nil
}
func (h *FakeHandler) ObjectCreated(obj interface{}) error {
	h.operation <- "created"
	return nil
}
func (h *FakeHandler) ObjectDeleted(key string) error{
	h.operation <- "deleted"
	return nil
}
func (h *FakeHandler) ObjectUpdated(objOld, objNew interface{}) error{
	h.operation <- "updated"
	return nil
}
func init() {
	log.SetOutput(ioutil.Discard)
}

func Test_Controller_Calls_Handler(t *testing.T) {
	client := fake.NewSimpleClientset()
	radixClient := fakeradix.NewSimpleClientset()

	registration, err := common.GetRadixRegistrationFromFile("testdata/sampleregistration.yaml")
	if err != nil {
		log.Fatalf("Could not read configuration data: %v", err)
	}

	fakeHandler := &FakeHandler{
		operation: make(chan string),
	}

	controller := NewController(client, radixClient, fakeHandler)

	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(stop)

	t.Run("Create app", func(t *testing.T) {
		registeredApp, err := radixClient.RadixV1().RadixRegistrations("default").Create(registration)
		assert.NoError(t, err)
		assert.NotNil(t, registeredApp)
		assert.Equal(t, "created", <-fakeHandler.operation)
	})
	t.Run("Update app", func(t *testing.T) {
		var err error
		registration.ObjectMeta.Annotations = map[string]string{
			"update": "test",
		}
		updatedApp, err := radixClient.RadixV1().RadixRegistrations("default").Update(registration)
		assert.NoError(t, err)
		assert.NotNil(t, updatedApp)
		assert.NotNil(t, updatedApp.Annotations)
		assert.Equal(t, "test", updatedApp.Annotations["update"])
	})
	t.Run("Delete app", func(t *testing.T) {
		var err error
		<-fakeHandler.operation
		err = radixClient.RadixV1().RadixRegistrations("default").Delete(registration.Name, &meta.DeleteOptions{})
		assert.NoError(t, err)
		assert.Equal(t, "deleted", <-fakeHandler.operation)
	})
}

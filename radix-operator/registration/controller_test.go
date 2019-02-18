package registration

import (
	"io/ioutil"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/utils"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
)

type FakeHandler struct {
	operation chan string
}

func (h *FakeHandler) Init() error {
	return nil
}
func (h *FakeHandler) Sync(namespace, name string, eventRecorder record.EventRecorder) error {
	h.operation <- "synced"
	return nil
}

func (h *FakeHandler) ObjectCreated(obj interface{}) error {
	h.operation <- "created"
	return nil
}
func (h *FakeHandler) ObjectDeleted(key string) error {
	h.operation <- "deleted"
	return nil
}
func (h *FakeHandler) ObjectUpdated(objOld, objNew interface{}) error {
	h.operation <- "updated"
	return nil
}
func init() {
	log.SetOutput(ioutil.Discard)
}

func Test_Controller_Calls_Handler(t *testing.T) {
	client := fake.NewSimpleClientset()
	radixClient := fakeradix.NewSimpleClientset()

	registration, err := utils.GetRadixRegistrationFromFile("testdata/sampleregistration.yaml")
	if err != nil {
		log.Fatalf("Could not read configuration data: %v", err)
	}

	fakeHandler := &FakeHandler{
		operation: make(chan string),
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, 0)
	registrationInformerFactory := informers.NewSharedInformerFactory(radixClient, 0)
	eventRecorder := &record.FakeRecorder{}

	controller := NewController(client, radixClient, fakeHandler,
		registrationInformerFactory.Radix().V1().RadixRegistrations(),
		kubeInformerFactory.Core().V1().Namespaces(), eventRecorder)

	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(1, stop)

	t.Run("Create app", func(t *testing.T) {
		registeredApp, err := radixClient.RadixV1().RadixRegistrations("default").Create(registration)
		assert.NoError(t, err)
		assert.NotNil(t, registeredApp)
		assert.Equal(t, "created", <-fakeHandler.operation)
		close(stop)
	})

	/*
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
		})*/
}

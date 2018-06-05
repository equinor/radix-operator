package main

import (
	"io/ioutil"
	"testing"

	log "github.com/Sirupsen/logrus"

	radix_v1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
	fakeradix "github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
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
func (h *FakeHandler) ObjectCreated(obj interface{}) {
	h.operation <- "created"
}
func (h *FakeHandler) ObjectDeleted(key string) {
	h.operation <- "deleted"
}
func (h *FakeHandler) ObjectUpdated(objOld, objNew interface{}) {
	h.operation <- "updated"
}
func init() {
	log.SetOutput(ioutil.Discard)
}

func Test_Controller_Calls_Handler(t *testing.T) {
	radixApp, controller, radixClient, fakeHandler := initializeTest()

	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(stop)

	var createdApp *radix_v1.RadixApplication

	t.Run("Create app", func(t *testing.T) {
		var err error
		createdApp, err = radixClient.RadixV1().RadixApplications("DefaultNS").Create(radixApp)
		assert.NoError(t, err)
		assert.Equal(t, "created", <-fakeHandler.operation)
	})
}

func TestControllerUpdate(t *testing.T) {
	radixApp, controller, radixClient, _ := initializeTest()

	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(stop)

	var createdApp *radix_v1.RadixApplication

	t.Run("Update app", func(t *testing.T) {
		var err error
		createdApp, err = radixClient.RadixV1().RadixApplications("DefaultNS").Create(radixApp)
		createdApp.ObjectMeta.Annotations = map[string]string{
			"update": "test",
		}
		updatedApp, err := radixClient.RadixV1().RadixApplications("DefaultNS").Update(createdApp)
		assert.NoError(t, err)
		assert.NotNil(t, updatedApp)
		assert.NotNil(t, updatedApp.Annotations)
		assert.Equal(t, "test", updatedApp.Annotations["update"])
	})
}

func TestControllerDelete(t *testing.T) {
	radixApp, controller, radixClient, fakeHandler := initializeTest()

	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(stop)

	var createdApp *radix_v1.RadixApplication

	t.Run("Delete app", func(t *testing.T) {
		var err error
		createdApp, err = radixClient.RadixV1().RadixApplications("DefaultNS").Create(radixApp)
		<-fakeHandler.operation
		err = radixClient.RadixV1().RadixApplications("DefaultNS").Delete(createdApp.Name, &meta.DeleteOptions{})
		assert.NoError(t, err)
		assert.Equal(t, "deleted", <-fakeHandler.operation)
	})
}

func initializeTest() (*radix_v1.RadixApplication, *Controller, *fakeradix.Clientset, *FakeHandler) {
	client := fake.NewSimpleClientset()
	radixClient := fakeradix.NewSimpleClientset()

	radixApp := &radix_v1.RadixApplication{
		ObjectMeta: meta.ObjectMeta{
			Name: "testapp",
		},
		Spec: radix_v1.RadixApplicationSpec{
			Secrets: nil,
		},
	}

	fakeHandler := &FakeHandler{
		operation: make(chan string),
	}
	controller := NewController(client, radixClient, fakeHandler)

	return radixApp, controller, radixClient, fakeHandler
}

package main

import (
	"testing"

	radix_v1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
	fakeradix "github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type FakeHandler struct {
	operation chan string
}

func TestController(t *testing.T) {
	client := fake.NewSimpleClientset()
	radixClient := fakeradix.NewSimpleClientset()

	radixApp := &radix_v1.RadixApplication{
		ObjectMeta: meta.ObjectMeta{
			Name: "testapp",
		},
		Spec: radix_v1.RadixApplicationSpec{
			Image: "testing:0.1",
		},
	}

	controller := NewController(client, radixClient)
	fakeHandler := &FakeHandler{
		operation: make(chan string),
	}
	controller.handler = fakeHandler
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

	t.Run("Update app", func(t *testing.T) {
		createdApp.ObjectMeta.Annotations = map[string]string{
			"update": "test",
		}
		updatedApp, err := radixClient.RadixV1().RadixApplications("DefaultNS").Update(createdApp)
		assert.NoError(t, err)
		assert.NotNil(t, updatedApp)
		assert.NotNil(t, updatedApp.Annotations)
		assert.Equal(t, "test", updatedApp.Annotations["update"])
	})

	t.Run("Delete app", func(t *testing.T) {
		err := radixClient.RadixV1().RadixApplications("DefaultNS").Delete(createdApp.Name, &meta.DeleteOptions{})
		assert.NoError(t, err)
		assert.Equal(t, "deleted", <-fakeHandler.operation)
	})

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

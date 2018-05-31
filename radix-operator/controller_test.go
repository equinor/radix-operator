package main

import (
	"testing"
	"time"

	radix_v1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
	fakeradix "github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
)

type FakeHandler struct {
	WasUpdated bool
	WasCreated bool
	WasDeleted bool
}

func TestController(t *testing.T) {
	appCreated := false
	appUpdated := false
	appDeleted := false
	client := fake.NewSimpleClientset()
	radixClient := fakeradix.NewSimpleClientset()
	radixClient.PrependReactor("create", "radixapplications", func(action core.Action) (bool, runtime.Object, error) {
		appCreated = true
		return false, nil, nil
	})

	radixClient.PrependReactor("update", "radixapplications", func(action core.Action) (bool, runtime.Object, error) {
		appUpdated = true
		return false, nil, nil
	})

	radixClient.PrependReactor("delete", "radixapplications", func(action core.Action) (bool, runtime.Object, error) {
		appDeleted = true
		return false, nil, nil
	})

	radixApp := &radix_v1.RadixApplication{
		ObjectMeta: meta.ObjectMeta{
			Name: "testapp",
		},
		Spec: radix_v1.RadixApplicationSpec{
			Image: "testing:0.1",
		},
	}

	controller := NewController(client, radixClient)
	fakeHandler := &FakeHandler{}
	controller.handler = fakeHandler
	stop := make(chan struct{})
	defer close(stop)

	go controller.Run(stop)
	createdApp, err := radixClient.RadixV1().RadixApplications("DefaultNS").Create(radixApp)
	assert.NoError(t, err)
	wait.Poll(100*time.Millisecond, wait.ForeverTestTimeout, func() (bool, error) {
		return appCreated, nil
	})
	radixClient.RadixV1().RadixApplications("DefaultNS").Update(createdApp)
	wait.Poll(100*time.Millisecond, wait.ForeverTestTimeout, func() (bool, error) {
		return appUpdated, nil
	})
	radixClient.RadixV1().RadixApplications("DefaultNS").Delete(createdApp.Name, &meta.DeleteOptions{})

	wait.Poll(100*time.Millisecond, wait.ForeverTestTimeout, func() (bool, error) {
		return appDeleted, nil
	})

	assert.True(t, fakeHandler.WasCreated)
}

func (h *FakeHandler) Init() error {
	return nil
}
func (h *FakeHandler) ObjectCreated(obj interface{}) {
	h.WasCreated = true
}
func (h *FakeHandler) ObjectDeleted(key string) {
	h.WasDeleted = true
}
func (h *FakeHandler) ObjectUpdated(objOld, objNew interface{}) {
	h.WasUpdated = true
}

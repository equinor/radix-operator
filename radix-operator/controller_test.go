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

func TestController(t *testing.T) {
	appCreated := false
	client := fake.NewSimpleClientset()
	radixClient := fakeradix.NewSimpleClientset()
	radixClient.PrependReactor("create", "radixapplications", func(action core.Action) (bool, runtime.Object, error) {
		appCreated = true
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
	stop := make(chan struct{})
	defer close(stop)

	go controller.Run(stop)
	createdApp, err := radixClient.RadixV1().RadixApplications("DefaultNS").Create(radixApp)
	wait.Poll(200*time.Millisecond, wait.ForeverTestTimeout, func() (bool, error) {
		return appCreated, nil
	})
	assert.NoError(t, err)
	assert.NotNil(t, createdApp)
}

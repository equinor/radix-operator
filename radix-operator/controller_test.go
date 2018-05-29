package main

import (
	"testing"
	"time"

	radix_v1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
	fakeradix "github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
)

func TestController(t *testing.T) {
	client := fake.NewSimpleClientset()
	radixClient := fakeradix.NewSimpleClientset()
	radixClient.AddWatchReactor("radixapplication", func(action core.Action) (bool, watch.Interface, error) {
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
	radixClient.RadixV1().RadixApplications("DefaultNS").Create(radixApp)
	wait.Poll(200*time.Millisecond, wait.ForeverTestTimeout, func() (bool, error) {
		return true, nil
	})

}

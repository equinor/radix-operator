package registration

import (
	"io/ioutil"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/utils"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
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
	radixInformerFactory := informers.NewSharedInformerFactory(radixClient, 0)
	eventRecorder := &record.FakeRecorder{}

	controller := NewController(client, radixClient, fakeHandler,
		radixInformerFactory.Radix().V1().RadixRegistrations(),
		kubeInformerFactory.Core().V1().Namespaces(), eventRecorder)

	stop := make(chan struct{})
	defer close(stop)

	kubeInformerFactory.Start(stop)
	radixInformerFactory.Start(stop)
	go controller.Run(1, stop)

	t.Run("Create app", func(t *testing.T) {
		registeredApp, err := radixClient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Create(registration)

		assert.NoError(t, err)
		assert.NotNil(t, registeredApp)

		op, ok := <-fakeHandler.operation
		assert.True(t, ok)
		assert.Equal(t, "synced", op)
	})

	t.Run("Update app", func(t *testing.T) {
		var err error
		registration.ObjectMeta.Annotations = map[string]string{
			"update": "test",
		}
		updatedApp, err := radixClient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Update(registration)

		op, ok := <-fakeHandler.operation
		assert.True(t, ok)
		assert.Equal(t, "synced", op)

		assert.NoError(t, err)
		assert.NotNil(t, updatedApp)
		assert.NotNil(t, updatedApp.Annotations)
		assert.Equal(t, "test", updatedApp.Annotations["update"])
	})
}

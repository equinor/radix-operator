package registration

import (
	"reflect"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixinformer "github.com/equinor/radix-operator/pkg/client/informers/externalversions/radix/v1"
	"github.com/equinor/radix-operator/radix-operator/common"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

var logger *log.Entry

const (
	controllerAgentName = "registration-controller"
	crType              = "RadixRegistrations"
)

func init() {
	logger = log.WithFields(log.Fields{"radixOperatorComponent": "registration-controller"})
}

//NewController creates a new controller that handles RadixRegistrations
func NewController(client kubernetes.Interface,
	radixClient radixclient.Interface, handler common.Handler,
	registrationInformer radixinformer.RadixRegistrationInformer,
	namespaceInformer coreinformers.NamespaceInformer,
	recorder record.EventRecorder) *common.Controller {

	controller := &common.Controller{
		Name:        controllerAgentName,
		KubeClient:  client,
		RadixClient: radixClient,
		Informer:    registrationInformer.Informer(),
		WorkQueue:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), crType),
		Handler:     handler,
		Log:         logger,
		Recorder:    recorder,
	}

	logger.Info("Setting up event handlers")

	registrationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(new interface{}) {
			controller.Enqueue(new)
			controller.CustomResourceAdded(crType)
		},
		UpdateFunc: func(old, new interface{}) {
			newRR := new.(*v1.RadixRegistration)
			oldRR := old.(*v1.RadixRegistration)

			if deepEqual(oldRR, newRR) {
				logger.Debugf("Registration object is equal to old for %s. Do nothing", newRR.GetName())
				return
			}

			controller.Enqueue(new)
		},
		DeleteFunc: func(obj interface{}) {
			radixRegistration, _ := obj.(*v1.RadixRegistration)
			key, err := cache.MetaNamespaceKeyFunc(radixRegistration)
			if err == nil {
				logger.Debugf("Registration object deleted event received for %s. Do nothing", key)
			}
			controller.CustomResourceDeleted(crType)
		},
	})

	namespaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ns := obj.(*corev1.Namespace)

			// May need to sync ad-groups
			controller.HandleObject(ns, "RadixRegistration", getObject)
		},
		UpdateFunc: func(old, new interface{}) {
			newNs := new.(*corev1.Namespace)
			oldNs := old.(*corev1.Namespace)
			if newNs.ResourceVersion == oldNs.ResourceVersion {
				return
			}
			controller.HandleObject(new, "RadixRegistration", getObject)
		},
		DeleteFunc: func(obj interface{}) {
			controller.HandleObject(obj, "RadixRegistration", getObject)
		},
	})

	return controller
}

func deepEqual(old, new *v1.RadixRegistration) bool {
	if !reflect.DeepEqual(new.Spec, old.Spec) ||
		!reflect.DeepEqual(new.ObjectMeta.Labels, old.ObjectMeta.Labels) ||
		!reflect.DeepEqual(new.ObjectMeta.Annotations, old.ObjectMeta.Annotations) {
		return false
	}

	return true
}

func getObject(radixClient radixclient.Interface, namespace, name string) (interface{}, error) {
	return radixClient.RadixV1().RadixRegistrations().Get(name, metav1.GetOptions{})
}

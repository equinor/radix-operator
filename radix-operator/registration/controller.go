package registration

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixinformer "github.com/equinor/radix-operator/pkg/client/informers/externalversions/radix/v1"
	"github.com/equinor/radix-operator/radix-operator/common"
	log "github.com/sirupsen/logrus"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

var logger *log.Entry

func init() {
	logger = log.WithFields(log.Fields{"radixOperatorComponent": "registration-controller"})
}

//NewController creates a new controller that handles RadixRegistrations
func NewController(client kubernetes.Interface,
	radixClient radixclient.Interface, handler common.Handler,
	registrationInformer radixinformer.RadixRegistrationInformer,
	namespaceInformer coreinformers.NamespaceInformer) *common.Controller {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	registrationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			radixRegistration, ok := obj.(*v1.RadixRegistration)
			if !ok {
				logger.Errorf("Provided object was not a valid Radix Registration; instead was %v", obj)
				return
			}

			logger = logger.WithFields(log.Fields{"registrationName": radixRegistration.ObjectMeta.Name, "registrationNamespace": radixRegistration.ObjectMeta.Namespace})

			key, err := cache.MetaNamespaceKeyFunc(radixRegistration)
			if err == nil {
				logger.Infof("Adding radix registration created event to queue: %s", key)
				queue.Add(common.QueueItem{Key: key, Operation: common.Add})
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			newRadixRegistration, ok := newObj.(*v1.RadixRegistration)
			if !ok {
				logger.Errorf("New object was not a valid Radix Registration; instead was %v", newObj)
				return
			}

			logger = logger.WithFields(log.Fields{"registrationName": newRadixRegistration.ObjectMeta.Name, "registrationNamespace": newRadixRegistration.ObjectMeta.Namespace})

			key, err := cache.MetaNamespaceKeyFunc(oldObj)
			if err == nil {
				logger.Infof("Adding radix registration updated event to queue: %s", key)
				queue.Add(common.QueueItem{Key: key, OldObject: oldObj, Operation: common.Update})
			}
		},
		DeleteFunc: func(obj interface{}) {
			radixRegistration, ok := obj.(*v1.RadixRegistration)
			if !ok {
				logger.Errorf("Provided object was not a valid Radix Registration; instead was %v", obj)
				return
			}

			logger = logger.WithFields(log.Fields{"registrationName": radixRegistration.ObjectMeta.Name, "registrationNamespace": radixRegistration.ObjectMeta.Namespace})

			key, err := cache.MetaNamespaceKeyFunc(radixRegistration)
			if err == nil {
				logger.Infof("Adding radix registration deleted event to queue: %s", key)
				queue.Add(common.QueueItem{Key: key, Operation: common.Delete})
			}
		},
	})

	controller := &common.Controller{
		KubeClient:  client,
		RadixClient: radixClient,
		Informer:    registrationInformer.Informer(),
		Queue:       queue,
		Handler:     handler,
		Log:         logger,
	}
	return controller
}

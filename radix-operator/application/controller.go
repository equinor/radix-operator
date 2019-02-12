package application

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixinformer "github.com/equinor/radix-operator/pkg/client/informers/externalversions/radix/v1"
	"github.com/equinor/radix-operator/radix-operator/common"
	log "github.com/sirupsen/logrus"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// ApplicationController Instance variables
type ApplicationController struct {
	clientset   kubernetes.Interface
	radixclient radixclient.Interface
	queue       workqueue.RateLimitingInterface
	informer    cache.SharedIndexInformer
	handler     common.Handler
}

var logger *log.Entry

func init() {
	logger = log.WithFields(log.Fields{"radixOperatorComponent": "application-controller"})
}

// NewApplicationController creates a new controller that handles RadixDeployments
func NewApplicationController(client kubernetes.Interface, radixClient radixclient.Interface, handler common.Handler) *common.Controller {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	informer := radixinformer.NewRadixApplicationInformer(
		radixClient,
		meta_v1.NamespaceAll,
		0,
		cache.Indexers{},
	)

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			radixApplication, ok := obj.(*v1.RadixApplication)
			if !ok {
				logger.Errorf("Provided object was not a valid Radix Application; instead was %v", obj)
				return
			}

			logger = logger.WithFields(log.Fields{"applicationName": radixApplication.ObjectMeta.Name, "applicationNamespace": radixApplication.ObjectMeta.Namespace})

			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				logger.Infof("Adding radix application created event to queue: %s", key)
				queue.Add(common.QueueItem{Key: key, Operation: common.Add})
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			radixApplication, ok := newObj.(*v1.RadixApplication)
			if !ok {
				logger.Errorf("Provided object was not a valid Radix Application; instead was %v", newObj)
				return
			}

			logger = logger.WithFields(log.Fields{"applicationName": radixApplication.ObjectMeta.Name, "applicationNamespace": radixApplication.ObjectMeta.Namespace})

			key, err := cache.MetaNamespaceKeyFunc(oldObj)
			if err == nil {
				logger.Infof("Adding radix application updated event to queue: %s", key)
				queue.Add(common.QueueItem{Key: key, OldObject: oldObj, Operation: common.Update})
			}
		},
		DeleteFunc: func(obj interface{}) {
			radixApplication, ok := obj.(*v1.RadixApplication)
			if !ok {
				logger.Errorf("Provided object was not a valid Radix Application; instead was %v", obj)
				return
			}

			logger = logger.WithFields(log.Fields{"applicationName": radixApplication.ObjectMeta.Name, "applicationNamespace": radixApplication.ObjectMeta.Namespace})

			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				logger.Infof("Adding radix application deleted event to queue: %s", key)
				queue.Add(common.QueueItem{Key: key, Operation: common.Delete})
			}
		},
	})

	controller := &common.Controller{
		KubeClient:  client,
		RadixClient: radixClient,
		Informer:    informer,
		Queue:       queue,
		Handler:     handler,
		Log:         logger,
	}
	return controller
}

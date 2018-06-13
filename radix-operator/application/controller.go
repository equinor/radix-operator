package application

import (
	log "github.com/Sirupsen/logrus"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	radixinformer "github.com/statoil/radix-operator/pkg/client/informers/externalversions/radix/v1"
	"github.com/statoil/radix-operator/radix-operator/common"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"k8s.io/client-go/util/workqueue"
)

func NewController(client kubernetes.Interface, radixClient radixclient.Interface) *common.Controller {
	handler := NewApplicationHandler(client, radixClient)

	informer := radixinformer.NewRadixApplicationInformer(
		radixClient,
		meta_v1.NamespaceAll,
		0,
		cache.Indexers{},
	)
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			log.Infof("Added radix application: %s", key)
			if err == nil {
				queue.Add(key)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(oldObj)
			log.Infof("Updated radix application: %s", key)
			if err == nil {
				queue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			log.Infof("Deleted radix application: %s", key)
			if err == nil {
				queue.Add(key)
			}
		},
	})

	controller := &common.Controller{
		KubeClient:  client,
		RadixClient: radixClient,
		Informer:    informer,
		Queue:       queue,
		Handler:     &handler,
		Log:         log.WithField("controller", "application"),
	}
	return controller
}

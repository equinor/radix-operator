package main

import (
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	radixinformer "github.com/statoil/radix-operator/pkg/client/informers/externalversions/radix/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type AppController struct {
	clientset   kubernetes.Interface
	radixclient radixclient.Interface
	queue       workqueue.RateLimitingInterface
	informer    cache.SharedIndexInformer
	handler     Handler
}

func NewAppController(client kubernetes.Interface, radixClient radixclient.Interface, handler Handler) *AppController {
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

	controller := &AppController{
		clientset:   client,
		radixclient: radixClient,
		informer:    informer,
		queue:       queue,
		handler:     handler,
	}
	return controller
}

func (c *AppController) Run(stop <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	log.Info("AppController.Run: initiating")

	go c.informer.Run(stop)

	if !cache.WaitForCacheSync(stop, c.HasSynced) {
		utilruntime.HandleError(fmt.Errorf("Error syncing cache"))
		return
	}
	log.Info("AppController.Run: cache sync complete")

	wait.Until(c.runWorker, time.Second, stop)
}

func (c *AppController) HasSynced() bool {
	return c.informer.HasSynced()
}

func (c *AppController) runWorker() {
	log.Info("AppController.runWorker: starting")
	for c.processNextItem() {
		log.Info("AppController.runWorker: processing next item")
	}
	log.Info("AppController.runWorker: completed")
}

func (c *AppController) processNextItem() bool {
	log.Info("AppController.processNextItem: start")
	key, quit := c.queue.Get()

	if quit {
		return false
	}

	defer c.queue.Done(key)
	keyRaw := key.(string)

	item, exists, err := c.informer.GetIndexer().GetByKey(keyRaw)
	if err != nil {
		if c.queue.NumRequeues(key) < 5 {
			log.Errorf("AppController.processNextItem: Failed processing item with key %s with error %v, retrying", key, err)
			c.queue.AddRateLimited(key)
		} else {
			log.Errorf("AppController.processNextItem: Failed processing item with key %s with error %v, no more retries", key, err)
			c.queue.Forget(key)
			utilruntime.HandleError(err)
		}
	}

	if !exists {
		log.Infof("AppController.processNextItem: object deletion detected: %s", keyRaw)
		c.handler.ObjectDeleted(keyRaw)
	} else {
		log.Infof("AppController.processNextItem: object creation detected: %s", keyRaw)
		c.handler.ObjectCreated(item)
	}
	c.queue.Forget(key)
	return true
}

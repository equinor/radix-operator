package main

import (
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type Controller struct {
	logger    *log.Entry
	clientset kubernetes.Interface
	queue     workqueue.RateLimitingInterface
	informer  cache.SharedIndexInformer
	handler   Handler
}

func (c *Controller) Run(stop <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	c.logger.Info("Controller.Run: initiating")

	go c.informer.Run(stop)

	if !cache.WaitForCacheSync(stop, c.HasSynced) {
		utilruntime.HandleError(fmt.Errorf("Error syncing cache"))
		return
	}
	c.logger.Info("Controller.Run: cache sync complete")

	wait.Until(c.runWorker, time.Second, stop)
}

func (c *Controller) HasSynced() bool {
	return c.informer.HasSynced()
}

func (c *Controller) runWorker() {
	log.Info("Controller.runWorker: starting")
	for c.processNextItem() {
		log.Info("Controller.runWorker: processing next item")
	}
	log.Info("Controller.runWorker: completed")
}

func (c *Controller) processNextItem() bool {
	log.Info("Controller.processNextItem: start")
	key, quit := c.queue.Get()

	if quit {
		return false
	}

	defer c.queue.Done(key)
	keyRaw := key.(string)

	item, exists, err := c.informer.GetIndexer().GetByKey(keyRaw)
	if err != nil {
		if c.queue.NumRequeues(key) < 5 {
			c.logger.Errorf("Controller.processNextItem: Failed processing item with key %s with error %v, retrying", key, err)
			c.queue.AddRateLimited(key)
		} else {
			c.logger.Errorf("Controller.processNextItem: Failed processing item with key %s with error %v, no more retries", key, err)
			c.queue.Forget(key)
			utilruntime.HandleError(err)
		}
	}

	if !exists {
		c.logger.Infof("Controller.processNextItem: object deleted detected: %s", keyRaw)
		c.handler.ObjectDeleted(item)
		c.queue.Forget(key)
	} else {
		c.logger.Infof("Controller.processNextItem: object created detected: %s", keyRaw)
		c.handler.ObjectCreated(item)
		c.queue.Forget(key)
	}
	return true
}

package common

import (
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type Controller struct {
	KubeClient  kubernetes.Interface
	RadixClient radixclient.Interface
	Queue       workqueue.RateLimitingInterface
	Informer    cache.SharedIndexInformer
	Handler     Handler
}

func (c *Controller) Run(stop <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.Queue.ShutDown()

	log.Info("Controller.Run: initiating")

	go c.Informer.Run(stop)

	if !cache.WaitForCacheSync(stop, c.HasSynced) {
		utilruntime.HandleError(fmt.Errorf("Error syncing cache"))
		return
	}
	log.Info("Controller.Run: cache sync complete")

	wait.Until(c.runWorker, time.Second, stop)
}

func (c *Controller) HasSynced() bool {
	return c.Informer.HasSynced()
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
	key, quit := c.Queue.Get()

	if quit {
		return false
	}

	defer c.Queue.Done(key)
	keyRaw := key.(string)

	item, exists, err := c.Informer.GetIndexer().GetByKey(keyRaw)
	if err != nil {
		if c.Queue.NumRequeues(key) < 5 {
			log.Errorf("Controller.processNextItem: Failed processing item with key %s with error %v, retrying", key, err)
			c.Queue.AddRateLimited(key)
		} else {
			log.Errorf("Controller.processNextItem: Failed processing item with key %s with error %v, no more retries", key, err)
			c.Queue.Forget(key)
			utilruntime.HandleError(err)
		}
	}

	if !exists {
		log.Infof("Controller.processNextItem: object deletion detected: %s", keyRaw)
		c.Handler.ObjectDeleted(keyRaw)
	} else {
		log.Infof("Controller.processNextItem: object creation detected: %s", keyRaw)
		c.Handler.ObjectCreated(item)
	}
	c.Queue.Forget(key)
	return true
}

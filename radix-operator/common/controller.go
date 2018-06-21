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
	Log         *log.Entry
}

func (c *Controller) Run(stop <-chan struct{}) {
	c.Log.Info("Starting controller")
	defer utilruntime.HandleCrash()
	defer c.Queue.ShutDown()
	go c.Informer.Run(stop)

	if !cache.WaitForCacheSync(stop, c.HasSynced) {
		utilruntime.HandleError(fmt.Errorf("Error syncing cache"))
		return
	}
	wait.Until(c.runWorker, time.Second, stop)
}

func (c *Controller) HasSynced() bool {
	return c.Informer.HasSynced()
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
		c.Log.Info("Controller.runWorker: processing next item")
	}
}

func (c *Controller) processNextItem() bool {
	o, quit := c.Queue.Get()
	queueItem := o.(QueueItem)
	if quit {
		return false
	}

	defer c.Queue.Done(o)

	item, exists, err := c.Informer.GetIndexer().GetByKey(queueItem.Key)
	if err != nil {
		if c.Queue.NumRequeues(o) < 5 {
			c.Log.Errorf("Controller.processNextItem: Failed processing item with key %s with error %v, retrying", o, err)
			c.Queue.AddRateLimited(o)
		} else {
			c.Log.Errorf("Controller.processNextItem: Failed processing item with key %s with error %v, no more retries", o, err)
			c.Queue.Forget(o)
			utilruntime.HandleError(err)
		}
	}

	if !exists {
		c.Log.Infof("Controller.processNextItem: object deletion detected: %s", queueItem.Key)
		c.Handler.ObjectDeleted(queueItem.Key)
	} else if queueItem.Operation == Add {
		c.Log.Infof("Controller.processNextItem: object creation detected: %s", queueItem.Key)
		c.Handler.ObjectCreated(item)
	} else if queueItem.Operation == Update {
		c.Log.Infof("Controller.processNextItem: object update detected: %s", queueItem.Key)
		c.Handler.ObjectUpdated(nil, item)
	} else {
		c.Log.Infof("Controller.processNextItem: unknown operation")
	}
	c.Queue.Forget(o)
	return true
}

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

// Controller Instance variables
type Controller struct {
	KubeClient  kubernetes.Interface
	RadixClient radixclient.Interface
	Queue       workqueue.RateLimitingInterface
	Informer    cache.SharedIndexInformer
	Handler     Handler
	Log         *log.Entry
}

// Run starts the shared informer, which will be stopped when stopCh is closed.
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

// HasSynced returns true if the shared informer's store has synced.
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
		err := c.Handler.ObjectCreated(item)
		if err != nil {
			log.Errorf("Failed to create object: %v", err)
		}
	} else if queueItem.Operation == Update {
		c.Log.Infof("Controller.processNextItem: object update detected: %s", queueItem.Key)
		err := c.Handler.ObjectUpdated(queueItem.OldObject, item)
		if err != nil {
			log.Errorf("Failed to create object: %v", err)
		}
	} else {
		c.Log.Infof("Controller.processNextItem: unknown operation")
	}
	c.Queue.Forget(o)
	return true
}

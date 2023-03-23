package common

import (
	"errors"
	"fmt"

	"time"

	"golang.org/x/sync/errgroup"

	"github.com/equinor/radix-operator/pkg/apis/metrics"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"

	log "github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

// Controller Instance variables
type Controller struct {
	Name                  string
	HandlerOf             string
	KubeClient            kubernetes.Interface
	RadixClient           radixclient.Interface
	WorkQueue             workqueue.RateLimitingInterface
	Informer              cache.SharedIndexInformer
	KubeInformerFactory   kubeinformers.SharedInformerFactory
	Handler               Handler
	Log                   *log.Entry
	WaitForChildrenToSync bool
	Recorder              record.EventRecorder
	LockKeyAndIdentifier  LockKeyAndIdentifierFunc
	locker                resourceLocker
}

// Run starts the shared informer, which will be stopped when stopCh is closed.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()

	if c.LockKeyAndIdentifier == nil {
		return errors.New("LockKeyandIdentifier must be set")
	}

	// Start the informer factories to begin populating the informer caches
	c.Log.Debugf("Starting %s", c.Name)

	cacheSyncs := []cache.InformerSynced{
		c.hasSynced,
	}

	c.Log.Debug("start WaitForChildrenToSync")
	if c.WaitForChildrenToSync {
		cacheSyncs = append(cacheSyncs,
			c.KubeInformerFactory.Core().V1().Namespaces().Informer().HasSynced,
			c.KubeInformerFactory.Core().V1().Secrets().Informer().HasSynced,
		)
	}
	c.Log.Debug("completed WaitForChildrenToSync")

	// Wait for the caches to be synced before starting workers
	c.Log.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh,
		cacheSyncs...); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	c.Log.Info("Starting workers")

	// Launch workers to process resources
	c.run(threadiness, stopCh)

	return nil
}

func (c *Controller) run(threadiness int, stopCh <-chan struct{}) {
	var errorGroup errgroup.Group
	errorGroup.SetLimit(threadiness)
	defer func() {
		c.Log.Info("Waiting for workers to complete")
		errorGroup.Wait()
		c.Log.Info("Workers completed")
	}()

	locker := c.locker
	if locker == nil {
		locker = &defaultResourceLocker{}
	}

	for c.processNext(&errorGroup, stopCh, locker) {
	}

	errorGroup.Wait()
}

func (c *Controller) processNext(errorGroup *errgroup.Group, stopCh <-chan struct{}, locker resourceLocker) bool {
	select {
	case <-stopCh:
		return false
	default:
	}

	workItem, shutdown := c.WorkQueue.Get()
	if shutdown || c.WorkQueue.ShuttingDown() {
		return false
	}

	errorGroup.Go(func() error {
		defer func() {
			c.WorkQueue.Done(workItem)
		}()

		if workItem == nil || fmt.Sprint(workItem) == "" {
			return nil
		}

		lockKey, identifier, err := c.LockKeyAndIdentifier(workItem)
		if err != nil {
			c.WorkQueue.Forget(workItem)
			metrics.CustomResourceRemovedFromQueue(c.HandlerOf)
			utilruntime.HandleError(err)
			metrics.OperatorError(c.HandlerOf, "work_queue", "error_workqueue_type")
			return nil
		}

		if !locker.TryGetLock(lockKey) {
			c.Log.Debugf("Lock for %s was busy, requeuing %s", lockKey, identifier)
			c.WorkQueue.AddRateLimited(identifier)
			return nil
		}
		defer func() {
			locker.ReleaseLock(lockKey)
			c.Log.Debugf("Released lock for %s after processing %s", lockKey, identifier)
		}()

		c.Log.Debugf("Acquired lock for %s, processing %s", lockKey, identifier)
		c.processWorkItem(workItem, identifier)
		return nil
	})

	return true
}

func (c *Controller) processWorkItem(workItem interface{}, workItemString string) {
	err := func(workItem interface{}) error {
		if err := c.syncHandler(workItemString); err != nil {
			c.WorkQueue.AddRateLimited(workItemString)
			metrics.OperatorError(c.HandlerOf, "work_queue", "requeuing")
			metrics.CustomResourceRemovedFromQueue(c.HandlerOf)
			metrics.CustomResourceUpdatedAndRequeued(c.HandlerOf)

			return fmt.Errorf("error syncing %s: %s, requeuing", workItemString, err.Error())
		}

		c.WorkQueue.Forget(workItem)
		metrics.CustomResourceRemovedFromQueue(c.HandlerOf)
		c.Log.Infof("Successfully synced %s", workItemString)
		return nil
	}(workItem)

	if err != nil {
		utilruntime.HandleError(err)
		metrics.OperatorError(c.HandlerOf, "process_next_work_item", "unhandled")
	}
}

func (c *Controller) syncHandler(key string) error {
	start := time.Now()

	defer func() {
		duration := time.Since(start)
		metrics.AddDurationOfReconciliation(c.HandlerOf, duration)
	}()

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		metrics.OperatorError(c.HandlerOf, "split_meta_namespace_key", "invalid_resource_key")
		return nil
	}

	err = c.Handler.Sync(namespace, name, c.Recorder)
	if err != nil {
		// DEBUG
		log.Errorf("Error while syncing: %v", err)

		utilruntime.HandleError(fmt.Errorf("problems syncing: %s", key))
		metrics.OperatorError(c.HandlerOf, "c_handler_sync", fmt.Sprintf("problems_sync_%s", key))
		return err
	}

	return nil
}

// Enqueue takes a resource and converts it into a namespace/name
// string which is then put onto the work queue
func (c *Controller) Enqueue(obj interface{}) (requeued bool, err error) {
	var key string
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		metrics.OperatorError(c.HandlerOf, "enqueue", fmt.Sprintf("problems_sync_%s", key))
		return requeued, err
	}

	requeued = (c.WorkQueue.NumRequeues(key) > 0)
	c.WorkQueue.AddRateLimited(key)
	return requeued, nil
}

// HandleObject ensures that when anything happens to object which any
// custom resouce is owner of, that custom resource is synced
func (c *Controller) HandleObject(obj interface{}, ownerKind string, getOwnerFn GetOwner) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			metrics.OperatorError(c.HandlerOf, "handle_object", "error_decoding_object")
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			metrics.OperatorError(c.HandlerOf, "handle_object", "error_decoding_object_tombstone")
			return
		}
		c.Log.Infof("Recovered deleted object %s from tombstone", object.GetName())
	}
	c.Log.Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		if ownerRef.Kind != ownerKind {
			return
		}

		obj, err := getOwnerFn(c.RadixClient, object.GetNamespace(), ownerRef.Name)
		if err != nil {
			c.Log.Debugf("Ignoring orphaned object %s of %s %s", object.GetSelfLink(), ownerKind, ownerRef.Name)
			return
		}

		requeued, err := c.Enqueue(obj)
		if err == nil && !requeued {
			metrics.CustomResourceUpdated(c.HandlerOf)
		}

		return
	}
}

func SelfOrObjFromDeletedFinalStateUnknown(obj interface{}) interface{} {
	if staleObj, stale := obj.(cache.DeletedFinalStateUnknown); stale {
		return staleObj.Obj
	}
	return obj
}

func (c *Controller) hasSynced() bool {
	return c.Informer.HasSynced()
}

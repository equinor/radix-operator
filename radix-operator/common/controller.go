package common

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/equinor/radix-operator/pkg/apis/metrics"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixscheme "github.com/equinor/radix-operator/pkg/client/clientset/versioned/scheme"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

type resourceLocker struct {
	l sync.Map
}

func (r *resourceLocker) TryGetLock(key string) bool {
	_, loaded := r.l.LoadOrStore(key, true)

	return !loaded
}

func (r *resourceLocker) ReleaseLock(key string) {
	r.l.Delete(key)
}

type LockKeyAndIdentifierFunc func(obj interface{}) (lockKey string, identifier string, err error)

func getStringFromObj(obj interface{}) (string, error) {
	var key string
	var ok bool
	if key, ok = obj.(string); !ok {
		return "", fmt.Errorf("expected string in workqueue but got %#v", obj)
	}
	return key, nil
}

func NamespacePartitionKey(obj interface{}) (lockKey string, identifier string, err error) {
	identifier, err = getStringFromObj(obj)
	if err != nil {
		return
	}
	lockKey, _, err = cache.SplitMetaNamespaceKey(identifier)
	return
}

func NamePartitionKey(obj interface{}) (lockKey string, identifier string, err error) {
	identifier, err = getStringFromObj(obj)
	if err != nil {
		return
	}
	_, lockKey, err = cache.SplitMetaNamespaceKey(identifier)
	return
}

// GetOwner Function pointer to pass to retrieve owner
type GetOwner func(radixclient.Interface, string, string) (interface{}, error)

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

// NewEventRecorder Creates an event recorder for controller
func NewEventRecorder(controllerAgentName string, events typedcorev1.EventInterface, logger *log.Entry) record.EventRecorder {
	radixscheme.AddToScheme(scheme.Scheme)
	logger.Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(logger.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: events})
	return eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})
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

	for c.processNext(&errorGroup, stopCh) {
	}
}

func (c *Controller) processNext(errorGroup *errgroup.Group, stopCh <-chan struct{}) bool {
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
		defer c.WorkQueue.Done(workItem)

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

		if !c.locker.TryGetLock(lockKey) {
			c.Log.Debugf("Lock for %s was busy, requeuing %s", lockKey, identifier)
			c.WorkQueue.AddRateLimited(identifier)
			return nil
		}
		defer func() {
			c.locker.ReleaseLock(lockKey)
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
		metrics.AddDurrationOfReconciliation(c.HandlerOf, duration)
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

func (c *Controller) hasSynced() bool {
	return c.Informer.HasSynced()
}

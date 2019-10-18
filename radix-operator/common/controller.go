package common

import (
	"fmt"
	"time"

	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixscheme "github.com/equinor/radix-operator/pkg/client/clientset/versioned/scheme"
	"github.com/equinor/radix-operator/radix-operator/metrics"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

// GetOwner Function pointer to pass to retrieve owner
type GetOwner func(radixclient.Interface, string, string) (interface{}, error)

// Controller Instance variables
type Controller struct {
	Name        string
	HandlerOf   string
	KubeClient  kubernetes.Interface
	RadixClient radixclient.Interface
	WorkQueue   workqueue.RateLimitingInterface
	Informer    cache.SharedIndexInformer
	Handler     Handler
	Log         *log.Entry
	Recorder    record.EventRecorder
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
	defer c.WorkQueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	c.Log.Debugf("Starting %s", c.Name)

	// Wait for the caches to be synced before starting workers
	c.Log.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.hasSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	c.Log.Info("Starting workers")
	// Launch workers to process resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	c.Log.Info("Started workers")
	<-stopCh
	c.Log.Info("Shutting down workers")

	return nil
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.WorkQueue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.WorkQueue.Done(obj)
		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			c.WorkQueue.Forget(obj)
			metrics.CustomResourceRemovedFromQueue(c.HandlerOf)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			metrics.OperatorError(c.HandlerOf, "work_queue", "error_workqueue_type")
			return nil
		}

		if err := c.syncHandler(key); err != nil {
			c.WorkQueue.AddRateLimited(key)
			metrics.OperatorError(c.HandlerOf, "work_queue", "requeuing")
			metrics.CustomResourceRemovedFromQueue(c.HandlerOf)
			metrics.CustomResourceUpdatedAndRequeued(c.HandlerOf)

			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}

		c.WorkQueue.Forget(obj)
		metrics.CustomResourceRemovedFromQueue(c.HandlerOf)
		c.Log.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		metrics.OperatorError(c.HandlerOf, "process_next_work_item", "unhandled")
		return true
	}

	return true
}

func (c *Controller) syncHandler(key string) error {
	start := time.Now()

	defer func() {
		duration := time.Since(start)
		metrics.AddDurrationOfReconciliation(c.HandlerOf, duration)
	}()

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Invalid resource key: %s", key))
		metrics.OperatorError(c.HandlerOf, "split_meta_namespace_key", "invalid_resource_key")
		return nil
	}

	err = c.Handler.Sync(namespace, name, c.Recorder)
	if err != nil {
		// DEBUG
		log.Errorf("Error while syncing: %v", err)

		utilruntime.HandleError(fmt.Errorf("Problems syncing: %s", key))
		metrics.OperatorError(c.HandlerOf, "c_handler_sync", fmt.Sprintf("problems_sync_%s", key))
		return err
	}

	return nil
}

// Enqueue takes a resource and converts it into a namespace/name
// string which is then put onto the work queue
func (c *Controller) Enqueue(obj interface{}) (bool, error) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		metrics.OperatorError(c.HandlerOf, "enqueue", fmt.Sprintf("problems_sync_%s", key))
		return false, err
	}

	c.WorkQueue.AddRateLimited(key)

	requeues := c.WorkQueue.NumRequeues(key)
	if requeues > 1 {
		return true, nil
	}

	return false, nil
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
		c.Log.Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	c.Log.Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		if ownerRef.Kind != ownerKind {
			return
		}

		obj, err := getOwnerFn(c.RadixClient, object.GetNamespace(), ownerRef.Name)
		if err != nil {
			c.Log.Infof("Ignoring orphaned object '%s' of %s '%s'", object.GetSelfLink(), ownerKind, ownerRef.Name)
			return
		}

		requeued, err := c.Enqueue(obj)
		if err == nil && !requeued {
			metrics.CustomResourceUpdatedAndRequeued(c.HandlerOf)
		}

		return
	}
}

func (c *Controller) hasSynced() bool {
	return c.Informer.HasSynced()
}

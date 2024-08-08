package common

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/metrics"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
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
	Name                 string
	HandlerOf            string
	KubeClient           kubernetes.Interface
	RadixClient          radixclient.Interface
	WorkQueue            workqueue.RateLimitingInterface
	KubeInformerFactory  kubeinformers.SharedInformerFactory
	RadixInformerFactory informers.SharedInformerFactory
	Handler              Handler
	Recorder             record.EventRecorder
	LockKeyAndIdentifier LockKeyAndIdentifierFunc
	locker               resourceLocker
}

// Run starts the shared informer, which will be stopped when stopCh is closed.
func (c *Controller) Run(ctx context.Context, threadiness int) error {
	defer utilruntime.HandleCrash()

	if c.LockKeyAndIdentifier == nil {
		return errors.New("LockKeyandIdentifier must be set")
	}

	// Start the informer factories to begin populating the informer caches
	ctx = log.Ctx(ctx).With().Str("controller", c.Name).Logger().WithContext(ctx)
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Starting")

	// Wait for the caches to be synced before starting workers
	logger.Info().Msg("Waiting for Kube informer caches to sync")
	c.KubeInformerFactory.WaitForCacheSync(ctx.Done())
	logger.Info().Msg("Waiting for Radix informer caches to sync")
	c.RadixInformerFactory.WaitForCacheSync(ctx.Done())
	logger.Info().Msg("Completed syncing informer caches")

	logger.Info().Msg("Starting workers")

	// Launch workers to process resources
	c.run(ctx, threadiness)

	return nil
}

func (c *Controller) run(ctx context.Context, threadiness int) {
	errorGroup, ctx := errgroup.WithContext(ctx)
	errorGroup.SetLimit(threadiness)
	defer func() {
		log.Ctx(ctx).Info().Msg("Waiting for workers to complete")
		if err := errorGroup.Wait(); err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("errGroup returned error")
		}
		log.Ctx(ctx).Info().Msg("Workers completed")
	}()

	locker := c.locker
	if locker == nil {
		locker = &defaultResourceLocker{}
	}

	for c.processNext(ctx, errorGroup, locker) {
	}

	if err := errorGroup.Wait(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("errGroup returned error")
	}
}

func (c *Controller) processNext(ctx context.Context, errorGroup *errgroup.Group, locker resourceLocker) bool {
	workItem, shutdown := c.WorkQueue.Get()
	if shutdown || c.WorkQueue.ShuttingDown() {
		return false
	}

	errorGroup.Go(func() error {
		// Let this Goroutine finish any work, the parent errorgroup will stop
		// scheduling more tasks when the parent context is cancelled.
		// Kubernetes will kill the process if it takes to long to finish any way
		workCtx := context.WithoutCancel(ctx)

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
			log.Ctx(ctx).Error().Err(err).Msg("Failed to get lock key and identifier")
			metrics.OperatorError(c.HandlerOf, "work_queue", "error_workqueue_type")
			return nil
		}

		if !locker.TryGetLock(lockKey) {
			log.Ctx(ctx).Debug().Msgf("Lock for %s was busy, requeuing %s", lockKey, identifier)
			// Use AddAfter instead of AddRateLimited. AddRateLimited can potentially cause a delay of 1000 seconds
			c.WorkQueue.AddAfter(identifier, 100*time.Millisecond)
			return nil
		}
		defer func() {
			locker.ReleaseLock(lockKey)
			log.Ctx(ctx).Debug().Msgf("Released lock for %s after processing %s", lockKey, identifier)
		}()

		log.Ctx(ctx).Debug().Msgf("Acquired lock for %s, processing %s", lockKey, identifier)
		c.processWorkItem(workCtx, workItem, identifier)
		return nil
	})

	return true
}

func (c *Controller) processWorkItem(ctx context.Context, workItem interface{}, workItemString string) {
	err := func(workItem interface{}) error {
		if err := c.syncHandler(ctx, workItemString); err != nil {
			c.WorkQueue.AddRateLimited(workItemString)
			metrics.OperatorError(c.HandlerOf, "work_queue", "requeuing")
			metrics.CustomResourceRemovedFromQueue(c.HandlerOf)
			metrics.CustomResourceUpdatedAndRequeued(c.HandlerOf)
			return err
		}

		c.WorkQueue.Forget(workItem)
		metrics.CustomResourceRemovedFromQueue(c.HandlerOf)
		log.Ctx(ctx).Info().Msgf("Successfully synced %s", workItemString)
		return nil
	}(workItem)

	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msgf("Failed to sync %s, requeuing", workItemString)
		metrics.OperatorError(c.HandlerOf, "process_next_work_item", "unhandled")
	}
}

func (c *Controller) syncHandler(ctx context.Context, key string) error {
	start := time.Now()

	defer func() {
		duration := time.Since(start)
		metrics.AddDurationOfReconciliation(c.HandlerOf, duration)
	}()

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	ctx = log.Ctx(ctx).With().Str("resource_namespace", namespace).Str("resource_name", name).Logger().WithContext(ctx)

	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msgf("invalid resource key: %s", key)
		metrics.OperatorError(c.HandlerOf, "split_meta_namespace_key", "invalid_resource_key")
		return nil
	}

	err = c.Handler.Sync(ctx, namespace, name, c.Recorder)
	if err != nil {
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
		metrics.OperatorError(c.HandlerOf, "enqueue", fmt.Sprintf("problems_sync_%s", key))
		return requeued, err
	}

	requeued = (c.WorkQueue.NumRequeues(key) > 0)
	c.WorkQueue.AddRateLimited(key)
	return requeued, nil
}

// HandleObject ensures that when anything happens to object which any
// custom resource is owner of, that custom resource is synced
func (c *Controller) HandleObject(ctx context.Context, obj interface{}, ownerKind string, getOwnerFn GetOwner) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			log.Ctx(ctx).Error().Msg("Failed to cast object as type cache.DeletedFinalStateUnknown")
			metrics.OperatorError(c.HandlerOf, "handle_object", "error_decoding_object")
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			log.Ctx(ctx).Error().Msg("Failed to cast tombstone.Obj as type metav1.Object")
			metrics.OperatorError(c.HandlerOf, "handle_object", "error_decoding_object_tombstone")
			return
		}
		log.Ctx(ctx).Info().Msgf("Recovered deleted object %s from tombstone", object.GetName())
	}
	log.Ctx(ctx).Info().Msgf("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		if ownerRef.Kind != ownerKind {
			return
		}

		obj, err := getOwnerFn(ctx, c.RadixClient, object.GetNamespace(), ownerRef.Name)
		if err != nil {
			log.Ctx(ctx).Debug().Msgf("Ignoring orphaned object %s of %s %s", object.GetSelfLink(), ownerKind, ownerRef.Name)
			return
		}

		requeued, err := c.Enqueue(obj)
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("Failed to enqueue object")
		}
		if err == nil && !requeued {
			metrics.CustomResourceUpdated(c.HandlerOf)
		}

		return
	}
}

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
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
)

// Controller Instance variables
type Controller struct {
	Name                 string
	HandlerOf            string
	KubeClient           kubernetes.Interface
	RadixClient          radixclient.Interface
	WorkQueue            workqueue.TypedRateLimitingInterface[cache.ObjectName]
	KubeInformerFactory  kubeinformers.SharedInformerFactory
	RadixInformerFactory informers.SharedInformerFactory
	Handler              Handler
	LockKey              LockKeyFunc
	locker               resourceLocker
}

// Run starts the shared informer, which will be stopped when stopCh is closed.
func (c *Controller) Run(ctx context.Context, threadiness int) error {
	defer utilruntime.HandleCrash()

	if c.LockKey == nil {
		return errors.New("LockKey must be set")
	}

	// Start the informer factories to begin populating the informer caches
	ctx = log.Ctx(ctx).With().Str("controller", c.Name).Logger().WithContext(ctx)
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Starting")

	// Wait for the caches to be synced before starting workers
	logger.Info().Msg("Waiting for Kube objects caches to sync")
	c.KubeInformerFactory.WaitForCacheSync(ctx.Done())
	logger.Info().Msg("Waiting for Radix objects caches to sync")
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

		lockKey := c.LockKey(workItem)
		if !locker.TryGetLock(lockKey) {
			log.Ctx(ctx).Debug().Msgf("Lock for %s was busy, requeuing %s", lockKey, workItem)
			// Use AddAfter instead of AddRateLimited. AddRateLimited can potentially cause a delay of 1000 seconds
			c.WorkQueue.AddAfter(workItem, 100*time.Millisecond)
			return nil
		}
		defer func() {
			locker.ReleaseLock(lockKey)
			log.Ctx(ctx).Debug().Msgf("Released lock for %s after processing %s", lockKey, workItem)
		}()

		log.Ctx(ctx).Debug().Msgf("Acquired lock for %s, processing %s", lockKey, workItem)
		c.processWorkItem(workCtx, workItem)
		return nil
	})

	return true
}

func (c *Controller) processWorkItem(ctx context.Context, workItem cache.ObjectName) {
	err := func(workItem cache.ObjectName) error {
		if err := c.syncHandler(ctx, workItem); err != nil {
			c.WorkQueue.AddRateLimited(workItem)
			metrics.OperatorError(c.HandlerOf, "work_queue", "requeuing")
			metrics.CustomResourceRemovedFromQueue(c.HandlerOf)
			metrics.CustomResourceUpdatedAndRequeued(c.HandlerOf)
			return err
		}

		c.WorkQueue.Forget(workItem)
		metrics.CustomResourceRemovedFromQueue(c.HandlerOf)
		log.Ctx(ctx).Info().Msgf("Successfully synced %s", workItem)
		return nil
	}(workItem)

	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msgf("Failed to sync %s, requeuing", workItem)
		metrics.OperatorError(c.HandlerOf, "process_next_work_item", "unhandled")
	}
}

func (c *Controller) syncHandler(ctx context.Context, key cache.ObjectName) error {
	start := time.Now()
	namespace, name := key.Namespace, key.Name
	ctx = log.Ctx(ctx).With().Str("resource_namespace", namespace).Str("resource_name", name).Logger().WithContext(ctx)

	defer func() {
		duration := time.Since(start)
		metrics.AddDurationOfReconciliation(c.HandlerOf, duration)
		log.Ctx(ctx).Debug().Dur("elapsed_ms", duration).Msg("Reconciliation duration")
	}()

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return c.Handler.Sync(ctx, namespace, name)
	})
	if err != nil {
		metrics.OperatorError(c.HandlerOf, "c_handler_sync", fmt.Sprintf("problems_sync_%s", key))
		return err
	}

	return nil
}

// Enqueue takes a resource and converts it into a namespace/name
// string which is then put onto the work queue
func (c *Controller) Enqueue(obj interface{}) error {
	objRef, err := cache.ObjectToName(obj)
	if err != nil {
		metrics.OperatorError(c.HandlerOf, "enqueue", "object_to_name")
		return err
	}

	c.WorkQueue.Add(objRef)
	return nil
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

		if err := c.Enqueue(obj); err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("Failed to enqueue object")
			return
		}

		metrics.CustomResourceUpdated(c.HandlerOf)
		return
	}
}

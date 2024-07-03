package batch

import (
	"context"
	"reflect"

	"github.com/equinor/radix-operator/pkg/apis/metrics"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

const (
	controllerAgentName = "batch-controller"
	crType              = "RadixBatches"
)

// NewController creates a new controller that handles RadixBatches
func NewController(ctx context.Context, client kubernetes.Interface,
	radixClient radixclient.Interface, handler common.Handler,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	radixInformerFactory informers.SharedInformerFactory,
	waitForChildrenToSync bool,
	recorder record.EventRecorder) *common.Controller {
	logger := log.With().Str("controller", controllerAgentName).Logger()
	batchInformer := radixInformerFactory.Radix().V1().RadixBatches()
	jobInformer := kubeInformerFactory.Batch().V1().Jobs()

	controller := &common.Controller{
		Name:                  controllerAgentName,
		HandlerOf:             crType,
		KubeClient:            client,
		RadixClient:           radixClient,
		Informer:              batchInformer.Informer(),
		KubeInformerFactory:   kubeInformerFactory,
		WorkQueue:             common.NewRateLimitedWorkQueue(ctx, crType),
		Handler:               handler,
		WaitForChildrenToSync: waitForChildrenToSync,
		Recorder:              recorder,
		LockKeyAndIdentifier:  common.NamespacePartitionKey,
	}

	logger.Info().Msg("Setting up event handlers")
	if _, err := batchInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			if _, err := controller.Enqueue(cur); err != nil {
				logger.Error().Err(err).Msg("Failed to enqueue object received from RadixBatch informer AddFunc")
			}
			metrics.CustomResourceAdded(crType)
		},
		UpdateFunc: func(old, cur interface{}) {
			oldRadixBatch := old.(*radixv1.RadixBatch)
			newRadixBatch := cur.(*radixv1.RadixBatch)
			if deepEqual(oldRadixBatch, newRadixBatch) {
				logger.Debug().Msgf("RadixBatch object is equal to old for %s. Do nothing", newRadixBatch.GetName())
				metrics.CustomResourceUpdatedButSkipped(crType)
				return
			}
			if _, err := controller.Enqueue(cur); err != nil {
				logger.Error().Err(err).Msg("Failed to enqueue object received from RadixBatch informer UpdateFunc")
			}
		},
		DeleteFunc: func(obj interface{}) {
			// TODO: We don't do any processing of the deleted object, so perhaps we should remove everything except for metrics call
			// Also check if other event handlers have the same noop code
			radixBatch, converted := obj.(*radixv1.RadixBatch)
			if !converted {
				log.Ctx(ctx).Error().Msg("RadixBatch object cast failed during deleted event.")
				return
			}
			key, err := cache.MetaNamespaceKeyFunc(radixBatch)
			if err == nil {
				logger.Debug().Msgf("RadixBatch object deleted event received for %s. Do nothing", key)
			}
			metrics.CustomResourceDeleted(crType)
		},
	}); err != nil {
		panic(err)
	}

	if _, err := jobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldMeta := oldObj.(metav1.Object)
			newMeta := newObj.(metav1.Object)
			if oldMeta.GetResourceVersion() == newMeta.GetResourceVersion() {
				return
			}
			controller.HandleObject(ctx, newObj, radixv1.KindRadixBatch, getOwner)
		},
		DeleteFunc: func(obj interface{}) {
			controller.HandleObject(ctx, obj, radixv1.KindRadixBatch, getOwner)
		},
	}); err != nil {
		panic(err)
	}
	return controller
}

func deepEqual(old, new *radixv1.RadixBatch) bool {
	return reflect.DeepEqual(new.Spec, old.Spec)
}

func getOwner(ctx context.Context, radixClient radixclient.Interface, namespace, name string) (interface{}, error) {
	return radixClient.RadixV1().RadixBatches(namespace).Get(ctx, name, metav1.GetOptions{})
}

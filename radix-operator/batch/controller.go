package batch

import (
	"context"
	"reflect"

	"github.com/equinor/radix-operator/pkg/apis/metrics"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/equinor/radix-operator/radix-operator/common"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

var logger *log.Entry

const (
	controllerAgentName = "batch-controller"
	crType              = "RadixBatches"
)

func init() {
	logger = log.WithFields(log.Fields{"radixOperatorComponent": controllerAgentName})
}

// NewController creates a new controller that handles RadixBatches
func NewController(client kubernetes.Interface,
	radixClient radixclient.Interface, handler common.Handler,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	radixInformerFactory informers.SharedInformerFactory,
	waitForChildrenToSync bool,
	recorder record.EventRecorder) *common.Controller {

	batchInformer := radixInformerFactory.Radix().V1().RadixBatches()
	jobInformer := kubeInformerFactory.Batch().V1().Jobs()
	// podInformer := kubeInformerFactory.Core().V1().Pods()

	controller := &common.Controller{
		Name:                  controllerAgentName,
		HandlerOf:             crType,
		KubeClient:            client,
		RadixClient:           radixClient,
		Informer:              batchInformer.Informer(),
		KubeInformerFactory:   kubeInformerFactory,
		WorkQueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), crType),
		Handler:               handler,
		Log:                   logger,
		WaitForChildrenToSync: waitForChildrenToSync,
		Recorder:              recorder,
		LockKeyAndIdentifier:  common.NamespacePartitionKey,
	}

	logger.Info("Setting up event handlers")
	batchInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			controller.Enqueue(cur)
			metrics.CustomResourceAdded(crType)
		},
		UpdateFunc: func(old, cur interface{}) {
			oldRadixBatch := old.(*radixv1.RadixBatch)
			newRadixBatch := cur.(*radixv1.RadixBatch)
			if deepEqual(oldRadixBatch, newRadixBatch) {
				logger.Debugf("RadixBatch object is equal to old for %s. Do nothing", newRadixBatch.GetName())
				metrics.CustomResourceUpdatedButSkipped(crType)
				return
			}

			controller.Enqueue(cur)
		},
		DeleteFunc: func(obj interface{}) {
			radixBatch, _ := obj.(*radixv1.RadixBatch)
			key, err := cache.MetaNamespaceKeyFunc(radixBatch)
			if err == nil {
				logger.Debugf("RadixBatch object deleted event received for %s. Do nothing", key)
			}
			metrics.CustomResourceDeleted(crType)
		},
	})

	jobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldMeta := oldObj.(metav1.Object)
			newMeta := newObj.(metav1.Object)
			if oldMeta.GetResourceVersion() == newMeta.GetResourceVersion() {
				return
			}
			controller.HandleObject(newObj, "RadixBatch", getOwner)
		},
		DeleteFunc: func(obj interface{}) {
			controller.HandleObject(obj, "RadixBatch", getOwner)
		},
	})

	return controller
}

func deepEqual(old, new *radixv1.RadixBatch) bool {
	return reflect.DeepEqual(new.Spec, old.Spec)
}

func getOwner(radixClient radixclient.Interface, namespace, name string) (interface{}, error) {
	return radixClient.RadixV1().RadixBatches(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}
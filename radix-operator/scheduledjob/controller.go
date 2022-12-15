package scheduledjob

import (
	"reflect"

	"github.com/equinor/radix-operator/pkg/apis/metrics"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/equinor/radix-operator/radix-operator/common"
	log "github.com/sirupsen/logrus"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

var logger *log.Entry

const (
	controllerAgentName = "scheduled-job-controller"
	crType              = "RadixScheduledJobs"
)

func init() {
	logger = log.WithFields(log.Fields{"radixOperatorComponent": controllerAgentName})
}

// NewController creates a new controller that handles RadixScheduledJobs
func NewController(client kubernetes.Interface,
	radixClient radixclient.Interface, handler common.Handler,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	radixInformerFactory informers.SharedInformerFactory,
	waitForChildrenToSync bool,
	recorder record.EventRecorder) *common.Controller {

	scheduledJobInformer := radixInformerFactory.Radix().V1().RadixScheduledJobs()

	controller := &common.Controller{
		Name:                  controllerAgentName,
		HandlerOf:             crType,
		KubeClient:            client,
		RadixClient:           radixClient,
		Informer:              scheduledJobInformer.Informer(),
		KubeInformerFactory:   kubeInformerFactory,
		WorkQueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), crType),
		Handler:               handler,
		Log:                   logger,
		WaitForChildrenToSync: waitForChildrenToSync,
		Recorder:              recorder,
		LockKeyAndIdentifier:  common.NamespacePartitionKey,
	}

	logger.Info("Setting up event handlers")
	scheduledJobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			controller.Enqueue(cur)
			metrics.CustomResourceAdded(crType)
		},
		UpdateFunc: func(old, cur interface{}) {
			oldRadixScheduledJob := old.(*radixv1.RadixScheduledJob)
			newRadixScheduledJob := cur.(*radixv1.RadixScheduledJob)
			if deepEqual(oldRadixScheduledJob, newRadixScheduledJob) {
				logger.Debugf("RadixScheduledJob object is equal to old for %s. Do nothing", newRadixScheduledJob.GetName())
				metrics.CustomResourceUpdatedButSkipped(crType)
				return
			}

			controller.Enqueue(cur)
		},
		DeleteFunc: func(obj interface{}) {
			radixScheduledJob, _ := obj.(*radixv1.RadixScheduledJob)
			key, err := cache.MetaNamespaceKeyFunc(radixScheduledJob)
			if err == nil {
				logger.Debugf("RadixScheduledJob object deleted event received for %s. Do nothing", key)
			}
			metrics.CustomResourceDeleted(crType)
		},
	})

	return controller
}

func deepEqual(old, new *radixv1.RadixScheduledJob) bool {
	return reflect.DeepEqual(new.Spec, old.Spec)
}

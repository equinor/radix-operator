package scheduledjob

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
	jobInformer := kubeInformerFactory.Batch().V1().Jobs()
	podInformer := kubeInformerFactory.Core().V1().Pods()

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

	jobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldMeta := oldObj.(metav1.Object)
			newMeta := newObj.(metav1.Object)
			if oldMeta.GetResourceVersion() == newMeta.GetResourceVersion() {
				return
			}
			controller.HandleObject(newObj, "RadixScheduledJob", getOwner)
		},
		DeleteFunc: func(obj interface{}) {
			controller.HandleObject(obj, "RadixScheduledJob", getOwner)
		},
	})

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldMeta := oldObj.(metav1.Object)
			newMeta := newObj.(metav1.Object)
			if oldMeta.GetResourceVersion() == newMeta.GetResourceVersion() {
				return
			}

			podOwnerRef := metav1.GetControllerOf(newMeta)
			if podOwnerRef == nil || podOwnerRef.Kind != "Job" {
				return
			}

			job, err := client.BatchV1().Jobs(newMeta.GetNamespace()).Get(context.TODO(), podOwnerRef.Name, metav1.GetOptions{})
			if err != nil {
				// This job may not be found because application is being deleted and resources are being deleted
				logger.Debugf("Could not find owning job of pod %s: %v", newMeta.GetName(), err)
				return
			}

			controller.HandleObject(job, "RadixScheduledJob", getOwner)
		},
	})

	return controller
}

func deepEqual(old, new *radixv1.RadixScheduledJob) bool {
	return reflect.DeepEqual(new.Spec, old.Spec)
}

func getOwner(radixClient radixclient.Interface, namespace, name string) (interface{}, error) {
	return radixClient.RadixV1().RadixScheduledJobs(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

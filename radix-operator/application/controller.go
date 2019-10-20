package application

import (
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/equinor/radix-operator/radix-operator/metrics"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

// Controller Instance variables
type Controller struct {
	clientset   kubernetes.Interface
	radixclient radixclient.Interface
	queue       workqueue.RateLimitingInterface
	informer    cache.SharedIndexInformer
	handler     common.Handler
}

var logger *log.Entry

const (
	controllerAgentName = "application-controller"
	crType              = "RadixApplications"
)

func init() {
	logger = log.WithFields(log.Fields{"radixOperatorComponent": controllerAgentName})
}

// NewController creates a new controller that handles RadixApplications
func NewController(client kubernetes.Interface,
	kubeutil *kube.Kube,
	radixClient radixclient.Interface, handler common.Handler,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	radixInformerFactory informers.SharedInformerFactory,
	recorder record.EventRecorder) *common.Controller {

	applicationInformer := radixInformerFactory.Radix().V1().RadixApplications()
	namespaceInformer := kubeInformerFactory.Core().V1().Namespaces()

	controller := &common.Controller{
		Name:                controllerAgentName,
		HandlerOf:           crType,
		KubeClient:          client,
		RadixClient:         radixClient,
		Informer:            applicationInformer.Informer(),
		KubeInformerFactory: kubeInformerFactory,
		WorkQueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), crType),
		Handler:             handler,
		Log:                 logger,
		Recorder:            recorder,
	}

	logger.Info("Setting up event handlers")
	applicationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			controller.Enqueue(cur)
			metrics.CustomResourceAdded(crType)
		},
		UpdateFunc: func(old, cur interface{}) {
			controller.Enqueue(cur)
		},
		DeleteFunc: func(obj interface{}) {
			radixApplication, _ := obj.(*v1.RadixApplication)
			key, err := cache.MetaNamespaceKeyFunc(radixApplication)
			if err == nil {
				logger.Debugf("Application object deleted event received for %s. Do nothing", key)
			}
			metrics.CustomResourceDeleted(crType)
		},
	})

	namespaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, cur interface{}) {
			newNs := cur.(*corev1.Namespace)
			oldNs := old.(*corev1.Namespace)
			if newNs.ResourceVersion == oldNs.ResourceVersion {
				return
			}

			if newNs.Annotations[kube.AdGroupsAnnotation] == oldNs.Annotations[kube.AdGroupsAnnotation] {
				return
			}

			// Trigger sync of RA, living in the namespace
			ra, err := radixClient.RadixV1().RadixApplications(newNs.Name).List(metav1.ListOptions{})
			if err == nil && len(ra.Items) == 1 {
				// Will sync the RA (there can only be one)
				var obj metav1.Object
				obj = &ra.Items[0]
				controller.Enqueue(obj)
			}
		},
	})

	return controller
}

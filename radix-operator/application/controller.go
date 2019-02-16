package application

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixinformer "github.com/equinor/radix-operator/pkg/client/informers/externalversions/radix/v1"
	"github.com/equinor/radix-operator/radix-operator/common"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

// ApplicationController Instance variables
type ApplicationController struct {
	clientset   kubernetes.Interface
	radixclient radixclient.Interface
	queue       workqueue.RateLimitingInterface
	informer    cache.SharedIndexInformer
	handler     common.Handler
}

var logger *log.Entry

const controllerAgentName = "application-controller"

func init() {
	logger = log.WithFields(log.Fields{"radixOperatorComponent": "application-controller"})
}

// NewApplicationController creates a new controller that handles RadixDeployments
func NewApplicationController(client kubernetes.Interface,
	radixClient radixclient.Interface, handler common.Handler,
	applicationInformer radixinformer.RadixApplicationInformer) *common.Controller {

	//recorder := common.NewEventRecorder(controllerAgentName, client.CoreV1().Events(""))

	controller := &common.Controller{
		Name:        controllerAgentName,
		KubeClient:  client,
		RadixClient: radixClient,
		Informer:    applicationInformer.Informer(),
		WorkQueue:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "RadixApplications"),
		Handler:     handler,
		Log:         logger,
		Recorder:    nil,
	}

	klog.Info("Setting up event handlers")
	applicationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.Enqueue,
		UpdateFunc: func(old, new interface{}) {
			controller.Enqueue(new)
		},
		DeleteFunc: func(obj interface{}) {
			radixApplication, _ := obj.(*v1.RadixApplication)
			key, err := cache.MetaNamespaceKeyFunc(radixApplication)
			if err == nil {
				logger.Infof("Application object deleted event received for %s. Do nothing", key)
			}
		},
	})

	return controller
}

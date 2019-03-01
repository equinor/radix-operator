package deployment

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixinformer "github.com/equinor/radix-operator/pkg/client/informers/externalversions/radix/v1"
	"github.com/equinor/radix-operator/radix-operator/common"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	extinformers "k8s.io/client-go/informers/extensions/v1beta1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

// DeployController Instance variables
type DeployController struct {
	clientset   kubernetes.Interface
	radixclient radixclient.Interface
	queue       workqueue.RateLimitingInterface
	informer    cache.SharedIndexInformer
	handler     common.Handler
}

var logger *log.Entry

const controllerAgentName = "deployment-controller"

func init() {
	logger = log.WithFields(log.Fields{"radixOperatorComponent": controllerAgentName})
}

// NewDeployController creates a new controller that handles RadixDeployments
func NewDeployController(client kubernetes.Interface,
	radixClient radixclient.Interface, handler common.Handler,
	deploymentInformer radixinformer.RadixDeploymentInformer,
	kubeDeployInformer extinformers.DeploymentInformer,
	serviceInformer coreinformers.ServiceInformer,
	ingressInformer extinformers.IngressInformer,
	recorder record.EventRecorder) *common.Controller {

	controller := &common.Controller{
		Name:        controllerAgentName,
		KubeClient:  client,
		RadixClient: radixClient,
		Informer:    deploymentInformer.Informer(),
		WorkQueue:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "RadixDeployments"),
		Handler:     handler,
		Log:         logger,
		Recorder:    recorder,
	}

	logger.Info("Setting up event handlers")
	deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.Enqueue,
		UpdateFunc: func(old, new interface{}) {
			controller.Enqueue(new)
		},
		DeleteFunc: func(obj interface{}) {
			radixDeployment, _ := obj.(*v1.RadixDeployment)
			key, err := cache.MetaNamespaceKeyFunc(radixDeployment)
			if err == nil {
				logger.Debugf("Deployment object deleted event received for %s. Do nothing", key)
			}
		},
	})

	/*
		kubeDeployInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				deploy := obj.(*v1beta1.Deployment)
				logger.Debugf("Deployment object added event received for %s. Do nothing", deploy.Name)
			},
			UpdateFunc: func(old, new interface{}) {
				newDeploy := new.(*v1beta1.Deployment)
				oldDeploy := old.(*v1beta1.Deployment)
				if newDeploy.ResourceVersion == oldDeploy.ResourceVersion {
					return
				}
				controller.HandleObject(new, "RadixDeployment", getObject)
			},
			DeleteFunc: func(obj interface{}) {
				controller.HandleObject(obj, "RadixDeployment", getObject)
			},
		})
	*/
	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			service := obj.(*corev1.Service)
			logger.Debugf("Service object added event received for %s. Do nothing", service.Name)
		},
		UpdateFunc: func(old, new interface{}) {
			newService := new.(*corev1.Service)
			oldService := old.(*corev1.Service)
			if newService.ResourceVersion == oldService.ResourceVersion {
				return
			}
			controller.HandleObject(new, "RadixDeployment", getObject)
		},
		DeleteFunc: func(obj interface{}) {
			controller.HandleObject(obj, "RadixDeployment", getObject)
		},
	})

	/*
		ingressInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				ingress := obj.(*v1beta1.Ingress)
				logger.Debugf("Ingress object added event received for %s. Do nothing", ingress.Name)
			},
			UpdateFunc: func(old, new interface{}) {
				newIngress := new.(*v1beta1.Ingress)
				oldIngress := old.(*v1beta1.Ingress)
				if newIngress.ResourceVersion == oldIngress.ResourceVersion {
					return
				}
				controller.HandleObject(new, "RadixDeployment", getObject)
			},
			DeleteFunc: func(obj interface{}) {
				controller.HandleObject(obj, "RadixDeployment", getObject)
			},
		})
	*/
	return controller
}

func getObject(radixClient radixclient.Interface, namespace, name string) (interface{}, error) {
	return radixClient.RadixV1().RadixDeployments(namespace).Get(name, metav1.GetOptions{})
}

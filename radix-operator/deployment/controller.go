package deployment

import (
	"reflect"

	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixinformer "github.com/equinor/radix-operator/pkg/client/informers/externalversions/radix/v1"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/equinor/radix-operator/radix-operator/metrics"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
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
	controllerAgentName = "deployment-controller"
	crType              = "RadixDeployments"
)

func init() {
	logger = log.WithFields(log.Fields{"radixOperatorComponent": controllerAgentName})
}

// NewController creates a new controller that handles RadixDeployments
func NewController(client kubernetes.Interface,
	radixClient radixclient.Interface, handler common.Handler,
	deploymentInformer radixinformer.RadixDeploymentInformer,
	serviceInformer coreinformers.ServiceInformer,
	namespaceInformer coreinformers.NamespaceInformer,
	recorder record.EventRecorder) *common.Controller {

	controller := &common.Controller{
		Name:        controllerAgentName,
		KubeClient:  client,
		RadixClient: radixClient,
		Informer:    deploymentInformer.Informer(),
		WorkQueue:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), crType),
		Handler:     handler,
		Log:         logger,
		Recorder:    recorder,
	}

	logger.Info("Setting up event handlers")
	deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			radixDeployment, _ := cur.(*v1.RadixDeployment)
			if deployment.IsRadixDeploymentInactive(radixDeployment) {
				logger.Debugf("Skip deployment object %s as it is inactive", radixDeployment.GetName())
				return
			}

			controller.Enqueue(cur)
			metrics.CustomResourceAdded(crType)
		},
		UpdateFunc: func(old, cur interface{}) {
			newRD := cur.(*v1.RadixDeployment)
			oldRD := old.(*v1.RadixDeployment)
			if deployment.IsRadixDeploymentInactive(newRD) {
				logger.Debugf("Skip deployment object %s as it is inactive", newRD.GetName())
				return
			}

			if deepEqual(oldRD, newRD) {
				logger.Debugf("Deployment object is equal to old for %s. Do nothing", newRD.GetName())
				return
			}

			controller.Enqueue(cur)
		},
		DeleteFunc: func(obj interface{}) {
			radixDeployment, _ := obj.(*v1.RadixDeployment)
			key, err := cache.MetaNamespaceKeyFunc(radixDeployment)
			if err == nil {
				logger.Debugf("Deployment object deleted event received for %s. Do nothing", key)
			}
			metrics.CustomResourceDeleted(crType)
		},
	})

	// Only the service informer works with this, because it makes use of patch
	// if not it will end up in an endless loop (deployment, ingress etc.)
	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			service := obj.(*corev1.Service)
			logger.Debugf("Service object added event received for %s. Do nothing", service.Name)
		},
		UpdateFunc: func(old, cur interface{}) {
			newService := cur.(*corev1.Service)
			oldService := old.(*corev1.Service)
			if newService.ResourceVersion == oldService.ResourceVersion {
				return
			}
			controller.HandleObject(cur, "RadixDeployment", getObject)
		},
		DeleteFunc: func(obj interface{}) {
			controller.HandleObject(obj, "RadixDeployment", getObject)
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

			// Trigger sync of active RD, living in the namespace
			rds, err := radixClient.RadixV1().RadixDeployments(newNs.Name).List(metav1.ListOptions{})

			if err == nil && len(rds.Items) > 0 {
				// Will sync the active RD (there can only be one)
				for _, rd := range rds.Items {
					if !deployment.IsRadixDeploymentInactive(&rd) {
						var obj metav1.Object
						obj = &rd
						controller.Enqueue(obj)
					}
				}
			}
		},
	})

	return controller
}

func deepEqual(old, new *v1.RadixDeployment) bool {
	if !reflect.DeepEqual(new.Spec, old.Spec) ||
		!reflect.DeepEqual(new.ObjectMeta.Labels, old.ObjectMeta.Labels) ||
		!reflect.DeepEqual(new.ObjectMeta.Annotations, old.ObjectMeta.Annotations) {
		return false
	}

	return true
}

func getObject(radixClient radixclient.Interface, namespace, name string) (interface{}, error) {
	return radixClient.RadixV1().RadixDeployments(namespace).Get(name, metav1.GetOptions{})
}

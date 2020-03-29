package environment

import (
	"reflect"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/equinor/radix-operator/radix-operator/metrics"
	"github.com/sirupsen/logrus"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

const (
	controllerAgentName = "environment-controller"
	crType              = "RadixEnvironments"
)

var logger *logrus.Entry

func init() {
	logger = logrus.WithFields(logrus.Fields{"radixOperatorComponent": "environment-controller"})
}

// NewController creates a new controller that handles RadixEnvironments
func NewController(client kubernetes.Interface,
	kubeutil *kube.Kube,
	radixClient radixclient.Interface,
	handler common.Handler,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	radixInformerFactory informers.SharedInformerFactory,
	waitForChildrenToSync bool,
	recorder record.EventRecorder) *common.Controller {

	environmentInformer := radixInformerFactory.Radix().V1().RadixEnvironments()
	registrationInformer := radixInformerFactory.Radix().V1().RadixRegistrations()

	controller := &common.Controller{
		Name:                  controllerAgentName,
		HandlerOf:             crType,
		KubeClient:            client,
		RadixClient:           radixClient,
		Informer:              environmentInformer.Informer(),
		KubeInformerFactory:   kubeInformerFactory,
		WorkQueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), crType),
		Handler:               handler,
		Log:                   logger,
		WaitForChildrenToSync: waitForChildrenToSync,
		Recorder:              recorder,
	}

	logger.Info("Setting up event handlers")

	environmentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			controller.Enqueue(cur)
			metrics.CustomResourceAdded(crType)
		},
		UpdateFunc: func(old, cur interface{}) {
			newRR := cur.(*v1.RadixEnvironment)
			oldRR := old.(*v1.RadixEnvironment)

			if deepEqual(oldRR, newRR) {
				logger.Debugf("Environment object is equal to old for %s. Do nothing", newRR.GetName())
				metrics.CustomResourceUpdatedButSkipped(crType)
				return
			}

			controller.Enqueue(cur)
			metrics.CustomResourceUpdated(crType)
		},
		DeleteFunc: func(obj interface{}) {
			radixEnvironment, _ := obj.(*v1.RadixEnvironment)
			key, err := cache.MetaNamespaceKeyFunc(radixEnvironment)
			if err == nil {
				logger.Debugf("Environment object deleted event received for %s. Do nothing", key)
			}
			metrics.CustomResourceDeleted(crType)
		},
	})

	namespaceInformer := kubeInformerFactory.Core().V1().Namespaces()
	namespaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) {
			// attempt to sync environment if it is the owner of this namespace
			controller.HandleObject(obj, "RadixEnvironment", getOwner)
		},
	})

	rolebindingInformer := kubeInformerFactory.Rbac().V1().RoleBindings()
	rolebindingInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) {
			// attempt to sync environment if it is the owner of this role-binding
			controller.HandleObject(obj, "RadixEnvironment", getOwner)
		},
	})

	limitrangeInformer := kubeInformerFactory.Core().V1().LimitRanges()
	limitrangeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) {
			// attempt to sync environment if it is the owner of this limit-range
			controller.HandleObject(obj, "RadixEnvironment", getOwner)
		},
	})

	registrationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, cur interface{}) {
			newRr := cur.(*v1.RadixRegistration)
			oldRr := old.(*v1.RadixRegistration)
			if newRr.ResourceVersion == oldRr.ResourceVersion {
				return
			}

			// If neither ad group did change, nor the machine user, this
			// does not affect the deployment
			if utils.ArrayEqualElements(newRr.Spec.AdGroups, oldRr.Spec.AdGroups) &&
				newRr.Spec.MachineUser == oldRr.Spec.MachineUser {
				return
			}

			// Trigger sync of RA, living in the namespace
			ra, err := radixClient.RadixV1().RadixApplications(utils.GetAppNamespace(newRr.Name)).List(metav1.ListOptions{})
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

func deepEqual(old, new *v1.RadixEnvironment) bool {
	if !reflect.DeepEqual(new.Spec, old.Spec) ||
		!reflect.DeepEqual(new.ObjectMeta.Labels, old.ObjectMeta.Labels) ||
		!reflect.DeepEqual(new.ObjectMeta.Annotations, old.ObjectMeta.Annotations) {
		return false
	}

	return true
}

func getOwner(radixClient radixclient.Interface, namespace, name string) (interface{}, error) {
	return radixClient.RadixV1().RadixEnvironments().Get(name, meta.GetOptions{})
}

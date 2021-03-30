package registration

import (
	"context"
	"reflect"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/metrics"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/equinor/radix-operator/radix-operator/common"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

var logger *log.Entry

const (
	controllerAgentName = "registration-controller"
	crType              = "RadixRegistrations"
)

func init() {
	logger = log.WithFields(log.Fields{"radixOperatorComponent": "registration-controller"})
}

//NewController creates a new controller that handles RadixRegistrations
func NewController(client kubernetes.Interface,
	kubeutil *kube.Kube,
	radixClient radixclient.Interface, handler common.Handler,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	radixInformerFactory informers.SharedInformerFactory,
	waitForChildrenToSync bool,
	recorder record.EventRecorder) *common.Controller {

	registrationInformer := radixInformerFactory.Radix().V1().RadixRegistrations()
	controller := &common.Controller{
		Name:                  controllerAgentName,
		HandlerOf:             crType,
		KubeClient:            client,
		RadixClient:           radixClient,
		Informer:              registrationInformer.Informer(),
		KubeInformerFactory:   kubeInformerFactory,
		WorkQueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), crType),
		Handler:               handler,
		Log:                   logger,
		WaitForChildrenToSync: waitForChildrenToSync,
		Recorder:              recorder,
	}

	logger.Info("Setting up event handlers")

	registrationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			controller.Enqueue(cur)
			metrics.CustomResourceAdded(crType)
		},
		UpdateFunc: func(old, cur interface{}) {
			newRR := cur.(*v1.RadixRegistration)
			oldRR := old.(*v1.RadixRegistration)

			if deepEqual(oldRR, newRR) {
				logger.Debugf("Registration object is equal to old for %s. Do nothing", newRR.GetName())
				metrics.CustomResourceUpdatedButSkipped(crType)
				return
			}

			controller.Enqueue(cur)
			metrics.CustomResourceUpdated(crType)
		},
		DeleteFunc: func(obj interface{}) {
			radixRegistration, _ := obj.(*v1.RadixRegistration)
			key, err := cache.MetaNamespaceKeyFunc(radixRegistration)
			if err == nil {
				logger.Debugf("Registration object deleted event received for %s. Do nothing", key)
			}
			metrics.CustomResourceDeleted(crType)
		},
	})

	namespaceInformer := kubeInformerFactory.Core().V1().Namespaces()
	namespaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) {
			controller.HandleObject(obj, "RadixRegistration", getObject)
		},
	})

	secretInformer := kubeInformerFactory.Core().V1().Secrets()
	secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) {
			secret, _ := obj.(*corev1.Secret)
			namespace, _ := client.CoreV1().Namespaces().Get(context.TODO(), secret.Namespace, metav1.GetOptions{})
			appName := namespace.Labels[kube.RadixAppLabel]

			if isMachineUserToken(appName, secret) {
				// Resync, as token is deleted. Resync is triggered on namespace, since RR not directly own the
				// secret
				controller.HandleObject(namespace, "RadixRegistration", getObject)
			}
		},
	})

	return controller
}

func isMachineUserToken(appName string, secret *corev1.Secret) bool {
	machineUserServiceAccount := defaults.GetMachineUserRoleName(appName)
	return secret.Annotations[corev1.ServiceAccountNameKey] == machineUserServiceAccount
}

func deepEqual(old, new *v1.RadixRegistration) bool {
	if !reflect.DeepEqual(new.Spec, old.Spec) ||
		!reflect.DeepEqual(new.ObjectMeta.Labels, old.ObjectMeta.Labels) ||
		!reflect.DeepEqual(new.ObjectMeta.Annotations, old.ObjectMeta.Annotations) {
		return false
	}

	return true
}

func getObject(radixClient radixclient.Interface, namespace, name string) (interface{}, error) {
	return radixClient.RadixV1().RadixRegistrations().Get(name, metav1.GetOptions{})
}

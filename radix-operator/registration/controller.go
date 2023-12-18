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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
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

// NewController creates a new controller that handles RadixRegistrations
func NewController(client kubernetes.Interface,
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
		LockKeyAndIdentifier:  common.NamePartitionKey,
	}

	logger.Info("Setting up event handlers")

	if _, err := registrationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			if _, err := controller.Enqueue(cur); err != nil {
				utilruntime.HandleError(err)
			}
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

			if _, err := controller.Enqueue(cur); err != nil {
				utilruntime.HandleError(err)
			}
			metrics.CustomResourceUpdated(crType)
		},
		DeleteFunc: func(obj interface{}) {
			radixRegistration, converted := obj.(*v1.RadixRegistration)
			if !converted || radixRegistration == nil {
				logger.Errorf("v1.RadixRegistration object cast failed during deleted event received.")
				return
			}
			key, err := cache.MetaNamespaceKeyFunc(radixRegistration)
			if err == nil {
				logger.Debugf("Registration object deleted event received for %s. Do nothing", key)
			}
			metrics.CustomResourceDeleted(crType)
		},
	}); err != nil {
		panic(err)
	}

	namespaceInformer := kubeInformerFactory.Core().V1().Namespaces()
	if _, err := namespaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) {
			controller.HandleObject(obj, v1.KindRadixRegistration, getObject)
		},
	}); err != nil {
		panic(err)
	}

	secretInformer := kubeInformerFactory.Core().V1().Secrets()
	if _, err := secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldSecret := oldObj.(*corev1.Secret)
			newSecret := newObj.(*corev1.Secret)
			namespace, err := client.CoreV1().Namespaces().Get(context.TODO(), oldSecret.Namespace, metav1.GetOptions{})
			if err != nil {
				logger.Error(err)
				return
			}
			if oldSecret.ResourceVersion == newSecret.ResourceVersion {
				// Periodic resync will send update events for all known Secrets.
				// Two different versions of the same Secret will always have different RVs.
				return
			}
			if isGitDeployKey(newSecret) && newSecret.Namespace != "" {
				// Resync, as deploy key is updated. Resync is triggered on namespace, since RR not directly own the
				// secret
				controller.HandleObject(namespace, v1.KindRadixRegistration, getObject)
			}
		},
		DeleteFunc: func(obj interface{}) {
			secret, converted := obj.(*corev1.Secret)
			if !converted {
				logger.Errorf("corev1.Secret object cast failed during deleted event received.")
				return
			}
			namespace, err := client.CoreV1().Namespaces().Get(context.TODO(), secret.Namespace, metav1.GetOptions{})
			if err != nil {
				// Ignore error if namespace does not exist.
				// This is normal when a RR is deleted, resulting in deletion of namespaces and it's secrets
				if errors.IsNotFound(err) {
					return
				}
				logger.Error(err)
				return
			}
			if isGitDeployKey(secret) && namespace.Labels[kube.RadixAppLabel] != "" {
				// Resync, as deploy key is deleted. Resync is triggered on namespace, since RR not directly own the
				// secret
				controller.HandleObject(namespace, v1.KindRadixRegistration, getObject)
			}
		},
	}); err != nil {
		panic(err)
	}

	return controller
}

func isGitDeployKey(secret *corev1.Secret) bool {
	return secret.Name == defaults.GitPrivateKeySecretName
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
	return radixClient.RadixV1().RadixRegistrations().Get(context.TODO(), name, metav1.GetOptions{})
}

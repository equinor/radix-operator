package dnsalias

import (
	"context"
	"reflect"

	"github.com/equinor/radix-operator/pkg/apis/metrics"
	"github.com/equinor/radix-operator/pkg/apis/radix"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/sirupsen/logrus"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

const (
	controllerAgentName = "dns-alias-controller"
)

var logger *logrus.Entry

func init() {
	logger = logrus.WithFields(logrus.Fields{"radixOperatorComponent": "dns-alias-controller"})
}

// NewController creates a new controller that handles RadixDNSAlias
func NewController(kubeClient kubernetes.Interface,
	radixClient radixclient.Interface,
	handler common.Handler,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	radixInformerFactory informers.SharedInformerFactory,
	waitForChildrenToSync bool,
	recorder record.EventRecorder) *common.Controller {

	dnsAliasInformer := radixInformerFactory.Radix().V1().RadixDNSAliases()
	ingressInformer := kubeInformerFactory.Networking().V1().Ingresses()

	controller := &common.Controller{
		Name:                  controllerAgentName,
		HandlerOf:             radix.KindRadixDNSAlias,
		KubeClient:            kubeClient,
		RadixClient:           radixClient,
		Informer:              dnsAliasInformer.Informer(),
		KubeInformerFactory:   kubeInformerFactory,
		WorkQueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), radix.KindRadixDNSAlias),
		Handler:               handler,
		Log:                   logger,
		WaitForChildrenToSync: waitForChildrenToSync,
		Recorder:              recorder,
		LockKeyAndIdentifier:  common.NamePartitionKey,
	}

	logger.Info("Setting up event handlers")

	dnsAliasInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			alias := cur.(*v1.RadixDNSAlias)
			logger.Debugf("added RadixDNSAlias %s. Do nothing", alias.GetName())
			controller.Enqueue(cur)
			metrics.CustomResourceAdded(radix.KindRadixDNSAlias)
		},
		UpdateFunc: func(old, cur interface{}) {
			newAlias := cur.(*v1.RadixDNSAlias)
			oldAlias := old.(*v1.RadixDNSAlias)
			if deepEqual(oldAlias, newAlias) {
				logger.Debugf("RadixDNSAlias object is equal to old for %s. Do nothing", newAlias.GetName())
				metrics.CustomResourceUpdatedButSkipped(radix.KindRadixDNSAlias)
				return
			}
			controller.Enqueue(cur)
			metrics.CustomResourceUpdated(radix.KindRadixDNSAlias)
		},
		DeleteFunc: func(obj interface{}) {
			alias, converted := obj.(*v1.RadixDNSAlias)
			if !converted {
				logger.Errorf("RadixDNSAlias object cast failed during deleted event received.")
				return
			}
			key, err := cache.MetaNamespaceKeyFunc(alias)
			if err != nil {
				logger.Errorf("error on RadixDNSAlias object deleted event received for %s: %v", key, err)
			}
			metrics.CustomResourceDeleted(radix.KindRadixDNSAlias)
		},
	})

	ingressInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldIng := oldObj.(metav1.Object)
			newIng := newObj.(metav1.Object)
			if oldIng.GetResourceVersion() == newIng.GetResourceVersion() {
				return
			}
			controller.HandleObject(newObj, radix.KindRadixDNSAlias, getOwner) // restore ingress if it does not correspond to RadixDNSAlias
		},
		DeleteFunc: func(obj interface{}) {
			ing, converted := obj.(*networkingv1.Ingress)
			if !converted {
				logger.Errorf("Ingress object cast failed during deleted event received.")
				return
			}
			controller.HandleObject(ing, radix.KindRadixDNSAlias, getOwner) // restore ingress if RadixDNSAlias exist
		},
	})
	return controller
}

func deepEqual(old, new *v1.RadixDNSAlias) bool {
	return reflect.DeepEqual(new.Spec, old.Spec) &&
		reflect.DeepEqual(new.ObjectMeta.Labels, old.ObjectMeta.Labels) &&
		reflect.DeepEqual(new.ObjectMeta.Annotations, old.ObjectMeta.Annotations)
}

func getOwner(radixClient radixclient.Interface, _, name string) (interface{}, error) {
	return radixClient.RadixV1().RadixDNSAliases().Get(context.Background(), name, metav1.GetOptions{})
}

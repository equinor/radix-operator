package dnsalias

import (
	"context"
	"reflect"

	"github.com/equinor/radix-operator/pkg/apis/metrics"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

const (
	controllerAgentName = "dns-alias-controller"
	crType              = "RadixDNSAlias"
)

var logger *logrus.Entry

func init() {
	logger = logrus.WithFields(logrus.Fields{"radixOperatorComponent": "dns-alias-controller"})
}

// NewController creates a new controller that handles RadixDNSAlias
func NewController(client kubernetes.Interface,
	radixClient radixclient.Interface,
	handler common.Handler,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	radixInformerFactory informers.SharedInformerFactory,
	waitForChildrenToSync bool,
	recorder record.EventRecorder) *common.Controller {

	dnsAliasInformer := radixInformerFactory.Radix().V1().RadixDNSAliases()
	applicationInformer := radixInformerFactory.Radix().V1().RadixApplications()
	environmentInformer := radixInformerFactory.Radix().V1().RadixEnvironments()
	namespaceInformer := kubeInformerFactory.Core().V1().Namespaces()
	ingressInformer := kubeInformerFactory.Networking().V1().Ingresses()

	controller := &common.Controller{
		Name:                  controllerAgentName,
		HandlerOf:             crType,
		KubeClient:            client,
		RadixClient:           radixClient,
		Informer:              dnsAliasInformer.Informer(),
		KubeInformerFactory:   kubeInformerFactory,
		WorkQueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), crType),
		Handler:               handler,
		Log:                   logger,
		WaitForChildrenToSync: waitForChildrenToSync,
		Recorder:              recorder,
		LockKeyAndIdentifier:  common.NamePartitionKey,
	}

	logger.Info("Setting up event handlers")

	dnsAliasInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			controller.Enqueue(cur)
			metrics.CustomResourceAdded(crType)
		},
		UpdateFunc: func(old, cur interface{}) {
			newRDA := cur.(*v1.RadixDNSAlias)
			oldRDA := old.(*v1.RadixDNSAlias)

			if deepEqual(oldRDA, newRDA) {
				logger.Debugf("RadixDNSAlias object is equal to old for %s. Do nothing", newRDA.GetName())
				metrics.CustomResourceUpdatedButSkipped(crType)
				return
			}

			controller.Enqueue(cur)
			metrics.CustomResourceUpdated(crType)
		},
		DeleteFunc: func(obj interface{}) {
			// TODO delete ingresses
			// radixDNSAlias, converted := obj.(*v1.RadixDNSAlias)
			_, converted := obj.(*v1.RadixDNSAlias)
			if !converted {
				logger.Errorf("RadixDNSAlias object cast failed during deleted event received.")
				return
			}
			// key, err := cache.MetaNamespaceKeyFunc(radixDNSAlias)
			// if err != nil {
			// 	logger.Errorf("RadixDNSAlias object deleted event received for %s. Do nothing", key)
			// }
			metrics.CustomResourceDeleted(crType)
		},
	})

	environmentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) {
			// TODO alert in events if missing ns
		},
	})

	namespaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) {
			// TODO alert in events if missing ns
		},
	})

	ingressInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) {
			// TODO restore ingress if missing
		},
	})

	applicationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, cur interface{}) {
			newRa := cur.(*v1.RadixApplication)
			oldRa := old.(*v1.RadixApplication)
			if newRa.ResourceVersion == oldRa.ResourceVersion {
				return
			}
			// TODO update SNS aliases
			// dnsAliasesToResync := getAddedOrDroppedDNSAliasDomains(oldRa, newRa)
			// for _, domain := range dnsAliasesToResync {
			// 	uniqueName := utils.GetEnvironmentNamespace(oldRa.Name, envName)
			// 	re, err := radixClient.RadixV1().RadixDNSAliases().Get(context.TODO(), uniqueName, metav1.GetOptions{})
			// 	if err == nil {
			// 		controller.Enqueue(re)
			// 	}
			// }
		},
		DeleteFunc: func(cur interface{}) {
			// TODO delete DNS aliases
			// radixApplication, converted := cur.(*v1.RadixApplication)
			_, converted := cur.(*v1.RadixApplication)
			if !converted {
				logger.Errorf("RadixApplication object cast failed during deleted event received.")
				return
			}
			// for _, env := range radixApplication.Spec.Environments {
			// 	uniqueName := utils.GetEnvironmentNamespace(radixApplication.Name, env.Name)
			// 	re, err := radixClient.RadixV1().RadixDNSAliases().Get(context.TODO(), uniqueName, metav1.GetOptions{})
			// 	if err == nil {
			// 		controller.Enqueue(re)
			// 	}
			// }
		},
	})

	return controller
}

func deepEqual(old, new *v1.RadixDNSAlias) bool {
	if !reflect.DeepEqual(new.Spec, old.Spec) ||
		!reflect.DeepEqual(new.ObjectMeta.Labels, old.ObjectMeta.Labels) ||
		!reflect.DeepEqual(new.ObjectMeta.Annotations, old.ObjectMeta.Annotations) {
		return false
	}

	return true
}

func getOwner(radixClient radixclient.Interface, namespace, name string) (interface{}, error) {
	return radixClient.RadixV1().RadixDNSAliases().Get(context.TODO(), name, metav1.GetOptions{})
}

func getAddedOrDroppedDNSAliasDomains(oldRa *v1.RadixApplication, newRa *v1.RadixApplication) []string {
	var dnsAliasDomains []string
	dnsAliasDomains = append(dnsAliasDomains, getMissingDNSAliasDomains(oldRa.Spec.DNSAlias, newRa.Spec.DNSAlias)...)
	dnsAliasDomains = append(dnsAliasDomains, getMissingDNSAliasDomains(newRa.Spec.DNSAlias, oldRa.Spec.DNSAlias)...)
	return dnsAliasDomains
}

// getMissingDNSAliasDomains returns dnsAlias domains that exists in source list but not in target list
func getMissingDNSAliasDomains(source []v1.DNSAlias, target []v1.DNSAlias) []string {
	droppedDomains := make([]string, 0)
	for _, oldDNSAlias := range source {
		dropped := true
		for _, newDnsAlias := range target {
			if oldDNSAlias.Domain == newDnsAlias.Domain {
				dropped = false
			}
		}
		if dropped {
			droppedDomains = append(droppedDomains, oldDNSAlias.Domain)
		}
	}
	return droppedDomains
}

package dnsalias

import (
	"context"
	"reflect"

	"github.com/equinor/radix-operator/pkg/apis/metrics"
	"github.com/equinor/radix-operator/pkg/apis/radix"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
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

	radixDNSAliasInformer := radixInformerFactory.Radix().V1().RadixDNSAliases()
	radixDeploymentInformer := radixInformerFactory.Radix().V1().RadixDeployments()
	ingressInformer := kubeInformerFactory.Networking().V1().Ingresses()

	controller := &common.Controller{
		Name:                controllerAgentName,
		HandlerOf:           radix.KindRadixDNSAlias,
		KubeClient:          kubeClient,
		RadixClient:         radixClient,
		Informer:            radixDNSAliasInformer.Informer(),
		KubeInformerFactory: kubeInformerFactory,
		WorkQueue: workqueue.NewRateLimitingQueueWithConfig(workqueue.DefaultControllerRateLimiter(), workqueue.RateLimitingQueueConfig{
			Name: radix.KindRadixDNSAlias,
		}),
		Handler:               handler,
		Log:                   logger,
		WaitForChildrenToSync: waitForChildrenToSync,
		Recorder:              recorder,
		LockKeyAndIdentifier:  common.NamePartitionKey,
	}

	logger.Info("Setting up event handlers")

	_, err := radixDNSAliasInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			alias := cur.(*radixv1.RadixDNSAlias)
			logger.Debugf("added RadixDNSAlias %s", alias.GetName())
			_, err := controller.Enqueue(cur)
			if err != nil {
				logger.Errorf("failed to enqueue the RadixDNSAlias %s", alias.GetName())
			}
			metrics.CustomResourceAdded(radix.KindRadixDNSAlias)
		},
		UpdateFunc: func(old, cur interface{}) {
			oldAlias := old.(*radixv1.RadixDNSAlias)
			newAlias := cur.(*radixv1.RadixDNSAlias)
			if deepEqual(oldAlias, newAlias) {
				logger.Debugf("RadixDNSAlias object is equal to old for %s. Do nothing", newAlias.GetName())
				metrics.CustomResourceUpdatedButSkipped(radix.KindRadixDNSAlias)
				return
			}
			logger.Debugf("updated RadixDNSAlias %s", newAlias.GetName())
			_, err := controller.Enqueue(cur)
			if err != nil {
				logger.Errorf("failed to enqueue the RadixDNSAlias %s", newAlias.GetName())
			}
			metrics.CustomResourceUpdated(radix.KindRadixDNSAlias)
		},
		DeleteFunc: func(obj interface{}) {
			alias, converted := obj.(*radixv1.RadixDNSAlias)
			if !converted {
				logger.Errorf("RadixDNSAlias object cast failed during deleted event received.")
				return
			}
			logger.Debugf("deleted RadixDNSAlias %s", alias.GetName())
			key, err := cache.MetaNamespaceKeyFunc(alias)
			if err != nil {
				logger.Errorf("error on RadixDNSAlias object deleted event received for %s: %v", key, err)
			}
			metrics.CustomResourceDeleted(radix.KindRadixDNSAlias)
		},
	})
	if err != nil {
		logger.Errorf("failed to add an event hanflers to the radixDNSAliasInformer")
	}

	_, err = radixDeploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			rd := cur.(*radixv1.RadixDeployment)
			logger.Debugf("Added RadixDeployment %s to application %s in the environment %s. Re-sync relevant RadixDNSAliases", rd.GetName(), rd.Spec.AppName, rd.Spec.Environment)
			radixDNSAliases, err := getRadixDNSAliasForAppAndEnvironment(radixClient, rd.Spec.AppName, rd.Spec.Environment)
			if err != nil {
				logger.Errorf("failed to get list of RadixDNSAliases for the application %s", rd.Spec.AppName)
				return
			}
			for _, radixDNSAlias := range radixDNSAliases {
				logger.Debugf("re-sync RadixDNSAlias %s", radixDNSAlias.GetName())
				if _, err := controller.Enqueue(&radixDNSAlias); err != nil {
					logger.Errorf("failed to re-sync RadixDNSAlias %s. Error: %v", radixDNSAlias.GetName(), err)
				}
			}
		},
	})
	if err != nil {
		logger.Errorf("failed to add an event hanflers to the radixDeploymentInformer")
	}

	_, err = ingressInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldIng := oldObj.(metav1.Object)
			newIng := newObj.(metav1.Object)
			if oldIng.GetResourceVersion() == newIng.GetResourceVersion() {
				logger.Debugf("updating Ingress %s has the same resource version. Do nothing.", newIng.GetName())
				return
			}
			logger.Debugf("updated Ingress %s", newIng.GetName())
			controller.HandleObject(newObj, radix.KindRadixDNSAlias, getOwner) // restore ingress if it does not correspond to RadixDNSAlias
		},
		DeleteFunc: func(obj interface{}) {
			ing, converted := obj.(*networkingv1.Ingress)
			if !converted {
				logger.Errorf("Ingress object cast failed during deleted event received.")
				return
			}
			logger.Debugf("deleted Ingress %s", ing.GetName())
			controller.HandleObject(ing, radix.KindRadixDNSAlias, getOwner) // restore ingress if RadixDNSAlias exist
		},
	})
	if err != nil {
		logger.Errorf("failed to add an event hanflers to the ingressInformer")
	}
	return controller
}

func getRadixDNSAliasForAppAndEnvironment(radixClient radixclient.Interface, appName string, envName string) ([]radixv1.RadixDNSAlias, error) {
	radixDNSAliasList, err := radixClient.RadixV1().RadixDNSAliases().List(context.Background(), metav1.ListOptions{
		LabelSelector: radixlabels.Merge(radixlabels.ForApplicationName(appName),
			radixlabels.ForEnvironmentName(envName)).String(),
	})
	if err != nil {
		return nil, err
	}
	return radixDNSAliasList.Items, err
}

func deepEqual(old, new *radixv1.RadixDNSAlias) bool {
	return reflect.DeepEqual(new.Spec, old.Spec) &&
		reflect.DeepEqual(new.ObjectMeta.Labels, old.ObjectMeta.Labels) &&
		reflect.DeepEqual(new.ObjectMeta.Annotations, old.ObjectMeta.Annotations)
}

func getOwner(radixClient radixclient.Interface, _, name string) (interface{}, error) {
	return radixClient.RadixV1().RadixDNSAliases().Get(context.Background(), name, metav1.GetOptions{})
}

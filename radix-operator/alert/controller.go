package alert

import (
	"context"
	"fmt"
	"reflect"

	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/metrics"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/equinor/radix-operator/radix-operator/common"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

var logger *log.Entry

const (
	controllerAgentName = "alert-controller"
	crType              = "RadixAlerts"
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
	waitForChildrenToSync bool,
	recorder record.EventRecorder) *common.Controller {

	alertConfigInformer := radixInformerFactory.Radix().V1().RadixAlerts()
	registrationInformer := radixInformerFactory.Radix().V1().RadixRegistrations()

	controller := &common.Controller{
		Name:                  controllerAgentName,
		HandlerOf:             crType,
		KubeClient:            client,
		RadixClient:           radixClient,
		Informer:              alertConfigInformer.Informer(),
		KubeInformerFactory:   kubeInformerFactory,
		WorkQueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), crType),
		Handler:               handler,
		Log:                   logger,
		WaitForChildrenToSync: waitForChildrenToSync,
		Recorder:              recorder,
	}

	logger.Info("Setting up event handlers")
	alertConfigInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			controller.Enqueue(cur)
			metrics.CustomResourceAdded(crType)
		},
		UpdateFunc: func(old, cur interface{}) {
			oldRadixAlert := old.(*radixv1.RadixAlert)
			newRadixAlert := cur.(*radixv1.RadixAlert)
			if deepEqual(oldRadixAlert, newRadixAlert) {
				logger.Debugf("RadixAlert object is equal to old for %s. Do nothing", newRadixAlert.GetName())
				metrics.CustomResourceUpdatedButSkipped(crType)
				return
			}

			controller.Enqueue(cur)
		},
		DeleteFunc: func(obj interface{}) {
			radixAlert, _ := obj.(*radixv1.RadixAlert)
			key, err := cache.MetaNamespaceKeyFunc(radixAlert)
			if err == nil {
				logger.Debugf("RadixAlert object deleted event received for %s. Do nothing", key)
			}
			metrics.CustomResourceDeleted(crType)
		},
	})

	registrationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			newRr := newObj.(*radixv1.RadixRegistration)
			oldRr := oldObj.(*radixv1.RadixRegistration)
			if newRr.ResourceVersion == oldRr.ResourceVersion {
				return
			}

			// If neither ad group did change, nor the machine user, this
			// does not affect the alert
			if radixutils.ArrayEqualElements(newRr.Spec.AdGroups, oldRr.Spec.AdGroups) &&
				newRr.Spec.MachineUser == oldRr.Spec.MachineUser {
				return
			}

			// Get all namespaces belonging to this RR and resync all RadixAlert that exists in the namespaces
			radixalerts, err := radixClient.RadixV1().RadixAlerts(corev1.NamespaceAll).List(context.TODO(), v1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", kube.RadixAppLabel, newRr.Name),
			})

			if err == nil {
				for _, radixalert := range radixalerts.Items {
					controller.Enqueue(&radixalert)
				}
			}
		},
	})
	// TODO: On RR update, get all namespaces with radix-app=rrname=>foreach ns;get radixalerts and queue

	return controller
}

func deepEqual(old, new *radixv1.RadixAlert) bool {
	if !reflect.DeepEqual(new.Spec, old.Spec) ||
		!reflect.DeepEqual(new.ObjectMeta.Labels, old.ObjectMeta.Labels) ||
		!reflect.DeepEqual(new.ObjectMeta.Annotations, old.ObjectMeta.Annotations) {
		return false
	}

	return true
}

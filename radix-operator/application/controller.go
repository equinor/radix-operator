package application

import (
	"context"
	"reflect"

	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/equinor/radix-operator/pkg/apis/metrics"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/equinor/radix-operator/radix-operator/common"
	log "github.com/sirupsen/logrus"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

var logger *log.Entry

const (
	controllerAgentName = "application-controller"
	crType              = "RadixApplications"
)

func init() {
	logger = log.WithFields(log.Fields{"radixOperatorComponent": controllerAgentName})
}

// NewController creates a new controller that handles RadixApplications
func NewController(client kubernetes.Interface,
	radixClient radixclient.Interface, handler common.Handler,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	radixInformerFactory informers.SharedInformerFactory,
	waitForChildrenToSync bool,
	recorder record.EventRecorder) *common.Controller {

	applicationInformer := radixInformerFactory.Radix().V1().RadixApplications()
	registrationInformer := radixInformerFactory.Radix().V1().RadixRegistrations()

	controller := &common.Controller{
		Name:                  controllerAgentName,
		HandlerOf:             crType,
		KubeClient:            client,
		RadixClient:           radixClient,
		Informer:              applicationInformer.Informer(),
		KubeInformerFactory:   kubeInformerFactory,
		WorkQueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), crType),
		Handler:               handler,
		Log:                   logger,
		WaitForChildrenToSync: waitForChildrenToSync,
		Recorder:              recorder,
		KeyFunc:               common.KeyByNamespace,
	}

	logger.Info("Setting up event handlers")
	applicationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			controller.Enqueue(cur)
			metrics.CustomResourceAdded(crType)
		},
		UpdateFunc: func(old, cur interface{}) {
			oldRA := old.(*v1.RadixApplication)
			newRA := cur.(*v1.RadixApplication)
			if deepEqual(oldRA, newRA) {
				logger.Debugf("Application object is equal to old for %s. Do nothing", newRA.GetName())
				metrics.CustomResourceUpdatedButSkipped(crType)
				return
			}

			controller.Enqueue(cur)
		},
		DeleteFunc: func(obj interface{}) {
			radixApplication, _ := obj.(*v1.RadixApplication)
			key, err := cache.MetaNamespaceKeyFunc(radixApplication)
			if err == nil {
				logger.Debugf("Application object deleted event received for %s. Do nothing", key)
			}
			metrics.CustomResourceDeleted(crType)
		},
	})
	registrationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, cur interface{}) {
			newRr := cur.(*v1.RadixRegistration)
			oldRr := old.(*v1.RadixRegistration)

			// If neither ad group did change, nor the machine user, this
			// does not affect the deployment
			if radixutils.ArrayEqualElements(newRr.Spec.AdGroups, oldRr.Spec.AdGroups) &&
				newRr.Spec.MachineUser == oldRr.Spec.MachineUser {
				return
			}
			ra, err := radixClient.RadixV1().RadixApplications(utils.GetAppNamespace(newRr.Name)).Get(context.TODO(), newRr.Name, metav1.GetOptions{})
			if err != nil {
				logger.Errorf("cannot get Radix Application object by name %s: %v", newRr.Name, err)
				return
			}
			logger.Debugf("update Radix Application due to changed AAD group or machine user")
			controller.Enqueue(ra)
			metrics.CustomResourceUpdated(crType)
		},
	})

	return controller
}

func deepEqual(old, new *v1.RadixApplication) bool {
	if !reflect.DeepEqual(new.Spec, old.Spec) ||
		!reflect.DeepEqual(new.ObjectMeta.Labels, old.ObjectMeta.Labels) ||
		!reflect.DeepEqual(new.ObjectMeta.Annotations, old.ObjectMeta.Annotations) {
		return false
	}

	return true
}

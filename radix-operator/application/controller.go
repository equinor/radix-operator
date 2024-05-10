package application

import (
	"context"
	"reflect"

	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/equinor/radix-operator/pkg/apis/metrics"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/equinor/radix-operator/radix-operator/common"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

const (
	controllerAgentName = "application-controller"
	crType              = "RadixApplications"
)

// NewController creates a new controller that handles RadixApplications
func NewController(ctx context.Context, client kubernetes.Interface,
	radixClient radixclient.Interface, handler common.Handler,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	radixInformerFactory informers.SharedInformerFactory,
	waitForChildrenToSync bool,
	recorder record.EventRecorder) *common.Controller {
	logger := log.With().Str("controller", controllerAgentName).Logger()
	applicationInformer := radixInformerFactory.Radix().V1().RadixApplications()
	registrationInformer := radixInformerFactory.Radix().V1().RadixRegistrations()

	controller := &common.Controller{
		Name:                  controllerAgentName,
		HandlerOf:             crType,
		KubeClient:            client,
		RadixClient:           radixClient,
		Informer:              applicationInformer.Informer(),
		KubeInformerFactory:   kubeInformerFactory,
		WorkQueue:             common.NewRateLimitedWorkQueue(ctx, crType),
		Handler:               handler,
		Log:                   logger,
		WaitForChildrenToSync: waitForChildrenToSync,
		Recorder:              recorder,
		LockKeyAndIdentifier:  common.NamespacePartitionKey,
	}

	logger.Info().Msg("Setting up event handlers")
	if _, err := applicationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			if _, err := controller.Enqueue(cur); err != nil {
				logger.Error().Err(err).Msg("Failed to enqueue object received from RadixApplication informer AddFunc")
			}
			metrics.CustomResourceAdded(crType)
		},
		UpdateFunc: func(old, cur interface{}) {
			oldRA := old.(*v1.RadixApplication)
			newRA := cur.(*v1.RadixApplication)
			if deepEqual(oldRA, newRA) {
				logger.Debug().Msgf("Application object is equal to old for %s. Do nothing", newRA.GetName())
				metrics.CustomResourceUpdatedButSkipped(crType)
				return
			}

			if _, err := controller.Enqueue(cur); err != nil {
				logger.Error().Err(err).Msg("Failed to enqueue object received from RadixApplication informer UpdateFunc")
			}
		},
		DeleteFunc: func(obj interface{}) {
			radixApplication, converted := obj.(*v1.RadixApplication)
			if !converted {
				logger.Error().Msg("RadixApplication object cast failed during deleted event received.")
				return
			}
			key, err := cache.MetaNamespaceKeyFunc(radixApplication)
			if err == nil {
				logger.Debug().Msgf("Application object deleted event received for %s. Do nothing", key)
			}
			metrics.CustomResourceDeleted(crType)
		},
	}); err != nil {
		panic(err)
	}
	if _, err := registrationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, cur interface{}) {
			newRr := cur.(*v1.RadixRegistration)
			oldRr := old.(*v1.RadixRegistration)

			// If neither admin or reader AD groups change, this
			// does not affect the deployment
			if radixutils.ArrayEqualElements(newRr.Spec.AdGroups, oldRr.Spec.AdGroups) &&
				radixutils.ArrayEqualElements(newRr.Spec.ReaderAdGroups, oldRr.Spec.ReaderAdGroups) {
				return
			}
			ra, err := radixClient.RadixV1().RadixApplications(utils.GetAppNamespace(newRr.Name)).Get(ctx, newRr.Name, metav1.GetOptions{})
			if err != nil {
				logger.Error().Err(err).Msgf("Cannot get Radix Application object by name %s", newRr.Name)
				return
			}
			logger.Debug().Msg("update Radix Application due to changed admin or reader AD groups")
			if _, err := controller.Enqueue(ra); err != nil {
				logger.Error().Err(err).Msg("Failed to enqueue object received from RadixRegistration informer UpdateFunc")
			}
			metrics.CustomResourceUpdated(crType)
		},
	}); err != nil {
		panic(err)
	}

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

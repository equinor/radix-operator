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
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

const (
	controllerAgentName = "alert-controller"
	crType              = "RadixAlerts"
)

// NewController creates a new controller that handles RadixAlerts
func NewController(ctx context.Context,
	kubeClient kubernetes.Interface,
	radixClient radixclient.Interface,
	handler common.Handler,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	radixInformerFactory informers.SharedInformerFactory,
	recorder record.EventRecorder) *common.Controller {
	logger := log.With().Str("controller", controllerAgentName).Logger()
	alertInformer := radixInformerFactory.Radix().V1().RadixAlerts()
	registrationInformer := radixInformerFactory.Radix().V1().RadixRegistrations()

	controller := &common.Controller{
		Name:                 controllerAgentName,
		HandlerOf:            crType,
		KubeClient:           kubeClient,
		RadixClient:          radixClient,
		KubeInformerFactory:  kubeInformerFactory,
		RadixInformerFactory: radixInformerFactory,
		WorkQueue:            common.NewRateLimitedWorkQueue(ctx, crType),
		Handler:              handler,
		Recorder:             recorder,
		LockKeyAndIdentifier: common.NamespacePartitionKey,
	}

	logger.Info().Msg("Setting up event handlers")
	if _, err := alertInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			if _, err := controller.Enqueue(cur); err != nil {
				logger.Error().Err(err).Msg("Failed to enqueue object received from RadixAlert informer AddFunc")
			}
			metrics.CustomResourceAdded(crType)
		},
		UpdateFunc: func(old, cur interface{}) {
			oldRadixAlert := old.(*radixv1.RadixAlert)
			newRadixAlert := cur.(*radixv1.RadixAlert)
			if deepEqual(oldRadixAlert, newRadixAlert) {
				logger.Debug().Msgf("RadixAlert object is equal to old for %s. Do nothing", newRadixAlert.GetName())
				metrics.CustomResourceUpdatedButSkipped(crType)
				return
			}

			if _, err := controller.Enqueue(cur); err != nil {
				logger.Error().Err(err).Msg("Failed to enqueue object received from RadixAlert informer UpdateFunc")
			}
		},
		DeleteFunc: func(obj interface{}) {
			radixAlert, converted := obj.(*radixv1.RadixAlert)
			if !converted {
				logger.Error().Msg("RadixAlert object cast failed during deleted event received.")
				return
			}
			key, err := cache.MetaNamespaceKeyFunc(radixAlert)
			if err == nil {
				logger.Debug().Msgf("RadixAlert object deleted event received for %s. Do nothing", key)
			}
			metrics.CustomResourceDeleted(crType)
		},
	}); err != nil {
		panic(err)
	}

	if _, err := registrationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			newRr := newObj.(*radixv1.RadixRegistration)
			oldRr := oldObj.(*radixv1.RadixRegistration)
			if newRr.ResourceVersion == oldRr.ResourceVersion {
				return
			}

			// If neither admin or reader AD groups change, this
			// does not affect the alert
			if radixutils.ArrayEqualElements(newRr.Spec.AdGroups, oldRr.Spec.AdGroups) &&
				radixutils.ArrayEqualElements(newRr.Spec.ReaderAdGroups, oldRr.Spec.ReaderAdGroups) {
				return
			}

			// Enqueue all RadixAlerts with radix-app label matching name of RR
			radixalerts, err := radixClient.RadixV1().RadixAlerts(corev1.NamespaceAll).List(ctx, v1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", kube.RadixAppLabel, newRr.Name),
			})
			if err == nil {
				for _, radixalert := range radixalerts.Items {
					if _, err := controller.Enqueue(&radixalert); err != nil {
						logger.Error().Err(err).Msg("Failed to enqueue object received from RadixRegistration informer UpdateFunc")
					}
				}
			}
		},
	}); err != nil {
		panic(err)
	}

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

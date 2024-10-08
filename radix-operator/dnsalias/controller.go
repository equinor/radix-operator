package dnsalias

import (
	"context"
	"reflect"

	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/metrics"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	radixinformersv1 "github.com/equinor/radix-operator/pkg/client/informers/externalversions/radix/v1"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

const (
	controllerAgentName = "dns-alias-controller"
)

// NewController creates a new controller that handles RadixDNSAlias
func NewController(ctx context.Context, kubeClient kubernetes.Interface,
	radixClient radixclient.Interface,
	handler common.Handler,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	radixInformerFactory informers.SharedInformerFactory,
	recorder record.EventRecorder) *common.Controller {
	logger := log.With().Str("controller", controllerAgentName).Logger()
	radixDNSAliasInformer := radixInformerFactory.Radix().V1().RadixDNSAliases()

	controller := &common.Controller{
		Name:                 controllerAgentName,
		HandlerOf:            radixv1.KindRadixDNSAlias,
		KubeClient:           kubeClient,
		RadixClient:          radixClient,
		KubeInformerFactory:  kubeInformerFactory,
		RadixInformerFactory: radixInformerFactory,
		WorkQueue:            common.NewRateLimitedWorkQueue(ctx, radixv1.KindRadixDNSAlias),
		Handler:              handler,
		Recorder:             recorder,
		LockKeyAndIdentifier: common.NamePartitionKey,
	}

	logger.Info().Msg("Setting up event handlers")
	addEventHandlersForRadixDNSAliases(radixDNSAliasInformer, controller, &logger)
	addEventHandlersForRadixDeployments(radixInformerFactory, controller, radixClient, &logger)
	addEventHandlersForIngresses(ctx, kubeInformerFactory, controller, &logger)
	addEventHandlersForRadixRegistrations(radixInformerFactory, controller, radixClient, &logger)
	addEventHandlersForRadixApplication(radixInformerFactory, controller, radixClient, &logger)
	return controller
}

func addEventHandlersForRadixRegistrations(radixInformerFactory informers.SharedInformerFactory, controller *common.Controller, radixClient radixclient.Interface, logger *zerolog.Logger) {
	radixRegistrationInformer := radixInformerFactory.Radix().V1().RadixRegistrations()
	if _, err := radixRegistrationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldRR := oldObj.(*radixv1.RadixRegistration)
			newRR := newObj.(*radixv1.RadixRegistration)
			if oldRR.GetResourceVersion() == newRR.GetResourceVersion() &&
				radixutils.ArrayEqualElements(oldRR.Spec.AdGroups, newRR.Spec.AdGroups) &&
				radixutils.ArrayEqualElements(oldRR.Spec.AdUsers, newRR.Spec.AdUsers) &&
				radixutils.ArrayEqualElements(oldRR.Spec.ReaderAdGroups, newRR.Spec.ReaderAdGroups) &&
				radixutils.ArrayEqualElements(oldRR.Spec.ReaderAdUsers, newRR.Spec.ReaderAdUsers) {
				return // updating RadixRegistration has the same resource version. Do nothing.
			}
			enqueueRadixDNSAliasesForAppName(controller, radixClient, newRR.GetName(), logger)
		},
	}); err != nil {
		panic(err)
	}
}

func addEventHandlersForRadixApplication(radixInformerFactory informers.SharedInformerFactory, controller *common.Controller, radixClient radixclient.Interface, logger *zerolog.Logger) {
	informer := radixInformerFactory.Radix().V1().RadixApplications()
	if _, err := informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldRA := oldObj.(*radixv1.RadixApplication)
			newRA := newObj.(*radixv1.RadixApplication)
			if oldRA.GetResourceVersion() == newRA.GetResourceVersion() ||
				equalDNSAliases(oldRA.Spec.DNSAlias, newRA.Spec.DNSAlias) {
				return // updating RadixApplication has the same resource version and DNS aliases. Do nothing.
			}
			enqueueRadixDNSAliasesForAppName(controller, radixClient, newRA.GetName(), logger)
		},
	}); err != nil {
		panic(err)
	}
}

func equalDNSAliases(dnsAliases1, dnsAliases2 []radixv1.DNSAlias) bool {
	if len(dnsAliases1) != len(dnsAliases2) {
		return false
	}
	dnsAlias1Map := slice.Reduce(dnsAliases1, make(map[string]radixv1.DNSAlias), func(acc map[string]radixv1.DNSAlias, dnsAlias radixv1.DNSAlias) map[string]radixv1.DNSAlias {
		acc[dnsAlias.Alias] = dnsAlias
		return acc
	})
	return slice.All(dnsAliases2, func(dnsAlias2 radixv1.DNSAlias) bool {
		dnsAlias1, ok := dnsAlias1Map[dnsAlias2.Alias]
		return ok && dnsAlias1.Environment == dnsAlias2.Environment && dnsAlias1.Component == dnsAlias2.Component
	})
}

func addEventHandlersForIngresses(ctx context.Context, kubeInformerFactory kubeinformers.SharedInformerFactory, controller *common.Controller, logger *zerolog.Logger) {
	ingressInformer := kubeInformerFactory.Networking().V1().Ingresses()
	if _, err := ingressInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldIng := oldObj.(metav1.Object)
			newIng := newObj.(metav1.Object)
			if oldIng.GetResourceVersion() == newIng.GetResourceVersion() {
				logger.Debug().Msgf("updating Ingress %s has the same resource version. Do nothing.", newIng.GetName())
				return
			}
			logger.Debug().Msgf("updated Ingress %s", newIng.GetName())
			controller.HandleObject(ctx, newObj, radixv1.KindRadixDNSAlias, getOwner) // restore ingress if it does not correspond to RadixDNSAlias
		},
		DeleteFunc: func(obj interface{}) {
			ing, converted := obj.(*networkingv1.Ingress)
			if !converted {
				logger.Error().Msg("Ingress object cast failed during deleted event received.")
				return
			}
			logger.Debug().Msgf("deleted Ingress %s", ing.GetName())
			controller.HandleObject(ctx, ing, radixv1.KindRadixDNSAlias, getOwner) // restore ingress if RadixDNSAlias exist
		},
	}); err != nil {
		panic(err)
	}
}

func addEventHandlersForRadixDeployments(radixInformerFactory informers.SharedInformerFactory, controller *common.Controller, radixClient radixclient.Interface, logger *zerolog.Logger) {
	radixDeploymentInformer := radixInformerFactory.Radix().V1().RadixDeployments()
	if _, err := radixDeploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			rd := cur.(*radixv1.RadixDeployment)
			enqueueRadixDNSAliasesForRadixDeployment(controller, radixClient, rd, logger)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldRD := oldObj.(metav1.Object)
			newRD := newObj.(metav1.Object)
			if oldRD.GetResourceVersion() == newRD.GetResourceVersion() {
				return // updating RadixDeployment has the same resource version. Do nothing.
			}
			rd := newRD.(*radixv1.RadixDeployment)
			enqueueRadixDNSAliasesForRadixDeployment(controller, radixClient, rd, logger)
		},
	}); err != nil {
		panic(err)
	}
}

func addEventHandlersForRadixDNSAliases(radixDNSAliasInformer radixinformersv1.RadixDNSAliasInformer, controller *common.Controller, logger *zerolog.Logger) {
	if _, err := radixDNSAliasInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			alias := cur.(*radixv1.RadixDNSAlias)
			logger.Debug().Msgf("added RadixDNSAlias %s", alias.GetName())
			_, err := controller.Enqueue(cur)
			if err != nil {
				logger.Error().Err(err).Msgf("failed to enqueue the RadixDNSAlias %s", alias.GetName())
			}
			metrics.CustomResourceAdded(radixv1.KindRadixDNSAlias)
		},
		UpdateFunc: func(old, cur interface{}) {
			oldAlias := old.(*radixv1.RadixDNSAlias)
			newAlias := cur.(*radixv1.RadixDNSAlias)
			if deepEqual(oldAlias, newAlias) {
				logger.Debug().Msgf("RadixDNSAlias object is equal to old for %s. Do nothing", newAlias.GetName())
				metrics.CustomResourceUpdatedButSkipped(radixv1.KindRadixDNSAlias)
				return
			}
			logger.Debug().Msgf("updated RadixDNSAlias %s", newAlias.GetName())
			_, err := controller.Enqueue(cur)
			if err != nil {
				logger.Error().Err(err).Msgf("failed to enqueue the RadixDNSAlias %s", newAlias.GetName())
			}
			metrics.CustomResourceUpdated(radixv1.KindRadixDNSAlias)
		},
		DeleteFunc: func(obj interface{}) {
			alias, converted := obj.(*radixv1.RadixDNSAlias)
			if !converted {
				logger.Error().Msg("RadixDNSAlias object cast failed during deleted event received.")
				return
			}
			logger.Debug().Msgf("deleted RadixDNSAlias %s", alias.GetName())
			key, err := cache.MetaNamespaceKeyFunc(alias)
			if err != nil {
				logger.Error().Err(err).Msgf("error on RadixDNSAlias object deleted event received for %s", key)
			}
			metrics.CustomResourceDeleted(radixv1.KindRadixDNSAlias)
		},
	}); err != nil {
		panic(err)
	}
}

func enqueueRadixDNSAliasesForRadixDeployment(controller *common.Controller, radixClient radixclient.Interface, rd *radixv1.RadixDeployment, logger *zerolog.Logger) {
	if !rd.Status.ActiveTo.IsZero() {
		return // skip not active RadixDeployments
	}
	logger.Debug().Msgf("Added or updated an active RadixDeployment %s to application %s in the environment %s. Enqueue relevant RadixDNSAliases", rd.GetName(), rd.Spec.AppName, rd.Spec.Environment)
	radixDNSAliases, err := getRadixDNSAliasForAppAndEnvironment(radixClient, rd.Spec.AppName, rd.Spec.Environment)
	if err != nil {
		logger.Error().Err(err).Msgf("failed to get list of RadixDNSAliases for the application %s", rd.Spec.AppName)
		return
	}
	for _, radixDNSAlias := range radixDNSAliases {
		radixDNSAlias := radixDNSAlias
		logger.Debug().Msgf("re-sync RadixDNSAlias %s", radixDNSAlias.GetName())
		if _, err := controller.Enqueue(&radixDNSAlias); err != nil {
			logger.Error().Err(err).Msgf("failed to enqueue RadixDNSAlias %s", radixDNSAlias.GetName())
		}
	}
}

func enqueueRadixDNSAliasesForAppName(controller *common.Controller, radixClient radixclient.Interface, appName string, logger *zerolog.Logger) {
	logger.Debug().Msgf("Added or updated an RadixRegistration %s. Enqueue relevant RadixDNSAliases", appName)
	radixDNSAliases, err := getRadixDNSAliasForApp(radixClient, appName)
	if err != nil {
		logger.Error().Err(err).Msgf("failed to get list of RadixDNSAliases for the application %s", appName)
		return
	}
	for _, radixDNSAlias := range radixDNSAliases {
		radixDNSAlias := radixDNSAlias
		logger.Debug().Msgf("Enqueue RadixDNSAlias %s", radixDNSAlias.GetName())
		if _, err := controller.Enqueue(&radixDNSAlias); err != nil {
			logger.Error().Err(err).Msgf("failed to enqueue RadixDNSAlias %s", radixDNSAlias.GetName())
		}
	}
}

func getRadixDNSAliasForAppAndEnvironment(radixClient radixclient.Interface, appName, envName string) ([]radixv1.RadixDNSAlias, error) {
	radixDNSAliasList, err := radixClient.RadixV1().RadixDNSAliases().List(context.Background(), metav1.ListOptions{
		LabelSelector: radixlabels.Merge(radixlabels.ForApplicationName(appName),
			radixlabels.ForEnvironmentName(envName)).String(),
	})
	if err != nil {
		return nil, err
	}
	return radixDNSAliasList.Items, err
}

func getRadixDNSAliasForApp(radixClient radixclient.Interface, appName string) ([]radixv1.RadixDNSAlias, error) {
	radixDNSAliasList, err := radixClient.RadixV1().RadixDNSAliases().List(context.Background(), metav1.ListOptions{
		LabelSelector: radixlabels.ForApplicationName(appName).String(),
	})
	if err != nil {
		return nil, err
	}
	return radixDNSAliasList.Items, err
}

func deepEqual(old, new *radixv1.RadixDNSAlias) bool {
	return reflect.DeepEqual(new.Spec, old.Spec) &&
		reflect.DeepEqual(new.ObjectMeta.Labels, old.ObjectMeta.Labels) &&
		reflect.DeepEqual(new.ObjectMeta.Annotations, old.ObjectMeta.Annotations) &&
		reflect.DeepEqual(new.ObjectMeta.Finalizers, old.ObjectMeta.Finalizers) &&
		reflect.DeepEqual(new.ObjectMeta.DeletionTimestamp, old.ObjectMeta.DeletionTimestamp)
}

func getOwner(ctx context.Context, radixClient radixclient.Interface, _, name string) (interface{}, error) {
	return radixClient.RadixV1().RadixDNSAliases().Get(ctx, name, metav1.GetOptions{})
}

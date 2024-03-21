package environment

import (
	"context"
	"fmt"
	"reflect"

	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/metrics"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

const (
	controllerAgentName = "environment-controller"
	crType              = "RadixEnvironments"
)

// NewController creates a new controller that handles RadixEnvironments
func NewController(client kubernetes.Interface,
	radixClient radixclient.Interface,
	handler common.Handler,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	radixInformerFactory informers.SharedInformerFactory,
	waitForChildrenToSync bool,
	recorder record.EventRecorder) *common.Controller {
	logger := log.With().Str("controller", controllerAgentName).Logger()
	environmentInformer := radixInformerFactory.Radix().V1().RadixEnvironments()
	registrationInformer := radixInformerFactory.Radix().V1().RadixRegistrations()
	applicationInformer := radixInformerFactory.Radix().V1().RadixApplications()

	controller := &common.Controller{
		Name:                  controllerAgentName,
		HandlerOf:             crType,
		KubeClient:            client,
		RadixClient:           radixClient,
		Informer:              environmentInformer.Informer(),
		KubeInformerFactory:   kubeInformerFactory,
		WorkQueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), crType),
		Handler:               handler,
		Log:                   logger,
		WaitForChildrenToSync: waitForChildrenToSync,
		Recorder:              recorder,
		LockKeyAndIdentifier:  common.NamePartitionKey,
	}

	logger.Info().Msg("Setting up event handlers")

	if _, err := environmentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			if _, err := controller.Enqueue(cur); err != nil {
				logger.Error().Err(err).Msg("Failed to enqueue object received from RadixEnvironment informer AddFunc")
			}
			metrics.CustomResourceAdded(crType)
		},
		UpdateFunc: func(old, cur interface{}) {
			newRR := cur.(*v1.RadixEnvironment)
			oldRR := old.(*v1.RadixEnvironment)

			if deepEqual(oldRR, newRR) {
				logger.Debug().Msgf("RadixEnvironment %s (revision %s) is equal to old (revision %s). Do nothing", newRR.GetName(), newRR.GetResourceVersion(), oldRR.GetResourceVersion())
				metrics.CustomResourceUpdatedButSkipped(crType)
				return
			}
			logger.Debug().Msgf("update RadixEnvironment %s (from revision %s to %s)", oldRR.GetName(), oldRR.GetResourceVersion(), newRR.GetResourceVersion())
			if _, err := controller.Enqueue(cur); err != nil {
				logger.Error().Err(err).Msg("Failed to enqueue object received from RadixEnvironment informer UpdateFunc")
			}
			metrics.CustomResourceUpdated(crType)
		},
		DeleteFunc: func(obj interface{}) {
			radixEnvironment, converted := obj.(*v1.RadixEnvironment)
			if !converted {
				logger.Error().Msg("RadixEnvironment object cast failed during deleted event received")
				return
			}
			key, err := cache.MetaNamespaceKeyFunc(radixEnvironment)
			if err == nil {
				logger.Debug().Msgf("RadixEnvironment object deleted event received for %s (revision %s). Do nothing", key, radixEnvironment.GetResourceVersion())
			}
			metrics.CustomResourceDeleted(crType)
		},
	}); err != nil {
		panic(err)
	}

	namespaceInformer := kubeInformerFactory.Core().V1().Namespaces()
	if _, err := namespaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) {
			// attempt to sync environment if it is the owner of this namespace
			controller.HandleObject(obj, v1.KindRadixEnvironment, getOwner)
		},
	}); err != nil {
		panic(err)
	}

	rolebindingInformer := kubeInformerFactory.Rbac().V1().RoleBindings()
	if _, err := rolebindingInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) {
			// attempt to sync environment if it is the owner of this role-binding
			controller.HandleObject(obj, v1.KindRadixEnvironment, getOwner)
		},
	}); err != nil {
		panic(err)
	}

	limitrangeInformer := kubeInformerFactory.Core().V1().LimitRanges()
	if _, err := limitrangeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) {
			// attempt to sync environment if it is the owner of this limit-range
			controller.HandleObject(obj, v1.KindRadixEnvironment, getOwner)
		},
	}); err != nil {
		panic(err)
	}

	if _, err := registrationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, cur interface{}) {
			newRr := cur.(*v1.RadixRegistration)
			oldRr := old.(*v1.RadixRegistration)
			if newRr.ResourceVersion == oldRr.ResourceVersion {
				return
			}

			// If neither admin or reader AD groups change, this
			// does not affect the deployment
			if radixutils.ArrayEqualElements(newRr.Spec.AdGroups, oldRr.Spec.AdGroups) &&
				radixutils.ArrayEqualElements(newRr.Spec.ReaderAdGroups, oldRr.Spec.ReaderAdGroups) {
				return
			}

			// Trigger sync of all REs, belonging to the registration
			environments, err := radixClient.RadixV1().RadixEnvironments().List(
				context.TODO(),
				metav1.ListOptions{
					LabelSelector: fmt.Sprintf("%s=%s", kube.RadixAppLabel, oldRr.Name),
				})

			if err == nil {
				for _, environment := range environments.Items {
					// Will sync the environment
					if _, err := controller.Enqueue(&environment); err != nil {
						logger.Error().Err(err).Msg("Failed to enqueue object received from RadixRegistration informer UpdateFunc")
					}
				}
			}
		},
	}); err != nil {
		panic(err)
	}

	if _, err := applicationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, cur interface{}) {
			newRa := cur.(*v1.RadixApplication)
			oldRa := old.(*v1.RadixApplication)
			if newRa.ResourceVersion == oldRa.ResourceVersion {
				return
			}

			environmentsToResync := getAddedOrDroppedEnvironmentNames(oldRa, newRa)
			for _, envName := range environmentsToResync {
				uniqueName := utils.GetEnvironmentNamespace(oldRa.Name, envName)
				re, err := radixClient.RadixV1().RadixEnvironments().Get(context.TODO(), uniqueName, metav1.GetOptions{})
				if err == nil {
					if _, err := controller.Enqueue(re); err != nil {
						logger.Error().Err(err).Msg("Failed to enqueue object received from RadixApplication informer UpdateFunc")
					}
				}
			}
		},
		DeleteFunc: func(cur interface{}) {
			radixApplication, converted := cur.(*v1.RadixApplication)
			if !converted {
				logger.Error().Msg("RadixApplication object cast failed during deleted event received.")
				return
			}
			for _, env := range radixApplication.Spec.Environments {
				uniqueName := utils.GetEnvironmentNamespace(radixApplication.Name, env.Name)
				re, err := radixClient.RadixV1().RadixEnvironments().Get(context.TODO(), uniqueName, metav1.GetOptions{})
				if err == nil {
					if _, err := controller.Enqueue(re); err != nil {
						logger.Error().Err(err).Msg("Failed to enqueue object received from RadixApplication informer DeleteFunc")
					}
				}
			}
		},
	}); err != nil {
		panic(err)
	}

	return controller
}

func deepEqual(old, new *v1.RadixEnvironment) bool {
	if !reflect.DeepEqual(new.Spec, old.Spec) ||
		!reflect.DeepEqual(new.ObjectMeta.Labels, old.ObjectMeta.Labels) ||
		!reflect.DeepEqual(new.ObjectMeta.Annotations, old.ObjectMeta.Annotations) ||
		!reflect.DeepEqual(new.ObjectMeta.DeletionTimestamp, old.ObjectMeta.DeletionTimestamp) ||
		!reflect.DeepEqual(new.ObjectMeta.Finalizers, old.ObjectMeta.Finalizers) {
		return false
	}

	return true
}

func getOwner(radixClient radixclient.Interface, namespace, name string) (interface{}, error) {
	return radixClient.RadixV1().RadixEnvironments().Get(context.TODO(), name, metav1.GetOptions{})
}

func getAddedOrDroppedEnvironmentNames(oldRa *v1.RadixApplication, newRa *v1.RadixApplication) []string {
	var environmentNames []string
	environmentNames = append(environmentNames, getMissingEnvironmentNames(oldRa.Spec.Environments, newRa.Spec.Environments)...)
	environmentNames = append(environmentNames, getMissingEnvironmentNames(newRa.Spec.Environments, oldRa.Spec.Environments)...)
	return environmentNames
}

// getMissingEnvironmentNames returns environment names that exists in source list but not in target list
func getMissingEnvironmentNames(source []v1.Environment, target []v1.Environment) []string {
	droppedNames := make([]string, 0)
	for _, oldEnvConfig := range source {
		dropped := true
		for _, newEnvConfig := range target {
			if oldEnvConfig.Name == newEnvConfig.Name {
				dropped = false
			}
		}
		if dropped {
			droppedNames = append(droppedNames, oldEnvConfig.Name)
		}
	}
	return droppedNames
}

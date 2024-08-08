package deployment

import (
	"context"
	"fmt"
	"reflect"

	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/metrics"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

const (
	controllerAgentName = "deployment-controller"
	crType              = "RadixDeployments"
)

// NewController creates a new controller that handles RadixDeployments
func NewController(ctx context.Context,
	kubeClient kubernetes.Interface,
	radixClient radixclient.Interface,
	handler common.Handler,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	radixInformerFactory informers.SharedInformerFactory,
	recorder record.EventRecorder) *common.Controller {
	logger := log.Ctx(ctx).With().Str("controller", controllerAgentName).Logger()
	ctx = logger.WithContext(ctx)
	deploymentInformer := radixInformerFactory.Radix().V1().RadixDeployments()
	serviceInformer := kubeInformerFactory.Core().V1().Services()
	registrationInformer := radixInformerFactory.Radix().V1().RadixRegistrations()

	controller := &common.Controller{
		Name:                 controllerAgentName,
		HandlerOf:            crType,
		KubeClient:           kubeClient,
		RadixClient:          radixClient,
		Informer:             deploymentInformer.Informer(),
		KubeInformerFactory:  kubeInformerFactory,
		RadixInformerFactory: radixInformerFactory,
		WorkQueue:            common.NewRateLimitedWorkQueue(ctx, crType),
		Handler:              handler,
		Recorder:             recorder,
		LockKeyAndIdentifier: common.NamespacePartitionKey,
	}

	logger.Info().Msg("Setting up event handlers")
	if _, err := deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			radixDeployment, _ := cur.(*v1.RadixDeployment)
			if deployment.IsRadixDeploymentInactive(radixDeployment) {
				logger.Debug().Msgf("Skip deployment object %s as it is inactive", radixDeployment.GetName())
				metrics.CustomResourceAddedButSkipped(crType)
				return
			}

			if _, err := controller.Enqueue(cur); err != nil {
				logger.Error().Err(err).Msg("Failed to enqueue object received from RadixRegistration informer AddFunc")
			}
			metrics.CustomResourceAdded(crType)
		},
		UpdateFunc: func(old, cur interface{}) {
			newRD := cur.(*v1.RadixDeployment)
			oldRD := old.(*v1.RadixDeployment)
			if deployment.IsRadixDeploymentInactive(newRD) {
				logger.Debug().Msgf("Skip deployment object %s as it is inactive", newRD.GetName())
				metrics.CustomResourceUpdatedButSkipped(crType)
				return
			}

			if deepEqual(oldRD, newRD) {
				logger.Debug().Msgf("Deployment object is equal to old for %s. Do nothing", newRD.GetName())
				metrics.CustomResourceUpdatedButSkipped(crType)
				return
			}

			if _, err := controller.Enqueue(cur); err != nil {
				logger.Error().Err(err).Msg("Failed to enqueue object received from RadixDeployment informer UpdateFunc")
			}
			metrics.CustomResourceUpdated(crType)
		},
		DeleteFunc: func(obj interface{}) {
			radixDeployment, converted := obj.(*v1.RadixDeployment)
			if !converted {
				logger.Error().Msg("RadixDeployment object cast failed during deleted event received.")
				return
			}
			key, err := cache.MetaNamespaceKeyFunc(radixDeployment)
			if err == nil {
				logger.Debug().Msgf("Deployment object deleted event received for %s. Do nothing", key)
			}
			metrics.CustomResourceDeleted(crType)
		},
	}); err != nil {
		panic(err)
	}

	// Only the service informer works with this, because it makes use of patch
	// if not it will end up in an endless loop (deployment, ingress etc.)
	if _, err := serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			service := obj.(*corev1.Service)
			logger.Debug().Msgf("Service object added event received for %s. Do nothing", service.Name)
		},
		UpdateFunc: func(old, cur interface{}) {
			newService := cur.(*corev1.Service)
			oldService := old.(*corev1.Service)
			if newService.ResourceVersion == oldService.ResourceVersion {
				return
			}
			controller.HandleObject(ctx, cur, v1.KindRadixDeployment, getObject)
		},
		DeleteFunc: func(obj interface{}) {
			controller.HandleObject(ctx, obj, v1.KindRadixDeployment, getObject)
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

			// Trigger sync of active RD, living in the namespaces of the app
			rds, err := radixClient.RadixV1().RadixDeployments(corev1.NamespaceAll).List(
				ctx,
				metav1.ListOptions{
					LabelSelector: fmt.Sprintf("%s=%s", kube.RadixAppLabel, newRr.Name),
				})

			if err == nil && len(rds.Items) > 0 {
				// Will sync the active RD (there can only be one within each namespace)
				for _, rd := range rds.Items {
					if !deployment.IsRadixDeploymentInactive(&rd) {
						obj := &rd
						if _, err := controller.Enqueue(obj); err != nil {
							logger.Error().Str("rd", rd.Name).Err(err).Msg("Failed to enqueue object received from RadixRegistration informer UpdateFunc")
						}
					}
				}
			}
		},
	}); err != nil {
		panic(err)
	}

	return controller
}

func deepEqual(old, new *v1.RadixDeployment) bool {
	if !reflect.DeepEqual(new.Spec, old.Spec) ||
		!reflect.DeepEqual(new.ObjectMeta.Labels, old.ObjectMeta.Labels) ||
		!reflect.DeepEqual(new.ObjectMeta.Annotations, old.ObjectMeta.Annotations) {
		return false
	}

	return true
}

func getObject(ctx context.Context, radixClient radixclient.Interface, namespace, name string) (interface{}, error) {
	return radixClient.RadixV1().RadixDeployments(namespace).Get(ctx, name, metav1.GetOptions{})
}

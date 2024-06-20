package registration

import (
	"context"
	"reflect"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/metrics"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

const (
	controllerAgentName = "registration-controller"
	crType              = "RadixRegistrations"
)

// NewController creates a new controller that handles RadixRegistrations
func NewController(ctx context.Context, client kubernetes.Interface,
	radixClient radixclient.Interface, handler common.Handler,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	radixInformerFactory informers.SharedInformerFactory,
	waitForChildrenToSync bool,
	recorder record.EventRecorder) *common.Controller {
	logger := log.With().Str("controller", controllerAgentName).Logger()
	registrationInformer := radixInformerFactory.Radix().V1().RadixRegistrations()
	controller := &common.Controller{
		Name:                  controllerAgentName,
		HandlerOf:             crType,
		KubeClient:            client,
		RadixClient:           radixClient,
		Informer:              registrationInformer.Informer(),
		KubeInformerFactory:   kubeInformerFactory,
		WorkQueue:             common.NewRateLimitedWorkQueue(ctx, crType),
		Handler:               handler,
		WaitForChildrenToSync: waitForChildrenToSync,
		Recorder:              recorder,
		LockKeyAndIdentifier:  common.NamePartitionKey,
	}

	logger.Info().Msg("Setting up event handlers")

	if _, err := registrationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			if _, err := controller.Enqueue(cur); err != nil {
				logger.Error().Err(err).Msg("Failed to enqueue object received from RadixRegistration informer AddFunc")
			}
			metrics.CustomResourceAdded(crType)
		},
		UpdateFunc: func(old, cur interface{}) {
			newRR := cur.(*v1.RadixRegistration)
			oldRR := old.(*v1.RadixRegistration)

			if deepEqual(oldRR, newRR) {
				logger.Debug().Msgf("Registration object is equal to old for %s. Do nothing", newRR.GetName())
				metrics.CustomResourceUpdatedButSkipped(crType)
				return
			}

			if _, err := controller.Enqueue(cur); err != nil {
				logger.Error().Err(err).Msg("Failed to enqueue object received from RadixRegistration informer UpdateFunc")
			}
			metrics.CustomResourceUpdated(crType)
		},
		DeleteFunc: func(obj interface{}) {
			radixRegistration, converted := obj.(*v1.RadixRegistration)
			if !converted || radixRegistration == nil {
				logger.Error().Msg("v1.RadixRegistration object cast failed during deleted event received.")
				return
			}
			key, err := cache.MetaNamespaceKeyFunc(radixRegistration)
			if err == nil {
				logger.Debug().Msgf("Registration object deleted event received for %s. Do nothing", key)
			}
			metrics.CustomResourceDeleted(crType)
		},
	}); err != nil {
		panic(err)
	}

	namespaceInformer := kubeInformerFactory.Core().V1().Namespaces()
	if _, err := namespaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) {
			controller.HandleObject(ctx, obj, v1.KindRadixRegistration, getObject)
		},
	}); err != nil {
		panic(err)
	}

	secretInformer := kubeInformerFactory.Core().V1().Secrets()
	if _, err := secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldSecret := oldObj.(*corev1.Secret)
			newSecret := newObj.(*corev1.Secret)
			namespace, err := client.CoreV1().Namespaces().Get(ctx, oldSecret.Namespace, metav1.GetOptions{})
			if err != nil {
				logger.Error().Err(err).Msg("Failed to get namespace")
				return
			}
			if oldSecret.ResourceVersion == newSecret.ResourceVersion {
				// Periodic resync will send update events for all known Secrets.
				// Two different versions of the same Secret will always have different RVs.
				return
			}
			if isGitDeployKey(newSecret) && newSecret.Namespace != "" {
				// Resync, as deploy key is updated. Resync is triggered on namespace, since RR not directly own the
				// secret
				controller.HandleObject(ctx, namespace, v1.KindRadixRegistration, getObject)
			}
		},
		DeleteFunc: func(obj interface{}) {
			secret, converted := obj.(*corev1.Secret)
			if !converted {
				logger.Error().Msg("corev1.Secret object cast failed during deleted event received.")
				return
			}
			namespace, err := client.CoreV1().Namespaces().Get(ctx, secret.Namespace, metav1.GetOptions{})
			if err != nil {
				// Ignore error if namespace does not exist.
				// This is normal when a RR is deleted, resulting in deletion of namespaces and it's secrets
				if errors.IsNotFound(err) {
					return
				}
				logger.Error().Err(err).Msg("Failed to get namespace")
				return
			}
			if isGitDeployKey(secret) && namespace.Labels[kube.RadixAppLabel] != "" {
				// Resync, as deploy key is deleted. Resync is triggered on namespace, since RR not directly own the
				// secret
				controller.HandleObject(ctx, namespace, v1.KindRadixRegistration, getObject)
			}
		},
	}); err != nil {
		panic(err)
	}

	return controller
}

func isGitDeployKey(secret *corev1.Secret) bool {
	return secret.Name == defaults.GitPrivateKeySecretName
}

func deepEqual(old, new *v1.RadixRegistration) bool {
	if !reflect.DeepEqual(new.Spec, old.Spec) ||
		!reflect.DeepEqual(new.ObjectMeta.Labels, old.ObjectMeta.Labels) ||
		!reflect.DeepEqual(new.ObjectMeta.Annotations, old.ObjectMeta.Annotations) {
		return false
	}

	return true
}

func getObject(ctx context.Context, radixClient radixclient.Interface, namespace, name string) (interface{}, error) {
	return radixClient.RadixV1().RadixRegistrations().Get(ctx, name, metav1.GetOptions{})
}

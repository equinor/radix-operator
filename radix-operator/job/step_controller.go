package job

import (
	"context"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/rs/zerolog/log"
	eventsv1 "k8s.io/api/events/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

const (
	stepControllerAgentName = "job-step-controller"
	stepCrType              = "Events"
)

// NewStepsController creates a new controller that handles RadixJobs
func NewStepsController(ctx context.Context, client kubernetes.Interface, radixClient radixclient.Interface, handler StepHandler, kubeInformerFactory kubeinformers.SharedInformerFactory, radixInformerFactory informers.SharedInformerFactory, recorder record.EventRecorder) *common.Controller {
	logger := log.With().Str("controller", stepControllerAgentName).Logger()
	kubernetesEventsInformer := kubeInformerFactory.Events().V1().Events()

	controller := &common.Controller{
		Name:                 stepControllerAgentName,
		HandlerOf:            stepCrType,
		KubeClient:           client,
		RadixClient:          radixClient,
		KubeInformerFactory:  kubeInformerFactory,
		RadixInformerFactory: radixInformerFactory,
		WorkQueue:            common.NewRateLimitedWorkQueue(ctx, stepCrType),
		Handler:              handler,
		Recorder:             recorder,
		LockKeyAndIdentifier: common.NamespacePartitionKey,
	}

	logger.Info().Msg("Setting up event handlers")
	informer := kubernetesEventsInformer.Informer()
	if _, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			event, err := cur.(*eventsv1.Event)
			if !err {
				return // The event is not an event
			}
			if event.Regarding.Kind == radixv1.KindRadixJob {
				if _, err := controller.Enqueue(cur); err != nil {
					logger.Error().Err(err).Msg("Failed to enqueue object received from Events informer AddFunc")
				}
			}
		},
	}); err != nil {
		panic(err)
	}
	return controller
}

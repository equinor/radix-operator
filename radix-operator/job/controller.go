package job

import (
	"context"

	"github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/metrics"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/rs/zerolog/log"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

const (
	controllerAgentName = "job-controller"
	crType              = "RadixJobs"
)

// NewController creates a new controller that handles RadixJobs
func NewController(ctx context.Context, client kubernetes.Interface, radixClient radixclient.Interface, handler common.Handler, kubeInformerFactory kubeinformers.SharedInformerFactory, radixInformerFactory informers.SharedInformerFactory, waitForChildrenToSync bool, recorder record.EventRecorder) *common.Controller {
	logger := log.With().Str("controller", controllerAgentName).Logger()
	jobInformer := radixInformerFactory.Radix().V1().RadixJobs()
	kubernetesJobInformer := kubeInformerFactory.Batch().V1().Jobs()
	podInformer := kubeInformerFactory.Core().V1().Pods()

	controller := &common.Controller{
		Name:                  controllerAgentName,
		HandlerOf:             crType,
		KubeClient:            client,
		RadixClient:           radixClient,
		Informer:              jobInformer.Informer(),
		KubeInformerFactory:   kubeInformerFactory,
		WorkQueue:             common.NewRateLimitedWorkQueue(ctx, crType),
		Handler:               handler,
		WaitForChildrenToSync: waitForChildrenToSync,
		Recorder:              recorder,
		LockKeyAndIdentifier:  common.NamespacePartitionKey,
	}

	logger.Info().Msg("Setting up event handlers")
	if _, err := jobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			radixJob, _ := cur.(*v1.RadixJob)
			if job.IsRadixJobDone(radixJob) {
				logger.Debug().Msgf("Skip job object %s as it is complete", radixJob.GetName())
				metrics.CustomResourceAddedButSkipped(crType)
				metrics.InitiateRadixJobStatusChanged(radixJob)
				return
			}

			if _, err := controller.Enqueue(cur); err != nil {
				logger.Error().Err(err).Msg("Failed to enqueue object received from RadixJob informer AddFunc")
			}
			metrics.CustomResourceAdded(crType)
		},
		UpdateFunc: func(old, cur interface{}) {
			newRJ := cur.(*v1.RadixJob)
			if job.IsRadixJobDone(newRJ) {
				logger.Debug().Msgf("Skip job object %s as it is complete", newRJ.GetName())
				metrics.CustomResourceUpdatedButSkipped(crType)
				metrics.InitiateRadixJobStatusChanged(newRJ)
				return
			}

			if _, err := controller.Enqueue(cur); err != nil {
				logger.Error().Err(err).Msg("Failed to enqueue object received from RadixJob informer UpdateFunc")
			}
			metrics.CustomResourceUpdated(crType)
		},
		DeleteFunc: func(obj interface{}) {
			radixJob, converted := obj.(*v1.RadixJob)
			if !converted {
				logger.Error().Msg("RadixJob object cast failed during deleted event received.")
				return
			}
			key, err := cache.MetaNamespaceKeyFunc(radixJob)
			if err == nil {
				logger.Debug().Msgf("Job object deleted event received for %s. Do nothing", key)
			}
			metrics.CustomResourceDeleted(crType)
		},
	}); err != nil {
		panic(err)
	}

	if _, err := kubernetesJobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, cur interface{}) {
			newJob := cur.(*batchv1.Job)
			oldJob := old.(*batchv1.Job)
			if newJob.ResourceVersion == oldJob.ResourceVersion {
				return
			}
			controller.HandleObject(ctx, cur, v1.KindRadixJob, getObject)
		},
		DeleteFunc: func(obj interface{}) {
			radixJob, converted := obj.(*batchv1.Job)
			if !converted {
				logger.Error().Msg("RadixJob object cast failed during deleted event received.")
				return
			}
			// If a kubernetes job gets deleted for a running job, the running radix job should
			// take this into account. The running job will get restarted
			controller.HandleObject(ctx, radixJob, v1.KindRadixJob, getObject)
		},
	}); err != nil {
		panic(err)
	}

	if _, err := podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, cur interface{}) {
			newPod := cur.(*corev1.Pod)
			oldPod := old.(*corev1.Pod)
			if newPod.ResourceVersion == oldPod.ResourceVersion {
				return
			}

			if ownerRef := metav1.GetControllerOf(newPod); ownerRef != nil {
				if ownerRef.Kind != "Job" || newPod.Labels[kube.RadixJobNameLabel] == "" {
					return
				}

				job, err := client.BatchV1().Jobs(newPod.Namespace).Get(ctx, newPod.Labels[kube.RadixJobNameLabel], metav1.GetOptions{})
				if err != nil {
					// This job may not be found because application is being deleted and resources are being deleted
					logger.Debug().Msgf("Could not find owning job of pod %s due to %v", newPod.Name, err)
					return
				}

				controller.HandleObject(ctx, job, v1.KindRadixJob, getObject)
			}
		},
	}); err != nil {
		panic(err)
	}

	return controller
}

func getObject(ctx context.Context, radixClient radixclient.Interface, namespace, name string) (interface{}, error) {
	return radixClient.RadixV1().RadixJobs(namespace).Get(ctx, name, metav1.GetOptions{})
}

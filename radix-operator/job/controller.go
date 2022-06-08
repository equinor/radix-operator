package job

import (
	"context"

	"github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/equinor/radix-operator/pkg/apis/kube"

	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	kubeinformers "k8s.io/client-go/informers"

	"github.com/equinor/radix-operator/pkg/apis/metrics"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/radix-operator/common"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

var logger *log.Entry

const (
	controllerAgentName = "job-controller"
	crType              = "RadixJobs"
)

func init() {
	logger = log.WithFields(log.Fields{"radixOperatorComponent": controllerAgentName})
}

// NewController creates a new controller that handles RadixDeployments
func NewController(client kubernetes.Interface,
	radixClient radixclient.Interface, handler common.Handler,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	radixInformerFactory informers.SharedInformerFactory,
	waitForChildrenToSync bool,
	recorder record.EventRecorder) *common.Controller {

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
		WorkQueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), crType),
		Handler:               handler,
		Log:                   logger,
		WaitForChildrenToSync: waitForChildrenToSync,
		Recorder:              recorder,
		LockKeyAndIdentifier:  common.NamespacePartitionKey,
	}

	logger.Info("Setting up event handlers")
	jobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			radixJob, _ := cur.(*v1.RadixJob)
			if job.IsRadixJobDone(radixJob) {
				logger.Debugf("Skip job object %s as it is complete", radixJob.GetName())
				metrics.CustomResourceAddedButSkipped(crType)
				metrics.InitiateRadixJobStatusChanged(radixJob)
				return
			}

			controller.Enqueue(cur)
			metrics.CustomResourceAdded(crType)
		},
		UpdateFunc: func(old, cur interface{}) {
			newRJ := cur.(*v1.RadixJob)
			if job.IsRadixJobDone(newRJ) {
				logger.Debugf("Skip job object %s as it is complete", newRJ.GetName())
				metrics.CustomResourceUpdatedButSkipped(crType)
				metrics.InitiateRadixJobStatusChanged(newRJ)
				return
			}

			controller.Enqueue(cur)
			metrics.CustomResourceUpdated(crType)
		},
		DeleteFunc: func(obj interface{}) {
			radixJob, _ := obj.(*v1.RadixJob)
			key, err := cache.MetaNamespaceKeyFunc(radixJob)
			if err == nil {
				logger.Debugf("Job object deleted event received for %s. Do nothing", key)
			}
			metrics.CustomResourceDeleted(crType)
		},
	})

	kubernetesJobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, cur interface{}) {
			newJob := cur.(*batchv1.Job)
			oldJob := old.(*batchv1.Job)
			if newJob.ResourceVersion == oldJob.ResourceVersion {
				return
			}
			controller.HandleObject(cur, "RadixJob", getObject)
		},
		DeleteFunc: func(obj interface{}) {
			job := obj.(*batchv1.Job)
			// If a kubernetes job gets deleted for a running job, the running radix job should
			// take this into account. The running job will get restarted
			controller.HandleObject(job, "RadixJob", getObject)
		},
	})

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
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

				job, err := client.BatchV1().Jobs(newPod.Namespace).Get(context.TODO(), newPod.Labels[kube.RadixJobNameLabel], metav1.GetOptions{})
				if err != nil {
					// This job may not be found because application is being deleted and resources are being deleted
					logger.Debugf("Could not find owning job of pod %s due to %v", newPod.Name, err)
					return
				}

				controller.HandleObject(job, "RadixJob", getObject)
			}
		},
	})

	return controller
}

func getObject(radixClient radixclient.Interface, namespace, name string) (interface{}, error) {
	return radixClient.RadixV1().RadixJobs(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

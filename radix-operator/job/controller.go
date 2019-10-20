package job

import (
	"errors"
	"fmt"
	"reflect"

	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	kubeinformers "k8s.io/client-go/informers"

	"github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/equinor/radix-operator/radix-operator/metrics"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

// Controller Instance variables
type Controller struct {
	clientset   kubernetes.Interface
	radixclient radixclient.Interface
	queue       workqueue.RateLimitingInterface
	informer    cache.SharedIndexInformer
	handler     common.Handler
}

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
	kubeutil *kube.Kube,
	radixClient radixclient.Interface, handler common.Handler,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	radixInformerFactory informers.SharedInformerFactory,
	recorder record.EventRecorder) *common.Controller {

	jobInformer := radixInformerFactory.Radix().V1().RadixJobs()
	kubernetesJobInformer := kubeInformerFactory.Batch().V1().Jobs()
	podInformer := kubeInformerFactory.Core().V1().Pods()

	controller := &common.Controller{
		Name:                controllerAgentName,
		HandlerOf:           crType,
		KubeClient:          client,
		RadixClient:         radixClient,
		Informer:            jobInformer.Informer(),
		KubeInformerFactory: kubeInformerFactory,
		WorkQueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), crType),
		Handler:             handler,
		Log:                 logger,
		Recorder:            recorder,
	}

	logger.Info("Setting up event handlers")
	jobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			radixJob, _ := cur.(*v1.RadixJob)
			if job.IsRadixJobDone(radixJob) {
				logger.Debugf("Skip job object %s as it is complete", radixJob.GetName())
				metrics.CustomResourceAddedButSkipped(crType)
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

				job, err := client.BatchV1().Jobs(newPod.Namespace).Get(newPod.Labels[kube.RadixJobNameLabel], metav1.GetOptions{})
				if err != nil {
					// Nothing more to do than escape
					logger.Errorf("Could not find owning job of pod %s due to %v", newPod.Name, err)
					return
				}

				controller.HandleObject(job, "RadixJob", getObject)
			}
		},
	})

	return controller
}

func deepEqual(old, new *v1.RadixJob) bool {
	if !reflect.DeepEqual(new.Spec, old.Spec) ||
		!reflect.DeepEqual(new.ObjectMeta.Labels, old.ObjectMeta.Labels) ||
		!reflect.DeepEqual(new.ObjectMeta.Annotations, old.ObjectMeta.Annotations) {
		return false
	}

	return true
}

func getObject(radixClient radixclient.Interface, namespace, name string) (interface{}, error) {
	rj, err := radixClient.RadixV1().RadixJobs(namespace).Get(name, metav1.GetOptions{})
	if job.IsRadixJobDone(rj) {
		errorMessage := fmt.Sprintf("Ignoring RadixJob %s/%s as it's done", rj.GetNamespace(), rj.GetName())
		logger.Info(errorMessage)
		return nil, errors.New(errorMessage)
	}

	return rj, err
}

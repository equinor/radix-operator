package job

import (
	"errors"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixinformer "github.com/equinor/radix-operator/pkg/client/informers/externalversions/radix/v1"
	"github.com/equinor/radix-operator/radix-operator/common"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	batchinformers "k8s.io/client-go/informers/batch/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
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
	radixClient radixclient.Interface, handler common.Handler,
	jobInformer radixinformer.RadixJobInformer,
	kubernetesJobInformer batchinformers.JobInformer,
	podInformer coreinformers.PodInformer,
	recorder record.EventRecorder) *common.Controller {

	controller := &common.Controller{
		Name:        controllerAgentName,
		KubeClient:  client,
		RadixClient: radixClient,
		Informer:    jobInformer.Informer(),
		WorkQueue:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), crType),
		Handler:     handler,
		Log:         logger,
		Recorder:    recorder,
	}

	logger.Info("Setting up event handlers")
	jobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(new interface{}) {
			radixJob, _ := new.(*v1.RadixJob)
			if job.IsRadixJobDone(radixJob) {
				logger.Infof("#########Skip RJ: %s", radixJob.GetName())
				return
			}

			controller.Enqueue(new)
			controller.CustomResourceAdded(crType)
		},
		UpdateFunc: func(old, new interface{}) {
			radixJob, _ := new.(*v1.RadixJob)
			if job.IsRadixJobDone(radixJob) {
				logger.Infof("#########Skip RJ: %s", radixJob.GetName())
				return
			}

			controller.Enqueue(new)
		},
		DeleteFunc: func(obj interface{}) {
			radixJob, _ := obj.(*v1.RadixJob)
			key, err := cache.MetaNamespaceKeyFunc(radixJob)
			if err == nil {
				logger.Debugf("Job object deleted event received for %s. Do nothing", key)
			}
			controller.CustomResourceDeleted(crType)
		},
	})

	kubernetesJobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, new interface{}) {
			newJob := new.(*batchv1.Job)
			oldJob := old.(*batchv1.Job)
			if newJob.ResourceVersion == oldJob.ResourceVersion {
				return
			}
			controller.HandleObject(new, "RadixJob", getObject)
		},
	})

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, new interface{}) {
			newPod := new.(*corev1.Pod)
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

func getObject(radixClient radixclient.Interface, namespace, name string) (interface{}, error) {
	rj, err := radixClient.RadixV1().RadixJobs(namespace).Get(name, metav1.GetOptions{})
	if job.IsRadixJobDone(rj) {
		errorMessage := fmt.Sprintf("Ignoring RadixJob %s/%s as it's done", rj.GetNamespace(), rj.GetName())
		logger.Info(errorMessage)
		return nil, errors.New(errorMessage)
	}

	return rj, err
}

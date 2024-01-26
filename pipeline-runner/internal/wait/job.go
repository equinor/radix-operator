package wait

import (
	"errors"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	logger "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const jobNameLabel = "job-name"

var imageErrors = map[string]bool{"ImagePullBackOff": true, "ImageInspectError": true, "ErrImagePull": true,
	"ErrImageNeverPull": true, "RegistryUnavailable": true, "InvalidImageName": true}

type JobCompletionWaiter interface {
	Wait(job *batchv1.Job) error
}

func NewJobCompletionWaiter(kubeClient kubernetes.Interface) JobCompletionWaiter {
	return JobCompletionWaiterFunc(func(job *batchv1.Job) error {
		return waitForCompletionOf(kubeClient, job)
	})
}

type JobCompletionWaiterFunc func(job *batchv1.Job) error

func (f JobCompletionWaiterFunc) Wait(job *batchv1.Job) error {
	return f(job)
}

// WaitForCompletionOf Will wait for job to complete
func waitForCompletionOf(kubeClient kubernetes.Interface, job *batchv1.Job) error {
	errChan := make(chan error)
	stop := make(chan struct{})
	defer close(stop)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(
		kubeClient, 0, kubeinformers.WithNamespace(job.GetNamespace()))
	jobsInformer := kubeInformerFactory.Batch().V1().Jobs().Informer()

	_, _ = jobsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, cur interface{}) {
			currJob, success := cur.(*batchv1.Job)
			if success && currJob.GetName() == job.GetName() && currJob.GetNamespace() == job.GetNamespace() {
				switch {
				case currJob.Status.Succeeded == 1:
					errChan <- nil
				case currJob.Status.Failed == 1:
					errChan <- fmt.Errorf("job failed. See log of the %s job %s for more details", currJob.GetLabels()[kube.RadixJobTypeLabel], currJob.GetName())
				}
			}
		},
		DeleteFunc: func(old interface{}) {
			currJob, converted := old.(*batchv1.Job)
			if !converted {
				logger.Errorf("Job object cast failed during deleted event received.")
				return
			}
			if currJob.GetName() == job.GetName() && currJob.GetNamespace() == job.GetNamespace() {
				errChan <- errors.New("job failed - Job deleted")
			}
		},
	})

	podsInformer := kubeInformerFactory.Core().V1().Pods().Informer()
	_, _ = podsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, cur interface{}) {
			pod, success := cur.(*corev1.Pod)
			if success && job.GetNamespace() == pod.GetNamespace() &&
				pod.ObjectMeta.Labels[jobNameLabel] == job.GetName() &&
				len(pod.Status.ContainerStatuses) > 0 {
				err := checkPodIsTerminatedOrFailed(&pod.Status.ContainerStatuses)
				if err != nil {
					errChan <- err
				}
			}
		},
		DeleteFunc: func(old interface{}) {
			pod, converted := old.(*corev1.Pod)
			if !converted {
				logger.Errorf("Pod object cast failed during deleted event received.")
				return
			}
			if job.GetNamespace() == pod.GetNamespace() && pod.ObjectMeta.Labels[jobNameLabel] == job.GetName() {
				errChan <- fmt.Errorf("job's pod deleted")
			}
		},
	})

	go jobsInformer.Run(stop)
	if !cache.WaitForCacheSync(stop, jobsInformer.HasSynced) {
		errChan <- fmt.Errorf("timed out waiting for caches to sync")
	} else {
		go podsInformer.Run(stop)
		if !cache.WaitForCacheSync(stop, podsInformer.HasSynced) {
			errChan <- fmt.Errorf("timed out waiting for caches to sync")
		}
	}

	err := <-errChan
	return err
}

func checkPodIsTerminatedOrFailed(containerStatuses *[]corev1.ContainerStatus) error {
	for _, containerStatus := range *containerStatuses {
		if containerStatus.State.Terminated != nil {
			terminated := containerStatus.State.Terminated
			if terminated.Reason == "Failed" {
				return fmt.Errorf("job's pod failed: %s", terminated.Message)
			} else {
				return nil
			}
		}
		if containerStatus.State.Waiting != nil {
			if _, ok := imageErrors[containerStatus.State.Waiting.Reason]; ok {
				return fmt.Errorf("job's pod failed: %s", containerStatus.State.Waiting.Message)
			}
		}
		if containerStatus.LastTerminationState.Terminated != nil {
			return fmt.Errorf("job's pod failed: %s",
				containerStatus.LastTerminationState.Terminated.Message)
		}
	}
	return nil
}

package wait

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/rs/zerolog/log"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type JobCompletionWaiter interface {
	Wait(job *batchv1.Job) error
}

func NewJobCompletionWaiter(ctx context.Context, kubeClient kubernetes.Interface) JobCompletionWaiter {
	return JobCompletionWaiterFunc(func(job *batchv1.Job) error {
		return waitForCompletionOfJob(ctx, kubeClient, job)
	})
}

type JobCompletionWaiterFunc func(job *batchv1.Job) error

func (f JobCompletionWaiterFunc) Wait(job *batchv1.Job) error {
	return f(job)
}

// WaitForCompletionOf Will wait for job to complete
func waitForCompletionOfJob(ctx context.Context, kubeClient kubernetes.Interface, job *batchv1.Job) error {
	errChan := make(chan error)
	stop := make(chan struct{})
	defer close(stop)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(
		kubeClient, 0, kubeinformers.WithNamespace(job.GetNamespace()))
	jobsInformer := kubeInformerFactory.Batch().V1().Jobs().Informer()

	_, _ = jobsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, cur any) {
			currJob, converted := cur.(*batchv1.Job)
			if !converted {
				log.Ctx(ctx).Error().Msg("Job object cast failed during update event.")
				return
			}
			if currJob.GetName() == job.GetName() && currJob.GetNamespace() == job.GetNamespace() {
				switch {
				case currJob.Status.Succeeded == 1:
					errChan <- nil
				case currJob.Status.Failed == 1:
					errChan <- fmt.Errorf("job failed. See more details in the log of the job %s", getJobDescription(currJob))
				}
			}
		},
		DeleteFunc: func(old any) {
			currJob, converted := old.(*batchv1.Job)
			if !converted {
				log.Ctx(ctx).Error().Msg("Job object cast failed during deleted event.")
				return
			}
			if currJob.GetName() == job.GetName() && currJob.GetNamespace() == job.GetNamespace() {
				errChan <- errors.New("job failed - Job deleted")
			}
		},
	})

	podsInformer := kubeInformerFactory.Core().V1().Pods().Informer()
	_, _ = podsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, cur any) {
			pod, converted := cur.(*corev1.Pod)
			if !converted {
				log.Ctx(ctx).Error().Msg("Pod object cast failed during update event.")
				return
			}

			if metav1.IsControlledBy(pod, job) {
				if err := checkPodErrors(pod); err != nil {
					errChan <- err
				}
			}
		},
		DeleteFunc: func(old any) {
			pod, converted := old.(*corev1.Pod)
			if !converted {
				log.Ctx(ctx).Error().Msg("Pod object cast failed during deleted event.")
				return
			}
			if metav1.IsControlledBy(pod, job) {
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

func getJobDescription(job *batchv1.Job) string {
	var lines []string
	if jobType, ok := job.GetLabels()[kube.RadixJobTypeLabel]; ok {
		lines = append(lines, jobType)
	}
	if componentName, ok := job.GetLabels()[kube.RadixComponentLabel]; ok {
		lines = append(lines, fmt.Sprintf("for the component %s", componentName))
	}
	if envName, ok := job.GetLabels()[kube.RadixEnvLabel]; ok {
		lines = append(lines, fmt.Sprintf("for the environment %s", envName))
	}
	return strings.Join(lines, " ")
}

var waitingReasonPermanentError = map[string]bool{"ImageInspectError": true, "ErrImageNeverPull": true, "InvalidImageName": true}
var waitingReasonTransientError = map[string]bool{"ImagePullBackOff": true, "ErrImagePull": true, "RegistryUnavailable": true}
var transientErrorTimeout = time.Duration(1) * time.Minute

func checkPodErrors(pod *corev1.Pod) error {
	if err := checkPodContainerStatusWaitingReasonErrors(pod.Status.InitContainerStatuses, waitingReasonPermanentError); err != nil {
		return err
	}
	if err := checkPodContainerStatusWaitingReasonErrors(pod.Status.ContainerStatuses, waitingReasonPermanentError); err != nil {
		return err
	}

	if time.Since(pod.CreationTimestamp.Time) > transientErrorTimeout {
		if err := checkPodContainerStatusWaitingReasonErrors(pod.Status.InitContainerStatuses, waitingReasonTransientError); err != nil {
			return err
		}
		if err := checkPodContainerStatusWaitingReasonErrors(pod.Status.ContainerStatuses, waitingReasonTransientError); err != nil {
			return err
		}
	}

	return nil
}

func checkPodContainerStatusWaitingReasonErrors(containerStatuses []corev1.ContainerStatus, reasons map[string]bool) error {
	for _, containerStatus := range containerStatuses {
		if containerStatus.State.Waiting != nil {
			if _, ok := reasons[containerStatus.State.Waiting.Reason]; ok {
				return fmt.Errorf("container %s failed with reason %q, message %q", containerStatus.Name, containerStatus.State.Waiting.Reason, containerStatus.State.Waiting.Message)
			}
		}
	}

	return nil
}

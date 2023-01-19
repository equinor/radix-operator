package kube

import (
	"context"
	"errors"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

const jobNameLabel = "job-name"

var imageErrors = map[string]bool{"ImagePullBackOff": true, "ImageInspectError": true, "ErrImagePull": true,
	"ErrImageNeverPull": true, "RegistryUnavailable": true, "InvalidImageName": true}

// WaitForCompletionOf Will wait for job to complete
func (kubeutil *Kube) WaitForCompletionOf(job *batchv1.Job) error {
	errChan := make(chan error)
	stop := make(chan struct{})
	defer close(stop)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(
		kubeutil.kubeClient, 0, kubeinformers.WithNamespace(job.GetNamespace()))
	jobsInformer := kubeInformerFactory.Batch().V1().Jobs().Informer()

	jobsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, cur interface{}) {
			currJob, success := cur.(*batchv1.Job)
			if success && currJob.GetName() == job.GetName() && currJob.GetNamespace() == job.GetNamespace() {
				switch {
				case currJob.Status.Succeeded == 1:
					errChan <- nil
				case currJob.Status.Failed == 1:
					errChan <- fmt.Errorf("job failed. See log for more details")
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
	podsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
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

// ListJobs Lists jobs from cache or from cluster
func (kubeutil *Kube) ListJobs(namespace string) ([]*batchv1.Job, error) {
	if kubeutil.JobLister != nil {
		jobs, err := kubeutil.JobLister.Jobs(namespace).List(labels.NewSelector())
		if err != nil {
			return nil, err
		}
		return jobs, nil
	} else {
		list, err := kubeutil.kubeClient.BatchV1().Jobs(namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		jobs := slice.PointersOf(list.Items).([]*batchv1.Job)
		return jobs, nil
	}
}

// ListJobsWithSelector List jobs with selector
func (kubeutil *Kube) ListJobsWithSelector(namespace, labelSelectorString string) ([]*batchv1.Job, error) {
	if kubeutil.JobLister != nil {
		selector, err := labels.Parse(labelSelectorString)
		if err != nil {
			return nil, err
		}
		return kubeutil.JobLister.Jobs(namespace).List(selector)
	}

	list, err := kubeutil.kubeClient.BatchV1().Jobs(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelectorString})
	if err != nil {
		return nil, err
	}

	return slice.PointersOf(list.Items).([]*batchv1.Job), nil

}

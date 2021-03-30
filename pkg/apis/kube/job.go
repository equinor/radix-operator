package kube

import (
	"errors"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

// WaitForCompletionOf Will wait for job to complete
func (kubeutil *Kube) WaitForCompletionOf(job *batchv1.Job) error {
	errChan := make(chan error)
	stop := make(chan struct{})
	defer close(stop)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(
		kubeutil.kubeClient, 0, kubeinformers.WithNamespace(job.GetNamespace()))
	informer := kubeInformerFactory.Batch().V1().Jobs().Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, cur interface{}) {
			j, success := cur.(*batchv1.Job)
			if success && job.GetName() == j.GetName() && job.GetNamespace() == j.GetNamespace() {
				switch {
				case j.Status.Succeeded == 1:
					errChan <- nil
				case j.Status.Failed == 1:
					errChan <- fmt.Errorf("Job failed. See log for more details")
				default:
					log.Debugf("Ongoing - job has not completed yet")
				}
			}
		},
		DeleteFunc: func(old interface{}) {
			j, success := old.(*batchv1.Job)
			if success && j.GetName() == job.GetName() && job.GetNamespace() == j.GetNamespace() {
				errChan <- errors.New("Job failed - Job deleted")
			}
		},
	})

	go informer.Run(stop)
	if !cache.WaitForCacheSync(stop, informer.HasSynced) {
		errChan <- fmt.Errorf("Timed out waiting for caches to sync")
	}

	err := <-errChan
	return err
}

// ListJobs Lists jobs from cache or from cluster
func (k *Kube) ListJobs(namespace string) ([]*batchv1.Job, error) {
	if k.JobLister != nil {
		jobs, err := k.JobLister.Jobs(namespace).List(labels.NewSelector())
		if err != nil {
			return nil, err
		}
		return jobs, nil
	} else {
		list, err := k.kubeClient.BatchV1().Jobs(namespace).List(metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		jobs := slice.PointersOf(list.Items).([]*batchv1.Job)
		return jobs, nil
	}
}

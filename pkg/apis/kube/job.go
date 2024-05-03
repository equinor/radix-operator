package kube

import (
	"context"

	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// ListJobs Lists jobs from cache or from cluster
func (kubeutil *Kube) ListJobs(ctx context.Context, namespace string) ([]*batchv1.Job, error) {
	if kubeutil.JobLister != nil {
		jobs, err := kubeutil.JobLister.Jobs(namespace).List(labels.NewSelector())
		if err != nil {
			return nil, err
		}
		return jobs, nil
	} else {
		list, err := kubeutil.kubeClient.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		jobs := slice.PointersOf(list.Items).([]*batchv1.Job)
		return jobs, nil
	}
}

// ListJobsWithSelector List jobs with selector
func (kubeutil *Kube) ListJobsWithSelector(ctx context.Context, namespace, labelSelectorString string) ([]*batchv1.Job, error) {
	if kubeutil.JobLister != nil {
		selector, err := labels.Parse(labelSelectorString)
		if err != nil {
			return nil, err
		}
		return kubeutil.JobLister.Jobs(namespace).List(selector)
	}

	list, err := kubeutil.kubeClient.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelectorString})
	if err != nil {
		return nil, err
	}

	return slice.PointersOf(list.Items).([]*batchv1.Job), nil

}

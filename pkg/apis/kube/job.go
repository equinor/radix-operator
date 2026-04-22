package kube

import (
	"context"

	commonslice "github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// IsJobSucceeded returns true if the job has a Complete or SuccessCriteriaMet condition with status True.
func IsJobSucceeded(jobStatus batchv1.JobStatus) bool {
	_, ok := commonslice.FindFirst(jobStatus.Conditions, JobHasOneOfConditionTypes(batchv1.JobComplete, batchv1.JobSuccessCriteriaMet))
	return ok
}

// IsJobFailed returns true if the job has a Failed condition with status True.
func IsJobFailed(jobStatus batchv1.JobStatus) bool {
	_, ok := commonslice.FindFirst(jobStatus.Conditions, JobHasOneOfConditionTypes(batchv1.JobFailed))
	return ok
}

// IsJobRunning returns true if the job has active pods.
func IsJobRunning(jobStatus batchv1.JobStatus) bool {
	return jobStatus.Active > 0
}

// JobHasOneOfConditionTypes returns a predicate that checks if a job condition matches any of the given types with status True.
func JobHasOneOfConditionTypes(conditionTypes ...batchv1.JobConditionType) func(batchv1.JobCondition) bool {
	return func(condition batchv1.JobCondition) bool {
		return commonslice.Any(conditionTypes, func(c batchv1.JobConditionType) bool {
			return condition.Type == c && condition.Status == corev1.ConditionTrue
		})
	}
}


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

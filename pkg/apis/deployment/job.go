package deployment

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (deploy *Deployment) garbageCollectScheduledJobsNoLongerInSpec() error {
	jobs, err := deploy.kubeutil.ListJobs(deploy.radixDeployment.GetNamespace())
	if err != nil {
		return err
	}

	for _, job := range jobs {
		componentName, ok := RadixComponentNameFromComponentLabel(job)
		if !ok {
			continue
		}

		jobType, ok := NewRadixJobTypeFromObjectLabels(job)
		if !ok || !jobType.IsJobScheduler() {
			continue
		}

		if !componentName.ExistInDeploymentSpecJobList(deploy.radixDeployment) {
			propagationPolicy := metav1.DeletePropagationBackground
			err = deploy.kubeclient.BatchV1().Jobs(deploy.radixDeployment.GetNamespace()).Delete(
				context.TODO(),
				job.Name,
				metav1.DeleteOptions{
					PropagationPolicy: &propagationPolicy,
				})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

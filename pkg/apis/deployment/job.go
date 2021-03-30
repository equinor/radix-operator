package deployment

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (deploy *Deployment) garbageCollectScheduledJobsNoLongerInSpec() error {
	jobs, err := deploy.kubeutil.ListJobs(deploy.radixDeployment.GetNamespace())

	for _, job := range jobs {
		componentName, ok := NewRadixComponentNameFromLabels(job)
		if !ok {
			continue
		}

		jobType, ok := NewRadixJobTypeFromObjectLabels(job)
		if !ok {
			continue
		}

		// Delete job is it originates from job-scheduler and is no longed defined in RD jobs section
		if jobType.IsJobScheduler() && !componentName.ExistInDeploymentSpecJobList(deploy.radixDeployment) {
			err = deploy.kubeclient.BatchV1().Jobs(deploy.radixDeployment.GetNamespace()).Delete(context.TODO(), job.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

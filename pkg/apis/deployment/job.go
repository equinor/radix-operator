package deployment

import (
	"github.com/equinor/radix-operator/pkg/apis/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (deploy *Deployment) garbageCollectScheduledJobsNoLongerInSpec() error {
	jobs, err := deploy.kubeutil.ListJobs(deploy.radixDeployment.GetNamespace())

	for _, job := range jobs {
		componentName, ok := NewRadixComponentNameFromLabels(job)
		if !ok {
			continue
		}

		// Get value for label radix-job-type. If label doesn't exist, the job is not handled by a job in RD
		jobType, ok := job.GetLabels()[kube.RadixJobTypeLabel]
		if !ok {
			continue
		}

		// If value of radix-job-type equal job-scheduler, it means the job was created from a component in jobs section of RD
		// If value for radix-component-name label does not exist in the jobs section, we can delete the job
		if jobType == kube.RadixJobTypeJobSchedule {
			if !componentName.ExistInDeploymentSpecJobList(deploy.radixDeployment) {
				err = deploy.kubeclient.BatchV1().Jobs(deploy.radixDeployment.GetNamespace()).Delete(job.Name, &metav1.DeleteOptions{})
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

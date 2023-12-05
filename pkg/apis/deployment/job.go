package deployment

import (
	"context"
	stderrors "errors"

	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
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
		if !ok {
			continue
		}

		// Delete job if it originates from job-scheduler and is no longed defined in RD jobs section
		if jobType.IsJobScheduler() && !componentName.ExistInDeploymentSpecJobList(deploy.radixDeployment) {
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

func (deploy *Deployment) garbageCollectScheduledJobAuxDeploymentsNoLongerInSpec() error {
	jobAuxDeployments, err := deploy.kubeutil.ListDeploymentsWithSelector(deploy.radixDeployment.GetNamespace(), labels.ForIsJobAuxObject().String())
	if err != nil {
		return err
	}
	var errs []error
	for _, deployment := range jobAuxDeployments {
		componentName, ok := RadixComponentNameFromComponentLabel(deployment)
		if !ok {
			continue
		}

		// Delete job aux deployment if a job is no longed defined in RD job section
		if !componentName.ExistInDeploymentSpecJobList(deploy.radixDeployment) {
			err = deploy.kubeutil.DeleteDeployment(deploy.radixDeployment.GetNamespace(), deployment.Name)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	return stderrors.Join(errs...)
}

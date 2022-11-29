package job

import (
	"context"
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/utils"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type radixJobsWithRadixDeployments map[string]v1.RadixDeployment
type radixJobsForBranches map[string][]v1.RadixJob
type radixJobsForConditions map[v1.RadixJobCondition]radixJobsForBranches

func (job *Job) maintainHistoryLimit() {
	radixJobs, err := job.getAllRadixJobs()
	if err != nil {
		log.Errorf("failed to get Radix jobs in maintain job history. Error: %v", err)
		return
	}
	if err != nil || len(radixJobs) == 0 {
		return
	}
	rdRadixJobs, err := job.getRadixJobsWithDeployments()
	if err != nil {
		log.Errorf("failed to get Radix jobs with deployments in maintain job history. Error: %v", err)
		return
	}
	deletingJobs, radixJobsForConditions := job.groupSortedRadixJobs(radixJobs, rdRadixJobs)
	jobHistoryLimit := job.config.GetJobsHistoryLimit()
	log.Infof("Delete history jobs for limit %d", jobHistoryLimit)
	jobsByConditionAndBranch := job.getJobsToGarbageCollectByJobConditionAndBranch(radixJobsForConditions, jobHistoryLimit)

	deletingJobs = append(deletingJobs, jobsByConditionAndBranch...)

	if len(deletingJobs) > 0 {
		log.Infof("jobs to delete: %d", len(deletingJobs))
	} else {
		log.Infof("there is no jobs to delete")
	}

	job.garbageCollectJobs(deletingJobs)
}

func (job *Job) garbageCollectJobs(deletingJobs []v1.RadixJob) {
	for _, deletingJob := range deletingJobs {
		log.Infof("Removing job %s from %s", deletingJob.GetName(), deletingJob.GetNamespace())
		err := job.radixclient.RadixV1().RadixJobs(deletingJob.GetNamespace()).Delete(context.TODO(), deletingJob.GetName(), metav1.DeleteOptions{})
		if err != nil {
			log.Errorf("error deleting the RadixJob %s from %s: %v", deletingJob.GetName(), deletingJob.GetNamespace(), err)
		}
	}
}

func (job *Job) groupSortedRadixJobs(radixJobs []v1.RadixJob, rdRadixJobs radixJobsWithRadixDeployments) ([]v1.RadixJob, radixJobsForConditions) {
	var deletingJobs []v1.RadixJob
	radixJobsForConditions := make(radixJobsForConditions)
	for _, rj := range radixJobs {
		rj := rj
		jobCondition := rj.Status.Condition
		switch jobCondition {
		case v1.JobSucceeded:
			if _, ok := rdRadixJobs[rj.GetName()]; !ok {
				log.Debugf("- delete job %s", rj.GetName())
				deletingJobs = append(deletingJobs, rj)
			}
		case v1.JobRunning:
			continue
		default:
			if radixJobsForConditions[jobCondition] == nil {
				radixJobsForConditions[jobCondition] = make(radixJobsForBranches)
			}
			jobBranch := getRadixJobBranch(rj)
			radixJobsForConditions[jobCondition][jobBranch] = append(radixJobsForConditions[jobCondition][jobBranch], rj)
		}
	}
	return sortJobsByActiveFromDesc(deletingJobs), sortRadixJobGroupsByActiveFromDesc(radixJobsForConditions)
}

func sortRadixJobGroupsByActiveFromDesc(radixJobsForConditions radixJobsForConditions) radixJobsForConditions {
	for jobCondition, jobsForBranches := range radixJobsForConditions {
		for jobBranch, jobs := range jobsForBranches {
			radixJobsForConditions[jobCondition][jobBranch] = sortJobsByActiveFromDesc(jobs)
		}
	}
	return radixJobsForConditions
}

func (job *Job) getRadixJobsWithDeployments() (radixJobsWithRadixDeployments, error) {
	appName, ok := job.radixJob.GetLabels()[kube.RadixAppLabel]
	if !ok || len(appName) == 0 {
		return nil, fmt.Errorf("missing label %s in the RadixJob", kube.RadixAppLabel)
	}
	ra, err := job.radixclient.RadixV1().RadixApplications(job.radixJob.Namespace).Get(context.TODO(), appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	rdRadixJobs := make(radixJobsWithRadixDeployments)
	for _, env := range ra.Spec.Environments {
		envNamespace := utils.GetEnvironmentNamespace(appName, env.Name)
		envRdList, err := job.radixclient.RadixV1().RadixDeployments(envNamespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get RadixDeployments from the environment %s. Error: %w", env.Name, err)
		}
		for _, rd := range envRdList.Items {
			rd := rd
			if jobName, ok := rd.GetLabels()[kube.RadixJobNameLabel]; ok {
				rdRadixJobs[jobName] = rd
			}
		}
	}
	return rdRadixJobs, nil
}

func getRadixJobBranch(rj v1.RadixJob) string {
	if branch, ok := rj.GetAnnotations()[kube.RadixBranchAnnotation]; ok && len(branch) > 0 {
		return branch
	}
	if branch, ok := rj.GetLabels()[kube.RadixBuildLabel]; ok && len(branch) > 0 {
		return branch
	}
	return ""
}

func (job *Job) getAllRadixJobs() ([]v1.RadixJob, error) {
	allRJs, err := job.radixclient.RadixV1().RadixJobs(job.radixJob.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get all RadixJobs. Error: %w", err)
	}
	return allRJs.Items, err
}

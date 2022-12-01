package job

import (
	"context"
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

type radixJobsWithRadixDeployments map[string]v1.RadixDeployment
type radixJobsForBranches map[string][]v1.RadixJob
type radixJobsForConditions map[v1.RadixJobCondition]radixJobsForBranches

func (job *Job) maintainHistoryLimit() {
	radixJobs, err := job.getAllRadixJobs()
	if err != nil {
		log.Errorf("failed to get RadixJob in maintain job history. Error: %v", err)
		return
	}
	if err != nil || len(radixJobs) == 0 {
		return
	}
	radixJobsWithRDs, err := job.getRadixJobsWithRadixDeployments()
	if err != nil {
		log.Errorf("failed to get RadixJobs with RadixDeployments in maintain job history. Error: %v", err)
		return
	}
	deletingJobs, radixJobsForConditions := job.groupSortedRadixJobs(radixJobs, radixJobsWithRDs)
	jobHistoryLimit := job.config.PipelineJobsHistoryLimit
	log.Infof("Delete history RadixJob for limit %d", jobHistoryLimit)
	jobsByConditionAndBranch := job.getJobsToGarbageCollectByJobConditionAndBranch(radixJobsForConditions, jobHistoryLimit)

	deletingJobs = append(deletingJobs, jobsByConditionAndBranch...)
	job.garbageCollectRadixJobs(deletingJobs)
}

func (job *Job) garbageCollectRadixJobs(radixJobs []v1.RadixJob) {
	if len(radixJobs) == 0 {
		log.Infof("There is no RadixJobs to delete")
		return
	}
	for _, rj := range radixJobs {
		if strings.EqualFold(rj.GetName(), job.radixJob.GetName()) {
			continue //do not remove current job
		}
		log.Infof("- delete RadixJob %s from %s", rj.GetName(), rj.GetNamespace())
		err := job.radixclient.RadixV1().RadixJobs(rj.GetNamespace()).Delete(context.TODO(), rj.GetName(), metav1.DeleteOptions{})
		if err != nil {
			log.Errorf("error deleting the RadixJob %s from %s: %v", rj.GetName(), rj.GetNamespace(), err)
		}
	}
}

func (job *Job) groupSortedRadixJobs(radixJobs []v1.RadixJob, radixJobsWithRDs radixJobsWithRadixDeployments) ([]v1.RadixJob, radixJobsForConditions) {
	var deletingJobs []v1.RadixJob
	radixJobsForConditions := make(radixJobsForConditions)
	for _, rj := range radixJobs {
		rj := rj
		jobCondition := rj.Status.Condition
		switch jobCondition {
		case v1.JobSucceeded:
			if _, ok := radixJobsWithRDs[rj.GetName()]; !ok {
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
	return sortRadixJobsByCreatedDesc(deletingJobs), sortRadixJobGroupsByCreatedDesc(radixJobsForConditions)
}

func sortRadixJobGroupsByCreatedDesc(radixJobsForConditions radixJobsForConditions) radixJobsForConditions {
	for jobCondition, jobsForBranches := range radixJobsForConditions {
		for jobBranch, jobs := range jobsForBranches {
			radixJobsForConditions[jobCondition][jobBranch] = sortRadixJobsByCreatedDesc(jobs)
		}
	}
	return radixJobsForConditions
}

func (job *Job) getRadixJobsWithRadixDeployments() (radixJobsWithRadixDeployments, error) {
	appName, err := job.getAppName()
	if err != nil {
		return nil, err
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

func (job *Job) getAppName() (string, error) {
	appName, ok := job.radixJob.GetLabels()[kube.RadixAppLabel]
	if !ok || len(appName) == 0 {
		return "", fmt.Errorf("missing label %s in the RadixJob", kube.RadixAppLabel)
	}
	return appName, nil
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
	radixJobList, err := job.radixclient.RadixV1().RadixJobs(job.radixJob.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get all RadixJobs. Error: %w", err)
	}
	return radixJobList.Items, err
}

func (job *Job) getJobsToGarbageCollectByJobConditionAndBranch(jobsForConditions radixJobsForConditions, jobHistoryLimit int) []v1.RadixJob {
	var deletingJobs []v1.RadixJob
	for jobCondition, jobsForBranches := range jobsForConditions {
		for jobBranch, jobs := range jobsForBranches {
			jobs := sortRadixJobsByCreatedDesc(jobs)
			for i := jobHistoryLimit; i < len(jobs); i++ {
				log.Debugf("- collect for deleting RadixJob %s for the branch %s, condition %s", jobs[i].GetName(), jobBranch, jobCondition)
				deletingJobs = append(deletingJobs, jobs[i])
			}
		}
	}
	return deletingJobs
}

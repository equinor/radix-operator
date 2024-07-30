package job

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

// History Interface for job History
type History interface {
	// Cleanup the pipeline job history
	Cleanup(ctx context.Context, appName, radixJobName string)
}

type history struct {
	namespacesRequestsToCleanup sync.Map
	namespacesCleanupInProgress sync.Map
	radixClient                 radixclient.Interface
	historyLimit                int
	kubeUtil                    *kube.Kube
}

// NewHistory Constructor for job History
func NewHistory(radixClient radixclient.Interface, kubeUtil *kube.Kube, historyLimit int) History {
	return &history{
		radixClient:  radixClient,
		historyLimit: historyLimit,
		kubeUtil:     kubeUtil,
	}
}

// Cleanup the pipeline job history
func (h *history) Cleanup(ctx context.Context, appName, radixJobName string) {
	namespace := utils.GetAppNamespace(appName)
	if _, ok := h.namespacesRequestsToCleanup.LoadOrStore(namespace, struct{}{}); ok {
		return // a request to clean up history in this namespace already exists
	}
	if _, ok := h.namespacesCleanupInProgress.LoadOrStore(namespace, struct{}{}); ok {
		return // clean up history in this namespace is already in progress
	}
	go func() {
		defer h.namespacesCleanupInProgress.Delete(namespace)
		for {
			if _, ok := h.namespacesRequestsToCleanup.LoadAndDelete(namespace); !ok {
				return // if there were no more requests to clean up history in this namespace - exit
			}
			h.garbageCollectRadixJobs(ctx, appName, radixJobName)
			h.garbageCollectConfigMaps(ctx, namespace)
		}
	}()
}

type radixJobsWithRadixDeployments map[string]radixv1.RadixDeployment
type radixJobsForBranches map[string][]radixv1.RadixJob
type radixJobsForConditionsMap map[radixv1.RadixJobCondition]radixJobsForBranches

func (h *history) garbageCollectRadixJobs(ctx context.Context, appName string, activeRadixJobName string) {
	namespace := utils.GetAppNamespace(appName)
	radixJobs, err := h.getAllRadixJobs(ctx, namespace)
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Msg("failed to get RadixJob in cleanup job history")
		return
	}
	if len(radixJobs) == 0 {
		return
	}
	radixJobsWithRDs, err := h.getRadixJobsWithRadixDeployments(ctx, appName, namespace)
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Msg("failed to get RadixJobs with RadixDeployments in cleanup job history")
		return
	}
	deletingJobs, radixJobsForConditions := groupSortedRadixJobs(radixJobs, radixJobsWithRDs)
	log.Ctx(ctx).Info().Msgf("Delete history RadixJob for limit %d", h.historyLimit)
	jobsByConditionAndBranch := h.getJobsToGarbageCollectByJobConditionAndBranch(ctx, radixJobsForConditions)

	deletingJobs = append(deletingJobs, jobsByConditionAndBranch...)
	if len(deletingJobs) == 0 {
		log.Ctx(ctx).Info().Msg("There is no RadixJobs to delete")
		return
	}
	for _, radixJob := range deletingJobs {
		if strings.EqualFold(radixJob.GetName(), activeRadixJobName) {
			continue // do not remove active RadixJob
		}
		log.Ctx(ctx).Info().Msgf("Delete RadixJob %s from %s", radixJob.GetName(), namespace)
		if err := h.radixClient.RadixV1().RadixJobs(namespace).Delete(ctx, radixJob.GetName(), metav1.DeleteOptions{}); err != nil {
			log.Ctx(ctx).Warn().Err(err).Msgf("Failed to delete RadixJob %s from %s", radixJob.GetName(), namespace)
		}
	}
}

func groupSortedRadixJobs(radixJobs []radixv1.RadixJob, radixJobsWithRDs radixJobsWithRadixDeployments) ([]radixv1.RadixJob, radixJobsForConditionsMap) {
	var deletingJobs []radixv1.RadixJob
	radixJobsForConditions := make(radixJobsForConditionsMap)
	for _, radixJob := range radixJobs {
		rj := radixJob
		jobCondition := rj.Status.Condition
		switch {
		case jobCondition == radixv1.JobSucceeded && rj.Spec.PipeLineType != radixv1.Build:
			if _, ok := radixJobsWithRDs[rj.GetName()]; !ok {
				deletingJobs = append(deletingJobs, rj)
			}
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

func sortRadixJobGroupsByCreatedDesc(radixJobsForConditions radixJobsForConditionsMap) radixJobsForConditionsMap {
	for jobCondition, jobsForBranches := range radixJobsForConditions {
		for jobBranch, jobs := range jobsForBranches {
			radixJobsForConditions[jobCondition][jobBranch] = sortRadixJobsByCreatedDesc(jobs)
		}
	}
	return radixJobsForConditions
}

func (h *history) getRadixJobsWithRadixDeployments(ctx context.Context, appName, namespace string) (radixJobsWithRadixDeployments, error) {
	ra, err := h.radixClient.RadixV1().RadixApplications(namespace).Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		// RadixApplication may not exist if this is the first job for a new application
		if k8errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	rdRadixJobs := make(radixJobsWithRadixDeployments)
	for _, env := range ra.Spec.Environments {
		envNamespace := utils.GetEnvironmentNamespace(appName, env.Name)
		envRdList, err := h.radixClient.RadixV1().RadixDeployments(envNamespace).List(ctx, metav1.ListOptions{})
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

func getRadixJobBranch(rj radixv1.RadixJob) string {
	if branch, ok := rj.GetAnnotations()[kube.RadixBranchAnnotation]; ok && len(branch) > 0 {
		return branch
	}
	if branch, ok := rj.GetLabels()[kube.RadixBuildLabel]; ok && len(branch) > 0 {
		return branch
	}
	return ""
}

func (h *history) getAllRadixJobs(ctx context.Context, namespace string) ([]radixv1.RadixJob, error) {
	radixJobList, err := h.radixClient.RadixV1().RadixJobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return radixJobList.Items, err
}

func (h *history) getJobsToGarbageCollectByJobConditionAndBranch(ctx context.Context, jobsForConditions radixJobsForConditionsMap) []radixv1.RadixJob {
	var deletingJobs []radixv1.RadixJob
	for jobCondition, jobsForBranches := range jobsForConditions {
		switch jobCondition {
		case radixv1.JobRunning, radixv1.JobQueued, radixv1.JobWaiting, "": // Jobs with this condition should never be garbage collected
			continue
		default:
			for jobBranch, jobs := range jobsForBranches {
				jobs := sortRadixJobsByCreatedDesc(jobs)
				for i := h.historyLimit; i < len(jobs); i++ {
					log.Ctx(ctx).Debug().Msgf("Collect for deleting RadixJob %s for the env %s, condition %s", jobs[i].GetName(), jobBranch, jobCondition)
					deletingJobs = append(deletingJobs, jobs[i])
				}
			}
		}
	}
	return deletingJobs
}

func (h *history) garbageCollectConfigMaps(ctx context.Context, namespace string) {
	radixJobConfigMaps, err := h.kubeUtil.ListConfigMapsWithSelector(ctx, namespace, getRadixJobNameExistsSelector().String())
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Msgf("Failed to get ConfigMaps while garbage collecting config-maps in %s", namespace)
		return
	}
	radixJobNameSet, err := h.getRadixJobNameSet(ctx, namespace)
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Msgf("Failed to get RadixJobs while garbage collecting config-maps in %s", namespace)
		return
	}
	for _, configMap := range radixJobConfigMaps {
		jobName := configMap.GetLabels()[kube.RadixJobNameLabel]
		configMapName := configMap.GetName()
		if _, radixJobExists := radixJobNameSet[jobName]; !radixJobExists {
			log.Ctx(ctx).Debug().Msgf("Delete ConfigMap %s in %s", configMapName, namespace)
			err := h.kubeUtil.DeleteConfigMap(ctx, namespace, configMapName)
			if err != nil {
				log.Ctx(ctx).Warn().Err(err).Msgf("Failed to delete ConfigMap %s while garbage collecting config-maps in %s", configMapName, namespace)
			}
		}
	}
}

func (h *history) getRadixJobNameSet(ctx context.Context, namespace string) (map[string]struct{}, error) {
	radixJobs, err := h.getAllRadixJobs(ctx, namespace)
	if err != nil {
		return nil, err
	}
	return slice.Reduce(radixJobs, make(map[string]struct{}), func(acc map[string]struct{}, radixJob radixv1.RadixJob) map[string]struct{} {
		acc[radixJob.GetName()] = struct{}{}
		return acc
	}), nil
}

func getRadixJobNameExistsSelector() labels.Selector {
	requirement, _ := labels.NewRequirement(kube.RadixJobNameLabel, selection.Exists, []string{})
	return labels.NewSelector().Add(*requirement)
}

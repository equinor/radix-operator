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
	radixClient                 radixclient.Interface
	historyLimit                int
	kubeUtil                    *kube.Kube
	done                        chan struct{}
}

type historyOption func(history *history)

// NewHistory Constructor for job History
func NewHistory(radixClient radixclient.Interface, kubeUtil *kube.Kube, historyLimit int, opts ...historyOption) History {
	h := &history{
		radixClient:  radixClient,
		historyLimit: historyLimit,
		kubeUtil:     kubeUtil,
		done:         make(chan struct{}),
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// WithDoneChannel Option to set done channel
func WithDoneChannel(done chan struct{}) historyOption {
	return func(h *history) {
		h.done = done
	}
}

// Cleanup the pipeline job history
func (h *history) Cleanup(ctx context.Context, appName, radixJobName string) {
	namespace := utils.GetAppNamespace(appName)
	if _, ok := h.namespacesRequestsToCleanup.LoadOrStore(namespace, struct{}{}); ok {
		return // a request to clean up history in this namespace already exists
	}
	go func() {
		defer func() {
			h.namespacesRequestsToCleanup.Delete(namespace)
			h.done <- struct{}{}
		}()
		h.garbageCollectRadixJobs(ctx, appName, radixJobName)
		h.garbageCollectConfigMaps(ctx, namespace)
	}()
}

type radixJobsWithRadixDeployments map[string]radixv1.RadixDeployment
type radixJobsForBranches map[string][]radixv1.RadixJob
type radixJobsForConditionsMap map[radixv1.RadixJobCondition]radixJobsForBranches

func (h *history) garbageCollectRadixJobs(ctx context.Context, appName string, activeRadixJobName string) {
	namespace := utils.GetAppNamespace(appName)
	ra, err := h.radixClient.RadixV1().RadixApplications(namespace).Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		// RadixApplication may not exist if this is the first job for a new application
		if k8errors.IsNotFound(err) {
			return // no need to delete anything
		}
		log.Ctx(ctx).Error().Err(err).Msg("failed to get RadixApplication in cleanup job history")
		return
	}
	radixJobs, err := h.getAllRadixJobs(ctx, namespace)
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Msg("failed to get RadixJob in cleanup job history")
		return
	}
	if len(radixJobs) == 0 {
		return
	}
	radixJobsWithRDs, err := h.getRadixJobsMapToRadixDeployments(ctx, appName, ra)
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Msg("failed to get RadixJobs with RadixDeployments in cleanup job history")
		return
	}

	log.Ctx(ctx).Info().Msgf("Delete history RadixJob for limit %d", h.historyLimit)

	completedRadixJobsWithoutRDs, radixJobsForConditionsAndEnvs := getRadixJobCandidatesForDeletion(radixJobs, radixJobsWithRDs, ra, activeRadixJobName)
	if len(completedRadixJobsWithoutRDs) > 0 {
		log.Ctx(ctx).Info().Msg("Delete %d completed RadixJobs without RadixDeployment")
	}

	deletingRadixJobsByConditionsAndEnvs := h.getRadixJobsToGarbageCollectByJobConditionsAndEnvs(ctx, radixJobsForConditionsAndEnvs)

	deletingRadixJobs := append(completedRadixJobsWithoutRDs, deletingRadixJobsByConditionsAndEnvs...)
	if len(deletingRadixJobs) == 0 {
		log.Ctx(ctx).Info().Msg("There is no RadixJobs to delete")
		return
	}
	for _, radixJob := range deletingRadixJobs {
		log.Ctx(ctx).Info().Msgf("Delete RadixJob %s from %s", radixJob.GetName(), namespace)
		if err := h.radixClient.RadixV1().RadixJobs(namespace).Delete(ctx, radixJob.GetName(), metav1.DeleteOptions{}); err != nil {
			log.Ctx(ctx).Warn().Err(err).Msgf("Failed to delete RadixJob %s from %s", radixJob.GetName(), namespace)
		}
	}
}

func getRadixJobCandidatesForDeletion(radixJobs []radixv1.RadixJob, radixJobsWithRDs radixJobsWithRadixDeployments, ra *radixv1.RadixApplication, activeRadixJobName string) ([]radixv1.RadixJob, radixJobsForConditionsMap) {
	branchToEnvsMap := getBranchesToEnvsMap(ra)
	var deletingJobs []radixv1.RadixJob
	radixJobsForConditionsAndEnvs := make(radixJobsForConditionsMap)
	for _, radixJob := range radixJobs {
		if strings.EqualFold(radixJob.GetName(), activeRadixJobName) {
			continue // do not remove active RadixJob
		}
		jobCondition := radixJob.Status.Condition
		if !jobIsCompleted(jobCondition) {
			continue // keep not completed jobs
		}
		rj := radixJob
		if jobCondition == radixv1.JobSucceeded && rj.Spec.PipeLineType != radixv1.Build {
			if _, ok := radixJobsWithRDs[rj.GetName()]; !ok {
				deletingJobs = append(deletingJobs, rj) // delete all completed job, which does not have a RadixDeployment, excluding build-only jobs
				continue
			}
		}
		if radixJobsForConditionsAndEnvs[jobCondition] == nil {
			radixJobsForConditionsAndEnvs[jobCondition] = make(radixJobsForBranches)
		}
		radixJobTargetEnvs := getRadixJobEnvs(rj, branchToEnvsMap)
		for _, targetEnv := range radixJobTargetEnvs {
			radixJobsForConditionsAndEnvs[jobCondition][targetEnv] = append(radixJobsForConditionsAndEnvs[jobCondition][targetEnv], rj)
		}
	}
	return deletingJobs, radixJobsForConditionsAndEnvs
}

func getBranchesToEnvsMap(ra *radixv1.RadixApplication) map[string][]string {
	return slice.Reduce(ra.Spec.Environments, make(map[string][]string), func(acc map[string][]string, env radixv1.Environment) map[string][]string {
		if len(env.Build.From) > 0 {
			acc[env.Build.From] = append(acc[env.Build.From], env.Name)
		}
		return acc
	})
}

func jobIsCompleted(jobCondition radixv1.RadixJobCondition) bool {
	return jobCondition == radixv1.JobSucceeded || jobCondition == radixv1.JobFailed || jobCondition == radixv1.JobStopped || jobCondition == radixv1.JobStoppedNoChanges
}

func (h *history) getRadixJobsMapToRadixDeployments(ctx context.Context, appName string, ra *radixv1.RadixApplication) (radixJobsWithRadixDeployments, error) {
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

func getRadixJobEnvs(rj radixv1.RadixJob, envsMap map[string][]string) []string {
	switch rj.Spec.PipeLineType {
	case radixv1.BuildDeploy:
		return envsMap[rj.Spec.Build.Branch]
	case radixv1.Deploy:
		return []string{rj.Spec.Deploy.ToEnvironment}
	case radixv1.Promote:
		return []string{rj.Spec.Promote.ToEnvironment}
	}
	return nil
}

func (h *history) getAllRadixJobs(ctx context.Context, namespace string) ([]radixv1.RadixJob, error) {
	radixJobList, err := h.radixClient.RadixV1().RadixJobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return radixJobList.Items, err
}

func (h *history) getRadixJobsToGarbageCollectByJobConditionsAndEnvs(ctx context.Context, jobsForConditions radixJobsForConditionsMap) []radixv1.RadixJob {
	var deletingJobs []radixv1.RadixJob
	for jobCondition, jobsForEnvs := range jobsForConditions {
		for env, jobsForEnv := range jobsForEnvs {
			jobs := sortRadixJobsByCreatedDesc(jobsForEnv)
			for i := h.historyLimit; i < len(jobs); i++ {
				log.Ctx(ctx).Debug().Msgf("Collect for deleting RadixJob %s for the env %s, condition %s", jobs[i].GetName(), env, jobCondition)
				deletingJobs = append(deletingJobs, jobs[i])
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

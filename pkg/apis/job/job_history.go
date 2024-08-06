package job

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

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
	// Cleanup the pipeline job history for the Radix application
	Cleanup(ctx context.Context, appName string) error
}

type history struct {
	namespacesRequestsToCleanup sync.Map
	radixClient                 radixclient.Interface
	historyLimit                int
	kubeUtil                    *kube.Kube
	historyPeriodLimit          time.Duration
}

// NewHistory Constructor for job History
func NewHistory(radixClient radixclient.Interface, kubeUtil *kube.Kube, historyLimit int, historyPeriodLimit time.Duration) History {
	return &history{
		radixClient:        radixClient,
		historyLimit:       historyLimit,
		historyPeriodLimit: historyPeriodLimit,
		kubeUtil:           kubeUtil,
	}
}

// Cleanup the pipeline job history
func (h *history) Cleanup(ctx context.Context, appName string) error {
	namespace := utils.GetAppNamespace(appName)
	if _, ok := h.namespacesRequestsToCleanup.LoadOrStore(namespace, struct{}{}); ok {
		return nil // a request to clean up history in this namespace already exists
	}
	defer h.namespacesRequestsToCleanup.Delete(namespace)
	if err := h.garbageCollectRadixJobs(ctx, appName); err != nil {
		return err
	}
	return h.garbageCollectConfigMaps(ctx, namespace)
}

type radixJobsWithRadixDeployments map[string]radixv1.RadixDeployment
type radixJobsForBranches map[string][]radixv1.RadixJob
type radixJobsForConditionsMap map[radixv1.RadixJobCondition]radixJobsForBranches
type radixJobsNamesMap map[string]struct{}

func (h *history) garbageCollectRadixJobs(ctx context.Context, appName string) error {
	namespace := utils.GetAppNamespace(appName)
	radixJobs, err := h.getAllRadixJobs(ctx, namespace)
	if err != nil {
		return err
	}
	if len(radixJobs) == 0 {
		return nil // no need to delete anything or the active job is already completed
	}
	ra, err := h.radixClient.RadixV1().RadixApplications(namespace).Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		// RadixApplication may not exist if this is the first job for a new application
		if k8errors.IsNotFound(err) {
			return nil // no need to delete anything
		}
		return err
	}
	radixJobsWithRDs, err := h.getRadixJobsMapToRadixDeployments(ctx, appName, ra)
	if err != nil {
		return err
	}

	log.Ctx(ctx).Info().Msgf("Delete history RadixJob for limit %d", h.historyLimit)

	radixJobsToBeExplicitlyDeleted, radixJobsForConditionsAndEnvs, radixJobsNamesWithExistingRadixDeployments := h.getRadixJobCandidatesForDeletion(radixJobs, radixJobsWithRDs, ra)
	if len(radixJobsToBeExplicitlyDeleted) > 0 {
		log.Ctx(ctx).Info().Msgf("Delete %d RadixJobs without considering history rules", len(radixJobsToBeExplicitlyDeleted))
	}

	deletingRadixJobsByConditionsAndEnvs := h.getRadixJobsToGarbageCollectByJobConditionsAndEnvs(ctx, radixJobsForConditionsAndEnvs, radixJobsNamesWithExistingRadixDeployments)

	deletingRadixJobs := append(radixJobsToBeExplicitlyDeleted, deletingRadixJobsByConditionsAndEnvs...)
	if len(deletingRadixJobs) == 0 {
		log.Ctx(ctx).Info().Msg("There is no RadixJobs to delete")
		return nil
	}
	var errs []error
	for _, radixJob := range deletingRadixJobs {
		select {
		case <-ctx.Done():
			return fmt.Errorf("failed deleting of RadixJobs: %w", ctx.Err())
		default:
			log.Ctx(ctx).Info().Msgf("Delete RadixJob %s from %s", radixJob.GetName(), namespace)
			if err := h.radixClient.RadixV1().RadixJobs(namespace).Delete(ctx, radixJob.GetName(), metav1.DeleteOptions{}); err != nil {
				errs = append(errs, fmt.Errorf("failed to delete RadixJob %s from %s: %w", radixJob.GetName(), namespace, err))
			}
		}
	}
	return errors.Join(errs...)
}

func (h *history) getRadixJobCandidatesForDeletion(radixJobs []radixv1.RadixJob, radixJobsWithRDs radixJobsWithRadixDeployments, ra *radixv1.RadixApplication) ([]radixv1.RadixJob, radixJobsForConditionsMap, radixJobsNamesMap) {
	branchToEnvsMap := getBranchesToEnvsMap(ra)
	var radixJobsToBeExplicitlyDeleted []radixv1.RadixJob
	radixJobsForConditionsAndEnvs := make(radixJobsForConditionsMap)
	radixJobsNamesWithExistingRadixDeployments := make(radixJobsNamesMap)
	for _, radixJob := range radixJobs {
		jobCondition := radixJob.Status.Condition
		rj := radixJob
		if _, rdExists := radixJobsWithRDs[rj.GetName()]; rdExists {
			radixJobsNamesWithExistingRadixDeployments[rj.GetName()] = struct{}{}
		} else {
			if jobCondition == radixv1.JobSucceeded && rj.Spec.PipeLineType != radixv1.Build && rj.Spec.PipeLineType != radixv1.ApplyConfig {
				radixJobsToBeExplicitlyDeleted = append(radixJobsToBeExplicitlyDeleted, rj) // delete all completed job, which does not have a RadixDeployment, excluding build-only jobs
				continue
			}
			if rj.Status.Created != nil && rj.Status.Created.Time.Before(time.Now().Add(-h.historyPeriodLimit)) {
				radixJobsToBeExplicitlyDeleted = append(radixJobsToBeExplicitlyDeleted, rj) // delete all job, which is older than the history period limit
				continue
			}
		}
		if !jobIsCompleted(jobCondition) {
			continue // keep not completed jobs
		}
		if radixJobsForConditionsAndEnvs[jobCondition] == nil {
			radixJobsForConditionsAndEnvs[jobCondition] = make(radixJobsForBranches)
		}
		radixJobTargetEnvs := getRadixJobEnvs(rj, branchToEnvsMap)
		for _, targetEnv := range radixJobTargetEnvs {
			radixJobsForConditionsAndEnvs[jobCondition][targetEnv] = append(radixJobsForConditionsAndEnvs[jobCondition][targetEnv], rj)
		}
	}
	return radixJobsToBeExplicitlyDeleted, radixJobsForConditionsAndEnvs, radixJobsNamesWithExistingRadixDeployments
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
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("failed getting of RadixDeployments: %w", ctx.Err())
		default:
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
	return []string{""}
}

func (h *history) getAllRadixJobs(ctx context.Context, namespace string) ([]radixv1.RadixJob, error) {
	radixJobList, err := h.radixClient.RadixV1().RadixJobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return radixJobList.Items, err
}

func (h *history) getRadixJobsToGarbageCollectByJobConditionsAndEnvs(ctx context.Context, jobsForConditions radixJobsForConditionsMap, radixJobsNamesWithExistingRadixDeployments radixJobsNamesMap) []radixv1.RadixJob {
	var deletingJobs []radixv1.RadixJob
	for jobCondition, jobsForEnvs := range jobsForConditions {
		for env, jobsForEnv := range jobsForEnvs {
			jobs := sortRadixJobsByCreatedDesc(jobsForEnv)
			for i := h.historyLimit; i < len(jobs); i++ {
				if _, jobExists := radixJobsNamesWithExistingRadixDeployments[jobs[i].GetName()]; jobExists {
					continue // keep RadixJobs with existing RadixDeployments
				}
				log.Ctx(ctx).Debug().Msgf("Collect for deleting RadixJob %s for the env %s, condition %s", jobs[i].GetName(), env, jobCondition)
				deletingJobs = append(deletingJobs, jobs[i])
			}
		}
	}
	return deletingJobs
}

func (h *history) garbageCollectConfigMaps(ctx context.Context, namespace string) error {
	radixJobConfigMaps, err := h.kubeUtil.ListConfigMapsWithSelector(ctx, namespace, getRadixJobNameExistsSelector().String())
	if err != nil {
		return err
	}
	radixJobNameSet, err := h.getRadixJobNameSet(ctx, namespace)
	if err != nil {
		return err
	}
	var errs []error
	for _, configMap := range radixJobConfigMaps {
		select {
		case <-ctx.Done():
			return fmt.Errorf("failed deleting of RadixJob's ConfigMaps: %w", ctx.Err())
		default:
			jobName := configMap.GetLabels()[kube.RadixJobNameLabel]
			configMapName := configMap.GetName()
			if _, radixJobExists := radixJobNameSet[jobName]; !radixJobExists {
				log.Ctx(ctx).Debug().Msgf("Delete ConfigMap %s in %s", configMapName, namespace)
				err := h.kubeUtil.DeleteConfigMap(ctx, namespace, configMapName)
				if err != nil {
					errs = append(errs, err)
				}
			}
		}
	}
	return errors.Join(errs...)
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

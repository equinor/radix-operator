package volumemount

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GarbageCollectVolumesNoLongerInSpec deletes the Radix owned PVCs and PVs of the application in the namespace of the RadixDeployment
// that belong to components or jobs that are not in the RadixDeployment. PVCs and PVs created less than deletionGracePeriod ago,
// and PVCs used by Kubernetes Jobs that have not finished and their PVs, are kept.
//
// The RadixDeployment and RadixBatch syncers do not share a lock, so the RadixBatch syncer can create a PV/PVC for a job
// while the RadixDeployment syncer garbage collects the PVs/PVCs of the job. Without a grace period, the new PV/PVC
// could be deleted before the Pod of the Kubernetes Job uses it. Once a Pod uses the PVC,
// kubernetes.io/pvc-protection and kubernetes.io/pv-protection keep the PVC and PV until the Pod is gone.
func GarbageCollectVolumesNoLongerInSpec(ctx context.Context, kubeClient kubernetes.Interface, appName string, rd *radixv1.RadixDeployment, deletionGracePeriod time.Duration) error {
	componentNames := make(map[string]struct{})
	for _, component := range rd.Spec.Components {
		componentNames[component.GetName()] = struct{}{}
	}
	for _, job := range rd.Spec.Jobs {
		componentNames[job.GetName()] = struct{}{}
	}
	isInSpec := func(labels map[string]string) bool {
		_, ok := componentNames[labels[kube.RadixComponentLabel]]
		return ok
	}

	namespace := rd.GetNamespace()
	pvcs, err := listRadixPvcs(ctx, kubeClient, appName, namespace, nil)
	if err != nil {
		return err
	}
	pvs, err := listRadixPvs(ctx, kubeClient, appName, namespace, nil)
	if err != nil {
		return err
	}
	pvcs = slices.DeleteFunc(pvcs, func(pvc corev1.PersistentVolumeClaim) bool { return isInSpec(pvc.GetLabels()) })
	pvs = slices.DeleteFunc(pvs, func(pv corev1.PersistentVolume) bool { return isInSpec(pv.GetLabels()) })
	return newVolumeCollector(kubeClient, namespace, deletionGracePeriod).delete(ctx, pvcs, pvs)
}

// volumeCollector deletes Radix owned PVCs and PVs of a namespace, except those that must be kept
type volumeCollector struct {
	kubeClient          kubernetes.Interface
	namespace           string
	deletionGracePeriod time.Duration
}

// newVolumeCollector returns a volumeCollector for the namespace that keeps PVCs and PVs created less than deletionGracePeriod ago
func newVolumeCollector(kubeClient kubernetes.Interface, namespace string, deletionGracePeriod time.Duration) *volumeCollector {
	return &volumeCollector{kubeClient: kubeClient, namespace: namespace, deletionGracePeriod: deletionGracePeriod}
}

// deleteUnused deletes the PVCs and PVs that are not used by any volume mount. usedPvcs has the used PVC by volume mount name.
func (c *volumeCollector) deleteUnused(ctx context.Context, pvcs []corev1.PersistentVolumeClaim, pvs []corev1.PersistentVolume, usedPvcs map[string]*corev1.PersistentVolumeClaim) error {
	usedPvcNames := make(map[string]struct{}, len(usedPvcs))
	usedPvNames := make(map[string]struct{}, len(usedPvcs))
	for _, pvc := range usedPvcs {
		usedPvcNames[pvc.GetName()] = struct{}{}
		usedPvNames[pvc.Spec.VolumeName] = struct{}{}
	}
	unusedPvcs := slices.DeleteFunc(slices.Clone(pvcs), func(pvc corev1.PersistentVolumeClaim) bool {
		_, used := usedPvcNames[pvc.GetName()]
		return used
	})
	unusedPvs := slices.DeleteFunc(slices.Clone(pvs), func(pv corev1.PersistentVolume) bool {
		_, used := usedPvNames[pv.GetName()]
		return used
	})
	return c.delete(ctx, unusedPvcs, unusedPvs)
}

// delete deletes the PVCs, then the PVs.
// Resources that are already being deleted, or that are within the deletion grace period, are skipped.
// PVCs used by the Pod template of a Kubernetes Job in the namespace that has not finished, and the PVs bound to them, are skipped,
// because the Job can still create Pods, e.g. to retry a failed Pod, and they cannot start without the PVC.
func (c *volumeCollector) delete(ctx context.Context, pvcs []corev1.PersistentVolumeClaim, pvs []corev1.PersistentVolume) error {
	if len(pvcs) == 0 && len(pvs) == 0 {
		return nil
	}
	jobClaimNames, err := c.getClaimNamesUsedByUnfinishedJobs(ctx)
	if err != nil {
		return err
	}
	jobPvNames := make(map[string]struct{})
	for _, pvc := range pvcs {
		if _, ok := jobClaimNames[pvc.GetName()]; ok && len(pvc.Spec.VolumeName) > 0 {
			jobPvNames[pvc.Spec.VolumeName] = struct{}{}
		}
	}

	logger := log.Ctx(ctx)
	now := time.Now()
	var errs []error
	for _, pvc := range pvcs {
		if pvc.GetDeletionTimestamp() != nil {
			continue
		}
		if c.isWithinDeletionGracePeriod(&pvc, now) {
			logger.Debug().Msgf("Skip delete of PersistentVolumeClaim %s in namespace %s, created less than %s ago", pvc.GetName(), pvc.GetNamespace(), c.deletionGracePeriod)
			continue
		}
		if _, ok := jobClaimNames[pvc.GetName()]; ok {
			logger.Debug().Msgf("Skip delete of PersistentVolumeClaim %s in namespace %s, used by a Job that has not finished", pvc.GetName(), pvc.GetNamespace())
			continue
		}
		logger.Info().Msgf("Delete PersistentVolumeClaim %s in namespace %s", pvc.GetName(), pvc.GetNamespace())
		if err := c.kubeClient.CoreV1().PersistentVolumeClaims(pvc.GetNamespace()).Delete(ctx, pvc.GetName(), deleteOptionsWithUIDPrecondition(&pvc)); ignoreNotFoundOrConflict(err) != nil {
			errs = append(errs, fmt.Errorf("failed to delete PersistentVolumeClaim %s: %w", pvc.GetName(), err))
		}
	}
	for _, pv := range pvs {
		if pv.GetDeletionTimestamp() != nil {
			continue
		}
		if c.isWithinDeletionGracePeriod(&pv, now) {
			logger.Debug().Msgf("Skip delete of PersistentVolume %s, created less than %s ago", pv.GetName(), c.deletionGracePeriod)
			continue
		}
		if c.isPvUsedByUnfinishedJob(&pv, jobClaimNames, jobPvNames) {
			logger.Debug().Msgf("Skip delete of PersistentVolume %s, its PersistentVolumeClaim is used by a Job that has not finished", pv.GetName())
			continue
		}
		logger.Info().Msgf("Delete PersistentVolume %s (phase %s)", pv.GetName(), pv.Status.Phase)
		if err := c.kubeClient.CoreV1().PersistentVolumes().Delete(ctx, pv.GetName(), deleteOptionsWithUIDPrecondition(&pv)); ignoreNotFoundOrConflict(err) != nil {
			errs = append(errs, fmt.Errorf("failed to delete PersistentVolume %s: %w", pv.GetName(), err))
		}
	}
	return errors.Join(errs...)
}

// getClaimNamesUsedByUnfinishedJobs returns the names of the PVCs used by the Pod template of the Kubernetes Jobs in the namespace that have not finished
func (c *volumeCollector) getClaimNamesUsedByUnfinishedJobs(ctx context.Context) (map[string]struct{}, error) {
	jobs, err := c.kubeClient.BatchV1().Jobs(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list Jobs: %w", err)
	}
	claimNames := make(map[string]struct{})
	for _, job := range jobs.Items {
		if isJobFinished(&job) {
			continue
		}
		for _, volume := range job.Spec.Template.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil {
				claimNames[volume.PersistentVolumeClaim.ClaimName] = struct{}{}
			}
		}
	}
	return claimNames, nil
}

// isPvUsedByUnfinishedJob returns true if the PV is bound to a PVC used by a Kubernetes Job that has not finished.
// jobClaimNames has the PVCs used by such Jobs, and jobPvNames the PVs of those PVCs.
func (c *volumeCollector) isPvUsedByUnfinishedJob(pv *corev1.PersistentVolume, jobClaimNames, jobPvNames map[string]struct{}) bool {
	if _, ok := jobPvNames[pv.GetName()]; ok {
		return true
	}
	if ref := pv.Spec.ClaimRef; ref != nil && ref.Namespace == c.namespace {
		_, ok := jobClaimNames[ref.Name]
		return ok
	}
	return false
}

// isWithinDeletionGracePeriod returns true if the object was created less than deletionGracePeriod before now
func (c *volumeCollector) isWithinDeletionGracePeriod(obj metav1.Object, now time.Time) bool {
	return obj.GetCreationTimestamp().Add(c.deletionGracePeriod).After(now)
}

// isJobFinished returns true if the Job has the terminal condition Complete or Failed. A finished Job does not create new Pods.
func isJobFinished(job *batchv1.Job) bool {
	return slices.ContainsFunc(job.Status.Conditions, func(c batchv1.JobCondition) bool {
		return (c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed) && c.Status == corev1.ConditionTrue
	})
}

// deleteOptionsWithUIDPrecondition makes a delete fail with a Conflict error, if the object was replaced by another object with the same name
func deleteOptionsWithUIDPrecondition(obj metav1.Object) metav1.DeleteOptions {
	if uid := obj.GetUID(); len(uid) > 0 {
		return metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}}
	}
	return metav1.DeleteOptions{}
}

func ignoreNotFoundOrConflict(err error) error {
	if k8serrors.IsNotFound(err) || k8serrors.IsConflict(err) {
		return nil
	}
	return err
}

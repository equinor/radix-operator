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
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GarbageCollectVolumesNoLongerInSpec deletes the Radix owned PVCs and PVs of the application in the namespace of the RadixDeployment
// that belong to components or jobs that are not in the RadixDeployment. PVCs and PVs created less than deletionGracePeriod ago are kept.
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
	return deleteVolumeResources(ctx, kubeClient, pvcs, pvs, deletionGracePeriod)
}

// deleteUnusedVolumeResources deletes the PVCs and PVs that are not used by any volume mount. usedPvcs has the used PVC by volume mount name.
func deleteUnusedVolumeResources(ctx context.Context, kubeClient kubernetes.Interface, pvcs []corev1.PersistentVolumeClaim, pvs []corev1.PersistentVolume, usedPvcs map[string]*corev1.PersistentVolumeClaim, deletionGracePeriod time.Duration) error {
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
	return deleteVolumeResources(ctx, kubeClient, unusedPvcs, unusedPvs, deletionGracePeriod)
}

// deleteVolumeResources deletes the PVCs, then the PVs.
// Resources that are already being deleted, or that are within the deletion grace period, are skipped.
func deleteVolumeResources(ctx context.Context, kubeClient kubernetes.Interface, pvcs []corev1.PersistentVolumeClaim, pvs []corev1.PersistentVolume, deletionGracePeriod time.Duration) error {
	logger := log.Ctx(ctx)
	now := time.Now()
	var errs []error
	for _, pvc := range pvcs {
		if pvc.GetDeletionTimestamp() != nil {
			continue
		}
		if isWithinDeletionGracePeriod(&pvc, now, deletionGracePeriod) {
			logger.Debug().Msgf("Skip delete of PersistentVolumeClaim %s in namespace %s, created less than %s ago", pvc.GetName(), pvc.GetNamespace(), deletionGracePeriod)
			continue
		}
		logger.Info().Msgf("Delete PersistentVolumeClaim %s in namespace %s", pvc.GetName(), pvc.GetNamespace())
		if err := kubeClient.CoreV1().PersistentVolumeClaims(pvc.GetNamespace()).Delete(ctx, pvc.GetName(), deleteOptionsWithUIDPrecondition(&pvc)); ignoreNotFoundOrConflict(err) != nil {
			errs = append(errs, fmt.Errorf("failed to delete PersistentVolumeClaim %s: %w", pvc.GetName(), err))
		}
	}
	for _, pv := range pvs {
		if pv.GetDeletionTimestamp() != nil {
			continue
		}
		if isWithinDeletionGracePeriod(&pv, now, deletionGracePeriod) {
			logger.Debug().Msgf("Skip delete of PersistentVolume %s, created less than %s ago", pv.GetName(), deletionGracePeriod)
			continue
		}
		logger.Info().Msgf("Delete PersistentVolume %s (phase %s)", pv.GetName(), pv.Status.Phase)
		if err := kubeClient.CoreV1().PersistentVolumes().Delete(ctx, pv.GetName(), deleteOptionsWithUIDPrecondition(&pv)); ignoreNotFoundOrConflict(err) != nil {
			errs = append(errs, fmt.Errorf("failed to delete PersistentVolume %s: %w", pv.GetName(), err))
		}
	}
	return errors.Join(errs...)
}

// isWithinDeletionGracePeriod returns true if the object was created less than deletionGracePeriod before now
func isWithinDeletionGracePeriod(obj metav1.Object, now time.Time, deletionGracePeriod time.Duration) bool {
	return obj.GetCreationTimestamp().Add(deletionGracePeriod).After(now)
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

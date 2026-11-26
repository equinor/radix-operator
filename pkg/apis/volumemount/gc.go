package volumemount

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GarbageCollectVolumesNoLongerInSpec deletes the Radix owned PVCs and PVs of the application in the namespace of the RadixDeployment
// that belong to components or jobs that are not in the RadixDeployment. PVCs used by workloads that can still create Pods
// (see volumeCollector.getClaimNamesUsedByWorkloads) and their PVs are kept. Once a Pod uses the PVC,
// kubernetes.io/pvc-protection and kubernetes.io/pv-protection keep the PVC and PV until the Pod is gone.
func GarbageCollectVolumesNoLongerInSpec(ctx context.Context, kubeClient kubernetes.Interface, appName string, rd *radixv1.RadixDeployment) error {
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
	return newVolumeCollector(kubeClient, namespace).delete(ctx, pvcs, pvs)
}

// volumeCollector deletes Radix owned PVCs and PVs of a namespace, except those that must be kept
type volumeCollector struct {
	kubeClient kubernetes.Interface
	namespace  string
}

// newVolumeCollector returns a volumeCollector for the namespace
func newVolumeCollector(kubeClient kubernetes.Interface, namespace string) *volumeCollector {
	return &volumeCollector{kubeClient: kubeClient, namespace: namespace}
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
// Resources that are already being deleted are skipped.
// PVCs used by a workload that can still create Pods (see getClaimNamesUsedByWorkloads), and the PVs bound to them, are skipped,
// because Kubernetes does not start a new Pod with a terminating or deleted PVC. Storage object protection only keeps the PVC for existing Pods.
func (c *volumeCollector) delete(ctx context.Context, pvcs []corev1.PersistentVolumeClaim, pvs []corev1.PersistentVolume) error {
	if len(pvcs) == 0 && len(pvs) == 0 {
		return nil
	}
	workloadClaimNames, err := c.getClaimNamesUsedByWorkloads(ctx)
	if err != nil {
		return err
	}
	workloadPvNames := make(map[string]struct{})
	for _, pvc := range pvcs {
		if _, ok := workloadClaimNames[pvc.GetName()]; ok && len(pvc.Spec.VolumeName) > 0 {
			workloadPvNames[pvc.Spec.VolumeName] = struct{}{}
		}
	}

	logger := log.Ctx(ctx)
	var errs []error
	for _, pvc := range pvcs {
		if pvc.GetDeletionTimestamp() != nil {
			continue
		}
		if _, ok := workloadClaimNames[pvc.GetName()]; ok {
			logger.Debug().Msgf("Skip delete of PersistentVolumeClaim %s in namespace %s, used by a Job that has not finished or a ReplicaSet with replicas", pvc.GetName(), pvc.GetNamespace())
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
		if c.isPvUsedByWorkload(&pv, workloadClaimNames, workloadPvNames) {
			logger.Debug().Msgf("Skip delete of PersistentVolume %s, its PersistentVolumeClaim is used by a Job that has not finished or a ReplicaSet with replicas", pv.GetName())
			continue
		}
		logger.Info().Msgf("Delete PersistentVolume %s (phase %s)", pv.GetName(), pv.Status.Phase)
		if err := c.kubeClient.CoreV1().PersistentVolumes().Delete(ctx, pv.GetName(), deleteOptionsWithUIDPrecondition(&pv)); ignoreNotFoundOrConflict(err) != nil {
			errs = append(errs, fmt.Errorf("failed to delete PersistentVolume %s: %w", pv.GetName(), err))
		}
	}
	return errors.Join(errs...)
}

// getClaimNamesUsedByWorkloads returns the names of the PVCs used by the Pod template of workloads in the namespace that can still create Pods:
//   - Kubernetes Jobs that have not finished, which can create Pods, e.g. to retry a failed Pod.
//   - ReplicaSets with replicas, which recreate Pods, e.g. after an eviction. This includes the ReplicaSet of the previous revision
//     of a Deployment while a rollout is in progress or has failed, so the previous revision keeps working until it is scaled down.
func (c *volumeCollector) getClaimNamesUsedByWorkloads(ctx context.Context) (map[string]struct{}, error) {
	claimNames := make(map[string]struct{})
	addClaimNames := func(podSpec *corev1.PodSpec) {
		for _, volume := range podSpec.Volumes {
			if volume.PersistentVolumeClaim != nil {
				claimNames[volume.PersistentVolumeClaim.ClaimName] = struct{}{}
			}
		}
	}

	jobs, err := c.kubeClient.BatchV1().Jobs(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list Jobs: %w", err)
	}
	for _, job := range jobs.Items {
		if !isJobFinished(&job) {
			addClaimNames(&job.Spec.Template.Spec)
		}
	}

	replicaSets, err := c.kubeClient.AppsV1().ReplicaSets(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list ReplicaSets: %w", err)
	}
	for _, replicaSet := range replicaSets.Items {
		if hasReplicas(&replicaSet) {
			addClaimNames(&replicaSet.Spec.Template.Spec)
		}
	}
	return claimNames, nil
}

// isPvUsedByWorkload returns true if the PV is bound to a PVC used by a workload that can still create Pods.
// workloadClaimNames has the PVCs used by such workloads, and workloadPvNames the PVs of those PVCs.
func (c *volumeCollector) isPvUsedByWorkload(pv *corev1.PersistentVolume, workloadClaimNames, workloadPvNames map[string]struct{}) bool {
	if _, ok := workloadPvNames[pv.GetName()]; ok {
		return true
	}
	if ref := pv.Spec.ClaimRef; ref != nil && ref.Namespace == c.namespace {
		_, ok := workloadClaimNames[ref.Name]
		return ok
	}
	return false
}

// hasReplicas returns true if the ReplicaSet wants Pods. A ReplicaSet without spec.replicas defaults to 1.
func hasReplicas(replicaSet *appsv1.ReplicaSet) bool {
	return replicaSet.Spec.Replicas == nil || *replicaSet.Spec.Replicas > 0
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

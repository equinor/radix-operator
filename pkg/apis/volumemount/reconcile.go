package volumemount

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubelabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
)

// ReconcileComponentVolumes returns the Pod volumes and container volume mounts for a component or job in the RadixDeployment.
// For each BlobFuse2 volume mount it reuses the current PV/PVC of the component if it matches the desired spec, or creates the desired PV/PVC.
// All other PVs/PVCs of the component are deleted. Kubernetes keeps a PVC, and its PV, that is used by a Pod in Terminating state
// (kubernetes.io/pvc-protection and kubernetes.io/pv-protection) until the Pod is gone, so running workloads are not affected.
// Blob data is never deleted, as all PVs use the reclaim policy Retain.
// The application name is read from the radix-app label of the namespace of the RadixDeployment.
func ReconcileComponentVolumes(ctx context.Context, kubeUtil *kube.Kube, rd *radixv1.RadixDeployment, component radixv1.RadixCommonDeployComponent) ([]corev1.Volume, []corev1.VolumeMount, error) {
	return getComponentVolumes(ctx, kubeUtil, rd, component, true)
}

// EnsureComponentVolumes returns the Pod volumes and container volume mounts for a component or job in the RadixDeployment.
// For each BlobFuse2 volume mount it reuses the current PV/PVC of the component if it matches the desired spec, or creates the desired PV/PVC.
// It never deletes anything. The application name is read from the radix-app label of the namespace of the RadixDeployment.
func EnsureComponentVolumes(ctx context.Context, kubeUtil *kube.Kube, rd *radixv1.RadixDeployment, component radixv1.RadixCommonDeployComponent) ([]corev1.Volume, []corev1.VolumeMount, error) {
	return getComponentVolumes(ctx, kubeUtil, rd, component, false)
}

// GarbageCollectVolumesNoLongerInSpec deletes the Radix owned PVCs and PVs in the namespace of the RadixDeployment
// that belong to components or jobs that are not in the RadixDeployment.
// The application name is read from the radix-app label of the namespace of the RadixDeployment.
func GarbageCollectVolumesNoLongerInSpec(ctx context.Context, kubeClient kubernetes.Interface, rd *radixv1.RadixDeployment) error {
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
	appName, err := kube.GetAppNameFromNamespaceWithKubeClient(ctx, namespace, kubeClient)
	if err != nil {
		return err
	}
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
	return deleteVolumeResources(ctx, kubeClient, pvcs, pvs)
}

func getComponentVolumes(ctx context.Context, kubeUtil *kube.Kube, rd *radixv1.RadixDeployment, component radixv1.RadixCommonDeployComponent, deleteUnused bool) ([]corev1.Volume, []corev1.VolumeMount, error) {
	claimNames, err := reconcileBlobFuse2Storage(ctx, kubeUtil.KubeClient(), rd, component, deleteUnused)
	if err != nil {
		return nil, nil, err
	}

	volumes, err := getVolumeMountVolumes(component, claimNames)
	if err != nil {
		return nil, nil, err
	}

	secretRefsVolumes, err := getComponentSecretRefsVolumes(ctx, kubeUtil, rd.GetNamespace(), component, rd.GetName())
	if err != nil {
		return nil, nil, err
	}
	volumes = append(volumes, secretRefsVolumes...)

	volumeMounts, err := GetRadixDeployComponentVolumeMounts(component, rd.GetName())
	if err != nil {
		return nil, nil, err
	}

	return volumes, volumeMounts, nil
}

// reconcileBlobFuse2Storage reuses or creates the PV/PVC for each BlobFuse2 volume mount of the component, and returns the PVC name by volume mount name.
// When deleteUnused is set, all other PVCs/PVs of the component are deleted, unless the PV/PVC of any volume mount could not be reused or created.
// The application name is read from the radix-app label of the namespace of the RadixDeployment.
func reconcileBlobFuse2Storage(ctx context.Context, kubeClient kubernetes.Interface, rd *radixv1.RadixDeployment, component radixv1.RadixCommonDeployComponent, deleteUnused bool) (map[string]string, error) {
	namespace := rd.GetNamespace()
	appName, err := kube.GetAppNameFromNamespaceWithKubeClient(ctx, namespace, kubeClient)
	if err != nil {
		return nil, err
	}
	componentSelector := kubelabels.Set{kube.RadixComponentLabel: component.GetName()}

	currentPvcs, err := listRadixPvcs(ctx, kubeClient, appName, namespace, componentSelector)
	if err != nil {
		return nil, err
	}
	currentPvcsByVolumeMount := make(map[string][]corev1.PersistentVolumeClaim)
	for _, pvc := range currentPvcs {
		name := pvc.GetLabels()[kube.RadixVolumeMountNameLabel]
		currentPvcsByVolumeMount[name] = append(currentPvcsByVolumeMount[name], pvc)
	}

	usedPvcs := make(map[string]*corev1.PersistentVolumeClaim)
	var errs []error
	for _, radixVolumeMount := range component.GetVolumeMounts() {
		if !radixVolumeMount.HasBlobFuse2() {
			continue
		}
		pvc, err := ensurePersistentVolumeClaim(ctx, kubeClient, appName, rd, component, radixVolumeMount, currentPvcsByVolumeMount[radixVolumeMount.Name])
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to ensure storage for volume mount %s in component %s: %w", radixVolumeMount.Name, component.GetName(), err))
			continue
		}
		usedPvcs[radixVolumeMount.Name] = pvc
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	if deleteUnused {
		currentPvs, err := listRadixPvs(ctx, kubeClient, appName, namespace, componentSelector)
		if err != nil {
			return nil, err
		}
		if err := deleteUnusedVolumeResources(ctx, kubeClient, currentPvcs, currentPvs, usedPvcs); err != nil {
			return nil, fmt.Errorf("failed to delete unused storage for component %s: %w", component.GetName(), err)
		}
	}

	claimNames := make(map[string]string, len(usedPvcs))
	for volumeMountName, pvc := range usedPvcs {
		claimNames[volumeMountName] = pvc.GetName()
	}
	return claimNames, nil
}

// getVolumeMountVolumes returns the Pod volumes for the volume mounts of the component. claimNames has the PVC name by volume mount name for BlobFuse2 volume mounts.
func getVolumeMountVolumes(component radixv1.RadixCommonDeployComponent, claimNames map[string]string) ([]corev1.Volume, error) {
	var volumes []corev1.Volume
	for _, radixVolumeMount := range component.GetVolumeMounts() {
		volumeName, err := GetVolumeMountVolumeName(&radixVolumeMount, component.GetName())
		if err != nil {
			return nil, err
		}
		switch {
		case radixVolumeMount.HasBlobFuse2():
			volumes = append(volumes, corev1.Volume{
				Name:         volumeName,
				VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claimNames[radixVolumeMount.Name]}},
			})
		case radixVolumeMount.HasEmptyDir():
			volumes = append(volumes, corev1.Volume{
				Name:         volumeName,
				VolumeSource: *getComponentVolumeMountEmptyDirVolumeSource(radixVolumeMount.EmptyDir),
			})
		default:
			return nil, fmt.Errorf("missing configuration for volumeMount %s", radixVolumeMount.Name)
		}
	}
	return volumes, nil
}

// ensurePersistentVolumeClaim returns a current PVC of the volume mount that matches the desired spec, or creates the desired PV/PVC.
func ensurePersistentVolumeClaim(ctx context.Context, kubeClient kubernetes.Interface, appName string, rd *radixv1.RadixDeployment, component radixv1.RadixCommonDeployComponent, radixVolumeMount radixv1.RadixVolumeMount, currentPvcs []corev1.PersistentVolumeClaim) (*corev1.PersistentVolumeClaim, error) {
	desired, err := buildDesiredVolumeResources(appName, rd, component, radixVolumeMount)
	if err != nil {
		return nil, err
	}

	var matching []corev1.PersistentVolumeClaim
	for _, pvc := range currentPvcs {
		if pvc.GetDeletionTimestamp() != nil || pvc.Status.Phase == corev1.ClaimLost || len(pvc.Spec.VolumeName) == 0 {
			continue
		}
		pv, err := kubeClient.CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		// A PV with the uid of a deleted PVC with the same name in its claimRef never binds to the PVC
		if pv.GetDeletionTimestamp() != nil || hasStaleClaimRef(pv, &pvc) || !matchesDesired(pv, &pvc, desired.pv, desired.pvc) {
			continue
		}
		matching = append(matching, pvc)
	}

	if len(matching) > 0 {
		// Select deterministically, not by list order, so every sync and both the RadixDeployment and RadixBatch syncers pick the same PVC.
		// The oldest is most likely the one running workloads use; the others are deleted by ReconcileComponentVolumes.
		selected := selectOldestPersistentVolumeClaim(matching)
		return &selected, nil
	}

	if err := createDesiredVolumeResources(ctx, kubeClient, desired); err != nil {
		return nil, err
	}
	return desired.pvc, nil
}

// selectOldestPersistentVolumeClaim returns the oldest PVC by creationTimestamp, then by name
func selectOldestPersistentVolumeClaim(pvcs []corev1.PersistentVolumeClaim) corev1.PersistentVolumeClaim {
	return slices.MinFunc(pvcs, func(a, b corev1.PersistentVolumeClaim) int {
		if c := a.GetCreationTimestamp().Compare(b.GetCreationTimestamp().Time); c != 0 {
			return c
		}
		return strings.Compare(a.GetName(), b.GetName())
	})
}

// createDesiredVolumeResources creates the desired PV, then the PVC. The names are unique, so they do not conflict with existing resources.
// If the PVC cannot be created, the PV is left behind, and deleted when the component is reconciled.
func createDesiredVolumeResources(ctx context.Context, kubeClient kubernetes.Interface, desired *desiredVolumeResources) error {
	logger := log.Ctx(ctx)
	logger.Info().Msgf("Create PersistentVolume %s for PersistentVolumeClaim %s in namespace %s", desired.pv.GetName(), desired.pvc.GetName(), desired.pvc.GetNamespace())
	if _, err := kubeClient.CoreV1().PersistentVolumes().Create(ctx, desired.pv, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create PersistentVolume %s: %w", desired.pv.GetName(), err)
	}
	logger.Info().Msgf("Create PersistentVolumeClaim %s in namespace %s for PersistentVolume %s", desired.pvc.GetName(), desired.pvc.GetNamespace(), desired.pv.GetName())
	if _, err := kubeClient.CoreV1().PersistentVolumeClaims(desired.pvc.GetNamespace()).Create(ctx, desired.pvc, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create PersistentVolumeClaim %s: %w", desired.pvc.GetName(), err)
	}
	return nil
}

// hasStaleClaimRef returns true if the PV claimRef refers to a previous PVC with the same name as the PVC
func hasStaleClaimRef(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) bool {
	return pv.Spec.ClaimRef != nil && len(pv.Spec.ClaimRef.UID) > 0 && pv.Spec.ClaimRef.UID != pvc.GetUID()
}

// listRadixPvcs lists the PVCs in the namespace that are labelled for the app, have a volume mount name label, and match the extra labels
func listRadixPvcs(ctx context.Context, kubeClient kubernetes.Interface, appName, namespace string, extraLabels kubelabels.Set) ([]corev1.PersistentVolumeClaim, error) {
	selector := kubelabels.SelectorFromSet(kubelabels.Merge(extraLabels, kubelabels.Set{kube.RadixAppLabel: appName})).Add(volumeMountNameLabelExists())
	pvcList, err := kubeClient.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, fmt.Errorf("failed to list PersistentVolumeClaims: %w", err)
	}
	return pvcList.Items, nil
}

// listRadixPvs lists the PVs that are labelled for the app and the namespace, have a volume mount name label, and match the extra labels
func listRadixPvs(ctx context.Context, kubeClient kubernetes.Interface, appName, namespace string, extraLabels kubelabels.Set) ([]corev1.PersistentVolume, error) {
	selector := kubelabels.SelectorFromSet(kubelabels.Merge(extraLabels, kubelabels.Set{kube.RadixAppLabel: appName, kube.RadixNamespace: namespace})).Add(volumeMountNameLabelExists())
	pvList, err := kubeClient.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, fmt.Errorf("failed to list PersistentVolumes: %w", err)
	}
	return pvList.Items, nil
}

func volumeMountNameLabelExists() kubelabels.Requirement {
	requirement, err := kubelabels.NewRequirement(kube.RadixVolumeMountNameLabel, selection.Exists, nil)
	if err != nil {
		panic(err)
	}
	return *requirement
}

// deleteUnusedVolumeResources deletes the PVCs and PVs that are not used by any volume mount. usedPvcs has the used PVC by volume mount name.
func deleteUnusedVolumeResources(ctx context.Context, kubeClient kubernetes.Interface, pvcs []corev1.PersistentVolumeClaim, pvs []corev1.PersistentVolume, usedPvcs map[string]*corev1.PersistentVolumeClaim) error {
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
	return deleteVolumeResources(ctx, kubeClient, unusedPvcs, unusedPvs)
}

// deleteVolumeResources deletes the PVCs, then the PVs. Resources that are already being deleted are skipped.
func deleteVolumeResources(ctx context.Context, kubeClient kubernetes.Interface, pvcs []corev1.PersistentVolumeClaim, pvs []corev1.PersistentVolume) error {
	logger := log.Ctx(ctx)
	var errs []error
	for _, pvc := range pvcs {
		if pvc.GetDeletionTimestamp() != nil {
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
		logger.Info().Msgf("Delete PersistentVolume %s (phase %s)", pv.GetName(), pv.Status.Phase)
		if err := kubeClient.CoreV1().PersistentVolumes().Delete(ctx, pv.GetName(), deleteOptionsWithUIDPrecondition(&pv)); ignoreNotFoundOrConflict(err) != nil {
			errs = append(errs, fmt.Errorf("failed to delete PersistentVolume %s: %w", pv.GetName(), err))
		}
	}
	return errors.Join(errs...)
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

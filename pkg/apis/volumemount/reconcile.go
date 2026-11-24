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

// componentStorage gets the Pod volumes and container volume mounts of a component or job in a RadixDeployment,
// and reconciles the PVs/PVCs of its BlobFuse2 volume mounts.
// It holds the arguments of a single call, and must not be reused for another component or RadixDeployment.
type componentStorage struct {
	kubeUtil  *kube.Kube
	appName   string
	rd        *radixv1.RadixDeployment
	component radixv1.RadixCommonDeployComponent
}

// ReconcileComponentVolumes returns the Pod volumes and container volume mounts for a component or job in the RadixDeployment.
// For each BlobFuse2 volume mount it reuses the current PV/PVC of the component if it matches the desired spec, or creates the desired PV/PVC.
// All other PVs/PVCs of the component are deleted. Kubernetes keeps a PVC, and its PV, that is used by a Pod in Terminating state
// (kubernetes.io/pvc-protection and kubernetes.io/pv-protection) until the Pod is gone, so running workloads are not affected.
// Blob data is never deleted, as all PVs use the reclaim policy Retain.
func ReconcileComponentVolumes(ctx context.Context, kubeUtil *kube.Kube, appName string, rd *radixv1.RadixDeployment, component radixv1.RadixCommonDeployComponent) ([]corev1.Volume, []corev1.VolumeMount, error) {
	storage := componentStorage{kubeUtil: kubeUtil, appName: appName, rd: rd, component: component}
	return storage.getComponentVolumes(ctx, true)
}

// EnsureComponentVolumes returns the Pod volumes and container volume mounts for a component or job in the RadixDeployment.
// For each BlobFuse2 volume mount it reuses the current PV/PVC of the component if it matches the desired spec, or creates the desired PV/PVC.
// It never deletes anything.
func EnsureComponentVolumes(ctx context.Context, kubeUtil *kube.Kube, appName string, rd *radixv1.RadixDeployment, component radixv1.RadixCommonDeployComponent) ([]corev1.Volume, []corev1.VolumeMount, error) {
	storage := componentStorage{kubeUtil: kubeUtil, appName: appName, rd: rd, component: component}
	return storage.getComponentVolumes(ctx, false)
}

// getComponentVolumes returns the Pod volumes and container volume mounts of the component. When deleteUnused is set, unused PVCs/PVs of the component are deleted.
func (s *componentStorage) getComponentVolumes(ctx context.Context, deleteUnused bool) ([]corev1.Volume, []corev1.VolumeMount, error) {
	claimNames, err := s.reconcile(ctx, deleteUnused)
	if err != nil {
		return nil, nil, err
	}

	volumes, err := getVolumeMountVolumes(s.component, claimNames)
	if err != nil {
		return nil, nil, err
	}

	secretRefsVolumes, err := getComponentSecretRefsVolumes(ctx, s.kubeUtil, s.namespace(), s.component, s.rd.GetName())
	if err != nil {
		return nil, nil, err
	}
	volumes = append(volumes, secretRefsVolumes...)

	volumeMounts, err := getRadixDeployComponentVolumeMounts(s.component, s.rd.GetName())
	if err != nil {
		return nil, nil, err
	}

	return volumes, volumeMounts, nil
}

func (s *componentStorage) namespace() string {
	return s.rd.GetNamespace()
}

func (s *componentStorage) componentSelector() kubelabels.Set {
	return kubelabels.Set{kube.RadixComponentLabel: s.component.GetName()}
}

// reconcile reuses or creates the PV/PVC for each BlobFuse2 volume mount of the component, and returns the PVC name by volume mount name.
// When deleteUnused is set, all other PVCs/PVs of the component are deleted, unless the PV/PVC of any volume mount could not be reused or created.
func (s *componentStorage) reconcile(ctx context.Context, deleteUnused bool) (map[string]string, error) {
	currentPvcs, err := listRadixPvcs(ctx, s.kubeUtil.KubeClient(), s.appName, s.namespace(), s.componentSelector())
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
	for _, radixVolumeMount := range s.component.GetVolumeMounts() {
		if !radixVolumeMount.HasBlobFuse2() {
			continue
		}
		pvc, err := s.ensurePersistentVolumeClaim(ctx, radixVolumeMount, currentPvcsByVolumeMount[radixVolumeMount.Name])
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to ensure storage for volume mount %s in component %s: %w", radixVolumeMount.Name, s.component.GetName(), err))
			continue
		}
		usedPvcs[radixVolumeMount.Name] = pvc
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	if deleteUnused {
		currentPvs, err := listRadixPvs(ctx, s.kubeUtil.KubeClient(), s.appName, s.namespace(), s.componentSelector())
		if err != nil {
			return nil, err
		}
		if err := deleteUnusedVolumeResources(ctx, s.kubeUtil.KubeClient(), currentPvcs, currentPvs, usedPvcs); err != nil {
			return nil, fmt.Errorf("failed to delete unused storage for component %s: %w", s.component.GetName(), err)
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
		volumeName, err := getVolumeMountVolumeName(&radixVolumeMount, component.GetName())
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
func (s *componentStorage) ensurePersistentVolumeClaim(ctx context.Context, radixVolumeMount radixv1.RadixVolumeMount, currentPvcs []corev1.PersistentVolumeClaim) (*corev1.PersistentVolumeClaim, error) {
	desired, err := buildDesiredVolumeResources(s.appName, s.rd, s.component, radixVolumeMount)
	if err != nil {
		return nil, err
	}

	var matching []corev1.PersistentVolumeClaim
	for _, pvc := range currentPvcs {
		if pvc.GetDeletionTimestamp() != nil || pvc.Status.Phase == corev1.ClaimLost || len(pvc.Spec.VolumeName) == 0 {
			continue
		}
		pv, err := s.kubeUtil.KubeClient().CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
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

	if err := createDesiredVolumeResources(ctx, s.kubeUtil.KubeClient(), desired); err != nil {
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

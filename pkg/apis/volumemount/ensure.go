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
	"k8s.io/client-go/kubernetes"
)

// EnsureComponentVolumes returns the Pod volumes and container volume mounts for a component.
// For each BlobFuse2 volume mount it reuses an existing PV/PVC whose spec and labels match the
// desired spec, or creates a new pair named after a hash of that spec.
// It never deletes resources, and only updates a PV to clear a stale claimRef uid, so a recreated PVC can bind. currentVolumes is an optional hint: if one of its
// claims matches, it is preferred, so a workload that is already running keeps its claim.
func EnsureComponentVolumes(ctx context.Context, kubeUtil *kube.Kube, rd *radixv1.RadixDeployment, component radixv1.RadixCommonDeployComponent, currentVolumes []corev1.Volume) ([]corev1.Volume, []corev1.VolumeMount, error) {
	volumes, err := ensureVolumeMountVolumes(ctx, kubeUtil.KubeClient(), rd, component, currentVolumes)
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

func ensureVolumeMountVolumes(ctx context.Context, kubeClient kubernetes.Interface, rd *radixv1.RadixDeployment, component radixv1.RadixCommonDeployComponent, currentVolumes []corev1.Volume) ([]corev1.Volume, error) {
	currentClaimNames := getClaimNames(currentVolumes)
	candidates := &candidatePvcs{kubeClient: kubeClient, rd: rd, component: component}

	var volumes []corev1.Volume
	var errs []error
	for _, radixVolumeMount := range component.GetVolumeMounts() {
		volumeName, err := GetVolumeMountVolumeName(&radixVolumeMount, component.GetName())
		if err != nil {
			errs = append(errs, err)
			continue
		}

		switch {
		case radixVolumeMount.HasBlobFuse2():
			claimName, err := ensurePersistentVolumeClaim(ctx, kubeClient, rd, component, radixVolumeMount, candidates, currentClaimNames)
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to ensure storage for volume mount %s in component %s: %w", radixVolumeMount.Name, component.GetName(), err))
				continue
			}
			volumes = append(volumes, corev1.Volume{
				Name:         volumeName,
				VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claimName}},
			})
		case radixVolumeMount.HasEmptyDir():
			volumes = append(volumes, corev1.Volume{
				Name:         volumeName,
				VolumeSource: *getComponentVolumeMountEmptyDirVolumeSource(radixVolumeMount.EmptyDir),
			})
		default:
			errs = append(errs, fmt.Errorf("missing configuration for volumeMount %s", radixVolumeMount.Name))
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return volumes, nil
}

// ensurePersistentVolumeClaim returns the name of an existing PVC that matches the desired spec, or creates a new PV/PVC pair.
func ensurePersistentVolumeClaim(ctx context.Context, kubeClient kubernetes.Interface, rd *radixv1.RadixDeployment, component radixv1.RadixCommonDeployComponent, radixVolumeMount radixv1.RadixVolumeMount, candidates *candidatePvcs, currentClaimNames map[string]struct{}) (string, error) {
	desired, err := buildDesiredVolumeResources(rd, component, radixVolumeMount)
	if err != nil {
		return "", err
	}

	pvcs, err := candidates.forVolumeMount(ctx, radixVolumeMount.Name)
	if err != nil {
		return "", err
	}

	var matching []corev1.PersistentVolumeClaim
	for _, pvc := range pvcs {
		if pvc.GetDeletionTimestamp() != nil || pvc.Status.Phase == corev1.ClaimLost || len(pvc.Spec.VolumeName) == 0 {
			continue
		}
		pv, err := kubeClient.CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				continue
			}
			return "", err
		}
		if pv.GetDeletionTimestamp() != nil {
			continue
		}
		if !matchesDesired(pv, &pvc, desired.pv, desired.pvc) {
			continue
		}
		if hasStaleClaimRef(pv, &pvc) {
			// The PV was bound to a deleted PVC with the same name, and the recreated PVC cannot bind until the claimRef uid is cleared
			if err := clearClaimRefUID(ctx, kubeClient, pv); err != nil {
				return "", err
			}
		}
		matching = append(matching, pvc)
	}

	if len(matching) > 0 {
		selected := selectPersistentVolumeClaim(matching, desired.pvc.GetName(), currentClaimNames)
		return selected.GetName(), nil
	}

	if err := createDesiredVolumeResources(ctx, kubeClient, desired); err != nil {
		return "", err
	}
	return desired.pvc.GetName(), nil
}

// selectPersistentVolumeClaim picks one of the matching PVCs. It prefers, in order:
// a claim used by the current workload, the hash-named claim, and the oldest claim (by creationTimestamp, then name).
func selectPersistentVolumeClaim(pvcs []corev1.PersistentVolumeClaim, desiredPvcName string, currentClaimNames map[string]struct{}) corev1.PersistentVolumeClaim {
	rank := func(pvc corev1.PersistentVolumeClaim) int {
		if _, ok := currentClaimNames[pvc.GetName()]; ok {
			return 0
		}
		if pvc.GetName() == desiredPvcName {
			return 1
		}
		return 2
	}
	return slices.MinFunc(pvcs, func(a, b corev1.PersistentVolumeClaim) int {
		if c := rank(a) - rank(b); c != 0 {
			return c
		}
		if c := a.GetCreationTimestamp().Compare(b.GetCreationTimestamp().Time); c != 0 {
			return c
		}
		return strings.Compare(a.GetName(), b.GetName())
	})
}

// createDesiredVolumeResources creates the desired PV, then the PVC. AlreadyExists is treated as success,
// because the names are derived from the desired spec, so an existing resource with the same name has the same spec.
func createDesiredVolumeResources(ctx context.Context, kubeClient kubernetes.Interface, desired *desiredVolumeResources) error {
	logger := log.Ctx(ctx)
	logger.Info().Msgf("Create PersistentVolume %s for PersistentVolumeClaim %s in namespace %s", desired.pv.GetName(), desired.pvc.GetName(), desired.pvc.GetNamespace())
	if _, err := kubeClient.CoreV1().PersistentVolumes().Create(ctx, desired.pv, metav1.CreateOptions{}); err != nil {
		if !k8serrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create PersistentVolume %s: %w", desired.pv.GetName(), err)
		}
		if err := releaseClaimRefFromReleasedPersistentVolume(ctx, kubeClient, desired.pv.GetName(), desired.pvc); err != nil {
			return err
		}
	}

	logger.Info().Msgf("Create PersistentVolumeClaim %s in namespace %s for PersistentVolume %s", desired.pvc.GetName(), desired.pvc.GetNamespace(), desired.pv.GetName())
	if _, err := kubeClient.CoreV1().PersistentVolumeClaims(desired.pvc.GetNamespace()).Create(ctx, desired.pvc, metav1.CreateOptions{}); err != nil {
		if !k8serrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create PersistentVolumeClaim %s: %w", desired.pvc.GetName(), err)
		}
		existingPvc, err := kubeClient.CoreV1().PersistentVolumeClaims(desired.pvc.GetNamespace()).Get(ctx, desired.pvc.GetName(), metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get PersistentVolumeClaim %s: %w", desired.pvc.GetName(), err)
		}
		if existingPvc.GetDeletionTimestamp() != nil {
			return fmt.Errorf("PersistentVolumeClaim %s in namespace %s is being deleted", existingPvc.GetName(), existingPvc.GetNamespace())
		}
	}
	return nil
}

// releaseClaimRefFromReleasedPersistentVolume clears claimRef.uid and claimRef.resourceVersion of a Released PV,
// which was bound to a deleted PVC with the desired name, so a recreated PVC can bind to it. The PV spec is otherwise not changed.
func releaseClaimRefFromReleasedPersistentVolume(ctx context.Context, kubeClient kubernetes.Interface, pvName string, desiredPvc *corev1.PersistentVolumeClaim) error {
	pv, err := kubeClient.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get PersistentVolume %s: %w", pvName, err)
	}
	claimRef := pv.Spec.ClaimRef
	if pv.Status.Phase != corev1.VolumeReleased || claimRef == nil || claimRef.Namespace != desiredPvc.GetNamespace() || claimRef.Name != desiredPvc.GetName() {
		return nil
	}
	if len(claimRef.UID) == 0 && len(claimRef.ResourceVersion) == 0 {
		return nil
	}
	return clearClaimRefUID(ctx, kubeClient, pv)
}

// hasStaleClaimRef returns true if the PV claimRef refers to a previous PVC with the same name as the PVC
func hasStaleClaimRef(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) bool {
	return pv.Spec.ClaimRef != nil && len(pv.Spec.ClaimRef.UID) > 0 && pv.Spec.ClaimRef.UID != pvc.GetUID()
}

// clearClaimRefUID clears claimRef.uid and claimRef.resourceVersion of the PV, so a PVC with the claimRef name can bind to it.
func clearClaimRefUID(ctx context.Context, kubeClient kubernetes.Interface, pv *corev1.PersistentVolume) error {
	log.Ctx(ctx).Info().Msgf("Clear claimRef uid and resourceVersion in PersistentVolume %s, to allow PersistentVolumeClaim %s in namespace %s to bind", pv.GetName(), pv.Spec.ClaimRef.Name, pv.Spec.ClaimRef.Namespace)
	updatedPv := pv.DeepCopy()
	updatedPv.Spec.ClaimRef.UID = ""
	updatedPv.Spec.ClaimRef.ResourceVersion = ""
	if _, err := kubeClient.CoreV1().PersistentVolumes().Update(ctx, updatedPv, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to update PersistentVolume %s: %w", pv.GetName(), err)
	}
	return nil
}

// candidatePvcs lists, once, the PVCs labelled for the component, and groups them by volume mount name
type candidatePvcs struct {
	kubeClient kubernetes.Interface
	rd         *radixv1.RadixDeployment
	component  radixv1.RadixCommonDeployComponent
	byName     map[string][]corev1.PersistentVolumeClaim
}

func (c *candidatePvcs) forVolumeMount(ctx context.Context, volumeMountName string) ([]corev1.PersistentVolumeClaim, error) {
	if c.byName == nil {
		selector := kubelabels.SelectorFromSet(kubelabels.Set{
			kube.RadixAppLabel:       c.rd.Spec.AppName, //nolint:staticcheck
			kube.RadixComponentLabel: c.component.GetName(),
		})
		pvcList, err := c.kubeClient.CoreV1().PersistentVolumeClaims(c.rd.GetNamespace()).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
		if err != nil {
			return nil, fmt.Errorf("failed to list PersistentVolumeClaims: %w", err)
		}
		c.byName = make(map[string][]corev1.PersistentVolumeClaim)
		for _, pvc := range pvcList.Items {
			name := pvc.GetLabels()[kube.RadixVolumeMountNameLabel]
			c.byName[name] = append(c.byName[name], pvc)
		}
	}
	return c.byName[volumeMountName], nil
}

func getClaimNames(volumes []corev1.Volume) map[string]struct{} {
	claimNames := make(map[string]struct{})
	for _, volume := range volumes {
		if volume.PersistentVolumeClaim != nil && len(volume.PersistentVolumeClaim.ClaimName) > 0 {
			claimNames[volume.PersistentVolumeClaim.ClaimName] = struct{}{}
		}
	}
	return claimNames
}

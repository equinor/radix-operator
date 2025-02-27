package volumemount

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubelabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// CreateOrUpdateCsiAzureVolumeResourcesForDeployComponent Create or update CSI Azure volume resources for a DeployComponent - PersistentVolumes, PersistentVolumeClaims, PersistentVolume
// Returns actual volumes, with existing relevant PersistentVolumeClaimName and PersistentVolumeName
func CreateOrUpdateCsiAzureVolumeResourcesForDeployComponent(ctx context.Context, kubeClient kubernetes.Interface, radixDeployment *radixv1.RadixDeployment, namespace string, deployComponent radixv1.RadixCommonDeployComponent, desiredVolumes []corev1.Volume) ([]corev1.Volume, error) {
	handler := azureCSIBlobResourceHandler{
		kubeClient:      kubeClient,
		radixDeployment: radixDeployment,
		deployComponent: deployComponent,
	}
	actualVolumes, err := handler.SyncVolumeResources(ctx, desiredVolumes)
	if err != nil {
		return nil, fmt.Errorf("failed to sync volumes: %w", err)
	}
	return actualVolumes, nil
}

type azureCSIBlobResourceHandler struct {
	kubeClient      kubernetes.Interface
	radixDeployment *radixv1.RadixDeployment
	deployComponent radixv1.RadixCommonDeployComponent
}

func (h *azureCSIBlobResourceHandler) SyncVolumeResources(ctx context.Context, desiredVolumes []corev1.Volume) ([]corev1.Volume, error) {
	namespace := h.radixDeployment.GetNamespace()
	functionalPvList, err := getCsiAzurePvsForNamespace(ctx, h.kubeClient, namespace, true)
	if err != nil {
		return nil, err
	}
	pvcByNameMap, err := h.getComponentPvcByNameMap(ctx)
	if err != nil {
		return nil, err
	}
	var errs []error
	var volumes []corev1.Volume
	for _, volume := range desiredVolumes {
		if volume.PersistentVolumeClaim == nil {
			volumes = append(volumes, volume)
			continue
		}
		radixVolumeMount, ok := slice.FindFirst(h.deployComponent.GetVolumeMounts(), h.isRadixVolumeMountForVolume(volume))
		if !ok {
			continue
		}
		processedVolume, err := h.createOrUpdateCsiAzureVolumeResourcesForVolume(ctx, volume, radixVolumeMount, functionalPvList, pvcByNameMap)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if processedVolume != nil {
			volumes = append(volumes, *processedVolume)
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return volumes, nil
}

func (h *azureCSIBlobResourceHandler) isRadixVolumeMountForVolume(volume corev1.Volume) func(vm radixv1.RadixVolumeMount) bool {
	return func(vm radixv1.RadixVolumeMount) bool {
		if !vm.HasBlobFuse2() && !vm.HasDeprecatedVolume() {
			return false
		}
		volumeMountVolumeName, err := GetVolumeMountVolumeName(&vm, h.deployComponent.GetName())
		if err != nil { // TODO: We should do something with the error other than swallow it. Probably the GetVolumeMountVolumeName should not return an error
			return false
		}
		return volumeMountVolumeName == volume.Name
	}
}

func (h *azureCSIBlobResourceHandler) getComponentPvcByNameMap(ctx context.Context) (map[string]*corev1.PersistentVolumeClaim, error) {
	pvcList, err := h.kubeClient.CoreV1().PersistentVolumeClaims(h.radixDeployment.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	pvcs := slice.FindAll(pvcList.Items, func(pvc corev1.PersistentVolumeClaim) bool {
		return pvc.GetLabels()[kube.RadixComponentLabel] == h.deployComponent.GetName()
	})
	pvcMap := make(map[string]*corev1.PersistentVolumeClaim)
	for _, pvc := range pvcs {
		pvc := pvc
		pvcMap[pvc.Name] = &pvc
	}
	return pvcMap, nil
}

func (h *azureCSIBlobResourceHandler) matchComponentVolumeMountLabels(radixVolumeMount radixv1.RadixVolumeMount, pv corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) bool {
	appName := h.radixDeployment.Spec.AppName
	namespace := h.radixDeployment.Namespace
	componentName := h.deployComponent.GetName()
	expectedPvLabels := radixlabels.ForBlobCSIAzurePersistentVolume(appName, namespace, componentName, radixVolumeMount)
	if expectedPvLabels.AsSelector().Matches(kubelabels.Set(pv.Labels)) {
		return true
	}
	if pvc != nil {
		expectedPvcLabels := radixlabels.ForBlobCSIAzurePersistentVolumeClaim(appName, componentName, radixVolumeMount)
		return expectedPvcLabels.AsSelector().Matches(kubelabels.Set(pvc.Labels))
	}
	return false
}

func (h *azureCSIBlobResourceHandler) createOrUpdateCsiAzureVolumeResourcesForVolume(ctx context.Context, volume corev1.Volume, radixVolumeMount radixv1.RadixVolumeMount, persistentVolumes []corev1.PersistentVolume, pvcByNameMap map[string]*corev1.PersistentVolumeClaim) (*corev1.Volume, error) {
	if volume.PersistentVolumeClaim == nil {
		return &volume, nil
	}

	namespace := h.radixDeployment.Namespace
	componentName := h.deployComponent.GetName()
	pvcName := volume.PersistentVolumeClaim.ClaimName
	existingPvc := pvcByNameMap[pvcName]

	pvName := getCsiAzurePvName()
	existingPv, pvExists := slice.FindFirst(persistentVolumes, h.isPersistentVolumeWithClaimRefName(pvcName))
	if pvExists {
		radixVolumeMountPv := h.buildCsiAzurePv(existingPv.GetName(), pvcName, radixVolumeMount)
		if !h.matchComponentVolumeMountLabels(radixVolumeMount, existingPv, existingPvc) || !EqualPersistentVolumes(&existingPv, radixVolumeMountPv) {
			pvExists = false
			newPvcName, err := getCsiAzurePvcName(componentName, &radixVolumeMount)
			if err != nil {
				return nil, err
			}
			pvcName = newPvcName
		}
	} else {
		existingPv, pvExists = h.getCsiAzureComponentPvByRadixVolumeMountContent(radixVolumeMount, persistentVolumes, pvcName, existingPvc)
	}
	if pvExists {
		pvName = existingPv.GetName()
		pvcName = existingPv.Spec.ClaimRef.Name
	}

	existingPvc, pvcExists := pvcByNameMap[pvcName]
	if !pvExists && pvcExists {
		pvName = existingPvc.Spec.VolumeName
		if len(pvName) == 0 {
			pvName = getCsiAzurePvName() // for auto-provisioned persistent volume
		}
	}
	needToCreatePvc := !pvcExists
	needToReCreatePv := pvExists && !pvcExists && len(existingPv.Spec.StorageClassName) > 0 // HACK: always re-create PV if it uses SC and PVC is missing
	radixVolumeMountPvc := h.buildPvc(pvName, pvcName, &radixVolumeMount)
	needToReCreatePvc := pvcExists && (existingPvc.Spec.VolumeName != pvName || !EqualPersistentVolumeClaims(existingPvc, radixVolumeMountPvc))

	if needToReCreatePv || needToReCreatePvc {
		newPvcName, err := getCsiAzurePvcName(componentName, &radixVolumeMount)
		if err != nil {
			return nil, err
		}
		pvcName = newPvcName

		pvExists = false
		pvName = getCsiAzurePvName()
	}

	if !pvExists || needToReCreatePv {
		log.Ctx(ctx).Debug().Msgf("Create PersistentVolume %s in namespace %s", pvName, namespace)
		pv := h.buildCsiAzurePv(pvName, pvcName, radixVolumeMount)
		if _, err := h.kubeClient.CoreV1().PersistentVolumes().Create(ctx, pv, metav1.CreateOptions{}); err != nil {
			return nil, err
		}
	}

	if needToCreatePvc || needToReCreatePvc {
		radixVolumeMountPvc.SetName(pvcName)
		radixVolumeMountPvc.Spec.VolumeName = pvName
		log.Ctx(ctx).Debug().Msgf("Create PersistentVolumeClaim %s in namespace %s for PersistentVolume %s", radixVolumeMountPvc.GetName(), namespace, pvName)
		if _, err := h.kubeClient.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, radixVolumeMountPvc, metav1.CreateOptions{}); err != nil {
			return nil, err
		}
	}
	volume.PersistentVolumeClaim.ClaimName = pvcName // in case it was updated with new name
	return &volume, nil
}

func (h *azureCSIBlobResourceHandler) buildPvc(pvName, pvcName string, radixVolumeMount *radixv1.RadixVolumeMount) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: h.radixDeployment.Namespace,
			Labels:    radixlabels.ForBlobCSIAzurePersistentVolumeClaim(h.radixDeployment.Spec.AppName, h.deployComponent.GetName(), *radixVolumeMount),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{h.getVolumeMountAccessMode(radixVolumeMount)},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: h.getVolumeCapacity(radixVolumeMount)},
			},
			VolumeName:       pvName,
			StorageClassName: pointers.Ptr(""), // use "" to avoid to use the "default" storage class
			VolumeMode:       pointers.Ptr(corev1.PersistentVolumeFilesystem),
		},
	}
}

func (h *azureCSIBlobResourceHandler) getCsiAzureComponentPvByRadixVolumeMountContent(radixVolumeMount radixv1.RadixVolumeMount, persistentVolumes []corev1.PersistentVolume, pvcName string, pvc *corev1.PersistentVolumeClaim) (corev1.PersistentVolume, bool) {
	for _, pv := range persistentVolumes {
		if pv.Spec.PersistentVolumeSource.CSI == nil || pv.Spec.ClaimRef == nil || !h.matchComponentVolumeMountLabels(radixVolumeMount, pv, pvc) {
			continue
		}
		radixVolumeMountPv := h.buildCsiAzurePv(pv.GetName(), pvcName, radixVolumeMount)
		if EqualPersistentVolumes(&pv, radixVolumeMountPv) {
			return pv, true
		}
	}
	return corev1.PersistentVolume{}, false
}

func (h *azureCSIBlobResourceHandler) buildCsiAzurePv(pvName, pvcName string, radixVolumeMount radixv1.RadixVolumeMount) *corev1.PersistentVolume {
	namespace := h.radixDeployment.Namespace
	componentName := h.deployComponent.GetName()

	pv := corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:   pvName,
			Labels: radixlabels.ForBlobCSIAzurePersistentVolume(h.radixDeployment.Spec.AppName, namespace, componentName, radixVolumeMount),
		},
		Spec: corev1.PersistentVolumeSpec{
			StorageClassName: "",
			MountOptions:     h.getCsiAzurePvMountOptionsForAzureBlob(&radixVolumeMount),
			Capacity:         corev1.ResourceList{corev1.ResourceStorage: h.getVolumeCapacity(&radixVolumeMount)},
			AccessModes:      []corev1.PersistentVolumeAccessMode{h.getVolumeMountAccessMode(&radixVolumeMount)},
			ClaimRef: &corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       k8s.KindPersistentVolumeClaim,
				Namespace:  namespace,
				Name:       pvcName,
			},
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:           provisionerBlobCsiAzure,
					VolumeHandle:     getVolumeHandle(namespace, componentName, pvName, radixVolumeMount.GetStorageContainerName()),
					VolumeAttributes: h.getCsiAzurePvAttributes(&radixVolumeMount),
				},
			},
		},
	}
	if !radixVolumeMount.UseAzureIdentity() {
		csiVolumeCredSecretName := defaults.GetCsiAzureVolumeMountCredsSecretName(componentName, radixVolumeMount.Name)
		pv.Spec.CSI.NodeStageSecretRef = &corev1.SecretReference{Name: csiVolumeCredSecretName, Namespace: namespace}
	}
	pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain // Using only PersistentVolumeReclaimRetain. PersistentVolumeReclaimPolicy deletes volume on unmount.
	return &pv
}

func (h *azureCSIBlobResourceHandler) getCsiAzurePvAttributes(radixVolumeMount *radixv1.RadixVolumeMount) map[string]string {
	namespace := h.radixDeployment.Namespace
	attributes := make(map[string]string)
	switch radixVolumeMount.GetVolumeMountType() {
	case radixv1.MountTypeBlobFuse2FuseCsiAzure:
		attributes[csiVolumeMountAttributeContainerName] = radixVolumeMount.GetStorageContainerName()
		attributes[csiVolumeMountAttributeProtocol] = csiVolumeAttributeProtocolParameterFuse
		attributes[csiVolumeMountAttributeSecretNamespace] = namespace
	case radixv1.MountTypeBlobFuse2Fuse2CsiAzure:
		attributes[csiVolumeMountAttributeContainerName] = radixVolumeMount.GetStorageContainerName()
		attributes[csiVolumeMountAttributeProtocol] = csiVolumeAttributeProtocolParameterFuse2
		if radixVolumeMount.BlobFuse2 != nil {
			if len(radixVolumeMount.BlobFuse2.StorageAccount) > 0 {
				attributes[csiVolumeAttributeStorageAccount] = radixVolumeMount.BlobFuse2.StorageAccount
			}
			if radixVolumeMount.UseAzureIdentity() {
				attributes[csiVolumeAttributeClientID] = h.deployComponent.GetIdentity().GetAzure().GetClientId()
				attributes[csiVolumeAttributeResourceGroup] = radixVolumeMount.BlobFuse2.ResourceGroup
				if len(radixVolumeMount.BlobFuse2.SubscriptionId) > 0 {
					attributes[csiVolumeAttributeSubscriptionId] = radixVolumeMount.BlobFuse2.SubscriptionId
				}
				if len(radixVolumeMount.BlobFuse2.TenantId) > 0 {
					attributes[csiVolumeAttributeTenantId] = radixVolumeMount.BlobFuse2.TenantId
				}
			} else {
				attributes[csiVolumeMountAttributeSecretNamespace] = namespace
			}
		}
	}
	// Do not specify the key storage.kubernetes.io/csiProvisionerIdentity in csi.volumeAttributes in PV specification. This key indicates dynamically provisioned PVs
	// https://github.com/kubernetes-csi/external-provisioner/blob/master/pkg/controller/controller.go#L289C5-L289C21
	// It looks like this: storage.kubernetes.io/csiProvisionerIdentity: 1731647415428-2825-blob.csi.azure.com
	return attributes
}

func (h *azureCSIBlobResourceHandler) isPersistentVolumeWithClaimRefName(claimRefName string) func(pv corev1.PersistentVolume) bool {
	return func(pv corev1.PersistentVolume) bool {
		return pv.Spec.PersistentVolumeSource.CSI != nil && pv.Spec.ClaimRef != nil && pv.Spec.ClaimRef.Name == claimRefName
	}
}

func (h *azureCSIBlobResourceHandler) getCsiAzurePvMountOptionsForAzureBlob(radixVolumeMount *radixv1.RadixVolumeMount) []string {
	mountOptions := []string{
		"--file-cache-timeout-in-seconds=120",
		"--use-attr-cache=true",
		"--cancel-list-on-mount-seconds=0",
		"-o allow_other",
		"-o attr_timeout=120",
		"-o entry_timeout=120",
		"-o negative_timeout=120",
	}

	if gid := radixVolumeMount.GetGID(); len(gid) > 0 {
		mountOptions = append(mountOptions, fmt.Sprintf("-o gid=%s", gid))
	} else {
		if uid := radixVolumeMount.GetUID(); len(uid) > 0 {
			mountOptions = append(mountOptions, fmt.Sprintf("-o uid=%s", uid))
		}
	}
	if h.getVolumeMountAccessMode(radixVolumeMount) == corev1.ReadOnlyMany {
		mountOptions = append(mountOptions, "--read-only=true")
	}
	if radixVolumeMount.BlobFuse2 != nil {
		mountOptions = append(mountOptions, h.getStreamingMountOptions(radixVolumeMount.BlobFuse2.StreamingOptions)...)
		mountOptions = append(mountOptions, fmt.Sprintf("--use-adls=%v", radixVolumeMount.BlobFuse2.UseAdls != nil && *radixVolumeMount.BlobFuse2.UseAdls))
	}
	return mountOptions
}

func (h *azureCSIBlobResourceHandler) getVolumeMountAccessMode(radixVolumeMount *radixv1.RadixVolumeMount) corev1.PersistentVolumeAccessMode {
	//nolint:staticcheck
	accessMode := radixVolumeMount.AccessMode
	if radixVolumeMount.BlobFuse2 != nil {
		accessMode = radixVolumeMount.BlobFuse2.AccessMode
	}
	switch strings.ToLower(accessMode) {
	case strings.ToLower(string(corev1.ReadWriteOnce)):
		return corev1.ReadWriteOnce
	case strings.ToLower(string(corev1.ReadWriteMany)):
		return corev1.ReadWriteMany
	case strings.ToLower(string(corev1.ReadWriteOncePod)):
		return corev1.ReadWriteOncePod
	}
	return corev1.ReadOnlyMany // default access mode
}

func (h *azureCSIBlobResourceHandler) getStreamingMountOptions(streaming *radixv1.BlobFuse2StreamingOptions) []string {
	var mountOptions []string
	if streaming != nil && streaming.Enabled != nil && !*streaming.Enabled {
		return nil
	}
	mountOptions = append(mountOptions, "--streaming=true")

	var streamCache uint64 = 750
	if streaming != nil && streaming.StreamCache != nil {
		streamCache = *streaming.StreamCache
	}
	mountOptions = append(mountOptions, fmt.Sprintf("--block-cache-pool-size=%v", streamCache))
	return mountOptions
}

func (h *azureCSIBlobResourceHandler) getVolumeCapacity(radixVolumeMount *radixv1.RadixVolumeMount) resource.Quantity {
	requestsVolumeMountSize, err := resource.ParseQuantity(h.getRadixBlobFuse2VolumeMountRequestsStorage(radixVolumeMount))
	if err != nil {
		return resource.MustParse("1Mi")
	}
	return requestsVolumeMountSize
}

func (h *azureCSIBlobResourceHandler) getRadixBlobFuse2VolumeMountRequestsStorage(radixVolumeMount *radixv1.RadixVolumeMount) string {
	if radixVolumeMount.BlobFuse2 != nil {
		return radixVolumeMount.BlobFuse2.RequestsStorage
	}
	//nolint:staticcheck
	return radixVolumeMount.RequestsStorage
}

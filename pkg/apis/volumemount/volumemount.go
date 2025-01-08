package volumemount

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	internal "github.com/equinor/radix-operator/pkg/apis/internal/deployment"
	"github.com/equinor/radix-operator/pkg/apis/internal/persistentvolume"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	csiVolumeTypeBlobFuse2ProtocolFuse      = "csi-az-blob"
	csiVolumeTypeBlobFuse2ProtocolFuse2     = "csi-blobfuse2-fuse2"
	csiVolumeNameTemplate                   = "%s-%s-%s-%s" // <radixvolumeid>-<componentname>-<radixvolumename>-<storage>
	csiAzureKeyVaultSecretMountPathTemplate = "/mnt/azure-key-vault/%s"

	volumeNameMaxLength = 63
)

// These are valid volume mount provisioners
const (
	// provisionerBlobCsiAzure Use of azure/csi driver for blob in Azure storage account
	provisionerBlobCsiAzure string = "blob.csi.azure.com"
)

var (
	csiVolumeProvisioners                 = map[string]any{provisionerBlobCsiAzure: struct{}{}}
	functionalPersistentVolumePhases      = []corev1.PersistentVolumePhase{corev1.VolumePending, corev1.VolumeBound, corev1.VolumeAvailable}
	functionalPersistentVolumeClaimPhases = []corev1.PersistentVolumeClaimPhase{corev1.ClaimPending, corev1.ClaimBound}
)

// GetRadixDeployComponentVolumeMounts Gets list of v1.VolumeMount for radixv1.RadixCommonDeployComponent
func GetRadixDeployComponentVolumeMounts(deployComponent radixv1.RadixCommonDeployComponent, radixDeploymentName string) ([]corev1.VolumeMount, error) {
	componentName := deployComponent.GetName()
	volumeMounts := make([]corev1.VolumeMount, 0)
	componentVolumeMounts, err := getRadixComponentVolumeMounts(deployComponent)
	if err != nil {
		return nil, err
	}
	volumeMounts = append(volumeMounts, componentVolumeMounts...)
	secretRefsVolumeMounts := getRadixComponentSecretRefsVolumeMounts(deployComponent, componentName, radixDeploymentName)
	volumeMounts = append(volumeMounts, secretRefsVolumeMounts...)
	return volumeMounts, nil
}

// GetVolumes Get volumes of a component by RadixVolumeMounts
func GetVolumes(ctx context.Context, kubeUtil *kube.Kube, namespace string, deployComponent radixv1.RadixCommonDeployComponent, radixDeploymentName string, existingVolumes []corev1.Volume) ([]corev1.Volume, error) {
	var volumes []corev1.Volume
	volumeMountVolumes, err := getComponentVolumeMountVolumes(deployComponent, existingVolumes)
	if err != nil {
		return nil, err
	}
	volumes = append(volumes, volumeMountVolumes...)

	storageRefsVolumes, err := getComponentSecretRefsVolumes(ctx, kubeUtil, namespace, deployComponent, radixDeploymentName)
	if err != nil {
		return nil, err
	}
	volumes = append(volumes, storageRefsVolumes...)

	return volumes, nil
}

// GarbageCollectVolumeMountsSecretsNoLongerInSpecForComponent Garbage collect volume-mount related secrets that are no longer in the spec
func GarbageCollectVolumeMountsSecretsNoLongerInSpecForComponent(ctx context.Context, kubeUtil *kube.Kube, namespace string, component radixv1.RadixCommonDeployComponent, excludeSecretNames []string) error {
	secrets, err := listSecretsForVolumeMounts(ctx, kubeUtil, namespace, component)
	if err != nil {
		return err
	}
	for _, secret := range secrets {
		if slice.Any(excludeSecretNames, func(s string) bool { return s == secret.Name }) {
			continue
		}
		if err := kubeUtil.DeleteSecret(ctx, namespace, secret.Name); err != nil && !k8serrors.IsNotFound(err) {
			return err
		}
	}

	return garbageCollectSecrets(ctx, kubeUtil, namespace, secrets, excludeSecretNames)
}

// CreateOrUpdateCsiAzureVolumeResources Create or update CSI Azure volume resources - PersistentVolumes, PersistentVolumeClaims, PersistentVolume
// Returns actual volumes, with existing relevant PersistentVolumeClaimName and PersistentVolumeName
func CreateOrUpdateCsiAzureVolumeResources(ctx context.Context, kubeClient kubernetes.Interface, radixDeployment *radixv1.RadixDeployment, namespace string, deployComponent radixv1.RadixCommonDeployComponent, desiredVolumes []corev1.Volume) ([]corev1.Volume, error) {
	componentName := deployComponent.GetName()
	actualVolumes, err := createOrUpdateCsiAzureVolumeResourcesForVolumes(ctx, kubeClient, radixDeployment, namespace, componentName, deployComponent.GetIdentity(), desiredVolumes)
	if err != nil {
		return nil, err
	}
	currentlyUsedPvcNames, err := getCurrentlyUsedPersistentVolumeClaims(ctx, kubeClient, radixDeployment, actualVolumes)
	if err != nil {
		return nil, err
	}
	if err = garbageCollectCsiAzurePersistentVolumeClaimsAndPersistentVolumes(ctx, kubeClient, namespace, componentName, currentlyUsedPvcNames); err != nil {
		return nil, err
	}
	return actualVolumes, garbageCollectOrphanedCsiAzurePersistentVolumes(ctx, kubeClient, currentlyUsedPvcNames)
}

// CreateOrUpdateVolumeMountSecrets creates or updates secrets for volume mounts
func CreateOrUpdateVolumeMountSecrets(ctx context.Context, kubeUtil *kube.Kube, appName, namespace, componentName string, volumeMounts []radixv1.RadixVolumeMount) ([]string, error) {
	var volumeMountSecretsToManage []string
	for _, volumeMount := range volumeMounts {
		secretName, accountKey, accountName := getCsiAzureVolumeMountCredsSecrets(ctx, kubeUtil, namespace, componentName, volumeMount.Name)
		volumeMountSecretsToManage = append(volumeMountSecretsToManage, secretName)
		err := createOrUpdateCsiAzureVolumeMountsSecrets(ctx, kubeUtil, appName, namespace, componentName, &volumeMount, secretName, accountName, accountKey)
		if err != nil {
			return nil, err
		}
	}
	return volumeMountSecretsToManage, nil
}

// GetCsiAzureVolumeMountType Gets the CSI Azure volume mount type
func GetCsiAzureVolumeMountType(radixVolumeMount *radixv1.RadixVolumeMount) radixv1.MountType {
	if radixVolumeMount.BlobFuse2 == nil {
		return radixVolumeMount.Type
	}
	switch radixVolumeMount.BlobFuse2.Protocol {
	case radixv1.BlobFuse2ProtocolFuse2, "": // default protocol if not set
		return radixv1.MountTypeBlobFuse2Fuse2CsiAzure
	default:
		return "unsupported"
	}
}

func getCsiPersistentVolumesForNamespace(ctx context.Context, kubeClient kubernetes.Interface, namespace string, onlyFunctional bool) ([]corev1.PersistentVolume, error) {
	pvList, err := kubeClient.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return slice.FindAll(pvList.Items, func(pv corev1.PersistentVolume) bool {
		return pvIsForCsiDriver(pv) && pvIsForNamespace(pv, namespace) && (!onlyFunctional || pvIsFunctional(pv))
	}), nil
}

func isKnownCsiAzureVolumeMount(volumeMount string) bool {
	switch volumeMount {
	case string(radixv1.MountTypeBlobFuse2FuseCsiAzure), string(radixv1.MountTypeBlobFuse2Fuse2CsiAzure):
		return true
	}
	return false
}

func getRadixComponentVolumeMounts(deployComponent radixv1.RadixCommonDeployComponent) ([]corev1.VolumeMount, error) {
	if internal.IsDeployComponentJobSchedulerDeployment(deployComponent) {
		return nil, nil
	}

	var volumeMounts []corev1.VolumeMount
	for _, volumeMount := range deployComponent.GetVolumeMounts() {
		name, err := GetVolumeMountVolumeName(&volumeMount, deployComponent.GetName())
		if err != nil {
			return nil, err
		}
		volumeMounts = append(volumeMounts, corev1.VolumeMount{Name: name, MountPath: volumeMount.Path})
	}
	return volumeMounts, nil
}

func getRadixComponentSecretRefsVolumeMounts(deployComponent radixv1.RadixCommonDeployComponent, componentName, radixDeploymentName string) []corev1.VolumeMount {
	secretRefs := deployComponent.GetSecretRefs()
	var volumeMounts []corev1.VolumeMount
	for _, azureKeyVault := range secretRefs.AzureKeyVaults {
		k8sSecretTypeMap := make(map[corev1.SecretType]bool)
		for _, keyVaultItem := range azureKeyVault.Items {
			kubeSecretType := kube.GetSecretTypeForRadixAzureKeyVault(keyVaultItem.K8sSecretType)
			if _, ok := k8sSecretTypeMap[kubeSecretType]; !ok {
				k8sSecretTypeMap[kubeSecretType] = true
			}
		}
		for kubeSecretType := range k8sSecretTypeMap {
			volumeMountName := trimVolumeNameToValidLength(kube.GetAzureKeyVaultSecretRefSecretName(componentName, radixDeploymentName, azureKeyVault.Name, kubeSecretType))
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      volumeMountName,
				ReadOnly:  true,
				MountPath: getCsiAzureKeyVaultSecretMountPath(azureKeyVault),
			})
		}
	}
	return volumeMounts
}

func getCsiAzureKeyVaultSecretMountPath(azureKeyVault radixv1.RadixAzureKeyVault) string {
	if azureKeyVault.Path == nil || *(azureKeyVault.Path) == "" {
		return fmt.Sprintf(csiAzureKeyVaultSecretMountPathTemplate, azureKeyVault.Name)
	}
	return *azureKeyVault.Path
}

func getCsiAzureVolumeMountName(componentName string, volumeMount *radixv1.RadixVolumeMount) (string, error) {
	// volumeName: <component-name>-<csi-volume-type-dashed>-<radix-volume-name>-<storage-name>
	csiVolumeType, err := getCsiRadixVolumeTypeId(volumeMount)
	if err != nil {
		return "", err
	}
	if len(volumeMount.Name) == 0 {
		return "", fmt.Errorf("name is empty for volume mount in the component %s", componentName)
	}
	csiAzureVolumeStorageName := getRadixVolumeMountStorage(volumeMount)
	if len(csiAzureVolumeStorageName) == 0 {
		return "", fmt.Errorf("storage is empty for volume mount %s in the component %s", volumeMount.Name, componentName)
	}
	if len(volumeMount.Path) == 0 {
		return "", fmt.Errorf("path is empty for volume mount %s in the component %s", volumeMount.Name, componentName)
	}
	return trimVolumeNameToValidLength(fmt.Sprintf(csiVolumeNameTemplate, csiVolumeType, componentName, volumeMount.Name, csiAzureVolumeStorageName)), nil
}

func getCsiRadixVolumeTypeId(radixVolumeMount *radixv1.RadixVolumeMount) (string, error) {
	if radixVolumeMount.BlobFuse2 != nil {
		switch radixVolumeMount.BlobFuse2.Protocol {
		case radixv1.BlobFuse2ProtocolFuse2, "":
			return csiVolumeTypeBlobFuse2ProtocolFuse2, nil
		default:
			return "", fmt.Errorf("unknown blobfuse2 protocol %s", radixVolumeMount.BlobFuse2.Protocol)
		}
	}
	if radixVolumeMount.Type == radixv1.MountTypeBlobFuse2FuseCsiAzure {
		return csiVolumeTypeBlobFuse2ProtocolFuse, nil
	}
	return "", fmt.Errorf("unknown volume mount type %s", radixVolumeMount.Type)
}

func getComponentSecretRefsVolumes(ctx context.Context, kubeUtil *kube.Kube, namespace string, deployComponent radixv1.RadixCommonDeployComponent, radixDeploymentName string) ([]corev1.Volume, error) {
	azureKeyVaultVolumes, err := getComponentSecretRefsAzureKeyVaultVolumes(ctx, kubeUtil, namespace, deployComponent, radixDeploymentName)
	if err != nil {
		return nil, err
	}
	return azureKeyVaultVolumes, nil
}

func getComponentSecretRefsAzureKeyVaultVolumes(ctx context.Context, kubeutil *kube.Kube, namespace string, deployComponent radixv1.RadixCommonDeployComponent, radixDeploymentName string) ([]corev1.Volume, error) {
	secretRef := deployComponent.GetSecretRefs()
	var volumes []corev1.Volume
	for _, azureKeyVault := range secretRef.AzureKeyVaults {
		secretProviderClassName := kube.GetComponentSecretProviderClassName(radixDeploymentName, deployComponent.GetName(), radixv1.RadixSecretRefTypeAzureKeyVault, azureKeyVault.Name)
		secretProviderClass, err := kubeutil.GetSecretProviderClass(ctx, namespace, secretProviderClassName)
		if err != nil {
			return nil, err
		}
		for _, secretObject := range secretProviderClass.Spec.SecretObjects {
			volumeName := trimVolumeNameToValidLength(secretObject.SecretName)
			volume := corev1.Volume{
				Name: volumeName,
			}
			provider := string(secretProviderClass.Spec.Provider)
			switch provider {
			case "azure":
				volume.VolumeSource.CSI = &corev1.CSIVolumeSource{
					Driver:           persistentvolume.CsiVolumeSourceDriverSecretStore,
					ReadOnly:         pointers.Ptr(true),
					VolumeAttributes: map[string]string{persistentvolume.CsiVolumeSourceVolumeAttributeSecretProviderClass: secretProviderClass.Name},
				}

				useAzureIdentity := azureKeyVault.UseAzureIdentity != nil && *azureKeyVault.UseAzureIdentity
				if !useAzureIdentity {
					azKeyVaultName, azKeyVaultNameExists := secretProviderClass.Spec.Parameters[defaults.CsiSecretProviderClassParameterKeyVaultName]
					if !azKeyVaultNameExists {
						return nil, fmt.Errorf("missing Azure Key vault name in the secret provider class %s", secretProviderClass.Name)
					}
					credsSecretName := defaults.GetCsiAzureKeyVaultCredsSecretName(deployComponent.GetName(), azKeyVaultName)
					volume.VolumeSource.CSI.NodePublishSecretRef = &corev1.LocalObjectReference{Name: credsSecretName}
				}
			default:
				log.Ctx(ctx).Error().Msgf("Not supported provider %s in the secret provider class %s", provider, secretProviderClass.Name)
				continue
			}
			volumes = append(volumes, volume)
		}
	}
	return volumes, nil
}

func getComponentVolumeMountVolumes(deployComponent radixv1.RadixCommonDeployComponent, existingVolumes []corev1.Volume) ([]corev1.Volume, error) {
	componentName := deployComponent.GetName()
	existingVolumeSourcesMap := getVolumesSourcesByVolumeNamesMap(existingVolumes)
	var volumes []corev1.Volume
	var errs []error
	for _, radixVolumeMount := range deployComponent.GetVolumeMounts() {
		volume, err := createVolume(radixVolumeMount, componentName, existingVolumeSourcesMap)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		volumes = append(volumes, *volume)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return volumes, nil
}

func createVolume(radixVolumeMount radixv1.RadixVolumeMount, componentName string, existingVolumeSourcesMap map[string]corev1.VolumeSource) (*corev1.Volume, error) {
	volumeName, err := GetVolumeMountVolumeName(&radixVolumeMount, componentName)
	if err != nil {
		return nil, err
	}
	volumeSource, err := getOrCreateVolumeSource(volumeName, componentName, radixVolumeMount, existingVolumeSourcesMap)
	if err != nil {
		return nil, err
	}
	return &corev1.Volume{
		Name:         volumeName,
		VolumeSource: *volumeSource,
	}, nil
}

func getOrCreateVolumeSource(volumeName string, componentName string, radixVolumeMount radixv1.RadixVolumeMount, existingVolumeSourcesMap map[string]corev1.VolumeSource) (*corev1.VolumeSource, error) {
	if existingVolumeSource, ok := existingVolumeSourcesMap[volumeName]; ok {
		return &existingVolumeSource, nil
	}
	return getVolumeSource(componentName, &radixVolumeMount)
}

func getVolumesSourcesByVolumeNamesMap(volumes []corev1.Volume) map[string]corev1.VolumeSource {
	return slice.Reduce(volumes, make(map[string]corev1.VolumeSource), func(acc map[string]corev1.VolumeSource, volume corev1.Volume) map[string]corev1.VolumeSource {
		if volume.PersistentVolumeClaim != nil {
			acc[volume.Name] = volume.VolumeSource
		}
		return acc
	})
}

func getVolumeSource(componentName string, volumeMount *radixv1.RadixVolumeMount) (*corev1.VolumeSource, error) {
	switch {
	case volumeMount.HasDeprecatedVolume():
		return getComponentVolumeMountDeprecatedVolumeSource(componentName, volumeMount)
	case volumeMount.HasBlobFuse2():
		return getCsiAzureVolumeSource(componentName, volumeMount)
	case volumeMount.HasEmptyDir():
		return getComponentVolumeMountEmptyDirVolumeSource(volumeMount.EmptyDir), nil
	}
	return nil, fmt.Errorf("missing configuration for volumeMount %s", volumeMount.Name)
}

func getCsiAzureVolumeSource(componentName string, radixVolumeMount *radixv1.RadixVolumeMount) (*corev1.VolumeSource, error) {
	pvcName, err := getCsiAzurePersistentVolumeClaimName(componentName, radixVolumeMount)
	if err != nil {
		return nil, err
	}
	return &corev1.VolumeSource{
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
			ClaimName: pvcName,
		},
	}, nil
}

func getComponentVolumeMountDeprecatedVolumeSource(componentName string, volumeMount *radixv1.RadixVolumeMount) (*corev1.VolumeSource, error) {
	if volumeMount.Type == radixv1.MountTypeBlobFuse2FuseCsiAzure {
		return getCsiAzureVolumeSource(componentName, volumeMount)
	}
	return nil, fmt.Errorf("unsupported volume type %s", volumeMount.Type)
}

func getComponentVolumeMountEmptyDirVolumeSource(spec *radixv1.RadixEmptyDirVolumeMount) *corev1.VolumeSource {
	return &corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{
			SizeLimit: &spec.SizeLimit,
		},
	}
}

// GetVolumeMountVolumeName Gets the volume name for a volume mount
func GetVolumeMountVolumeName(volumeMount *radixv1.RadixVolumeMount, componentName string) (string, error) {
	switch {
	case volumeMount.HasDeprecatedVolume():
		return getVolumeMountDeprecatedVolumeName(volumeMount, componentName)
	case volumeMount.HasBlobFuse2():
		return getCsiAzureVolumeMountName(componentName, volumeMount)
	}
	return fmt.Sprintf("radix-vm-%s", volumeMount.Name), nil
}

func getVolumeMountDeprecatedVolumeName(volumeMount *radixv1.RadixVolumeMount, componentName string) (string, error) {
	switch volumeMount.Type {
	case radixv1.MountTypeBlobFuse2FuseCsiAzure:
		return getCsiAzureVolumeMountName(componentName, volumeMount)
	}
	return "", fmt.Errorf("unsupported volume type %s", volumeMount.Type)
}

func getFunctionalPvcList(ctx context.Context, kubeclient kubernetes.Interface, namespace string, componentName string, radixVolumeMount *radixv1.RadixVolumeMount) ([]corev1.PersistentVolumeClaim, error) {
	pvcList, err := kubeclient.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: getLabelSelectorForCsiAzurePersistenceVolumeClaimForComponentVolumeMount(componentName, radixVolumeMount.Name),
	})
	if err != nil {
		return nil, err
	}
	existingPvcs := sortPvcsByCreatedTimestampDesc(pvcList.Items)
	return slice.FindAll(existingPvcs, func(pvc corev1.PersistentVolumeClaim) bool { return pvcIsFunctional(pvc) }), nil
}

func pvcIsFunctional(pvc corev1.PersistentVolumeClaim) bool {
	return slice.Any(functionalPersistentVolumeClaimPhases, func(phase corev1.PersistentVolumeClaimPhase) bool { return pvc.Status.Phase == phase })
}

func getCsiAzurePersistentVolumeClaimName(componentName string, radixVolumeMount *radixv1.RadixVolumeMount) (string, error) {
	volumeName, err := getCsiAzureVolumeMountName(componentName, radixVolumeMount)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(persistentvolume.CsiPersistentVolumeClaimNameTemplate, volumeName, strings.ToLower(commonUtils.RandString(5))), nil
}

func getCsiAzurePersistentVolumeName() string {
	return fmt.Sprintf(persistentvolume.CsiPersistentVolumeNameTemplate, uuid.New().String())
}

func createOrUpdateCsiAzureVolumeMountsSecrets(ctx context.Context, kubeUtil *kube.Kube, appName, namespace, componentName string, radixVolumeMount *radixv1.RadixVolumeMount, secretName string, accountName, accountKey []byte) error {
	secret := corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: map[string]string{
				kube.RadixAppLabel:             appName,
				kube.RadixComponentLabel:       componentName,
				kube.RadixMountTypeLabel:       string(GetCsiAzureVolumeMountType(radixVolumeMount)),
				kube.RadixVolumeMountNameLabel: radixVolumeMount.Name,
			},
		},
	}

	// Will need to set fake data in order to apply the secret. The user then need to set data to real values
	data := make(map[string][]byte)
	data[defaults.CsiAzureCredsAccountKeyPart] = accountKey
	data[defaults.CsiAzureCredsAccountNamePart] = accountName

	secret.Data = data

	_, err := kubeUtil.ApplySecret(ctx, namespace, &secret) //nolint:staticcheck // must be updated to use UpdateSecret or CreateSecret
	if err != nil {
		return err
	}

	return nil
}

func pvIsForCsiDriver(pv corev1.PersistentVolume) bool {
	return pv.Spec.CSI != nil
}

func pvIsForNamespace(pv corev1.PersistentVolume, namespace string) bool {
	return pv.Spec.ClaimRef != nil && pv.Spec.ClaimRef.Namespace == namespace
}

func pvIsFunctional(pv corev1.PersistentVolume) bool {
	return slice.Any(functionalPersistentVolumePhases, func(phase corev1.PersistentVolumePhase) bool { return pv.Status.Phase == phase })
}

func getCsiAzurePersistentVolumeClaims(ctx context.Context, kubeClient kubernetes.Interface, namespace, componentName string) (*corev1.PersistentVolumeClaimList, error) {
	return kubeClient.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: getLabelSelectorForCsiAzurePersistenceVolumeClaim(componentName),
	})
}

func getLabelSelectorForCsiAzurePersistenceVolumeClaim(componentName string) string {
	return fmt.Sprintf("%s=%s, %s in (%s, %s)", kube.RadixComponentLabel, componentName, kube.RadixMountTypeLabel, string(radixv1.MountTypeBlobFuse2FuseCsiAzure), string(radixv1.MountTypeBlobFuse2Fuse2CsiAzure))
}

func getLabelSelectorForCsiAzurePersistenceVolumeClaimForComponentVolumeMount(componentName, radixVolumeMountName string) string {
	return fmt.Sprintf("%s=%s, %s=%s", kube.RadixComponentLabel, componentName, kube.RadixVolumeMountNameLabel, radixVolumeMountName)
}

func buildPersistentVolumeClaim(appName, namespace, componentName, pvName string, radixVolumeMount *radixv1.RadixVolumeMount) (*corev1.PersistentVolumeClaim, error) {
	pvcName, err := getCsiAzurePersistentVolumeClaimName(componentName, radixVolumeMount)
	if err != nil {
		return nil, err
	}
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: namespace,
			Labels: map[string]string{
				kube.RadixAppLabel:             appName,
				kube.RadixComponentLabel:       componentName,
				kube.RadixMountTypeLabel:       string(GetCsiAzureVolumeMountType(radixVolumeMount)),
				kube.RadixVolumeMountNameLabel: radixVolumeMount.Name,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{getVolumeMountAccessMode(radixVolumeMount)},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: getVolumeCapacity(radixVolumeMount)},
			},
			VolumeName:       pvName,
			StorageClassName: pointers.Ptr(""), // use "" to avoid to use the "default" storage class
			VolumeMode:       pointers.Ptr(corev1.PersistentVolumeFilesystem),
		},
	}, nil
}

func getVolumeCapacity(radixVolumeMount *radixv1.RadixVolumeMount) resource.Quantity {
	requestsVolumeMountSize, err := resource.ParseQuantity(getRadixBlobFuse2VolumeMountRequestsStorage(radixVolumeMount))
	if err != nil {
		return resource.MustParse("1Mi")
	}
	return requestsVolumeMountSize
}

func populateCsiAzurePersistentVolume(persistentVolume *corev1.PersistentVolume, appName, namespace, componentName, pvName, pvcName string, radixVolumeMount *radixv1.RadixVolumeMount, identity *radixv1.Identity) *corev1.PersistentVolume {
	identityClientId := getIdentityClientId(identity)
	useAzureIdentity := getUseAzureIdentity(identity, radixVolumeMount.UseAzureIdentity)
	csiVolumeCredSecretName := defaults.GetCsiAzureVolumeMountCredsSecretName(componentName, radixVolumeMount.Name)
	persistentVolume.ObjectMeta.Name = pvName
	persistentVolume.ObjectMeta.Labels = getCsiAzurePersistentVolumeLabels(appName, namespace, componentName, radixVolumeMount)
	persistentVolume.ObjectMeta.Annotations = getCsiAzurePersistentVolumeAnnotations(csiVolumeCredSecretName, namespace, useAzureIdentity)
	persistentVolume.Spec.StorageClassName = ""
	persistentVolume.Spec.MountOptions = getCsiAzurePersistentVolumeMountOptionsForAzureBlob(radixVolumeMount)
	persistentVolume.Spec.Capacity = corev1.ResourceList{corev1.ResourceStorage: getVolumeCapacity(radixVolumeMount)}
	persistentVolume.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{getVolumeMountAccessMode(radixVolumeMount)}
	persistentVolume.Spec.ClaimRef = &corev1.ObjectReference{
		APIVersion: "v1",
		Kind:       k8s.KindPersistentVolumeClaim,
		Namespace:  namespace,
		Name:       pvcName,
	}
	persistentVolume.Spec.CSI = &corev1.CSIPersistentVolumeSource{
		Driver:           provisionerBlobCsiAzure,
		VolumeHandle:     getVolumeHandle(namespace, componentName, pvName, getRadixVolumeMountStorage(radixVolumeMount)),
		VolumeAttributes: getCsiAzurePersistentVolumeAttributes(namespace, radixVolumeMount, pvName, pvcName, useAzureIdentity, identityClientId),
	}
	if !useAzureIdentity {
		persistentVolume.Spec.CSI.NodeStageSecretRef = &corev1.SecretReference{Name: csiVolumeCredSecretName, Namespace: namespace}
	}
	persistentVolume.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain // Using only PersistentVolumeReclaimRetain. PersistentVolumeReclaimPolicy deletes volume on unmount.
	return persistentVolume
}

func getVolumeHandle(namespace, componentName, pvName, storageName string) string {
	// Specify a value the driver can use to uniquely identify the share in the cluster.
	// https://github.com/kubernetes-csi/csi-driver-smb/blob/master/docs/driver-parameters.md#pvpvc-usage
	return fmt.Sprintf("%s#%s#%s#%s", namespace, componentName, pvName, storageName)
}

func getUseAzureIdentity(identity *radixv1.Identity, useAzureIdentity *bool) bool {
	return len(getIdentityClientId(identity)) > 0 && useAzureIdentity != nil && *useAzureIdentity
}

func getIdentityClientId(identity *radixv1.Identity) string {
	if identity != nil && identity.Azure != nil && len(identity.Azure.ClientId) > 0 {
		return identity.Azure.ClientId
	}
	return ""
}

func getCsiAzurePersistentVolumeAnnotations(csiVolumeCredSecretName, namespace string, useAzureIdentity bool) map[string]string {
	annotationsMap := map[string]string{
		persistentvolume.CsiAnnotationProvisionedBy: provisionerBlobCsiAzure,
	}
	if !useAzureIdentity {
		annotationsMap[persistentvolume.CsiAnnotationProvisionerDeletionSecretName] = csiVolumeCredSecretName
		annotationsMap[persistentvolume.CsiAnnotationProvisionerDeletionSecretNamespace] = namespace
	}
	return annotationsMap
}

func getCsiAzurePersistentVolumeLabels(appName, namespace, componentName string, radixVolumeMount *radixv1.RadixVolumeMount) map[string]string {
	return map[string]string{
		kube.RadixAppLabel:             appName,
		kube.RadixNamespace:            namespace,
		kube.RadixComponentLabel:       componentName,
		kube.RadixVolumeMountNameLabel: radixVolumeMount.Name,
	}
}

func getCsiAzurePersistentVolumeAttributes(namespace string, radixVolumeMount *radixv1.RadixVolumeMount, pvName, pvcName string, useAzureIdentity bool, clientId string) map[string]string {
	attributes := make(map[string]string)
	switch GetCsiAzureVolumeMountType(radixVolumeMount) {
	case radixv1.MountTypeBlobFuse2FuseCsiAzure:
		attributes[persistentvolume.CsiVolumeMountAttributeContainerName] = getRadixBlobFuse2VolumeMountContainerName(radixVolumeMount)
		attributes[persistentvolume.CsiVolumeMountAttributeProtocol] = persistentvolume.CsiVolumeAttributeProtocolParameterFuse
	case radixv1.MountTypeBlobFuse2Fuse2CsiAzure:
		attributes[persistentvolume.CsiVolumeMountAttributeContainerName] = getRadixBlobFuse2VolumeMountContainerName(radixVolumeMount)
		attributes[persistentvolume.CsiVolumeMountAttributeProtocol] = persistentvolume.CsiVolumeAttributeProtocolParameterFuse2
		if len(radixVolumeMount.BlobFuse2.StorageAccount) > 0 {
			attributes[persistentvolume.CsiVolumeAttributeStorageAccount] = radixVolumeMount.BlobFuse2.StorageAccount
		}
		if useAzureIdentity {
			attributes[persistentvolume.CsiVolumeAttributeClientID] = clientId
			attributes[persistentvolume.CsiVolumeAttributeResourceGroup] = radixVolumeMount.BlobFuse2.ResourceGroup
		}
	}
	attributes[persistentvolume.CsiVolumeMountAttributePvName] = pvName
	attributes[persistentvolume.CsiVolumeMountAttributePvcName] = pvcName
	attributes[persistentvolume.CsiVolumeMountAttributePvcNamespace] = namespace
	if !useAzureIdentity {
		attributes[persistentvolume.CsiVolumeMountAttributeSecretNamespace] = namespace
	}
	// Do not specify the key storage.kubernetes.io/csiProvisionerIdentity in csi.volumeAttributes in PV specification. This key indicates dynamically provisioned PVs
	// https://github.com/kubernetes-csi/external-provisioner/blob/master/pkg/controller/controller.go#L289C5-L289C21
	// It looks like this: storage.kubernetes.io/csiProvisionerIdentity: 1731647415428-2825-blob.csi.azure.com
	return attributes
}

func getCsiAzurePersistentVolumeMountOptionsForAzureBlob(radixVolumeMount *radixv1.RadixVolumeMount) []string {
	mountOptions := []string{
		"--file-cache-timeout-in-seconds=120",
		"--use-attr-cache=true",
		"--cancel-list-on-mount-seconds=0",
		"-o allow_other",
		"-o attr_timeout=120",
		"-o entry_timeout=120",
		"-o negative_timeout=120",
	}
	gid := getRadixBlobFuse2VolumeMountGid(radixVolumeMount)
	if len(gid) > 0 {
		mountOptions = append(mountOptions, fmt.Sprintf("-o %s=%s", persistentvolume.CsiMountOptionGid, gid))
	} else {
		uid := getRadixBlobFuse2VolumeMountUid(radixVolumeMount)
		if len(uid) > 0 {
			mountOptions = append(mountOptions, fmt.Sprintf("-o %s=%s", persistentvolume.CsiMountOptionUid, uid))
		}
	}
	if getVolumeMountAccessMode(radixVolumeMount) == corev1.ReadOnlyMany {
		mountOptions = append(mountOptions, "-o ro")
	}
	if radixVolumeMount.BlobFuse2 != nil {
		mountOptions = append(mountOptions, getStreamingMountOptions(radixVolumeMount.BlobFuse2.Streaming)...)
		mountOptions = append(mountOptions, fmt.Sprintf("--%s=%v", persistentvolume.CsiMountOptionUseAdls, radixVolumeMount.BlobFuse2.UseAdls != nil && *radixVolumeMount.BlobFuse2.UseAdls))
	}
	return mountOptions
}

func getStreamingMountOptions(streaming *radixv1.RadixVolumeMountStreaming) []string {
	var mountOptions []string
	if streaming != nil && streaming.Enabled != nil && !*streaming.Enabled {
		return nil
	}
	mountOptions = append(mountOptions, fmt.Sprintf("--%s=%t", persistentvolume.CsiMountOptionStreamingEnabled, true))
	if streaming == nil {
		return mountOptions
	}
	if streaming.StreamCache != nil {
		mountOptions = append(mountOptions, fmt.Sprintf("--%s=%v", persistentvolume.CsiMountOptionStreamingCache, *streaming.StreamCache))
	}
	if streaming.BlockSize != nil {
		mountOptions = append(mountOptions, fmt.Sprintf("--%s=%v", persistentvolume.CsiMountOptionStreamingBlockSize, *streaming.BlockSize))
	}
	if streaming.BufferSize != nil {
		mountOptions = append(mountOptions, fmt.Sprintf("--%s=%v", persistentvolume.CsiMountOptionStreamingBufferSize, *streaming.BufferSize))
	}
	if streaming.MaxBuffers != nil {
		mountOptions = append(mountOptions, fmt.Sprintf("--%s=%v", persistentvolume.CsiMountOptionStreamingMaxBuffers, *streaming.MaxBuffers))
	}
	if streaming.MaxBlocksPerFile != nil {
		mountOptions = append(mountOptions, fmt.Sprintf("--%s=%v", persistentvolume.CsiMountOptionStreamingMaxBlocksPerFile, *streaming.MaxBlocksPerFile))
	}
	return mountOptions
}

func getVolumeMountAccessMode(radixVolumeMount *radixv1.RadixVolumeMount) corev1.PersistentVolumeAccessMode {
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

func getRadixBlobFuse2VolumeMountUid(radixVolumeMount *radixv1.RadixVolumeMount) string {
	if radixVolumeMount.BlobFuse2 != nil {
		return radixVolumeMount.BlobFuse2.UID
	}
	return radixVolumeMount.UID
}

func getRadixBlobFuse2VolumeMountGid(radixVolumeMount *radixv1.RadixVolumeMount) string {
	if radixVolumeMount.BlobFuse2 != nil {
		return radixVolumeMount.BlobFuse2.GID
	}
	return radixVolumeMount.GID
}

func getRadixBlobFuse2VolumeMountSkuName(radixVolumeMount *radixv1.RadixVolumeMount) string {
	if radixVolumeMount.BlobFuse2 != nil {
		return radixVolumeMount.BlobFuse2.SkuName
	}
	return radixVolumeMount.SkuName
}

func getRadixBlobFuse2VolumeMountContainerName(radixVolumeMount *radixv1.RadixVolumeMount) string {
	if radixVolumeMount.BlobFuse2 != nil {
		return radixVolumeMount.BlobFuse2.Container
	}
	return radixVolumeMount.Storage
}

func getRadixBlobFuse2VolumeMountRequestsStorage(radixVolumeMount *radixv1.RadixVolumeMount) string {
	if radixVolumeMount.BlobFuse2 != nil {
		return radixVolumeMount.BlobFuse2.RequestsStorage
	}
	return radixVolumeMount.RequestsStorage
}

func getRadixBlobFuse2VolumeMountBindingMode(radixVolumeMount *radixv1.RadixVolumeMount) string {
	if radixVolumeMount.BlobFuse2 != nil {
		return radixVolumeMount.BlobFuse2.BindingMode
	}
	return radixVolumeMount.BindingMode
}

func deletePersistentVolumeClaim(ctx context.Context, kubeClient kubernetes.Interface, namespace, pvcName string) error {
	if len(namespace) == 0 || len(pvcName) == 0 {
		log.Ctx(ctx).Debug().Msgf("Skip deleting PVC - namespace %s or name %s is empty", namespace, pvcName)
		return nil
	}
	if err := kubeClient.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, pvcName, metav1.DeleteOptions{}); err != nil { // && !k8serrors.IsNotFound(err) {
		return err
	}
	return nil
}

func deletePersistentVolume(ctx context.Context, kubeClient kubernetes.Interface, pvName string) error {
	if len(pvName) == 0 {
		log.Ctx(ctx).Debug().Msg("Skip deleting PersistentVolume - name is empty")
		return nil
	}
	if err := kubeClient.CoreV1().PersistentVolumes().Delete(ctx, pvName, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	return nil
}

func getRadixVolumeMountStorage(radixVolumeMount *radixv1.RadixVolumeMount) string {
	blobFuse2VolumeMountContainer := getRadixBlobFuse2VolumeMountContainerName(radixVolumeMount)
	if len(blobFuse2VolumeMountContainer) != 0 {
		return blobFuse2VolumeMountContainer
	}
	return radixVolumeMount.Storage
}

func garbageCollectOrphanedCsiAzurePersistentVolumes(ctx context.Context, kubeClient kubernetes.Interface, excludePvcNames map[string]any) error {
	pvList, err := kubeClient.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	var errs []error
	for _, pv := range pvList.Items {
		if pv.Spec.ClaimRef == nil || pv.Spec.ClaimRef.Kind != k8s.KindPersistentVolumeClaim || pv.Spec.CSI == nil || !knownCSIDriver(pv.Spec.CSI.Driver) {
			continue
		}
		if !(pv.Status.Phase == corev1.VolumeReleased || pv.Status.Phase == corev1.VolumeFailed) {
			continue
		}
		if _, ok := excludePvcNames[pv.Spec.ClaimRef.Name]; ok {
			continue
		}
		log.Ctx(ctx).Info().Msgf("Delete orphaned Csi Azure PersistantVolume %s of PersistantVolumeClaim %s", pv.Name, pv.Spec.ClaimRef.Name)
		if err := deletePersistentVolume(ctx, kubeClient, pv.Name); err != nil && !k8serrors.IsNotFound(err) {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func knownCSIDriver(driver string) bool {
	_, ok := csiVolumeProvisioners[driver]
	return ok
}

func createOrUpdateCsiAzureVolumeResourcesForVolumes(ctx context.Context, kubeClient kubernetes.Interface, radixDeployment *radixv1.RadixDeployment, namespace, componentName string, identity *radixv1.Identity, desiredVolumes []corev1.Volume) ([]corev1.Volume, error) {
	functionalPvList, err := getCsiPersistentVolumesForNamespace(ctx, kubeClient, namespace, true)
	if err != nil {
		return nil, err
	}
	pvcByNameMap, err := getPvcByNameMap(ctx, kubeClient, namespace, componentName)
	if err != nil {
		return nil, err
	}
	radixVolumeMountsByNameMap := getRadixVolumeMountsByNameMap(radixDeployment, componentName)
	var errs []error
	var volumes []corev1.Volume
	for _, volume := range desiredVolumes {
		if volume.PersistentVolumeClaim == nil {
			volumes = append(volumes, volume)
			continue
		}
		radixVolumeMount, existsRadixVolumeMount := radixVolumeMountsByNameMap[volume.Name]
		if !existsRadixVolumeMount {
			continue
		}
		processedVolume, err := createOrUpdateCsiAzureVolumeResourcesForVolume(ctx, kubeClient, radixDeployment, namespace, componentName, identity, volume, radixVolumeMount, functionalPvList, pvcByNameMap)
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

func createOrUpdateCsiAzureVolumeResourcesForVolume(ctx context.Context, kubeClient kubernetes.Interface, radixDeployment *radixv1.RadixDeployment, namespace string, componentName string, identity *radixv1.Identity, volume corev1.Volume, radixVolumeMount *radixv1.RadixVolumeMount, functionalPvList []corev1.PersistentVolume, pvcByNameMap map[string]*corev1.PersistentVolumeClaim) (*corev1.Volume, error) {
	if volume.PersistentVolumeClaim == nil {
		return &volume, nil
	}
	appName := radixDeployment.Spec.AppName
	pvcName := volume.PersistentVolumeClaim.ClaimName
	pvName, existsPvByPvcName, existsPvByVolumeContent := getActualExistingCsiAzurePvName(appName, namespace, componentName, radixVolumeMount, functionalPvList, pvcName, identity)
	if !existsPvByPvcName && !existsPvByVolumeContent {
		pvName = getCsiAzurePersistentVolumeName()
	}
	existingPvc, pvcExist := pvcByNameMap[pvcName]
	newPvc, err := buildPersistentVolumeClaim(appName, namespace, componentName, pvName, radixVolumeMount)
	if err != nil {
		return nil, err
	}
	if pvcExist &&
		(!persistentvolume.EqualPersistentVolumeClaims(existingPvc, newPvc) || existingPvc.Spec.VolumeName != pvName) {
		pvcName = newPvc.GetName()
	}
	if !existsPvByPvcName && !existsPvByVolumeContent {
		log.Ctx(ctx).Debug().Msgf("Create PersistentVolume %s in namespace %s", pvName, namespace)
		pv := populateCsiAzurePersistentVolume(&corev1.PersistentVolume{}, appName, namespace, componentName, pvName, pvcName, radixVolumeMount, identity)
		if _, err = kubeClient.CoreV1().PersistentVolumes().Create(ctx, pv, metav1.CreateOptions{}); err != nil {
			return nil, err
		}
	}
	if !pvcExist || !persistentvolume.EqualPersistentVolumeClaims(existingPvc, newPvc) {
		newPvc.SetName(pvcName)
		log.Ctx(ctx).Debug().Msgf("Create PersistentVolumeClaim %s in namespace %s for PersistentVolume %s", newPvc.GetName(), namespace, pvName)
		if _, err := kubeClient.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, newPvc, metav1.CreateOptions{}); err != nil {
			return nil, err
		}
	}
	volume.PersistentVolumeClaim.ClaimName = pvcName // in case it was updated with new name
	return &volume, nil
}

func getPvcByNameMap(ctx context.Context, kubeClient kubernetes.Interface, namespace string, componentName string) (map[string]*corev1.PersistentVolumeClaim, error) {
	pvcList, err := getCsiAzurePersistentVolumeClaims(ctx, kubeClient, namespace, componentName)
	if err != nil {
		return nil, err
	}
	return persistentvolume.GetPersistentVolumeClaimMap(&pvcList.Items), nil
}

func getCurrentlyUsedPersistentVolumeClaims(ctx context.Context, kubeClient kubernetes.Interface, radixDeployment *radixv1.RadixDeployment, volumes []corev1.Volume) (map[string]any, error) {
	namespace := radixDeployment.GetNamespace()
	pvcNames := make(map[string]any)
	deploymentList, err := kubeClient.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, deployment := range deploymentList.Items {
		pvcNames = appendUsedPersistenceVolumeClaimsFrom(pvcNames, deployment.Spec.Template.Spec.Volumes)
	}
	jobsList, err := kubeClient.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, job := range jobsList.Items {
		pvcNames = appendUsedPersistenceVolumeClaimsFrom(pvcNames, job.Spec.Template.Spec.Volumes)
	}
	// TODO add from RadixDeployments, connected to existing scheduled jobs and batches
	return appendUsedPersistenceVolumeClaimsFrom(pvcNames, volumes), nil
}

func appendUsedPersistenceVolumeClaimsFrom(pvcMap map[string]any, volumes []corev1.Volume) map[string]any {
	return slice.Reduce(volumes, pvcMap, func(acc map[string]any, volume corev1.Volume) map[string]any {
		if volume.PersistentVolumeClaim != nil && len(volume.PersistentVolumeClaim.ClaimName) > 0 {
			acc[volume.PersistentVolumeClaim.ClaimName] = struct{}{}
		}
		return acc
	})
}

func garbageCollectCsiAzurePersistentVolumeClaimsAndPersistentVolumes(ctx context.Context, kubeClient kubernetes.Interface, namespace, componentName string, excludePvcNames map[string]any) error {
	pvcList, err := getCsiAzurePersistentVolumeClaims(ctx, kubeClient, namespace, componentName)
	if err != nil {
		return err
	}
	var errs []error
	for _, pvc := range pvcList.Items {
		if _, ok := excludePvcNames[pvc.Name]; ok {
			continue
		}
		log.Ctx(ctx).Debug().Msgf("Delete not used CSI Azure PersistentVolumeClaim %s in namespace %s", pvc.Name, namespace)
		if err := deletePersistentVolumeClaim(ctx, kubeClient, namespace, pvc.Name); err != nil {
			errs = append(errs, err)
			continue
		}
		pvName := pvc.Spec.VolumeName
		log.Ctx(ctx).Debug().Msgf("Delete not used CSI Azure PersistentVolume %s in namespace %s", pvName, namespace)
		if err := deletePersistentVolume(ctx, kubeClient, pvName); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func getActualExistingCsiAzurePvName(appName, namespace, componentName string, radixVolumeMount *radixv1.RadixVolumeMount, persistentVolumes []corev1.PersistentVolume, pvcName string, identity *radixv1.Identity) (string, bool, bool) {
	if pvName, ok := getActualCsiAzurePvNameByPvcName(appName, namespace, componentName, radixVolumeMount, persistentVolumes, pvcName, identity); ok {
		return pvName, true, true
	}
	if pvName, ok := getFirstActualCsiAzurePvName(appName, namespace, componentName, radixVolumeMount, persistentVolumes, pvcName, identity); ok {
		return pvName, false, true
	}
	return "", false, false
}

func getFirstActualCsiAzurePvName(appName string, namespace string, componentName string, radixVolumeMount *radixv1.RadixVolumeMount, persistentVolumes []corev1.PersistentVolume, pvcName string, identity *radixv1.Identity) (string, bool) {
	for _, pv := range persistentVolumes {
		if pv.Spec.ClaimRef == nil {
			continue
		}
		desiredPv := populateCsiAzurePersistentVolume(pv.DeepCopy(), appName, namespace, componentName, pv.GetName(), pvcName, radixVolumeMount, identity)
		if persistentvolume.EqualPersistentVolumes(&pv, desiredPv) {
			return pv.GetName(), true
		}
	}
	return "", false
}

func getActualCsiAzurePvNameByPvcName(appName string, namespace string, componentName string, radixVolumeMount *radixv1.RadixVolumeMount, persistentVolumes []corev1.PersistentVolume, pvcName string, identity *radixv1.Identity) (string, bool) {
	if pv, ok := slice.FindFirst(persistentVolumes, func(pv corev1.PersistentVolume) bool {
		return pv.Spec.ClaimRef != nil && pv.Spec.ClaimRef.Name == pvcName
	}); ok {
		desiredPv := populateCsiAzurePersistentVolume(pv.DeepCopy(), appName, namespace, componentName, pv.GetName(), pvcName, radixVolumeMount, identity)
		if persistentvolume.EqualPersistentVolumes(&pv, desiredPv) {
			return pv.GetName(), true
		}
	}
	return "", false
}

func getRadixVolumeMountsByNameMap(radixDeployment *radixv1.RadixDeployment, componentName string) map[string]*radixv1.RadixVolumeMount {
	volumeMountsByNameMap := make(map[string]*radixv1.RadixVolumeMount)
	for _, component := range radixDeployment.Spec.Components {
		if findCsiAzureVolumeForComponent(volumeMountsByNameMap, component.VolumeMounts, componentName, &component) {
			break
		}
	}
	for _, component := range radixDeployment.Spec.Jobs {
		if findCsiAzureVolumeForComponent(volumeMountsByNameMap, component.VolumeMounts, componentName, &component) {
			break
		}
	}
	return volumeMountsByNameMap
}

func findCsiAzureVolumeForComponent(volumeMountsByNameMap map[string]*radixv1.RadixVolumeMount, volumeMounts []radixv1.RadixVolumeMount, componentName string, component radixv1.RadixCommonDeployComponent) bool {
	if !strings.EqualFold(componentName, component.GetName()) {
		return false
	}
	for _, radixVolumeMount := range volumeMounts {
		if radixVolumeMount.BlobFuse2 == nil && !isKnownCsiAzureVolumeMount(string(GetCsiAzureVolumeMountType(&radixVolumeMount))) {
			continue
		}
		radixVolumeMount := radixVolumeMount
		volumeMountName, err := getCsiAzureVolumeMountName(componentName, &radixVolumeMount)
		if err != nil {
			return false
		}
		volumeMountsByNameMap[volumeMountName] = &radixVolumeMount
	}
	return true
}

func sortPvcsByCreatedTimestampDesc(persistentVolumeClaims []corev1.PersistentVolumeClaim) []corev1.PersistentVolumeClaim {
	sort.SliceStable(persistentVolumeClaims, func(i, j int) bool {
		return (persistentVolumeClaims)[j].ObjectMeta.CreationTimestamp.Before(&(persistentVolumeClaims)[i].ObjectMeta.CreationTimestamp)
	})
	return persistentVolumeClaims
}

func trimVolumeNameToValidLength(volumeName string) string {
	const randSize = 5
	if len(volumeName) <= volumeNameMaxLength {
		return volumeName
	}

	randString := strings.ToLower(commonUtils.RandStringStrSeed(randSize, volumeName))
	sprintf := fmt.Sprintf("%s-%s", volumeName[:63-randSize-1], randString)
	return sprintf
}

func getCsiAzureVolumeMountCredsSecrets(ctx context.Context, kubeUtil *kube.Kube, namespace, componentName, volumeMountName string) (string, []byte, []byte) {
	secretName := defaults.GetCsiAzureVolumeMountCredsSecretName(componentName, volumeMountName)
	accountKey := []byte(defaults.SecretDefaultData)
	accountName := []byte(defaults.SecretDefaultData)
	if kubeUtil.SecretExists(ctx, namespace, secretName) {
		oldSecret, _ := kubeUtil.GetSecret(ctx, namespace, secretName)
		accountKey = oldSecret.Data[defaults.CsiAzureCredsAccountKeyPart]
		accountName = oldSecret.Data[defaults.CsiAzureCredsAccountNamePart]
	}
	return secretName, accountKey, accountName
}

func getLabelSelectorForCsiAzureVolumeMountSecret(component radixv1.RadixCommonDeployComponent) string {
	return fmt.Sprintf("%s=%s, %s in (%s, %s)", kube.RadixComponentLabel, component.GetName(), kube.RadixMountTypeLabel, string(radixv1.MountTypeBlobFuse2FuseCsiAzure), string(radixv1.MountTypeBlobFuse2Fuse2CsiAzure))
}

func listSecretsForVolumeMounts(ctx context.Context, kubeUtil *kube.Kube, namespace string, component radixv1.RadixCommonDeployComponent) ([]*corev1.Secret, error) {
	csiAzureVolumeMountSecret := getLabelSelectorForCsiAzureVolumeMountSecret(component)
	csiSecrets, err := kubeUtil.ListSecretsWithSelector(ctx, namespace, csiAzureVolumeMountSecret)
	if err != nil {
		return nil, err
	}
	return csiSecrets, nil
}

func garbageCollectSecrets(ctx context.Context, kubeUtil *kube.Kube, namespace string, secrets []*corev1.Secret, excludeSecretNames []string) error {
	for _, secret := range secrets {
		if slice.Any(excludeSecretNames, func(s string) bool { return s == secret.Name }) {
			continue
		}
		if err := kubeUtil.DeleteSecret(ctx, namespace, secret.GetName()); err != nil && !k8serrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// GetExistingJobAuxComponentVolumes Get existing job aux component volumes
func GetExistingJobAuxComponentVolumes(ctx context.Context, kubeUtil *kube.Kube, namespace, jobComponentName string) ([]corev1.Volume, error) {
	jobAuxKubeDeploymentName := defaults.GetJobAuxKubeDeployName(jobComponentName)
	jobAuxKubeDeployment, err := kubeUtil.GetDeployment(ctx, namespace, jobAuxKubeDeploymentName)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return jobAuxKubeDeployment.Spec.Template.Spec.Volumes, nil
}

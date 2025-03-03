package volumemount

import (
	"context"
	"errors"
	"fmt"
	"strings"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	internalDeployment "github.com/equinor/radix-operator/pkg/apis/internal/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	csiVolumeTypeBlobFuse2ProtocolFuse      = "csi-az-blob"
	csiVolumeTypeBlobFuse2ProtocolFuse2     = "csi-blobfuse2-fuse2"
	csiVolumeNameTemplate                   = "%s-%s-%s-%s" // <radixvolumeid>-<componentname>-<radixvolumename>-<storage>
	csiAzureKeyVaultSecretMountPathTemplate = "/mnt/azure-key-vault/%s"
	volumeNameMaxLength                     = 63
)

var (
	csiVolumeProvisioners = map[string]any{provisionerBlobCsiAzure: struct{}{}}
	functionalPvPhases    = []corev1.PersistentVolumePhase{corev1.VolumePending, corev1.VolumeBound, corev1.VolumeAvailable}
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

// GetExistingJobAuxComponentVolumes Get existing job aux component volumes
func GetExistingJobAuxComponentVolumes(ctx context.Context, kubeUtil *kube.Kube, namespace, jobComponentName string) ([]corev1.Volume, error) {
	jobAuxKubeDeploymentName := defaults.GetJobAuxKubeDeployName(jobComponentName)
	jobAuxKubeDeployment, err := kubeUtil.KubeClient().AppsV1().Deployments(namespace).Get(ctx, jobAuxKubeDeploymentName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return jobAuxKubeDeployment.Spec.Template.Spec.Volumes, nil
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

// GarbageCollectCsiAzureVolumeResourcesForDeployComponent Garbage collect CSI Azure volume resources - PersistentVolumes, PersistentVolumeClaims
func GarbageCollectCsiAzureVolumeResourcesForDeployComponent(ctx context.Context, kubeClient kubernetes.Interface, radixDeployment *radixv1.RadixDeployment, namespace string) error {
	currentlyUsedPvcNames, err := getCurrentlyUsedPvcNames(ctx, kubeClient, radixDeployment)
	if err != nil {
		return err
	}
	var errs []error
	if err = garbageCollectCsiAzurePvcs(ctx, kubeClient, namespace, currentlyUsedPvcNames); err != nil {
		errs = append(errs, err)
	}
	if err = garbageCollectCsiAzurePvs(ctx, kubeClient, namespace, currentlyUsedPvcNames); err != nil {
		errs = append(errs, err)
	}
	if err = garbageCollectOrphanedCsiAzurePvs(ctx, kubeClient, currentlyUsedPvcNames); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// CreateOrUpdateVolumeMountSecrets creates or updates secrets for volume mounts
func CreateOrUpdateVolumeMountSecrets(ctx context.Context, kubeUtil *kube.Kube, appName, namespace, componentName string, volumeMounts []radixv1.RadixVolumeMount) ([]string, error) {
	var volumeMountSecretsToManage []string
	for _, volumeMount := range volumeMounts {
		if volumeMount.HasEmptyDir() || volumeMount.UseAzureIdentity() {
			continue
		}
		secretName, accountKey, accountName := getCsiAzureVolumeMountCredsSecrets(ctx, kubeUtil, namespace, componentName, volumeMount.Name)
		volumeMountSecretsToManage = append(volumeMountSecretsToManage, secretName)
		if err := createOrUpdateCsiAzureVolumeMountsSecrets(ctx, kubeUtil, appName, namespace, componentName, &volumeMount, secretName, accountName, accountKey); err != nil {
			return nil, err
		}
	}
	return volumeMountSecretsToManage, nil
}

func getCsiAzurePvsForNamespace(ctx context.Context, kubeClient kubernetes.Interface, namespace string, onlyFunctional bool) ([]corev1.PersistentVolume, error) {
	pvList, err := kubeClient.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return slice.FindAll(pvList.Items, func(pv corev1.PersistentVolume) bool {
		return pvIsForCsiDriver(pv) && pvIsForNamespace(pv, namespace) && (!onlyFunctional || pvIsFunctional(pv))
	}), nil
}

func getRadixComponentVolumeMounts(deployComponent radixv1.RadixCommonDeployComponent) ([]corev1.VolumeMount, error) {
	if internalDeployment.IsDeployComponentJobSchedulerDeployment(deployComponent) {
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
	csiAzureVolumeStorageName := volumeMount.GetStorageContainerName()
	if len(csiAzureVolumeStorageName) == 0 {
		return "", fmt.Errorf("storage is empty for volume mount %s in the component %s", volumeMount.Name, componentName)
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
	//nolint:staticcheck
	if radixVolumeMount.Type == radixv1.MountTypeBlobFuse2FuseCsiAzure {
		return csiVolumeTypeBlobFuse2ProtocolFuse, nil
	}
	//nolint:staticcheck
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
					Driver:           CsiVolumeSourceDriverSecretStore,
					ReadOnly:         pointers.Ptr(true),
					VolumeAttributes: map[string]string{CsiVolumeSourceVolumeAttributeSecretProviderClass: secretProviderClass.Name},
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
	pvcName, err := getCsiAzurePvcName(componentName, radixVolumeMount)
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
	//nolint:staticcheck
	if volumeMount.Type == radixv1.MountTypeBlobFuse2FuseCsiAzure {
		return getCsiAzureVolumeSource(componentName, volumeMount)
	}
	//nolint:staticcheck
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
	//nolint:staticcheck
	switch volumeMount.Type {
	case radixv1.MountTypeBlobFuse2FuseCsiAzure:
		return getCsiAzureVolumeMountName(componentName, volumeMount)
	}
	//nolint:staticcheck
	return "", fmt.Errorf("unsupported volume type %s", volumeMount.Type)
}

func getCsiAzurePvcName(componentName string, radixVolumeMount *radixv1.RadixVolumeMount) (string, error) {
	volumeName, err := getCsiAzureVolumeMountName(componentName, radixVolumeMount)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(csiPersistentVolumeClaimNameTemplate, volumeName, strings.ToLower(commonUtils.RandString(nameRandPartLength))), nil
}

func getCsiAzurePvName() string {
	return fmt.Sprintf(csiPersistentVolumeNameTemplate, uuid.New().String())
}

func createOrUpdateCsiAzureVolumeMountsSecrets(ctx context.Context, kubeUtil *kube.Kube, appName, namespace, componentName string, radixVolumeMount *radixv1.RadixVolumeMount, secretName string, accountName, accountKey []byte) error {
	secret := corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: map[string]string{
				kube.RadixAppLabel:             appName,
				kube.RadixComponentLabel:       componentName,
				kube.RadixMountTypeLabel:       string(radixVolumeMount.GetVolumeMountType()),
				kube.RadixVolumeMountNameLabel: radixVolumeMount.Name,
			},
		},
	}

	// Will need to set fake data in order to apply the secret. The user then need to set data to real values
	data := make(map[string][]byte)
	data[defaults.CsiAzureCredsAccountKeyPart] = accountKey
	if radixVolumeMount.BlobFuse2 == nil || len(radixVolumeMount.BlobFuse2.StorageAccount) == 0 {
		data[defaults.CsiAzureCredsAccountNamePart] = accountName
	}

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
	// not Terminating or Released
	return slice.Any(functionalPvPhases, func(phase corev1.PersistentVolumePhase) bool { return pv.Status.Phase == phase })
}

func getCsiAzurePvcs(ctx context.Context, kubeClient kubernetes.Interface, namespace string) (*corev1.PersistentVolumeClaimList, error) {
	return kubeClient.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
}

func deletePvc(ctx context.Context, kubeClient kubernetes.Interface, namespace, pvcName string) error {
	if len(namespace) == 0 || len(pvcName) == 0 {
		log.Ctx(ctx).Debug().Msgf("Skip deleting PVC - namespace %s or name %s is empty", namespace, pvcName)
		return nil
	}
	if err := kubeClient.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, pvcName, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	return nil
}

func deletePv(ctx context.Context, kubeClient kubernetes.Interface, pvName string) error {
	if len(pvName) == 0 {
		log.Ctx(ctx).Debug().Msg("Skip deleting PersistentVolume - name is empty")
		return nil
	}
	if err := kubeClient.CoreV1().PersistentVolumes().Delete(ctx, pvName, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	return nil
}

func garbageCollectOrphanedCsiAzurePvs(ctx context.Context, kubeClient kubernetes.Interface, excludePvcNames map[string]any) error {
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
		if err := deletePv(ctx, kubeClient, pv.Name); err != nil && !k8serrors.IsNotFound(err) {
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

func getCurrentlyUsedPvcNames(ctx context.Context, kubeClient kubernetes.Interface, radixDeployment *radixv1.RadixDeployment) (map[string]any, error) {
	namespace := radixDeployment.GetNamespace()
	currentlyUsedPvcNames := make(map[string]any)
	existingDeployments, err := kubeClient.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, deployment := range existingDeployments.Items {
		currentlyUsedPvcNames = appendPvcNamesFromVolumes(currentlyUsedPvcNames, deployment.Spec.Template.Spec.Volumes)
	}
	existingJobs, err := kubeClient.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, job := range existingJobs.Items {
		currentlyUsedPvcNames = appendPvcNamesFromVolumes(currentlyUsedPvcNames, job.Spec.Template.Spec.Volumes)
	}
	return currentlyUsedPvcNames, nil
}

func appendPvcNamesFromVolumes(pvcMap map[string]any, volumes []corev1.Volume) map[string]any {
	return slice.Reduce(volumes, pvcMap, func(acc map[string]any, volume corev1.Volume) map[string]any {
		if volume.PersistentVolumeClaim != nil && len(volume.PersistentVolumeClaim.ClaimName) > 0 {
			acc[volume.PersistentVolumeClaim.ClaimName] = struct{}{}
		}
		return acc
	})
}

func garbageCollectCsiAzurePvs(ctx context.Context, kubeClient kubernetes.Interface, namespace string, excludePvcNames map[string]any) error {
	pvs, err := getCsiAzurePvsForNamespace(ctx, kubeClient, namespace, false)
	if err != nil {
		return err
	}
	for _, pv := range pvs {
		if _, ok := excludePvcNames[pv.Spec.ClaimRef.Name]; ok {
			continue
		}
		log.Ctx(ctx).Debug().Msgf("Delete not used CSI Azure PersistentVolume %s in namespace %s", pv.Name, namespace)
		if err = deletePv(ctx, kubeClient, pv.Name); err != nil {
			return err
		}
	}
	return nil
}
func garbageCollectCsiAzurePvcs(ctx context.Context, kubeClient kubernetes.Interface, namespace string, excludePvcNames map[string]any) error {
	pvcList, err := getCsiAzurePvcs(ctx, kubeClient, namespace)
	if err != nil {
		return err
	}
	var errs []error
	for _, pvc := range pvcList.Items {
		if _, ok := excludePvcNames[pvc.Name]; ok {
			continue
		}
		log.Ctx(ctx).Debug().Msgf("Delete not used CSI Azure PersistentVolumeClaim %s in namespace %s", pvc.Name, namespace)
		if err := deletePvc(ctx, kubeClient, namespace, pvc.Name); err != nil {
			errs = append(errs, err)
			continue
		}
		pvName := pvc.Spec.VolumeName
		log.Ctx(ctx).Debug().Msgf("Delete not used CSI Azure PersistentVolume %s in namespace %s", pvName, namespace)
		if err := deletePv(ctx, kubeClient, pvName); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func trimVolumeNameToValidLength(volumeName string) string {
	if len(volumeName) <= volumeNameMaxLength {
		return volumeName
	}

	randString := strings.ToLower(commonUtils.RandStringStrSeed(nameRandPartLength, volumeName))
	return fmt.Sprintf("%s-%s", volumeName[:63-nameRandPartLength-1], randString)
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

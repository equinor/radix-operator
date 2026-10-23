package volumemount

import (
	"context"
	"fmt"
	"strings"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	internalDeployment "github.com/equinor/radix-operator/pkg/apis/internal/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/random"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	csiVolumeTypeBlobFuse2ProtocolFuse2     = "csi-blobfuse2-fuse2"
	csiVolumeNameTemplate                   = "%s-%s-%s-%s" // <radixvolumeid>-<componentname>-<radixvolumename>-<storage>
	csiAzureKeyVaultSecretMountPathTemplate = "/mnt/azure-key-vault/%s"
	volumeNameMaxLength                     = 63
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

// CreateOrUpdateVolumeMountSecrets creates or updates secrets for volume mounts
func CreateOrUpdateVolumeMountSecrets(ctx context.Context, kubeUtil *kube.Kube, appName, namespace string, component radixv1.RadixCommonDeployComponent) ([]string, error) {
	var secretsToManage []string
	for _, volumeMount := range component.GetVolumeMounts() {
		if volumeMount.HasEmptyDir() || volumeMount.UseAzureIdentity() {
			continue
		}
		secretName := defaults.GetCsiAzureVolumeMountCredsSecretName(component.GetName(), volumeMount.Name)
		if err := createOrUpdateVolumeMountSecret(ctx, kubeUtil, appName, namespace, component.GetName(), secretName, volumeMount); err != nil {
			return nil, err
		}
		secretsToManage = append(secretsToManage, secretName)
	}
	return secretsToManage, nil
}

func createOrUpdateVolumeMountSecret(ctx context.Context, kubeUtil *kube.Kube, appName, namespace, componentName string, secretName string, volumeMount radixv1.RadixVolumeMount) error {
	var currentSecret, desiredSecret *corev1.Secret
	currentSecret, err := kubeUtil.KubeClient().CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to get current volume mount secret: %w", err)
		}
		desiredSecret = &corev1.Secret{
			Name:      secretName,
			Namespace: namespace,
		}
		currentSecret = nil
	} else {
		desiredSecret = currentSecret.DeepCopy()
	}

	desiredSecret.Labels = map[string]string{
		kube.RadixAppLabel:             appName,
		kube.RadixComponentLabel:       componentName,
		kube.RadixMountTypeLabel:       string(volumeMount.GetVolumeMountType()),
		kube.RadixVolumeMountNameLabel: volumeMount.Name,
	}
	data := map[string][]byte{}
	data[defaults.CsiAzureCredsAccountKeyPart] = desiredSecret.Data[defaults.CsiAzureCredsAccountKeyPart]
	if volumeMount.BlobFuse2 == nil || len(volumeMount.BlobFuse2.StorageAccount) == 0 {
		data[defaults.CsiAzureCredsAccountNamePart] = desiredSecret.Data[defaults.CsiAzureCredsAccountNamePart]
	}
	desiredSecret.Data = data

	if currentSecret == nil {
		if _, err := kubeUtil.CreateSecret(ctx, namespace, desiredSecret); err != nil {
			return fmt.Errorf("failed to create volume mount secret: %w", err)
		}
		return nil
	}

	if _, err := kubeUtil.UpdateSecret(ctx, currentSecret, desiredSecret); err != nil {
		return fmt.Errorf("failed to update volume mount secret: %w", err)
	}

	return nil
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
	return "", fmt.Errorf("volume mount %s is not a blobFuse2 volume mount", radixVolumeMount.Name)
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
					ReadOnly:         new(true),
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

func getComponentVolumeMountEmptyDirVolumeSource(spec *radixv1.RadixEmptyDirVolumeMount) *corev1.VolumeSource {
	return &corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{
			SizeLimit: &spec.SizeLimit,
		},
	}
}

// GetVolumeMountVolumeName Gets the volume name for a volume mount
func GetVolumeMountVolumeName(volumeMount *radixv1.RadixVolumeMount, componentName string) (string, error) {
	// TODO: Why does the name of the volume include info about the driver, external container etc? Can we make a static name instead, like done in the last line?
	switch {
	case volumeMount.HasBlobFuse2():
		return getCsiAzureVolumeMountName(componentName, volumeMount)
	}
	return fmt.Sprintf("radix-vm-%s", volumeMount.Name), nil
}

func trimVolumeNameToValidLength(volumeName string) string {
	if len(volumeName) <= volumeNameMaxLength {
		return volumeName
	}

	randString := strings.ToLower(random.RandStringStrSeed(nameRandPartLength, volumeName))
	return fmt.Sprintf("%s-%s", volumeName[:63-nameRandPartLength-1], randString)
}

func getLabelSelectorForCsiAzureVolumeMountSecret(component radixv1.RadixCommonDeployComponent) string {
	return fmt.Sprintf("%s=%s, %s=%s", kube.RadixComponentLabel, component.GetName(), kube.RadixMountTypeLabel, string(radixv1.MountTypeBlobFuse2Fuse2CsiAzure))
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

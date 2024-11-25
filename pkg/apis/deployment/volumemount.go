package deployment

import (
	"context"
	"fmt"
	"sort"
	"strings"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog/log"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	persistentVolumeClaimKind = "PersistentVolumeClaim"

	blobfuseDriver      = "azure/blobfuse"
	defaultMountOptions = "--file-cache-timeout-in-seconds=120"

	blobFuseVolumeNameTemplate          = "blobfuse-%s-%s"         // blobfuse-<componentname>-<radixvolumename>
	blobFuseVolumeNodeMountPathTemplate = "/tmp/%s/%s/%s/%s/%s/%s" // /tmp/<namespace>/<componentname>/<environment>/<volumetype>/<radixvolumename>/<container>

	csiVolumeNameTemplate                = "%s-%s-%s-%s"       // <radixvolumeid>-<componentname>-<radixvolumename>-<storage>
	csiPersistentVolumeClaimNameTemplate = "pvc-%s-%s"         // pvc-<volumename>-<randomstring5>
	csiPersistentVolumeNameTemplate      = "sc-%s-%s"          // sc-<namespace>-<volumename>
	csiVolumeNodeMountPathTemplate       = "%s/%s/%s/%s/%s/%s" // <volumeRootMount>/<namespace>/<radixvolumeid>/<componentname>/<radixvolumename>/<storage>

	csiPersistentVolumeProvisionerSecretNameParameter       = "csi.storage.k8s.io/provisioner-secret-name"      // Secret name, containing storage account name and key
	csiPersistentVolumeProvisionerSecretNamespaceParameter  = "csi.storage.k8s.io/provisioner-secret-namespace" // namespace of the secret
	csiPersistentVolumeNodeStageSecretNameParameter         = "csi.storage.k8s.io/node-stage-secret-name"       // Usually equal to csiPersistentVolumeProvisionerSecretNameParameter
	csiPersistentVolumeNodeStageSecretNamespaceParameter    = "csi.storage.k8s.io/node-stage-secret-namespace"  // Usually equal to csiPersistentVolumeProvisionerSecretNamespaceParameter
	csiVolumeMountAttributeContainerName                    = "containerName"                                   // Container name - foc container storages
	csiPersistentVolumeTmpPathMountOption                   = "tmp-path"                                        // Path within the node, where the volume mount has been mounted to
	csiPersistentVolumeGidMountOption                       = "gid"                                             // Volume mount owner GroupID. Used when drivers do not honor fsGroup securityContext setting
	csiPersistentVolumeUidMountOption                       = "uid"                                             // Volume mount owner UserID. Used instead of GroupID
	csiPersistentVolumeUseAdlsMountOption                   = "use-adls"                                        // Use ADLS or Block Blob
	csiPersistentVolumeStreamingEnabledMountOption          = "streaming"                                       // Enable Streaming
	csiPersistentVolumeStreamingCacheMountOption            = "stream-cache-mb"                                 // Limit total amount of data being cached in memory to conserve memory
	csiPersistentVolumeStreamingMaxBlocksPerFileMountOption = "max-blocks-per-file"                             // Maximum number of blocks to be cached in memory for streaming
	csiPersistentVolumeStreamingMaxBuffersMountOption       = "max-buffers"                                     // The total number of buffers to be cached in memory (in MB).
	csiPersistentVolumeStreamingBlockSizeMountOption        = "block-size-mb"                                   // The size of each block to be cached in memory (in MB).
	csiPersistentVolumeStreamingBufferSizeMountOption       = "buffer-size-mb"                                  // The size of each buffer to be cached in memory (in MB).
	csiVolumeMountAttributeProtocol                         = "protocol"                                        // Protocol
	csiPersistentVolumeProtocolParameterFuse                = "fuse"                                            // Protocol "blobfuse"
	csiPersistentVolumeProtocolParameterFuse2               = "fuse2"                                           // Protocol "blobfuse2"
	csiVolumeMountAttributeStorageAccount                   = "storageAccount"
	csiVolumeMountAttributeClientID                         = "clientID"
	csiVolumeMountAttributeResourceGroup                    = "resourcegroup"
	azureCsiDriverVolumeHandle                              = "azure-csi-driver-volume-handle"

	csiSecretStoreDriver                             = "secrets-store.csi.k8s.io"
	csiVolumeSourceVolumeAttrSecretProviderClassName = "secretProviderClass"
	csiAzureKeyVaultSecretMountPathTemplate          = "/mnt/azure-key-vault/%s"

	volumeNameMaxLength = 63
)

// These are valid volume mount provisioners
const (
	// provisionerBlobCsiAzure Use of azure/csi driver for blob in Azure storage account
	provisionerBlobCsiAzure string = "blob.csi.azure.com"
)

var (
	csiVolumeProvisioners = map[string]any{provisionerBlobCsiAzure: struct{}{}}
)

// isKnownCsiAzureVolumeMount Supported volume mount type CSI Azure Blob volume
func isKnownCsiAzureVolumeMount(volumeMount string) bool {
	switch volumeMount {
	case string(radixv1.MountTypeBlobFuse2FuseCsiAzure), string(radixv1.MountTypeBlobFuse2Fuse2CsiAzure):
		return true
	}
	return false
}

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

func getRadixComponentVolumeMounts(deployComponent radixv1.RadixCommonDeployComponent) ([]corev1.VolumeMount, error) {
	if isDeployComponentJobSchedulerDeployment(deployComponent) {
		return nil, nil
	}

	var volumeMounts []corev1.VolumeMount
	for _, volumeMount := range deployComponent.GetVolumeMounts() {
		name, err := getVolumeMountVolumeName(&volumeMount, deployComponent.GetName())
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

func getBlobFuseVolumeMountName(volumeMount *radixv1.RadixVolumeMount, componentName string) string {
	return trimVolumeNameToValidLength(fmt.Sprintf(blobFuseVolumeNameTemplate, componentName, volumeMount.Name))
}

func getCsiAzureVolumeMountName(volumeMount *radixv1.RadixVolumeMount, componentName string) (string, error) {
	csiVolumeType, err := getCsiRadixVolumeTypeIdForName(volumeMount)
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

func getCsiRadixVolumeTypeIdForName(radixVolumeMount *radixv1.RadixVolumeMount) (string, error) {
	if radixVolumeMount.BlobFuse2 != nil {
		switch radixVolumeMount.BlobFuse2.Protocol {
		case radixv1.BlobFuse2ProtocolFuse2, "":
			return "csi-blobfuse2-fuse2", nil
		default:
			return "", fmt.Errorf("unknown blobfuse2 protocol %s", radixVolumeMount.BlobFuse2.Protocol)
		}
	}
	switch radixVolumeMount.Type {
	case radixv1.MountTypeBlobFuse2FuseCsiAzure:
		return "csi-az-blob", nil
	}
	return "", fmt.Errorf("unknown volume mount type %s", radixVolumeMount.Type)
}

// GetVolumesForComponent Gets volumes for Radix deploy component or job
func (deploy *Deployment) GetVolumesForComponent(ctx context.Context, deployComponent radixv1.RadixCommonDeployComponent) ([]corev1.Volume, error) {
	return GetVolumes(ctx, deploy.kubeclient, deploy.kubeutil, deploy.getNamespace(), deploy.radixDeployment.Spec.Environment, deployComponent, deploy.radixDeployment.GetName())
}

// GetVolumes Get volumes of a component by RadixVolumeMounts
func GetVolumes(ctx context.Context, kubeclient kubernetes.Interface, kubeutil *kube.Kube, namespace string, environment string, deployComponent radixv1.RadixCommonDeployComponent, radixDeploymentName string) ([]corev1.Volume, error) {
	var volumes []corev1.Volume

	volumeMountVolumes, err := getComponentVolumeMountVolumes(ctx, kubeclient, namespace, environment, deployComponent)
	if err != nil {
		return nil, err
	}
	volumes = append(volumes, volumeMountVolumes...)

	storageRefsVolumes, err := getComponentSecretRefsVolumes(ctx, kubeutil, namespace, deployComponent, radixDeploymentName)
	if err != nil {
		return nil, err
	}
	volumes = append(volumes, storageRefsVolumes...)

	return volumes, nil
}

func getComponentSecretRefsVolumes(ctx context.Context, kubeutil *kube.Kube, namespace string, deployComponent radixv1.RadixCommonDeployComponent, radixDeploymentName string) ([]corev1.Volume, error) {
	var volumes []corev1.Volume
	azureKeyVaultVolumes, err := getComponentSecretRefsAzureKeyVaultVolumes(ctx, kubeutil, namespace, deployComponent, radixDeploymentName)
	if err != nil {
		return nil, err
	}
	volumes = append(volumes, azureKeyVaultVolumes...)
	return volumes, nil
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
					Driver:           csiSecretStoreDriver,
					ReadOnly:         commonUtils.BoolPtr(true),
					VolumeAttributes: map[string]string{csiVolumeSourceVolumeAttrSecretProviderClassName: secretProviderClass.Name},
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

func getComponentVolumeMountVolumes(ctx context.Context, kubeclient kubernetes.Interface, namespace string, environment string, deployComponent radixv1.RadixCommonDeployComponent) ([]corev1.Volume, error) {
	var volumes []corev1.Volume

	volumeSourceFunc := func(volumeMount *radixv1.RadixVolumeMount) (*corev1.VolumeSource, error) {
		switch {
		case volumeMount.HasDeprecatedVolume():
			return getComponentVolumeMountDeprecatedVolumeSource(ctx, volumeMount, namespace, environment, deployComponent.GetName(), kubeclient)
		case volumeMount.HasBlobFuse2():
			return getComponentVolumeMountBlobFuse2VolumeSource(ctx, volumeMount, namespace, deployComponent.GetName(), kubeclient)
		case volumeMount.HasEmptyDir():
			return getComponentVolumeMountEmptyDirVolumeSource(volumeMount.EmptyDir), nil
		}
		return nil, fmt.Errorf("missing configuration for volumeMount %s", volumeMount.Name)
	}

	for _, volumeMount := range deployComponent.GetVolumeMounts() {
		volumeSource, err := volumeSourceFunc(&volumeMount)
		if err != nil {
			return nil, err
		}
		volumeName, err := getVolumeMountVolumeName(&volumeMount, deployComponent.GetName())
		if err != nil {
			return nil, err
		}
		volumes = append(volumes, corev1.Volume{
			Name:         volumeName,
			VolumeSource: *volumeSource,
		})
	}
	return volumes, nil
}

func getComponentVolumeMountDeprecatedVolumeSource(ctx context.Context, volumeMount *radixv1.RadixVolumeMount, namespace, environment, componentName string, kubeclient kubernetes.Interface) (*corev1.VolumeSource, error) {
	switch volumeMount.Type {
	case radixv1.MountTypeBlob:
		return getBlobFuseVolume(namespace, environment, componentName, volumeMount), nil
	case radixv1.MountTypeBlobFuse2FuseCsiAzure:
		return getCsiAzureVolume(ctx, kubeclient, namespace, componentName, volumeMount)
	}

	return nil, fmt.Errorf("unsupported volume type %s", volumeMount.Type)
}

func getComponentVolumeMountBlobFuse2VolumeSource(ctx context.Context, volumeMount *radixv1.RadixVolumeMount, namespace, componentName string, kubeclient kubernetes.Interface) (*corev1.VolumeSource, error) {
	return getCsiAzureVolume(ctx, kubeclient, namespace, componentName, volumeMount)
}

func getComponentVolumeMountEmptyDirVolumeSource(spec *radixv1.RadixEmptyDirVolumeMount) *corev1.VolumeSource {
	return &corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{
			SizeLimit: &spec.SizeLimit,
		},
	}
}

func getCsiAzureVolume(ctx context.Context, kubeclient kubernetes.Interface, namespace, componentName string, radixVolumeMount *radixv1.RadixVolumeMount) (*corev1.VolumeSource, error) {
	existingNotTerminatingPvcForComponentStorage, err := getPvcNotTerminating(ctx, kubeclient, namespace, componentName, radixVolumeMount)
	if err != nil {
		return nil, err
	}

	var pvcName string
	if existingNotTerminatingPvcForComponentStorage != nil {
		pvcName = existingNotTerminatingPvcForComponentStorage.Name
	} else {
		pvcName, err = createCsiAzurePersistentVolumeClaimName(componentName, radixVolumeMount)
		if err != nil {
			return nil, err
		}
	}
	return &corev1.VolumeSource{
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
			ClaimName: pvcName,
		},
	}, nil
}

func getVolumeMountVolumeName(volumeMount *radixv1.RadixVolumeMount, componentName string) (string, error) {
	switch {
	case volumeMount.HasDeprecatedVolume():
		return getVolumeMountDeprecatedVolumeName(volumeMount, componentName)
	case volumeMount.HasBlobFuse2():
		return getVolumeMountBlobFuse2VolumeName(volumeMount, componentName)
	}

	return fmt.Sprintf("radix-vm-%s", volumeMount.Name), nil
}

func getVolumeMountBlobFuse2VolumeName(volumeMount *radixv1.RadixVolumeMount, componentName string) (string, error) {
	return getCsiAzureVolumeMountName(volumeMount, componentName)
}

func getVolumeMountDeprecatedVolumeName(volumeMount *radixv1.RadixVolumeMount, componentName string) (string, error) {
	switch volumeMount.Type {
	case radixv1.MountTypeBlob:
		return getBlobFuseVolumeMountName(volumeMount, componentName), nil
	case radixv1.MountTypeBlobFuse2FuseCsiAzure:
		return getCsiAzureVolumeMountName(volumeMount, componentName)
	}

	return "", fmt.Errorf("unsupported type %s", volumeMount.Type)
}

func getPvcNotTerminating(ctx context.Context, kubeclient kubernetes.Interface, namespace string, componentName string, radixVolumeMount *radixv1.RadixVolumeMount) (*corev1.PersistentVolumeClaim, error) {
	existingPvcForComponentStorage, err := kubeclient.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: getLabelSelectorForCsiAzurePersistenceVolumeClaimForComponentStorage(componentName, radixVolumeMount.Name),
	})
	if err != nil {
		return nil, err
	}
	existingPvcs := sortPvcsByCreatedTimestampDesc(existingPvcForComponentStorage.Items)
	if len(existingPvcs) == 0 {
		return nil, nil
	}
	for _, pvc := range existingPvcs {
		switch pvc.Status.Phase {
		case corev1.ClaimPending, corev1.ClaimBound:
			return &pvc, nil
		}
	}
	return nil, nil
}

func createCsiAzurePersistentVolumeClaimName(componentName string, radixVolumeMount *radixv1.RadixVolumeMount) (string, error) {
	volumeName, err := getCsiAzureVolumeMountName(radixVolumeMount, componentName)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(csiPersistentVolumeClaimNameTemplate, volumeName, strings.ToLower(commonUtils.RandString(5))), nil // volumeName: <component-name>-<csi-volume-type-dashed>-<radix-volume-name>-<storage-name>
}

func getCsiAzurePersistentVolumeName(namespace, volumeName string) string {
	return fmt.Sprintf(csiPersistentVolumeNameTemplate, namespace, volumeName) // volumeName: <component-name>-<csi-volume-type-dashed>-<radix-volume-name>-<storage-name>
}

func getBlobFuseVolume(namespace, environment, componentName string, volumeMount *radixv1.RadixVolumeMount) *corev1.VolumeSource {
	secretName := defaults.GetBlobFuseCredsSecretName(componentName, volumeMount.Name)

	flexVolumeOptions := make(map[string]string)
	flexVolumeOptions["name"] = volumeMount.Name
	flexVolumeOptions["container"] = volumeMount.Container
	flexVolumeOptions["mountoptions"] = defaultMountOptions
	flexVolumeOptions["tmppath"] = fmt.Sprintf(blobFuseVolumeNodeMountPathTemplate, namespace, componentName, environment, radixv1.MountTypeBlob, volumeMount.Name, volumeMount.Container)

	return &corev1.VolumeSource{
		FlexVolume: &corev1.FlexVolumeSource{
			Driver:  blobfuseDriver,
			Options: flexVolumeOptions,
			SecretRef: &corev1.LocalObjectReference{
				Name: secretName,
			},
		},
	}
}

func (deploy *Deployment) createOrUpdateVolumeMountsSecrets(ctx context.Context, namespace, componentName, secretName string, accountName, accountKey []byte) error {
	blobfusecredsSecret := corev1.Secret{
		Type: blobfuseDriver,
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: map[string]string{
				kube.RadixAppLabel:       deploy.registration.Name,
				kube.RadixComponentLabel: componentName,
				kube.RadixMountTypeLabel: string(radixv1.MountTypeBlob),
			},
		},
	}

	// Will need to set fake data in order to apply the secret. The user then need to set data to real values
	data := make(map[string][]byte)
	data[defaults.BlobFuseCredsAccountKeyPart] = accountKey
	data[defaults.BlobFuseCredsAccountNamePart] = accountName

	blobfusecredsSecret.Data = data

	_, err := deploy.kubeutil.ApplySecret(ctx, namespace, &blobfusecredsSecret) //nolint:staticcheck // must be updated to use UpdateSecret or CreateSecret
	if err != nil {
		return err
	}

	return nil
}
func (deploy *Deployment) createOrUpdateCsiAzureVolumeMountsSecrets(ctx context.Context, namespace, componentName string, radixVolumeMount *radixv1.RadixVolumeMount, secretName string, accountName, accountKey []byte) error {
	secret := corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: map[string]string{
				kube.RadixAppLabel:             deploy.registration.Name,
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

	_, err := deploy.kubeutil.ApplySecret(ctx, namespace, &secret) //nolint:staticcheck // must be updated to use UpdateSecret or CreateSecret
	if err != nil {
		return err
	}

	return nil
}

func (deploy *Deployment) garbageCollectVolumeMountsSecretsNoLongerInSpecForComponent(ctx context.Context, component radixv1.RadixCommonDeployComponent, excludeSecretNames []string) error {
	secrets, err := deploy.listSecretsForVolumeMounts(ctx, component)
	if err != nil {
		return err
	}
	return deploy.GarbageCollectSecrets(ctx, secrets, excludeSecretNames)
}

func (deploy *Deployment) getCsiAzurePersistentVolume(ctx context.Context, namespace, componentName string) (*corev1.PersistentVolumeList, error) {
	return deploy.kubeclient.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{
		LabelSelector: getLabelSelectorForCsiAzurePersistentVolume(namespace, componentName),
	})
}

func (deploy *Deployment) getCsiAzurePersistentVolumeClaims(ctx context.Context, namespace, componentName string) (*corev1.PersistentVolumeClaimList, error) {
	return deploy.kubeclient.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: getLabelSelectorForCsiAzurePersistenceVolumeClaim(componentName),
	})
}

func (deploy *Deployment) getPersistentVolumesForPvc(ctx context.Context) (*corev1.PersistentVolumeList, error) {
	return deploy.kubeclient.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
}

func getLabelSelectorForCsiAzurePersistentVolume(namespace, componentName string) string {
	return fmt.Sprintf("%s=%s, %s=%s, %s in (%s, %s)", kube.RadixNamespace, namespace, kube.RadixComponentLabel, componentName, kube.RadixMountTypeLabel, string(radixv1.MountTypeBlobFuse2FuseCsiAzure), string(radixv1.MountTypeBlobFuse2Fuse2CsiAzure))
}

func getLabelSelectorForCsiAzurePersistenceVolumeClaim(componentName string) string {
	return fmt.Sprintf("%s=%s, %s in (%s, %s)", kube.RadixComponentLabel, componentName, kube.RadixMountTypeLabel, string(radixv1.MountTypeBlobFuse2FuseCsiAzure), string(radixv1.MountTypeBlobFuse2Fuse2CsiAzure))
}

func getLabelSelectorForCsiAzurePersistenceVolumeClaimForComponentStorage(componentName, radixVolumeMountName string) string {
	return fmt.Sprintf("%s=%s, %s in (%s, %s), %s = %s", kube.RadixComponentLabel, componentName, kube.RadixMountTypeLabel, string(radixv1.MountTypeBlobFuse2FuseCsiAzure), string(radixv1.MountTypeBlobFuse2Fuse2CsiAzure), kube.RadixVolumeMountNameLabel, radixVolumeMountName)
}

func (deploy *Deployment) createPersistentVolumeClaim(ctx context.Context, appName, namespace, componentName, pvcName, pvName string, radixVolumeMount *radixv1.RadixVolumeMount) (*corev1.PersistentVolumeClaim, error) {
	requestsVolumeMountSize, err := resource.ParseQuantity(getRadixBlobFuse2VolumeMountRequestsStorage(radixVolumeMount))
	if err != nil {
		requestsVolumeMountSize = resource.MustParse("1Mi")
	}
	pvc := &corev1.PersistentVolumeClaim{
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
				Requests: corev1.ResourceList{corev1.ResourceStorage: requestsVolumeMountSize}, // it seems correct number is not needed for CSI driver
			},
			VolumeName: pvName,
		},
	}
	return deploy.kubeclient.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{})
}

func populateCsiAzurePersistentVolume(persistentVolume *corev1.PersistentVolume, appName, volumeRootMount, namespace, componentName, pvName string, radixVolumeMount *radixv1.RadixVolumeMount, identity *radixv1.Identity) error {
	identityClientId := getIdentityClientId(identity)
	useAzureIdentity := getUseAzureIdentity(identity, radixVolumeMount.UseAzureIdentity)
	csiVolumeCredSecretName := defaults.GetCsiAzureVolumeMountCredsSecretName(componentName, radixVolumeMount.Name)
	persistentVolume.ObjectMeta.Name = pvName
	persistentVolume.ObjectMeta.Labels = getCsiAzurePersistentVolumeLabels(appName, namespace, componentName, radixVolumeMount)
	persistentVolume.ObjectMeta.Annotations = getCsiAzurePersistentVolumeAnnotations(csiVolumeCredSecretName, namespace, useAzureIdentity)
	mountOptions, err := getCsiAzurePersistentVolumeMountOptions(volumeRootMount, namespace, componentName, radixVolumeMount)
	if err != nil {
		return err
	}
	persistentVolume.Spec.MountOptions = mountOptions
	persistentVolume.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{getVolumeMountAccessMode(radixVolumeMount)}
	persistentVolume.Spec.ClaimRef = &corev1.ObjectReference{
		APIVersion: "v1",
		Kind:       persistentVolumeClaimKind,
		Namespace:  namespace,
		Name:       pvName,
	}
	persistentVolume.Spec.CSI = &corev1.CSIPersistentVolumeSource{
		Driver:           provisionerBlobCsiAzure,
		VolumeHandle:     azureCsiDriverVolumeHandle,
		VolumeAttributes: getCsiAzurePersistentVolumeAttributes(radixVolumeMount, pvName, namespace, useAzureIdentity, identityClientId),
	}
	if !useAzureIdentity {
		persistentVolume.Spec.CSI.NodeStageSecretRef = &corev1.SecretReference{Name: csiVolumeCredSecretName, Namespace: namespace}
	}
	persistentVolume.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain // Using only PersistentVolumeReclaimRetain. PersistentVolumeReclaimPolicy deletes volume on unmount.
	return nil
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
		"pv.kubernetes.io/provisioned-by": provisionerBlobCsiAzure,
	}
	if !useAzureIdentity {
		annotationsMap["volume.kubernetes.io/provisioner-deletion-secret-name"] = csiVolumeCredSecretName
		annotationsMap["volume.kubernetes.io/provisioner-deletion-secret-namespace"] = namespace
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

func getCsiAzurePersistentVolumeAttributes(radixVolumeMount *radixv1.RadixVolumeMount, pvName, namespace string, useAzureIdentity bool, clientId string) map[string]string {
	attributes := make(map[string]string)
	switch GetCsiAzureVolumeMountType(radixVolumeMount) {
	case radixv1.MountTypeBlobFuse2FuseCsiAzure:
		attributes[csiVolumeMountAttributeContainerName] = getRadixBlobFuse2VolumeMountContainerName(radixVolumeMount)
		attributes[csiVolumeMountAttributeProtocol] = csiPersistentVolumeProtocolParameterFuse
	case radixv1.MountTypeBlobFuse2Fuse2CsiAzure:
		attributes[csiVolumeMountAttributeContainerName] = getRadixBlobFuse2VolumeMountContainerName(radixVolumeMount)
		attributes[csiVolumeMountAttributeProtocol] = csiPersistentVolumeProtocolParameterFuse2
		if len(radixVolumeMount.BlobFuse2.StorageAccount) > 0 {
			attributes[csiVolumeMountAttributeStorageAccount] = radixVolumeMount.BlobFuse2.StorageAccount
		}
		if useAzureIdentity {
			attributes[csiVolumeMountAttributeClientID] = clientId
			attributes[csiVolumeMountAttributeResourceGroup] = radixVolumeMount.BlobFuse2.ResourceGroup
		}
	}
	attributes["csi.storage.k8s.io/pv/name"] = pvName
	attributes["csi.storage.k8s.io/pvc/name"] = pvName
	attributes["csi.storage.k8s.io/pvc/namespace"] = namespace
	if !useAzureIdentity {
		attributes["secretnamespace"] = namespace
	}
	// ? storage.kubernetes.io/csiProvisionerIdentity: 1731647415428-2825-blob.csi.azure.com
	return attributes
}

func getCsiAzurePersistentVolumeMountOptions(volumeRootMount, namespace, componentName string, radixVolumeMount *radixv1.RadixVolumeMount) ([]string, error) {
	csiVolumeTypeId, err := getCsiRadixVolumeTypeIdForName(radixVolumeMount)
	if err != nil {
		return nil, err
	}
	tmpPath := fmt.Sprintf(csiVolumeNodeMountPathTemplate, volumeRootMount, namespace, csiVolumeTypeId, componentName, radixVolumeMount.Name, getRadixVolumeMountStorage(radixVolumeMount))
	return getCsiAzurePersistentVolumeMountOptionsForAzureBlob(tmpPath, radixVolumeMount)
}

func getCsiAzurePersistentVolumeMountOptionsForAzureBlob(tmpPath string, radixVolumeMount *radixv1.RadixVolumeMount) ([]string, error) {
	mountOptions := []string{
		// fmt.Sprintf("--%s=%s", csiPersistentVolumeTmpPathMountOption, tmpPath),//TODO fix this path to be able to mount on external mount
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
		mountOptions = append(mountOptions, fmt.Sprintf("-o %s=%s", csiPersistentVolumeGidMountOption, gid))
	} else {
		uid := getRadixBlobFuse2VolumeMountUid(radixVolumeMount)
		if len(uid) > 0 {
			mountOptions = append(mountOptions, fmt.Sprintf("-o %s=%s", csiPersistentVolumeUidMountOption, uid))
		}
	}
	if getVolumeMountAccessMode(radixVolumeMount) == corev1.ReadOnlyMany {
		mountOptions = append(mountOptions, "-o ro")
	}
	if radixVolumeMount.BlobFuse2 != nil {
		mountOptions = append(mountOptions, getStreamingMountOptions(radixVolumeMount.BlobFuse2.Streaming)...)
		mountOptions = append(mountOptions, fmt.Sprintf("--%s=%v", csiPersistentVolumeUseAdlsMountOption, radixVolumeMount.BlobFuse2.UseAdls != nil && *radixVolumeMount.BlobFuse2.UseAdls))
	}
	return mountOptions, nil
}

func getStreamingMountOptions(streaming *radixv1.RadixVolumeMountStreaming) []string {
	var mountOptions []string
	if streaming != nil && streaming.Enabled != nil && !*streaming.Enabled {
		return nil
	}
	mountOptions = append(mountOptions, fmt.Sprintf("--%s=%t", csiPersistentVolumeStreamingEnabledMountOption, true))
	if streaming == nil {
		return mountOptions
	}
	if streaming.StreamCache != nil {
		mountOptions = append(mountOptions, fmt.Sprintf("--%s=%v", csiPersistentVolumeStreamingCacheMountOption, *streaming.StreamCache))
	}
	if streaming.BlockSize != nil {
		mountOptions = append(mountOptions, fmt.Sprintf("--%s=%v", csiPersistentVolumeStreamingBlockSizeMountOption, *streaming.BlockSize))
	}
	if streaming.BufferSize != nil {
		mountOptions = append(mountOptions, fmt.Sprintf("--%s=%v", csiPersistentVolumeStreamingBufferSizeMountOption, *streaming.BufferSize))
	}
	if streaming.MaxBuffers != nil {
		mountOptions = append(mountOptions, fmt.Sprintf("--%s=%v", csiPersistentVolumeStreamingMaxBuffersMountOption, *streaming.MaxBuffers))
	}
	if streaming.MaxBlocksPerFile != nil {
		mountOptions = append(mountOptions, fmt.Sprintf("--%s=%v", csiPersistentVolumeStreamingMaxBlocksPerFileMountOption, *streaming.MaxBlocksPerFile))
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

func (deploy *Deployment) deletePersistentVolumeClaim(ctx context.Context, namespace, pvcName string) error {
	if len(namespace) > 0 && len(pvcName) > 0 {
		return deploy.kubeclient.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
	}
	log.Ctx(ctx).Debug().Msgf("Skip deleting PVC - namespace %s or name %s is empty", namespace, pvcName)
	return nil
}

func (deploy *Deployment) deleteCsiAzurePersistentVolumes(ctx context.Context, pvName string) error {
	if len(pvName) > 0 {
		return deploy.kubeclient.CoreV1().PersistentVolumes().Delete(ctx, pvName, metav1.DeleteOptions{})
	}
	log.Ctx(ctx).Debug().Msg("Skip deleting PersistentVolume - name is empty")
	return nil
}

func (deploy *Deployment) deletePersistentVolume(ctx context.Context, pvName string) error {
	if len(pvName) > 0 {
		return deploy.kubeclient.CoreV1().PersistentVolumes().Delete(ctx, pvName, metav1.DeleteOptions{})
	}
	log.Ctx(ctx).Debug().Msg("Skip deleting PersistentVolume - name is empty")
	return nil
}

func getRadixVolumeMountStorage(radixVolumeMount *radixv1.RadixVolumeMount) string {
	if radixVolumeMount.Type == radixv1.MountTypeBlob {
		return radixVolumeMount.Container // Outdated
	}
	blobFuse2VolumeMountContainer := getRadixBlobFuse2VolumeMountContainerName(radixVolumeMount)
	if len(blobFuse2VolumeMountContainer) != 0 {
		return blobFuse2VolumeMountContainer
	}
	return radixVolumeMount.Storage
}

func (deploy *Deployment) garbageCollectOrphanedCsiAzurePersistentVolumes(ctx context.Context, excludePvcNames map[string]any) error {
	pvList, err := deploy.getPersistentVolumesForPvc(ctx)
	if err != nil {
		return err
	}
	for _, pv := range pvList.Items {
		if pv.Spec.ClaimRef == nil || pv.Spec.ClaimRef.Kind != persistentVolumeClaimKind ||
			!knownCSIDriver(pv.Spec.CSI) ||
			pv.Status.Phase != corev1.VolumeReleased {
			continue
		}
		if _, ok := excludePvcNames[pv.Spec.ClaimRef.Name]; ok {
			continue
		}
		log.Ctx(ctx).Info().Msgf("Delete orphaned Csi Azure PersistantVolume %s of PersistantVolumeClaim %s", pv.Name, pv.Spec.ClaimRef.Name)
		err := deploy.deletePersistentVolume(ctx, pv.Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func knownCSIDriver(csiPersistentVolumeSource *corev1.CSIPersistentVolumeSource) bool {
	if csiPersistentVolumeSource == nil {
		return false
	}
	_, ok := csiVolumeProvisioners[csiPersistentVolumeSource.Driver]
	return ok
}

// createOrUpdateCsiAzureVolumeResources Create or update CSI Azure volume resources - PersistentVolumes, PersistentVolumeClaims, PersistentVolume
func (deploy *Deployment) createOrUpdateCsiAzureVolumeResources(ctx context.Context, desiredDeployment *appsv1.Deployment, deployComponent radixv1.RadixCommonDeployComponent) error {
	namespace := deploy.radixDeployment.GetNamespace()
	appName := deploy.radixDeployment.Spec.AppName
	componentName := desiredDeployment.ObjectMeta.Name
	volumeRootMount := "/tmp" // TODO: add to environment variable, so this volume can be mounted to external disk
	pvList, err := deploy.getCsiAzurePersistentVolume(ctx, namespace, componentName)
	if err != nil {
		return err
	}
	pvcList, err := deploy.getCsiAzurePersistentVolumeClaims(ctx, namespace, componentName)
	if err != nil {
		return err
	}

	pvMap := utils.GetPersistentVolumeMap(&pvList.Items)
	pvcMap := utils.GetPersistentVolumeClaimMap(&pvcList.Items)
	radixVolumeMountMap := deploy.getRadixVolumeMountMapByCsiAzureVolumeMountName(componentName)
	var actualPersistentVolumeNames []string
	actualPvcNames, err := deploy.getCurrentlyUsedPersistentVolumeClaims(ctx, namespace)
	if err != nil {
		return err
	}
	identity := deployComponent.GetIdentity()
	for _, volume := range desiredDeployment.Spec.Template.Spec.Volumes {
		if volume.PersistentVolumeClaim == nil {
			continue
		}
		radixVolumeMount, existsRadixVolumeMount := radixVolumeMountMap[volume.Name]
		if !existsRadixVolumeMount {
			return fmt.Errorf("not found Radix volume mount for desired volume %s", volume.Name)
		}
		pv, pvIsCreated, err := deploy.getOrCreateCsiAzurePersistentVolume(ctx, appName, volumeRootMount, namespace, componentName, radixVolumeMount, volume.Name, pvMap, identity)
		if err != nil {
			return err
		}
		actualPersistentVolumeNames = append(actualPersistentVolumeNames, pv.Name)
		pvc, err := deploy.createCsiAzurePersistentVolumeClaim(ctx, pv, pvIsCreated, appName, namespace, componentName, radixVolumeMount, volume.PersistentVolumeClaim.ClaimName, pvcMap)
		if err != nil {
			return err
		}
		volume.PersistentVolumeClaim.ClaimName = pvc.Name
		actualPvcNames[pvc.Name] = struct{}{}
	}
	err = deploy.garbageCollectCsiAzurePersistentVolumeClaimsAndPersistentVolumes(ctx, namespace, pvcList, actualPvcNames)
	if err != nil {
		return err
	}
	err = deploy.garbageCollectOrphanedCsiAzurePersistentVolumes(ctx, actualPvcNames)
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (deploy *Deployment) getCurrentlyUsedPersistentVolumeClaims(ctx context.Context, namespace string) (map[string]any, error) {
	pvcNames := make(map[string]any)
	deploymentList, err := deploy.kubeclient.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, deployment := range deploymentList.Items {
		addUsedPersistenceVolumeClaimsFrom(deployment.Spec.Template, pvcNames)
	}
	jobsList, err := deploy.kubeclient.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, job := range jobsList.Items {
		addUsedPersistenceVolumeClaimsFrom(job.Spec.Template, pvcNames)
	}
	return pvcNames, nil
}

func addUsedPersistenceVolumeClaimsFrom(podTemplate corev1.PodTemplateSpec, pvcMap map[string]any) {
	for _, volume := range podTemplate.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil && len(volume.PersistentVolumeClaim.ClaimName) > 0 {
			pvcMap[volume.PersistentVolumeClaim.ClaimName] = struct{}{}
		}
	}
}

func (deploy *Deployment) garbageCollectCsiAzurePersistentVolumeClaimsAndPersistentVolumes(ctx context.Context, namespace string, pvcList *corev1.PersistentVolumeClaimList, excludePvcNames map[string]any) error {
	for _, pvc := range pvcList.Items {
		if _, ok := excludePvcNames[pvc.Name]; ok {
			continue
		}
		pvName := pvc.Spec.VolumeName
		log.Ctx(ctx).Debug().Msgf("Delete not used CSI Azure PersistentVolumeClaim %s in namespace %s", pvc.Name, namespace)
		err := deploy.deletePersistentVolumeClaim(ctx, namespace, pvc.Name)
		if err != nil {
			return err
		}
		log.Ctx(ctx).Debug().Msgf("Delete not used CSI Azure PersistentVolume %s in namespace %s", pvName, namespace)
		err = deploy.deletePersistentVolume(ctx, pvName)
		if err != nil {
			return err
		}
	}
	return nil
}

func (deploy *Deployment) createCsiAzurePersistentVolumeClaim(ctx context.Context, pv *corev1.PersistentVolume, requiredNewPvc bool, appName, namespace, componentName string, radixVolumeMount *radixv1.RadixVolumeMount, persistentVolumeClaimName string, pvcMap map[string]*corev1.PersistentVolumeClaim) (*corev1.PersistentVolumeClaim, error) {
	if pvc, ok := pvcMap[persistentVolumeClaimName]; ok {
		if len(pvc.Spec.VolumeName) == 0 {
			return pvc, nil
		}
		if !requiredNewPvc && strings.EqualFold(pvc.Spec.VolumeName, pv.Name) {
			return pvc, nil
		}

		log.Ctx(ctx).Debug().Msgf("Delete in garbage-collect an old PersistentVolumeClaim %s in namespace %s: changed PersistentVolume name to %s", pvc.Name, namespace, pv.Name)
	}
	persistentVolumeClaimName, err := createCsiAzurePersistentVolumeClaimName(componentName, radixVolumeMount)
	if err != nil {
		return nil, err
	}
	log.Ctx(ctx).Debug().Msgf("Create PersistentVolumeClaim %s in namespace %s for PersistentVolume %s", persistentVolumeClaimName, namespace, pv.Name)
	return deploy.createPersistentVolumeClaim(ctx, appName, namespace, componentName, persistentVolumeClaimName, pv.Name, radixVolumeMount)
}

func (deploy *Deployment) getOrCreateCsiAzurePersistentVolume(ctx context.Context, appName, volumeRootMount, namespace, componentName string, radixVolumeMount *radixv1.RadixVolumeMount, volumeName string, pvMap map[string]*corev1.PersistentVolume, identity *radixv1.Identity) (*corev1.PersistentVolume, bool, error) {
	pvName := getCsiAzurePersistentVolumeName(namespace, volumeName)
	if existingPv, exists := pvMap[pvName]; exists {
		desiredPv := existingPv.DeepCopy()
		err := populateCsiAzurePersistentVolume(desiredPv, appName, volumeRootMount, namespace, componentName, pvName, radixVolumeMount, identity)
		if err != nil {
			return nil, false, err
		}
		if equal, err := utils.EqualPersistentVolumes(existingPv, desiredPv); equal || err != nil {
			return existingPv, false, err
		}

		log.Ctx(ctx).Info().Msgf("Delete PersistentVolume %s in namespace %s", existingPv.Name, namespace)
		err = deploy.deleteCsiAzurePersistentVolumes(ctx, existingPv.Name)
		if err != nil {
			return nil, false, err
		}
	}

	log.Ctx(ctx).Debug().Msgf("Create PersistentVolume %s in namespace %s", pvName, namespace)
	newPv := &corev1.PersistentVolume{}
	err := populateCsiAzurePersistentVolume(newPv, appName, volumeRootMount, namespace, componentName, pvName, radixVolumeMount, identity)
	if err != nil {
		return nil, false, err
	}
	desiredPersistentVolume, err := deploy.kubeclient.CoreV1().PersistentVolumes().Create(ctx, newPv, metav1.CreateOptions{})
	return desiredPersistentVolume, true, err
}

func (deploy *Deployment) getRadixVolumeMountMapByCsiAzureVolumeMountName(componentName string) map[string]*radixv1.RadixVolumeMount {
	volumeMountMap := make(map[string]*radixv1.RadixVolumeMount)
	for _, component := range deploy.radixDeployment.Spec.Components {
		if findCsiAzureVolumeForComponent(volumeMountMap, component.VolumeMounts, componentName, &component) {
			break
		}
	}
	for _, component := range deploy.radixDeployment.Spec.Jobs {
		if findCsiAzureVolumeForComponent(volumeMountMap, component.VolumeMounts, componentName, &component) {
			break
		}
	}
	return volumeMountMap
}

func findCsiAzureVolumeForComponent(volumeMountMap map[string]*radixv1.RadixVolumeMount, volumeMounts []radixv1.RadixVolumeMount, componentName string, component radixv1.RadixCommonDeployComponent) bool {
	if !strings.EqualFold(componentName, component.GetName()) {
		return false
	}
	for _, radixVolumeMount := range volumeMounts {
		if radixVolumeMount.BlobFuse2 == nil && !isKnownCsiAzureVolumeMount(string(GetCsiAzureVolumeMountType(&radixVolumeMount))) {
			continue
		}
		radixVolumeMount := radixVolumeMount
		volumeMountName, err := getCsiAzureVolumeMountName(&radixVolumeMount, componentName)
		if err != nil {
			return false
		}
		volumeMountMap[volumeMountName] = &radixVolumeMount
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
	return fmt.Sprintf("%s-%s", volumeName[:63-randSize-1], randString)
}

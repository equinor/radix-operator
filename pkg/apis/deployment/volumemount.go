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
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
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
	csiStorageClassNameTemplate          = "sc-%s-%s"          // sc-<namespace>-<volumename>
	csiVolumeNodeMountPathTemplate       = "%s/%s/%s/%s/%s/%s" // <volumeRootMount>/<namespace>/<radixvolumeid>/<componentname>/<radixvolumename>/<storage>

	csiStorageClassProvisionerSecretNameParameter       = "csi.storage.k8s.io/provisioner-secret-name"      // Secret name, containing storage account name and key
	csiStorageClassProvisionerSecretNamespaceParameter  = "csi.storage.k8s.io/provisioner-secret-namespace" // Namespace of the secret
	csiStorageClassNodeStageSecretNameParameter         = "csi.storage.k8s.io/node-stage-secret-name"       // Usually equal to csiStorageClassProvisionerSecretNameParameter
	csiStorageClassNodeStageSecretNamespaceParameter    = "csi.storage.k8s.io/node-stage-secret-namespace"  // Usually equal to csiStorageClassProvisionerSecretNamespaceParameter
	csiAzureStorageClassSkuNameParameter                = "skuName"                                         // Available values: Standard_LRS (default), Premium_LRS, Standard_GRS, Standard_RAGRS. https://docs.microsoft.com/en-us/rest/api/storagerp/srp_sku_types
	csiStorageClassContainerNameParameter               = "containerName"                                   // Container name - foc container storages
	csiStorageClassShareNameParameter                   = "shareName"                                       // File Share name - for file storages
	csiStorageClassTmpPathMountOption                   = "tmp-path"                                        // Path within the node, where the volume mount has been mounted to
	csiStorageClassGidMountOption                       = "gid"                                             // Volume mount owner GroupID. Used when drivers do not honor fsGroup securityContext setting
	csiStorageClassUidMountOption                       = "uid"                                             // Volume mount owner UserID. Used instead of GroupID
	csiStorageClassUseAdlsMountOption                   = "use-adls"                                        // Use ADLS or Block Blob
	csiStorageClassStreamingEnabledMountOption          = "streaming"                                       // Enable Streaming
	csiStorageClassStreamingCacheMountOption            = "stream-cache-mb"                                 // Limit total amount of data being cached in memory to conserve memory
	csiStorageClassStreamingMaxBlocksPerFileMountOption = "max-blocks-per-file"                             // Maximum number of blocks to be cached in memory for streaming
	csiStorageClassStreamingMaxBuffersMountOption       = "max-buffers"                                     // The total number of buffers to be cached in memory (in MB).
	csiStorageClassStreamingBlockSizeMountOption        = "block-size-mb"                                   // The size of each block to be cached in memory (in MB).
	csiStorageClassStreamingBufferSizeMountOption       = "buffer-size-mb"                                  // The size of each buffer to be cached in memory (in MB).
	csiStorageClassStreamingFileCachingMountOption      = "file-caching"                                    // File name based caching. Default is false which specifies file handle based caching.
	csiStorageClassProtocolParameter                    = "protocol"                                        // Protocol
	csiStorageClassProtocolParameterFuse                = "fuse"                                            // Protocol "blobfuse"
	csiStorageClassProtocolParameterFuse2               = "fuse2"                                           // Protocol "blobfuse2"
	csiStorageClassProtocolParameterNfs                 = "nfs"                                             // Protocol "nfs"

	csiSecretStoreDriver                             = "secrets-store.csi.k8s.io"
	csiVolumeSourceVolumeAttrSecretProviderClassName = "secretProviderClass"
	csiAzureKeyVaultSecretMountPathTemplate          = "/mnt/azure-key-vault/%s"

	volumeNameMaxLength = 63
)

// GetRadixDeployComponentVolumeMounts Gets list of v1.VolumeMount for radixv1.RadixCommonDeployComponent
func GetRadixDeployComponentVolumeMounts(deployComponent radixv1.RadixCommonDeployComponent, radixDeploymentName string) ([]corev1.VolumeMount, error) {
	componentName := deployComponent.GetName()
	volumeMounts := make([]corev1.VolumeMount, 0)
	externalVolumeMounts, err := getRadixComponentExternalVolumeMounts(deployComponent, componentName)
	if err != nil {
		return nil, err
	}
	volumeMounts = append(volumeMounts, externalVolumeMounts...)
	secretRefsVolumeMounts := getRadixComponentSecretRefsVolumeMounts(deployComponent, componentName, radixDeploymentName)
	volumeMounts = append(volumeMounts, secretRefsVolumeMounts...)
	return volumeMounts, nil
}

func getRadixComponentExternalVolumeMounts(deployComponent radixv1.RadixCommonDeployComponent, componentName string) ([]corev1.VolumeMount, error) {
	if isDeployComponentJobSchedulerDeployment(deployComponent) {
		return nil, nil
	}

	var volumeMounts []corev1.VolumeMount
	for _, radixVolumeMount := range deployComponent.GetVolumeMounts() {
		switch GetCsiAzureVolumeMountType(&radixVolumeMount) {
		case radixv1.MountTypeBlob:
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      getBlobFuseVolumeMountName(radixVolumeMount, componentName),
				MountPath: radixVolumeMount.Path,
			})
		case radixv1.MountTypeAzureFileCsiAzure, radixv1.MountTypeBlobFuse2FuseCsiAzure, radixv1.MountTypeBlobFuse2Fuse2CsiAzure, radixv1.MountTypeBlobFuse2NfsCsiAzure:
			volumeMountName, err := getCsiAzureVolumeMountName(componentName, &radixVolumeMount)
			if err != nil {
				return nil, err
			}
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      volumeMountName,
				MountPath: radixVolumeMount.Path,
			})
		}
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

func getBlobFuseVolumeMountName(volumeMount radixv1.RadixVolumeMount, componentName string) string {
	return trimVolumeNameToValidLength(fmt.Sprintf(blobFuseVolumeNameTemplate, componentName, volumeMount.Name))
}

func getCsiAzureVolumeMountName(componentName string, radixVolumeMount *radixv1.RadixVolumeMount) (string, error) {
	csiVolumeType, err := getCsiRadixVolumeTypeIdForName(radixVolumeMount)
	if err != nil {
		return "", err
	}
	if len(radixVolumeMount.Name) == 0 {
		return "", fmt.Errorf("name is empty for volume mount in the component %s", componentName)
	}
	csiAzureVolumeStorageName := GetRadixVolumeMountStorage(radixVolumeMount)
	if len(csiAzureVolumeStorageName) == 0 {
		return "", fmt.Errorf("storage is empty for volume mount %s in the component %s", radixVolumeMount.Name, componentName)
	}
	if len(radixVolumeMount.Path) == 0 {
		return "", fmt.Errorf("path is empty for volume mount %s in the component %s", radixVolumeMount.Name, componentName)
	}
	return trimVolumeNameToValidLength(fmt.Sprintf(csiVolumeNameTemplate, csiVolumeType, componentName, radixVolumeMount.Name, csiAzureVolumeStorageName)), nil
}

// GetCsiAzureVolumeMountType Gets the CSI Azure volume mount type
func GetCsiAzureVolumeMountType(radixVolumeMount *radixv1.RadixVolumeMount) radixv1.MountType {
	if radixVolumeMount.BlobFuse2 != nil {
		switch radixVolumeMount.BlobFuse2.Protocol {
		case radixv1.BlobFuse2ProtocolFuse2, "": // default protocol if not set
			return radixv1.MountTypeBlobFuse2Fuse2CsiAzure
		case radixv1.BlobFuse2ProtocolNfs:
			return radixv1.MountTypeBlobFuse2NfsCsiAzure
		default:
			return "unsupported"
		}
	}
	if radixVolumeMount.AzureFile != nil {
		return radixv1.MountTypeAzureFileCsiAzure
	}
	return radixVolumeMount.Type
}

func getCsiRadixVolumeTypeIdForName(radixVolumeMount *radixv1.RadixVolumeMount) (string, error) {
	if radixVolumeMount.BlobFuse2 != nil {
		switch radixVolumeMount.BlobFuse2.Protocol {
		case radixv1.BlobFuse2ProtocolFuse2, "":
			return "csi-blobfuse2-fuse2", nil
		case radixv1.BlobFuse2ProtocolNfs:
			return "csi-blobfuse2-nfs", nil
		default:
			return "", fmt.Errorf("unknown blobfuse2 protocol %s", radixVolumeMount.BlobFuse2.Protocol)
		}
	}
	if radixVolumeMount.AzureFile != nil {
		return "csi-az-file", nil
	}
	switch radixVolumeMount.Type {
	case radixv1.MountTypeBlobFuse2FuseCsiAzure:
		return "csi-az-blob", nil
	case radixv1.MountTypeAzureFileCsiAzure:
		return "csi-az-file", nil
	}
	return "", fmt.Errorf("unknown volume mount type %s", radixVolumeMount.Type)
}

// GetVolumesForComponent Gets volumes for Radix deploy component or job
func (deploy *Deployment) GetVolumesForComponent(deployComponent radixv1.RadixCommonDeployComponent) ([]corev1.Volume, error) {
	return GetVolumes(deploy.kubeclient, deploy.kubeutil, deploy.getNamespace(), deploy.radixDeployment.Spec.Environment, deployComponent, deploy.radixDeployment.GetName())
}

// GetVolumes Get volumes of a component by RadixVolumeMounts
func GetVolumes(kubeclient kubernetes.Interface, kubeutil *kube.Kube, namespace string, environment string, deployComponent radixv1.RadixCommonDeployComponent, radixDeploymentName string) ([]corev1.Volume, error) {
	var volumes []corev1.Volume

	blobVolumes, err := getExternalVolumes(kubeclient, namespace, environment, deployComponent)
	if err != nil {
		return nil, err
	}
	volumes = append(volumes, blobVolumes...)

	storageRefsVolumes, err := getStorageRefsVolumes(kubeutil, namespace, deployComponent, radixDeploymentName)
	if err != nil {
		return nil, err
	}
	volumes = append(volumes, storageRefsVolumes...)

	return volumes, nil
}

func getStorageRefsVolumes(kubeutil *kube.Kube, namespace string, deployComponent radixv1.RadixCommonDeployComponent, radixDeploymentName string) ([]corev1.Volume, error) {
	var volumes []corev1.Volume
	azureKeyVaultVolumes, err := getStorageRefsAzureKeyVaultVolumes(kubeutil, namespace, deployComponent, radixDeploymentName)
	if err != nil {
		return nil, err
	}
	volumes = append(volumes, azureKeyVaultVolumes...)
	return volumes, nil
}

func getStorageRefsAzureKeyVaultVolumes(kubeutil *kube.Kube, namespace string, deployComponent radixv1.RadixCommonDeployComponent, radixDeploymentName string) ([]corev1.Volume, error) {
	secretRef := deployComponent.GetSecretRefs()
	var volumes []corev1.Volume
	for _, azureKeyVault := range secretRef.AzureKeyVaults {
		secretProviderClassName := kube.GetComponentSecretProviderClassName(radixDeploymentName, deployComponent.GetName(), radixv1.RadixSecretRefTypeAzureKeyVault, azureKeyVault.Name)
		secretProviderClass, err := kubeutil.GetSecretProviderClass(namespace, secretProviderClassName)
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
				log.Errorf("not supported provider %s in the secret provider class %s", provider, secretProviderClass.Name)
				continue
			}
			volumes = append(volumes, volume)
		}
	}
	return volumes, nil
}

func getExternalVolumes(kubeclient kubernetes.Interface, namespace string, environment string, deployComponent radixv1.RadixCommonDeployComponent) ([]corev1.Volume, error) {
	var volumes []corev1.Volume
	for _, volumeMount := range deployComponent.GetVolumeMounts() {
		volumeMountType := GetCsiAzureVolumeMountType(&volumeMount)
		switch volumeMountType {
		case radixv1.MountTypeBlob:
			volumes = append(volumes, getBlobFuseVolume(namespace, environment, deployComponent.GetName(), volumeMount))
		case radixv1.MountTypeBlobFuse2FuseCsiAzure, radixv1.MountTypeBlobFuse2Fuse2CsiAzure, radixv1.MountTypeBlobFuse2NfsCsiAzure, radixv1.MountTypeAzureFileCsiAzure:
			volume, err := getCsiAzureVolume(kubeclient, namespace, deployComponent.GetName(), &volumeMount)
			if err != nil {
				return nil, err
			}
			volumes = append(volumes, *volume)
		default:
			return nil, fmt.Errorf("unsupported volume type %s", volumeMountType)
		}
	}
	return volumes, nil
}

func getCsiAzureVolume(kubeclient kubernetes.Interface, namespace, componentName string, radixVolumeMount *radixv1.RadixVolumeMount) (*corev1.Volume, error) {
	existingNotTerminatingPvcForComponentStorage, err := getPvcNotTerminating(kubeclient, namespace, componentName, radixVolumeMount)
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
	volumeMountName, err := getCsiAzureVolumeMountName(componentName, radixVolumeMount)
	if err != nil {
		return nil, err
	}
	return &corev1.Volume{
		Name: volumeMountName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
			},
		},
	}, nil
}

func getPvcNotTerminating(kubeclient kubernetes.Interface, namespace string, componentName string, radixVolumeMount *radixv1.RadixVolumeMount) (*corev1.PersistentVolumeClaim, error) {
	existingPvcForComponentStorage, err := kubeclient.CoreV1().PersistentVolumeClaims(namespace).List(context.TODO(), metav1.ListOptions{
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
	volumeName, err := getCsiAzureVolumeMountName(componentName, radixVolumeMount)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(csiPersistentVolumeClaimNameTemplate, volumeName, strings.ToLower(commonUtils.RandString(5))), nil // volumeName: <component-name>-<csi-volume-type-dashed>-<radix-volume-name>-<storage-name>
}

// GetCsiAzureStorageClassName hold a name of CSI volume storage class
func GetCsiAzureStorageClassName(namespace, volumeName string) string {
	return fmt.Sprintf(csiStorageClassNameTemplate, namespace, volumeName) // volumeName: <component-name>-<csi-volume-type-dashed>-<radix-volume-name>-<storage-name>
}

func getBlobFuseVolume(namespace, environment, componentName string, volumeMount radixv1.RadixVolumeMount) corev1.Volume {
	secretName := defaults.GetBlobFuseCredsSecretName(componentName, volumeMount.Name)

	flexVolumeOptions := make(map[string]string)
	flexVolumeOptions["name"] = volumeMount.Name
	flexVolumeOptions["container"] = volumeMount.Container
	flexVolumeOptions["mountoptions"] = defaultMountOptions
	flexVolumeOptions["tmppath"] = fmt.Sprintf(blobFuseVolumeNodeMountPathTemplate, namespace, componentName, environment, radixv1.MountTypeBlob, volumeMount.Name, volumeMount.Container)

	return corev1.Volume{
		Name: getBlobFuseVolumeMountName(volumeMount, componentName),
		VolumeSource: corev1.VolumeSource{
			FlexVolume: &corev1.FlexVolumeSource{
				Driver:  blobfuseDriver,
				Options: flexVolumeOptions,
				SecretRef: &corev1.LocalObjectReference{
					Name: secretName,
				},
			},
		},
	}
}

func (deploy *Deployment) createOrUpdateVolumeMountsSecrets(namespace, componentName, secretName string, accountName, accountKey []byte) error {
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

	_, err := deploy.kubeutil.ApplySecret(namespace, &blobfusecredsSecret)
	if err != nil {
		return err
	}

	return nil
}
func (deploy *Deployment) createOrUpdateCsiAzureVolumeMountsSecrets(namespace, componentName string, radixVolumeMount *radixv1.RadixVolumeMount, secretName string, accountName, accountKey []byte) error {
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

	_, err := deploy.kubeutil.ApplySecret(namespace, &secret)
	if err != nil {
		return err
	}

	return nil
}

func (deploy *Deployment) garbageCollectVolumeMountsSecretsNoLongerInSpecForComponent(component radixv1.RadixCommonDeployComponent, excludeSecretNames []string) error {
	secrets, err := deploy.listSecretsForVolumeMounts(component)
	if err != nil {
		return err
	}
	return deploy.GarbageCollectSecrets(secrets, excludeSecretNames)
}

func (deploy *Deployment) getCsiAzureStorageClasses(namespace, componentName string) (*storagev1.StorageClassList, error) {
	return deploy.kubeclient.StorageV1().StorageClasses().List(context.TODO(), metav1.ListOptions{
		LabelSelector: getLabelSelectorForCsiAzureStorageClass(namespace, componentName),
	})
}

func (deploy *Deployment) getCsiAzurePersistentVolumeClaims(namespace, componentName string) (*corev1.PersistentVolumeClaimList, error) {
	return deploy.kubeclient.CoreV1().PersistentVolumeClaims(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: getLabelSelectorForCsiAzurePersistenceVolumeClaim(componentName),
	})
}

func (deploy *Deployment) getPersistentVolumesForPvc() (*corev1.PersistentVolumeList, error) {
	return deploy.kubeclient.CoreV1().PersistentVolumes().List(context.TODO(), metav1.ListOptions{})
}

func getLabelSelectorForCsiAzureStorageClass(namespace, componentName string) string {
	return fmt.Sprintf("%s=%s, %s=%s, %s in (%s, %s, %s, %s)", kube.RadixNamespace, namespace, kube.RadixComponentLabel, componentName, kube.RadixMountTypeLabel, string(radixv1.MountTypeBlobFuse2FuseCsiAzure), string(radixv1.MountTypeBlobFuse2Fuse2CsiAzure), string(radixv1.MountTypeBlobFuse2NfsCsiAzure), string(radixv1.MountTypeAzureFileCsiAzure))
}

func getLabelSelectorForCsiAzurePersistenceVolumeClaim(componentName string) string {
	return fmt.Sprintf("%s=%s, %s in (%s, %s, %s, %s)", kube.RadixComponentLabel, componentName, kube.RadixMountTypeLabel, string(radixv1.MountTypeBlobFuse2FuseCsiAzure), string(radixv1.MountTypeBlobFuse2Fuse2CsiAzure), string(radixv1.MountTypeBlobFuse2NfsCsiAzure), string(radixv1.MountTypeAzureFileCsiAzure))
}

func getLabelSelectorForCsiAzurePersistenceVolumeClaimForComponentStorage(componentName, radixVolumeMountName string) string {
	return fmt.Sprintf("%s=%s, %s in (%s, %s, %s, %s), %s = %s", kube.RadixComponentLabel, componentName, kube.RadixMountTypeLabel, string(radixv1.MountTypeBlobFuse2FuseCsiAzure), string(radixv1.MountTypeBlobFuse2Fuse2CsiAzure), string(radixv1.MountTypeBlobFuse2NfsCsiAzure), string(radixv1.MountTypeAzureFileCsiAzure), kube.RadixVolumeMountNameLabel, radixVolumeMountName)
}

func (deploy *Deployment) createPersistentVolumeClaim(appName, namespace, componentName, pvcName, storageClassName string, radixVolumeMount *radixv1.RadixVolumeMount) (*corev1.PersistentVolumeClaim, error) {
	requestsVolumeMountSize, err := resource.ParseQuantity(getRadixBlobFuse2VolumeMountRequestsStorage(radixVolumeMount))
	if err != nil {
		requestsVolumeMountSize = resource.MustParse("1Mi")
	}
	volumeAccessMode := getVolumeAccessMode(getRadixBlobFuse2VolumeMountAccessMode(radixVolumeMount))
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
			AccessModes: []corev1.PersistentVolumeAccessMode{volumeAccessMode},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: requestsVolumeMountSize}, // it seems correct number is not needed for CSI driver
			},
			StorageClassName: &storageClassName,
		},
	}
	return deploy.kubeclient.CoreV1().PersistentVolumeClaims(namespace).Create(context.TODO(), pvc, metav1.CreateOptions{})
}

func populateCsiAzureStorageClass(storageClass *storagev1.StorageClass, appName string, volumeRootMount string, namespace string, componentName string, storageClassName string, radixVolumeMount *radixv1.RadixVolumeMount, secretName string, provisioner string) error {
	reclaimPolicy := corev1.PersistentVolumeReclaimRetain // Using only PersistentVolumeReclaimPolicy. PersistentVolumeReclaimPolicy deletes volume on unmount.
	bindingMode := getBindingMode(getRadixBlobFuse2VolumeMountBindingMode(radixVolumeMount))
	storageClass.ObjectMeta.Name = storageClassName
	storageClass.ObjectMeta.Labels = getCsiAzureStorageClassLabels(appName, namespace, componentName, radixVolumeMount)
	storageClass.Provisioner = provisioner
	storageClass.Parameters = getCsiAzureStorageClassParameters(secretName, namespace, radixVolumeMount)
	mountOptions, err := getCsiAzureStorageClassMountOptions(volumeRootMount, namespace, componentName, radixVolumeMount)
	if err != nil {
		return err
	}
	storageClass.MountOptions = mountOptions
	storageClass.ReclaimPolicy = &reclaimPolicy
	storageClass.VolumeBindingMode = &bindingMode
	return nil
}

func getBindingMode(bindingModeValue string) storagev1.VolumeBindingMode {
	if strings.EqualFold(strings.ToLower(bindingModeValue), strings.ToLower(string(storagev1.VolumeBindingWaitForFirstConsumer))) {
		return storagev1.VolumeBindingWaitForFirstConsumer
	}
	return storagev1.VolumeBindingImmediate
}

func getCsiAzureStorageClassLabels(appName, namespace, componentName string, radixVolumeMount *radixv1.RadixVolumeMount) map[string]string {
	return map[string]string{
		kube.RadixAppLabel:             appName,
		kube.RadixNamespace:            namespace,
		kube.RadixComponentLabel:       componentName,
		kube.RadixMountTypeLabel:       string(GetCsiAzureVolumeMountType(radixVolumeMount)),
		kube.RadixVolumeMountNameLabel: radixVolumeMount.Name,
	}
}

func getCsiAzureStorageClassParameters(secretName string, namespace string, radixVolumeMount *radixv1.RadixVolumeMount) map[string]string {
	parameters := map[string]string{
		csiStorageClassProvisionerSecretNameParameter:      secretName,
		csiStorageClassProvisionerSecretNamespaceParameter: namespace,
		csiStorageClassNodeStageSecretNameParameter:        secretName,
		csiStorageClassNodeStageSecretNamespaceParameter:   namespace,
	}
	skuName := getRadixBlobFuse2VolumeMountSkuName(radixVolumeMount)
	if len(skuName) > 0 {
		parameters[csiAzureStorageClassSkuNameParameter] = skuName
	}
	switch GetCsiAzureVolumeMountType(radixVolumeMount) {
	case radixv1.MountTypeBlobFuse2FuseCsiAzure:
		parameters[csiStorageClassContainerNameParameter] = getRadixBlobFuse2VolumeMountContainerName(radixVolumeMount)
		parameters[csiStorageClassProtocolParameter] = csiStorageClassProtocolParameterFuse
	case radixv1.MountTypeBlobFuse2Fuse2CsiAzure:
		parameters[csiStorageClassContainerNameParameter] = getRadixBlobFuse2VolumeMountContainerName(radixVolumeMount)
		parameters[csiStorageClassProtocolParameter] = csiStorageClassProtocolParameterFuse2
	case radixv1.MountTypeBlobFuse2NfsCsiAzure:
		parameters[csiStorageClassContainerNameParameter] = getRadixBlobFuse2VolumeMountContainerName(radixVolumeMount)
		parameters[csiStorageClassProtocolParameter] = csiStorageClassProtocolParameterNfs
	case radixv1.MountTypeAzureFileCsiAzure:
		parameters[csiStorageClassShareNameParameter] = getRadixAzureFileVolumeMountShareName(radixVolumeMount)
	}
	return parameters
}

func getCsiAzureStorageClassMountOptions(volumeRootMount, namespace, componentName string, radixVolumeMount *radixv1.RadixVolumeMount) ([]string, error) {
	csiVolumeTypeId, err := getCsiRadixVolumeTypeIdForName(radixVolumeMount)
	if err != nil {
		return nil, err
	}
	tmpPath := fmt.Sprintf(csiVolumeNodeMountPathTemplate, volumeRootMount, namespace, csiVolumeTypeId, componentName, radixVolumeMount.Name, GetRadixVolumeMountStorage(radixVolumeMount))
	return getCsiAzureStorageClassMountOptionsForAzureBlob(tmpPath, radixVolumeMount)
}

func getCsiAzureStorageClassMountOptionsForAzureBlob(tmpPath string, radixVolumeMount *radixv1.RadixVolumeMount) ([]string, error) {
	mountOptions := []string{
		// fmt.Sprintf("--%s=%s", csiStorageClassTmpPathMountOption, tmpPath),//TODO fix this path to be able to mount on external mount
		"--file-cache-timeout-in-seconds=120",
		"--use-attr-cache=true",
		"-o allow_other",
		"-o attr_timeout=120",
		"-o entry_timeout=120",
		"-o negative_timeout=120",
	}
	gid := getRadixBlobFuse2VolumeMountGid(radixVolumeMount)
	if len(gid) > 0 {
		mountOptions = append(mountOptions, fmt.Sprintf("-o %s=%s", csiStorageClassGidMountOption, gid))
	} else {
		uid := getRadixBlobFuse2VolumeMountUid(radixVolumeMount)
		if len(uid) > 0 {
			mountOptions = append(mountOptions, fmt.Sprintf("-o %s=%s", csiStorageClassUidMountOption, uid))
		}
	}
	if getRadixBlobFuse2VolumeMountAccessMode(radixVolumeMount) == string(corev1.ReadOnlyMany) {
		mountOptions = append(mountOptions, "-o ro")
	}
	if radixVolumeMount.BlobFuse2 != nil {
		if radixVolumeMount.BlobFuse2.Streaming != nil {
			streaming := radixVolumeMount.BlobFuse2.Streaming
			if streaming.Enabled == nil || *streaming.Enabled {
				mountOptions = append(mountOptions, fmt.Sprintf("--%s=%t", csiStorageClassStreamingEnabledMountOption, true))
				if streaming.StreamCache != nil {
					mountOptions = append(mountOptions, fmt.Sprintf("--%s=%v", csiStorageClassStreamingCacheMountOption, *streaming.StreamCache))
				}
				if streaming.BlockSize != nil {
					mountOptions = append(mountOptions, fmt.Sprintf("--%s=%v", csiStorageClassStreamingBlockSizeMountOption, *streaming.BlockSize))
				}
				if streaming.BufferSize != nil {
					mountOptions = append(mountOptions, fmt.Sprintf("--%s=%v", csiStorageClassStreamingBufferSizeMountOption, *streaming.BufferSize))
				}
				if streaming.MaxBuffers != nil {
					mountOptions = append(mountOptions, fmt.Sprintf("--%s=%v", csiStorageClassStreamingMaxBuffersMountOption, *streaming.MaxBuffers))
				}
				if streaming.MaxBlocksPerFile != nil {
					mountOptions = append(mountOptions, fmt.Sprintf("--%s=%v", csiStorageClassStreamingMaxBlocksPerFileMountOption, *streaming.MaxBlocksPerFile))
				}
				if streaming.FileCaching != nil {
					mountOptions = append(mountOptions, fmt.Sprintf("--%s=%v", csiStorageClassStreamingFileCachingMountOption, *streaming.FileCaching))
				}
			}
		}
		mountOptions = append(mountOptions, fmt.Sprintf("--%s=%v", csiStorageClassUseAdlsMountOption, radixVolumeMount.BlobFuse2.UseAdls != nil && *radixVolumeMount.BlobFuse2.UseAdls))
	}
	return mountOptions, nil
}

func getRadixBlobFuse2VolumeMountAccessMode(radixVolumeMount *radixv1.RadixVolumeMount) string {
	if radixVolumeMount.BlobFuse2 != nil {
		return radixVolumeMount.BlobFuse2.AccessMode
	}
	return radixVolumeMount.AccessMode
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

func getRadixAzureFileVolumeMountShareName(radixVolumeMount *radixv1.RadixVolumeMount) string {
	if radixVolumeMount.AzureFile != nil {
		return radixVolumeMount.AzureFile.Share
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

func (deploy *Deployment) deletePersistentVolumeClaim(namespace, pvcName string) error {
	if len(namespace) > 0 && len(pvcName) > 0 {
		return deploy.kubeclient.CoreV1().PersistentVolumeClaims(namespace).Delete(context.TODO(), pvcName, metav1.DeleteOptions{})
	}
	log.Debugf("Skip deleting PVC - namespace %s or name %s is empty", namespace, pvcName)
	return nil
}

func (deploy *Deployment) deleteCsiAzureStorageClasses(storageClassName string) error {
	if len(storageClassName) > 0 {
		return deploy.kubeclient.StorageV1().StorageClasses().Delete(context.TODO(), storageClassName, metav1.DeleteOptions{})
	}
	log.Debugf("Skip deleting StorageClass - name is empty")
	return nil
}

func (deploy *Deployment) deletePersistentVolume(pvName string) error {
	if len(pvName) > 0 {
		return deploy.kubeclient.CoreV1().PersistentVolumes().Delete(context.TODO(), pvName, metav1.DeleteOptions{})
	}
	log.Debugf("Skip deleting PersistentVolume - name is empty")
	return nil
}

// GetRadixVolumeMountStorage get RadixVolumeMount storage property, depend on volume type
func GetRadixVolumeMountStorage(radixVolumeMount *radixv1.RadixVolumeMount) string {
	if radixVolumeMount.Type == radixv1.MountTypeBlob {
		return radixVolumeMount.Container // Outdated
	}
	blobFuse2VolumeMountContainer := getRadixBlobFuse2VolumeMountContainerName(radixVolumeMount)
	if len(blobFuse2VolumeMountContainer) != 0 {
		return blobFuse2VolumeMountContainer
	}
	azureFileVolumeMountShare := getRadixAzureFileVolumeMountShareName(radixVolumeMount)
	if len(azureFileVolumeMountShare) != 0 {
		return azureFileVolumeMountShare
	}
	return radixVolumeMount.Storage
}

func (deploy *Deployment) garbageCollectOrphanedCsiAzurePersistentVolumes(excludePvcNames []string) error {
	pvList, err := deploy.getPersistentVolumesForPvc()
	if err != nil {
		return err
	}
	csiAzureStorageClassProvisioners := radixv1.GetCsiAzureStorageClassProvisioners()
	for _, pv := range pvList.Items {
		switch {
		case pv.Spec.ClaimRef == nil || pv.Spec.ClaimRef.Kind != persistentVolumeClaimKind:
			continue
		case pv.Spec.CSI == nil || !slice.ContainsString(csiAzureStorageClassProvisioners, pv.Spec.CSI.Driver):
			continue
		case slice.ContainsString(excludePvcNames, pv.Spec.ClaimRef.Name):
			continue
		case pv.Status.Phase != corev1.VolumeReleased:
			continue
		}
		log.Infof("Delete orphaned Csi Azure PersistantVolume %s of PersistantVolumeClaim %s", pv.Name, pv.Spec.ClaimRef.Name)
		err := deploy.deletePersistentVolume(pv.Name)
		if err != nil {
			return err
		}
	}
	return nil
}

// createOrUpdateCsiAzureVolumeResources Create or update CSI Azure volume resources - StorageClasses, PersistentVolumeClaims, PersistentVolume
func (deploy *Deployment) createOrUpdateCsiAzureVolumeResources(desiredDeployment *appsv1.Deployment) error {
	namespace := deploy.radixDeployment.GetNamespace()
	appName := deploy.radixDeployment.Spec.AppName
	componentName := desiredDeployment.ObjectMeta.Name
	volumeRootMount := "/tmp" // TODO: add to environment variable, so this volume can be mounted to external disk
	scList, err := deploy.getCsiAzureStorageClasses(namespace, componentName)
	if err != nil {
		return err
	}
	pvcList, err := deploy.getCsiAzurePersistentVolumeClaims(namespace, componentName)
	if err != nil {
		return err
	}

	scMap := utils.GetStorageClassMap(&scList.Items)
	pvcMap := utils.GetPersistentVolumeClaimMap(&pvcList.Items)
	radixVolumeMountMap := deploy.getRadixVolumeMountMapByCsiAzureVolumeMountName(componentName)
	var actualStorageClassNames, actualPvcNames []string
	for _, volume := range desiredDeployment.Spec.Template.Spec.Volumes {
		if volume.PersistentVolumeClaim == nil {
			continue
		}
		radixVolumeMount, existsRadixVolumeMount := radixVolumeMountMap[volume.Name]
		if !existsRadixVolumeMount {
			return fmt.Errorf("not found Radix volume mount for desired volume %s", volume.Name)
		}
		storageClass, storageClassIsCreated, err := deploy.getOrCreateCsiAzureVolumeMountStorageClass(appName, volumeRootMount, namespace, componentName, radixVolumeMount, volume.Name, scMap)
		if err != nil {
			return err
		}
		actualStorageClassNames = append(actualStorageClassNames, storageClass.Name)
		pvc, err := deploy.createCsiAzurePersistentVolumeClaim(storageClass, storageClassIsCreated, appName, namespace, componentName, radixVolumeMount, volume.PersistentVolumeClaim.ClaimName, pvcMap)
		if err != nil {
			return err
		}
		volume.PersistentVolumeClaim.ClaimName = pvc.Name
		actualPvcNames = append(actualPvcNames, pvc.Name)
	}
	err = deploy.garbageCollectCsiAzureStorageClasses(scList, actualStorageClassNames)
	if err != nil {
		return err
	}
	err = deploy.garbageCollectCsiAzurePersistentVolumeClaimsAndPersistentVolumes(namespace, pvcList, actualPvcNames)
	if err != nil {
		return err
	}
	err = deploy.garbageCollectOrphanedCsiAzurePersistentVolumes(actualPvcNames)
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (deploy *Deployment) garbageCollectCsiAzureStorageClasses(scList *storagev1.StorageClassList, excludeStorageClassName []string) error {
	for _, storageClass := range scList.Items {
		if slice.ContainsString(excludeStorageClassName, storageClass.Name) {
			continue
		}
		log.Debugf("Delete Csi Azure StorageClass %s", storageClass.Name)
		err := deploy.deleteCsiAzureStorageClasses(storageClass.Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func (deploy *Deployment) garbageCollectCsiAzurePersistentVolumeClaimsAndPersistentVolumes(namespace string, pvcList *corev1.PersistentVolumeClaimList, excludePvcNames []string) error {
	for _, pvc := range pvcList.Items {
		if slice.ContainsString(excludePvcNames, pvc.Name) {
			continue
		}
		pvName := pvc.Spec.VolumeName
		log.Debugf("Delete not used CSI Azure PersistentVolumeClaim %s in namespace %s", pvc.Name, namespace)
		err := deploy.deletePersistentVolumeClaim(namespace, pvc.Name)
		if err != nil {
			return err
		}
		log.Debugf("Delete not used CSI Azure PersistentVolume %s in namespace %s", pvName, namespace)
		err = deploy.deletePersistentVolume(pvName)
		if err != nil {
			return err
		}
	}
	return nil
}

func (deploy *Deployment) createCsiAzurePersistentVolumeClaim(storageClass *storagev1.StorageClass, requiredNewPvc bool, appName, namespace, componentName string, radixVolumeMount *radixv1.RadixVolumeMount, persistentVolumeClaimName string, pvcMap map[string]*corev1.PersistentVolumeClaim) (*corev1.PersistentVolumeClaim, error) {
	if pvc, ok := pvcMap[persistentVolumeClaimName]; ok {
		if pvc.Spec.StorageClassName == nil || len(*pvc.Spec.StorageClassName) == 0 {
			return pvc, nil
		}
		if !requiredNewPvc && strings.EqualFold(*pvc.Spec.StorageClassName, storageClass.Name) {
			return pvc, nil
		}

		log.Debugf("Delete PersistentVolumeClaim %s in namespace %s: changed StorageClass name to %s", pvc.Name, namespace, storageClass.Name)
		err := deploy.deletePersistentVolumeClaim(namespace, pvc.Name)
		if err != nil {
			return nil, err
		}
	}
	persistentVolumeClaimName, err := createCsiAzurePersistentVolumeClaimName(componentName, radixVolumeMount)
	if err != nil {
		return nil, err
	}
	log.Debugf("Create PersistentVolumeClaim %s in namespace %s for StorageClass %s", persistentVolumeClaimName, namespace, storageClass.Name)
	return deploy.createPersistentVolumeClaim(appName, namespace, componentName, persistentVolumeClaimName, storageClass.Name, radixVolumeMount)
}

// getOrCreateCsiAzureVolumeMountStorageClass returns creates or existing StorageClass, storageClassIsCreated=true, if created; error, if any
func (deploy *Deployment) getOrCreateCsiAzureVolumeMountStorageClass(appName, volumeRootMount, namespace, componentName string, radixVolumeMount *radixv1.RadixVolumeMount, volumeName string, scMap map[string]*storagev1.StorageClass) (*storagev1.StorageClass, bool, error) {
	var volumeMountProvisioner, foundProvisioner = radixv1.GetStorageClassProvisionerByVolumeMountType(radixVolumeMount)
	if !foundProvisioner {
		return nil, false, fmt.Errorf("not found Storage Class provisioner for volume mount type %s", string(GetCsiAzureVolumeMountType(radixVolumeMount)))
	}
	storageClassName := GetCsiAzureStorageClassName(namespace, volumeName)
	csiVolumeSecretName := defaults.GetCsiAzureVolumeMountCredsSecretName(componentName, radixVolumeMount.Name)
	if existingStorageClass, exists := scMap[storageClassName]; exists {
		desiredStorageClass := existingStorageClass.DeepCopy()
		err := populateCsiAzureStorageClass(desiredStorageClass, appName, volumeRootMount, namespace, componentName, storageClassName, radixVolumeMount, csiVolumeSecretName, volumeMountProvisioner)
		if err != nil {
			return nil, false, err
		}
		if equal, err := utils.EqualStorageClasses(existingStorageClass, desiredStorageClass); equal || err != nil {
			return existingStorageClass, false, err
		}

		log.Infof("Delete StorageClass %s in namespace %s", existingStorageClass.Name, namespace)
		err = deploy.deleteCsiAzureStorageClasses(existingStorageClass.Name)
		if err != nil {
			return nil, false, err
		}
	}

	log.Debugf("Create StorageClass %s in namespace %s", storageClassName, namespace)
	storageClass := &storagev1.StorageClass{}
	err := populateCsiAzureStorageClass(storageClass, appName, volumeRootMount, namespace, componentName, storageClassName, radixVolumeMount, csiVolumeSecretName, volumeMountProvisioner)
	if err != nil {
		return nil, false, err
	}
	desiredStorageClass, err := deploy.kubeclient.StorageV1().StorageClasses().Create(context.TODO(), storageClass, metav1.CreateOptions{})
	return desiredStorageClass, true, err
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
		if radixVolumeMount.BlobFuse2 == nil && radixVolumeMount.AzureFile == nil && !radixv1.IsKnownCsiAzureVolumeMount(string(GetCsiAzureVolumeMountType(&radixVolumeMount))) {
			continue
		}
		radixVolumeMount := radixVolumeMount
		volumeMountName, err := getCsiAzureVolumeMountName(componentName, &radixVolumeMount)
		if err != nil {
			return false
		}
		volumeMountMap[volumeMountName] = &radixVolumeMount
	}
	return true
}

func getVolumeAccessMode(modeValue string) corev1.PersistentVolumeAccessMode {
	switch strings.ToLower(modeValue) {
	case strings.ToLower(string(corev1.ReadWriteOnce)):
		return corev1.ReadWriteOnce
	case strings.ToLower(string(corev1.ReadOnlyMany)):
		return corev1.ReadOnlyMany
	}
	return corev1.ReadWriteMany // default access mode
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

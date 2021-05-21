package deployment

import (
	"context"
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	blobfuseDriver                            = "azure/blobfuse"
	csiAzureDriver                            = "azure/csi"
	defaultData                               = "xx"
	defaultMountOptions                       = "--file-cache-timeout-in-seconds=120"
	blobFuseVolumeNameTemplate                = "blobfuse-%s-%s"              // blobfuse-<componentname>-<radixvolumename>
	blobFuseVolumeNodeMountPathTemplate       = "/tmp/%s/%s/%s/%s/%s/%s"      // /tmp/<namespace>/<componentname>/<environment>/<volumetype>/<radixvolumename>/<container>
	csiAzureVolumeNameTemplate                = "csi-az-%s-%s-%s-%s"          // csi-az-<componentname>/<volumetype>/<radixvolumename>/<container>
	csiAzurePersistentVolumeClaimNameTemplate = "pvc-%s"                      // csi-az-<volumename>
	csiAzureStorageClassNameTemplate          = "sc-%s-%s"                    // csi-az-<namespace>-<volumename>
	csiAzureVolumeNodeMountPathTemplate       = "%s/%s/%s/csi-azure/%s/%s/%s" // <volumeRootMount>/<namespace>/<componentname>/csi-azure/<radixvolumetypeidforname>/<radixvolumename>/<container>
)

func GetRadixDeployComponentVolumeMounts(deployComponent radixv1.RadixCommonDeployComponent) []corev1.VolumeMount {
	componentName := deployComponent.GetName()
	radixVolumeMounts := deployComponent.GetVolumeMounts()
	volumeMounts := make([]corev1.VolumeMount, 0)

	if len(radixVolumeMounts) <= 0 {
		return volumeMounts
	}

	for _, radixVolumeMount := range radixVolumeMounts {
		switch radixVolumeMount.Type {
		case radixv1.MountTypeBlob:
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      getBlobFuseVolumeMountName(radixVolumeMount, componentName),
				MountPath: radixVolumeMount.Path,
			})
		case radixv1.MountTypeFileCsiAzure, radixv1.MountTypeBlobCsiAzure:
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      getCsiAzureVolumeMountName(componentName, radixVolumeMount),
				MountPath: radixVolumeMount.Path,
			})
		}
	}
	return volumeMounts
}

func getBlobFuseVolumeMountName(volumeMount radixv1.RadixVolumeMount, componentName string) string {
	return fmt.Sprintf(blobFuseVolumeNameTemplate, componentName, volumeMount.Name)
}

func getCsiAzureVolumeMountName(componentName string, radixVolumeMount radixv1.RadixVolumeMount) string {
	csiVolumeType := getRadixVolumeTypeIdForName(radixVolumeMount.Type)
	return fmt.Sprintf(csiAzureVolumeNameTemplate, componentName, csiVolumeType, radixVolumeMount.Name, radixVolumeMount.Storage)
}

func getRadixVolumeTypeIdForName(radixVolumeMountType radixv1.MountType) string {
	switch radixVolumeMountType {
	case radixv1.MountTypeBlobCsiAzure:
		return "bca"
	case radixv1.MountTypeFileCsiAzure:
		return "fca"
	}
	return "undef"
}

func (deploy *Deployment) getVolumes(deployComponent radixv1.RadixCommonDeployComponent) []corev1.Volume {
	return GetVolumes(deploy.getNamespace(), deploy.radixDeployment.Spec.Environment, deployComponent.GetName(), deployComponent.GetVolumeMounts())
}

func GetVolumes(namespace string, environment string, componentName string, volumeMounts []radixv1.RadixVolumeMount) []v1.Volume {
	volumes := make([]corev1.Volume, 0)
	for _, volumeMount := range volumeMounts {
		switch volumeMount.Type {
		case radixv1.MountTypeBlob:
			volumes = append(volumes, getBlobFuseVolume(namespace, environment, componentName, volumeMount))
		case radixv1.MountTypeBlobCsiAzure, radixv1.MountTypeFileCsiAzure:
			volumes = append(volumes, getCsiAzureVolume(componentName, volumeMount))
		}
	}
	return volumes
}

func getCsiAzureVolume(componentName string, radixVolumeMount radixv1.RadixVolumeMount) v1.Volume {
	volumeName := getCsiAzureVolumeMountName(componentName, radixVolumeMount)
	volume := corev1.Volume{
		Name: getCsiAzureVolumeMountName(componentName, radixVolumeMount),
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: getCsiAzurePersistentVolumeClaimName(volumeName),
			},
		},
	}
	return volume
}

func getCsiAzurePersistentVolumeClaimName(volumeName string) string {
	return fmt.Sprintf(csiAzurePersistentVolumeClaimNameTemplate, volumeName) //volumeName: <component-name>-<csi-volume-type-dashed>-<radix-volume-name>-<container-name>
}

//GetCsiAzureStorageClassName hold a name of CSI volume storage class
func GetCsiAzureStorageClassName(namespace, volumeName string) string {
	return fmt.Sprintf(csiAzureStorageClassNameTemplate, namespace, volumeName) //volumeName: <component-name>-<csi-volume-type-dashed>-<radix-volume-name>-<container-name>
}

func getBlobFuseVolume(namespace, environment, componentName string, volumeMount radixv1.RadixVolumeMount) v1.Volume {
	secretName := defaults.GetBlobFuseCredsSecretName(componentName, volumeMount.Name)

	flexVolumeOptions := make(map[string]string)
	flexVolumeOptions["name"] = volumeMount.Name
	flexVolumeOptions["container"] = volumeMount.Container
	flexVolumeOptions["mountoptions"] = defaultMountOptions
	flexVolumeOptions["tmppath"] = fmt.Sprintf(blobFuseVolumeNodeMountPathTemplate, namespace, componentName, environment, radixv1.MountTypeBlob, volumeMount.Name, volumeMount.Container)

	return v1.Volume{
		Name: getBlobFuseVolumeMountName(volumeMount, componentName),
		VolumeSource: v1.VolumeSource{
			FlexVolume: &v1.FlexVolumeSource{
				Driver:  blobfuseDriver,
				Options: flexVolumeOptions,
				SecretRef: &v1.LocalObjectReference{
					Name: secretName,
				},
			},
		},
	}
}

func (deploy *Deployment) createOrUpdateVolumeMountsSecrets(namespace, componentName, secretName string, accountName, accountKey []byte) error {
	blobfusecredsSecret := v1.Secret{
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
func (deploy *Deployment) createOrUpdateCsiAzureVolumeMountsSecrets(namespace, componentName, volumeMountName string, volumeMountType radixv1.MountType, secretName string, accountName, accountKey []byte) error {
	secret := v1.Secret{
		Type: csiAzureDriver,
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: map[string]string{
				kube.RadixAppLabel:             deploy.registration.Name,
				kube.RadixComponentLabel:       componentName,
				kube.RadixMountTypeLabel:       string(volumeMountType),
				kube.RadixVolumeMountNameLabel: volumeMountName,
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

func (deploy *Deployment) garbageCollectVolumeMountsSecretsNoLongerInSpecForComponent(component radixv1.RadixCommonDeployComponent) error {
	secrets, err := deploy.listSecretsForVolumeMounts(component)
	if err != nil {
		return err
	}

	for _, secret := range secrets {
		err = deploy.kubeclient.CoreV1().Secrets(deploy.radixDeployment.GetNamespace()).Delete(context.TODO(), secret.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func (deploy *Deployment) GetCsiAzureStorageClasses(namespace, componentName string) (*storagev1.StorageClassList, error) {
	return deploy.kubeclient.StorageV1().StorageClasses().List(context.TODO(), metav1.ListOptions{
		LabelSelector: getLabelSelectorForCsiAzureStorageClass(namespace, componentName),
	})
}

func (deploy *Deployment) GetCsiAzurePersistentVolumeClaims(namespace, componentName string) (*v1.PersistentVolumeClaimList, error) {
	return deploy.kubeclient.CoreV1().PersistentVolumeClaims(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: getLabelSelectorForCsiAzurePersistenceVolumeClaim(componentName),
	})
}

func getLabelSelectorForCsiAzureStorageClass(namespace, componentName string) string {
	return fmt.Sprintf("%s=%s, %s=%s, %s in (%s, %s)", kube.RadixNamespace, namespace, kube.RadixComponentLabel, componentName, kube.RadixMountTypeLabel, string(radixv1.MountTypeBlobCsiAzure), string(radixv1.MountTypeFileCsiAzure))
}

func getLabelSelectorForCsiAzurePersistenceVolumeClaim(componentName string) string {
	return fmt.Sprintf("%s=%s, %s in (%s, %s)", kube.RadixComponentLabel, componentName, kube.RadixMountTypeLabel, string(radixv1.MountTypeBlobCsiAzure), string(radixv1.MountTypeFileCsiAzure))
}

func (deploy *Deployment) CreatePersistentVolumeClaim(namespace, componentName, pvcName, storageClassName string, radixVolumeMountType radixv1.MountType, radixVolumeName string) (*v1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: namespace,
			Labels: map[string]string{
				kube.RadixComponentLabel:       componentName,
				kube.RadixMountTypeLabel:       string(radixVolumeMountType),
				kube.RadixVolumeMountNameLabel: radixVolumeName,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, //TODO - specify in configuration
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Mi")}, //it seems correct number is not needed for CSI driver
			},
			StorageClassName: &storageClassName,
		},
	}
	createdPvc, err := deploy.kubeclient.CoreV1().PersistentVolumeClaims(namespace).Create(context.TODO(), pvc, metav1.CreateOptions{})
	return createdPvc, err
}

func (deploy *Deployment) CreateCsiAzureStorageClasses(volumeRootMount, namespace, componentName, storageClassName string, radixVolumeMountType radixv1.MountType, storageName, radixVolumeName, secretName string) (*storagev1.StorageClass, error) {
	volumeMountProvisioner, foundProvisioner := radixv1.GetStorageClassProvisionerByVolumeMountType(radixVolumeMountType)
	if !foundProvisioner {
		return nil, fmt.Errorf("not found Storage Class for volume mount type %s", string(radixVolumeMountType))
	}
	reclaimPolicy := corev1.PersistentVolumeReclaimRetain
	bindingMode := storagev1.VolumeBindingImmediate
	csiVolumeType := getRadixVolumeTypeIdForName(radixVolumeMountType)
	tmpPath := fmt.Sprintf(csiAzureVolumeNodeMountPathTemplate, volumeRootMount, namespace, componentName, csiVolumeType, radixVolumeName, storageName)
	storageClass := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: storageClassName,
			Labels: map[string]string{
				kube.RadixNamespace:            namespace,
				kube.RadixComponentLabel:       componentName,
				kube.RadixMountTypeLabel:       string(radixVolumeMountType),
				kube.RadixVolumeMountNameLabel: radixVolumeName,
			},
		},
		Provisioner: volumeMountProvisioner,
		Parameters: map[string]string{
			"csi.storage.k8s.io/provisioner-secret-name":      secretName,
			"csi.storage.k8s.io/provisioner-secret-namespace": namespace,
			"csi.storage.k8s.io/node-stage-secret-name":       secretName,
			"csi.storage.k8s.io/node-stage-secret-namespace":  namespace,
			"skuName": "Standard_LRS", //available values: Standard_LRS, Premium_LRS, Standard_GRS, Standard_RAGRS
		},
		ReclaimPolicy: &reclaimPolicy,
		MountOptions: []string{
			fmt.Sprintf("--tmp-path=%s", tmpPath),
			"--file-cache-timeout-in-seconds=120",
			"--use-attr-cache=true",
			"-o allow_other",
			"-o attr_timeout=120",
			"-o entry_timeout=120",
			"-o negative_timeout=120",
		},
		VolumeBindingMode: &bindingMode,
	}
	switch radixVolumeMountType {
	case radixv1.MountTypeBlobCsiAzure:
		storageClass.Parameters["containerName"] = storageName
	case radixv1.MountTypeFileCsiAzure:
		storageClass.Parameters["shareName"] = storageName
	}
	return deploy.kubeclient.StorageV1().StorageClasses().Create(context.TODO(), storageClass, metav1.CreateOptions{})
}

func (deploy *Deployment) DeletePersistentVolumeClaim(namespace, pvcName string) error {
	return deploy.kubeclient.CoreV1().PersistentVolumeClaims(namespace).Delete(context.TODO(), pvcName, metav1.DeleteOptions{})
}

func (deploy *Deployment) DeleteCsiAzureStorageClasses(storageClassName string) error {
	return deploy.kubeclient.StorageV1().StorageClasses().Delete(context.TODO(), storageClassName, metav1.DeleteOptions{})
}

//GetRadixVolumeMountStorage get RadixVolumeMount storage property, depend on volume type
func GetRadixVolumeMountStorage(radixVolumeMount *radixv1.RadixVolumeMount) string {
	if radixVolumeMount.Type == radixv1.MountTypeBlob {
		return radixVolumeMount.Container //Outdated
	}
	return radixVolumeMount.Storage
}

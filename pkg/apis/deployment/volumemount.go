package deployment

import (
	"context"
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	blobfuseDriver                       = "azure/blobfuse"
	csiAzureDriver                       = "azure/csi"
	defaultData                          = "xx"
	defaultMountOptions                  = "--file-cache-timeout-in-seconds=120"
	blobFuseVolumeNameTemplate           = "blobfuse-%s-%s"         // blobfuse-<componentname>-<radixvolumename>
	csiVolumeNameTemplate                = "%s-%s-%s"               // <csidriverdashed>-<componentname>-<radixvolumename>
	csiPersistentVolumeClaimNameTemplate = "csi-pvc-%s"             // csi-pvc-<volumename>
	csiStorageClassNameTemplate          = "csi-sc-%s-%s"           // csi-sc-<namespace>-<volumename>
	blobFuseVolumeNodeMountPathTemplate  = "/tmp/%s/%s/%s/%s/%s/%s" // /tmp/<namespace>/<componentname>/<environment>/<volumetype>/<volumename>/<container>
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
		case radixv1.MountTypeDiskCsiAzure, radixv1.MountTypeBlobCsiAzure:
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      getCsiVolumeMountName(componentName, radixVolumeMount),
				MountPath: radixVolumeMount.Path,
			})
		}
	}

	return volumeMounts
}

func getBlobFuseVolumeMountName(volumeMount radixv1.RadixVolumeMount, componentName string) string {
	return fmt.Sprintf(blobFuseVolumeNameTemplate, componentName, volumeMount.Name)
}

func getCsiVolumeMountName(componentName string, radixVolumeMount radixv1.RadixVolumeMount) string {
	csiDriverName := strings.Replace(string(radixVolumeMount.Type), ".", "-", -1)
	sprintf := fmt.Sprintf(csiVolumeNameTemplate, csiDriverName, componentName, radixVolumeMount.Name)
	return sprintf
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
		case radixv1.MountTypeBlobCsiAzure, radixv1.MountTypeDiskCsiAzure:
			volumes = append(volumes, getCsiVolume(componentName, volumeMount))
		}
	}
	return volumes
}

func getCsiVolume(componentName string, radixVolumeMount radixv1.RadixVolumeMount) v1.Volume {
	volumeName := getCsiVolumeMountName(componentName, radixVolumeMount)
	volume := corev1.Volume{
		Name: getCsiVolumeMountName(componentName, radixVolumeMount),
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: GetCsiPersistentVolumeClaimName(volumeName),
			},
		},
	}
	return volume
}

//GetCsiPersistentVolumeClaimName hold a name of CSI volume persistent volume claim
func GetCsiPersistentVolumeClaimName(volumeName string) string {
	return fmt.Sprintf(csiPersistentVolumeClaimNameTemplate, volumeName) //volumeName: <csi-driver-dashed>-<component-name>-<radix-volume-name>
}

//GetCsiStorageClassName hold a name of CSI volume storage class
func GetCsiStorageClassName(namespace, volumeName string) string {
	return fmt.Sprintf(csiStorageClassNameTemplate, namespace, volumeName) //volumeName: <csi-driver-dashed>-<component-name>-<radix-volume-name>
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

func GetCsiAzureResourceName(namespace, componentName string) string {
	return fmt.Sprintf("csi-azure-%s-%s-%s", namespace, componentName, utils.RandString(5))
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
func (deploy *Deployment) createOrUpdateCsiAzureVolumeMountsSecrets(namespace, componentName, volumeMountName, secretName string, accountName, accountKey []byte) error {
	secret := v1.Secret{
		Type: csiAzureDriver,
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: map[string]string{
				kube.RadixAppLabel:             deploy.registration.Name,
				kube.RadixComponentLabel:       componentName,
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
	secrets, err := deploy.listSecretsForForBlobVolumeMount(component)
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

func (deploy *Deployment) GetStorageClasses(namespace, componentName string) (*storagev1.StorageClassList, error) {
	return deploy.kubeclient.StorageV1().StorageClasses().List(context.TODO(), metav1.ListOptions{
		LabelSelector: getLabelSelectorForCsiVolumeStorageClass(namespace, componentName),
	})
}

func (deploy *Deployment) GetPersistentVolumeClaims(namespace, componentName string) (*v1.PersistentVolumeClaimList, error) {
	return deploy.kubeclient.CoreV1().PersistentVolumeClaims(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: getLabelSelectorForCsiPersistenceVolumeClaim(componentName),
	})
}

func getLabelSelectorForCsiVolumeStorageClass(namespace, componentName string) string {
	return fmt.Sprintf("%s=%s, %s=%s", kube.RadixNamespace, namespace, kube.RadixComponentLabel, componentName)
}

func getLabelSelectorForCsiPersistenceVolumeClaim(componentName string) string {
	return fmt.Sprintf("%s=%s", kube.RadixComponentLabel, componentName)
}

func (deploy *Deployment) CreatePersistentVolumeClaim(namespace, componentName, pvcName, storageClassName, volumeName string) (*v1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: namespace,
			Labels: map[string]string{
				kube.RadixComponentLabel:       componentName,
				kube.RadixVolumeMountNameLabel: volumeName,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, //TODO - specify in configuration
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("50Mi")}, //TODO - check if it is needed
			},
			StorageClassName: &storageClassName,
		},
	}
	createdPvc, err := deploy.kubeclient.CoreV1().PersistentVolumeClaims(namespace).Create(context.TODO(), pvc, metav1.CreateOptions{})
	return createdPvc, err
}

func (deploy *Deployment) DeletePersistentVolumeClaim(namespace, pvcName string) error {
	return deploy.kubeclient.CoreV1().PersistentVolumeClaims(namespace).Delete(context.TODO(), pvcName, metav1.DeleteOptions{})
}

func (deploy *Deployment) DeleteCsiAzureStorageClasses(storageClassName string) error {
	return deploy.kubeclient.StorageV1().StorageClasses().Delete(context.TODO(), storageClassName, metav1.DeleteOptions{})
}

func (deploy *Deployment) CreateCsiAzureStorageClasses(namespace, componentName, storageClassName, containerName, secretName, volumeName string) (*storagev1.StorageClass, error) {
	reclaimPolicy := corev1.PersistentVolumeReclaimRetain
	bindingMode := storagev1.VolumeBindingImmediate
	storageClass := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: storageClassName,
			Labels: map[string]string{
				kube.RadixNamespace:            namespace,
				kube.RadixComponentLabel:       componentName,
				kube.RadixMountTypeLabel:       string(radixv1.MountTypeBlobCsiAzure),
				kube.RadixVolumeMountNameLabel: volumeName,
			},
		},
		Provisioner: "blob.csi.azure.com", //TODO - get from radix-config
		Parameters: map[string]string{
			"containerName": containerName,
			"csi.storage.k8s.io/provisioner-secret-name":      secretName,
			"csi.storage.k8s.io/provisioner-secret-namespace": namespace,
			"skuName": "Standard_LRS", //available values: Standard_LRS, Premium_LRS, Standard_GRS, Standard_RAGRS
		},
		ReclaimPolicy: &reclaimPolicy,
		MountOptions: []string{
			"-o allow_other",
			"--file-cache-timeout-in-seconds=120",
			"--use-attr-cache=true",
			"-o attr_timeout=120",
			"-o entry_timeout=120",
			"-o negative_timeout=120",
		},
		VolumeBindingMode: &bindingMode,
	}
	createdStorageClass, err := deploy.kubeclient.StorageV1().StorageClasses().Create(context.TODO(), storageClass, metav1.CreateOptions{})
	return createdStorageClass, err
}

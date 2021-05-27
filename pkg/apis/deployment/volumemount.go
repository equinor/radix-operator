package deployment

import (
	"context"
	"errors"
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

const (
	persistentVolumeClaimKind = "PersistentVolumeClaim"

	blobfuseDriver      = "azure/blobfuse"
	defaultData         = "xx"
	defaultMountOptions = "--file-cache-timeout-in-seconds=120"

	blobFuseVolumeNameTemplate          = "blobfuse-%s-%s"         // blobfuse-<componentname>-<radixvolumename>
	blobFuseVolumeNodeMountPathTemplate = "/tmp/%s/%s/%s/%s/%s/%s" // /tmp/<namespace>/<componentname>/<environment>/<volumetype>/<radixvolumename>/<container>

	csiVolumeNameTemplate                = "%s-%s-%s-%s"       // <radixvolumeid>-<componentname>-<radixvolumename>-<storage>
	csiPersistentVolumeClaimNameTemplate = "pvc-%s"            // pvc-<volumename>
	csiStorageClassNameTemplate          = "sc-%s-%s"          // sc-<namespace>-<volumename>
	csiVolumeNodeMountPathTemplate       = "%s/%s/%s/%s/%s/%s" // <volumeRootMount>/<namespace>/<radixvolumeid>/<componentname>/<radixvolumename>/<storage>

	csiStorageClassProvisionerSecretNameParameter      = "csi.storage.k8s.io/provisioner-secret-name"      //Secret name, containing storage account name and key
	csiStorageClassProvisionerSecretNamespaceParameter = "csi.storage.k8s.io/provisioner-secret-namespace" //Namespace of the secret
	csiStorageClassNodeStageSecretNameParameter        = "csi.storage.k8s.io/node-stage-secret-name"       //Usually equal to csiStorageClassProvisionerSecretNameParameter
	csiStorageClassNodeStageSecretNamespaceParameter   = "csi.storage.k8s.io/node-stage-secret-namespace"  //Usually equal to csiStorageClassProvisionerSecretNamespaceParameter
	csiAzureStorageClassSkuNameParameter               = "skuName"                                         //Available values: Standard_LRS (default), Premium_LRS, Standard_GRS, Standard_RAGRS. https://docs.microsoft.com/en-us/rest/api/storagerp/srp_sku_types
	csiStorageClassContainerNameParameter              = "containerName"                                   //Container name - foc container storages
	csiStorageClassShareNameParameter                  = "shareName"                                       //File Share name - for file storages
	csiStorageClassTmpPathMountOption                  = "tmp-path"                                        //Path within the node, where the volume mount has been mounted to
	csiStorageClassGidMountOption                      = "gid"                                             //Volume mount owner GroupID. Used when drivers do not honor fsGroup securityContext setting
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
	return fmt.Sprintf(csiVolumeNameTemplate, csiVolumeType, componentName, radixVolumeMount.Name, radixVolumeMount.Storage)
}

func getRadixVolumeTypeIdForName(radixVolumeMountType radixv1.MountType) string {
	switch radixVolumeMountType {
	case radixv1.MountTypeBlobCsiAzure:
		return "csi-az-blob"
	case radixv1.MountTypeFileCsiAzure:
		return "csi-az-file"
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
	return fmt.Sprintf(csiPersistentVolumeClaimNameTemplate, volumeName) //volumeName: <component-name>-<csi-volume-type-dashed>-<radix-volume-name>-<storage-name>
}

//GetCsiAzureStorageClassName hold a name of CSI volume storage class
func GetCsiAzureStorageClassName(namespace, volumeName string) string {
	return fmt.Sprintf(csiStorageClassNameTemplate, namespace, volumeName) //volumeName: <component-name>-<csi-volume-type-dashed>-<radix-volume-name>-<storage-name>
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
		Type: v1.SecretTypeOpaque,
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

func (deploy *Deployment) garbageCollectSecrets(secrets []*v1.Secret, excludeSecretNames []string) error {
	for _, secret := range secrets {
		if slice.ContainsString(excludeSecretNames, secret.Name) {
			continue
		}
		err := deploy.kubeclient.CoreV1().Secrets(deploy.radixDeployment.GetNamespace()).Delete(context.TODO(), secret.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (deploy *Deployment) garbageCollectVolumeMountsSecretsNoLongerInSpecForComponent(component radixv1.RadixCommonDeployComponent, excludeSecretNames []string) error {
	secrets, err := deploy.listSecretsForVolumeMounts(component)
	if err != nil {
		return err
	}
	return deploy.garbageCollectSecrets(secrets, excludeSecretNames)
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

func (deploy *Deployment) GetCsiAzureStorageClassesSecrets(namespace, componentName string) (*storagev1.StorageClassList, error) {
	return deploy.kubeclient.StorageV1().StorageClasses().List(context.TODO(), metav1.ListOptions{
		LabelSelector: getLabelSelectorForCsiAzureStorageClass(namespace, componentName),
	})
}

func (deploy *Deployment) GetPersistentVolumesForPvc() (*v1.PersistentVolumeList, error) {
	return deploy.kubeclient.CoreV1().PersistentVolumes().List(context.TODO(), metav1.ListOptions{})
}

func getLabelSelectorForCsiAzureStorageClass(namespace, componentName string) string {
	return fmt.Sprintf("%s=%s, %s=%s, %s in (%s, %s)", kube.RadixNamespace, namespace, kube.RadixComponentLabel, componentName, kube.RadixMountTypeLabel, string(radixv1.MountTypeBlobCsiAzure), string(radixv1.MountTypeFileCsiAzure))
}

func getLabelSelectorForCsiAzurePersistenceVolumeClaim(componentName string) string {
	return fmt.Sprintf("%s=%s, %s in (%s, %s)", kube.RadixComponentLabel, componentName, kube.RadixMountTypeLabel, string(radixv1.MountTypeBlobCsiAzure), string(radixv1.MountTypeFileCsiAzure))
}

func (deploy *Deployment) CreatePersistentVolumeClaim(namespace, componentName, pvcName, storageClassName string, radixVolumeMount *radixv1.RadixVolumeMount) (*v1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: namespace,
			Labels: map[string]string{
				kube.RadixComponentLabel:       componentName,
				kube.RadixMountTypeLabel:       string(radixVolumeMount.Type),
				kube.RadixVolumeMountNameLabel: radixVolumeMount.Name,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}, //TODO - specify in configuration
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Mi")}, //it seems correct number is not needed for CSI driver
			},
			StorageClassName: &storageClassName,
		},
	}
	return deploy.kubeclient.CoreV1().PersistentVolumeClaims(namespace).Create(context.TODO(), pvc, metav1.CreateOptions{})
}

func (deploy *Deployment) CreateCsiAzureStorageClasses(volumeRootMount, namespace, componentName, storageClassName string, radixVolumeMount *radixv1.RadixVolumeMount, secretName string) (*storagev1.StorageClass, error) {
	volumeMountProvisioner, foundProvisioner := radixv1.GetStorageClassProvisionerByVolumeMountType(radixVolumeMount.Type)
	if !foundProvisioner {
		return nil, fmt.Errorf("not found Storage Class provisioner for volume mount type %s", string(radixVolumeMount.Type))
	}
	reclaimPolicy := corev1.PersistentVolumeReclaimRetain
	bindingMode := storagev1.VolumeBindingImmediate
	storageClass := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:   storageClassName,
			Labels: getCsiAzureStorageClassLabels(namespace, componentName, radixVolumeMount),
		},
		Provisioner:       volumeMountProvisioner,
		Parameters:        getCsiAzureStorageClassParameters(secretName, namespace, radixVolumeMount),
		MountOptions:      getCsiAzureStorageClassMountOptions(volumeRootMount, namespace, componentName, radixVolumeMount),
		ReclaimPolicy:     &reclaimPolicy,
		VolumeBindingMode: &bindingMode,
	}
	return deploy.kubeclient.StorageV1().StorageClasses().Create(context.TODO(), storageClass, metav1.CreateOptions{})
}

func getCsiAzureStorageClassLabels(namespace string, componentName string, radixVolumeMount *radixv1.RadixVolumeMount) map[string]string {
	return map[string]string{
		kube.RadixNamespace:            namespace,
		kube.RadixComponentLabel:       componentName,
		kube.RadixMountTypeLabel:       string(radixVolumeMount.Type),
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
	if len(radixVolumeMount.SkuName) > 0 {
		parameters[csiAzureStorageClassSkuNameParameter] = radixVolumeMount.SkuName
	}
	switch radixVolumeMount.Type {
	case radixv1.MountTypeBlobCsiAzure:
		parameters[csiStorageClassContainerNameParameter] = radixVolumeMount.Storage
	case radixv1.MountTypeFileCsiAzure:
		parameters[csiStorageClassShareNameParameter] = radixVolumeMount.Storage
	}
	return parameters
}

func getCsiAzureStorageClassMountOptions(volumeRootMount, namespace, componentName string, radixVolumeMount *radixv1.RadixVolumeMount) []string {
	csiVolumeTypeId := getRadixVolumeTypeIdForName(radixVolumeMount.Type)
	tmpPath := fmt.Sprintf(csiVolumeNodeMountPathTemplate, volumeRootMount, namespace, csiVolumeTypeId, componentName, radixVolumeMount.Name, radixVolumeMount.Storage)
	mountOptions := []string{
		fmt.Sprintf("--%s=%s", csiStorageClassTmpPathMountOption, tmpPath),
		"--file-cache-timeout-in-seconds=120",
		"--use-attr-cache=true",
		"-o allow_other",
		"-o attr_timeout=120",
		"-o entry_timeout=120",
		"-o negative_timeout=120",
	}
	if len(radixVolumeMount.GID) > 0 {
		mountOptions = append(mountOptions, fmt.Sprintf("-o %s=%s", csiStorageClassGidMountOption, radixVolumeMount.GID))
	}
	return mountOptions
}

func (deploy *Deployment) DeletePersistentVolumeClaim(namespace, pvcName string) error {
	return deploy.kubeclient.CoreV1().PersistentVolumeClaims(namespace).Delete(context.TODO(), pvcName, metav1.DeleteOptions{})
}

func (deploy *Deployment) DeleteCsiAzureStorageClasses(storageClassName string) error {
	return deploy.kubeclient.StorageV1().StorageClasses().Delete(context.TODO(), storageClassName, metav1.DeleteOptions{})
}

func (deploy *Deployment) DeletePersistentVolume(pvName string) error {
	return deploy.kubeclient.CoreV1().PersistentVolumes().Delete(context.TODO(), pvName, metav1.DeleteOptions{})
}

//GetRadixVolumeMountStorage get RadixVolumeMount storage property, depend on volume type
func GetRadixVolumeMountStorage(radixVolumeMount *radixv1.RadixVolumeMount) string {
	if radixVolumeMount.Type == radixv1.MountTypeBlob {
		return radixVolumeMount.Container //Outdated
	}
	return radixVolumeMount.Storage
}

func (deploy *Deployment) garbageCollectOrphanedCsiAzurePersistentVolumes(excludePvcNames []string) error {
	pvList, err := deploy.GetPersistentVolumesForPvc()
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
		deploy.DeletePersistentVolume(pv.Name)
	}
	return nil
}

//CreateOrUpdateCsiAzureResources Create or update CSI Azure volume resources - StorageClasses, PersistentVolumeClaims, PersistentVolume
func (deploy *Deployment) CreateOrUpdateCsiAzureResources(desiredDeployment *appsv1.Deployment) error {
	namespace := deploy.radixDeployment.GetNamespace()
	componentName := desiredDeployment.ObjectMeta.Name
	volumeRootMount := "/tmp" //TODO: add to environment variable, so this volume can be mounted to external disk
	scList, err := deploy.GetCsiAzureStorageClasses(namespace, componentName)
	if err != nil {
		return err
	}
	pvcList, err := deploy.GetCsiAzurePersistentVolumeClaims(namespace, componentName)
	if err != nil {
		return err
	}

	scMap := getStorageClassMapByName(scList)
	pvcMap := getPersistentVolumeClaimMapByName(pvcList)
	radixVolumeMountMap := deploy.getRadixVolumeMountMapByCsiAzureVolumeMountName(componentName)
	var actualStorageClassNames, actualPvcNames []string
	for _, volume := range desiredDeployment.Spec.Template.Spec.Volumes {
		if volume.PersistentVolumeClaim == nil {
			continue
		}
		radixVolumeMount, existsRadixVolumeMount := radixVolumeMountMap[volume.Name]
		if !existsRadixVolumeMount {
			return errors.New(fmt.Sprintf("not found Radix volume mount for desired volume %s", volume.Name))
		}
		storageClass, err := deploy.createCsiAzureStorageClass(volumeRootMount, namespace, componentName, radixVolumeMount, &volume, scMap)
		if err != nil {
			return err
		}
		actualStorageClassNames = append(actualStorageClassNames, storageClass.Name)
		pvc, err := deploy.createCsiAzurePersistentVolumeClaim(storageClass, namespace, componentName, radixVolumeMount, &volume, pvcMap)
		if err != nil {
			return err
		}
		actualPvcNames = append(actualPvcNames, pvc.Name)
	}
	deploy.garbageCollectCsiAzureStorageClasses(scList, actualStorageClassNames)
	deploy.garbageCollectCsiAzurePersistentVolumeClaimsAndPersistentVolumes(namespace, pvcList, actualPvcNames)
	err = deploy.garbageCollectOrphanedCsiAzurePersistentVolumes(actualPvcNames)
	return err
}

func (deploy *Deployment) garbageCollectCsiAzureStorageClasses(scList *storagev1.StorageClassList, excludeSecretName []string) {
	for _, sc := range scList.Items {
		if !slice.ContainsString(excludeSecretName, sc.Name) {
			deploy.DeleteCsiAzureStorageClasses(sc.Name)
		}
	}
}

func (deploy *Deployment) garbageCollectCsiAzurePersistentVolumeClaimsAndPersistentVolumes(namespace string, pvcList *corev1.PersistentVolumeClaimList, excludePvcNames []string) {
	for _, pvc := range pvcList.Items {
		if !slice.ContainsString(excludePvcNames, pvc.Name) {
			pvName := pvc.Spec.VolumeName
			deploy.DeletePersistentVolumeClaim(namespace, pvc.Name)
			deploy.DeletePersistentVolume(pvName)
		}
	}
}

func (deploy *Deployment) createCsiAzurePersistentVolumeClaim(storageClass *storagev1.StorageClass, namespace string, componentName string, radixVolumeMount *radixv1.RadixVolumeMount, volume *corev1.Volume, pvcMap map[string]*corev1.PersistentVolumeClaim) (*corev1.PersistentVolumeClaim, error) {
	if pvc, ok := pvcMap[volume.PersistentVolumeClaim.ClaimName]; ok {
		if pvc.Spec.StorageClassName == nil || len(*pvc.Spec.StorageClassName) == 0 || strings.EqualFold(*pvc.Spec.StorageClassName, storageClass.Name) {
			return pvc, nil
		}
		deploy.DeletePersistentVolumeClaim(namespace, pvc.Name)
	}
	return deploy.CreatePersistentVolumeClaim(namespace, componentName, volume.PersistentVolumeClaim.ClaimName, storageClass.Name, radixVolumeMount)
}

func (deploy *Deployment) createCsiAzureStorageClass(volumeRootMount string, namespace string, componentName string, radixVolumeMount *radixv1.RadixVolumeMount, volume *corev1.Volume, scMap map[string]*storagev1.StorageClass) (*storagev1.StorageClass, error) {
	csiVolumeStorageClassName := GetCsiAzureStorageClassName(namespace, volume.Name)
	if storageClass, ok := scMap[csiVolumeStorageClassName]; ok {
		if err := validateCsiAzureStorageClass(namespace, storageClass, radixVolumeMount); err == nil {
			return storageClass, nil
		}
		deploy.DeleteCsiAzureStorageClasses(storageClass.Name)
	}
	csiVolumeSecretName := defaults.GetCsiAzureCredsSecretName(componentName, radixVolumeMount.Name)
	storageClass, err := deploy.CreateCsiAzureStorageClasses(volumeRootMount, namespace, componentName, csiVolumeStorageClassName, radixVolumeMount, csiVolumeSecretName)
	return storageClass, err
}

func validateCsiAzureStorageClass(namespace string, storageClass *storagev1.StorageClass, radixVolumeMount *radixv1.RadixVolumeMount) error {
	radixVolumeExpectedProvisioner, foundProvisioner := radixv1.GetStorageClassProvisionerByVolumeMountType(radixVolumeMount.Type)
	if !foundProvisioner {
		return fmt.Errorf("not found Storage Class for volume mount type %s", string(radixVolumeMount.Type))
	}
	if !strings.EqualFold(storageClass.Provisioner, radixVolumeExpectedProvisioner) {
		return fmt.Errorf("component volume type %s does not match to storage class provisioner %s", radixVolumeExpectedProvisioner, storageClass.Provisioner)
	}
	switch radixVolumeMount.Type {
	case radixv1.MountTypeBlobCsiAzure:
		if containerName, ok := storageClass.Parameters["containerName"]; !ok || strings.EqualFold(containerName, radixVolumeMount.Storage) {
			return fmt.Errorf("component storage name %s does not match to storage class containerName parameter %s", radixVolumeMount.Storage, containerName)
		}
	case radixv1.MountTypeFileCsiAzure:
		if shareName, ok := storageClass.Parameters["shareName"]; !ok || strings.EqualFold(shareName, radixVolumeMount.Storage) {
			return fmt.Errorf("component storage name %s does not match to storage class shareName parameter %s", radixVolumeMount.Storage, shareName)
		}
	}
	//StorageClass is cluster-wide. This is a safety check to prevent using StorageClass made for one namespace for a component in another namespace
	//StorageClass name template: sc-<namespace>-<volumename>
	//Namespace name template: <appName>-<env>
	//Volume name template: <radixvolumeid>-<componentname>-<radixvolumename>-<storage>
	//Example (application|environment|componentName|radixVolumeName|storageName):
	// following configurations can be interfered by storage classes name: "a-b|c|d|e|f", "a|b-c|d|e|f", "a|b|c-d|e|f", "a|b|c|d-e|f", "a|b|c|d|e-f"
	if scNamespace, namespaceLabelFound := storageClass.Labels[kube.RadixNamespace]; !namespaceLabelFound || !strings.EqualFold(scNamespace, namespace) {
		if !namespaceLabelFound {
			return fmt.Errorf("storage class namespace label not found")
		} else {
			return fmt.Errorf("component namespace %s does not match to storage class namespace %s", scNamespace, namespace)
		}
	}
	return nil
}

func (deploy *Deployment) getRadixVolumeMountMapByCsiAzureVolumeMountName(componentName string) map[string]*radixv1.RadixVolumeMount {
	volumeMountMap := make(map[string]*radixv1.RadixVolumeMount)
	for _, component := range deploy.radixDeployment.Spec.Components {
		if !strings.EqualFold(componentName, component.GetName()) {
			continue
		}
		for _, radixVolumeMount := range component.VolumeMounts {
			mount := radixVolumeMount
			volumeMountMap[getCsiAzureVolumeMountName(componentName, radixVolumeMount)] = &mount
		}
		break
	}
	return volumeMountMap
}

func getPersistentVolumeClaimMapByName(pvcList *corev1.PersistentVolumeClaimList) map[string]*corev1.PersistentVolumeClaim {
	pvcMap := make(map[string]*corev1.PersistentVolumeClaim)
	for _, pvc := range pvcList.Items {
		pvcMap[pvc.Name] = &pvc
	}
	return pvcMap
}

func getStorageClassMapByName(scList *storagev1.StorageClassList) map[string]*storagev1.StorageClass {
	scMap := make(map[string]*storagev1.StorageClass)
	for _, sc := range scList.Items {
		scMap[sc.Name] = &sc
	}
	return scMap
}

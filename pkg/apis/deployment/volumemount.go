package deployment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	"sort"
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
	csiPersistentVolumeClaimNameTemplate = "pvc-%s-%s"         // pvc-<volumename>-<randomstring5>
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
	csiStorageClassUidMountOption                      = "uid"                                             //Volume mount owner UserID. Used instead of GroupID
)

func GetRadixDeployComponentVolumeMounts(deployComponent radixv1.RadixCommonDeployComponent) ([]v1.VolumeMount, error) {
	componentName := deployComponent.GetName()
	radixVolumeMounts := deployComponent.GetVolumeMounts()
	volumeMounts := make([]corev1.VolumeMount, 0)

	if len(radixVolumeMounts) <= 0 {
		return volumeMounts, nil
	}

	for _, radixVolumeMount := range radixVolumeMounts {
		switch radixVolumeMount.Type {
		case radixv1.MountTypeBlob:
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      getBlobFuseVolumeMountName(radixVolumeMount, componentName),
				MountPath: radixVolumeMount.Path,
			})
		case radixv1.MountTypeFileCsiAzure, radixv1.MountTypeBlobCsiAzure:
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

func getBlobFuseVolumeMountName(volumeMount radixv1.RadixVolumeMount, componentName string) string {
	return fmt.Sprintf(blobFuseVolumeNameTemplate, componentName, volumeMount.Name)
}

func getCsiAzureVolumeMountName(componentName string, radixVolumeMount *radixv1.RadixVolumeMount) (string, error) {
	csiVolumeType := getRadixVolumeTypeIdForName(radixVolumeMount.Type)
	if len(radixVolumeMount.Name) == 0 {
		return "", fmt.Errorf("name is empty for volume mount in the component %s", componentName)
	}
	if len(radixVolumeMount.Storage) == 0 {
		return "", fmt.Errorf("storage is empty for volume mount %s in the component %s", radixVolumeMount.Name, componentName)
	}
	if len(radixVolumeMount.Path) == 0 {
		return "", fmt.Errorf("path is empty for volume mount %s in the component %s", radixVolumeMount.Name, componentName)
	}
	return fmt.Sprintf(csiVolumeNameTemplate, csiVolumeType, componentName, radixVolumeMount.Name, radixVolumeMount.Storage), nil
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

//GetVolumesForComponent Gets volumes for Radix deploy component or job
func (deploy *Deployment) GetVolumesForComponent(deployComponent radixv1.RadixCommonDeployComponent) ([]corev1.Volume, error) {
	return GetVolumes(deploy.kubeclient, deploy.getNamespace(), deploy.radixDeployment.Spec.Environment, deployComponent.GetName(), deployComponent.GetVolumeMounts())
}

//GetVolumes Get volumes of a component by RadixVolumeMounts
func GetVolumes(kubeclient kubernetes.Interface, namespace string, environment string, componentName string, volumeMounts []radixv1.RadixVolumeMount) ([]v1.Volume, error) {
	volumes := make([]corev1.Volume, 0)
	for _, volumeMount := range volumeMounts {
		switch volumeMount.Type {
		case radixv1.MountTypeBlob:
			volumes = append(volumes, getBlobFuseVolume(namespace, environment, componentName, volumeMount))
		case radixv1.MountTypeBlobCsiAzure, radixv1.MountTypeFileCsiAzure:
			volume, err := getCsiAzureVolume(kubeclient, namespace, componentName, &volumeMount)
			if err != nil {
				return nil, err
			}
			volumes = append(volumes, *volume)
		default:
			return nil, fmt.Errorf("unsupported volume type %s", volumeMount.Type)
		}
	}
	return volumes, nil
}

func getCsiAzureVolume(kubeclient kubernetes.Interface, namespace, componentName string, radixVolumeMount *radixv1.RadixVolumeMount) (*v1.Volume, error) {
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

func getPvcNotTerminating(kubeclient kubernetes.Interface, namespace string, componentName string, radixVolumeMount *radixv1.RadixVolumeMount) (*v1.PersistentVolumeClaim, error) {
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
	return fmt.Sprintf(csiPersistentVolumeClaimNameTemplate, volumeName, strings.ToLower(utils.RandString(5))), nil //volumeName: <component-name>-<csi-volume-type-dashed>-<radix-volume-name>-<storage-name>
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

func (deploy *Deployment) garbageCollectVolumeMountsSecretsNoLongerInSpecForComponent(component radixv1.RadixCommonDeployComponent, excludeSecretNames []string) error {
	secrets, err := deploy.listSecretsForVolumeMounts(component)
	if err != nil {
		return err
	}
	return deploy.GarbageCollectSecrets(secrets, excludeSecretNames)
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

func getLabelSelectorForCsiAzurePersistenceVolumeClaimForComponentStorage(componentName, radixVolumeMountName string) string {
	return fmt.Sprintf("%s=%s, %s in (%s, %s), %s = %s", kube.RadixComponentLabel, componentName, kube.RadixMountTypeLabel, string(radixv1.MountTypeBlobCsiAzure), string(radixv1.MountTypeFileCsiAzure), kube.RadixVolumeMountNameLabel, radixVolumeMountName)
}

func (deploy *Deployment) CreatePersistentVolumeClaim(appName, namespace, componentName, pvcName, storageClassName string, radixVolumeMount *radixv1.RadixVolumeMount) (*v1.PersistentVolumeClaim, error) {
	requestsVolumeMountSize, err := resource.ParseQuantity(radixVolumeMount.RequestsStorage)
	if err != nil {
		requestsVolumeMountSize = resource.MustParse("1Mi")
	}
	volumeAccessMode := getVolumeAccessMode(radixVolumeMount.AccessMode)
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: namespace,
			Labels: map[string]string{
				kube.RadixAppLabel:             appName,
				kube.RadixComponentLabel:       componentName,
				kube.RadixMountTypeLabel:       string(radixVolumeMount.Type),
				kube.RadixVolumeMountNameLabel: radixVolumeMount.Name,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{volumeAccessMode},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: requestsVolumeMountSize}, //it seems correct number is not needed for CSI driver
			},
			StorageClassName: &storageClassName,
		},
	}
	return deploy.kubeclient.CoreV1().PersistentVolumeClaims(namespace).Create(context.TODO(), pvc, metav1.CreateOptions{})
}

func populateCsiAzureStorageClass(storageClass *storagev1.StorageClass, appName string, volumeRootMount string, namespace string, componentName string, storageClassName string, radixVolumeMount *radixv1.RadixVolumeMount, secretName string, provisioner string) {
	reclaimPolicy := corev1.PersistentVolumeReclaimRetain //Using only PersistentVolumeReclaimPolicy. PersistentVolumeReclaimPolicy deletes volume on unmount.
	bindingMode := getBindingMode(radixVolumeMount.BindingMode)
	storageClass.ObjectMeta.Name = storageClassName
	storageClass.ObjectMeta.Labels = getCsiAzureStorageClassLabels(appName, namespace, componentName, radixVolumeMount)
	storageClass.Provisioner = provisioner
	storageClass.Parameters = getCsiAzureStorageClassParameters(secretName, namespace, radixVolumeMount)
	storageClass.MountOptions = getCsiAzureStorageClassMountOptions(volumeRootMount, namespace, componentName, radixVolumeMount)
	storageClass.ReclaimPolicy = &reclaimPolicy
	storageClass.VolumeBindingMode = &bindingMode
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
	} else if len(radixVolumeMount.UID) > 0 {
		mountOptions = append(mountOptions, fmt.Sprintf("-o %s=%s", csiStorageClassUidMountOption, radixVolumeMount.UID))
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
		log.Infof("Delete orphaned Csi Azure PersistantVolume %s of PersistantVolumeClaim %s", pv.Name, pv.Spec.ClaimRef.Name)
		err := deploy.DeletePersistentVolume(pv.Name)
		if err != nil {
			return err
		}
	}
	return nil
}

//CreateOrUpdateCsiAzureResources Create or update CSI Azure volume resources - StorageClasses, PersistentVolumeClaims, PersistentVolume
func (deploy *Deployment) CreateOrUpdateCsiAzureResources(desiredDeployment *appsv1.Deployment) error {
	namespace := deploy.radixDeployment.GetNamespace()
	appName := deploy.radixDeployment.Spec.AppName
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
		storageClass, storageClassIsCreated, err := deploy.createOrGetCsiAzureStorageClass(appName, volumeRootMount, namespace, componentName, radixVolumeMount, volume.Name, scMap)
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
	return err
}

func (deploy *Deployment) garbageCollectCsiAzureStorageClasses(scList *storagev1.StorageClassList, excludeStorageClassName []string) error {
	for _, storageClass := range scList.Items {
		if !slice.ContainsString(excludeStorageClassName, storageClass.Name) {
			log.Debugf("Delete Csi Azure StorageClass %s", storageClass.Name)
			err := deploy.DeleteCsiAzureStorageClasses(storageClass.Name)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (deploy *Deployment) garbageCollectCsiAzurePersistentVolumeClaimsAndPersistentVolumes(namespace string, pvcList *v1.PersistentVolumeClaimList, excludePvcNames []string) error {
	for _, pvc := range pvcList.Items {
		if !slice.ContainsString(excludePvcNames, pvc.Name) {
			pvName := pvc.Spec.VolumeName
			log.Debugf("Delete not used CSI Azure PersistentVolumeClaim %s in namespace %s", pvc.Name, namespace)
			err := deploy.DeletePersistentVolumeClaim(namespace, pvc.Name)
			if err != nil {
				return err
			}
			log.Debugf("Delete not used CSI Azure PersistentVolume %s in namespace %s", pvName, namespace)
			err = deploy.DeletePersistentVolume(pvName)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (deploy *Deployment) createCsiAzurePersistentVolumeClaim(storageClass *storagev1.StorageClass, requiredNewPvc bool, appName, namespace, componentName string, radixVolumeMount *radixv1.RadixVolumeMount, persistentVolumeClaimName string, pvcMap map[string]*v1.PersistentVolumeClaim) (*v1.PersistentVolumeClaim, error) {
	if pvc, ok := pvcMap[persistentVolumeClaimName]; ok {
		if pvc.Spec.StorageClassName == nil || len(*pvc.Spec.StorageClassName) == 0 {
			return pvc, nil
		}
		if !requiredNewPvc && strings.EqualFold(*pvc.Spec.StorageClassName, storageClass.Name) {
			return pvc, nil
		}

		log.Debugf("Delete PersistentVolumeClaim %s in namespace %s: changed StorageClass name to %s", pvc.Name, namespace, storageClass.Name)
		err := deploy.DeletePersistentVolumeClaim(namespace, pvc.Name)
		if err != nil {
			return nil, err
		}
	}
	persistentVolumeClaimName, err := createCsiAzurePersistentVolumeClaimName(componentName, radixVolumeMount)
	if err != nil {
		return nil, err
	}
	log.Debugf("Create PersistentVolumeClaim %s in namespace %s for StorageClass %s", persistentVolumeClaimName, namespace, storageClass.Name)
	return deploy.CreatePersistentVolumeClaim(appName, namespace, componentName, persistentVolumeClaimName, storageClass.Name, radixVolumeMount)
}

func (deploy *Deployment) createOrGetCsiAzureStorageClass(appName, volumeRootMount, namespace, componentName string, radixVolumeMount *radixv1.RadixVolumeMount, volumeName string, scMap map[string]*storagev1.StorageClass) (*storagev1.StorageClass, bool, error) {
	volumeMountProvisioner, foundProvisioner := radixv1.GetStorageClassProvisionerByVolumeMountType(radixVolumeMount.Type)
	if !foundProvisioner {
		return nil, false, fmt.Errorf("not found Storage Class provisioner for volume mount type %s", string(radixVolumeMount.Type))
	}
	storageClassName := GetCsiAzureStorageClassName(namespace, volumeName)
	csiVolumeSecretName := defaults.GetCsiAzureCredsSecretName(componentName, radixVolumeMount.Name)
	if existingStorageClass, exists := scMap[storageClassName]; exists {
		desiredStorageClass := existingStorageClass.DeepCopy()
		populateCsiAzureStorageClass(desiredStorageClass, appName, volumeRootMount, namespace, componentName, storageClassName, radixVolumeMount, csiVolumeSecretName, volumeMountProvisioner)
		if equal, err := compareStorageClasses(existingStorageClass, desiredStorageClass); equal || err != nil {
			return existingStorageClass, false, err
		}

		log.Infof("Delete StorageClass %s in namespace %s", existingStorageClass.Name, namespace)
		err := deploy.DeleteCsiAzureStorageClasses(existingStorageClass.Name)
		if err != nil {
			return nil, false, err
		}
	}

	log.Debugf("Create StorageClass %s in namespace %s", storageClassName, namespace)
	storageClass := &storagev1.StorageClass{}
	populateCsiAzureStorageClass(storageClass, appName, volumeRootMount, namespace, componentName, storageClassName, radixVolumeMount, csiVolumeSecretName, volumeMountProvisioner)
	desiredStorageClass, err := deploy.kubeclient.StorageV1().StorageClasses().Create(context.TODO(), storageClass, metav1.CreateOptions{})
	return desiredStorageClass, true, err
}

func compareStorageClasses(sc1 *storagev1.StorageClass, sc2 *storagev1.StorageClass) (bool, error) {
	sc1Copy := sc1.DeepCopy()
	sc1Copy.ObjectMeta.ManagedFields = nil //HACK: to avoid ManagedFields comparison
	sc2Copy := sc2.DeepCopy()
	sc2Copy.ObjectMeta.ManagedFields = nil //HACK: to avoid ManagedFields comparison
	json1, err := json.Marshal(sc1Copy)
	if err != nil {
		return false, fmt.Errorf("failed to marshal StorageClass object: %v", err)
	}
	json2, err := json.Marshal(sc2Copy)
	if err != nil {
		return false, fmt.Errorf("failed to marshal StorageClass object: %v", err)
	}
	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(json1, json2, storagev1.StorageClass{})
	if err != nil {
		return false, fmt.Errorf("failed to create two way merge patch StorageClass objects: %v", err)
	}
	return kube.IsEmptyPatch(patchBytes), err
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
		if !radixv1.IsKnownCsiAzureVolumeMount(string(radixVolumeMount.Type)) {
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

func getPersistentVolumeClaimMapByName(pvcList *corev1.PersistentVolumeClaimList) map[string]*corev1.PersistentVolumeClaim {
	pvcMap := make(map[string]*corev1.PersistentVolumeClaim)
	for _, pvc := range pvcList.Items {
		persistentVolumeClaim := pvc
		pvcMap[pvc.Name] = &persistentVolumeClaim
	}
	return pvcMap
}

func getStorageClassMapByName(scList *storagev1.StorageClassList) map[string]*storagev1.StorageClass {
	scMap := make(map[string]*storagev1.StorageClass)
	for _, sc := range scList.Items {
		storageClass := sc
		scMap[sc.Name] = &storageClass
	}
	return scMap
}

func getVolumeAccessMode(modeValue string) v1.PersistentVolumeAccessMode {
	switch strings.ToLower(modeValue) {
	case strings.ToLower(string(corev1.ReadWriteOnce)):
		return corev1.ReadWriteOnce
	case strings.ToLower(string(corev1.ReadWriteMany)):
		return corev1.ReadWriteMany
	}
	return corev1.ReadOnlyMany
}

func sortPvcsByCreatedTimestampDesc(persistentVolumeClaims []v1.PersistentVolumeClaim) []v1.PersistentVolumeClaim {
	sort.SliceStable(persistentVolumeClaims, func(i, j int) bool {
		return (persistentVolumeClaims)[j].ObjectMeta.CreationTimestamp.Before(&(persistentVolumeClaims)[i].ObjectMeta.CreationTimestamp)
	})
	return persistentVolumeClaims
}

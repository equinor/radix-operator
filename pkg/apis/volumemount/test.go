package volumemount

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	"github.com/equinor/radix-operator/pkg/apis/internal"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/google/uuid"
	kedav2 "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	secretsstorevclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

func modifyPvc(pvc v1.PersistentVolumeClaim, modify func(pvc *v1.PersistentVolumeClaim)) v1.PersistentVolumeClaim {
	modify(&pvc)
	return pvc
}

type testSuite struct {
	suite.Suite
	radixCommonDeployComponentFactories []radixv1.RadixCommonDeployComponentFactory
}

type TestEnv struct {
	kubeClient           kubernetes.Interface
	radixClient          versioned.Interface
	secretProviderClient secretsstorevclient.Interface
	prometheusClient     prometheusclient.Interface
	kubeUtil             *kube.Kube
	kedaClient           kedav2.Interface
}

type volumeMountTestScenario struct {
	name               string
	radixVolumeMount   radixv1.RadixVolumeMount
	expectedVolumeName string
	expectedError      string
	expectedPvcName    string
}

type deploymentVolumesTestScenario struct {
	name              string
	props             expectedPvcPvProperties
	radixVolumeMounts []radixv1.RadixVolumeMount
	volumes           []v1.Volume
	existingPvs       []v1.PersistentVolume
	existingPvcs      []v1.PersistentVolumeClaim
	expectedPvs       []v1.PersistentVolume
	expectedPvcs      []v1.PersistentVolumeClaim
}

type pvcTestScenario struct {
	volumeMountTestScenario
	pv  v1.PersistentVolume
	pvc v1.PersistentVolumeClaim
}

const (
	appName1                      = "any-app"
	envName1                      = "some-env"
	componentName1                = "some-component"
	testClientId                  = "some-client-id"
	testTenantId                  = "some-tenant-id"
	testSubscriptionId            = "some-subscription-id"
	testResourceGroup             = "some-resource-group"
	testStorageAccountName        = "somestorageaccountname"
	testChangedStorageAccountName = "changedstorageaccountname"
)

var (
	anotherComponentName   = strings.ToLower(utils.RandString(10))
	anotherVolumeMountName = strings.ToLower(utils.RandString(10))
)

func (s *testSuite) SetupSuite() {
	s.radixCommonDeployComponentFactories = []radixv1.RadixCommonDeployComponentFactory{
		radixv1.RadixDeployComponentFactory{},
		radixv1.RadixDeployJobComponentFactory{},
	}
}

func getTestEnv() TestEnv {
	testEnv := TestEnv{
		kubeClient:           fake.NewSimpleClientset(),
		radixClient:          radixfake.NewSimpleClientset(),
		kedaClient:           kedafake.NewSimpleClientset(),
		secretProviderClient: secretproviderfake.NewSimpleClientset(),
		prometheusClient:     prometheusfake.NewSimpleClientset(),
	}
	kubeUtil, _ := kube.New(testEnv.kubeClient, testEnv.radixClient, testEnv.kedaClient, testEnv.secretProviderClient)
	testEnv.kubeUtil = kubeUtil
	return testEnv
}

type expectedPvcPvProperties struct {
	appName                 string
	environment             string
	componentName           string
	radixVolumeMountName    string
	blobStorageName         string
	pvcName                 string
	persistentVolumeName    string
	radixVolumeMountType    radixv1.MountType
	requestsVolumeMountSize string
	volumeAccessMode        v1.PersistentVolumeAccessMode
	volumeName              string
	pvProvisioner           string
	pvSecretName            string
	pvGid                   string
	pvUid                   string
	namespace               string
	readOnly                bool
	clientId                string
	resourceGroup           string
	subscriptionId          string
	tenantId                string
	storageAccountName      string
}

func modifyPv(pv v1.PersistentVolume, modify func(pv *v1.PersistentVolume)) v1.PersistentVolume {
	modify(&pv)
	return pv
}

func createRandomVolumeMount(modify func(mount *radixv1.RadixVolumeMount)) radixv1.RadixVolumeMount {
	vm := radixv1.RadixVolumeMount{
		Name: strings.ToLower(operatorUtils.RandString(10)),
		Path: "/tmp/" + strings.ToLower(operatorUtils.RandString(10)),
		BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
			Protocol:  radixv1.BlobFuse2ProtocolFuse2,
			Container: strings.ToLower(operatorUtils.RandString(10)),
		},
	}
	modify(&vm)
	return vm
}

func matchPvAndPvc(pv *v1.PersistentVolume, pvc *v1.PersistentVolumeClaim) {
	pv.Spec.CSI.VolumeAttributes[csiVolumeMountAttributePvcName] = pvc.GetName()
	pv.Spec.CSI.VolumeAttributes[csiVolumeMountAttributePvcNamespace] = pvc.GetNamespace()
	pv.Spec.ClaimRef = &v1.ObjectReference{
		APIVersion: "radixv1",
		Kind:       k8s.KindPersistentVolumeClaim,
		Name:       pvc.GetName(),
		Namespace:  pvc.GetNamespace(),
	}
	pvc.Spec.VolumeName = pv.Name
}

func modifyProps(props expectedPvcPvProperties, modify func(props *expectedPvcPvProperties)) expectedPvcPvProperties {
	modify(&props)
	return props
}

func createRandomPv(props expectedPvcPvProperties, namespace, componentName string) v1.PersistentVolume {
	return createExpectedPv(props, func(pv *v1.PersistentVolume) {
		pvName := getCsiAzurePvName()
		pv.ObjectMeta.Name = pvName
		pv.ObjectMeta.Labels[kube.RadixNamespace] = namespace
		pv.ObjectMeta.Labels[kube.RadixComponentLabel] = componentName
		pv.Spec.CSI.VolumeAttributes[csiVolumeMountAttributePvName] = pvName
	})
}

func createRandomPvc(props expectedPvcPvProperties, namespace, componentName string) v1.PersistentVolumeClaim {
	return createExpectedPvc(props, func(pvc *v1.PersistentVolumeClaim) {
		pvcName, err := getCsiAzurePvcName(componentName, &radixv1.RadixVolumeMount{Name: props.radixVolumeMountName, Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Storage: props.blobStorageName, Path: "/tmp"})
		if err != nil {
			panic(err)
		}
		pvName := getCsiAzurePvName()
		pvc.ObjectMeta.Name = pvcName
		pvc.ObjectMeta.Namespace = namespace
		pvc.ObjectMeta.Labels[kube.RadixComponentLabel] = componentName
		pvc.Spec.VolumeName = pvName
	})
}

func createRandomPvcBlobFuse2(props expectedPvcPvProperties, namespace, componentName string) v1.PersistentVolumeClaim {
	return createExpectedPvc(props, func(pvc *v1.PersistentVolumeClaim) {
		pvcName, err := getCsiAzurePvcName(componentName, &radixv1.RadixVolumeMount{Name: props.radixVolumeMountName, Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Storage: props.blobStorageName, Path: "/tmp"})
		if err != nil {
			panic(err)
		}
		pvName := getCsiAzurePvName()
		pvc.ObjectMeta.Name = pvcName
		pvc.ObjectMeta.Namespace = namespace
		pvc.ObjectMeta.Labels[kube.RadixComponentLabel] = componentName
		pvc.Spec.VolumeName = pvName
	})
}

func createRandomAutoProvisionedPvWithStorageClass(props expectedPvcPvProperties, namespace, componentName, anotherVolumeMountName string) v1.PersistentVolume {
	return createAutoProvisionedPvWithStorageClass(props, func(pv *v1.PersistentVolume) {
		pvName := "pvc-" + uuid.NewString()
		pv.ObjectMeta.Name = pvName
		if pv.ObjectMeta.Labels == nil {
			pv.ObjectMeta.Labels = make(map[string]string)
		}
		pv.ObjectMeta.Labels[kube.RadixNamespace] = namespace
		pv.ObjectMeta.Labels[kube.RadixComponentLabel] = componentName
		pv.ObjectMeta.Labels[kube.RadixVolumeMountNameLabel] = anotherVolumeMountName
		pv.Spec.CSI.VolumeAttributes[csiVolumeMountAttributePvName] = pvName
	})
}

func createRandomAutoProvisionedPvcWithStorageClass(props expectedPvcPvProperties, namespace, componentName, anotherVolumeMountName string) v1.PersistentVolumeClaim {
	return createExpectedAutoProvisionedPvcWithStorageClass(props, func(pvc *v1.PersistentVolumeClaim) {
		pvcName, err := getCsiAzurePvcName(componentName, &radixv1.RadixVolumeMount{Name: props.radixVolumeMountName, Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Storage: props.blobStorageName, Path: "/tmp"})
		if err != nil {
			panic(err)
		}
		pvName := getCsiAzurePvName()
		pvc.ObjectMeta.Name = pvcName
		pvc.ObjectMeta.Namespace = namespace
		pvc.ObjectMeta.Labels[kube.RadixComponentLabel] = componentName
		pvc.ObjectMeta.Labels[kube.RadixVolumeMountNameLabel] = anotherVolumeMountName
		pvc.Spec.VolumeName = pvName
	})
}

func getPropsCsiBlobVolume1Storage1(modify func(*expectedPvcPvProperties)) expectedPvcPvProperties {
	props := expectedPvcPvProperties{
		appName:                 appName1,
		environment:             envName1,
		namespace:               operatorUtils.GetEnvironmentNamespace(appName1, envName1),
		componentName:           componentName1,
		radixVolumeMountName:    "volume1",
		blobStorageName:         "storage1",
		pvcName:                 "pvc-csi-az-blob-some-component-volume1-storage1-12345",
		persistentVolumeName:    "pv-radixvolumemount-some-uuid",
		radixVolumeMountType:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
		requestsVolumeMountSize: "1Mi",
		volumeAccessMode:        v1.ReadOnlyMany, // default access mode
		volumeName:              "csi-az-blob-some-component-volume1-storage1",
		pvProvisioner:           provisionerBlobCsiAzure,
		pvSecretName:            "some-component-volume1-csiazurecreds",
		pvGid:                   "1000",
		pvUid:                   "",
		readOnly:                true,
	}
	if modify != nil {
		modify(&props)
	}
	return props
}

func getPropsCsiBlobFuse2Volume1Storage1(modify func(*expectedPvcPvProperties)) expectedPvcPvProperties {
	props := expectedPvcPvProperties{
		appName:                 appName1,
		environment:             envName1,
		namespace:               fmt.Sprintf("%s-%s", appName1, envName1),
		componentName:           componentName1,
		radixVolumeMountName:    "volume1",
		blobStorageName:         "storage1",
		pvcName:                 "pvc-csi-blobfuse2-fuse2-some-component-volume1-storage1-12345",
		persistentVolumeName:    "pv-radixvolumemount-some-uuid",
		radixVolumeMountType:    radixv1.MountTypeBlobFuse2Fuse2CsiAzure,
		requestsVolumeMountSize: "1Mi",
		volumeAccessMode:        v1.ReadOnlyMany, // default access mode
		volumeName:              "csi-blobfuse2-fuse2-some-component-volume1-storage1",
		pvProvisioner:           provisionerBlobCsiAzure,
		pvSecretName:            "some-component-volume1-csiazurecreds",
		pvGid:                   "1000",
		pvUid:                   "",
		readOnly:                true,
	}
	if modify != nil {
		modify(&props)
	}
	return props
}

func putExistingDeploymentVolumesScenarioDataToFakeCluster(kubeClient kubernetes.Interface, scenario *deploymentVolumesTestScenario) {
	for _, pvc := range scenario.existingPvcs {
		_, _ = kubeClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Create(context.Background(), &pvc, metav1.CreateOptions{})
	}
	for _, pv := range scenario.existingPvs {
		_, _ = kubeClient.CoreV1().PersistentVolumes().Create(context.Background(), &pv, metav1.CreateOptions{})
	}
}

func getExistingPvcsAndPersistentVolumeFromFakeCluster(kubeClient kubernetes.Interface) ([]v1.PersistentVolumeClaim, []v1.PersistentVolume, error) {
	var pvcItems []v1.PersistentVolumeClaim
	var pvItems []v1.PersistentVolume
	pvcList, err := kubeClient.CoreV1().PersistentVolumeClaims("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}
	if pvcList != nil && pvcList.Items != nil {
		pvcItems = pvcList.Items
	}
	pvList, err := kubeClient.CoreV1().PersistentVolumes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}
	if pvList != nil && pvList.Items != nil {
		pvItems = pvList.Items
	}
	return pvcItems, pvItems, nil
}

func getDesiredDeployment(componentName string, volumes []v1.Volume) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: componentName,
			Labels: map[string]string{
				kube.RadixComponentLabel: componentName,
			},
			Annotations: make(map[string]string),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointers.Ptr(defaults.DefaultReplicas),
			Selector: &metav1.LabelSelector{MatchLabels: make(map[string]string)},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: make(map[string]string), Annotations: make(map[string]string)},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{Name: componentName}},
					Volumes:    volumes,
				},
			},
		},
	}
}

func buildRd(appName string, environment string, componentName string, identityAzureClientId string, radixVolumeMounts []radixv1.RadixVolumeMount) *radixv1.RadixDeployment {
	return operatorUtils.ARadixDeployment().
		WithAppName(appName).
		WithEnvironment(environment).
		WithComponents(operatorUtils.NewDeployComponentBuilder().
			WithName(componentName).
			WithIdentity(&radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: identityAzureClientId}}).
			WithVolumeMounts(radixVolumeMounts...)).
		BuildRD()
}

func createPvc(namespace, componentName string, mountType radixv1.MountType, modify func(*v1.PersistentVolumeClaim)) v1.PersistentVolumeClaim {
	appName := "app"
	pvc := v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.RandString(10), // Set in test scenario
			Namespace: namespace,
			Labels: map[string]string{
				kube.RadixAppLabel:             appName,
				kube.RadixComponentLabel:       componentName,
				kube.RadixMountTypeLabel:       string(mountType),
				kube.RadixVolumeMountNameLabel: utils.RandString(10), // Set in test scenario
			},
		},
	}
	if modify != nil {
		modify(&pvc)
	}
	return pvc
}

func createExpectedPv(props expectedPvcPvProperties, modify func(pv *v1.PersistentVolume)) v1.PersistentVolume {
	pv := createPv(props)
	pv.Spec.PersistentVolumeSource.CSI.NodeStageSecretRef = &v1.SecretReference{
		Name:      props.pvSecretName,
		Namespace: props.namespace,
	}
	pv.Spec.PersistentVolumeSource.CSI.VolumeAttributes[csiVolumeMountAttributeSecretNamespace] = props.namespace
	pv.Spec.PersistentVolumeSource.CSI.NodeStageSecretRef = &v1.SecretReference{
		Name:      props.pvSecretName,
		Namespace: props.namespace,
	}

	if modify != nil {
		modify(pv)
	}
	return *pv
}

func createExpectedPvWithIdentity(props expectedPvcPvProperties, modify func(pv *v1.PersistentVolume)) v1.PersistentVolume {
	pv := createPv(props)

	pv.Spec.PersistentVolumeSource.CSI.VolumeAttributes[csiVolumeAttributeStorageAccount] = props.storageAccountName
	pv.Spec.PersistentVolumeSource.CSI.VolumeAttributes[csiVolumeAttributeClientID] = props.clientId
	pv.Spec.PersistentVolumeSource.CSI.VolumeAttributes[csiVolumeAttributeResourceGroup] = props.resourceGroup
	if props.subscriptionId != "" {
		pv.Spec.PersistentVolumeSource.CSI.VolumeAttributes[csiVolumeAttributeSubscriptionId] = props.subscriptionId
	}
	if props.tenantId != "" {
		pv.Spec.PersistentVolumeSource.CSI.VolumeAttributes[csiVolumeAttributeTenantId] = props.tenantId
	}
	setVolumeMountAttribute(pv, props.radixVolumeMountType, props.blobStorageName, props.pvcName)
	if modify != nil {
		modify(pv)
	}
	return *pv
}

func createPv(props expectedPvcPvProperties) *v1.PersistentVolume {
	mountOptions := getMountOptions(props)
	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: props.persistentVolumeName,
			Labels: map[string]string{
				kube.RadixAppLabel:             props.appName,
				kube.RadixNamespace:            props.namespace,
				kube.RadixComponentLabel:       props.componentName,
				kube.RadixVolumeMountNameLabel: props.radixVolumeMountName,
			},
		},
		Spec: v1.PersistentVolumeSpec{
			Capacity: v1.ResourceList{v1.ResourceStorage: resource.MustParse(props.requestsVolumeMountSize)},
			PersistentVolumeSource: v1.PersistentVolumeSource{
				CSI: &v1.CSIPersistentVolumeSource{
					Driver:       props.pvProvisioner,
					VolumeHandle: getVolumeHandle(props.namespace, props.componentName, props.persistentVolumeName, props.blobStorageName),
					VolumeAttributes: map[string]string{
						csiVolumeMountAttributeContainerName: props.blobStorageName,
						csiVolumeMountAttributeProtocol:      csiVolumeAttributeProtocolParameterFuse2,
						csiVolumeMountAttributePvName:        props.persistentVolumeName,
						csiVolumeMountAttributePvcName:       props.pvcName,
						csiVolumeMountAttributePvcNamespace:  props.namespace,
						// skip auto-created by the provisioner "storage.kubernetes.io/csiProvisionerIdentity": "1732528668611-2190-blob.csi.azure.com"
					},
				},
			},
			AccessModes: []v1.PersistentVolumeAccessMode{props.volumeAccessMode},
			ClaimRef: &v1.ObjectReference{
				APIVersion: "radixv1",
				Kind:       k8s.KindPersistentVolumeClaim,
				Namespace:  props.namespace,
				Name:       props.pvcName,
			},
			StorageClassName:              "",
			MountOptions:                  mountOptions,
			PersistentVolumeReclaimPolicy: v1.PersistentVolumeReclaimRetain,
			VolumeMode:                    pointers.Ptr(v1.PersistentVolumeFilesystem),
		},
		Status: v1.PersistentVolumeStatus{Phase: v1.VolumeBound},
	}
	setVolumeMountAttribute(pv, props.radixVolumeMountType, props.blobStorageName, props.pvcName)
	return pv
}

func createAutoProvisionedPvWithStorageClass(props expectedPvcPvProperties, modify func(pv *v1.PersistentVolume)) v1.PersistentVolume {
	mountOptions := getMountOptionsInRandomOrder(props)
	pv := v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: props.persistentVolumeName,
		},
		Spec: v1.PersistentVolumeSpec{
			Capacity: v1.ResourceList{v1.ResourceStorage: resource.MustParse(props.requestsVolumeMountSize)},
			PersistentVolumeSource: v1.PersistentVolumeSource{
				CSI: &v1.CSIPersistentVolumeSource{
					Driver:       props.pvProvisioner,
					VolumeHandle: "MC_clusters_ABC_northeurope##testdata#pvc-681b9ffc-66cc-4e09-90b2-872688b792542#some-app-namespace#",
					VolumeAttributes: map[string]string{
						csiVolumeMountAttributeContainerName:       props.blobStorageName,
						csiVolumeMountAttributeProtocol:            csiVolumeAttributeProtocolParameterFuse2,
						csiVolumeMountAttributePvName:              props.persistentVolumeName,
						csiVolumeMountAttributePvcName:             props.pvcName,
						csiVolumeMountAttributePvcNamespace:        props.namespace,
						csiVolumeMountAttributeSecretNamespace:     props.namespace,
						csiVolumeMountAttributeProvisionerIdentity: "6540128941979-5154-blob.csi.azure.com",
					},
					NodeStageSecretRef: &v1.SecretReference{
						Name:      props.pvSecretName,
						Namespace: props.namespace,
					},
				},
			},
			AccessModes: []v1.PersistentVolumeAccessMode{props.volumeAccessMode},
			ClaimRef: &v1.ObjectReference{
				APIVersion: "radixv1",
				Kind:       k8s.KindPersistentVolumeClaim,
				Namespace:  props.namespace,
				Name:       props.pvcName,
			},
			StorageClassName:              "some-storage-class",
			MountOptions:                  mountOptions,
			PersistentVolumeReclaimPolicy: v1.PersistentVolumeReclaimRetain,
			VolumeMode:                    pointers.Ptr(v1.PersistentVolumeFilesystem),
		},
		Status: v1.PersistentVolumeStatus{Phase: v1.VolumeBound, LastPhaseTransitionTime: pointers.Ptr(metav1.Time{Time: time.Now()})},
	}
	setVolumeMountAttribute(&pv, props.radixVolumeMountType, props.blobStorageName, props.pvcName)
	if modify != nil {
		modify(&pv)
	}
	return pv
}

func getMountOptions(props expectedPvcPvProperties, extraOptions ...string) []string {
	options := []string{
		"--file-cache-timeout-in-seconds=120",
		"--use-attr-cache=true",
		"--cancel-list-on-mount-seconds=0",
		"-o allow_other",
		"-o attr_timeout=120",
		"-o entry_timeout=120",
		"-o negative_timeout=120",
	}
	if props.readOnly {
		options = append(options, ReadOnlyMountOption)
	}
	idOption := getPersistentVolumeIdMountOption(props)
	if len(idOption) > 0 {
		options = append(options, idOption)
	}
	if props.radixVolumeMountType == radixv1.MountTypeBlobFuse2Fuse2CsiAzure {
		options = append(options, fmt.Sprintf("--%s=%v", csiMountOptionUseAdls, false))
	}
	return append(options, extraOptions...)
}

func getMountOptionsInRandomOrder(props expectedPvcPvProperties, extraOptions ...string) []string {
	options := []string{
		"--file-cache-timeout-in-seconds=120",
		"--use-attr-cache=true",
		"-o allow_other",
		"--cancel-list-on-mount-seconds=0",
		"-o negative_timeout=120",
		"-o entry_timeout=120",
		"-o attr_timeout=120",
	}
	idOption := getPersistentVolumeIdMountOption(props)
	if len(idOption) > 0 {
		options = append(options, idOption)
	}
	if props.readOnly {
		options = append(options, ReadOnlyMountOption)
	}
	return append(options, extraOptions...)
}

func setVolumeMountAttribute(pv *v1.PersistentVolume, radixVolumeMountType radixv1.MountType, containerName, pvcName string) {
	pv.Spec.CSI.VolumeAttributes[csiVolumeMountAttributeContainerName] = containerName
	pv.Spec.CSI.VolumeAttributes[csiVolumeMountAttributePvcName] = pvcName
	switch radixVolumeMountType {
	case radixv1.MountTypeBlobFuse2FuseCsiAzure:
		pv.Spec.CSI.VolumeAttributes[csiVolumeMountAttributeProtocol] = csiVolumeAttributeProtocolParameterFuse
	case radixv1.MountTypeBlobFuse2Fuse2CsiAzure:
		pv.Spec.CSI.VolumeAttributes[csiVolumeMountAttributeProtocol] = csiVolumeAttributeProtocolParameterFuse2
	}
}

func getPersistentVolumeIdMountOption(props expectedPvcPvProperties) string {
	if len(props.pvGid) > 0 {
		return fmt.Sprintf("-o gid=%s", props.pvGid)
	}
	if len(props.pvUid) > 0 {
		return fmt.Sprintf("-o uid=%s", props.pvUid)
	}
	return ""
}

func createExpectedPvc(props expectedPvcPvProperties, modify func(*v1.PersistentVolumeClaim)) v1.PersistentVolumeClaim {
	labels := map[string]string{
		kube.RadixAppLabel:             props.appName,
		kube.RadixComponentLabel:       props.componentName,
		kube.RadixMountTypeLabel:       string(props.radixVolumeMountType),
		kube.RadixVolumeMountNameLabel: props.radixVolumeMountName,
	}
	pvc := v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      props.pvcName,
			Namespace: props.namespace,
			Labels:    labels,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{props.volumeAccessMode},
			Resources: v1.VolumeResourceRequirements{
				Requests: v1.ResourceList{v1.ResourceStorage: resource.MustParse(props.requestsVolumeMountSize)}, // it seems correct number is not needed for CSI driver
			},
			VolumeName: props.persistentVolumeName,
			VolumeMode: pointers.Ptr(v1.PersistentVolumeFilesystem),
		},
		Status: v1.PersistentVolumeClaimStatus{Phase: v1.ClaimBound},
	}
	if modify != nil {
		modify(&pvc)
	}
	return pvc
}

func createExpectedAutoProvisionedPvcWithStorageClass(props expectedPvcPvProperties, modify func(*v1.PersistentVolumeClaim)) v1.PersistentVolumeClaim {
	labels := map[string]string{
		kube.RadixAppLabel:             props.appName,
		kube.RadixComponentLabel:       props.componentName,
		kube.RadixMountTypeLabel:       string(props.radixVolumeMountType),
		kube.RadixVolumeMountNameLabel: props.radixVolumeMountName,
	}
	annotations := map[string]string{
		"pv.kubernetes.io/bind-completed":               "yes",
		"pv.kubernetes.io/bound-by-controller":          "yes",
		"volume.beta.kubernetes.io/storage-provisioner": props.pvProvisioner,
		"volume.kubernetes.io/storage-provisioner":      props.pvProvisioner,
	}
	pvc := v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:            props.pvcName,
			Namespace:       props.namespace,
			Labels:          labels,
			Annotations:     annotations,
			Finalizers:      []string{"kubernetes.io/pvc-protection"},
			ResourceVersion: "630363277",
			UID:             types.UID("681b9ffc-66cc-4e09-90b2-872688b792542"),
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{props.volumeAccessMode},
			Resources: v1.VolumeResourceRequirements{
				Requests: v1.ResourceList{v1.ResourceStorage: resource.MustParse(props.requestsVolumeMountSize)}, // it seems correct number is not needed for CSI driver
			},
			VolumeName:       props.persistentVolumeName,
			StorageClassName: pointers.Ptr("some-storage-class"),
		},
		Status: v1.PersistentVolumeClaimStatus{
			Phase:       v1.ClaimBound,
			AccessModes: []v1.PersistentVolumeAccessMode{props.volumeAccessMode},
			Capacity:    map[v1.ResourceName]resource.Quantity{v1.ResourceStorage: resource.MustParse(props.requestsVolumeMountSize)},
		},
	}
	if modify != nil {
		modify(&pvc)
	}
	return pvc
}

func createTestVolume(pvcProps expectedPvcPvProperties, modify func(v *v1.Volume)) v1.Volume {
	volume := v1.Volume{
		Name: pvcProps.volumeName,
		VolumeSource: v1.VolumeSource{PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
			ClaimName: pvcProps.pvcName,
		}},
	}
	if modify != nil {
		modify(&volume)
	}
	return volume
}

func createRadixVolumeMount(props expectedPvcPvProperties, modify func(mount *radixv1.RadixVolumeMount)) radixv1.RadixVolumeMount {
	volumeMount := radixv1.RadixVolumeMount{
		Type:    props.radixVolumeMountType,
		Name:    props.radixVolumeMountName,
		Storage: props.blobStorageName,
		Path:    "path1",
		GID:     "1000",
	}
	if modify != nil {
		modify(&volumeMount)
	}
	return volumeMount
}

func createBlobFuse2RadixVolumeMount(props expectedPvcPvProperties, modify func(mount *radixv1.RadixVolumeMount)) radixv1.RadixVolumeMount {
	volumeMount := radixv1.RadixVolumeMount{
		Name: props.radixVolumeMountName,
		Path: "path1",
		BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
			Container: props.blobStorageName,
			GID:       "1000",
		},
	}
	if modify != nil {
		modify(&volumeMount)
	}
	return volumeMount
}

func equalPersistentVolumes(expectedPvs, actualPvs *[]v1.PersistentVolume) bool {
	if len(*expectedPvs) != len(*actualPvs) {
		return false
	}
	for _, expectedPv := range *expectedPvs {
		for _, actualPv := range *actualPvs {
			if EqualPersistentVolumes(&expectedPv, &actualPv) {
				return true
			}
		}
	}
	return false
}

func equalPersistentVolumeClaims(list1, list2 *[]v1.PersistentVolumeClaim) bool {
	if len(*list1) != len(*list2) {
		return false
	}
	for _, pvc1 := range *list1 {
		var hasEqualPvc bool
		for _, pvc2 := range *list2 {
			if internal.EqualTillPostfix(pvc1.GetName(), pvc2.GetName(), nameRandPartLength) &&
				EqualPersistentVolumeClaims(&pvc1, &pvc2) {
				hasEqualPvc = true
				break
			}
		}
		if !hasEqualPvc {
			return false
		}
	}
	return true
}

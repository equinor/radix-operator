package deployment

import (
	"context"
	"errors"
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	storagev1 "k8s.io/api/storage/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"strings"

	"k8s.io/apimachinery/pkg/util/intstr"

	log "github.com/sirupsen/logrus"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	conditionUtils "github.com/equinor/radix-operator/pkg/apis/utils/conditions"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PRIVILEGED_CONTAINER       = false
	ALLOW_PRIVILEGE_ESCALATION = false
)

func (deploy *Deployment) createOrUpdateDeployment(deployComponent v1.RadixCommonDeployComponent) error {
	currentDeployment, desiredDeployment, err := deploy.getCurrentAndDesiredDeployment(deployComponent)
	if err != nil {
		return err
	}

	// If Replicas == 0 or HorizontalScaling is nil then delete hpa if exists before updating deployment
	deployReplicas := desiredDeployment.Spec.Replicas
	if deployReplicas != nil && *deployReplicas == 0 || deployComponent.GetHorizontalScaling() == nil {
		err = deploy.deleteHPAIfExists(deployComponent.GetName())
		if err != nil {
			return err
		}
	}

	err = deploy.createOrUpdateCsiVolumeResources(desiredDeployment)
	if err != nil {
		return err
	}

	return deploy.kubeutil.ApplyDeployment(deploy.radixDeployment.Namespace, currentDeployment, desiredDeployment)
}

func (deploy *Deployment) createOrUpdateCsiVolumeResources(desiredDeployment *appsv1.Deployment) error {
	namespace := deploy.radixDeployment.GetNamespace()
	componentName := desiredDeployment.ObjectMeta.Name
	scList, err := deploy.GetStorageClasses(namespace, componentName)
	if err != nil {
		return err
	}
	pvcList, err := deploy.GetPersistentVolumeClaims(namespace, componentName)
	if err != nil {
		return err
	}

	scMap := getStorageClassMap(scList)
	pvcMap := getPersistentVolumeClaimMap(pvcList)
	radixVolumeMountMap := deploy.getRadixVolumeMountMap(componentName)

	for _, volume := range desiredDeployment.Spec.Template.Spec.Volumes {
		radixVolumeMount, foundRadixVolumeMount := radixVolumeMountMap[volume.Name]
		if !foundRadixVolumeMount {
			return errors.New(fmt.Sprintf("not found Radix volume mount for desired volume mount %s", volume.Name))
		}
		err := deploy.processVolumeResources(namespace, componentName, radixVolumeMount, &volume, scMap, pvcMap)
		if err != nil {
			return err
		}
	}

	//TODO: garbageCollectCsiAzureStorageClasses()
	//TODO: garbageCollectCsiAzurePersistentVolumeClaims()
	return nil
}

func (deploy *Deployment) processVolumeResources(namespace, componentName string, radixVolumeMount *v1.RadixVolumeMount, volume *corev1.Volume, scMap map[string]*storagev1.StorageClass, pvcMap map[string]*corev1.PersistentVolumeClaim) error {
	if volume.PersistentVolumeClaim == nil {
		return nil //Not CSI volume
	}
	csiVolumeStorageClassName := GetCsiStorageClassName(namespace, volume.Name)
	csiPersistentVolumeClaimName := GetCsiPersistentVolumeClaimName(volume.Name)
	if pvc, ok := pvcMap[volume.PersistentVolumeClaim.ClaimName]; ok {
		if pvc.Spec.StorageClassName == nil || len(*pvc.Spec.StorageClassName) == 0 {
			return nil //Not CSI volume
		}
	} else {
		pvc, err := deploy.CreatePersistentVolumeClaim(namespace, componentName, csiPersistentVolumeClaimName, csiVolumeStorageClassName, volume.Name)
		if err != nil {
			return err
		}
		pvcMap[volume.PersistentVolumeClaim.ClaimName] = pvc
	}
	pvc := pvcMap[volume.PersistentVolumeClaim.ClaimName]
	if storageClass, ok := scMap[*pvc.Spec.StorageClassName]; ok {
		if err := validateCsiStorageClass(storageClass, volume); err == nil {
			return nil
		}
		deploy.DeleteCsiAzureStorageClasses(storageClass.Name)
	}
	csiVolumeSecretName := defaults.GetCsiAzureCredsSecretName(componentName, radixVolumeMount.Name)
	_, err := deploy.CreateCsiAzureStorageClasses(namespace, componentName, csiVolumeStorageClassName, radixVolumeMount.Container, csiVolumeSecretName, volume.Name)
	if err != nil {
		return err
	}
	return nil
}

func validateCsiStorageClass(storageClass *storagev1.StorageClass, volume *corev1.Volume) error {
	pvcVolumeType, err := getMandatoryLabel(storageClass.Labels, kube.RadixMountTypeLabel)
	if err != nil {
		return err
	}
	if !strings.EqualFold(storageClass.Provisioner, pvcVolumeType) {
		return fmt.Errorf("PVC volume type %s does not match to storage class provisioner %s", pvcVolumeType, volume.VolumeSource.CSI.Driver)
	}
	return nil
}

func (deploy *Deployment) getRadixVolumeMountMap(componentName string) map[string]*v1.RadixVolumeMount {
	volumeMountMap := make(map[string]*v1.RadixVolumeMount)
	for _, component := range deploy.radixDeployment.Spec.Components {
		if !strings.EqualFold(componentName, component.GetName()) {
			continue
		}
		for _, radixVolumeMount := range component.VolumeMounts {
			volumeMountMap[getCsiVolumeMountName(componentName, radixVolumeMount)] = &radixVolumeMount
		}
	}
	return volumeMountMap
}

//func garbageCollectCsiAzureStorageClass() {
//log.Debugf("GC outdated PVC %s and its Storage Class: %v", pvcName, err)
//err = deploy.garbageCollectCsiAzurePersistentVolumeClaim(namespace, componentName, pvc)
//if err != nil {
//	return err
//}
//if pvc.Spec.StorageClassName != nil {
//	delete(scNameMap, *pvc.Spec.StorageClassName)
//}
//delete(scCsiVolumeKeyMap, csiVolumeKey)
//delete(pvcCsiVolumeKeyMap, csiVolumeKey)
//delete(pvcNameMap, pvc.Name)
//}
//func (deploy *Deployment) garbageCollectCsiAzurePersistentVolumeClaim(namespace string, pvc *corev1.PersistentVolumeClaim) error {
//	if pvc.Spec.StorageClassName != nil {
//		err := deploy.DeleteCsiAzureStorageClasses(*pvc.Spec.StorageClassName)
//		if err != nil {
//			return err
//		}
//	}
//	return deploy.DeletePersistentVolumeClaim(namespace, *pvc.Spec.StorageClassName)
//}
//

func getPersistentVolumeClaimMap(pvcList *corev1.PersistentVolumeClaimList) map[string]*corev1.PersistentVolumeClaim {
	pvcMap := make(map[string]*corev1.PersistentVolumeClaim)
	for _, pvc := range pvcList.Items {
		pvcMap[pvc.Name] = &pvc
	}
	return pvcMap
}

func getStorageClassMap(scList *storagev1.StorageClassList) map[string]*storagev1.StorageClass {
	scMap := make(map[string]*storagev1.StorageClass)
	for _, sc := range scList.Items {
		scMap[sc.Name] = &sc
	}
	return scMap
}

//func getPersistentVolumeClaimMap(pvcList *corev1.PersistentVolumeClaimList) (map[string]*corev1.PersistentVolumeClaim, map[string]*corev1.PersistentVolumeClaim) {
//	pvcNameMap := make(map[string]*corev1.PersistentVolumeClaim)
//	pvcCsiVolumeKeyMap := make(map[string]*corev1.PersistentVolumeClaim)
//	for _, pvc := range pvcList.Items {
//		pvcNameMap[pvc.Name] = &pvc
//		pvcCsiVolumeKeyMap[getCsiVolumeKeyByLabels(pvc.Labels)] = &pvc
//	}
//	return pvcNameMap, pvcCsiVolumeKeyMap
//}
//
//func getStorageClassMap(scList *storagev1.StorageClassList) (map[string]*storagev1.StorageClass, map[string]*storagev1.StorageClass) {
//	scNameMap := make(map[string]*storagev1.StorageClass)
//	scCsiVolumeKeyMap := make(map[string]*storagev1.StorageClass)
//	for _, sc := range scList.Items {
//		key := getCsiVolumeKey(getLabel(sc.Labels, kube.RadixMountTypeLabel), getLabel(sc.Labels, kube.RadixVolumeMountNameLabel))
//		scNameMap[sc.Name] = &sc
//		scCsiVolumeKeyMap[key] = &sc
//	}
//	return scNameMap, scCsiVolumeKeyMap
//}

//func getCsiVolumeKeyByLabels(labels map[string]string) string {
//	return getCsiVolumeKey(getLabel(labels, kube.RadixMountTypeLabel), getLabel(labels, kube.RadixVolumeMountNameLabel))
//}
//
//func getCsiVolumeKeyByVolume(volume *corev1.Volume) string {
//	return getCsiVolumeKey(volume.CSI.Driver, volume.Name)
//}
//
//func getCsiVolumeKey(volumeType, volumeName string) string {
//	return fmt.Sprintf("%s--%s", volumeType, volumeName)
//}

//func getLabel(labels map[string]string, labelKey string) string {
//	if labelValue, err := getMandatoryLabel(labels, labelKey); err == nil {
//		return labelValue
//	}
//	return "undefined"
//}

func getMandatoryLabel(labels map[string]string, labelKey string) (string, error) {
	if labelValue, ok := labels[labelKey]; ok {
		return labelValue, nil
	}
	return "", fmt.Errorf("label not found: %s", labelKey)
}

func (deploy *Deployment) getCurrentAndDesiredDeployment(deployComponent v1.RadixCommonDeployComponent) (*appsv1.Deployment, *appsv1.Deployment, error) {
	var desiredDeployment *appsv1.Deployment
	namespace := deploy.radixDeployment.Namespace

	currentDeployment, err := deploy.kubeutil.GetDeployment(namespace, deployComponent.GetName())
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			return nil, nil, err
		}

		desiredDeployment, err = deploy.getDesiredCreatedDeploymentConfig(deployComponent)
		if err == nil {
			log.Debugf("Creating Deployment: %s in namespace %s", desiredDeployment.Name, namespace)
		}
	} else {
		desiredDeployment, err = deploy.getDesiredUpdatedDeploymentConfig(deployComponent, currentDeployment)
		if err == nil {
			log.Debugf("Deployment object %s already exists in namespace %s, updating the object now", currentDeployment.GetName(), namespace)
		}
	}

	deploy.configureDeploymentServiceAccountSettings(desiredDeployment, deployComponent)
	return currentDeployment, desiredDeployment, err
}

func (deploy *Deployment) configureDeploymentServiceAccountSettings(deployment *appsv1.Deployment, deployComponent v1.RadixCommonDeployComponent) {
	spec := NewServiceAccountSpec(deploy.radixDeployment, deployComponent)
	deployment.Spec.Template.Spec.AutomountServiceAccountToken = spec.AutomountServiceAccountToken()
	deployment.Spec.Template.Spec.ServiceAccountName = spec.ServiceAccountName()
}

func (deploy *Deployment) getDesiredCreatedDeploymentConfig(deployComponent v1.RadixCommonDeployComponent) (*appsv1.Deployment, error) {
	appName := deploy.radixDeployment.Spec.AppName
	componentName := deployComponent.GetName()
	componentType := deployComponent.GetType()
	automountServiceAccountToken := false
	branch, commitID := deploy.getRadixBranchAndCommitId()
	ownerReference := getOwnerReferenceOfDeployment(deploy.radixDeployment)
	containerSecurityContext := getSecurityContextForContainer(deployComponent.GetRunAsNonRoot())
	podSecurityContext := getSecurityContextForPod(deployComponent.GetRunAsNonRoot())
	ports := getContainerPorts(deployComponent)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: componentName,
			Labels: map[string]string{
				kube.RadixAppLabel:           appName,
				kube.RadixComponentLabel:     componentName,
				kube.RadixComponentTypeLabel: componentType,
				kube.RadixCommitLabel:        commitID,
			},
			Annotations: map[string]string{
				kube.RadixBranchAnnotation: branch,
			},
			OwnerReferences: ownerReference,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(DefaultReplicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					kube.RadixComponentLabel: componentName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						kube.RadixAppLabel:       appName,
						kube.RadixComponentLabel: componentName,
						kube.RadixCommitLabel:    commitID,
					},
					Annotations: map[string]string{
						"apparmor.security.beta.kubernetes.io/pod": "runtime/default",
						"seccomp.security.alpha.kubernetes.io/pod": "docker/default",
						kube.RadixBranchAnnotation:                 branch,
					},
				},
				Spec: corev1.PodSpec{
					SecurityContext:              podSecurityContext,
					AutomountServiceAccountToken: &automountServiceAccountToken,
					Containers: []corev1.Container{
						{
							Name:            componentName,
							Image:           deployComponent.GetImage(),
							ImagePullPolicy: corev1.PullAlways,
							SecurityContext: containerSecurityContext,
							Ports:           ports,
						},
					},
					ImagePullSecrets: deploy.radixDeployment.Spec.ImagePullSecrets,
				},
			},
		},
	}

	if len(ports) > 0 {
		log.Debugln("Set readiness Prob for ports. Amount of ports: ", len(ports))
		readinessProbe, err := getReadinessProbe(ports[0].ContainerPort)
		if err != nil {
			return nil, err
		}
		deployment.Spec.Template.Spec.Containers[0].ReadinessProbe = readinessProbe
	}

	deploymentStrategy, err := getDeploymentStrategy()
	if err != nil {
		return nil, err
	}
	deployment.Spec.Strategy = deploymentStrategy

	deploy.setCommonPodSpecProperties(&deployment.Spec.Template.Spec, deployComponent)

	return deploy.updateDeploymentByComponent(deployComponent, deployment, appName)
}

func (deploy *Deployment) getDesiredUpdatedDeploymentConfig(deployComponent v1.RadixCommonDeployComponent,
	currentDeployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	desiredDeployment := currentDeployment.DeepCopy()
	appName := deploy.radixDeployment.Spec.AppName
	log.Debugf("Get desired updated deployment config for application: %s.", appName)
	componentName := deployComponent.GetName()
	componentType := deployComponent.GetType()
	automountServiceAccountToken := false
	branch, commitID := deploy.getRadixBranchAndCommitId()
	ports := getContainerPorts(deployComponent)

	desiredDeployment.ObjectMeta.Name = componentName
	desiredDeployment.ObjectMeta.OwnerReferences = getOwnerReferenceOfDeployment(deploy.radixDeployment)
	desiredDeployment.ObjectMeta.Labels[kube.RadixAppLabel] = appName
	desiredDeployment.ObjectMeta.Labels[kube.RadixComponentLabel] = componentName
	desiredDeployment.ObjectMeta.Labels[kube.RadixComponentTypeLabel] = componentType
	desiredDeployment.ObjectMeta.Labels[kube.RadixCommitLabel] = commitID
	desiredDeployment.ObjectMeta.Annotations[kube.RadixBranchAnnotation] = branch
	desiredDeployment.Spec.Template.ObjectMeta.Labels[kube.RadixCommitLabel] = commitID
	desiredDeployment.Spec.Template.ObjectMeta.Annotations["apparmor.security.beta.kubernetes.io/pod"] = "runtime/default"
	desiredDeployment.Spec.Template.ObjectMeta.Annotations["seccomp.security.alpha.kubernetes.io/pod"] = "docker/default"
	desiredDeployment.Spec.Template.ObjectMeta.Annotations[kube.RadixBranchAnnotation] = branch
	desiredDeployment.Spec.Template.Spec.AutomountServiceAccountToken = &automountServiceAccountToken
	desiredDeployment.Spec.Template.Spec.Containers[0].Image = deployComponent.GetImage()
	desiredDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = corev1.PullAlways
	desiredDeployment.Spec.Template.Spec.Containers[0].SecurityContext = getSecurityContextForContainer(deployComponent.GetRunAsNonRoot())
	desiredDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = corev1.PullAlways
	desiredDeployment.Spec.Template.Spec.Containers[0].Ports = ports
	desiredDeployment.Spec.Template.Spec.ImagePullSecrets = deploy.radixDeployment.Spec.ImagePullSecrets
	desiredDeployment.Spec.Template.Spec.SecurityContext = getSecurityContextForPod(deployComponent.GetRunAsNonRoot())
	deploy.setCommonPodSpecProperties(&desiredDeployment.Spec.Template.Spec, deployComponent)

	if len(deployComponent.GetPorts()) > 0 {
		log.Debugf("Deployment component has %d ports.", len(deployComponent.GetPorts()))
		prob := desiredDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe
		err := getReadinessProbeSettings(prob, &(deployComponent.GetPorts()[0]))
		if err != nil {
			return nil, err
		}
	} else {
		log.Debugf("Deployment component has no ports - Readiness Probe is not set.")
	}

	err := setDeploymentStrategy(&desiredDeployment.Spec.Strategy)
	if err != nil {
		return nil, err
	}

	return deploy.updateDeploymentByComponent(deployComponent, desiredDeployment, appName)
}

func (deploy *Deployment) setCommonPodSpecProperties(podSpec *corev1.PodSpec, deployComponent v1.RadixCommonDeployComponent) {
	podSpec.Containers[0].VolumeMounts = GetRadixDeployComponentVolumeMounts(deployComponent)
	podSpec.Volumes = deploy.getVolumes(deployComponent)
	podSpec.Affinity = deploy.getPodSpecAffinity(deployComponent)
}

func (deploy *Deployment) getRadixBranchAndCommitId() (string, string) {
	const branchKey, commitIDKey = "radix-branch", "radix-commit"
	rdLabels := deploy.radixDeployment.Labels
	var branch, commitID string
	if branchVal, exists := rdLabels[branchKey]; exists {
		branch = branchVal
	}
	if commitIDVal, exists := rdLabels[commitIDKey]; exists {
		commitID = commitIDVal
	}
	return branch, commitID
}

func (deploy *Deployment) updateDeploymentByComponent(deployComponent v1.RadixCommonDeployComponent, desiredDeployment *appsv1.Deployment, appName string) (*appsv1.Deployment, error) {
	if deployComponent.IsAlwaysPullImageOnDeploy() {
		desiredDeployment.Spec.Template.Annotations[kube.RadixDeploymentNameAnnotation] = deploy.radixDeployment.Name
	}

	replicas := deployComponent.GetReplicas()
	if replicas != nil && *replicas >= 0 {
		desiredDeployment.Spec.Replicas = int32Ptr(int32(*replicas))
	} else {
		desiredDeployment.Spec.Replicas = int32Ptr(int32(DefaultReplicas))
	}

	// Override Replicas with horizontalScaling.minReplicas if exists
	horizontalScaling := deployComponent.GetHorizontalScaling()
	if replicas != nil && *replicas != 0 && horizontalScaling != nil {
		desiredDeployment.Spec.Replicas = horizontalScaling.MinReplicas
	}

	radixDeployment := deploy.radixDeployment
	environmentVariables := GetEnvironmentVariablesFrom(appName, deploy.kubeutil, radixDeployment, deployComponent)

	if environmentVariables != nil {
		desiredDeployment.Spec.Template.Spec.Containers[0].Env = environmentVariables
	}

	desiredDeployment.Spec.Template.Spec.Containers[0].Resources = utils.GetResourceRequirements(deployComponent)

	return desiredDeployment, nil
}

func getReadinessProbe(componentPort int32) (*corev1.Probe, error) {
	initialDelaySeconds, err := defaults.GetDefaultReadinessProbeInitialDelaySeconds()
	if err != nil {
		return nil, err
	}

	periodSeconds, err := defaults.GetDefaultReadinessProbePeriodSeconds()
	if err != nil {
		return nil, err
	}

	probe := corev1.Probe{
		Handler: corev1.Handler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.IntOrString{
					IntVal: componentPort,
				},
			},
		},
		InitialDelaySeconds: initialDelaySeconds,
		PeriodSeconds:       periodSeconds,
	}

	return &probe, nil
}

func getDeploymentStrategy() (appsv1.DeploymentStrategy, error) {
	rollingUpdateMaxUnavailable, err := defaults.GetDefaultRollingUpdateMaxUnavailable()
	if err != nil {
		return appsv1.DeploymentStrategy{}, err
	}

	rollingUpdateMaxSurge, err := defaults.GetDefaultRollingUpdateMaxSurge()
	if err != nil {
		return appsv1.DeploymentStrategy{}, err
	}

	deploymentStrategy := appsv1.DeploymentStrategy{
		RollingUpdate: &appsv1.RollingUpdateDeployment{
			MaxUnavailable: &intstr.IntOrString{
				Type:   intstr.String,
				StrVal: rollingUpdateMaxUnavailable,
			},
			MaxSurge: &intstr.IntOrString{
				Type:   intstr.String,
				StrVal: rollingUpdateMaxSurge,
			},
		},
	}

	return deploymentStrategy, nil
}

func (deploy *Deployment) garbageCollectDeploymentsNoLongerInSpec() error {
	deployments, err := deploy.kubeutil.ListDeployments(deploy.radixDeployment.GetNamespace())
	if err != nil {
		return err
	}

	for _, deployment := range deployments {
		componentName, ok := NewRadixComponentNameFromLabels(deployment)
		if !ok {
			continue
		}

		garbageCollect := false

		if !componentName.ExistInDeploymentSpec(deploy.radixDeployment) {
			garbageCollect = true
		} else {
			var componentType string
			commonComponent := componentName.GetCommonDeployComponent(deploy.radixDeployment)

			// If component type label is not set on the deployment, we default to "component"
			if componentType, ok = deployment.Labels[kube.RadixComponentTypeLabel]; !ok {
				componentType = defaults.RadixComponentTypeComponent
			}

			// Garbage collect if component type has changed.
			if componentType != commonComponent.GetType() {
				garbageCollect = true
			}
		}

		if garbageCollect {
			propagationPolicy := metav1.DeletePropagationForeground
			deleteOption := metav1.DeleteOptions{
				PropagationPolicy: &propagationPolicy,
			}
			err = deploy.kubeclient.AppsV1().Deployments(deploy.radixDeployment.GetNamespace()).Delete(context.TODO(), deployment.Name, deleteOption)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func getReadinessProbeSettings(probe *corev1.Probe, componentPort *v1.ComponentPort) error {
	if componentPort == nil {
		return fmt.Errorf("Null Component Port")
	}
	initialDelaySeconds, err := defaults.GetDefaultReadinessProbeInitialDelaySeconds()
	if err != nil {
		return err
	}

	periodSeconds, err := defaults.GetDefaultReadinessProbePeriodSeconds()
	if err != nil {
		return err
	}

	if probe == nil || probe.TCPSocket == nil {
		return fmt.Errorf("Null or invalid probe")
	}
	probe.TCPSocket.Port.IntVal = componentPort.Port
	probe.InitialDelaySeconds = initialDelaySeconds
	probe.PeriodSeconds = periodSeconds

	return nil
}

func setDeploymentStrategy(deploymentStrategy *appsv1.DeploymentStrategy) error {
	rollingUpdateMaxUnavailable, err := defaults.GetDefaultRollingUpdateMaxUnavailable()
	if err != nil {
		return err
	}

	rollingUpdateMaxSurge, err := defaults.GetDefaultRollingUpdateMaxSurge()
	if err != nil {
		return err
	}

	deploymentStrategy.RollingUpdate.MaxUnavailable.StrVal = rollingUpdateMaxUnavailable
	deploymentStrategy.RollingUpdate.MaxSurge.StrVal = rollingUpdateMaxSurge
	return nil
}

// Returns a security context for container. If root flag is overridden from application config, it's allowed to run as root.
func getSecurityContextForContainer(runAsNonRoot bool) *corev1.SecurityContext {
	// runAsNonRoot is false by default
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: conditionUtils.BoolPtr(ALLOW_PRIVILEGE_ESCALATION),
		Privileged:               conditionUtils.BoolPtr(PRIVILEGED_CONTAINER),
		RunAsNonRoot:             conditionUtils.BoolPtr(runAsNonRoot),
	}
}

func getContainerPorts(deployComponent v1.RadixCommonDeployComponent) []corev1.ContainerPort {
	componentPorts := deployComponent.GetPorts()
	var ports []corev1.ContainerPort
	for _, v := range componentPorts {
		containerPort := corev1.ContainerPort{
			Name:          v.Name,
			ContainerPort: int32(v.Port),
		}
		ports = append(ports, containerPort)
	}
	return ports
}

func getSecurityContextForPod(runAsNonRoot bool) *corev1.PodSecurityContext {
	// runAsNonRoot is false by default
	return &corev1.PodSecurityContext{
		RunAsNonRoot: conditionUtils.BoolPtr(runAsNonRoot),
	}
}

func int32Ptr(i int32) *int32 {
	return &i
}

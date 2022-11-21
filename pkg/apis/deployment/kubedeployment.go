package deployment

import (
	"context"
	"github.com/equinor/radix-operator/pkg/apis/securitycontext"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/numbers"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"

	log "github.com/sirupsen/logrus"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	err = deploy.createOrUpdateCsiAzureVolumeResources(desiredDeployment)
	if err != nil {
		return err
	}

	return deploy.kubeutil.ApplyDeployment(deploy.radixDeployment.Namespace, currentDeployment, desiredDeployment)
}

func (deploy *Deployment) getCurrentAndDesiredDeployment(deployComponent v1.RadixCommonDeployComponent) (*appsv1.Deployment, *appsv1.Deployment, error) {
	namespace := deploy.radixDeployment.Namespace

	currentDeployment, desiredDeployment, err := deploy.getDesiredDeployment(namespace, deployComponent)
	if err != nil {
		return nil, nil, err
	}

	deploy.configureDeploymentServiceAccountSettings(desiredDeployment, deployComponent)
	return currentDeployment, desiredDeployment, err
}

func (deploy *Deployment) getDesiredDeployment(namespace string, deployComponent v1.RadixCommonDeployComponent) (*appsv1.Deployment, *appsv1.Deployment, error) {
	currentDeployment, err := deploy.kubeutil.GetDeployment(namespace, deployComponent.GetName())

	if err == nil && currentDeployment != nil {
		desiredDeployment, err := deploy.getDesiredUpdatedDeploymentConfig(deployComponent, currentDeployment)
		if err != nil {
			return nil, nil, err
		}
		log.Debugf("Deployment object %s already exists in namespace %s, updating the object now", currentDeployment.GetName(), namespace)
		return currentDeployment, desiredDeployment, nil
	}

	if !k8sErrors.IsNotFound(err) {
		return nil, nil, err
	}

	desiredDeployment, err := deploy.getDesiredCreatedDeploymentConfig(deployComponent)
	if err != nil {
		return nil, nil, err
	}
	log.Debugf("Creating Deployment: %s in namespace %s", desiredDeployment.Name, namespace)
	return currentDeployment, desiredDeployment, nil
}

func (deploy *Deployment) configureDeploymentServiceAccountSettings(deployment *appsv1.Deployment, deployComponent v1.RadixCommonDeployComponent) {
	spec := NewServiceAccountSpec(deploy.radixDeployment, deployComponent)
	deployment.Spec.Template.Spec.AutomountServiceAccountToken = spec.AutomountServiceAccountToken()
	deployment.Spec.Template.Spec.ServiceAccountName = spec.ServiceAccountName()
}

func (deploy *Deployment) getDesiredCreatedDeploymentConfig(deployComponent v1.RadixCommonDeployComponent) (*appsv1.Deployment, error) {
	appName := deploy.radixDeployment.Spec.AppName
	componentName := deployComponent.GetName()
	log.Debugf("Get desired created deployment config for application: %s.", appName)

	desiredDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Labels: make(map[string]string), Annotations: make(map[string]string)},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(DefaultReplicas),
			Selector: &metav1.LabelSelector{MatchLabels: make(map[string]string)},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: make(map[string]string), Annotations: make(map[string]string)},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: componentName}}},
			},
		},
	}

	err := deploy.setDesiredDeploymentProperties(deployComponent, desiredDeployment, appName, componentName)
	if err != nil {
		return nil, err
	}

	deploymentStrategy, err := getDeploymentStrategy()
	if err != nil {
		return nil, err
	}
	desiredDeployment.Spec.Strategy = deploymentStrategy

	return deploy.updateDeploymentByComponent(deployComponent, desiredDeployment, appName)
}

func (deploy *Deployment) getDesiredUpdatedDeploymentConfig(deployComponent v1.RadixCommonDeployComponent, currentDeployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	appName := deploy.radixDeployment.Spec.AppName
	componentName := deployComponent.GetName()
	log.Debugf("Get desired updated deployment config for application: %s.", appName)

	desiredDeployment := currentDeployment.DeepCopy()
	err := deploy.setDesiredDeploymentProperties(deployComponent, desiredDeployment, appName, componentName)
	if err != nil {
		return nil, err
	}

	err = setDeploymentStrategy(&desiredDeployment.Spec.Strategy)
	if err != nil {
		return nil, err
	}

	return deploy.updateDeploymentByComponent(deployComponent, desiredDeployment, appName)
}

func (deploy *Deployment) setDesiredDeploymentProperties(deployComponent v1.RadixCommonDeployComponent, desiredDeployment *appsv1.Deployment, appName, componentName string) error {
	branch, commitID := deploy.getRadixBranchAndCommitId()

	desiredDeployment.ObjectMeta.Name = componentName
	desiredDeployment.ObjectMeta.OwnerReferences = []metav1.OwnerReference{getOwnerReferenceOfDeployment(deploy.radixDeployment)}
	desiredDeployment.ObjectMeta.Labels[kube.RadixAppLabel] = appName
	desiredDeployment.ObjectMeta.Labels[kube.RadixComponentLabel] = componentName
	desiredDeployment.ObjectMeta.Labels[kube.RadixComponentTypeLabel] = string(deployComponent.GetType())
	desiredDeployment.ObjectMeta.Labels[kube.RadixCommitLabel] = commitID
	desiredDeployment.ObjectMeta.Annotations[kube.RadixBranchAnnotation] = branch

	desiredDeployment.Spec.Selector.MatchLabels[kube.RadixComponentLabel] = componentName

	desiredDeployment.Spec.Template.ObjectMeta.Labels[kube.RadixAppLabel] = appName
	desiredDeployment.Spec.Template.ObjectMeta.Labels[kube.RadixComponentLabel] = componentName
	desiredDeployment.Spec.Template.ObjectMeta.Labels[kube.RadixCommitLabel] = commitID
	if deployComponent.GetType() == v1.RadixComponentTypeJobScheduler {
		desiredDeployment.Spec.Template.ObjectMeta.Labels[kube.RadixPodIsJobSchedulerLabel] = "true"
	}

	desiredDeployment.Spec.Template.ObjectMeta.Annotations["apparmor.security.beta.kubernetes.io/pod"] = "runtime/default"
	desiredDeployment.Spec.Template.ObjectMeta.Annotations[kube.RadixBranchAnnotation] = branch

	desiredDeployment.Spec.Template.Spec.AutomountServiceAccountToken = commonUtils.BoolPtr(false)
	desiredDeployment.Spec.Template.Spec.ImagePullSecrets = deploy.radixDeployment.Spec.ImagePullSecrets
	desiredDeployment.Spec.Template.Spec.SecurityContext = securitycontext.PodSecurityContext()

	desiredDeployment.Spec.Template.Spec.Containers[0].Image = deployComponent.GetImage()
	desiredDeployment.Spec.Template.Spec.Containers[0].Ports = getContainerPorts(deployComponent)
	desiredDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = corev1.PullAlways
	desiredDeployment.Spec.Template.Spec.Containers[0].SecurityContext = securitycontext.ContainerSecurityContext()

	volumeMounts, err := GetRadixDeployComponentVolumeMounts(deployComponent, deploy.radixDeployment.GetName())
	if err != nil {
		return err
	}
	desiredDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts

	volumes, err := deploy.GetVolumesForComponent(deployComponent)
	if err != nil {
		return err
	}
	desiredDeployment.Spec.Template.Spec.Volumes = volumes

	readinessProbe, err := getReadinessProbeForComponent(deployComponent)
	if err != nil {
		return err
	}
	desiredDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe = readinessProbe

	desiredDeployment.Spec.Template.Spec.Affinity = utils.GetPodSpecAffinity(deployComponent.GetNode(), appName, componentName)
	desiredDeployment.Spec.Template.Spec.Tolerations = utils.GetPodSpecTolerations(deployComponent.GetNode())

	return nil
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

	environmentVariables, err := getEnvironmentVariablesForRadixOperator(deploy.kubeutil, appName, radixDeployment, deployComponent)
	if err != nil {
		return nil, err
	}

	if environmentVariables != nil {
		desiredDeployment.Spec.Template.Spec.Containers[0].Env = environmentVariables
	}

	desiredDeployment.Spec.Template.Spec.Containers[0].Resources = utils.GetResourceRequirements(deployComponent)

	if hasRadixSecretRefs(deployComponent) {
		desiredDeployment.Spec.RevisionHistoryLimit = numbers.Int32Ptr(0)
	} else {
		desiredDeployment.Spec.RevisionHistoryLimit = nil
	}

	return desiredDeployment, nil
}

func hasRadixSecretRefs(deployComponent v1.RadixCommonDeployComponent) bool {
	return len(deployComponent.GetSecretRefs().AzureKeyVaults) > 0
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
		componentName, ok := RadixComponentNameFromComponentLabel(deployment)
		if !ok {
			continue
		}

		if deploy.isEligibleForGarbageCollectComponent(componentName, deployment) {
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

func (deploy *Deployment) isEligibleForGarbageCollectComponent(componentName RadixComponentName, deployment *appsv1.Deployment) bool {
	if !componentName.ExistInDeploymentSpec(deploy.radixDeployment) {
		return true
	}
	var componentType v1.RadixComponentType
	// If component type label is not set on the deployment, we default to "component"
	if componentTypeString, ok := deployment.Labels[kube.RadixComponentTypeLabel]; !ok {
		componentType = v1.RadixComponentTypeComponent
	} else {
		componentType = v1.RadixComponentType(componentTypeString)
	}

	commonComponent := componentName.GetCommonDeployComponent(deploy.radixDeployment)
	// Garbage collect if component type has changed.
	return componentType != commonComponent.GetType()
}

func getReadinessProbeForComponent(component v1.RadixCommonDeployComponent) (*corev1.Probe, error) {
	if len(component.GetPorts()) == 0 {
		return nil, nil
	}

	return getReadinessProbeWithDefaultsFromEnv(component.GetPorts()[0].Port)
}

func getReadinessProbeWithDefaultsFromEnv(componentPort int32) (*corev1.Probe, error) {
	initialDelaySeconds, err := defaults.GetDefaultReadinessProbeInitialDelaySeconds()
	if err != nil {
		return nil, err
	}

	periodSeconds, err := defaults.GetDefaultReadinessProbePeriodSeconds()
	if err != nil {
		return nil, err
	}

	probe := getReadinessProbe(componentPort, initialDelaySeconds, periodSeconds)
	return &probe, nil
}

func getReadinessProbe(componentPort, initialDelaySeconds, periodSeconds int32) corev1.Probe {
	return corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.IntOrString{
					IntVal: componentPort,
				},
			},
		},
		InitialDelaySeconds: initialDelaySeconds,
		PeriodSeconds:       periodSeconds,
	}
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

func int32Ptr(i int32) *int32 {
	return &i
}

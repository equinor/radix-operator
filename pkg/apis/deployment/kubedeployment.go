package deployment

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/intstr"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (deploy *Deployment) createOrUpdateDeployment(deployComponent v1.RadixDeployComponent) error {
	currentDeployment, desiredDeployment, err := deploy.getCurrentAndDesiredDeployment(&deployComponent)
	if err != nil {
		return err
	}

	// If Replicas == 0 or HorizontalScaling is nil then delete hpa if exists before updating deployment
	deployReplicas := desiredDeployment.Spec.Replicas
	if deployReplicas != nil && *deployReplicas == 0 || deployComponent.HorizontalScaling == nil {
		err = deploy.deleteHPAIfExists(deployComponent)
		if err != nil {
			return err
		}
	}

	deploy.customSecuritySettings(desiredDeployment)
	return deploy.kubeutil.ApplyDeployment(deploy.radixDeployment.Namespace, currentDeployment, desiredDeployment)
}

func (deploy *Deployment) getCurrentAndDesiredDeployment(deployComponent *v1.RadixDeployComponent) (*appsv1.Deployment, *appsv1.Deployment, error) {
	namespace := deploy.radixDeployment.Namespace
	currentDeployment, err := deploy.kubeutil.GetDeployment(namespace, deployComponent.Name)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, nil, err
		}
		desiredCreatedDeployment, err := deploy.getDesiredCreatedDeploymentConfig(deployComponent)
		if err == nil {
			log.Debugf("Creating Deployment: %s in namespace %s", desiredCreatedDeployment.Name, namespace)
		}
		return currentDeployment, desiredCreatedDeployment, err
	}

	desiredUpdatedDeployment, err := deploy.getDesiredUpdatedDeploymentConfig(deployComponent, currentDeployment)
	if err == nil {
		log.Debugf("Deployment object %s already exists in namespace %s, updating the object now", currentDeployment.GetName(), namespace)
	}
	return currentDeployment, desiredUpdatedDeployment, err
}

func (deploy *Deployment) getDesiredCreatedDeploymentConfig(deployComponent *v1.RadixDeployComponent) (*appsv1.Deployment, error) {
	appName := deploy.radixDeployment.Spec.AppName
	componentName := deployComponent.Name
	componentPorts := deployComponent.Ports
	automountServiceAccountToken := false
	branch, commitID := deploy.getRadixBranchAndCommitId()

	ownerReference := getOwnerReferenceOfDeployment(deploy.radixDeployment)
	securityContext := getSecurityContextForContainer()

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: componentName,
			Labels: map[string]string{
				kube.RadixAppLabel:       appName,
				kube.RadixComponentLabel: componentName,
				kube.RadixCommitLabel:    commitID,
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
					AutomountServiceAccountToken: &automountServiceAccountToken,
					Containers: []corev1.Container{
						{
							Name:            componentName,
							Image:           deployComponent.Image,
							ImagePullPolicy: corev1.PullAlways,
							SecurityContext: securityContext,
						},
					},
					ImagePullSecrets: deploy.radixDeployment.Spec.ImagePullSecrets,
				},
			},
		},
	}

	var ports []corev1.ContainerPort
	for _, v := range componentPorts {
		containerPort := corev1.ContainerPort{
			Name:          v.Name,
			ContainerPort: int32(v.Port),
		}
		ports = append(ports, containerPort)
	}
	deployment.Spec.Template.Spec.Containers[0].Ports = ports

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

	deployment.Spec.Template.Spec.Containers[0].VolumeMounts = deploy.getVolumeMounts(deployComponent)
	deployment.Spec.Template.Spec.Volumes = deploy.getVolumes(deployComponent)

	return deploy.updateDeploymentByComponent(deployComponent, deployment, appName, componentName)
}

func (deploy *Deployment) getDesiredUpdatedDeploymentConfig(deployComponent *v1.RadixDeployComponent,
	currentDeployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	desiredDeployment := currentDeployment.DeepCopy()
	appName := deploy.radixDeployment.Spec.AppName
	log.Debugf("Get desired updated deployment config for application: %s.", appName)
	componentName := deployComponent.Name
	automountServiceAccountToken := false
	branch, commitID := deploy.getRadixBranchAndCommitId()

	desiredDeployment.ObjectMeta.Name = componentName
	desiredDeployment.ObjectMeta.OwnerReferences = getOwnerReferenceOfDeployment(deploy.radixDeployment)
	desiredDeployment.ObjectMeta.Labels[kube.RadixCommitLabel] = commitID
	desiredDeployment.ObjectMeta.Annotations[kube.RadixBranchAnnotation] = branch
	desiredDeployment.Spec.Template.ObjectMeta.Labels[kube.RadixCommitLabel] = commitID
	desiredDeployment.Spec.Template.ObjectMeta.Annotations["apparmor.security.beta.kubernetes.io/pod"] = "runtime/default"
	desiredDeployment.Spec.Template.ObjectMeta.Annotations["seccomp.security.alpha.kubernetes.io/pod"] = "docker/default"
	desiredDeployment.Spec.Template.ObjectMeta.Annotations[kube.RadixBranchAnnotation] = branch
	desiredDeployment.Spec.Template.Spec.AutomountServiceAccountToken = &automountServiceAccountToken
	desiredDeployment.Spec.Template.Spec.Containers[0].Image = deployComponent.Image
	desiredDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = corev1.PullAlways
	desiredDeployment.Spec.Template.Spec.Containers[0].SecurityContext = getSecurityContextForContainer()
	desiredDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = corev1.PullAlways
	desiredDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = deploy.getVolumeMounts(deployComponent)
	desiredDeployment.Spec.Template.Spec.ImagePullSecrets = deploy.radixDeployment.Spec.ImagePullSecrets
	desiredDeployment.Spec.Template.Spec.Volumes = deploy.getVolumes(deployComponent)

	portMap := make(map[string]v1.ComponentPort)
	for _, port := range deployComponent.Ports {
		portMap[port.Name] = port
	}
	for _, containerPort := range desiredDeployment.Spec.Template.Spec.Containers[0].Ports {
		if componentPort, portExists := portMap[containerPort.Name]; portExists {
			containerPort.HostPort = componentPort.Port
		}
	}

	if len(deployComponent.Ports) > 0 {
		log.Debugf("Deployment component has %d ports.", len(deployComponent.Ports))
		prob := desiredDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe
		err := getReadinessProbeSettings(prob, &deployComponent.Ports[0])
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

	return deploy.updateDeploymentByComponent(deployComponent, desiredDeployment, appName, componentName)
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

func (deploy *Deployment) updateDeploymentByComponent(deployComponent *v1.RadixDeployComponent, desiredDeployment *appsv1.Deployment, appName string, componentName string) (*appsv1.Deployment, error) {
	if deployComponent.AlwaysPullImageOnDeploy {
		desiredDeployment.Spec.Template.Annotations[kube.RadixUpdateTimeAnnotation] = time.Now().Format(time.RFC3339)
	}

	replicas := deployComponent.Replicas
	if replicas != nil && *replicas >= 0 {
		desiredDeployment.Spec.Replicas = int32Ptr(int32(*replicas))
	} else {
		desiredDeployment.Spec.Replicas = int32Ptr(int32(DefaultReplicas))
	}

	// Override Replicas with horizontalScaling.minReplicas if exists
	if replicas != nil && *replicas != 0 && deployComponent.HorizontalScaling != nil {
		desiredDeployment.Spec.Replicas = deployComponent.HorizontalScaling.MinReplicas
	}

	// For backwards compatibility
	isDeployComponentPublic := deployComponent.PublicPort != "" || deployComponent.Public
	radixDeployment := deploy.radixDeployment
	environmentVariables := deploy.getEnvironmentVariables(deployComponent.EnvironmentVariables, deployComponent.Secrets,
		isDeployComponentPublic,
		deployComponent.Ports, radixDeployment.Name, radixDeployment.Namespace, radixDeployment.Spec.Environment,
		appName, componentName)

	if environmentVariables != nil {
		desiredDeployment.Spec.Template.Spec.Containers[0].Env = environmentVariables
	}

	resourceRequirements := deployComponent.GetResourceRequirements()

	if resourceRequirements != nil {
		desiredDeployment.Spec.Template.Spec.Containers[0].Resources = *resourceRequirements
	}

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

func getComponentNameFromLabels(object metav1.Object) (componentName RadixComponentName, ok bool) {
	var nameLabelValue string
	labels := object.GetLabels()

	nameLabelValue, labelOk := labels[kube.RadixComponentLabel]
	if !labelOk {
		return "", false
	}

	return RadixComponentName(nameLabelValue), true
}

func (deploy *Deployment) eligibleForGarbageCollection(object metav1.Object) bool {
	componentName, ok := getComponentNameFromLabels(object)
	if !ok {
		return false
	}

	return !componentName.ExistInDeploymentSpec(deploy.radixDeployment)
}

func (deploy *Deployment) garbageCollectDeploymentsNoLongerInSpec() error {
	deployments, err := deploy.kubeutil.ListDeployments(deploy.radixDeployment.GetNamespace())
	if err != nil {
		return err
	}

	for _, deployment := range deployments {
		if deploy.eligibleForGarbageCollection(deployment) {
			propagationPolicy := metav1.DeletePropagationForeground
			deleteOption := &metav1.DeleteOptions{
				PropagationPolicy: &propagationPolicy,
			}
			err = deploy.kubeclient.AppsV1().Deployments(deploy.radixDeployment.GetNamespace()).Delete(deployment.Name, deleteOption)
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

func getSecurityContextForContainer() *corev1.SecurityContext {
	allowPrivilegeEscalation := false
	// runAsNonRoot := true
	// runAsUser := int64(1000)

	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: &allowPrivilegeEscalation,
		// RunAsNonRoot:             &runAsNonRoot,
		// RunAsUser:                &runAsUser,
	}
}
func int32Ptr(i int32) *int32 {
	return &i
}

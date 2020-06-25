package deployment

import (
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/apimachinery/pkg/util/intstr"
)

func (deploy *Deployment) createDeployment(deployComponent v1.RadixDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace
	appName := deploy.radixDeployment.Spec.AppName

	currentDeployment, err := deploy.kubeutil.GetDeployment(namespace, deployComponent.Name)
	if err != nil && errors.IsNotFound(err) {
		createdDeployment, err := deploy.kubeclient.AppsV1().Deployments(namespace).Create(currentDeployment)
		if err != nil {
			return fmt.Errorf("Failed to create Deployment object: %v", err)
		}

		log.Debugf("Created Deployment: %s in namespace %s", createdDeployment.Name, namespace)
		return nil
	}

	desiredDeployment, err := deploy.getDesiredDeploymentConfig(deployComponent, currentDeployment)
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

	deploy.customSecuritySettings(appName, namespace, desiredDeployment)
	return deploy.kubeutil.ApplyDeployment(namespace, currentDeployment, desiredDeployment)
}

func (deploy *Deployment) getDesiredDeploymentConfig(deployComponent v1.RadixDeployComponent,
	currentDeployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	desiredDeployment := currentDeployment.DeepCopy()
	appName := deploy.radixDeployment.Spec.AppName
	environment := deploy.radixDeployment.Spec.Environment
	componentName := deployComponent.Name
	replicas := deployComponent.Replicas
	automountServiceAccountToken := false

	const branchKey, commitIDKey = "radix-branch", "radix-commit"
	rdLabels := deploy.radixDeployment.Labels
	var branch, commitID string
	if branchVal, exists := rdLabels[branchKey]; exists {
		branch = branchVal
	}
	if commitIDVal, exists := rdLabels[commitIDKey]; exists {
		commitID = commitIDVal
	}

	desiredDeployment.ObjectMeta.Name = componentName
	desiredDeployment.ObjectMeta.OwnerReferences = getOwnerReferenceOfDeployment(deploy.radixDeployment)
	desiredDeployment.ObjectMeta.Labels[kube.RadixAppLabel] = appName
	desiredDeployment.ObjectMeta.Labels[kube.RadixComponentLabel] = componentName
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
	desiredDeployment.Spec.Template.Spec.ImagePullSecrets = deploy.radixDeployment.Spec.ImagePullSecrets

	if deployComponent.AlwaysPullImageOnDeploy {
		desiredDeployment.Spec.Template.Annotations[kube.RadixUpdateTimeAnnotation] = time.Now().Format(time.RFC3339)
	}

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
		prob := desiredDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe
		err := getReadinessProbeSettings(prob, &deployComponent.Ports[0])
		if err != nil {
			return nil, err
		}
	}

	err := setDeploymentStrategy(&desiredDeployment.Spec.Strategy)
	if err != nil {
		return nil, err
	}
	if replicas != nil && *replicas >= 0 {
		desiredDeployment.Spec.Replicas = int32Ptr(int32(*replicas))
	}

	// Override Replicas with horizontalScaling.minReplicas if exists
	if replicas != nil && *replicas != 0 && deployComponent.HorizontalScaling != nil {
		desiredDeployment.Spec.Replicas = deployComponent.HorizontalScaling.MinReplicas
	}

	// For backwards compatibility
	isDeployComponentPublic := deployComponent.PublicPort != "" || deployComponent.Public
	environmentVariables := deploy.getEnvironmentVariables(deployComponent.EnvironmentVariables, deployComponent.Secrets,
		isDeployComponentPublic, deployComponent.Ports,
		deploy.radixDeployment.Name, deploy.radixDeployment.Namespace,
		environment, appName, componentName)

	if environmentVariables != nil {
		desiredDeployment.Spec.Template.Spec.Containers[0].Env = environmentVariables
	}

	resourceRequirements := deployComponent.GetResourceRequirements()

	if resourceRequirements != nil {
		desiredDeployment.Spec.Template.Spec.Containers[0].Resources = *resourceRequirements
	}

	return desiredDeployment, nil
}

func (deploy *Deployment) garbageCollectDeploymentsNoLongerInSpec() error {
	deployments, err := deploy.kubeutil.ListDeployments(deploy.radixDeployment.GetNamespace())
	if err != nil {
		return err
	}

	for _, exisitingComponent := range deployments {
		garbageCollect := true
		exisitingComponentName := exisitingComponent.ObjectMeta.Labels[kube.RadixComponentLabel]

		for _, component := range deploy.radixDeployment.Spec.Components {
			if strings.EqualFold(component.Name, exisitingComponentName) {
				garbageCollect = false
				break
			}
		}

		if garbageCollect {
			propagationPolicy := metav1.DeletePropagationForeground
			err = deploy.kubeclient.AppsV1().Deployments(deploy.radixDeployment.GetNamespace()).Delete(exisitingComponent.Name, &metav1.DeleteOptions{PropagationPolicy: &propagationPolicy})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func getReadinessProbeSettings(probe *corev1.Probe, componentPort *v1.ComponentPort) error {
	initialDelaySeconds, err := defaults.GetDefaultReadinessProbeInitialDelaySeconds()
	if err != nil {
		return err
	}

	periodSeconds, err := defaults.GetDefaultReadinessProbePeriodSeconds()
	if err != nil {
		return err
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

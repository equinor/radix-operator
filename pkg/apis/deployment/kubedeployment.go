package deployment

import (
	"context"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/securitycontext"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixannotations "github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/pkg/apis/utils/resources"
	"github.com/rs/zerolog/log"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (deploy *Deployment) createOrUpdateDeployment(ctx context.Context, deployComponent v1.RadixCommonDeployComponent) error {
	currentDeployment, desiredDeployment, err := deploy.getCurrentAndDesiredDeployment(ctx, deployComponent)
	if err != nil {
		return err
	}

	// If component is stopped or HorizontalScaling is nil then delete hpa if exists before updating deployment
	if isComponentStopped(deployComponent) || deployComponent.GetHorizontalScaling() == nil {
		err = deploy.deleteHPAIfExists(ctx, deployComponent.GetName())
		if err != nil {
			return err
		}
	}

	err = deploy.createOrUpdateCsiAzureVolumeResources(ctx, desiredDeployment)
	if err != nil {
		return err
	}
	err = deploy.handleJobAuxDeployment(deployComponent, desiredDeployment)
	if err != nil {
		return err
	}

	return deploy.kubeutil.ApplyDeployment(deploy.radixDeployment.Namespace, currentDeployment, desiredDeployment)
}

func (deploy *Deployment) handleJobAuxDeployment(deployComponent v1.RadixCommonDeployComponent, desiredDeployment *appsv1.Deployment) error {
	if !isDeployComponentJobSchedulerDeployment(deployComponent) {
		return nil
	}
	currentJobAuxDeployment, desiredJobAuxDeployment, err := deploy.createOrUpdateJobAuxDeployment(deployComponent, desiredDeployment)
	if err != nil {
		return err
	}
	return deploy.kubeutil.ApplyDeployment(deploy.radixDeployment.Namespace, currentJobAuxDeployment, desiredJobAuxDeployment)
}

func (deploy *Deployment) getCurrentAndDesiredDeployment(ctx context.Context, deployComponent v1.RadixCommonDeployComponent) (*appsv1.Deployment, *appsv1.Deployment, error) {
	namespace := deploy.radixDeployment.Namespace

	currentDeployment, desiredDeployment, err := deploy.getDesiredDeployment(ctx, namespace, deployComponent)
	if err != nil {
		return nil, nil, err
	}

	return currentDeployment, desiredDeployment, err
}

func (deploy *Deployment) getDesiredDeployment(ctx context.Context, namespace string, deployComponent v1.RadixCommonDeployComponent) (*appsv1.Deployment, *appsv1.Deployment, error) {
	currentDeployment, err := deploy.kubeutil.GetDeployment(namespace, deployComponent.GetName())

	if err == nil && currentDeployment != nil {
		desiredDeployment, err := deploy.getDesiredUpdatedDeploymentConfig(deployComponent, currentDeployment)
		if err != nil {
			return nil, nil, err
		}
		log.Ctx(ctx).Debug().Msgf("Deployment object %s already exists in namespace %s, updating the object now", currentDeployment.GetName(), namespace)
		return currentDeployment, desiredDeployment, nil
	}

	if !k8sErrors.IsNotFound(err) {
		return nil, nil, err
	}

	desiredDeployment, err := deploy.getDesiredCreatedDeploymentConfig(ctx, deployComponent)
	if err != nil {
		return nil, nil, err
	}
	log.Ctx(ctx).Debug().Msgf("Creating Deployment: %s in namespace %s", desiredDeployment.Name, namespace)
	return currentDeployment, desiredDeployment, nil
}

func (deploy *Deployment) getDesiredCreatedDeploymentConfig(ctx context.Context, deployComponent v1.RadixCommonDeployComponent) (*appsv1.Deployment, error) {
	log.Ctx(ctx).Debug().Msgf("Get desired created deployment config for application: %s.", deploy.radixDeployment.Spec.AppName)

	desiredDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Labels: make(map[string]string), Annotations: make(map[string]string)},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointers.Ptr[int32](DefaultReplicas),
			Selector: &metav1.LabelSelector{MatchLabels: make(map[string]string)},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: make(map[string]string), Annotations: make(map[string]string)},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: deployComponent.GetName()}}},
			},
		},
	}

	err := deploy.setDesiredDeploymentProperties(deployComponent, desiredDeployment)
	return desiredDeployment, err
}
func (deploy *Deployment) createJobAuxDeployment(deployComponent v1.RadixCommonDeployComponent) *appsv1.Deployment {
	jobName := deployComponent.GetName()
	jobAuxDeploymentName := getJobAuxObjectName(jobName)
	desiredDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            jobAuxDeploymentName,
			OwnerReferences: []metav1.OwnerReference{getOwnerReferenceOfDeployment(deploy.radixDeployment)},
			Labels:          make(map[string]string),
			Annotations:     make(map[string]string),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointers.Ptr[int32](1),
			Selector: &metav1.LabelSelector{MatchLabels: radixlabels.ForJobAuxObject(jobName)},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: make(map[string]string), Annotations: make(map[string]string)},
				Spec: corev1.PodSpec{Containers: []corev1.Container{
					{
						Name:      jobAuxDeploymentName,
						Resources: resources.New(resources.WithCPUMilli(1), resources.WithMemoryMega(10)),
					}},
				},
			},
		},
	}
	desiredDeployment.Spec.Template.Spec.AutomountServiceAccountToken = commonUtils.BoolPtr(false)
	desiredDeployment.Spec.Template.Spec.SecurityContext = securitycontext.Pod()

	desiredDeployment.Spec.Template.Spec.Containers[0].Image = "bitnami/bitnami-shell:latest"
	desiredDeployment.Spec.Template.Spec.Containers[0].Command = []string{"sh"}
	desiredDeployment.Spec.Template.Spec.Containers[0].Args = []string{"-c", "echo 'start'; while true; do echo $(date);sleep 3600; done; echo 'exit'"}
	desiredDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = corev1.PullIfNotPresent
	desiredDeployment.Spec.Template.Spec.Containers[0].SecurityContext = securitycontext.Container(securitycontext.WithReadOnlyRootFileSystem(pointers.Ptr(true)))

	return desiredDeployment
}

func (deploy *Deployment) getDesiredUpdatedDeploymentConfig(ctx context.Context, deployComponent v1.RadixCommonDeployComponent, currentDeployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	log.Ctx(ctx).Debug().Msgf("Get desired updated deployment config for application: %s.", deploy.radixDeployment.Spec.AppName)

	desiredDeployment := currentDeployment.DeepCopy()
	err := deploy.setDesiredDeploymentProperties(deployComponent, desiredDeployment)

	// When HPA is enabled for a component, the HPA controller will scale the Deployment up/down by changing Replicas
	// We must keep this value as long as replicas >= 0.
	// Current replicas will be 0 if the component was previously stopped (replicas set explicitly to 0)
	if hs := deployComponent.GetHorizontalScaling(); hs != nil && !isComponentStopped(deployComponent) {
		if replicas := currentDeployment.Spec.Replicas; replicas != nil && *replicas > 0 {
			desiredDeployment.Spec.Replicas = currentDeployment.Spec.Replicas
		}
	}

	return desiredDeployment, err
}

func (deploy *Deployment) getDeploymentPodLabels(deployComponent v1.RadixCommonDeployComponent) map[string]string {
	commitID := getDeployComponentCommitId(deployComponent)
	labels := radixlabels.Merge(
		radixlabels.ForApplicationName(deploy.radixDeployment.Spec.AppName),
		radixlabels.ForComponentName(deployComponent.GetName()),
		radixlabels.ForCommitId(commitID),
		radixlabels.ForPodWithRadixIdentity(deployComponent.GetIdentity()),
	)

	if isDeployComponentJobSchedulerDeployment(deployComponent) {
		labels = radixlabels.Merge(labels, radixlabels.ForPodIsJobScheduler())
	}

	return labels
}

func (deploy *Deployment) getJobAuxDeploymentPodLabels(deployComponent v1.RadixCommonDeployComponent) map[string]string {
	return radixlabels.Merge(
		radixlabels.ForApplicationName(deploy.radixDeployment.Spec.AppName),
		radixlabels.ForComponentName(deployComponent.GetName()),
		radixlabels.ForPodWithRadixIdentity(deployComponent.GetIdentity()),
		radixlabels.ForIsJobAuxObject(),
	)
}

func (deploy *Deployment) getDeploymentPodAnnotations(deployComponent v1.RadixCommonDeployComponent) map[string]string {
	branch, _ := deploy.getRadixBranchAndCommitId()
	annotations := radixannotations.ForRadixBranch(branch)

	if deployComponent.IsAlwaysPullImageOnDeploy() {
		annotations = radixlabels.Merge(annotations, radixannotations.ForRadixDeploymentName(deploy.radixDeployment.Name))
	}

	return annotations
}

func (deploy *Deployment) getDeploymentLabels(deployComponent v1.RadixCommonDeployComponent) map[string]string {
	commitID := getDeployComponentCommitId(deployComponent)
	return radixlabels.Merge(
		radixlabels.ForApplicationName(deploy.radixDeployment.Spec.AppName),
		radixlabels.ForComponentName(deployComponent.GetName()),
		radixlabels.ForComponentType(deployComponent.GetType()),
		radixlabels.ForCommitId(commitID),
	)
}

func getDeployComponentCommitId(deployComponent v1.RadixCommonDeployComponent) string {
	return deployComponent.GetEnvironmentVariables()[defaults.RadixCommitHashEnvironmentVariable]
}

func (deploy *Deployment) getJobAuxDeploymentLabels(deployComponent v1.RadixCommonDeployComponent) map[string]string {
	return radixlabels.Merge(
		radixlabels.ForApplicationName(deploy.radixDeployment.Spec.AppName),
		radixlabels.ForComponentName(deployComponent.GetName()),
		radixlabels.ForComponentType(deployComponent.GetType()),
		radixlabels.ForIsJobAuxObject(),
	)
}

func (deploy *Deployment) getDeploymentAnnotations() map[string]string {
	branch, _ := deploy.getRadixBranchAndCommitId()
	return radixannotations.ForRadixBranch(branch)
}

func (deploy *Deployment) setDesiredDeploymentProperties(deployComponent v1.RadixCommonDeployComponent, desiredDeployment *appsv1.Deployment) error {
	appName, componentName := deploy.radixDeployment.Spec.AppName, deployComponent.GetName()

	desiredDeployment.ObjectMeta.Name = deployComponent.GetName()
	desiredDeployment.ObjectMeta.OwnerReferences = []metav1.OwnerReference{getOwnerReferenceOfDeployment(deploy.radixDeployment)}
	desiredDeployment.ObjectMeta.Labels = deploy.getDeploymentLabels(deployComponent)
	desiredDeployment.ObjectMeta.Annotations = deploy.getDeploymentAnnotations()

	desiredDeployment.Spec.Selector.MatchLabels = radixlabels.ForComponentName(componentName)
	desiredDeployment.Spec.Replicas = getDesiredComponentReplicas(deployComponent)
	desiredDeployment.Spec.RevisionHistoryLimit = getRevisionHistoryLimit(deployComponent)

	deploymentStrategy, err := getDeploymentStrategy()
	if err != nil {
		return err
	}
	desiredDeployment.Spec.Strategy = deploymentStrategy

	desiredDeployment.Spec.Template.ObjectMeta.Labels = deploy.getDeploymentPodLabels(deployComponent)
	desiredDeployment.Spec.Template.ObjectMeta.Annotations = deploy.getDeploymentPodAnnotations(deployComponent)

	desiredDeployment.Spec.Template.Spec.AutomountServiceAccountToken = commonUtils.BoolPtr(false)
	desiredDeployment.Spec.Template.Spec.ImagePullSecrets = deploy.radixDeployment.Spec.ImagePullSecrets
	desiredDeployment.Spec.Template.Spec.SecurityContext = securitycontext.Pod(securitycontext.WithPodSeccompProfile(corev1.SeccompProfileTypeRuntimeDefault))

	spec := NewServiceAccountSpec(deploy.radixDeployment, deployComponent)
	desiredDeployment.Spec.Template.Spec.AutomountServiceAccountToken = spec.AutomountServiceAccountToken()
	desiredDeployment.Spec.Template.Spec.ServiceAccountName = spec.ServiceAccountName()
	desiredDeployment.Spec.Template.Spec.Affinity = utils.GetDeploymentPodSpecAffinity(deployComponent.GetNode(), appName, componentName)
	desiredDeployment.Spec.Template.Spec.Tolerations = utils.GetDeploymentPodSpecTolerations(deployComponent.GetNode())

	volumes, err := deploy.GetVolumesForComponent(deployComponent)
	if err != nil {
		return err
	}
	desiredDeployment.Spec.Template.Spec.Volumes = volumes

	containerSecurityCtx := securitycontext.Container(securitycontext.WithContainerSeccompProfileType(corev1.SeccompProfileTypeRuntimeDefault), securitycontext.WithReadOnlyRootFileSystem(deployComponent.GetReadOnlyFileSystem()))
	desiredDeployment.Spec.Template.Spec.Containers[0].Image = deployComponent.GetImage()
	desiredDeployment.Spec.Template.Spec.Containers[0].Ports = getContainerPorts(deployComponent)
	desiredDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = corev1.PullAlways
	desiredDeployment.Spec.Template.Spec.Containers[0].SecurityContext = containerSecurityCtx
	desiredDeployment.Spec.Template.Spec.Containers[0].Resources, err = utils.GetResourceRequirements(deployComponent)
	if err != nil {
		return err
	}

	volumeMounts, err := GetRadixDeployComponentVolumeMounts(deployComponent, deploy.radixDeployment.GetName())
	if err != nil {
		return err
	}
	desiredDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts

	readinessProbe, err := getReadinessProbeForComponent(deployComponent)
	if err != nil {
		return err
	}
	desiredDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe = readinessProbe

	environmentVariables, err := GetEnvironmentVariablesForRadixOperator(deploy.kubeutil, appName, deploy.radixDeployment, deployComponent)
	if err != nil {
		return err
	}
	desiredDeployment.Spec.Template.Spec.Containers[0].Env = environmentVariables

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

func isComponentStopped(deployComponent v1.RadixCommonDeployComponent) bool {
	if replicas := deployComponent.GetReplicas(); replicas != nil {
		return *replicas == 0
	}

	return false
}

func getDesiredComponentReplicas(deployComponent v1.RadixCommonDeployComponent) *int32 {
	if isComponentStopped(deployComponent) {
		return pointers.Ptr[int32](0)
	}

	componentReplicas := int32(DefaultReplicas)
	if replicas := deployComponent.GetReplicas(); replicas != nil {
		componentReplicas = int32(*replicas)
	}

	if hs := deployComponent.GetHorizontalScaling(); hs != nil {
		if hs.MinReplicas != nil && *hs.MinReplicas > componentReplicas {
			componentReplicas = *hs.MinReplicas
		}
		if hs.MaxReplicas < componentReplicas {
			componentReplicas = hs.MaxReplicas
		}
	}

	return pointers.Ptr(componentReplicas)
}

func getRevisionHistoryLimit(deployComponent v1.RadixCommonDeployComponent) *int32 {
	if len(deployComponent.GetSecretRefs().AzureKeyVaults) > 0 {
		return pointers.Ptr(int32(0))
	}
	return pointers.Ptr(int32(10))
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
		Type: appsv1.RollingUpdateDeploymentStrategyType,
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
		TimeoutSeconds:      1,
		FailureThreshold:    3,
		SuccessThreshold:    1,
	}
}

func getContainerPorts(deployComponent v1.RadixCommonDeployComponent) []corev1.ContainerPort {
	componentPorts := deployComponent.GetPorts()
	var ports []corev1.ContainerPort
	for _, v := range componentPorts {
		containerPort := corev1.ContainerPort{
			Name:          v.Name,
			ContainerPort: int32(v.Port),
			Protocol:      corev1.ProtocolTCP,
		}
		ports = append(ports, containerPort)
	}
	return ports
}

package environments

import (
	"context"

	"github.com/equinor/radix-common/utils/slice"
	deploymentModels "github.com/equinor/radix-operator/api-server/api/deployments/models"
	"github.com/equinor/radix-operator/api-server/api/kubequery"
	"github.com/equinor/radix-operator/api-server/api/models"
	"github.com/equinor/radix-operator/api-server/api/utils/event"
	"github.com/equinor/radix-operator/api-server/api/utils/predicate"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

// getComponentStateFromSpec Returns a component with the current state
func (eh EnvironmentHandler) getComponentStateFromSpec(ctx context.Context, rd *v1.RadixDeployment, component v1.RadixCommonDeployComponent, hpas []autoscalingv2.HorizontalPodAutoscaler, scaledObjects []v1alpha1.ScaledObject) (*deploymentModels.Component, error) {

	var componentPodNames []string
	var environmentVariables map[string]string
	var replicaSummaryList []deploymentModels.ReplicaSummary
	var auxResource deploymentModels.AuxiliaryResource
	var horizontalScalingSummary *deploymentModels.HorizontalScalingSummary
	deployments, err := kubequery.GetDeploymentsForEnvironment(ctx, eh.accounts.UserAccount.Client, rd.Spec.AppName, rd.Spec.Environment)
	if err != nil {
		return nil, err
	}
	pods, err := kubequery.GetPodsForEnvironmentComponents(ctx, eh.accounts.UserAccount.Client, rd.Spec.AppName, rd.Spec.Environment)
	if err != nil {
		return nil, err
	}

	status := deploymentModels.ConsistentComponent

	if rd.Status.ActiveTo.IsZero() && rd.Status.Condition == v1.DeploymentActive {
		// current active deployment - we get existing pods
		componentPods := getComponentPodsByNamespace(pods, component.GetName())
		componentPodNames = getPodNames(componentPods)
		environmentVariables = getRadixEnvironmentVariables(componentPods)
		eventList, err := kubequery.GetEventsForEnvironment(ctx, eh.accounts.UserAccount.Client, rd.Spec.AppName, rd.Spec.Environment)
		if err != nil {
			return nil, err
		}
		lastEventWarnings := event.ConvertToEventWarnings(eventList)
		replicaSummaryList = getReplicaSummaryList(componentPods, lastEventWarnings)
		auxResource = getAuxiliaryResources(pods, deployments, rd, component)

		kd, _ := slice.FindFirst(deployments, predicate.IsDeploymentForComponent(rd.Spec.AppName, component.GetName()))
		status = eh.ComponentStatuser(component, &kd, rd)
	}

	componentBuilder := deploymentModels.NewComponentBuilder()
	if jobComponent, ok := component.(*v1.RadixDeployJobComponent); ok {
		componentBuilder.WithSchedulerPort(&jobComponent.SchedulerPort)
		if jobComponent.Payload != nil {
			componentBuilder.WithScheduledJobPayloadPath(jobComponent.Payload.Path)
		}
		componentBuilder.WithNotifications(jobComponent.Notifications)
	}

	if component.GetType() == v1.RadixComponentTypeComponent {
		horizontalScalingSummary = models.GetHpaSummary(rd.Spec.AppName, component.GetName(), hpas, scaledObjects)
	}

	return componentBuilder.
		WithComponent(component).
		WithStatus(status).
		WithPodNames(componentPodNames).
		WithReplicaSummaryList(replicaSummaryList).
		WithRadixEnvironmentVariables(environmentVariables).
		WithAuxiliaryResource(auxResource).
		WithHorizontalScalingSummary(horizontalScalingSummary).
		BuildComponent()
}

func getPodNames(pods []corev1.Pod) []string {
	var names []string
	for _, pod := range pods {
		names = append(names, pod.GetName())
	}
	return names
}

func getComponentPodsByNamespace(allPods []corev1.Pod, componentName string) []corev1.Pod {
	var componentPods []corev1.Pod

	selector := getLabelSelectorForComponentPods(componentName)
	pods := slice.FindAll(allPods, func(pod corev1.Pod) bool {
		return selector.Matches(labels.Set(pod.Labels))
	})

	for _, pod := range pods {
		pod := pod

		// A previous version of the job-scheduler added the "radix-job-type" label to job pods.
		// For backward compatibility, we need to ignore these pods in the list of pods returned for a component
		if _, isScheduledJobPod := pod.GetLabels()[kube.RadixJobTypeLabel]; isScheduledJobPod {
			continue
		}

		// Ignore pods related to jobs created from RadixBatch
		if _, isRadixBatchJobPod := pod.GetLabels()[kube.RadixBatchNameLabel]; isRadixBatchJobPod {
			continue
		}

		componentPods = append(componentPods, pod)
	}

	return componentPods
}

func getLabelSelectorForComponentPods(componentName string) labels.Selector {
	componentNameRequirement, _ := labels.NewRequirement(kube.RadixComponentLabel, selection.Equals, []string{componentName})
	notJobAuxRequirement, _ := labels.NewRequirement(kube.RadixPodIsJobAuxObjectLabel, selection.DoesNotExist, []string{})
	return labels.NewSelector().Add(*componentNameRequirement, *notJobAuxRequirement)
}

func getRadixEnvironmentVariables(pods []corev1.Pod) map[string]string {
	radixEnvironmentVariables := make(map[string]string)

	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			for _, envVariable := range container.Env {
				if crdUtils.IsRadixEnvVar(envVariable.Name) {
					radixEnvironmentVariables[envVariable.Name] = envVariable.Value
				}
			}
		}
	}

	return radixEnvironmentVariables
}

func getReplicaSummaryList(pods []corev1.Pod, lastEventWarnings event.LastEventWarnings) []deploymentModels.ReplicaSummary {
	return slice.Map(pods, func(pod corev1.Pod) deploymentModels.ReplicaSummary {
		return deploymentModels.GetReplicaSummary(pod, lastEventWarnings[pod.GetName()])
	})
}

func getAuxiliaryResources(podList []corev1.Pod, deploymentList []appsv1.Deployment, deployment *v1.RadixDeployment, component v1.RadixCommonDeployComponent) deploymentModels.AuxiliaryResource {
	var auxResource deploymentModels.AuxiliaryResource
	if auth := component.GetAuthentication(); component.IsPublic() && auth != nil {
		auxResource.OAuth2 = getOAuth2AuxiliaryResource(podList, deploymentList, deployment, component, auth.OAuth2)
	}
	return auxResource
}

func getOAuth2AuxiliaryResource(podList []corev1.Pod, deploymentList []appsv1.Deployment, deployment *v1.RadixDeployment, component v1.RadixCommonDeployComponent, oauth2 *v1.OAuth2) *deploymentModels.OAuth2AuxiliaryResource {
	if oauth2 == nil {
		return nil
	}
	oauthProxyDeployment := getAuxiliaryResourceDeployment(podList, deploymentList, deployment, component, v1.OAuthProxyAuxiliaryComponentType)
	auxiliaryResource := deploymentModels.OAuth2AuxiliaryResource{
		Deployment:  oauthProxyDeployment, // for backward compatibility
		Deployments: []deploymentModels.AuxiliaryResourceDeployment{oauthProxyDeployment},
	}
	if oauth2.IsSessionStoreTypeSystemManaged() {
		auxiliaryResource.Deployments = append(auxiliaryResource.Deployments, getAuxiliaryResourceDeployment(podList, deploymentList, deployment, component, v1.OAuthRedisAuxiliaryComponentType))
	}
	return &auxiliaryResource
}

func getAuxiliaryResourceDeployment(podList []corev1.Pod, deploymentList []appsv1.Deployment, rd *v1.RadixDeployment, component v1.RadixCommonDeployComponent, auxType string) deploymentModels.AuxiliaryResourceDeployment {
	auxResourceDeployment := deploymentModels.AuxiliaryResourceDeployment{
		Type: auxType,
	}

	kd, ok := slice.FindFirst(deploymentList, predicate.IsDeploymentForAuxComponent(rd.Spec.AppName, component.GetName(), auxType))
	if !ok {
		auxResourceDeployment.Status = deploymentModels.ComponentReconciling.String()
		return auxResourceDeployment
	}

	pods := slice.FindAll(podList, predicate.IsPodForAuxComponent(rd.Spec.AppName, rd.Spec.Environment, auxType))

	auxResourceDeployment.ReplicaList = getReplicaSummaryList(pods, nil)
	auxResourceDeployment.Status = deploymentModels.ComponentStatusFromDeployment(component, &kd, rd).String()
	return auxResourceDeployment
}

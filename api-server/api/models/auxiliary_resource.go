package models

import (
	"github.com/equinor/radix-common/utils/slice"
	deploymentModels "github.com/equinor/radix-operator/api-server/api/deployments/models"
	"github.com/equinor/radix-operator/api-server/api/utils/predicate"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func getAuxiliaryResources(rd *radixv1.RadixDeployment, component radixv1.RadixCommonDeployComponent, deploymentList []appsv1.Deployment, podList []corev1.Pod, eventWarnings map[string]string) deploymentModels.AuxiliaryResource {
	var auxResource deploymentModels.AuxiliaryResource
	if component.IsPublic() && component.GetAuthentication().GetOAuth2() != nil {
		auxResource.OAuth2 = getOAuth2AuxiliaryResource(rd, component, deploymentList, podList, eventWarnings)
	}
	return auxResource
}

func getOAuth2AuxiliaryResource(rd *radixv1.RadixDeployment, component radixv1.RadixCommonDeployComponent, deploymentList []appsv1.Deployment, podList []corev1.Pod, eventWarnings map[string]string) *deploymentModels.OAuth2AuxiliaryResource {
	auxiliaryResource := deploymentModels.OAuth2AuxiliaryResource{}
	oauthProxyDeployment := getAuxiliaryResourceDeployment(rd, component, radixv1.OAuthProxyAuxiliaryComponentType, deploymentList, podList, eventWarnings)
	auxiliaryResource.Deployment = oauthProxyDeployment // for backward compatibility
	auxiliaryResource.Deployments = append(auxiliaryResource.Deployments, oauthProxyDeployment)
	if component.GetAuthentication().GetOAuth2().IsSessionStoreTypeSystemManaged() {
		oauthRedisDeployment := getAuxiliaryResourceDeployment(rd, component, radixv1.OAuthRedisAuxiliaryComponentType, deploymentList, podList, eventWarnings)
		auxiliaryResource.Deployments = append(auxiliaryResource.Deployments, oauthRedisDeployment)
	}

	oauth2 := component.GetAuthentication().GetOAuth2()
	if oauth2.GetUseAzureIdentity() {
		auxiliaryResource.Identity = &deploymentModels.Identity{
			Azure: &deploymentModels.AzureIdentity{
				ClientId:           oauth2.ClientID,
				ServiceAccountName: oauth2.GetServiceAccountName(component.GetName()),
			},
		}
	}
	auxiliaryResource.SessionStoreType = string(oauth2.SessionStoreType)
	return &auxiliaryResource
}

func getAuxiliaryResourceDeployment(rd *radixv1.RadixDeployment, component radixv1.RadixCommonDeployComponent, auxType string, deploymentList []appsv1.Deployment, podList []corev1.Pod, eventWarnings map[string]string) deploymentModels.AuxiliaryResourceDeployment {
	var auxResourceDeployment deploymentModels.AuxiliaryResourceDeployment
	deployment, exists := slice.FindFirst(deploymentList, predicate.IsDeploymentForAuxComponent(rd.Spec.AppName, component.GetName(), auxType))
	if !exists {
		auxResourceDeployment.Status = deploymentModels.ComponentReconciling.String()
		return auxResourceDeployment
	}
	auxPods := slice.FindAll(podList, predicate.IsPodForAuxComponent(rd.Spec.AppName, component.GetName(), auxType))
	auxResourceDeployment.ReplicaList = BuildReplicaSummaryList(auxPods, eventWarnings)
	auxResourceDeployment.Status = deploymentModels.ComponentStatusFromDeployment(component, &deployment, rd).String()
	auxResourceDeployment.Type = auxType
	return auxResourceDeployment
}

package deployments

import (
	"context"
	"strings"

	deploymentModels "github.com/equinor/radix-operator/api-server/api/deployments/models"
	"github.com/equinor/radix-operator/api-server/api/kubequery"
	"github.com/equinor/radix-operator/api-server/api/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

// GetComponentsForDeployment Gets a list of components for a given deployment
func (deploy *deployHandler) GetComponentsForDeployment(ctx context.Context, appName, deploymentName, envName string) ([]*deploymentModels.Component, error) {
	rd, err := kubequery.GetRadixDeploymentByName(ctx, deploy.accounts.UserAccount.RadixClient, appName, envName, deploymentName)
	if err != nil {
		return nil, err
	}
	ra, err := kubequery.GetRadixApplication(ctx, deploy.accounts.UserAccount.RadixClient, appName)
	if err != nil {
		return nil, err
	}
	deploymentList, err := kubequery.GetDeploymentsForEnvironment(ctx, deploy.accounts.UserAccount.Client, appName, envName)
	if err != nil {
		return nil, err
	}
	podList, err := kubequery.GetPodsForEnvironmentComponents(ctx, deploy.accounts.UserAccount.Client, appName, envName)
	if err != nil {
		return nil, err
	}
	hpas, err := kubequery.GetHorizontalPodAutoscalersForEnvironment(ctx, deploy.accounts.UserAccount.Client, appName, envName)
	if err != nil {
		return nil, err
	}
	scaledObjects, err := kubequery.GetScaledObjectsForEnvironment(ctx, deploy.accounts.UserAccount.KedaClient, appName, envName)
	if err != nil {
		return nil, err
	}
	noJobPayloadReq, err := labels.NewRequirement(kube.RadixSecretTypeLabel, selection.NotEquals, []string{string(kube.RadixSecretJobPayload)})
	if err != nil {
		return nil, err
	}
	secretList, err := kubequery.GetSecretsForEnvironment(ctx, deploy.accounts.ServiceAccount.Client, appName, envName, *noJobPayloadReq)
	if err != nil {
		return nil, err
	}
	eventList, err := kubequery.GetEventsForEnvironment(ctx, deploy.accounts.UserAccount.Client, appName, envName)
	if err != nil {
		return nil, err
	}
	certs, err := kubequery.GetCertificatesForEnvironment(ctx, deploy.accounts.ServiceAccount.CertManagerClient, appName, envName)
	if err != nil {
		return nil, err
	}
	certRequests, err := kubequery.GetCertificateRequestsForEnvironment(ctx, deploy.accounts.ServiceAccount.CertManagerClient, appName, envName)
	if err != nil {
		return nil, err
	}

	return models.BuildComponents(ra, rd, deploymentList, podList, hpas, secretList, eventList, certs, certRequests, nil, scaledObjects), nil
}

// GetComponentsForDeploymentName handler for GetDeployments
func (deploy *deployHandler) GetComponentsForDeploymentName(ctx context.Context, appName, deploymentName string) ([]*deploymentModels.Component, error) {
	deployments, err := deploy.GetDeploymentsForApplication(ctx, appName)
	if err != nil {
		return nil, err
	}

	for _, depl := range deployments {
		if strings.EqualFold(depl.Name, deploymentName) {
			return deploy.GetComponentsForDeployment(ctx, appName, depl.Name, depl.Environment)
		}
	}

	return nil, deploymentModels.NonExistingDeployment(nil, deploymentName)
}

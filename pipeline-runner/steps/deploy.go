package steps

import (
	"fmt"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
)

// Deploy Handles deploy step of the pipeline
func (cli *RadixStepHandler) Deploy(pipelineInfo model.PipelineInfo) ([]v1.RadixDeployment, error) {
	appName := pipelineInfo.RadixRegistration.Name
	containerRegistry, err := cli.kubeutil.GetContainerRegistry()
	if err != nil {
		return nil, err
	}

	log.Infof("Deploying app %s", appName)

	radixDeployments, err := deployment.ConstructForTargetEnvironments(
		pipelineInfo.RadixApplication,
		containerRegistry,
		pipelineInfo.JobName,
		pipelineInfo.ImageTag,
		pipelineInfo.Branch,
		pipelineInfo.CommitID,
		pipelineInfo.TargetEnvironments)
	if err != nil {
		return nil, fmt.Errorf("Failed to create radix deployments objects for app %s. %v", appName, err)
	}

	for _, radixDeployment := range radixDeployments {
		deployment, err := deployment.NewDeployment(cli.kubeclient, cli.radixclient, cli.prometheusOperatorClient, pipelineInfo.RadixRegistration, &radixDeployment)
		if err != nil {
			return nil, err
		}

		err = deployment.Apply()
		if err != nil {
			return nil, fmt.Errorf("Failed to apply radix deployments for app %s. %v", appName, err)
		}
	}

	log.Infof("App deployed %s", appName)
	return radixDeployments, nil
}

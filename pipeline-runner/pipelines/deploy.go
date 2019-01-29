package onpush

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// Deploy Handles deploy step of the pipeline
func (cli *RadixOnPushHandler) Deploy(jobName string, radixRegistration *v1.RadixRegistration, radixApplication *v1.RadixApplication, imageTag, branch, commitID string, targetEnvs map[string]bool) ([]v1.RadixDeployment, error) {
	appName := radixRegistration.Name
	containerRegistry, err := cli.kubeutil.GetContainerRegistry()
	if err != nil {
		return nil, err
	}

	log.Infof("Deploying app %s", appName)

	radixDeployments, err := deployment.ConstructForTargetEnvironments(radixApplication, containerRegistry, jobName, imageTag, branch, commitID, targetEnvs)
	if err != nil {
		return nil, fmt.Errorf("Failed to create radix deployments objects for app %s. %v", appName, err)
	}

	for _, radixDeployment := range radixDeployments {
		deployment, err := deployment.NewDeployment(cli.kubeclient, cli.radixclient, cli.prometheusOperatorClient, radixRegistration, &radixDeployment)
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

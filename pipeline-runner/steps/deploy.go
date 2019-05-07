package steps

import (
	"fmt"

	"github.com/coreos/prometheus-operator/pkg/client/monitoring"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

type DeployHandler struct {
	kubeclient               kubernetes.Interface
	radixclient              radixclient.Interface
	prometheusOperatorClient monitoring.Interface
	kubeutil                 *kube.Kube
}

// Init constructor
func InitDeployHandler(kubeclient kubernetes.Interface, radixclient radixclient.Interface, prometheusOperatorClient monitoring.Interface) DeployHandler {
	kube, _ := kube.New(kubeclient)
	return DeployHandler{
		kubeclient:               kubeclient,
		radixclient:              radixclient,
		prometheusOperatorClient: prometheusOperatorClient,
		kubeutil:                 kube,
	}
}

func (cli DeployHandler) SucceededMsg(pipelineInfo model.PipelineInfo) string {
	return fmt.Sprintf("Succeded: deploy application %s", pipelineInfo.GetAppName())
}

func (cli DeployHandler) ErrorMsg(pipelineInfo model.PipelineInfo, err error) string {
	return fmt.Sprintf("Failed to deploy application %s. Error: %v", pipelineInfo.GetAppName(), err)
}

func (cli DeployHandler) Run(pipelineInfo model.PipelineInfo) error {
	_, err := cli.Deploy(pipelineInfo)
	return err
}

// Deploy Handles deploy step of the pipeline
func (cli *DeployHandler) Deploy(pipelineInfo model.PipelineInfo) ([]v1.RadixDeployment, error) {
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
		deployment, err := deployment.NewDeployment(
			cli.kubeclient,
			cli.radixclient,
			cli.prometheusOperatorClient,
			pipelineInfo.RadixRegistration,
			&radixDeployment)
		if err != nil {
			return nil, err
		}

		err = deployment.Apply()
		if err != nil {
			return nil, fmt.Errorf("Failed to apply radix deployments for app %s. %v", appName, err)
		}
	}

	return radixDeployments, nil
}

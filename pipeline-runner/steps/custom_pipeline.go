package steps

import (
	"fmt"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	//"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	//tektonClient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
)

// CustomPipelineStepImplementation Step to run custom pipeline
type CustomPipelineStepImplementation struct {
	stepType pipeline.StepType
	model.DefaultStepImplementation
}

// NewCustomPipelineStep Constructor
func NewCustomPipelineStep() model.Step {
	return &CustomPipelineStepImplementation{
		stepType: pipeline.CustomPipelineStep,
	}
}

// ImplementationForType Override of default step method
func (cli *CustomPipelineStepImplementation) ImplementationForType() pipeline.StepType {
	return cli.stepType
}

// SucceededMsg Override of default step method
func (cli *CustomPipelineStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Succeded: custom pipeline step for application %s", cli.GetAppName())
}

// ErrorMsg Override of default step method
func (cli *CustomPipelineStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed to custom pipeline application %s. Error: %v", cli.GetAppName(), err)
}

// Run Override of default step method
func (cli *CustomPipelineStepImplementation) Run(pipelineInfo *model.PipelineInfo) error {
	branch := pipelineInfo.PipelineArguments.Branch
	commitID := pipelineInfo.PipelineArguments.CommitID

	log.Infof("Run custom pipeline app %s for branch %s and commit %s", cli.GetAppName(), branch, commitID)

	namespace := utils.GetAppNamespace(cli.GetAppName())
	buildSecrets, err := getBuildSecretsAsVariables(cli.GetKubeclient(), pipelineInfo.RadixApplication, namespace)
	if err != nil {
		return err
	}

	job, err := cli.runCustomPipeline(cli.GetRegistration(), pipelineInfo, buildSecrets)
	if err != nil {
		return err
	}

	return cli.GetKubeutil().WaitForCompletionOf(job)
}

func (cli *CustomPipelineStepImplementation) runCustomPipeline(ra *v1.RadixRegistration,
	pipelineInfo *model.PipelineInfo, buildSecrets []corev1.EnvVar) (*batchv1.Job, error) {
	//pipeline := v1alpha1.Pipeline{}
	cli.GetRadixclient()
	return nil, nil
}

package steps

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	jobUtil "github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildStepImplementation Step to build docker image
type BuildStepImplementation struct {
	stepType pipeline.StepType
	model.DefaultStepImplementation
}

// NewBuildStep Constructor
func NewBuildStep() model.Step {
	return &BuildStepImplementation{
		stepType: pipeline.BuildStep,
	}
}

// ImplementationForType Override of default step method
func (cli *BuildStepImplementation) ImplementationForType() pipeline.StepType {
	return cli.stepType
}

// SucceededMsg Override of default step method
func (cli *BuildStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Succeded: build step for application %s", cli.GetAppName())
}

// ErrorMsg Override of default step method
func (cli *BuildStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed to build application %s. Error: %v", cli.GetAppName(), err)
}

// Run Override of default step method
func (cli *BuildStepImplementation) Run(pipelineInfo *model.PipelineInfo) error {
	branch := pipelineInfo.PipelineArguments.Branch
	commitID := pipelineInfo.GitCommitHash

	if !pipelineInfo.BranchIsMapped {
		log.Infof("Skip build step as branch %s is not mapped to any environment", pipelineInfo.PipelineArguments.Branch)
		return nil
	}

	if !needToBuildComponents(pipelineInfo.ComponentImages) {
		log.Infof("No component in app %s requires building", cli.GetAppName())
		return nil
	}

	log.Infof("Building app %s for branch %s and commit %s", cli.GetAppName(), branch, commitID)

	namespace := utils.GetAppNamespace(cli.GetAppName())
	buildSecrets, err := getBuildSecretsAsVariables(cli.GetKubeclient(), pipelineInfo.RadixApplication, namespace)
	if err != nil {
		return err
	}

	job, err := createACRBuildJob(cli.GetRegistration(), pipelineInfo, buildSecrets)
	if err != nil {
		return err
	}

	// When debugging pipeline there will be no RJ
	if !pipelineInfo.PipelineArguments.Debug {
		ownerReference, err := jobUtil.GetOwnerReferenceOfJob(cli.GetRadixclient(), namespace, pipelineInfo.PipelineArguments.JobName)
		if err != nil {
			return err
		}

		job.OwnerReferences = ownerReference
	}

	log.Infof("Apply job (%s) to build components for app %s", job.Name, cli.GetAppName())
	job, err = cli.GetKubeclient().BatchV1().Jobs(namespace).Create(context.TODO(), job, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return cli.GetKubeutil().WaitForCompletionOf(job)
}

func needToBuildComponents(componentImages map[string]pipeline.ComponentImage) bool {
	for _, componentImage := range componentImages {
		if componentImage.Build {
			return true
		}
	}
	return false
}

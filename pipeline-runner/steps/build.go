package steps

import (
	"context"
	"fmt"

	internalwait "github.com/equinor/radix-operator/pipeline-runner/internal/wait"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	jobUtil "github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// BuildStepImplementation Step to build docker image
type BuildStepImplementation struct {
	stepType pipeline.StepType
	model.DefaultStepImplementation
	jobWaiter internalwait.JobCompletionWaiter
}

// NewBuildStep Constructor.
// jobWaiter is optional and will be set by Init(...) function if nil.
func NewBuildStep(jobWaiter internalwait.JobCompletionWaiter) model.Step {
	step := &BuildStepImplementation{
		stepType:  pipeline.BuildStep,
		jobWaiter: jobWaiter,
	}

	return step
}

func (step *BuildStepImplementation) Init(kubeclient kubernetes.Interface, radixclient radixclient.Interface, kubeutil *kube.Kube, prometheusOperatorClient monitoring.Interface, rr *v1.RadixRegistration) {
	step.DefaultStepImplementation.Init(kubeclient, radixclient, kubeutil, prometheusOperatorClient, rr)
	if step.jobWaiter == nil {
		step.jobWaiter = internalwait.NewJobCompletionWaiter(kubeclient)
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

	if len(pipelineInfo.BuildComponentImages) == 0 {
		log.Infof("No component in app %s requires building", cli.GetAppName())
		return nil
	}

	log.Infof("Building app %s for branch %s and commit %s", cli.GetAppName(), branch, commitID)

	namespace := utils.GetAppNamespace(cli.GetAppName())
	buildSecrets, err := getBuildSecretsAsVariables(pipelineInfo)
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

	return cli.jobWaiter.Wait(job)
}

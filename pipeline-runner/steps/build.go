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
	"golang.org/x/sync/errgroup"
	batchv1 "k8s.io/api/batch/v1"
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
func (step *BuildStepImplementation) ImplementationForType() pipeline.StepType {
	return step.stepType
}

// SucceededMsg Override of default step method
func (step *BuildStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Succeded: build step for application %s", step.GetAppName())
}

// ErrorMsg Override of default step method
func (step *BuildStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed to build application %s. Error: %v", step.GetAppName(), err)
}

// Run Override of default step method
func (step *BuildStepImplementation) Run(pipelineInfo *model.PipelineInfo) error {
	branch := pipelineInfo.PipelineArguments.Branch
	commitID := pipelineInfo.GitCommitHash

	if len(pipelineInfo.TargetEnvironments) == 0 {
		log.Infof("Skip build step as branch %s is not mapped to any environment", pipelineInfo.PipelineArguments.Branch)
		return nil
	}

	if len(pipelineInfo.BuildComponentImages) == 0 {
		log.Infof("No component in app %s requires building", step.GetAppName())
		return nil
	}

	log.Infof("Building app %s for branch %s and commit %s", step.GetAppName(), branch, commitID)

	namespace := utils.GetAppNamespace(step.GetAppName())
	buildSecrets, err := getBuildSecretsAsVariables(pipelineInfo)
	if err != nil {
		return err
	}

	jobs, err := step.buildACRBuildJobs(pipelineInfo, buildSecrets)
	if err != nil {
		return err
	}

	return step.createACRBuildJobs(pipelineInfo, jobs, namespace, err)
}

func (step *BuildStepImplementation) createACRBuildJobs(pipelineInfo *model.PipelineInfo, jobs []*batchv1.Job, namespace string, err error) error {
	var ownerReference []metav1.OwnerReference
	// When debugging pipeline there will be no RJ
	if !pipelineInfo.PipelineArguments.Debug {
		ownerReference, err = jobUtil.GetOwnerReferenceOfJob(step.GetRadixclient(), namespace, pipelineInfo.PipelineArguments.JobName)
		if err != nil {
			return err
		}
	}

	g := errgroup.Group{}
	for _, job := range jobs {
		g.Go(func() error {
			job.OwnerReferences = ownerReference
			log.Infof("Apply job %s to build components for app %s", job.Name, step.GetAppName())
			job, err = step.GetKubeclient().BatchV1().Jobs(namespace).Create(context.Background(), job, metav1.CreateOptions{})
			if err != nil {
				return err
			}
			return step.jobWaiter.Wait(job)
		})
	}
	return g.Wait()
}

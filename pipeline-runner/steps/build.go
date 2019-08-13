package steps

import (
	"errors"
	"fmt"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
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
	return fmt.Sprintf("Succeded: build docker image for application %s", cli.GetAppName())
}

// ErrorMsg Override of default step method
func (cli *BuildStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed to build application %s. Error: %v", cli.GetAppName(), err)
}

// Run Override of default step method
func (cli *BuildStepImplementation) Run(pipelineInfo *model.PipelineInfo) error {
	branch := pipelineInfo.PipelineArguments.Branch
	commitID := pipelineInfo.PipelineArguments.CommitID
	log.Infof("Building app %s for branch %s and commit %s", cli.GetAppName(), branch, commitID)

	if !pipelineInfo.BranchIsMapped {
		// Do nothing
		return fmt.Errorf("Skip build step as branch %s is not mapped to any environment", pipelineInfo.PipelineArguments.Branch)
	}

	namespace := utils.GetAppNamespace(cli.GetAppName())
	containerRegistry, err := cli.GetKubeutil().GetContainerRegistry()
	if err != nil {
		return err
	}

	// TODO - what about build secrets, e.g. credentials for private npm repository?
	job, err := createACRBuildXJob(cli.GetRegistration(), cli.GetApplicationConfig(), containerRegistry, pipelineInfo)
	if err != nil {
		return err
	}

	log.Infof("Apply job (%s) to build components for app %s", job.Name, cli.GetAppName())
	job, err = cli.GetKubeclient().BatchV1().Jobs(namespace).Create(job)
	if err != nil {
		return err
	}

	return cli.watchJob(job)
}

func (cli *BuildStepImplementation) watchJob(job *batchv1.Job) error {
	errChan := make(chan error)
	stop := make(chan struct{})
	defer close(stop)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(
		cli.GetKubeclient(), 0, kubeinformers.WithNamespace(job.GetNamespace()))
	informer := kubeInformerFactory.Batch().V1().Jobs().Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, new interface{}) {
			j, success := new.(*batchv1.Job)
			if success && job.GetName() == j.GetName() && job.GetNamespace() == j.GetNamespace() {
				switch {
				case j.Status.Succeeded == 1:
					errChan <- nil
				case j.Status.Failed == 1:
					errChan <- fmt.Errorf("Build docker image failed. See build log")
				default:
					log.Debugf("Ongoing - build docker image")
				}
			}
		},
		DeleteFunc: func(old interface{}) {
			j, success := old.(*batchv1.Job)
			if success && j.GetName() == job.GetName() && job.GetNamespace() == j.GetNamespace() {
				errChan <- errors.New("Build failed - Job deleted")
			}
		},
	})

	go informer.Run(stop)
	if !cache.WaitForCacheSync(stop, informer.HasSynced) {
		errChan <- fmt.Errorf("Timed out waiting for caches to sync")
	}

	err := <-errChan
	return err
}

			Annotations: map[string]string{
				kube.RadixBranchAnnotation: branch,
			},
		if c.Image != "" {
			// Using public image. Nothing to build
			continue
		}

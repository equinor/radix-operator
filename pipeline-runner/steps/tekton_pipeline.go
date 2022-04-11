package steps

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	tektonInformerFactory "github.com/tektoncd/pipeline/pkg/client/informers/externalversions"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	knativeApis "knative.dev/pkg/apis"
	knative "knative.dev/pkg/apis/duck/v1beta1"
)

// TektonPipelineStepImplementation Step to run custom pipeline
type TektonPipelineStepImplementation struct {
	stepType pipeline.StepType
	model.DefaultStepImplementation
}

// NewTektonPipelineStep Constructor
func NewTektonPipelineStep() model.Step {
	return &TektonPipelineStepImplementation{
		stepType: pipeline.RunTektonPipelineStep,
	}
}

// ImplementationForType Override of default step method
func (cli *TektonPipelineStepImplementation) ImplementationForType() pipeline.StepType {
	return cli.stepType
}

// SucceededMsg Override of default step method
func (cli *TektonPipelineStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Succeded: tekton pipeline step for application %s", cli.GetAppName())
}

// ErrorMsg Override of default step method
func (cli *TektonPipelineStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed tekton pipeline for the application %s. Error: %v", cli.GetAppName(), err)
}

// Run Override of default step method
func (cli *TektonPipelineStepImplementation) Run(pipelineInfo *model.PipelineInfo) error {
	branch := pipelineInfo.PipelineArguments.Branch
	commitID := pipelineInfo.PipelineArguments.CommitID

	log.Infof("Run tekton pipeline app %s for branch %s and commit %s", cli.GetAppName(), branch, commitID)

	//namespace := utils.GetAppNamespace(cli.GetAppName())
	//buildSecrets, err := getBuildSecretsAsVariables(cli.GetKubeclient(), pipelineInfo.RadixApplication, namespace)
	//if err != nil {
	//    return err
	//}

	job, err := cli.runTektonPipeline(cli.GetRegistration(), pipelineInfo, nil)
	if err != nil {
		return err
	}

	return cli.GetKubeutil().WaitForCompletionOf(job)
}

func (cli *TektonPipelineStepImplementation) runTektonPipeline(rr *v1.RadixRegistration,
	pipelineInfo *model.PipelineInfo, buildSecrets []corev1.EnvVar) (*batchv1.Job, error) {
	namespace := utils.GetAppNamespace(cli.GetAppName())
	appName := rr.Name
	ra := pipelineInfo.RadixApplication
	if ra == nil {
		return nil, fmt.Errorf("no RadixApplication")
	}
	timestamp := time.Now().Format("20060102150405")
	imageTag := pipelineInfo.PipelineArguments.ImageTag
	branch := pipelineInfo.PipelineArguments.Branch
	radixPipelineJobName := pipelineInfo.PipelineArguments.JobName
	hash := strings.ToLower(utils.RandStringStrSeed(5, radixPipelineJobName))
	task, err := cli.createTask("task1", timestamp, imageTag, hash, namespace)
	if err != nil {
		return nil, err
	}
	log.Debugf("created task %s", task.Name)
	pipeline, err := cli.createPipeline(namespace, timestamp, imageTag, hash, radixPipelineJobName, appName, branch,
		&task)
	if err != nil {
		return nil, err
	}
	log.Debugf("created pipeline %s with %d tasks", pipeline.Name, len(pipeline.Spec.Tasks))
	pipelineRun, err := cli.createPipelineRun(namespace, timestamp, imageTag, hash, radixPipelineJobName, appName,
		branch, pipeline)
	if err != nil {
		return nil, err
	}
	log.Debugf("created pipelineRun %s", pipelineRun.Name)
	err = cli.WaitForCompletionOf(pipelineRun)
	if err != nil {
		return nil, fmt.Errorf("failed tekton pipeline, %v, for app %s. %v",
			pipelineInfo.TargetEnvironments, appName,
			err)
	}

	return nil, nil
}

func (cli *TektonPipelineStepImplementation) createPipelineRun(namespace, timestamp, imageTag, hash,
	radixPipelineJobName, appName, branch string, pipeline *v1beta1.Pipeline) (*v1beta1.PipelineRun, error) {
	pipelineRunName := fmt.Sprintf("tekton-pipeline-run-%s-%s-%s", timestamp, imageTag, hash)
	pipelineRun := v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:   pipelineRunName,
			Labels: getLabels(radixPipelineJobName, appName, imageTag, hash),
			Annotations: map[string]string{
				kube.RadixBranchAnnotation: branch,
			},
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				Name: pipeline.GetName(),
			},
		},
	}
	return cli.GetTektonclient().TektonV1beta1().PipelineRuns(namespace).Create(context.Background(), &pipelineRun,
		metav1.CreateOptions{})
}

func (cli *TektonPipelineStepImplementation) createPipeline(namespace, timestamp, imageTag, hash,
	radixPipelineJobName, appName, branch string, tasks ...*v1beta1.Task) (*v1beta1.Pipeline, error) {
	taskList := v1beta1.PipelineTaskList{}
	for _, task := range tasks {
		taskList = append(taskList, v1beta1.PipelineTask{
			Name: task.Name,
			TaskRef: &v1beta1.TaskRef{
				Name: task.Name,
			},
		},
		)
	}
	pipelineName := fmt.Sprintf("tekton-pipeline-%s-%s-%s", timestamp, imageTag, hash)
	pipeline := v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:   pipelineName,
			Labels: getLabels(radixPipelineJobName, appName, imageTag, hash),
			Annotations: map[string]string{
				kube.RadixBranchAnnotation: branch,
			},
		},
		Spec: v1beta1.PipelineSpec{Tasks: taskList},
	}
	return cli.GetTektonclient().TektonV1beta1().Pipelines(namespace).Create(context.Background(), &pipeline,
		metav1.CreateOptions{})
}

func (cli *TektonPipelineStepImplementation) createTask(name string, timestamp string, imageTag interface{}, hash string, namespace string) (v1beta1.Task, error) {
	taskName := fmt.Sprintf("tekton-task-%s-%s-%s-%s", name, timestamp, imageTag, hash)
	task := v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name: taskName,
		},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{
				v1beta1.Step{
					Container: corev1.Container{
						Name:    "task1-container1",
						Image:   "bash:latest",
						Command: []string{"echo"},
						Args:    []string{"run task1"},
					},
				},
				//v1beta1.Step{
				//    Container: corev1.Container{
				//        Name:    "task1-container2",
				//        Image:   "bash:latest",
				//        Command: []string{"exit"},
				//        Args:    []string{"0"},
				//    },
				//},
			},
		},
	}
	_, err := cli.GetTektonclient().TektonV1beta1().Tasks(namespace).Create(context.Background(), &task,
		metav1.CreateOptions{})
	return task, err
}

func getLabels(radixPipelineJobName, appName, imageTag, hash string) map[string]string {
	return map[string]string{
		kube.RadixJobNameLabel:  radixPipelineJobName,
		kube.RadixBuildLabel:    fmt.Sprintf("%s-%s-%s", appName, imageTag, hash),
		kube.RadixAppLabel:      appName,
		kube.RadixImageTagLabel: imageTag,
		kube.RadixJobTypeLabel:  kube.RadixJobTypeBuild,
	}
}

// WaitForCompletionOf Will wait for job to complete
func (cli *TektonPipelineStepImplementation) WaitForCompletionOf(pipelineRun *v1beta1.PipelineRun) error {
	errChan := make(chan error)
	stop := make(chan struct{})
	defer close(stop)

	kubeInformerFactory := tektonInformerFactory.NewSharedInformerFactory(cli.GetTektonclient(), time.Second*5)
	genericInformer, err := kubeInformerFactory.ForResource(v1beta1.SchemeGroupVersion.WithResource("pipelineruns"))
	if err != nil {
		return err
	}
	informer := genericInformer.Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, cur interface{}) {
			run, success := cur.(*v1beta1.PipelineRun)
			if success && pipelineRun.GetName() == run.GetName() && pipelineRun.GetNamespace() == run.GetNamespace() && run.Status.PipelineRunStatusFields.CompletionTime != nil {
				conditions := sortByTimestampDesc(run.Status.Conditions)
				if len(conditions) == 0 {
					return
				}
				lastCondition := conditions[0]
				switch {
				case lastCondition.IsTrue():
					log.Infof("pipelineRun completed: %s", lastCondition.Message)
					errChan <- nil
				default:
					errChan <- fmt.Errorf("pipelineRun status reason %s. %s", lastCondition.Reason,
						lastCondition.Message)
				}
			} else {
				log.Debugf("Ongoing - PipelineRun has not completed yet")
			}
		},
		DeleteFunc: func(old interface{}) {
			j, success := old.(*v1beta1.PipelineRun)
			if success && j.GetName() == pipelineRun.GetName() && pipelineRun.GetNamespace() == j.GetNamespace() {
				errChan <- errors.New("PipelineRun failed - Job deleted")
			}
		},
	})

	go informer.Run(stop)
	if !cache.WaitForCacheSync(stop, informer.HasSynced) {
		errChan <- fmt.Errorf("Timed out waiting for caches to sync")
	}

	err = <-errChan
	return err
}

func sortByTimestampDesc(conditions knative.Conditions) knative.Conditions {
	sort.Slice(conditions, func(i, j int) bool {
		return isC1BeforeC2(&conditions[j], &conditions[i])
	})
	return conditions
}

func isC1BeforeC2(c1 *knativeApis.Condition, c2 *knativeApis.Condition) bool {
	return c1.LastTransitionTime.Inner.Before(&c2.LastTransitionTime.Inner)
}

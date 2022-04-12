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
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	tektonInformerFactory "github.com/tektoncd/pipeline/pkg/client/informers/externalversions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	knativeApis "knative.dev/pkg/apis"
	knative "knative.dev/pkg/apis/duck/v1beta1"
)

// RunTektonPipelineStepImplementation Step to run custom pipeline
type RunTektonPipelineStepImplementation struct {
	stepType pipeline.StepType
	model.DefaultStepImplementation
}

// NewTektonPipelineStep Constructor
func NewTektonPipelineStep() model.Step {
	return &RunTektonPipelineStepImplementation{
		stepType: pipeline.RunTektonPipelineStep,
	}
}

// ImplementationForType Override of default step method
func (cli *RunTektonPipelineStepImplementation) ImplementationForType() pipeline.StepType {
	return cli.stepType
}

// SucceededMsg Override of default step method
func (cli *RunTektonPipelineStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Succeded: tekton pipeline step for application %s", cli.GetAppName())
}

// ErrorMsg Override of default step method
func (cli *RunTektonPipelineStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed tekton pipeline for the application %s. Error: %v", cli.GetAppName(), err)
}

// Run Override of default step method
func (cli *RunTektonPipelineStepImplementation) Run(pipelineInfo *model.PipelineInfo) error {
	appName := cli.GetAppName()
	namespace := utils.GetAppNamespace(appName)
	log.Infof("Run tekton pipeline app %s", appName)
	ra := pipelineInfo.RadixApplication
	if ra == nil {
		return fmt.Errorf("no RadixApplication in the pipelineInfo")
	}
	timestamp := time.Now().Format("20060102150405")
	imageTag := pipelineInfo.PipelineArguments.ImageTag
	branch := pipelineInfo.PipelineArguments.Branch
	radixPipelineJobName := pipelineInfo.PipelineArguments.JobName
	hash := strings.ToLower(utils.RandStringStrSeed(5, radixPipelineJobName))
	pipelineList, err := cli.GetTektonclient().TektonV1beta1().Pipelines(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", kube.RadixPipelineRunLabel, pipelineInfo.RadixPipelineRun),
	})
	if err != nil {
		return err
	}

	pipelineRunMap := make(map[string]*v1beta1.PipelineRun)

	for _, pipeline := range pipelineList.Items {
		pipelineRunName := fmt.Sprintf("tekton-pipeline-run-%s-%s-%s", timestamp, imageTag, hash)
		pipelineRun := v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:   pipelineRunName,
				Labels: getLabels(radixPipelineJobName, appName, imageTag, hash, pipelineInfo.RadixPipelineRun),
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
		createdPipelineRun, err := cli.GetTektonclient().TektonV1beta1().PipelineRuns(namespace).Create(context.
			Background(),
			&pipelineRun,
			metav1.CreateOptions{})
		if err != nil {
			return err
		}
		pipelineRunMap[createdPipelineRun.GetName()] = createdPipelineRun
	}

	err = cli.WaitForCompletionOf(pipelineRunMap)
	if err != nil {
		return fmt.Errorf("failed tekton pipeline, %v, for app %s. %v",
			pipelineInfo.TargetEnvironments, appName,
			err)
	}
	return nil
}

func getLabels(radixPipelineJobName, appName, imageTag, hash, radixPipelineRun string) map[string]string {
	return map[string]string{
		kube.RadixJobNameLabel:     radixPipelineJobName,
		kube.RadixBuildLabel:       fmt.Sprintf("%s-%s-%s", appName, imageTag, hash),
		kube.RadixAppLabel:         appName,
		kube.RadixImageTagLabel:    imageTag,
		kube.RadixJobTypeLabel:     kube.RadixJobTypeBuild,
		kube.RadixPipelineRunLabel: radixPipelineRun,
	}
}

// WaitForCompletionOf Will wait for job to complete
func (cli *RunTektonPipelineStepImplementation) WaitForCompletionOf(pipelineRuns map[string]*v1beta1.PipelineRun) error {
	stop := make(chan struct{})
	defer close(stop)

	if len(pipelineRuns) == 0 {
		return nil
	}

	errChan := make(chan error)

	kubeInformerFactory := tektonInformerFactory.NewSharedInformerFactory(cli.GetTektonclient(), time.Second*5)
	genericInformer, err := kubeInformerFactory.ForResource(v1beta1.SchemeGroupVersion.WithResource("pipelineruns"))
	if err != nil {
		return err
	}
	informer := genericInformer.Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, cur interface{}) {
			run, success := cur.(*v1beta1.PipelineRun)
			if !success {
				return
			}
			pipelineRun, ok := pipelineRuns[run.GetName()]
			if !ok {
				return
			}
			if pipelineRun.GetName() == run.GetName() && pipelineRun.GetNamespace() == run.GetNamespace() && run.Status.PipelineRunStatusFields.CompletionTime != nil {
				conditions := sortByTimestampDesc(run.Status.Conditions)
				if len(conditions) == 0 {
					return
				}
				delete(pipelineRuns, run.GetName())
				lastCondition := conditions[0]
				switch {
				case lastCondition.IsTrue():
					log.Infof("pipelineRun completed: %s", lastCondition.Message)
				default:
					log.Errorf("pipelineRun status reason %s. %s", lastCondition.Reason,
						lastCondition.Message)
				}
				if len(pipelineRuns) == 0 {
					errChan <- nil
				}
			} else {
				log.Debugf("Ongoing - PipelineRun has not completed yet")
			}
		},
		DeleteFunc: func(old interface{}) {
			run, success := old.(*v1beta1.PipelineRun)
			if !success {
				return
			}
			pipelineRun, ok := pipelineRuns[run.GetName()]
			if !ok {
				return
			}
			if pipelineRun.GetNamespace() == run.GetNamespace() {
				delete(pipelineRuns, run.GetName())
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

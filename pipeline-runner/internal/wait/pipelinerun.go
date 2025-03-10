package wait

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/rs/zerolog/log"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	tektonInformerFactory "github.com/tektoncd/pipeline/pkg/client/informers/externalversions"
	"k8s.io/client-go/tools/cache"
	knativeApis "knative.dev/pkg/apis"
	knative "knative.dev/pkg/apis/duck/v1"
)

type PipelineRunsCompletionWaiter interface {
	Wait(pipelineRuns map[string]*pipelinev1.PipelineRun, pipelineInfo *model.PipelineInfo) error
}

func NewPipelineRunsCompletionWaiter(tektonClient tektonclient.Interface) PipelineRunsCompletionWaiter {
	return PipelineRunsCompletionWaiterFunc(func(pipelineRuns map[string]*pipelinev1.PipelineRun, pipelineInfo *model.PipelineInfo) error {
		return waitForCompletionOfPipelineRuns(pipelineRuns, tektonClient, pipelineInfo)
	})
}

type PipelineRunsCompletionWaiterFunc func(pipelineRuns map[string]*pipelinev1.PipelineRun, pipelineInfo *model.PipelineInfo) error

func (f PipelineRunsCompletionWaiterFunc) Wait(pipelineRuns map[string]*pipelinev1.PipelineRun, pipelineInfo *model.PipelineInfo) error {
	return f(pipelineRuns, pipelineInfo)
}

// WaitForCompletionOf Will wait for job to complete
func waitForCompletionOfPipelineRuns(pipelineRuns map[string]*pipelinev1.PipelineRun, tektonClient tektonclient.Interface, pipelineInfo *model.PipelineInfo) error {
	stop := make(chan struct{})
	defer close(stop)

	if len(pipelineRuns) == 0 {
		return nil
	}

	errChan := make(chan error)

	kubeInformerFactory := tektonInformerFactory.NewSharedInformerFactoryWithOptions(tektonClient, time.Second*5, tektonInformerFactory.WithNamespace(pipelineInfo.GetAppNamespace()))
	genericInformer, err := kubeInformerFactory.ForResource(pipelinev1.SchemeGroupVersion.WithResource("pipelineruns"))
	if err != nil {
		return fmt.Errorf("waitForCompletionOfPipelineRuns failed to create informer: %w", err)
	}
	informer := genericInformer.Informer()
	_, err = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, cur interface{}) {
			run, success := cur.(*pipelinev1.PipelineRun)
			if !success {
				errChan <- errors.New("updatefunc conversion failed")
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
				case lastCondition.Reason == pipelinev1.PipelineRunReasonCompleted.String():
					log.Info().Msgf("pipelineRun completed: %s", lastCondition.Message)
				case lastCondition.Reason == pipelinev1.PipelineRunReasonFailed.String():
					errChan <- fmt.Errorf("PipelineRun failed: %s", lastCondition.Message)
					return
				default:
					log.Info().Msgf("pipelineRun status %s: %s", lastCondition.Reason, lastCondition.Message)
				}
				if len(pipelineRuns) == 0 {
					errChan <- nil
				}
			} else {
				log.Debug().Msgf("Ongoing - PipelineRun has not completed yet")
			}
		},
		DeleteFunc: func(old interface{}) {
			run, success := old.(*pipelinev1.PipelineRun)
			if !success {
				errChan <- errors.New("deletefunc conversion failed")
				return
			}
			pipelineRun, ok := pipelineRuns[run.GetName()]
			if !ok {
				return
			}
			if pipelineRun.GetNamespace() == run.GetNamespace() {
				delete(pipelineRuns, run.GetName())
				errChan <- errors.New("pipelineRun failed - Job deleted")
			}
		},
	})
	if err != nil {
		return fmt.Errorf("waitForCompletionOfPipelineRuns failed to create event handler: %w", err)
	}

	go informer.Run(stop)
	if !cache.WaitForCacheSync(stop, informer.HasSynced) {
		errChan <- fmt.Errorf("timed out waiting for caches to sync")
	}

	err = <-errChan
	if err != nil {
		return fmt.Errorf("waitForCompletionOfPipelineRuns failed during wait: %w", err)
	}
	return nil
}

func sortByTimestampDesc(conditions knative.Conditions) knative.Conditions {
	sort.Slice(conditions, func(i, j int) bool {
		return isCondition1BeforeCondition2(&conditions[j], &conditions[i])
	})
	return conditions
}

func isCondition1BeforeCondition2(c1 *knativeApis.Condition, c2 *knativeApis.Condition) bool {
	return c1.LastTransitionTime.Inner.Before(&c2.LastTransitionTime.Inner)
}

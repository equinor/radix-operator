package runner

import (
	"context"
	"slices"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type UpdateStatusOption func(rj *v1.RadixJob)

func WithErrorOption(err error) UpdateStatusOption {
	return func(rj *v1.RadixJob) {
		if err != nil {
			rj.Status.Message = err.Error()
			rj.Status.Condition = v1.JobFailed
		} else {
			rj.Status.Message = ""
			rj.Status.Condition = v1.JobSucceeded
		}
	}
}

// WithStepStatusOption returns an UpdateStatusOption that updates the status of a specific step in the RadixJob status.
// If the step does not exist, it will be added to the status.
// The steps are sorted by their start time, with steps that have not started yet sorted to the end.
func WithStepStatusOption(name string, condition v1.RadixJobCondition, started *metav1.Time, ended *metav1.Time, podName *string) UpdateStatusOption {
	return func(rj *v1.RadixJob) {
		steps := rj.Status.Steps
		index := slices.IndexFunc(steps, func(s v1.RadixJobStep) bool {
			return s.Name == name
		})

		var step v1.RadixJobStep
		if index == -1 {
			step = v1.RadixJobStep{
				Name: name,
			}
		} else {
			step = steps[index]
		}

		step.Condition = condition
		step.Started = started
		step.Ended = ended
		step.PodName = *podName

		if index != -1 {
			steps[index] = step
		} else {
			steps = append(steps, step)
		}

		slices.SortStableFunc(steps, func(i, j v1.RadixJobStep) int {
			// Not started is sorted to the end
			if i.Started.IsZero() && !i.Ended.IsZero() {
				return 1
			}
			if !j.Started.IsZero() && j.Ended.IsZero() {
				return -1
			}

			return i.Started.Compare(j.Started.Time)
		})
		rj.Status.Steps = steps
	}
}

func (cli *PipelineRunner) UpdateStatus(ctx context.Context, condition v1.RadixJobCondition, options ...UpdateStatusOption) {

	rj := &v1.RadixJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cli.pipelineInfo.PipelineArguments.JobName,
			Namespace: utils.GetAppNamespace(cli.appName),
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, cli.dynamicClient, rj, func() error {

		rj.Status.Condition = condition
		rj.Status.Started = &metav1.Time{Time: cli.startTime}
		rj.Status.Ended = &metav1.Time{Time: cli.endTime}

		rj.Status.TargetEnvs = slice.Map(cli.pipelineInfo.TargetEnvironments, func(e model.TargetEnvironment) string {
			return e.Environment
		})

		for _, option := range options {
			option(rj)
		}

		return nil
	})
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msgf("Failed to update status of pipeline job %s", cli.pipelineInfo.PipelineArguments.JobName)
	}

	if op != controllerutil.OperationResultNone {
		log.Ctx(ctx).Debug().Msgf("Pipeline job %s status updated with operation: %s", cli.pipelineInfo.PipelineArguments.JobName, op)
	}
}

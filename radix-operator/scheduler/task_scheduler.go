package scheduler

import (
	"context"

	"github.com/equinor/radix-operator/radix-operator/scheduler/internal"
	"github.com/equinor/radix-operator/radix-operator/scheduler/tasks"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

// TaskScheduler Interface for scheduling tasks
type TaskScheduler interface {
	// Start Starts the task scheduler
	Start()
	// Stop Stops the task scheduler
	Stop()
}

type taskScheduler struct {
	cron *cron.Cron
}

func (e taskScheduler) Stop() {
	e.cron.Stop()
}

func (e taskScheduler) Start() {
	e.cron.Start()
}

// NewTaskScheduler Creates a new task scheduler
func NewTaskScheduler(ctx context.Context, task tasks.Task, scheduleSpec string) (TaskScheduler, error) {
	taskLogger := internal.NewLogger(ctx)
	c := cron.New(cron.WithLogger(taskLogger), cron.WithChain(cron.DelayIfStillRunning(taskLogger)))
	if _, err := c.AddFunc(scheduleSpec, task.Run); err != nil {
		return nil, err
	}
	log.Ctx(ctx).Info().Msgf("Created schedule %s for the task %s", scheduleSpec, task.String())
	return &taskScheduler{
		cron: c,
	}, nil
}

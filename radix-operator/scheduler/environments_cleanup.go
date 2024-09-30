package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/equinor/radix-operator/radix-operator/scheduler/internal"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

type environmentsCleanupTask struct {
	cron *cron.Cron
}

func (e environmentsCleanupTask) Stop() {
	e.cron.Stop()
}

func (e environmentsCleanupTask) Start() {
	e.cron.Start()
}

// NewEnvironmentsCleanupTask Creates a new environments cleanup task
func NewEnvironmentsCleanupTask(ctx context.Context, scheduleSpec string) (Task, error) {
	logger := internal.NewLogger(ctx)
	c := cron.New(cron.WithLogger(logger), cron.WithSeconds(), cron.WithChain(cron.DelayIfStillRunning(logger)))
	if _, err := c.AddFunc(scheduleSpec, func() {
		fmt.Println("Task running:", time.Now())
		time.Sleep(15 * time.Second)
	}); err != nil {
		return nil, err
	}
	log.Ctx(ctx).Debug().Msg("Environments cleanup task created")
	return &environmentsCleanupTask{
		cron: c,
	}, nil
}

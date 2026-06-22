package cronserver

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	jobApi "github.com/equinor/radix-operator/job-scheduler/api/v1/handlers/jobs"
	"github.com/equinor/radix-operator/job-scheduler/internal"
	"github.com/equinor/radix-operator/job-scheduler/models"
	"github.com/equinor/radix-operator/job-scheduler/models/common"
	"github.com/equinor/radix-operator/job-scheduler/pkg/batch"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

const (
	concurrencyAllow   = "Allow"
	concurrencyForbid  = "Forbid"
	concurrencyReplace = "Replace"
)

type Server struct {
	kubeUtil     *kube.Kube
	env          *models.Env
	jobComponent *radixv1.RadixDeployJobComponent
	jobHandler   jobApi.JobHandler

	mu sync.Mutex
}

func New(kubeUtil *kube.Kube, env *models.Env, jobComponent *radixv1.RadixDeployJobComponent, jobHandler jobApi.JobHandler) *Server {
	return &Server{
		kubeUtil:     kubeUtil,
		env:          env,
		jobComponent: jobComponent,
		jobHandler:   jobHandler,
	}
}

func (s *Server) Start(ctx context.Context) error {
	if s.jobComponent.Cron == nil {
		return nil
	}

	// time.LoadLocation("") already defaults to UTC; treat an empty/whitespace timezone as omitted
	// so the scheduler doesn't fail to start if the job component is created without prior validation.
	loc := time.UTC
	if tz := strings.TrimSpace(s.jobComponent.Cron.TimeZone); tz != "" {
		var err error
		if loc, err = time.LoadLocation(tz); err != nil {
			return fmt.Errorf("invalid timezone %q for cron job %s: %w",
				tz, s.jobComponent.GetName(), err)
		}
	}

	cronInstance := cron.New(cron.WithLocation(loc))

	for _, schedule := range s.jobComponent.Cron.Schedules {
		if _, err := cronInstance.AddFunc(schedule, func() {
			if ctx.Err() != nil {
				return
			}
			s.mu.Lock()
			defer s.mu.Unlock()

			// Detach from ctx so a SIGTERM mid-callback cannot tear down the
			// stop-then-create sequence used by the Replace concurrency mode.
			runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
			defer cancel()

			ok, err := s.shouldRun(runCtx)
			if err != nil {
				log.Err(err).Msg("failed to prepare cron run")
				return
			}
			if !ok {
				return
			}

			if _, err := s.jobHandler.CreateJob(runCtx, &common.JobScheduleDescription{}, true); err != nil {
				log.Error().Err(err).Msg("failed to create scheduled job")
			}
		}); err != nil {
			return err
		}
	}

	cronInstance.Start()

	<-ctx.Done()
	<-cronInstance.Stop().Done()

	return nil
}

func (s *Server) shouldRun(ctx context.Context) (bool, error) {
	activeCronBatches, err := s.findActiveCronBatches(ctx)
	if err != nil {
		return false, err
	}

	if len(activeCronBatches) == 0 {
		return true, nil
	}

	jobName := s.jobComponent.GetName()
	switch s.jobComponent.Cron.Concurrency {
	case concurrencyForbid:
		log.Info().Msgf("skipping cron job %s: an active batch is already running (Forbid)", jobName)
		return false, nil
	case concurrencyReplace:
		if err := s.stopBatchJobs(ctx, activeCronBatches); err != nil {
			return false, fmt.Errorf("failed to stop active batch for cron job %s: %w", jobName, err)
		}

		log.Info().Msgf("stopped active batch(es) for cron job %s (Replace)", jobName)
		return true, nil
	case concurrencyAllow:
		return true, nil
	default:
		log.Warn().Msgf("invalid concurrency value detected for cron job %s", jobName)
		return false, nil
	}
}

func (s *Server) findActiveCronBatches(ctx context.Context) ([]*radixv1.RadixBatch, error) {
	jobName := s.jobComponent.GetName()
	cronBatches, err := internal.GetRadixBatches(
		ctx,
		s.kubeUtil.RadixClient(),
		s.env.RadixDeploymentNamespace,
		labels.ForComponentName(jobName),
		labels.ForBatchCron(true),
	)
	if err != nil {
		return nil, err
	}

	activeCronBatches := make([]*radixv1.RadixBatch, 0)
	for _, b := range cronBatches {
		if b.Status.Condition.Type == radixv1.BatchConditionTypeActive {
			activeCronBatches = append(activeCronBatches, b)
		}
	}

	return activeCronBatches, nil
}

func (s *Server) stopBatchJobs(ctx context.Context, batches []*radixv1.RadixBatch) error {
	for _, b := range batches {
		if err := batch.StopRadixBatchJob(
			ctx,
			s.kubeUtil.RadixClient(),
			s.env.RadixAppName,
			s.env.RadixEnvironmentName,
			s.jobComponent.Name,
			b.GetName(),
			"",
		); err != nil {
			return err
		}
	}

	return nil
}

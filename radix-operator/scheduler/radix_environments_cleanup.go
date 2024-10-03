package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/radix-operator/scheduler/internal"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
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

// NewRadixEnvironmentsCleanupTask Creates a new RadixEnvironments cleanup task
// The task will run at the specified scheduleSpec and clean up orphaned environments older than the specified retentionPeriod
func NewRadixEnvironmentsCleanupTask(ctx context.Context, kubeUtil *kube.Kube, retentionPeriod time.Duration, scheduleSpec string) (Task, error) {
	taskLogger := internal.NewLogger(ctx)
	c := cron.New(cron.WithLogger(taskLogger), cron.WithChain(cron.DelayIfStillRunning(taskLogger)))
	if _, err := c.AddFunc(scheduleSpec, func() {
		log.Ctx(ctx).Debug().Msgf("Cleanup orphaned RadixEnvironments out of retention period %s", retentionPeriod.String())
		if err := cleanupRadixEnvironments(ctx, kubeUtil, retentionPeriod); err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("Error processing RadixEnvironments")
		}
	}); err != nil {
		return nil, err
	}
	log.Ctx(ctx).Debug().Msg("Created the RadixEnvironments cleanup task")
	return &environmentsCleanupTask{
		cron: c,
	}, nil
}

func cleanupRadixEnvironments(ctx context.Context, kubeUtil *kube.Kube, retentionPeriod time.Duration) error {
	radixEnvironments, err := kubeUtil.ListEnvironments(ctx)
	if err != nil {
		return err
	}
	var errs []error
	outdatedOrphanedEnvironments, err := getOutdatedOrphanedEnvironments(radixEnvironments, retentionPeriod)
	if err != nil {
		errs = append(errs, err)
	}
	for _, outdatedEnvironment := range outdatedOrphanedEnvironments {
		if err = kubeUtil.DeleteEnvironment(ctx, outdatedEnvironment.GetName()); err != nil && !k8serrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("error deleting of the outdated orphaned environment %s: %w", outdatedEnvironment.GetName(), err))
			continue
		}
		log.Ctx(ctx).Info().Msgf("Deleted the outdated orphaned environment %s", outdatedEnvironment.GetName())
	}
	return errors.Join(errs...)
}

func getOutdatedOrphanedEnvironments(radixEnvironments []*radixv1.RadixEnvironment, retentionPeriod time.Duration) ([]*radixv1.RadixEnvironment, error) {
	var errs []error
	return slice.FindAll(radixEnvironments, func(re *radixv1.RadixEnvironment) bool {
		if !re.Status.Orphaned {
			return false
		}
		setOrphanedTimestamp, exists := re.GetAnnotations()[kube.RadixEnvironmentIsOrphanedAnnotation]
		if !exists {
			return false
		}
		timestamp, err := time.Parse(time.RFC3339, setOrphanedTimestamp)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to parse orphaned timestamp in environment %s: %w", re.GetName(), err))
			return false
		}
		return timestamp.Before(time.Now().Add(-retentionPeriod))
	}), errors.Join(errs...)
}

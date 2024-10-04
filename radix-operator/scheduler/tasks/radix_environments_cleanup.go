package tasks

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

type environmentsCleanup struct {
	ctx             context.Context
	kubeUtil        *kube.Kube
	retentionPeriod time.Duration
}

// NewRadixEnvironmentsCleanup Creates a new RadixEnvironments cleanup task
// The task will clean up orphaned environments older than the specified retentionPeriod
func NewRadixEnvironmentsCleanup(ctx context.Context, kubeUtil *kube.Kube, retentionPeriod time.Duration) Task {
	return &environmentsCleanup{
		ctx:             ctx,
		kubeUtil:        kubeUtil,
		retentionPeriod: retentionPeriod,
	}
}

// String Returns the task description
func (e *environmentsCleanup) String() string {
	return "RadixEnvironments cleanup task"
}

// Run Runs the cleanup task
func (e *environmentsCleanup) Run() {
	ctx := e.ctx
	log.Ctx(ctx).Debug().Msgf("Cleanup orphaned RadixEnvironments out of the retention period %s", e.retentionPeriod.String())
	radixEnvironments, err := e.kubeUtil.ListEnvironments(ctx)
	var errs []error
	outdatedOrphanedEnvironments, err := getOutdatedOrphanedEnvironments(radixEnvironments, e.retentionPeriod)
	if err != nil {
		errs = append(errs, err)
	}
	for _, outdatedEnvironment := range outdatedOrphanedEnvironments {
		if err = e.kubeUtil.DeleteEnvironment(ctx, outdatedEnvironment.GetName()); err != nil && !k8serrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("error deleting of the outdated orphaned environment %s: %w", outdatedEnvironment.GetName(), err))
			continue
		}
		log.Ctx(ctx).Info().Msgf("Deleted the outdated orphaned environment %s", outdatedEnvironment.GetName())
	}
	if errs != nil {
		log.Ctx(ctx).Error().Err(errors.Join(errs...)).Msg("Errors processing RadixEnvironments")
	}
}

func getOutdatedOrphanedEnvironments(radixEnvironments []*radixv1.RadixEnvironment, retentionPeriod time.Duration) ([]*radixv1.RadixEnvironment, error) {
	var errs []error
	return slice.FindAll(radixEnvironments, func(re *radixv1.RadixEnvironment) bool {
		if !re.Status.Orphaned || re.Status.OrphanedTimestamp == "" {
			return false
		}
		timestamp, err := time.Parse(time.RFC3339, re.Status.OrphanedTimestamp)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to parse orphaned timestamp in environment %s: %w", re.GetName(), err))
			return false
		}
		return timestamp.Before(time.Now().Add(-retentionPeriod))
	}), errors.Join(errs...)
}

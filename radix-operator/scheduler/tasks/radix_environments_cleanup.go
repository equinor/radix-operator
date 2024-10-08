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

// Run Runs the cleanup task
func (e *environmentsCleanup) Run() {
	log.Ctx(e.ctx).Debug().Msgf("Cleanup orphaned RadixEnvironments out of the retention period %s", e.retentionPeriod.String())
	radixEnvironments, err := e.kubeUtil.ListEnvironments(e.ctx)
	if err != nil {
		log.Ctx(e.ctx).Error().Err(err).Msg("Failed to get RadixEnvironments")
		return
	}
	var errs []error
	outdatedOrphanedEnvironments := getOutdatedOrphanedEnvironments(radixEnvironments, e.retentionPeriod)
	for _, outdatedEnvironment := range outdatedOrphanedEnvironments {
		if err = e.kubeUtil.DeleteEnvironment(e.ctx, outdatedEnvironment.GetName()); err != nil && !k8serrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("error deleting of the outdated orphaned environment %s: %w", outdatedEnvironment.GetName(), err))
			continue
		}
		log.Ctx(e.ctx).Info().Msgf("Deleted the outdated orphaned environment %s", outdatedEnvironment.GetName())
	}
	if errs != nil {
		log.Ctx(e.ctx).Error().Err(errors.Join(errs...)).Msg("Errors processing RadixEnvironments")
	}
}

func getOutdatedOrphanedEnvironments(radixEnvironments []*radixv1.RadixEnvironment, retentionPeriod time.Duration) []*radixv1.RadixEnvironment {
	return slice.FindAll(radixEnvironments, func(re *radixv1.RadixEnvironment) bool {
		return re.Status.Orphaned && re.Status.OrphanedTimestamp != nil && re.Status.OrphanedTimestamp.Time.Before(time.Now().Add(-retentionPeriod))
	})
}

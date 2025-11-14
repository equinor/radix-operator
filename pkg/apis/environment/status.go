package environment

import (
	"context"
	"fmt"
	"time"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

func (env *Environment) syncStatus(ctx context.Context, reconcileErr error) error {
	err := env.updateStatus(ctx, func(currStatus *radixv1.RadixEnvironmentStatus) {
		now := metav1.NewTime(time.Now().UTC())

		isOrphaned := !existsInAppConfig(env.appConfig, env.config.Spec.EnvName)
		currStatus.Orphaned = isOrphaned
		if isOrphaned && currStatus.OrphanedTimestamp == nil {
			currStatus.OrphanedTimestamp = &now
		} else if !isOrphaned && currStatus.OrphanedTimestamp != nil {
			currStatus.OrphanedTimestamp = nil
		}

		currStatus.Reconciled = now
		currStatus.ObservedGeneration = env.config.Generation
		if reconcileErr != nil {
			currStatus.ReconcileStatus = radixv1.RadixAlertReconcileFailed
			currStatus.Message = reconcileErr.Error()
		} else {
			currStatus.ReconcileStatus = radixv1.RadixAlertReconcileSucceeded
			currStatus.Message = ""
		}
	})
	if err != nil {
		return fmt.Errorf("failed to sync status: %w", err)
	}
	return nil
}

func (env *Environment) updateStatus(ctx context.Context, changeStatusFunc func(currStatus *radixv1.RadixEnvironmentStatus)) error {
	radixEnvironmentInterface := env.radixclient.RadixV1().RadixEnvironments()
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		currentEnv, err := radixEnvironmentInterface.Get(ctx, env.config.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}
		changeStatusFunc(&currentEnv.Status)
		updated, err := radixEnvironmentInterface.UpdateStatus(ctx, currentEnv, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		env.config = updated
		return nil
	})
}

func existsInAppConfig(app *radixv1.RadixApplication, envName string) bool {
	if app == nil {
		return false
	}
	for _, appEnv := range app.Spec.Environments {
		if appEnv.Name == envName {
			return true
		}
	}
	return false
}

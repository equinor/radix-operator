package environment

import (
	"context"
	"fmt"

	"github.com/equinor/radix-common/utils/pointers"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

func (env *Environment) syncStatus(ctx context.Context, radixEnvironment *radixv1.RadixEnvironment, time metav1.Time) error {
	if err := updateRadixEnvironmentStatus(ctx, env.radixclient, env.appConfig, radixEnvironment, time); err != nil {
		return fmt.Errorf("failed to update status on environment %s: %w", radixEnvironment.Spec.EnvName, err)
	}
	currentEnv, err := env.radixclient.RadixV1().RadixEnvironments().Get(ctx, radixEnvironment.Spec.EnvName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	env.config = currentEnv
	env.logger.Debug().Msgf("updated status of RadixEnvironment (revision %s)", currentEnv.GetResourceVersion())
	return nil
}

func updateRadixEnvironmentStatus(ctx context.Context, radixClient versioned.Interface, radixApplication *radixv1.RadixApplication, radixEnvironment *radixv1.RadixEnvironment, time metav1.Time) error {
	radixEnvironmentInterface := radixClient.RadixV1().RadixEnvironments()
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		name := radixEnvironment.GetName()
		currentEnv, err := radixEnvironmentInterface.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		changeStatus(radixApplication, radixEnvironment.Spec.EnvName, &currentEnv.Status, time)
		_, err = radixEnvironmentInterface.UpdateStatus(ctx, currentEnv, metav1.UpdateOptions{})
		return err
	})
}

func changeStatus(radixApplication *radixv1.RadixApplication, envName string, currStatus *radixv1.RadixEnvironmentStatus, syncTime metav1.Time) {
	isOrphaned := !existsInAppConfig(radixApplication, envName)
	if isOrphaned && currStatus.OrphanedTimestamp == nil {
		currStatus.OrphanedTimestamp = pointers.Ptr(syncTime)
	} else if !isOrphaned && currStatus.OrphanedTimestamp != nil {
		currStatus.OrphanedTimestamp = nil
	}
	currStatus.Orphaned = isOrphaned
	// time is parameterized for testability
	currStatus.Reconciled = syncTime
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

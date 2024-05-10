package environment

import (
	"context"
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

func (env *Environment) syncStatus(ctx context.Context, re *radixv1.RadixEnvironment, time metav1.Time) error {
	err := env.updateRadixEnvironmentStatus(ctx, re, func(currStatus *radixv1.RadixEnvironmentStatus) {
		currStatus.Orphaned = !existsInAppConfig(env.appConfig, re.Spec.EnvName)
		// time is parameterized for testability
		currStatus.Reconciled = time
	})
	if err != nil {
		return fmt.Errorf("failed to update status on environment %s: %v", re.Spec.EnvName, err)
	}
	return nil
}

func (env *Environment) updateRadixEnvironmentStatus(ctx context.Context, re *radixv1.RadixEnvironment, changeStatusFunc func(currStatus *radixv1.RadixEnvironmentStatus)) error {
	radixEnvironmentInterface := env.radixclient.RadixV1().RadixEnvironments()
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		name := re.GetName()
		currentEnv, err := radixEnvironmentInterface.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		changeStatusFunc(&currentEnv.Status)
		updated, err := radixEnvironmentInterface.UpdateStatus(ctx, currentEnv, metav1.UpdateOptions{})
		if err == nil && env.config.GetName() == name {
			currentEnv, err = radixEnvironmentInterface.Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			env.config = currentEnv
			env.logger.Debug().Msgf("updated status of RadixEnvironment (revision %s)", updated.GetResourceVersion())
			return nil
		}
		return err
	})
}

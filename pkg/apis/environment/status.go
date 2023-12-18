package environment

import (
	"context"
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

func (env *Environment) syncStatus(re *radixv1.RadixEnvironment, time metav1.Time) error {
	err := env.updateRadixEnvironmentStatus(re, func(currStatus *radixv1.RadixEnvironmentStatus) {
		currStatus.Orphaned = !existsInAppConfig(env.appConfig, re.Spec.EnvName)
		// time is parameterized for testability
		currStatus.Reconciled = time
	})
	if err != nil {
		return fmt.Errorf("failed to update status on environment %s: %v", re.Spec.EnvName, err)
	}
	return nil
}

func (env *Environment) updateRadixEnvironmentStatus(re *radixv1.RadixEnvironment, changeStatusFunc func(currStatus *radixv1.RadixEnvironmentStatus)) error {
	radixEnvironmentInterface := env.radixclient.RadixV1().RadixEnvironments()
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		currentEnv, err := radixEnvironmentInterface.Get(context.Background(), re.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}
		changeStatusFunc(&currentEnv.Status)
		_, err = radixEnvironmentInterface.UpdateStatus(context.Background(), currentEnv, metav1.UpdateOptions{})
		if err == nil && env.config.GetName() == re.GetName() {
			currentEnv, err = radixEnvironmentInterface.Get(context.Background(), re.GetName(), metav1.GetOptions{})
			if err == nil {
				env.config = currentEnv
			}
		}
		return err
	})
}

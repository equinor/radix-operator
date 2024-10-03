package environment

import (
	"context"
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (env *Environment) syncStatus(ctx context.Context, radixEnvironment *radixv1.RadixEnvironment, time metav1.Time) error {
	if err := utils.UpdateRadixEnvironmentStatus(ctx, env.radixclient, env.appConfig, radixEnvironment, time); err != nil {
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

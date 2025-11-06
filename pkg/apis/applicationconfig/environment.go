package applicationconfig

import (
	"context"
	"errors"
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

func (app *ApplicationConfig) syncEnvironments(ctx context.Context) error {
	var errs []error
	for _, env := range app.config.Spec.Environments {
		if err := app.syncEnvironment(ctx, app.buildRadixEnvironment(env)); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (app *ApplicationConfig) buildRadixEnvironment(env radixv1.Environment) *radixv1.RadixEnvironment {
	return utils.NewEnvironmentBuilder().
		WithAppName(app.config.Name).
		WithAppLabel().
		WithEnvironmentName(env.Name).
		WithRegistrationOwner(app.registration).
		WithEgressConfig(env.Egress).
		BuildRE()
}

// syncEnvironment creates an environment or applies changes if it exists
func (app *ApplicationConfig) syncEnvironment(ctx context.Context, radixEnvironment *radixv1.RadixEnvironment) error {
	app.logger.Debug().Msgf("Apply RadixEnvironment %s", radixEnvironment.GetName())
	existingRE, err := app.getRadixEnvironment(ctx, radixEnvironment.GetName())
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			return app.createRadixEnvironment(ctx, radixEnvironment)
		}
		return fmt.Errorf("failed to get RadixEnvironment: %w", err)
	}
	return app.updateRadixEnvironment(ctx, existingRE, radixEnvironment)
}

func (app *ApplicationConfig) createRadixEnvironment(ctx context.Context, radixEnvironment *radixv1.RadixEnvironment) error {
	created, err := app.radixclient.RadixV1().RadixEnvironments().Create(ctx, radixEnvironment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create RadixEnvironment: %w", err)
	}
	app.logger.Debug().Msgf("Created RadixEnvironment %s (revision %s)", radixEnvironment.GetName(), created.GetResourceVersion())
	return nil
}

// updateRadixEnvironment updates a RadixEnvironment
func (app *ApplicationConfig) updateRadixEnvironment(ctx context.Context, existing, updated *radixv1.RadixEnvironment) error {
	var iter int
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		defer func() { iter++ }()

		if iter > 0 {
			newExisting, err := app.getRadixEnvironment(ctx, updated.GetName())
			if err != nil {
				if kubeerrors.IsNotFound(err) {
					return nil
				}
				return err
			}
			existing = newExisting
			app.logger.Debug().Msgf("reloaded existing RadixEnvironment %s (resource version %s)", existing.GetName(), existing.GetResourceVersion())
		}

		if cmp.Equal(existing.Spec, updated.Spec, cmpopts.EquateEmpty()) {
			return nil
		}

		newRE := existing.DeepCopy()
		newRE.Spec = updated.Spec
		// Will perform update as patching does not seem to work for this custom resource
		updated, err := app.kubeutil.UpdateRadixEnvironment(ctx, newRE)
		if err != nil {
			return err
		}
		app.logger.Debug().Msgf("Updated RadixEnvironment %s (revision %s)", existing.GetName(), updated.GetResourceVersion())
		return nil
	})
}

func (app *ApplicationConfig) getRadixEnvironment(ctx context.Context, name string) (*radixv1.RadixEnvironment, error) {
	return app.kubeutil.RadixClient().RadixV1().RadixEnvironments().Get(ctx, name, metav1.GetOptions{})
}

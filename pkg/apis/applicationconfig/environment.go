package applicationconfig

import (
	"context"
	"errors"
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

func (app *ApplicationConfig) syncEnvironments() error {
	var errs []error
	for _, env := range app.config.Spec.Environments {
		if err := app.syncEnvironment(app.buildRadixEnvironment(env)); err != nil {
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
func (app *ApplicationConfig) syncEnvironment(radixEnvironment *radixv1.RadixEnvironment) error {
	app.logger.Debug().Msgf("Apply RadixEnvironment %s", radixEnvironment.GetName())
	if _, err := app.getRadixEnvironment(radixEnvironment.GetName()); err != nil {
		if kubeerrors.IsNotFound(err) {
			return app.createRadixEnvironment(radixEnvironment)
		}
		return fmt.Errorf("failed to get RadixEnvironment: %v", err)
	}
	return app.updateRadixEnvironment(radixEnvironment)
}

func (app *ApplicationConfig) createRadixEnvironment(radixEnvironment *radixv1.RadixEnvironment) error {
	created, err := app.radixclient.RadixV1().RadixEnvironments().Create(context.Background(), radixEnvironment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create RadixEnvironment: %w", err)
	}
	app.logger.Debug().Msgf("Created RadixEnvironment %s (revision %s)", radixEnvironment.GetName(), created.GetResourceVersion())
	return nil
}

// updateRadixEnvironment updates a RadixEnvironment
func (app *ApplicationConfig) updateRadixEnvironment(radixEnvironment *radixv1.RadixEnvironment) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		existingRE, err := app.getRadixEnvironment(radixEnvironment.GetName())
		if err != nil {
			if kubeerrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		app.logger.Debug().Msgf("re-taken RadixEnvironment %s (revision %s)", radixEnvironment.GetName(), existingRE.GetResourceVersion())

		newRE := existingRE.DeepCopy()
		newRE.Spec = radixEnvironment.Spec
		// Will perform update as patching does not seem to work for this custom resource
		updated, err := app.kubeutil.UpdateRadixEnvironment(newRE)
		if err != nil {
			return err
		}
		app.logger.Debug().Msgf("Updated RadixEnvironment %s (revision %s)", radixEnvironment.GetName(), updated.GetResourceVersion())
		return nil
	})
}

func (app *ApplicationConfig) getRadixEnvironment(name string) (*radixv1.RadixEnvironment, error) {
	return app.kubeutil.RadixClient().RadixV1().RadixEnvironments().Get(context.Background(), name, metav1.GetOptions{})
}

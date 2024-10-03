package applicationconfig

import (
	"context"
	"errors"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
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
	if err := app.annotateOrphanedEnvironments(ctx, app.config.Spec.Environments); err != nil {
		errs = append(errs, err)
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
	if _, err := app.getRadixEnvironment(ctx, radixEnvironment.GetName()); err != nil {
		if kubeerrors.IsNotFound(err) {
			return app.createRadixEnvironment(ctx, radixEnvironment)
		}
		return fmt.Errorf("failed to get RadixEnvironment: %v", err)
	}
	if annotations := radixEnvironment.GetAnnotations(); annotations != nil {
		if _, ok := annotations[kube.RadixEnvironmentIsOrphanedAnnotation]; ok {
			delete(annotations, kube.RadixEnvironmentIsOrphanedAnnotation)
			radixEnvironment.SetAnnotations(annotations)
		}
	}
	return app.updateRadixEnvironment(ctx, radixEnvironment)
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
func (app *ApplicationConfig) updateRadixEnvironment(ctx context.Context, radixEnvironment *radixv1.RadixEnvironment) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		existingRE, err := app.getRadixEnvironment(ctx, radixEnvironment.GetName())
		if err != nil {
			if kubeerrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		app.logger.Debug().Msgf("re-taken RadixEnvironment %s (revision %s)", radixEnvironment.GetName(), existingRE.GetResourceVersion())

		newRE := existingRE.DeepCopy()
		newRE.SetAnnotations(radixEnvironment.GetAnnotations())
		newRE.Spec = radixEnvironment.Spec
		// Will perform update as patching does not seem to work for this custom resource
		updated, err := app.kubeutil.UpdateRadixEnvironment(ctx, newRE)
		if err != nil {
			return err
		}
		app.logger.Debug().Msgf("Updated RadixEnvironment %s (revision %s)", radixEnvironment.GetName(), updated.GetResourceVersion())
		return nil
	})
}

func (app *ApplicationConfig) getRadixEnvironment(ctx context.Context, name string) (*radixv1.RadixEnvironment, error) {
	return app.kubeutil.RadixClient().RadixV1().RadixEnvironments().Get(ctx, name, metav1.GetOptions{})
}

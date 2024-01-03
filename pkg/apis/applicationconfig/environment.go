package applicationconfig

import (
	"context"
	stderrors "errors"
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
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
	return stderrors.Join(errs...)
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
	logger := log.WithFields(log.Fields{"environment": radixEnvironment.GetName()})
	logger.Debugf("Apply RadixEnvironment")
	if _, err := app.getRadixEnvironment(radixEnvironment.GetName()); err != nil {
		if errors.IsNotFound(err) {
			return app.createRadixEnvironment(radixEnvironment, logger)
		}
		return fmt.Errorf("failed to get RadixEnvironment: %v", err)
	}
	return app.updateRadixEnvironment(radixEnvironment, logger)
}

func (app *ApplicationConfig) createRadixEnvironment(radixEnvironment *radixv1.RadixEnvironment, logger *log.Entry) error {
	created, err := app.radixclient.RadixV1().RadixEnvironments().Create(context.Background(), radixEnvironment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create RadixEnvironment: %v", err)
	}
	logger.Debugf("Created RadixEnvironment (revision %s)", created.GetResourceVersion())
	return nil
}

// updateRadixEnvironment updates a RadixEnvironment
func (app *ApplicationConfig) updateRadixEnvironment(radixEnvironment *radixv1.RadixEnvironment, logger *log.Entry) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		existingRE, err := app.getRadixEnvironment(radixEnvironment.GetName())
		if err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}
		logger.Debugf("re-taken RadixEnvironment (revision %s)", existingRE.GetResourceVersion())

		newRE := existingRE.DeepCopy()
		newRE.Spec = radixEnvironment.Spec
		// Will perform update as patching does not seem to work for this custom resource
		updated, err := app.kubeutil.UpdateRadixEnvironment(newRE)
		if err != nil {
			return fmt.Errorf("failed to update RadixEnvironment: %v", err)
		}
		logger.Debugf("Updated RadixEnvironment (revision %s)", updated.GetResourceVersion())
		return nil
	})
}

func (app *ApplicationConfig) getRadixEnvironment(name string) (*radixv1.RadixEnvironment, error) {
	return app.kubeutil.RadixClient().RadixV1().RadixEnvironments().Get(context.Background(), name, metav1.GetOptions{})
}

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
		// Orphaned flag will be set by the environment handler but until
		// reconciliation we must ensure it is false
		// Update: It seems Update method does not update status object when using real k8s client, but the fake client does.
		// Only an explicit call to UpdateStatus can update status object, and this is only done by the RadixEnvironment controller.
		WithOrphaned(false).
		BuildRE()
}

// syncEnvironment creates an environment or applies changes if it exists
func (app *ApplicationConfig) syncEnvironment(newRE *radixv1.RadixEnvironment) error {
	name := newRE.GetName()
	logger := log.WithFields(log.Fields{"environment": name})
	logger.Debugf("Apply RadixEnvironment ")
	oldRE, err := app.getRadixEnvironment(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return app.createRadixEnvironment(newRE, logger)
		}
		return fmt.Errorf("failed to get RadixEnvironment: %v", err)
	}
	logger.Debugf("taken RadixEnvironment (revision %s)", oldRE.GetResourceVersion())
	return app.updateRadixEnvironment(oldRE, newRE, logger)
}

func (app *ApplicationConfig) createRadixEnvironment(radixEnvironment *radixv1.RadixEnvironment, logger *log.Entry) error {
	created, err := app.radixclient.RadixV1().RadixEnvironments().Create(context.Background(), radixEnvironment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create RadixEnvironment: %v", err)
	}
	logger.Debugf("Created RadixEnvironment (revision %s)", created.GetResourceVersion())
	return nil
}

// updateRadixEnvironment creates a mergepatch, comparing old and new RadixEnvironments and issues the patch to radix
func (app *ApplicationConfig) updateRadixEnvironment(oldRE *radixv1.RadixEnvironment, newRE *radixv1.RadixEnvironment, logger *log.Entry) error {
	radixEnvironment := oldRE.DeepCopy()
	logger.Debugf("update RadixEnvironment (revision %s)", oldRE.GetResourceVersion())
	name := newRE.GetName()
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		radixEnvironment.Spec = newRE.Spec

		// Will perform update as patching does not seem to work for this custom resource
		updated, err := app.kubeutil.UpdateRadixEnvironment(radixEnvironment)
		if err == nil {
			logger.Debugf("Updated RadixEnvironment (revision %s)", updated.GetResourceVersion())
			return nil
		}
		if errors.IsNotFound(err) {
			logger.Debugf("RadixEnvironment does not exist")
			return nil
		}
		if !errors.IsConflict(err) {
			return fmt.Errorf("failed to update RadixEnvironment object: %v", err)
		}
		if radixEnvironment, err = app.getRadixEnvironment(name); err != nil {
			if errors.IsNotFound(err) {
				logger.Debugf("RadixEnvironment does not exist")
				return nil
			}
			logger.Debugf("re-taken RadixEnvironment  revision %s", radixEnvironment.GetResourceVersion())
			return err
		}
		return stderrors.New("failed to update changed RadixEnvironment")
	})
}

func (app *ApplicationConfig) getRadixEnvironment(name string) (*radixv1.RadixEnvironment, error) {
	return app.kubeutil.RadixClient().RadixV1().RadixEnvironments().Get(context.Background(), name, metav1.GetOptions{})
}

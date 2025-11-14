package application

import (
	"context"
	"fmt"

	"k8s.io/client-go/util/retry"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Application Instance variables
type Application struct {
	kubeclient   kubernetes.Interface
	radixclient  radixclient.Interface
	kubeutil     *kube.Kube
	registration *v1.RadixRegistration
}

// NewApplication Constructor
func NewApplication(
	kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	registration *v1.RadixRegistration) (Application, error) {

	return Application{
		kubeclient:   kubeclient,
		radixclient:  radixclient,
		kubeutil:     kubeutil,
		registration: registration,
	}, nil
}

// OnSync compares the actual state with the desired, and attempts to converge the two
func (app *Application) OnSync(ctx context.Context) error {
	ctx = log.Ctx(ctx).With().Str("resource_kind", v1.KindRadixRegistration).Logger().WithContext(ctx)
	log.Ctx(ctx).Info().Msg("Syncing")
	return app.syncStatus(ctx, app.reconcile(ctx))
}

func (app *Application) reconcile(ctx context.Context) error {

	if err := app.createAppNamespace(ctx); err != nil {
		return fmt.Errorf("failed to create app namespace: %w", err)
	}
	log.Ctx(ctx).Debug().Msg("App namespace created")

	if err := app.createLimitRangeOnAppNamespace(ctx); err != nil {
		return fmt.Errorf("failed to create limit range on app namespace: %w", err)
	}
	log.Ctx(ctx).Debug().Msg("Limit range on app namespace created")

	if err := app.applySecretsForPipelines(ctx); err != nil {
		return fmt.Errorf("failed to apply pipeline secrets: %w", err)
	}

	if err := utils.GrantAppAdminAccessToSecret(ctx, app.kubeutil, app.registration, defaults.GitPrivateKeySecretName, defaults.GitPrivateKeySecretName); err != nil {
		return fmt.Errorf("failed to grant access to git private key secret: %w", err)
	}
	log.Ctx(ctx).Debug().Msg("Applied secrets needed by pipelines")

	if err := app.applyRbacOnPipelineRunner(ctx); err != nil {
		return fmt.Errorf("failed to apply pipeline permissions: %w", err)
	}
	log.Ctx(ctx).Debug().Msg("Applied access permissions needed by pipeline")

	if err := app.applyRbacRadixRegistration(ctx); err != nil {
		return fmt.Errorf("failed to grant access to RadixRegistration: %w", err)
	}
	log.Ctx(ctx).Debug().Msg("Applied access permissions to RadixRegistration")

	if err := app.applyRbacAppNamespace(ctx); err != nil {
		return fmt.Errorf("failed to grant access to app namespace: %w", err)
	}

	return nil
}

func (app *Application) syncStatus(ctx context.Context, reconcileErr error) error {
	err := app.updateStatus(ctx, func(currStatus *v1.RadixRegistrationStatus) {
		currStatus.Reconciled = metav1.Now()
		currStatus.ObservedGeneration = app.registration.Generation

		if reconcileErr != nil {
			currStatus.ReconcileStatus = v1.RadixRegistrationReconcileFailed
			currStatus.Message = reconcileErr.Error()
		} else {
			currStatus.ReconcileStatus = v1.RadixRegistrationReconcileSucceeded
			currStatus.Message = ""
		}
	})
	if err != nil {
		return fmt.Errorf("failed to sync status: %w", err)
	}

	return reconcileErr
}

func (app *Application) updateStatus(ctx context.Context, changeStatusFunc func(currStatus *v1.RadixRegistrationStatus)) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		updateObj := app.registration.DeepCopy()
		changeStatusFunc(&updateObj.Status)
		updateObj, err := app.radixclient.RadixV1().RadixRegistrations().UpdateStatus(ctx, updateObj, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		app.registration = updateObj
		return nil
	})
}

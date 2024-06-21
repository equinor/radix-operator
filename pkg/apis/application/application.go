package application

import (
	"context"
	"fmt"
	"time"

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

// OnSync compares the actual state with the desired, and attempts to
// converge the two
func (app *Application) OnSync(ctx context.Context) error {
	ctx = log.Ctx(ctx).With().Str("resource_kind", v1.KindRadixRegistration).Logger().WithContext(ctx)
	log.Ctx(ctx).Info().Msg("Syncing")
	radixRegistration := app.registration

	err := app.createAppNamespace(ctx)
	if err != nil {
		return fmt.Errorf("failed to create app namespace: %w", err)
	}
	log.Ctx(ctx).Debug().Msg("App namespace created")

	err = app.createLimitRangeOnAppNamespace(ctx, utils.GetAppNamespace(radixRegistration.Name))
	if err != nil {
		return fmt.Errorf("failed to create limit range on app namespace: %w", err)
	}

	log.Ctx(ctx).Debug().Msg("Limit range on app namespace created")

	err = app.applySecretsForPipelines(ctx) // create deploy key in app namespace
	if err != nil {
		return fmt.Errorf("failed to apply pipeline secrets: %w", err)
	}

	err = utils.GrantAppAdminAccessToSecret(ctx, app.kubeutil, app.registration, defaults.GitPrivateKeySecretName, defaults.GitPrivateKeySecretName)
	if err != nil {
		return fmt.Errorf("failed to grant access to git private key secret: %w", err)
	}
	log.Ctx(ctx).Debug().Msg("Applied secrets needed by pipelines")

	err = app.applyRbacOnRadixTekton(ctx)
	if err != nil {
		return fmt.Errorf("failed to grant access to Tekton resources: %w", err)
	}

	err = app.applyRbacOnPipelineRunner(ctx)
	if err != nil {
		return fmt.Errorf("failed to apply pipeline permissions: %w", err)
	}
	log.Ctx(ctx).Debug().Msg("Applied access permissions needed by pipeline")

	err = app.applyRbacRadixRegistration(ctx)
	if err != nil {
		return fmt.Errorf("failed to grant access to RadixRegistration: %w", err)
	}
	log.Ctx(ctx).Debug().Msg("Applied access permissions to RadixRegistration")

	err = app.applyRbacAppNamespace(ctx)
	if err != nil {
		return fmt.Errorf("failed to grant access to app namespace: %w", err)
	}

	log.Ctx(ctx).Debug().Msg("Applied access to app namespace. Set registration to be reconciled")
	err = app.updateRadixRegistrationStatus(ctx, radixRegistration, func(currStatus *v1.RadixRegistrationStatus) {
		currStatus.Reconciled = metav1.NewTime(time.Now().UTC())
	})
	if err != nil {
		return fmt.Errorf("failed to update status on RadixRegistration: %w", err)
	}

	return nil
}

func (app *Application) updateRadixRegistrationStatus(ctx context.Context, rr *v1.RadixRegistration, changeStatusFunc func(currStatus *v1.RadixRegistrationStatus)) error {
	rrInterface := app.radixclient.RadixV1().RadixRegistrations()
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		currentRR, err := rrInterface.Get(ctx, rr.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}
		changeStatusFunc(&currentRR.Status)
		_, err = rrInterface.UpdateStatus(ctx, currentRR, metav1.UpdateOptions{})

		if err == nil && rr.GetName() == app.registration.GetName() {
			currentRR, err = rrInterface.Get(ctx, rr.GetName(), metav1.GetOptions{})
			if err == nil {
				app.registration = currentRR
			}
		}
		return err
	})
}

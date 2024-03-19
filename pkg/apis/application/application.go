package application

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog"
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
	logger       zerolog.Logger
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
		logger:       log.Logger.With().Str("kind", registration.Kind).Str("name", cache.MetaObjectToName(&registration.ObjectMeta).String()).Logger(),
	}, nil
}

// OnSync compares the actual state with the desired, and attempts to
// converge the two
func (app *Application) OnSync() error {
	radixRegistration := app.registration

	err := app.createAppNamespace()
	if err != nil {
		return fmt.Errorf("failed to create app namespace: %w", err)
	}
	app.logger.Debug().Msg("App namespace created")

	err = app.createLimitRangeOnAppNamespace(utils.GetAppNamespace(radixRegistration.Name))
	if err != nil {
		return fmt.Errorf("failed to create limit range on app namespace: %w", err)
	}

	app.logger.Debug().Msg("Limit range on app namespace created")

	err = app.applySecretsForPipelines() // create deploy key in app namespace
	if err != nil {
		return fmt.Errorf("failed to apply pipeline secrets: %w", err)
	}

	err = utils.GrantAppAdminAccessToSecret(app.kubeutil, app.registration, defaults.GitPrivateKeySecretName, defaults.GitPrivateKeySecretName)
	if err != nil {
		return fmt.Errorf("failed to grant access to git private key secret: %w", err)
	}
	app.logger.Debug().Msg("Applied secrets needed by pipelines")

	err = app.applyRbacOnRadixTekton()
	if err != nil {
		return fmt.Errorf("failed to grant access to Tekton resources: %w", err)
	}

	err = app.applyRbacOnPipelineRunner()
	if err != nil {
		return fmt.Errorf("failed to apply pipeline permissions: %w", err)
	}
	app.logger.Debug().Msg("Applied access permissions needed by pipeline")

	err = app.applyRbacRadixRegistration()
	if err != nil {
		return fmt.Errorf("failed to grant access to RadixRegistration: %w", err)
	}
	app.logger.Debug().Msg("Applied access permissions to RadixRegistration")

	err = app.applyRbacAppNamespace()
	if err != nil {
		return fmt.Errorf("failed to grant access to app namespace: %w", err)
	}

	app.logger.Debug().Msg("Applied access to app namespace. Set registration to be reconciled")
	err = app.updateRadixRegistrationStatus(radixRegistration, func(currStatus *v1.RadixRegistrationStatus) {
		currStatus.Reconciled = metav1.NewTime(time.Now().UTC())
	})
	if err != nil {
		return fmt.Errorf("failed to update status on RadixRegistration: %w", err)
	}

	return nil
}

func (app *Application) updateRadixRegistrationStatus(rr *v1.RadixRegistration, changeStatusFunc func(currStatus *v1.RadixRegistrationStatus)) error {
	rrInterface := app.radixclient.RadixV1().RadixRegistrations()
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		currentRR, err := rrInterface.Get(context.TODO(), rr.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}
		changeStatusFunc(&currentRR.Status)
		_, err = rrInterface.UpdateStatus(context.TODO(), currentRR, metav1.UpdateOptions{})

		if err == nil && rr.GetName() == app.registration.GetName() {
			currentRR, err = rrInterface.Get(context.TODO(), rr.GetName(), metav1.GetOptions{})
			if err == nil {
				app.registration = currentRR
			}
		}
		return err
	})
}

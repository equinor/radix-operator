package application

import (
	"context"
	"time"

	"k8s.io/client-go/util/retry"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var logger *log.Entry

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
		kubeclient,
		radixclient,
		kubeutil,
		registration}, nil
}

// OnSync compares the actual state with the desired, and attempts to
// converge the two
func (app Application) OnSync() error {
	radixRegistration := app.registration
	logger = log.WithFields(log.Fields{"registrationName": radixRegistration.GetName()})

	err := app.createAppNamespace()
	if err != nil {
		logger.Errorf("Failed to create app namespace. %v", err)
		return err
	}
	logger.Debugf("App namespace created")

	err = app.createLimitRangeOnAppNamespace(utils.GetAppNamespace(radixRegistration.Name))
	if err != nil {
		logger.Errorf("Failed to create limit range on app namespace. %v", err)
		return err
	}

	logger.Debugf("Limit range on app namespace created")

	err = app.applySecretsForPipelines() // create deploy key in app namespace
	if err != nil {
		logger.Errorf("Failed to apply secrets needed by pipeline. %v", err)
		return err
	}

	err = utils.GrantAppAdminAccessToSecret(app.kubeutil, app.registration, defaults.GitPrivateKeySecretName, defaults.GitPrivateKeySecretName)
	if err != nil {
		logger.Errorf("Failed to grant access to git private key secret. %v", err)
		return err
	}

	logger.Debugf("Applied secrets needed by pipelines")

	err = app.applyRbacOnRadixTekton()
	if err != nil {
		logger.Errorf("failed to set access permissions needed to copy radix config to configmap and load Tekton pipelines and tasks: %v", err)
		return err
	}
	err = app.applyRbacOnPipelineRunner()
	if err != nil {
		logger.Errorf("failed to set access permissions needed by pipeline: %v", err)
		return err
	}

	logger.Debugf("Applied access permissions needed by pipeline")

	err = app.applyRbacRadixRegistration()
	if err != nil {
		logger.Errorf("Failed to set access on RadixRegistration: %v", err)
		return err
	}

	logger.Debugf("Applied access permissions to RadixRegistration")

	err = app.applyRbacAppNamespace()
	if err != nil {
		logger.Errorf("Failed to grant access to app namespace: %v", err)
		return err
	}

	logger.Debugf("Applied access to app namespace. Set registration to be reconciled")
	err = app.updateRadixRegistrationStatus(radixRegistration, func(currStatus *v1.RadixRegistrationStatus) {
		currStatus.Reconciled = metav1.NewTime(time.Now().UTC())
	})
	if err != nil {
		logger.Errorf("Failed to update status on registration: %v", err)
		return err
	}
	return nil
}

func (app Application) updateRadixRegistrationStatus(rr *v1.RadixRegistration, changeStatusFunc func(currStatus *v1.RadixRegistrationStatus)) error {
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

package application

import (
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
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
	radixclient radixclient.Interface,
	registration *v1.RadixRegistration) (Application, error) {
	kubeutil, err := kube.New(kubeclient)
	if err != nil {
		return Application{}, err
	}

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
	logger = log.WithFields(log.Fields{"registrationName": radixRegistration.GetName(), "registrationNamespace": radixRegistration.GetNamespace()})

	err := app.createAppNamespace()
	if err != nil {
		logger.Errorf("Failed to create app namespace. %v", err)
		return err
	} else {
		logger.Debugf("App namespace created")
	}

	err = app.createLimitRangeOnAppNamespace(utils.GetAppNamespace(radixRegistration.Name))
	if err != nil {
		logger.Errorf("Failed to create limit range on app namespace. %v", err)
		return err
	} else {
		logger.Debugf("Limit range on app namespace created")
	}

	err = app.applySecretsForPipelines() // create deploy key in app namespace
	if err != nil {
		logger.Errorf("Failed to apply secrets needed by pipeline. %v", err)
		return err
	} else {
		logger.Debugf("Applied secrets needed by pipelines")
	}

	pipelineServiceAccount, err := app.applyPipelineServiceAccount()
	if err != nil {
		logger.Errorf("Failed to apply service account needed by pipeline. %v", err)
		return err
	} else {
		logger.Debugf("Applied service account needed by pipelines")
	}

	err = app.applyRbacOnPipelineRunner(pipelineServiceAccount)
	if err != nil {
		logger.Errorf("Failed to set access permissions needed by pipeline: %v", err)
		return err
	} else {
		logger.Debugf("Applied access permissions needed by pipeline")
	}

	err = app.applyRbacRadixRegistration()
	if err != nil {
		logger.Errorf("Failed to set access on RadixRegistration: %v", err)
		return err
	} else {
		logger.Debugf("Applied access permissions to RadixRegistration")
	}

	err = app.grantAccessToCICDLogs()
	if err != nil {
		logger.Errorf("Failed to grant access to ci/cd logs: %v", err)
		return err
	} else {
		logger.Debugf("Applied access to ci/cd logs")
	}

	return nil
}

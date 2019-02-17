package application

import (
	"strings"

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

// OnRegistered called when an application is registered (new RadixRegistration in cluster)
func (app Application) OnRegistered() error {
	radixRegistration := app.registration
	logger = log.WithFields(log.Fields{"registrationName": radixRegistration.GetName(), "registrationNamespace": radixRegistration.GetNamespace()})

	err := app.createAppNamespace()
	if err != nil {
		logger.Errorf("Failed to create app namespace. %v", err)
		return err
	} else {
		logger.Infof("App namespace created")
	}

	err = app.applySecretsForPipelines() // create deploy key in app namespace
	if err != nil {
		logger.Errorf("Failed to apply secrets needed by pipeline. %v", err)
		return err
	} else {
		logger.Infof("Applied secrets needed by pipelines")
	}

	pipelineServiceAccount, err := app.applyPipelineServiceAccount()
	if err != nil {
		logger.Errorf("Failed to apply service account needed by pipeline. %v", err)
		return err
	} else {
		logger.Infof("Applied service account needed by pipelines")
	}

	err = app.applyRbacRadixRegistration()
	if err != nil {
		logger.Errorf("Failed to set access on RadixRegistration: %v", err)
		return err
	} else {
		logger.Infof("Applied access permissions to RadixRegistration")
	}

	err = app.grantAccessToCICDLogs()
	if err != nil {
		logger.Errorf("Failed to grant access to ci/cd logs: %v", err)
		return err
	} else {
		logger.Infof("Applied access to ci/cd logs")
	}

	err = app.applyRbacOnPipelineRunner(pipelineServiceAccount)
	if err != nil {
		logger.Errorf("Failed to set access permissions needed by pipeline: %v", err)
		return err
	} else {
		logger.Infof("Applied access permissions needed by pipeline")
	}

	return nil
}

func (app Application) OnUpdated(radixRegistrationOld *v1.RadixRegistration) {
	radixRegistration := app.registration
	logger = logger.WithFields(log.Fields{"registrationName": radixRegistration.ObjectMeta.Name, "registrationNamespace": radixRegistration.ObjectMeta.Namespace})

	if !strings.EqualFold(radixRegistration.Spec.DeployKey, radixRegistrationOld.Spec.DeployKey) {
		err := app.applySecretsForPipelines() // create deploy key in app namespace
		if err != nil {
			logger.Errorf("Failed to apply secrets needed by pipeline. %v", err)
		} else {
			logger.Infof("Applied secrets needed by pipelines")
		}
	}

	if !utils.ArrayEqualElements(radixRegistration.Spec.AdGroups, radixRegistrationOld.Spec.AdGroups) {
		err := app.applyRbacRadixRegistration()
		if err != nil {
			logger.Errorf("Failed to set access on RadixRegistration: %v", err)
		} else {
			logger.Infof("Applied access permissions to RadixRegistration")
		}

		err = app.grantAccessToCICDLogs()
		if err != nil {
			logger.Errorf("Failed to grant access to ci/cd logs: %v", err)
		} else {
			logger.Infof("Applied access to ci/cd logs")
		}
	}
}

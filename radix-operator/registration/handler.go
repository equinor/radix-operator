package registration

import (
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"k8s.io/client-go/kubernetes"
)

type RadixRegistrationHandler struct {
	kubeclient kubernetes.Interface
}

//NewRegistrationHandler creates a handler which deals with RadixRegistration resources
func NewRegistrationHandler(kubeclient kubernetes.Interface) RadixRegistrationHandler {
	handler := RadixRegistrationHandler{
		kubeclient: kubeclient,
	}

	return handler
}

// Init handles any handler initialization
func (t *RadixRegistrationHandler) Init() error {
	logger.Info("RadixRegistrationHandler.Init")
	return nil
}

// ObjectCreated is called when an object is created
func (t *RadixRegistrationHandler) ObjectCreated(obj interface{}) error {
	radixRegistration, ok := obj.(*v1.RadixRegistration)
	if !ok {
		return fmt.Errorf("Provided object was not a valid Radix Registration; instead was %v", obj)
	}

	t.processRadixRegistration(radixRegistration)
	return nil
}

// TODO: Move this into Application domain/package
func (t *RadixRegistrationHandler) processRadixRegistration(radixRegistration *v1.RadixRegistration) {
	logger = logger.WithFields(log.Fields{"registrationName": radixRegistration.ObjectMeta.Name, "registrationNamespace": radixRegistration.ObjectMeta.Namespace})
	kube, _ := kube.New(t.kubeclient)

	err := kube.CreateEnvironment(radixRegistration, "app")
	if err != nil {
		logger.Errorf("Failed to create app namespace. %v", err)
	} else {
		logger.Infof("App namespace created")
	}

	err = kube.ApplySecretsForPipelines(radixRegistration) // create deploy key in app namespace
	if err != nil {
		logger.Errorf("Failed to apply secrets needed by pipeline. %v", err)
	} else {
		logger.Infof("Applied secrets needed by pipelines")
	}

	pipelineServiceAccount, err := kube.ApplyPipelineServiceAccount(radixRegistration)
	if err != nil {
		logger.Errorf("Failed to apply service account needed by pipeline. %v", err)
	} else {
		logger.Infof("Applied service account needed by pipelines")
	}

	err = kube.ApplyRbacRadixRegistration(radixRegistration)
	if err != nil {
		logger.Errorf("Failed to set access on RadixRegistration: %v", err)
	} else {
		logger.Infof("Applied access permissions to RadixRegistration")
	}

	err = kube.GrantAccessToCICDLogs(radixRegistration)
	if err != nil {
		logger.Errorf("Failed to grant access to ci/cd logs: %v", err)
	} else {
		logger.Infof("Applied access to ci/cd logs")
	}

	err = kube.ApplyRbacOnPipelineRunner(radixRegistration, pipelineServiceAccount)
	if err != nil {
		logger.Errorf("Failed to set access permissions needed by pipeline: %v", err)
	} else {
		logger.Infof("Applied access permissions needed by pipeline")
	}
}

// ObjectDeleted is called when an object is deleted
func (t *RadixRegistrationHandler) ObjectDeleted(key string) error {
	return nil
}

// ObjectUpdated is called when an object is updated
func (t *RadixRegistrationHandler) ObjectUpdated(objOld, objNew interface{}) error {
	if objOld == nil {
		log.Info("update radix registration - no new changes (objOld == nil)")
		return nil
	}

	radixRegistrationOld, ok := objOld.(*v1.RadixRegistration)
	if !ok {
		return fmt.Errorf("Provided old object was not a valid Radix Registration; instead was %v", objOld)
	}

	radixRegistration, ok := objNew.(*v1.RadixRegistration)
	if !ok {
		return fmt.Errorf("Provided new object was not a valid Radix Registration; instead was %v", objNew)
	}

	logger = logger.WithFields(log.Fields{"registrationName": radixRegistration.ObjectMeta.Name, "registrationNamespace": radixRegistration.ObjectMeta.Namespace})
	kube, _ := kube.New(t.kubeclient)

	if !strings.EqualFold(radixRegistration.Spec.DeployKey, radixRegistrationOld.Spec.DeployKey) {
		err := kube.ApplySecretsForPipelines(radixRegistration) // create deploy key in app namespace
		if err != nil {
			logger.Errorf("Failed to apply secrets needed by pipeline. %v", err)
		} else {
			logger.Infof("Applied secrets needed by pipelines")
		}
	}

	if !utils.ArrayEqualElements(radixRegistration.Spec.AdGroups, radixRegistrationOld.Spec.AdGroups) {
		err := kube.ApplyRbacRadixRegistration(radixRegistration)
		if err != nil {
			logger.Errorf("Failed to set access on RadixRegistration: %v", err)
		} else {
			logger.Infof("Applied access permissions to RadixRegistration")
		}

		err = kube.GrantAccessToCICDLogs(radixRegistration)
		if err != nil {
			logger.Errorf("Failed to grant access to ci/cd logs: %v", err)
		} else {
			logger.Infof("Applied access to ci/cd logs")
		}
	}

	return nil
}

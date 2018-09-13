package registration

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-operator/pkg/apis/kube"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
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

	logger = logger.WithFields(log.Fields{"registrationName": radixRegistration.ObjectMeta.Name, "registrationNamespace": radixRegistration.ObjectMeta.Namespace})
	kube, _ := kube.New(t.kubeclient)

	err := kube.CreateEnvironment(radixRegistration, "app")
	if err != nil {
		logger.Errorf("Failed to create app namespace. %v", err)
	} else {
		logger.Infof("App namespace created")
	}

	err = kube.ApplySecretsForPipelines(radixRegistration)
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

	err = kube.ApplyRbacOnPipelineRunner(radixRegistration, pipelineServiceAccount)
	if err != nil {
		logger.Errorf("Failed to set access permissions needed by pipeline: %v", err)
	} else {
		logger.Infof("Applied access permissions needed by pipeline")
	}

	return nil
}

// ObjectDeleted is called when an object is deleted
func (t *RadixRegistrationHandler) ObjectDeleted(key string) error {
	return nil
}

// ObjectUpdated is called when an object is updated
func (t *RadixRegistrationHandler) ObjectUpdated(objOld, objNew interface{}) error {
	radixRegistration, ok := objNew.(*v1.RadixRegistration)
	if !ok {
		return fmt.Errorf("Provided object was not a valid Radix Registration; instead was %v", objNew)
	}

	logger = logger.WithFields(log.Fields{"registrationName": radixRegistration.ObjectMeta.Name, "registrationNamespace": radixRegistration.ObjectMeta.Namespace})

	// todo, ensure update is handled correctly
	return nil
}

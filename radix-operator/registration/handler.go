package registration

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

type RadixRegistrationHandler struct {
	kubeclient  kubernetes.Interface
	radixclient radixclient.Interface
}

//NewRegistrationHandler creates a handler which deals with RadixRegistration resources
func NewRegistrationHandler(kubeclient kubernetes.Interface, radixclient radixclient.Interface) RadixRegistrationHandler {
	handler := RadixRegistrationHandler{
		kubeclient:  kubeclient,
		radixclient: radixclient,
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
	application, err := application.NewApplication(t.kubeclient, t.radixclient, radixRegistration)
	application.OnRegistered()
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

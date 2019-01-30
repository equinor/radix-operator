package kube

import (
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
)

// CreateEnvironment creates a namespace with RadixRegistration as owner
// TODO : This should be moved closer to Registration domain/package
func (k *Kube) CreateEnvironment(registration *radixv1.RadixRegistration, envName string) error {
	logger = logger.WithFields(log.Fields{"registrationName": registration.ObjectMeta.Name, "registrationNamespace": registration.ObjectMeta.Namespace})

	name := utils.GetEnvironmentNamespace(registration.Name, envName)

	labels := map[string]string{
		"radixApp":    registration.Name, // For backwards compatibility. Remove when cluster is migrated
		RadixAppLabel: registration.Name,
		RadixEnvLabel: envName,
	}

	// ownerRef := GetOwnerReferenceOfRegistration(registration)
	err := k.ApplyNamespace(name, labels, nil)

	if err != nil {
		logger.Errorf("Failed to create namespace %s: %v", name, err)
		return err
	}

	logger.Infof("Created namespace %s", name)
	return nil
}

package application

import (
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
)

// CreateEnvironment creates a namespace with RadixRegistration as owner
func (app Application) createAppNamespace() error {
	registration := app.registration
	envName := "app"
	name := utils.GetEnvironmentNamespace(registration.Name, envName)

	labels := map[string]string{
		"radixApp":         registration.Name, // For backwards compatibility. Remove when cluster is migrated
		kube.RadixAppLabel: registration.Name,
		kube.RadixEnvLabel: envName,
	}

	ownerRef := app.getOwnerReference()
	err := app.kubeutil.ApplyNamespace(name, labels, ownerRef)

	if err != nil {
		logger.Errorf("Failed to create namespace %s: %v", name, err)
		return err
	}

	logger.Infof("Created namespace %s", name)
	return nil
}

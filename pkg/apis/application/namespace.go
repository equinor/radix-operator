package application

import (
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
)

// createAppNamespace creates an app namespace with RadixRegistration as owner
func (app Application) createAppNamespace() error {
	registration := app.registration
	name := utils.GetAppNamespace(registration.Name)

	labels := map[string]string{
		kube.RadixAppLabel: registration.Name,
		kube.RadixEnvLabel: utils.AppNamespaceEnvName,
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

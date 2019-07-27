package application

import (
	"encoding/json"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
)

// createAppNamespace creates an app namespace with RadixRegistration as owner
func (app Application) createAppNamespace() error {
	registration := app.registration
	envName := "app"
	name := utils.GetAppNamespace(registration.Name)

	annotations, err := app.getNamespaceAnnotations()
	if err != nil {
		logger.Errorf("Failed to create namespace %s: %v", name, err)
		return err
	}

	labels := map[string]string{
		"radixApp":         registration.Name, // For backwards compatibility. Remove when cluster is migrated
		kube.RadixAppLabel: registration.Name,
		kube.RadixEnvLabel: envName,
	}

	ownerRef := app.getOwnerReference()
	err = app.kubeutil.ApplyNamespace(name, annotations, labels, ownerRef)

	if err != nil {
		logger.Errorf("Failed to create namespace %s: %v", name, err)
		return err
	}

	logger.Infof("Created namespace %s", name)
	return nil
}

// getNamespaceAnnotations Gets the namespace annotation for a RR
func (app Application) getNamespaceAnnotations() (map[string]string, error) {
	return GetNamespaceAnnotationsOfRegistration(app.registration)
}

// GetNamespaceAnnotationsOfRegistration Gets the namespace annotation for a RR
func GetNamespaceAnnotationsOfRegistration(registration *v1.RadixRegistration) (map[string]string, error) {
	adGroups, err := json.Marshal(registration.Spec.AdGroups)
	if err != nil {
		logger.Errorf("Failed to create namespace %s: %v", registration.Name, err)
		return nil, err
	}

	// Use this annotation to communicate with sync of RA and RD
	annotations := map[string]string{
		kube.AdGroupsAnnotation: string(adGroups),
	}

	return annotations, nil
}

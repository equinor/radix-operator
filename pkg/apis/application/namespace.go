package application

import (
	"encoding/json"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// createAppNamespace creates an app namespace with RadixRegistration as owner
func (app Application) createAppNamespace() error {
	registration := app.registration
	name := utils.GetAppNamespace(registration.Name)

	// This annotation is used for syncing any RA living below the app namespace
	annotations, err := app.getNamespaceAnnotations()
	if err != nil {
		logger.Errorf("Failed to create namespace %s: %v", name, err)
		return err
	}

	labels := map[string]string{
		"radixApp":         registration.Name, // For backwards compatibility. Remove when cluster is migrated
		kube.RadixAppLabel: registration.Name,
		kube.RadixEnvLabel: utils.AppNamespaceEnvName,
	}

	ownerRef := app.getOwnerReference()
	err = app.kubeutil.ApplyNamespace(name, annotations, labels, ownerRef)

	if err != nil {
		logger.Errorf("Failed to create namespace %s: %v", name, err)
		return err
	}

	logger.Debugf("Created namespace %s", name)
	return nil
}

// synchEnvironmentNamespaces Sync environment namespaces in order to communicate change to ad-groups with
// RA and RD controller
func (app Application) synchEnvironmentNamespaces() error {
	appName := app.registration.Name
	label := fmt.Sprintf("%s=%s", kube.RadixAppLabel, appName)

	annotations, err := app.getNamespaceAnnotations()
	if err != nil {
		logger.Errorf("Failed to sync environment namespace for app %s: %v", appName, err)
		return err
	}

	namespaces, _ := app.kubeclient.CoreV1().Namespaces().List(metav1.ListOptions{
		LabelSelector: label,
	})

	for _, namespace := range namespaces.Items {
		if namespace.Labels[kube.RadixEnvLabel] != utils.AppNamespaceEnvName {
			namespace.Annotations = annotations
			err = app.kubeutil.ApplyNamespace(namespace.Name, annotations, namespace.Labels, namespace.OwnerReferences)
		}
	}

	return nil
}

// getNamespaceAnnotations Gets the namespace annotation for a RR
func (app Application) getNamespaceAnnotations() (map[string]string, error) {
	return GetNamespaceAnnotationsOfRegistration(app.registration)
}

// GetNamespaceAnnotationsOfRegistration Gets the namespace annotation for a RR
func GetNamespaceAnnotationsOfRegistration(registration *v1.RadixRegistration) (map[string]string, error) {
	adGroups, err := GetAdGroups(registration)
	if err != nil {
		logger.Errorf("Failed to create namespace %s: %v", registration.Name, err)
		return nil, err
	}

	adGroupsAnnotation, err := json.Marshal(adGroups)
	if err != nil {
		logger.Errorf("Failed to create namespace %s: %v", registration.Name, err)
		return nil, err
	}

	// Use this annotation to communicate with sync of RA and RD
	annotations := map[string]string{
		kube.AdGroupsAnnotation: string(adGroupsAnnotation),
	}

	return annotations, nil
}

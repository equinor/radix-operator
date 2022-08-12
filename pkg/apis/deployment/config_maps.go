package deployment

import (
	"fmt"
	"github.com/equinor/radix-common/utils/errors"
	log "github.com/sirupsen/logrus"
)

func (deploy *Deployment) garbageCollectConfigMapsNoLongerInSpec() error {
	namespace := deploy.radixDeployment.Namespace

	// List configmaps
	envVarConfigMaps, err := deploy.kubeutil.ListEnvVarsConfigMaps(namespace)
	envVarMetadataConfigMaps, err := deploy.kubeutil.ListEnvVarsMetadataConfigMaps(namespace)
	cms := append(envVarConfigMaps, envVarMetadataConfigMaps...)

	if err != nil {
		return err
	}

	var errs []error

	// Iterate existing config maps. Check if any of them belong to components which no longer exist
	for _, cm := range cms {
		componentName, ok := RadixComponentNameFromComponentLabel(cm)
		if !ok {
			return fmt.Errorf("could not determine component name from labels in config map %s", cm.Name)
		}

		if !componentName.ExistInDeploymentSpecComponentList(deploy.radixDeployment) {
			log.Debugf("ConfigMap object %s in namespace %s belongs to deleted component %s, garbage collecting the configmap", cm.Name, namespace, componentName)
			err = deploy.kubeutil.DeleteConfigMap(namespace, cm.Name)
		}
		if err != nil {
			errs = append(errs, err)
		}

	}
	return errors.Concat(errs)
}

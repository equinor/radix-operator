package deployment

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"
)

func (deploy *Deployment) garbageCollectConfigMapsNoLongerInSpec(ctx context.Context) error {
	namespace := deploy.radixDeployment.Namespace

	// List env var config maps
	envVarConfigMaps, err := deploy.kubeutil.ListEnvVarsConfigMaps(ctx, namespace)
	if err != nil {
		return err
	}

	// List env var metadata config maps
	envVarMetadataConfigMaps, err := deploy.kubeutil.ListEnvVarsMetadataConfigMaps(ctx, namespace)
	if err != nil {
		return err
	}

	cms := append(envVarConfigMaps, envVarMetadataConfigMaps...)

	var errs []error

	// Iterate existing config maps. Check if any of them belong to components which no longer exist
	for _, cm := range cms {
		componentName, ok := RadixComponentNameFromComponentLabel(cm)
		if !ok {
			return fmt.Errorf("could not determine component name from labels in config map %s", cm.Name)
		}

		if !componentName.ExistInDeploymentSpecComponentList(deploy.radixDeployment) {
			log.Ctx(ctx).Debug().Msgf("ConfigMap object %s in namespace %s belongs to deleted component %s, garbage collecting the configmap", cm.Name, namespace, componentName)
			err = deploy.kubeutil.DeleteConfigMap(namespace, cm.Name)
		}
		if err != nil {
			errs = append(errs, err)
		}

	}
	return errors.Join(errs...)
}

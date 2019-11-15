package radixvalidators

import (
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/errors"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
)

const (
	minReplica = 0 // default is 1

	// MaxReplica Max number of replicas a deployment is allowed to have
	MaxReplica = 64
)

// InvalidNumberOfReplicaError Invalid number of replica
func InvalidNumberOfReplicaError(replica int) error {
	return fmt.Errorf("replicas %v must be between %v and %v", replica, minReplica, MaxReplica)
}

// CanRadixDeploymentBeInserted Checks if RD is valid
func CanRadixDeploymentBeInserted(client radixclient.Interface, deploy *radixv1.RadixDeployment) (bool, error) {
	// todo! ensure that all rules are valid
	errs := []error{}
	err := validateAppName(deploy.Name)
	if err != nil {
		errs = append(errs, err)
	}

	err = validateReplicas(deploy.Spec.Components)
	if err != nil {
		errs = append(errs, err)
	}

	err = validateComponentNames(deploy.Spec.Components)
	if err != nil {
		errs = append(errs, err)
	}

	err = validateRequiredResourceName("env name", deploy.Spec.Environment)
	if err != nil {
		errs = append(errs, err)
	}

	err = validateHPAConfigForRD(deploy.Spec)
	if err != nil {
		errs = append(errs, err)
	}

	if len(errs) <= 0 {
		return true, nil
	}
	return false, errors.Concat(errs)
}

func validateComponentNames(components []radixv1.RadixDeployComponent) error {
	for _, component := range components {
		err := validateRequiredResourceName("component name", component.Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func validateReplicas(components []radixv1.RadixDeployComponent) error {
	for _, component := range components {
		err := validateReplica(component.Replicas)
		if err != nil {
			return err
		}
	}
	return nil
}

func validateReplica(replica *int) error {
	if replica == nil {
		return nil
	}
	replicaValue := *replica
	if replicaValue > MaxReplica || replicaValue < minReplica {
		return InvalidNumberOfReplicaError(replicaValue)
	}
	return nil
}

func validateHPAConfigForRD(rdSpec radixv1.RadixDeploymentSpec) error {
	environment := rdSpec.Environment
	components := rdSpec.Components
	for _, component := range components {
		componentName := component.Name
		if component.HorizontalScaling == nil {
			continue
		}
		maxReplicas := component.HorizontalScaling.MaxReplicas
		minReplicas := component.HorizontalScaling.MinReplicas
		if maxReplicas == 0 {
			return MaxReplicasForHPANotSetOrZeroError(componentName, environment)
		}
		if minReplicas != nil && *minReplicas > maxReplicas {
			return MinReplicasGreaterThanMaxReplicasError(componentName, environment)
		}
	}
	return nil
}

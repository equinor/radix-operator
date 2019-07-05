package radixvalidators

import (
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
)

const (
	minReplica = 0 // default is 2

	// MaxReplica Max number of replicas a deployment is allowed to have
	MaxReplica = 64
)

// CanRadixDeploymentBeInserted Checks if RD is valid
func CanRadixDeploymentBeInserted(client radixclient.Interface, deploy *radixv1.RadixDeployment) (bool, error) {
	// todo! ensure that all rules are valid
	errors := []error{}
	err := validateAppName(deploy.Name)
	if err != nil {
		errors = append(errors, err)
	}

	err = validateReplicas(deploy.Spec.Components)
	if err != nil {
		errors = append(errors, err)
	}

	err = validateComponentNames(deploy.Spec.Components)
	if err != nil {
		errors = append(errors, err)
	}

	err = validateRequiredResourceName("env name", deploy.Spec.Environment)
	if err != nil {
		errors = append(errors, err)
	}

	if len(errors) <= 0 {
		return true, nil
	}
	return false, ConcatErrors(errors)
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

func validateReplica(replica int) error {
	if replica > MaxReplica || replica < minReplica {
		return fmt.Errorf("replicas %v must be between %v and %v", replica, minReplica, MaxReplica)
	}
	return nil
}

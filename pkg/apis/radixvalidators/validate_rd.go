package radixvalidators

import (
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/errors"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
)

const (
	minReplica = 0 // default is 2

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

func validateReplica(replica int) error {
	if replica > MaxReplica || replica < minReplica {
		return InvalidNumberOfReplicaError(replica)
	}
	return nil
}

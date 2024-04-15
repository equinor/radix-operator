package radixvalidators

import (
	stderrors "errors"
	"fmt"
	"strings"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

const (
	minReplica = 0 // default is 1

	// MaxReplica Max number of replicas a deployment is allowed to have
	MaxReplica = 64
)

func removeCharacters(input string, characters string) string {
	filter := func(r rune) rune {
		if !strings.ContainsRune(characters, r) {
			return r
		}
		return -1
	}
	return strings.Map(filter, input)
}

// InvalidNumberOfReplicaError Invalid number of replica
func InvalidNumberOfReplicaError(replica int) error {
	return fmt.Errorf("replicas %v must be between %v and %v", replica, minReplica, MaxReplica)
}

// GitTagsContainIllegalChars Git tags contain illegal characters
func GitTagsContainIllegalChars(gitTags string) error {
	illegalChars := "\"'$"
	strippedGitTags := removeCharacters(gitTags, illegalChars)
	if gitTags != strippedGitTags {
		return fmt.Errorf("git tags %s contained one or more illegal characters %s", gitTags, illegalChars)
	}
	return nil
}

// CanRadixDeploymentBeInserted Checks if RD is valid
func CanRadixDeploymentBeInserted(deploy *radixv1.RadixDeployment) error {
	// todo! ensure that all rules are valid
	errs := []error{}
	err := validateAppName(deploy.Name)
	if err != nil {
		errs = append(errs, err)
	}

	if err := validateDeployComponents(deploy); len(err) > 0 {
		errs = append(errs, err...)
	}

	if err := validateDeployJobComponents(deploy); len(err) > 0 {
		errs = append(errs, err...)
	}

	err = validateRequiredResourceName("env name", deploy.Spec.Environment)
	if err != nil {
		errs = append(errs, err)
	}

	return stderrors.Join(errs...)
}

func validateDeployComponents(deployment *radixv1.RadixDeployment) []error {
	errs := make([]error, 0)
	for _, component := range deployment.Spec.Components {
		if err := validateRequiredResourceName("component name", component.Name); err != nil {
			errs = append(errs, err)
		}

		if err := validateReplica(component.Replicas); err != nil {
			errs = append(errs, err)
		}

		if err := validateHPAConfigForRD(&component, deployment.Spec.Environment); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

func validateDeployJobComponents(deployment *radixv1.RadixDeployment) []error {
	errs := make([]error, 0)
	for _, job := range deployment.Spec.Jobs {
		if err := validateRequiredResourceName("job name", job.Name); err != nil {
			errs = append(errs, err)
		}

		if err := validateDeployJobPayload(&job); err != nil {
			errs = append(errs, err)
		}

		if err := validateDeployJobSchedulerPort(&job); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
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

func validateHPAConfigForRD(component *radixv1.RadixDeployComponent, environmentName string) error {
	if component.HorizontalScaling != nil {
		maxReplicas := component.HorizontalScaling.MaxReplicas
		minReplicas := component.HorizontalScaling.MinReplicas

		if maxReplicas == 0 {
			return MaxReplicasForHPANotSetOrZeroInEnvironmentErrorWithMessage(component.Name, environmentName)
		}
		if minReplicas != nil && *minReplicas > maxReplicas {
			return MinReplicasGreaterThanMaxReplicasInEnvironmentErrorWithMessage(component.Name, environmentName)
		}
	}

	return nil
}

func validateDeployJobSchedulerPort(job *radixv1.RadixDeployJobComponent) error {
	if job.SchedulerPort == nil {
		return SchedulerPortCannotBeEmptyForJobErrorWithMessage(job.Name)
	}

	return nil
}

func validateDeployJobPayload(job *radixv1.RadixDeployJobComponent) error {
	if job.Payload != nil && job.Payload.Path == "" {
		return PayloadPathCannotBeEmptyForJobErrorWithMessage(job.Name)
	}

	return nil
}

package radixvalidators

import (
	"github.com/pkg/errors"
)

var (
	ErrSchedulerPortCannotBeEmptyForJob             = errors.New("scheduler port cannot be empty for job")
	ErrPayloadPathCannotBeEmptyForJob               = errors.New("payload path cannot be empty for job")
	ErrMaxReplicasForHPANotSetOrZero                = errors.New("max replicas for hpa not set or zero")
	ErrMinReplicasGreaterThanMaxReplicas            = errors.New("min replicas greater than max replicas")
	ErrInvalidStringValueMaxLength                  = errors.New("invalid string value max length")
	ErrResourceNameCannotBeEmpty                    = errors.New("resource name cannot be empty")
	ErrInvalidLowerCaseAlphaNumericDashResourceName = errors.New("invalid lower case alpha numeric dash resource name")
)

// SchedulerPortCannotBeEmptyForJobErrorWithMessage Scheduler port cannot be empty for job
func SchedulerPortCannotBeEmptyForJobErrorWithMessage(jobName string) error {
	return errors.WithMessagef(ErrSchedulerPortCannotBeEmptyForJob, "scheduler port cannot be empty for %s", jobName)
}

// PayloadPathCannotBeEmptyForJobErrorWithMessage Payload path cannot be empty for job
func PayloadPathCannotBeEmptyForJobErrorWithMessage(jobName string) error {
	return errors.WithMessagef(ErrPayloadPathCannotBeEmptyForJob, "payload path cannot be empty for %s", jobName)
}

// MaxReplicasForHPANotSetOrZeroInEnvironmentErrorWithMessage Indicates that minReplicas of horizontalScaling is not set or set to 0
func MaxReplicasForHPANotSetOrZeroInEnvironmentErrorWithMessage(component, environment string) error {
	return errors.WithMessagef(ErrMaxReplicasForHPANotSetOrZero, "maxReplicas is not set or set to 0 for component %s in environment %s", component, environment)
}

// MinReplicasGreaterThanMaxReplicasInEnvironmentErrorWithMessage Indicates that minReplicas is greater than maxReplicas
func MinReplicasGreaterThanMaxReplicasInEnvironmentErrorWithMessage(component, environment string) error {
	return errors.WithMessagef(ErrMinReplicasGreaterThanMaxReplicas, "minReplicas is greater than maxReplicas for component %s in environment %s", component, environment)
}

// InvalidStringValueMaxLengthErrorWithMessage Invalid string value max length
func InvalidStringValueMaxLengthErrorWithMessage(resourceName, value string, maxValue int) error {
	return errors.WithMessagef(ErrInvalidStringValueMaxLength, "%s (\"%s\") max length is %d", resourceName, value, maxValue)
}

// ResourceNameCannotBeEmptyErrorWithMessage Resource name cannot be left empty
func ResourceNameCannotBeEmptyErrorWithMessage(resourceName string) error {
	return errors.WithMessagef(ErrResourceNameCannotBeEmpty, "%s is empty", resourceName)
}

// InvalidLowerCaseAlphaNumericDashResourceNameErrorWithMessage Invalid lower case alphanumeric, dash resource name error
func InvalidLowerCaseAlphaNumericDashResourceNameErrorWithMessage(resourceName, value string) error {
	return errors.WithMessagef(ErrInvalidLowerCaseAlphaNumericDashResourceName, "%s %s can only consist of lower case alphanumeric characters and '-'", resourceName, value)
}

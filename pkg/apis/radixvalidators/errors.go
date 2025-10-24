package radixvalidators

import (
	"github.com/pkg/errors"
)

var (
	ErrSchedulerPortCannotBeEmptyForJob  = errors.New("scheduler port cannot be empty for job")
	ErrPayloadPathCannotBeEmptyForJob    = errors.New("payload path cannot be empty for job")
	ErrMaxReplicasForHPANotSetOrZero     = errors.New("max replicas for hpa not set or zero")
	ErrMinReplicasGreaterThanMaxReplicas = errors.New("min replicas greater than max replicas")
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

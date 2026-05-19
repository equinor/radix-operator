package models

import (
	"fmt"

	radixhttp "github.com/equinor/radix-common/net/http"
)

// ScheduledJobPayloadNotFoundError Payload for the scheduled job not found
func ScheduledJobPayloadNotFoundError(appName, jobName string) error {
	return radixhttp.TypeMissingError(fmt.Sprintf("payload not found for job %s in app %s", jobName, appName), nil)
}

// ScheduledJobPayloadUnexpectedError Scheduled job has unexpected error
func ScheduledJobPayloadUnexpectedError(message, appName, jobName string) error {
	return radixhttp.UnexpectedError(fmt.Sprintf("error for job %s in app %s: %s", jobName, appName, message), nil)
}

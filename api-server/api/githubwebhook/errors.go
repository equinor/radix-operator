package githubwebhook

import "fmt"

var (
	notAGithubEventMessage                      = "Not a Github event"
	unmatchedRepoMessage                        = "Unable to match repo with any Radix application"
	multipleMatchingReposMessageWithoutAppName  = "Unable to match repo with unique Radix application without appName request parameter"
	unmatchedRepoMessageByAppName               = "Unable to match repo with unique Radix application by appName request parameter"
	unmatchedAppForMultipleMatchingReposMessage = "Unable to match repo with multiple Radix applications by appName request parameter"

	unhandledEventTypeMessage     = func(eventType string) string { return fmt.Sprintf("Unhandled event type %s", eventType) }
	webhookIncorrectConfiguration = func(appName string, err error) error {
		return fmt.Errorf("webhook is not configured correctly for Radix application %s. ApiError was: %w", appName, err)
	}
	webhookCorrectConfiguration = func(appName string) string {
		return fmt.Sprintf("Webhook is configured correctly with for Radix application %s", appName)
	}
	refDeletionPushEventUnsupportedMessage = func(refName string) string {
		return fmt.Sprintf("Deletion of %s not supported, aborting", refName)
	}
	createPipelineJobErrorMessage = func(appName string, apiError error) string {
		return fmt.Sprintf("Failed to create pipeline job for Radix application %s. ApiError was: %s", appName, apiError)
	}
	createPipelineJobSuccessMessage = func(jobName, appName, gitRefs, gitRefsType, commitID string) string {
		return fmt.Sprintf("Pipeline job %s created for Radix application %s on %s %s for commit %s", jobName, appName, gitRefsType, gitRefs, commitID)
	}
)

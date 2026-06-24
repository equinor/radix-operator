package githubwebhook

import (
	"errors"
	"fmt"
)

var (
	ErrNotAGithubEventMessage                      = errors.New("not a GitHub event")
	ErrUnmatchedRepoMessage                        = errors.New("unable to match repo with any Radix application")
	ErrMultipleMatchingReposMessageWithoutAppName  = errors.New("unable to match repo with unique Radix application without appName request parameter")
	ErrUnmatchedRepoMessageByAppName               = errors.New("unable to match repo with unique Radix application by appName request parameter")
	ErrUnmatchedAppForMultipleMatchingReposMessage = errors.New("unable to match repo with multiple Radix applications by appName request parameter")

	unhandledEventTypeMessage = func(eventType string) error {
		return fmt.Errorf("unhandled event type %s", eventType)
	}
	webhookIncorrectConfiguration = func(appName string, err error) error {
		return fmt.Errorf("webhook is not configured correctly for Radix application %s. ApiError was: %w", appName, err)
	}
	createPipelineJobErrorMessage = func(appName string, apiError error) error {
		return fmt.Errorf("failed to create pipeline job for Radix application %s. ApiError was: %w", appName, apiError)
	}
)

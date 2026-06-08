package githubwebhook

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/equinor/radix-operator/api-server/api/applications"
	"github.com/equinor/radix-operator/api-server/api/githubwebhook/metrics"
	"github.com/google/go-github/v72/github"
	"github.com/rs/zerolog"

	"github.com/equinor/radix-operator/api-server/models"
)

const rootPath = "/webhooks/github"
const githubEventHeader = "x-github-event"

var (
	notAGithubEventMessage                      = "Not a Github event"
	unhandledEventTypeMessage                   = func(eventType string) string { return fmt.Sprintf("Unhandled event type %s", eventType) }
	unmatchedRepoMessage                        = "Unable to match repo with any Radix application"
	multipleMatchingReposMessageWithoutAppName  = "Unable to match repo with unique Radix application without appName request parameter"
	unmatchedRepoMessageByAppName               = "Unable to match repo with unique Radix application by appName request parameter"
	unmatchedAppForMultipleMatchingReposMessage = "Unable to match repo with multiple Radix applications by appName request parameter"

	webhookIncorrectConfiguration = func(appName string, err error) string {
		return fmt.Sprintf("Webhook is not configured correctly for Radix application %s. ApiError was: %s", appName, err)
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

// WebhookResponse The response structure
type WebhookResponse struct {
	Ok      bool   `json:"ok"`
	Event   string `json:"event"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

type githubController struct {
	*models.DefaultController
	applicationHandlerFactory applications.ApplicationHandlerFactory
}

// NewGithubWebhookController Constructor
func NewGithubWebhookController(ah applications.ApplicationHandlerFactory) models.Controller {
	return &githubController{
		applicationHandlerFactory: ah,
	}
}

// GetRoutes List the supported routes of this handler
func (c *githubController) GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:                      rootPath,
			Method:                    "POST",
			HandlerFunc:               c.HandleGithubWebhook,
			AllowUnauthenticatedUsers: true,
		},
	}

	return routes
}
func (c *githubController) HandleGithubWebhook(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /webhooks/github webhook handleGithubWebhook
	// ---
	// summary: Handle GitHub webhook events
	// description: This endpoint receives GitHub webhook events and processes them accordingly.
	// parameters:
	//   - name: X-GitHub-Event
	//     in: header
	//     description: The type of GitHub event (e.g., push, pull_request)
	//     required: true
	//     type: string
	//   - name: appName
	//     in: query
	//     description: The name of the application associated with the webhook event
	//     required: true
	//     type: string
	// responses:
	//   '200':
	//     description: Webhook event processed successfully
	//   '400':
	//     description: Bad request, e.g., invalid payload or signature
	//   '500':
	//     description: Internal server error while processing the webhook event

	metrics.IncreaseAllCounter()

	event := github.WebHookType(r)

	writeErrorResponse := func(statusCode int, err error) {
		c.JSONResponseWithCode(w, r, statusCode, WebhookResponse{
			Ok:    false,
			Event: event,
			Error: err.Error(),
		})
	}

	writeSuccessResponse := func(statusCode int, message string) {
		zerolog.Ctx(r.Context()).Info().Msg(message)
		c.JSONResponseWithCode(w, r, statusCode, WebhookResponse{
			Ok:      true,
			Event:   event,
			Message: message,
		})
	}

	if len(strings.TrimSpace(event)) == 0 {
		metrics.IncreaseNotGithubEventCounter()
		c.ErrorResponse(w, r, errors.New(notAGithubEventMessage))
		return
	}

	// Need to parse webhook before validation because the secret is taken from the matching repo
	body, err := io.ReadAll(r.Body)
	if err != nil {
		metrics.IncreaseFailedParsingCounter()
		writeErrorResponse(http.StatusBadRequest, fmt.Errorf("could not parse webhook: err=%s ", err))
		return
	}

	payload, err := github.ParseWebHook(event, body)
	if err != nil {
		metrics.IncreaseFailedParsingCounter()
		writeErrorResponse(http.StatusBadRequest, fmt.Errorf("could not parse webhook: err=%s ", err))
		return
	}

	switch e := payload.(type) {
	case *github.PushEvent:
		gitRef, gitRefType := getGitRefWithType(e)
		commitID := getCommitID(e)
		sshURL := e.Repo.GetSSHURL()
		triggeredBy := getPushTriggeredBy(e)

		metrics.IncreasePushGithubEventTypeCounter(sshURL, gitRef, gitRefType, commitID)

		if isPushEventForRefDeletion(e) {
			writeSuccessResponse(http.StatusAccepted, refDeletionPushEventUnsupportedMessage(*e.Ref))
			return
		}

		applicationSummary, err := wh.getApplication(r, body, sshURL)
		if err != nil {
			metrics.IncreaseFailedCloneURLValidationCounter(sshURL)
			writeErrorResponse(http.StatusBadRequest, err)
			return
		}

		metrics.IncreasePushGithubEventTypeTriggerPipelineCounter(sshURL, gitRef, gitRefType, commitID, applicationSummary.Name)
		jobSummary, err := wh.apiServer.TriggerPipeline(r.Context(), applicationSummary.Name, gitRef, gitRefType, commitID, triggeredBy)
		if err != nil {
			if e, ok := err.(*radix.ApiError); ok && e.Code == 400 {
				writeSuccessResponse(http.StatusAccepted, createPipelineJobErrorMessage(applicationSummary.Name, err))
				return
			}
			metrics.IncreasePushGithubEventTypeFailedTriggerPipelineCounter(sshURL, gitRef, gitRefType, commitID)
			writeErrorResponse(http.StatusBadRequest, errors.New(createPipelineJobErrorMessage(applicationSummary.Name, err)))
			return
		}

		writeSuccessResponse(http.StatusOK, createPipelineJobSuccessMessage(jobSummary.Name, jobSummary.AppName, jobSummary.GetGitRefOrDefault(), jobSummary.GetGitRefTypeOrDefault(), jobSummary.CommitID))

	case *github.PingEvent:
		// sshURL := getSSHUrlFromPingURL(*e.Hook.URL)
		sshURL := e.Repo.GetSSHURL()
		metrics.IncreasePingGithubEventTypeCounter(sshURL)

		applicationSummary, err := wh.getApplication(c.Request, body, sshURL)
		if err != nil {
			metrics.IncreaseFailedCloneURLValidationCounter(sshURL)
			writeErrorResponse(http.StatusBadRequest, err)
			return
		}

		writeSuccessResponse(http.StatusOK, webhookCorrectConfiguration(applicationSummary.Name))

	default:
		metrics.IncreaseUnsupportedGithubEventTypeCounter()
		writeErrorResponse(http.StatusBadRequest, errors.New(unhandledEventTypeMessage(webhookEventType)))
		return
	}

	appName := r.URL.Query().Get("appName")
	c.ErrorResponse(w, r, errors.New("not implemented"))
}

func getApiGitRefType(gitRefsType string) string {
	switch gitRefsType {
	case "heads":
		return "branch"
	case "tags":
		return "tag"
	}
	return ""
}

func getCommitID(e *github.PushEvent) string {
	if e.Ref != nil && strings.HasPrefix(*e.Ref, "refs/tags/") && e.BaseRef == nil {
		// The property After has not an existing commit-ID, but other object ID
		// in the event for an "annotated tag", which can be created with a command
		// `git tag tag-name -m "annotation message"
		// https://git-scm.com/book/en/v2/Git-Basics-Tagging
		return *e.HeadCommit.ID
	}
	return *e.After
}

func getApplicationSummaryForSingleRegisteredApplication(appName string, applicationSummaries []*models.ApplicationSummary) (*models.ApplicationSummary, error) {
	if len(appName) == 0 || strings.EqualFold(applicationSummaries[0].Name, appName) {
		return applicationSummaries[0], nil
	}
	return nil, errors.New(unmatchedRepoMessageByAppName)
}

func getApplicationSummaryForMultipleRegisteredApplications(appName string, applicationSummaries []*models.ApplicationSummary) (*models.ApplicationSummary, error) {
	if len(appName) == 0 {
		return nil, errors.New(multipleMatchingReposMessageWithoutAppName)
	}
	for _, applicationSummary := range applicationSummaries {
		if strings.EqualFold(applicationSummary.Name, appName) {
			return applicationSummary, nil
		}
	}
	return nil, errors.New(unmatchedAppForMultipleMatchingReposMessage)
}

func getPushTriggeredBy(pushEvent *github.PushEvent) string {
	sender := pushEvent.GetSender()
	if sender != nil {
		return sender.GetLogin()
	}

	headCommit := pushEvent.GetHeadCommit()
	if headCommit != nil {
		author := headCommit.GetAuthor()
		if author != nil {
			return author.GetLogin()
		}
	}

	pusher := pushEvent.GetPusher()
	if pusher != nil {
		return pusher.GetLogin()
	}
	return ""
}

func getGitRefWithType(pushEvent *github.PushEvent) (string, string) {
	ref := strings.Split(*pushEvent.Ref, "/")
	gitRef := strings.Join(ref[2:], "/") // Remove refs/heads from ref
	gitRefType := ref[1]
	return gitRef, getApiGitRefType(gitRefType)
}

func isPushEventForRefDeletion(pushEvent *github.PushEvent) bool {
	// Deleted refers to the Ref in the Push event. See https://docs.github.com/en/developers/webhooks-and-events/webhooks/webhook-events-and-payloads#push
	if pushEvent.Deleted != nil {
		return *pushEvent.Deleted
	}
	return false
}

func validatePayload(header http.Header, payload []byte, sharedSecret []byte) error {
	signature := header.Get(github.SHA256SignatureHeader)
	contentType, _, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil {
		return err
	}

	if _, err = github.ValidatePayloadFromBody(contentType, bytes.NewBuffer(payload), signature, sharedSecret); err != nil {
		return err
	}

	return nil
}

package githubwebhook

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"

	applicationmodels "github.com/equinor/radix-operator/api-server/api/applications/models"
	"github.com/equinor/radix-operator/api-server/api/githubwebhook/metrics"
	"github.com/equinor/radix-operator/api-server/internal/pipelineservice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/google/go-github/v72/github"
	"github.com/rs/zerolog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/equinor/radix-operator/api-server/models"
)

const rootPath = "/webhooks/github"

var (
	notAGithubEventMessage                      = "Not a Github event"
	unhandledEventTypeMessage                   = func(eventType string) string { return fmt.Sprintf("Unhandled event type %s", eventType) }
	unmatchedRepoMessage                        = "Unable to match repo with any Radix application"
	multipleMatchingReposMessageWithoutAppName  = "Unable to match repo with unique Radix application without appName request parameter"
	unmatchedRepoMessageByAppName               = "Unable to match repo with unique Radix application by appName request parameter"
	unmatchedAppForMultipleMatchingReposMessage = "Unable to match repo with multiple Radix applications by appName request parameter"

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

// WebhookResponse The response structure
type WebhookResponse struct {
	Ok      bool   `json:"ok"`
	Event   string `json:"event"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

type githubController struct {
	*models.DefaultController
}

// NewGithubWebhookController Constructor
func NewGithubWebhookController() models.Controller {
	return &githubController{}
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
	//     required: false
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

	if len(strings.TrimSpace(event)) == 0 {
		metrics.IncreaseNotGithubEventCounter()
		c.ErrorResponse(w, r, errors.New(notAGithubEventMessage))
		return
	}

	// Need to parse webhook before validation because the secret is taken from the matching repo
	body, err := io.ReadAll(r.Body)
	if err != nil {
		metrics.IncreaseFailedParsingCounter()
		c.writeErrorResponse(w, r, http.StatusBadRequest, fmt.Errorf("could not parse webhook: err=%s ", err), event)
		return
	}

	payload, err := github.ParseWebHook(event, body)
	if err != nil {
		metrics.IncreaseFailedParsingCounter()
		c.writeErrorResponse(w, r, http.StatusBadRequest, fmt.Errorf("could not parse webhook: err=%s ", err), event)
		return
	}
	appName := r.URL.Query().Get("appName")

	switch e := payload.(type) {
	case *github.PushEvent:
		c.handlePushEvent(e, w, r, appName, accounts, body, event)
	case *github.PingEvent:
		c.handlePingEvent(e, w, r, appName, accounts, body, event)
	default:
		metrics.IncreaseUnsupportedGithubEventTypeCounter()
		c.writeErrorResponse(w, r, http.StatusBadRequest, errors.New(unhandledEventTypeMessage(event)), event)
		return
	}
}

func (c *githubController) handlePingEvent(e *github.PingEvent, w http.ResponseWriter, r *http.Request, appName string, accounts models.Accounts, body []byte, event string) {
	sshURL := e.Repo.GetSSHURL()
	metrics.IncreasePingGithubEventTypeCounter(sshURL)

	rr, err := getRadixRegistration(r.Context(), appName, sshURL, accounts.ServiceAccount.RadixClient)
	if err != nil {
		metrics.IncreaseFailedCloneURLValidationCounter(sshURL)
		c.writeErrorResponse(w, r, http.StatusBadRequest, err, event)
		return
	}
	err = validatePayload(r.Header, body, []byte(rr.Spec.SharedSecret))
	if err != nil {
		metrics.IncreaseFailedCloneURLValidationCounter(sshURL)
		c.writeErrorResponse(w, r, http.StatusBadRequest, webhookIncorrectConfiguration(rr.Name, err), event)
		return
	}

	c.writeSuccessResponse(w, r, http.StatusOK, webhookCorrectConfiguration(rr.Name), event)
}

func (c *githubController) handlePushEvent(e *github.PushEvent, w http.ResponseWriter, r *http.Request, appName string, accounts models.Accounts, body []byte, event string) {
	pipelineSvc := pipelineservice.New(accounts.ServiceAccount.RadixClient)
	gitRef, gitRefType := getGitRefWithType(e)
	commitID := getCommitID(e)
	sshURL := e.Repo.GetSSHURL()
	triggeredBy := getPushTriggeredBy(e)

	metrics.IncreasePushGithubEventTypeCounter(sshURL, gitRef, gitRefType, commitID)

	if isPushEventForRefDeletion(e) {
		c.writeSuccessResponse(w, r, http.StatusAccepted, refDeletionPushEventUnsupportedMessage(*e.Ref), event)
		return
	}

	rr, err := getRadixRegistration(r.Context(), appName, sshURL, accounts.ServiceAccount.RadixClient)
	if err != nil {
		metrics.IncreaseFailedCloneURLValidationCounter(sshURL)
		c.writeErrorResponse(w, r, http.StatusBadRequest, err, event)
		return
	}
	err = validatePayload(r.Header, body, []byte(rr.Spec.SharedSecret))
	if err != nil {
		metrics.IncreaseFailedCloneURLValidationCounter(sshURL)
		c.writeErrorResponse(w, r, http.StatusBadRequest, webhookIncorrectConfiguration(rr.Name, err), event)
		return
	}

	metrics.IncreasePushGithubEventTypeTriggerPipelineCounter(sshURL, gitRef, gitRefType, commitID, rr.Name)

	jobSummary, err := pipelineSvc.TriggerPipelineBuildDeploy(r.Context(), rr.Name, true, applicationmodels.PipelineParametersBuild{
		CommitID:    commitID,
		PushImage:   "true",
		TriggeredBy: triggeredBy,
		//Branch:      gitRef, //nolint:staticcheck
		GitRef: gitRef,
	})
	if err != nil {
		metrics.IncreasePushGithubEventTypeFailedTriggerPipelineCounter(sshURL, gitRef, gitRefType, commitID)
		c.writeErrorResponse(w, r, http.StatusBadRequest, errors.New(createPipelineJobErrorMessage(rr.Name, err)), event)
		log.Ctx(r.Context()).Error().Err(err).Msgf("Failed to create pipeline job for Radix application %s on %s %s for commit %s", rr.Name, gitRefType, gitRef, commitID)
		return
	}

	c.writeSuccessResponse(w, r, http.StatusOK, createPipelineJobSuccessMessage(jobSummary.Name, jobSummary.AppName, jobSummary.GitRef, jobSummary.GitRefType, jobSummary.CommitID), event)
}

func getRadixRegistration(ctx context.Context, appName, sshURL string, radixClient radixclient.Interface) (*radixv1.RadixRegistration, error) {
	radixRegistationList, err := radixClient.RadixV1().RadixRegistrations().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	filteredRegistrations := make([]radixv1.RadixRegistration, 0, len(radixRegistationList.Items))
	for _, rr := range radixRegistationList.Items {
		if strings.EqualFold(rr.Spec.CloneURL, sshURL) {
			filteredRegistrations = append(filteredRegistrations, rr)
		}
	}

	if len(filteredRegistrations) == 0 {
		return nil, errors.New(unmatchedRepoMessage)
	}
	if len(filteredRegistrations) == 1 {
		return getApplicationSummaryForSingleRegisteredApplication(appName, filteredRegistrations[0])
	}
	return getApplicationSummaryForMultipleRegisteredApplications(appName, filteredRegistrations)

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

func getApplicationSummaryForSingleRegisteredApplication(appName string, rr radixv1.RadixRegistration) (*radixv1.RadixRegistration, error) {
	if len(appName) == 0 || strings.EqualFold(rr.Name, appName) {
		return &rr, nil
	}
	return nil, errors.New(unmatchedRepoMessageByAppName)
}

func getApplicationSummaryForMultipleRegisteredApplications(appName string, rrs []radixv1.RadixRegistration) (*radixv1.RadixRegistration, error) {
	if len(appName) == 0 {
		return nil, errors.New(multipleMatchingReposMessageWithoutAppName)
	}
	for _, rr := range rrs {
		if strings.EqualFold(rr.Name, appName) {
			return &rr, nil
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

func (c *githubController) writeErrorResponse(w http.ResponseWriter, r *http.Request, statusCode int, err error, event string) {
	c.JSONResponseWithCode(w, r, statusCode, WebhookResponse{
		Ok:    false,
		Event: event,
		Error: err.Error(),
	})
}

func (c *githubController) writeSuccessResponse(w http.ResponseWriter, r *http.Request, statusCode int, message string, event string) {
	zerolog.Ctx(r.Context()).Info().Msg(message)
	c.JSONResponseWithCode(w, r, statusCode, WebhookResponse{
		Ok:      true,
		Event:   event,
		Message: message,
	})
}

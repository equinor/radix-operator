package githubwebhook

import (
	"bytes"
	"context"
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
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/google/go-github/v72/github"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	"github.com/equinor/radix-operator/api-server/models"
)

type githubController struct {
	*models.DefaultController
	eventRecorder record.EventRecorder
}

// NewGithubWebhookController Constructor
func NewGithubWebhookController(eventRecorder record.EventRecorder) models.Controller {
	return &githubController{eventRecorder: eventRecorder}
}

// GetRoutes List the supported routes of this handler
func (c *githubController) GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:                      "/webhooks/github",
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
		c.writeErrorResponse(w, r, http.StatusBadRequest, ErrNotAGithubEventMessage, "none")
		return
	}

	// Need to parse webhook before validation because the secret is taken from the matching repo
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 10*1024*1024)) // Limit request body to 10MB to prevent abuse
	if err != nil {
		metrics.IncreaseFailedParsingCounter()
		c.writeErrorResponse(w, r, http.StatusBadRequest, fmt.Errorf("could not parse webhook: %w", err), event)
		return
	}

	payload, err := github.ParseWebHook(event, body)
	if err != nil {
		metrics.IncreaseFailedParsingCounter()
		c.writeErrorResponse(w, r, http.StatusBadRequest, fmt.Errorf("could not parse webhook: %w", err), event)
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
		c.writeErrorResponse(w, r, http.StatusBadRequest, unhandledEventTypeMessage(event), event)
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

	c.writeSuccessResponse(w, r, http.StatusOK, fmt.Sprintf("Webhook is configured correctly for Radix application %s", rr.Name), event)
}

func (c *githubController) handlePushEvent(e *github.PushEvent, w http.ResponseWriter, r *http.Request, appName string, accounts models.Accounts, body []byte, event string) {
	pipelineSvc := pipelineservice.New(accounts.ServiceAccount.RadixClient)
	gitRef, gitRefType, err := getGitRefWithType(e)
	if err != nil {
		metrics.IncreaseFailedParsingCounter()
		c.writeErrorResponse(w, r, http.StatusBadRequest, fmt.Errorf("could not parse git ref: %w", err), event)
		return
	}
	commitID, err := getCommitID(e)
	if err != nil {
		metrics.IncreaseFailedParsingCounter()
		c.writeErrorResponse(w, r, http.StatusBadRequest, fmt.Errorf("could not parse commit ID: %w", err), event)
		return
	}
	sshURL := e.Repo.GetSSHURL()
	triggeredBy := getPushTriggeredBy(e)

	metrics.IncreasePushGithubEventTypeCounter(sshURL, gitRef, gitRefType, commitID)

	if isPushEventForRefDeletion(e) {
		c.writeSuccessResponse(w, r, http.StatusAccepted, fmt.Sprintf("Deletion of %s not supported, aborting", *e.Ref), event)
		return
	}

	rr, err := getRadixRegistration(r.Context(), appName, sshURL, accounts.ServiceAccount.RadixClient)
	if err != nil {
		metrics.IncreaseFailedCloneURLValidationCounter(sshURL)
		c.recordPushEventFailure(appName, err)
		c.writeErrorResponse(w, r, http.StatusBadRequest, err, event)
		return
	}
	err = validatePayload(r.Header, body, []byte(rr.Spec.SharedSecret))
	if err != nil {
		metrics.IncreaseFailedPayloadValidationCounter(rr.Name)
		c.recordPushEventFailure(rr.Name, err)
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
		c.recordPushEventFailure(rr.Name, err)
		c.writeErrorResponse(w, r, http.StatusBadRequest, createPipelineJobErrorMessage(rr.Name, err), event)
		log.Ctx(r.Context()).Error().Err(err).Msgf("Failed to create pipeline job for Radix application %s on %s %s for commit %s", rr.Name, gitRefType, gitRef, commitID)
		return
	}

	c.recordPushEventSuccess(rr, jobSummary.Name, gitRefType, gitRef, commitID)
	c.writeSuccessResponse(w, r, http.StatusOK, fmt.Sprintf("Pipeline job %s created for Radix application %s on %s %s for commit %s", jobSummary.Name, jobSummary.AppName, jobSummary.GitRefType, jobSummary.GitRef, jobSummary.CommitID), event)
}

// recordPushEventSuccess emits a Kubernetes event in the application namespace when a push event
// has successfully triggered a pipeline job.
func (c *githubController) recordPushEventSuccess(rr *radixv1.RadixRegistration, jobName, gitRefType, gitRef, commitID string) {
	if c.eventRecorder == nil {
		return
	}
	c.eventRecorder.Eventf(radixApplicationEventTarget(rr.Name), corev1.EventTypeNormal, "GithubPushReceived",
		"Pipeline job %s created for %s %s (commit %s)", jobName, gitRefType, gitRef, commitID)
}

// recordPushEventFailure emits a Warning Kubernetes event in the application namespace when a
// push event could not be processed for a known Radix application.
func (c *githubController) recordPushEventFailure(appName string, err error) {
	if c.eventRecorder == nil {
		return
	}
	if len(appName) == 0 {
		log.Warn().Err(err).Msg("Failed to process GitHub push event for unknown Radix application")
		return
	}

	c.eventRecorder.Eventf(radixApplicationEventTarget(appName), corev1.EventTypeWarning, "GithubPushFailed",
		"Failed to process GitHub push event: %s", err.Error())
}

// radixApplicationEventTarget returns a RadixApplication object placed in the application
// namespace (<appName>-app), used as the involved object for Kubernetes events emitted from the
// GitHub webhook handler.
func radixApplicationEventTarget(appName string) *radixv1.RadixApplication {
	return &radixv1.RadixApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: operatorutils.GetAppNamespace(appName),
		},
	}
}

func getRadixRegistration(ctx context.Context, appName, sshURL string, radixClient radixclient.Interface) (*radixv1.RadixRegistration, error) {
	if len(appName) > 0 {
		rr, err := radixClient.RadixV1().RadixRegistrations().Get(ctx, appName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get Radix registration for application %s: %w", appName, err)
		}
		if !strings.EqualFold(rr.Spec.CloneURL, sshURL) {
			return nil, ErrUnmatchedAppByCloneUrlMessage
		}
		return rr, nil
	}

	// If we dont have a appName, there must be exactly 1 matching Radix application for the repo
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
	if len(filteredRegistrations) == 1 {
		return &filteredRegistrations[0], nil
	}

	if len(filteredRegistrations) == 0 {
		return nil, ErrUnmatchedRepoMessage
	}
	return nil, ErrMultipleMatchingReposMessageWithoutAppName

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

func getCommitID(e *github.PushEvent) (string, error) {
	if e == nil {
		return "", fmt.Errorf("push event is nil")
	}

	if e.Ref != nil && strings.HasPrefix(*e.Ref, "refs/tags/") && e.BaseRef == nil && e.HeadCommit != nil && e.HeadCommit.ID != nil {
		// The property After has not an existing commit-ID, but other object ID
		// in the event for an "annotated tag", which can be created with a command
		// `git tag tag-name -m "annotation message"
		// https://git-scm.com/book/en/v2/Git-Basics-Tagging
		return *e.HeadCommit.ID, nil
	}

	if e.After == nil {
		return "", fmt.Errorf("after property is nil in push event")
	}

	return *e.After, nil
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

func getGitRefWithType(pushEvent *github.PushEvent) (string, string, error) {
	if pushEvent.Ref == nil {
		return "", "", fmt.Errorf("ref is nil in push event")
	}

	ref := strings.Split(*pushEvent.Ref, "/")
	if len(ref) < 3 {
		return "", "", fmt.Errorf("invalid ref format: %s", *pushEvent.Ref)
	}
	gitRef := strings.Join(ref[2:], "/") // Remove refs/heads from ref
	gitRefType := ref[1]
	return gitRef, getApiGitRefType(gitRefType), nil
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

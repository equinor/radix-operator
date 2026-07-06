package githubwebhook

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"

	applicationmodels "github.com/equinor/radix-operator/api-server/api/applications/models"
	"github.com/equinor/radix-operator/api-server/api/githubwebhook/metrics"
	"github.com/equinor/radix-operator/api-server/internal/pipelineservice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/google/go-github/v72/github"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/equinor/radix-operator/api-server/models"
)

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
	sharedSecret, err := getWebhookSharedSecret(r.Context(), rr, accounts)
	if err != nil {
		metrics.IncreaseFailedCloneURLValidationCounter(sshURL)
		c.writeErrorResponse(w, r, http.StatusBadRequest, webhookIncorrectConfiguration(rr.Name, err), event)
		return
	}
	err = validatePayload(r.Header, body, sharedSecret)
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
		c.writeErrorResponse(w, r, http.StatusBadRequest, err, event)
		return
	}
	sharedSecret, err := getWebhookSharedSecret(r.Context(), rr, accounts)
	if err != nil {
		metrics.IncreaseFailedCloneURLValidationCounter(sshURL)
		c.writeErrorResponse(w, r, http.StatusBadRequest, webhookIncorrectConfiguration(rr.Name, err), event)
		return
	}
	err = validatePayload(r.Header, body, sharedSecret)
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
		c.writeErrorResponse(w, r, http.StatusBadRequest, createPipelineJobErrorMessage(rr.Name, err), event)
		log.Ctx(r.Context()).Error().Err(err).Msgf("Failed to create pipeline job for Radix application %s on %s %s for commit %s", rr.Name, gitRefType, gitRef, commitID)
		return
	}

	c.writeSuccessResponse(w, r, http.StatusOK, fmt.Sprintf("Pipeline job %s created for Radix application %s on %s %s for commit %s", jobSummary.Name, jobSummary.AppName, jobSummary.GitRefType, jobSummary.GitRef, jobSummary.CommitID), event)
}

func getRadixRegistration(ctx context.Context, appName, sshURL string, radixClient radixclient.Interface) (*radixv1.RadixRegistration, error) {
	if len(appName) > 0 {
		rr, err := radixClient.RadixV1().RadixRegistrations().Get(ctx, appName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get Radix registration for application %s: %w", appName, err)
		}
		if !strings.EqualFold(rr.Spec.CloneURL, sshURL) {
			return nil, ErrUnmatchedRepoMessageByAppName
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

func getWebhookSharedSecret(ctx context.Context, rr *radixv1.RadixRegistration, accounts models.Accounts) ([]byte, error) {
	secret, err := accounts.ServiceAccount.Client.CoreV1().Secrets(operatorutils.GetAppNamespace(rr.Name)).Get(ctx, defaults.WebhookSharedSecretName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return []byte(rr.Spec.SharedSecret), nil // TODO: When all Secrets have been created and seeded, remove the deprecated Spec.SharedSecret field from RadixRegistration and stop seeding from it.
		}

		return nil, fmt.Errorf("failed to get webhook secret %s: %w", defaults.WebhookSharedSecretName, err)
	}

	encodedSecret, ok := secret.Data[defaults.WebhookSharedSecretKey]
	if !ok || len(encodedSecret) == 0 {
		return nil, fmt.Errorf("secret %s is missing key %s", defaults.WebhookSharedSecretName, defaults.WebhookSharedSecretKey)
	}

	decodedSecret, err := base64.StdEncoding.DecodeString(string(encodedSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to decode secret %s key %s: %w", defaults.WebhookSharedSecretName, defaults.WebhookSharedSecretKey, err)
	}

	if len(decodedSecret) == 0 {
		decodedSecret = []byte(rr.Spec.SharedSecret) // TODO: When all Secrets have been created and seeded, remove the deprecated Spec.SharedSecret field from RadixRegistration and stop seeding from it.
	}

	if len(strings.TrimSpace(string(decodedSecret))) == 0 {
		return nil, fmt.Errorf("secret %s key %s contains empty value", defaults.WebhookSharedSecretName, defaults.WebhookSharedSecretKey)
	}

	return decodedSecret, nil
}

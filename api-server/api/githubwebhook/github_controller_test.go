package githubwebhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	certclientfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	"github.com/equinor/radix-operator/api-server/api/router"
	controllertest "github.com/equinor/radix-operator/api-server/api/test"
	authnmock "github.com/equinor/radix-operator/api-server/api/utils/token/mock"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/google/go-github/v72/github"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

const (
	sharedSecret  = "AnySharedSecret"
	sshURL        = "git@github.com:equinor/my-app.git"
	appName       = "my-app"
	configBranch  = "main"
	webhookPath   = "/api/v1/webhooks/github"
	pushEventType = "push"
	pingEventType = "ping"
)

// executeWebhookRequest builds an HTTP request with the supplied body and headers and
// serves it through the same router and middleware stack used by the api-server.
func executeWebhookRequest(t *testing.T, radixClient *radixfake.Clientset, url string, headers map[string]string, body []byte) *httptest.ResponseRecorder {
	t.Helper()

	kubeClient := kubefake.NewSimpleClientset() //nolint:staticcheck
	kedaClient := kedafake.NewSimpleClientset()
	secretProviderClient := secretproviderfake.NewSimpleClientset()
	certClient := certclientfake.NewSimpleClientset()
	kubeUtil := controllertest.NewKubeUtilMock(kubeClient, radixClient, kedaClient, secretProviderClient, certClient, nil)

	mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
	handler := router.NewAPIHandler(mockValidator, kubeUtil, NewGithubWebhookController())

	req := httptest.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

// signPayload returns the GitHub style HMAC-SHA256 signature for the given payload and secret.
func signPayload(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func pushEventBody(t *testing.T, ref, after string, deleted bool) []byte {
	t.Helper()
	event := github.PushEvent{
		Ref:     github.Ptr(ref),
		After:   github.Ptr(after),
		Deleted: github.Ptr(deleted),
		Repo:    &github.PushEventRepository{SSHURL: github.Ptr(sshURL)},
		Sender:  &github.User{Login: github.Ptr("some-user")},
	}
	body, err := json.Marshal(event)
	require.NoError(t, err)
	return body
}

func pingEventBody(t *testing.T, repoSSHURL string) []byte {
	t.Helper()
	event := github.PingEvent{
		Repo: &github.Repository{SSHURL: github.Ptr(repoSSHURL)},
	}
	body, err := json.Marshal(event)
	require.NoError(t, err)
	return body
}

func registerApp(t *testing.T, radixClient *radixfake.Clientset) {
	t.Helper()
	rr := operatorutils.NewRegistrationBuilder().
		WithName(appName).
		WithCloneURL(sshURL).
		WithSharedSecret(sharedSecret).
		WithConfigBranch(configBranch).
		BuildRR()
	_, err := radixClient.RadixV1().RadixRegistrations().Create(t.Context(), rr, metav1.CreateOptions{})
	require.NoError(t, err)
}

// createRegistration registers an application with the given name that shares the
// package level sshURL clone URL, used to simulate multiple apps matching one repo.
func createRegistration(t *testing.T, radixClient *radixfake.Clientset, name string) {
	t.Helper()
	rr := operatorutils.NewRegistrationBuilder().
		WithName(name).
		WithCloneURL(sshURL).
		WithSharedSecret(sharedSecret).
		WithConfigBranch(configBranch).
		BuildRR()
	_, err := radixClient.RadixV1().RadixRegistrations().Create(t.Context(), rr, metav1.CreateOptions{})
	require.NoError(t, err)
}

func decodeWebhookResponse(t *testing.T, rr *httptest.ResponseRecorder) WebhookResponse {
	t.Helper()
	var resp WebhookResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	return resp
}

func TestNewGithubWebhookController_GetRoutes(t *testing.T) {
	controller := NewGithubWebhookController()
	routes := controller.GetRoutes()

	require.Len(t, routes, 1)
	assert.Equal(t, rootPath, routes[0].Path)
	assert.Equal(t, http.MethodPost, routes[0].Method)
	assert.True(t, routes[0].AllowUnauthenticatedUsers)
	assert.NotNil(t, routes[0].HandlerFunc)
}

func TestHandleGithubWebhook_NotAGithubEvent(t *testing.T) {
	radixClient := radixfake.NewSimpleClientset() //nolint:staticcheck

	rr := executeWebhookRequest(t, radixClient, webhookPath, map[string]string{
		"Content-Type": "application/json",
	}, []byte("{}"))

	assert.Equal(t, http.StatusBadRequest, rr.Result().StatusCode)
}

func TestHandleGithubWebhook_UnhandledEventType(t *testing.T) {
	radixClient := radixfake.NewSimpleClientset() //nolint:staticcheck

	rr := executeWebhookRequest(t, radixClient, webhookPath, map[string]string{
		"Content-Type":   "application/json",
		"X-GitHub-Event": "pull_request",
	}, []byte("{}"))

	assert.Equal(t, http.StatusBadRequest, rr.Result().StatusCode)
	resp := decodeWebhookResponse(t, rr)
	assert.False(t, resp.Ok)
	assert.Equal(t, "pull_request", resp.Event)
	assert.Equal(t, unhandledEventTypeMessage("pull_request"), resp.Error)
}

func TestHandleGithubWebhook_Ping_MatchingRepo(t *testing.T) {
	radixClient := radixfake.NewSimpleClientset() //nolint:staticcheck
	registerApp(t, radixClient)

	body := pingEventBody(t, sshURL)
	rr := executeWebhookRequest(t, radixClient, webhookPath+"?appName="+appName, map[string]string{
		"Content-Type":               "application/json",
		"X-GitHub-Event":             pingEventType,
		github.SHA256SignatureHeader: signPayload(body, sharedSecret),
	}, body)

	assert.Equal(t, http.StatusOK, rr.Result().StatusCode)
	resp := decodeWebhookResponse(t, rr)
	assert.True(t, resp.Ok)
	assert.Equal(t, pingEventType, resp.Event)
	assert.Equal(t, webhookCorrectConfiguration(appName), resp.Message)
}

func TestHandleGithubWebhook_Ping_UnmatchedRepo(t *testing.T) {
	radixClient := radixfake.NewSimpleClientset() //nolint:staticcheck

	body := pingEventBody(t, sshURL)
	rr := executeWebhookRequest(t, radixClient, webhookPath, map[string]string{
		"Content-Type":   "application/json",
		"X-GitHub-Event": pingEventType,
	}, body)

	assert.Equal(t, http.StatusBadRequest, rr.Result().StatusCode)
	resp := decodeWebhookResponse(t, rr)
	assert.False(t, resp.Ok)
	assert.Equal(t, unmatchedRepoMessage, resp.Error)
}

func TestHandleGithubWebhook_Push_RefDeletion(t *testing.T) {
	radixClient := radixfake.NewSimpleClientset() //nolint:staticcheck
	registerApp(t, radixClient)

	body := pushEventBody(t, "refs/heads/main", "0000000000000000000000000000000000000000", true)
	rr := executeWebhookRequest(t, radixClient, webhookPath+"?appName="+appName, map[string]string{
		"Content-Type":   "application/json",
		"X-GitHub-Event": pushEventType,
	}, body)

	assert.Equal(t, http.StatusAccepted, rr.Result().StatusCode)
	resp := decodeWebhookResponse(t, rr)
	assert.True(t, resp.Ok)
	assert.Equal(t, refDeletionPushEventUnsupportedMessage("refs/heads/main"), resp.Message)
}

func TestHandleGithubWebhook_Push_UnmatchedRepo(t *testing.T) {
	radixClient := radixfake.NewSimpleClientset() //nolint:staticcheck

	body := pushEventBody(t, "refs/heads/main", "abc123", false)
	rr := executeWebhookRequest(t, radixClient, webhookPath, map[string]string{
		"Content-Type":   "application/json",
		"X-GitHub-Event": pushEventType,
	}, body)

	assert.Equal(t, http.StatusBadRequest, rr.Result().StatusCode)
	resp := decodeWebhookResponse(t, rr)
	assert.False(t, resp.Ok)
	assert.Equal(t, unmatchedRepoMessage, resp.Error)
}

func TestHandleGithubWebhook_Push_InvalidSignature(t *testing.T) {
	radixClient := radixfake.NewSimpleClientset() //nolint:staticcheck
	registerApp(t, radixClient)

	body := pushEventBody(t, "refs/heads/main", "abc123", false)
	rr := executeWebhookRequest(t, radixClient, webhookPath+"?appName="+appName, map[string]string{
		"Content-Type":               "application/json",
		"X-GitHub-Event":             pushEventType,
		github.SHA256SignatureHeader: signPayload(body, "WrongSecret"),
	}, body)

	assert.Equal(t, http.StatusBadRequest, rr.Result().StatusCode)
	resp := decodeWebhookResponse(t, rr)
	assert.False(t, resp.Ok)
	assert.NotEmpty(t, resp.Error)
}

func TestHandleGithubWebhook_Push_TriggersPipeline(t *testing.T) {
	radixClient := radixfake.NewSimpleClientset() //nolint:staticcheck
	registerApp(t, radixClient)

	commitID := "0123456789abcdef0123456789abcdef01234567"
	body := pushEventBody(t, "refs/heads/"+configBranch, commitID, false)
	rr := executeWebhookRequest(t, radixClient, webhookPath+"?appName="+appName, map[string]string{
		"Content-Type":               "application/json",
		"X-GitHub-Event":             pushEventType,
		github.SHA256SignatureHeader: signPayload(body, sharedSecret),
	}, body)

	assert.Equal(t, http.StatusOK, rr.Result().StatusCode)
	resp := decodeWebhookResponse(t, rr)
	assert.True(t, resp.Ok)
	assert.Equal(t, pushEventType, resp.Event)

	jobs, err := radixClient.RadixV1().RadixJobs(operatorutils.GetAppNamespace(appName)).List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, jobs.Items, 1)
	job := jobs.Items[0]
	assert.Equal(t, radixv1.BuildDeploy, job.Spec.PipeLineType)
	assert.Equal(t, commitID, job.Spec.Build.CommitID)
	assert.Equal(t, configBranch, job.Spec.Build.GitRef)
}

func TestHandleGithubWebhook_Ping_IncorrectSecret(t *testing.T) {
	radixClient := radixfake.NewSimpleClientset() //nolint:staticcheck
	registerApp(t, radixClient)

	body := pingEventBody(t, sshURL)
	rr := executeWebhookRequest(t, radixClient, webhookPath+"?appName="+appName, map[string]string{
		"Content-Type":               "application/json",
		"X-GitHub-Event":             pingEventType,
		github.SHA256SignatureHeader: signPayload(body, "IncorrectSecret"),
	}, body)

	assert.Equal(t, http.StatusBadRequest, rr.Result().StatusCode)
	resp := decodeWebhookResponse(t, rr)
	assert.False(t, resp.Ok)
	assert.Equal(t, webhookIncorrectConfiguration(appName, errors.New("payload signature check failed")).Error(), resp.Error)
}

func TestHandleGithubWebhook_Ping_MultipleReposWithoutAppName(t *testing.T) {
	radixClient := radixfake.NewSimpleClientset() //nolint:staticcheck
	createRegistration(t, radixClient, "app1")
	createRegistration(t, radixClient, "app2")

	body := pingEventBody(t, sshURL)
	rr := executeWebhookRequest(t, radixClient, webhookPath, map[string]string{
		"Content-Type":   "application/json",
		"X-GitHub-Event": pingEventType,
	}, body)

	assert.Equal(t, http.StatusBadRequest, rr.Result().StatusCode)
	resp := decodeWebhookResponse(t, rr)
	assert.False(t, resp.Ok)
	assert.Equal(t, multipleMatchingReposMessageWithoutAppName, resp.Error)
}

func TestHandleGithubWebhook_Push_RepoMatching(t *testing.T) {
	const commitID = "0123456789abcdef0123456789abcdef01234567"

	scenarios := []struct {
		name           string
		appNames       []string
		queryAppName   string
		expectedStatus int
		expectedError  string
		expectJob      bool
	}{
		{
			name:           "multiple apps without appName",
			appNames:       []string{"app1", "app2"},
			queryAppName:   "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  multipleMatchingReposMessageWithoutAppName,
		},
		{
			name:           "multiple apps unmatched appName",
			appNames:       []string{"app1", "app2"},
			queryAppName:   "app3",
			expectedStatus: http.StatusBadRequest,
			expectedError:  unmatchedAppForMultipleMatchingReposMessage,
		},
		{
			name:           "single app unmatched appName",
			appNames:       []string{"app1"},
			queryAppName:   "app3",
			expectedStatus: http.StatusBadRequest,
			expectedError:  unmatchedRepoMessageByAppName,
		},
		{
			name:           "single app matched appName",
			appNames:       []string{"app1"},
			queryAppName:   "app1",
			expectedStatus: http.StatusOK,
			expectJob:      true,
		},
		{
			name:           "multiple apps matched appName",
			appNames:       []string{"app1", "app2"},
			queryAppName:   "app2",
			expectedStatus: http.StatusOK,
			expectJob:      true,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			radixClient := radixfake.NewSimpleClientset() //nolint:staticcheck
			for _, name := range scenario.appNames {
				createRegistration(t, radixClient, name)
			}

			body := pushEventBody(t, "refs/heads/"+configBranch, commitID, false)
			url := webhookPath
			if scenario.queryAppName != "" {
				url += "?appName=" + scenario.queryAppName
			}
			rr := executeWebhookRequest(t, radixClient, url, map[string]string{
				"Content-Type":               "application/json",
				"X-GitHub-Event":             pushEventType,
				github.SHA256SignatureHeader: signPayload(body, sharedSecret),
			}, body)

			assert.Equal(t, scenario.expectedStatus, rr.Result().StatusCode)
			resp := decodeWebhookResponse(t, rr)
			assert.Equal(t, scenario.expectedError, resp.Error)

			if scenario.expectJob {
				jobs, err := radixClient.RadixV1().RadixJobs(operatorutils.GetAppNamespace(scenario.queryAppName)).List(t.Context(), metav1.ListOptions{})
				require.NoError(t, err)
				assert.Len(t, jobs.Items, 1)
			}
		})
	}
}

func TestHandleGithubWebhook_Push_TriggerPipelineError(t *testing.T) {
	radixClient := radixfake.NewSimpleClientset() //nolint:staticcheck
	registerApp(t, radixClient)

	// Push to a non-config branch with no RadixApplication, so the pipeline
	// service fails to resolve target environments and returns an error.
	body := pushEventBody(t, "refs/heads/feature-branch", "abc123def456", false)
	rr := executeWebhookRequest(t, radixClient, webhookPath+"?appName="+appName, map[string]string{
		"Content-Type":               "application/json",
		"X-GitHub-Event":             pushEventType,
		github.SHA256SignatureHeader: signPayload(body, sharedSecret),
	}, body)

	assert.Equal(t, http.StatusBadRequest, rr.Result().StatusCode)
	resp := decodeWebhookResponse(t, rr)
	assert.False(t, resp.Ok)
	assert.Contains(t, resp.Error, "Failed to create pipeline job for Radix application "+appName)
}

func TestGetGitRefWithType(t *testing.T) {
	scenarios := []struct {
		ref            string
		expectedGitRef string
		expectedType   string
	}{
		{ref: "refs/tags/v1.0.2", expectedGitRef: "v1.0.2", expectedType: "tag"},
		{ref: "refs/heads/master", expectedGitRef: "master", expectedType: "branch"},
		{ref: "refs/heads/feature/RA-326-TestBranch", expectedGitRef: "feature/RA-326-TestBranch", expectedType: "branch"},
		{ref: "refs/heads/hotfix/api/refs/heads/fix1", expectedGitRef: "hotfix/api/refs/heads/fix1", expectedType: "branch"},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.ref, func(t *testing.T) {
			gitRef, gitRefType := getGitRefWithType(&github.PushEvent{Ref: github.Ptr(scenario.ref)})
			assert.Equal(t, scenario.expectedGitRef, gitRef)
			assert.Equal(t, scenario.expectedType, gitRefType)
		})
	}
}

func TestGetApiGitRefType(t *testing.T) {
	assert.Equal(t, "branch", getApiGitRefType("heads"))
	assert.Equal(t, "tag", getApiGitRefType("tags"))
	assert.Equal(t, "", getApiGitRefType("unknown"))
}

func TestGetCommitID(t *testing.T) {
	t.Run("push event returns After", func(t *testing.T) {
		commitID := getCommitID(&github.PushEvent{
			Ref:   github.Ptr("refs/heads/master"),
			After: github.Ptr("after-commit-id"),
		})
		assert.Equal(t, "after-commit-id", commitID)
	})

	t.Run("annotated tag returns head commit ID", func(t *testing.T) {
		commitID := getCommitID(&github.PushEvent{
			Ref:        github.Ptr("refs/tags/v1"),
			After:      github.Ptr("annotated-tag-object-id"),
			HeadCommit: &github.HeadCommit{ID: github.Ptr("head-commit-id")},
		})
		assert.Equal(t, "head-commit-id", commitID)
	})

	t.Run("lightweight tag with base ref returns After", func(t *testing.T) {
		commitID := getCommitID(&github.PushEvent{
			Ref:     github.Ptr("refs/tags/v1"),
			After:   github.Ptr("after-commit-id"),
			BaseRef: github.Ptr("refs/heads/master"),
		})
		assert.Equal(t, "after-commit-id", commitID)
	})
}

func TestGetPushTriggeredBy(t *testing.T) {
	t.Run("uses sender login", func(t *testing.T) {
		triggeredBy := getPushTriggeredBy(&github.PushEvent{
			Sender:     &github.User{Login: github.Ptr("sender-user")},
			HeadCommit: &github.HeadCommit{Author: &github.CommitAuthor{Login: github.Ptr("author-user")}},
			Pusher:     &github.CommitAuthor{Login: github.Ptr("pusher-user")},
		})
		assert.Equal(t, "sender-user", triggeredBy)
	})

	t.Run("falls back to head commit author", func(t *testing.T) {
		triggeredBy := getPushTriggeredBy(&github.PushEvent{
			HeadCommit: &github.HeadCommit{Author: &github.CommitAuthor{Login: github.Ptr("author-user")}},
			Pusher:     &github.CommitAuthor{Login: github.Ptr("pusher-user")},
		})
		assert.Equal(t, "author-user", triggeredBy)
	})

	t.Run("falls back to pusher", func(t *testing.T) {
		triggeredBy := getPushTriggeredBy(&github.PushEvent{
			Pusher: &github.CommitAuthor{Login: github.Ptr("pusher-user")},
		})
		assert.Equal(t, "pusher-user", triggeredBy)
	})

	t.Run("returns empty when nothing set", func(t *testing.T) {
		triggeredBy := getPushTriggeredBy(&github.PushEvent{})
		assert.Empty(t, triggeredBy)
	})
}

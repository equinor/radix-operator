package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	sshURLLabel      = "radix_webhook_request_ssh_url"
	gitRefsLabel     = "radix_webhook_request_branch"
	gitRefsTypeLabel = "radix_webhook_request_refs_type"
	commitIDLabel    = "radix_webhook_request_commit_id"
	appNameLabel     = "radix_webhook_request_app_name"
)

var (
	requestAllCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "radix_webhook_request_all_counter",
			Help: "Counter for all requests",
		},
	)
	notGithubEventCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "radix_webhook_request_not_github_event_counter",
			Help: "Counter for not GitHub event requests",
		},
	)
	failedParsingCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "radix_webhook_request_failed_parsing_counter",
			Help: "Counter for failed parsing requests",
		},
	)
	unsupportedGithubEventTypeCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "radix_webhook_request_unsupported_github_event_type_counter",
			Help: "Counter for unsupported GitHub event type requests",
		},
	)
	pingEventTypeCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "radix_webhook_request_ping_github_event_type_counter",
			Help: "Counter for ping GitHub event type requests",
		},
		[]string{sshURLLabel},
	)
	failedCloneURLValidationCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "radix_webhook_request_failed_clone_url_validation_counter",
			Help: "Counter for failed clone URL validation requests",
		},
		[]string{sshURLLabel},
	)
	pushEventTypeCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "radix_webhook_request_push_github_event_type_counter",
			Help: "Counter for push GitHub event type requests",
		},
		[]string{sshURLLabel, gitRefsLabel, gitRefsTypeLabel, commitIDLabel},
	)
	pushEventTypeTriggerPipelineCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "radix_webhook_request_push_github_event_type_trigger_pipeline_counter",
			Help: "Counter for push GitHub event type trigger pipeline requests",
		},
		[]string{sshURLLabel, gitRefsLabel, gitRefsTypeLabel, commitIDLabel, appNameLabel},
	)
	pushEventTypeFailedTriggerPipelineCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "radix_webhook_request_push_github_event_type_failed_trigger_pipeline_counter",
			Help: "Counter for push GitHub event type failed trigger pipeline requests",
		},
		[]string{sshURLLabel, gitRefsLabel, gitRefsTypeLabel, commitIDLabel},
	)
)

func init() {
	prometheus.MustRegister(requestAllCounter)
	prometheus.MustRegister(notGithubEventCounter)
	prometheus.MustRegister(failedParsingCounter)
	prometheus.MustRegister(unsupportedGithubEventTypeCounter)
	prometheus.MustRegister(pingEventTypeCounter)
	prometheus.MustRegister(failedCloneURLValidationCounter)
	prometheus.MustRegister(pushEventTypeCounter)
	prometheus.MustRegister(pushEventTypeTriggerPipelineCounter)
	prometheus.MustRegister(pushEventTypeFailedTriggerPipelineCounter)
}

// IncreaseAllCounter increases all HTTP request counter
func IncreaseAllCounter() {
	requestAllCounter.Inc()
}

// IncreaseNotGithubEventCounter increases not GitHub event request counter
func IncreaseNotGithubEventCounter() {
	notGithubEventCounter.Inc()
}

// IncreaseFailedParsingCounter increases failed parsing request counter
func IncreaseFailedParsingCounter() {
	failedParsingCounter.Inc()
}

// IncreaseUnsupportedGithubEventTypeCounter increases unsupported GitHub event type request counter
func IncreaseUnsupportedGithubEventTypeCounter() {
	unsupportedGithubEventTypeCounter.Inc()
}

// IncreasePingGithubEventTypeCounter increases all GitHub ping event type request counter
func IncreasePingGithubEventTypeCounter(sshURL string) {
	pingEventTypeCounter.With(prometheus.Labels{sshURLLabel: sshURL}).Inc()
}

// IncreaseFailedCloneURLValidationCounter increases unsupported GitHub event type request counter
func IncreaseFailedCloneURLValidationCounter(sshURL string) {
	failedCloneURLValidationCounter.With(prometheus.Labels{sshURLLabel: sshURL}).Inc()
}

// IncreasePushGithubEventTypeCounter increases all GitHub push event type request counter
func IncreasePushGithubEventTypeCounter(sshURL, gitRefs, gitRefsType, commitID string) {
	pushEventTypeCounter.With(prometheus.Labels{sshURLLabel: sshURL, gitRefsLabel: gitRefs, gitRefsTypeLabel: gitRefsType, commitIDLabel: commitID}).Inc()
}

// IncreasePushGithubEventTypeTriggerPipelineCounter increases GitHub push event type trigger pipeline request counter
func IncreasePushGithubEventTypeTriggerPipelineCounter(sshURL, gitRefs, gitRefsType, commitID, appName string) {
	pushEventTypeTriggerPipelineCounter.With(prometheus.Labels{sshURLLabel: sshURL, gitRefsLabel: gitRefs, gitRefsTypeLabel: gitRefsType, commitIDLabel: commitID, appNameLabel: appName}).Inc()
}

// IncreasePushGithubEventTypeFailedTriggerPipelineCounter increases GitHub push event type failed trigger pipeline request counter
func IncreasePushGithubEventTypeFailedTriggerPipelineCounter(sshURL, gitRefs, gitRefsType, commitID string) {
	pushEventTypeFailedTriggerPipelineCounter.With(prometheus.Labels{sshURLLabel: sshURL, gitRefsLabel: gitRefs, gitRefsTypeLabel: gitRefsType, commitIDLabel: commitID}).Inc()
}

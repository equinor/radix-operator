package handler

import (
	"github.com/google/go-github/github"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	"net/http"
)

type WebhookListener interface {
	ProcessPushEvent(rr *v1.RadixRegistration, pushEvent *github.PushEvent, req *http.Request) error
	ProcessPullRequestEvent(rr *v1.RadixRegistration, prEvent *github.PullRequestEvent, req *http.Request) error
}

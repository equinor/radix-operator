package handler

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const hubSignatureHeader = "X-Hub-Signature"

var pingRepoPattern = regexp.MustCompile(".*github.com/repos/(.*?)")
var pingHooksPattern = regexp.MustCompile("/hooks/[0-9]*")

type WebhookResponse struct {
	Ok      bool   `json:"ok"`
	Event   string `json:"event"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

type WebHookHandler struct {
	radixclient radixclient.Interface
}

func NewWebHookHandler(radixclient radixclient.Interface) *WebHookHandler {
	return &WebHookHandler{
		radixclient,
	}
}

func (wh *WebHookHandler) HandleWebhookEvents(listener WebhookListener) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		event := req.Header.Get("x-github-event")

		_fail := func(err error) {
			fail(w, event, err)
		}
		_succeed := func() {
			succeed(w, event)
		}
		_succeedWithMessage := func(message string) {
			log.Infof("Success: %s", message)
			succeedWithMessage(w, event, message)
		}

		if len(strings.TrimSpace(event)) == 0 {
			_fail(fmt.Errorf("Not a github event"))
			return
		}

		// Need to parse webhook before validation because the secret is taken from the matching repo
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			_fail(fmt.Errorf("Could not parse webhook: err=%s ", err))
			return
		}

		payload, err := github.ParseWebHook(github.WebHookType(req), body)
		if err != nil {
			_fail(fmt.Errorf("Could not parse webhook: err=%s ", err))
			return
		}

		switch e := payload.(type) {
		case *github.PushEvent:
			rr, err := wh.isValidSecret(req, body, e.Repo.GetSSHURL())
			if err != nil {
				_fail(err)
				return
			}

			err = listener.ProcessPushEvent(rr, e, req)
			if err != nil {
				_fail(err)
				return
			}

			_succeed()

		case *github.PingEvent:
			sshURL := getSSHUrlFromPingURL(*e.Hook.URL)
			rr, err := wh.isValidSecret(req, body, sshURL)
			if err != nil {
				_fail(err)
				return
			}

			_succeedWithMessage(fmt.Sprintf("Webhook is set up correctly with the Radix project: %s", rr.Name))

		case *github.PullRequestEvent:
			rr, err := wh.isValidSecret(req, body, e.Repo.GetSSHURL())
			if err != nil {
				_fail(err)
				return
			}

			err = listener.ProcessPullRequestEvent(rr, e, req)
			if err != nil {
				_fail(err)
				return
			}

			_succeed()

		default:
			_fail(fmt.Errorf("Unknown event type %s ", github.WebHookType(req)))
			return
		}
	})
}

func (wh *WebHookHandler) isValidSecret(req *http.Request, body []byte, sshUrl string) (*v1.RadixRegistration, error) {
	rr, err := wh.getRadixRegistrationFromRepo(sshUrl)
	if err != nil {
		return nil, err
	}

	signature := req.Header.Get(hubSignatureHeader)
	if err := validateSignature(signature, rr.Spec.SharedSecret, body); err != nil {
		log.Printf("Failed to validate signature for app %s", rr.Name)
		return nil, err
	}

	return rr, nil
}

func (wh *WebHookHandler) getRadixRegistrationFromRepo(sshUrl string) (*v1.RadixRegistration, error) {
	rrs, err := wh.radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var matchedRR *v1.RadixRegistration
	for _, rr := range rrs.Items {
		if strings.EqualFold(rr.Spec.CloneURL, sshUrl) {
			matchedRR = &rr
			break
		}
	}

	if matchedRR == nil {
		return nil, errors.New("No matching Radix registration found for this webhook: " + sshUrl)
	}

	return matchedRR, nil
}

func getSSHUrlFromPingURL(pingURL string) string {
	fullName := pingRepoPattern.ReplaceAllString(pingURL, "")
	fullName = pingHooksPattern.ReplaceAllString(fullName, "")
	return fmt.Sprintf("git@github.com:%s.git", fullName)
}

func succeed(w http.ResponseWriter, event string) {
	render(w, WebhookResponse{
		Ok:    true,
		Event: event,
	})
}

func succeedWithMessage(w http.ResponseWriter, event, message string) {
	render(w, WebhookResponse{
		Ok:      true,
		Event:   event,
		Message: message,
	})
}

func fail(w http.ResponseWriter, event string, err error) {
	log.Printf("%s\n", err)
	w.WriteHeader(500)
	render(w, WebhookResponse{
		Ok:    false,
		Event: event,
		Error: err.Error(),
	})
}

func render(w http.ResponseWriter, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Write(data)
}

//  Taken from brigade pkg/webhook/github.go
//
// validateSignature compares the salted digest in the header with our own computing of the body.
func validateSignature(signature, secretKey string, payload []byte) error {
	sum := SHA1HMAC([]byte(secretKey), payload)
	if subtle.ConstantTimeCompare([]byte(sum), []byte(signature)) != 1 {
		log.Printf("Expected signature %q (sum), got %q (hub-signature)", sum, signature)
		return errors.New("payload signature check failed")
	}
	return nil
}

// SHA1HMAC computes the GitHub SHA1 HMAC.
func SHA1HMAC(salt, message []byte) string {
	// GitHub creates a SHA1 HMAC, where the key is the GitHub secret and the
	// message is the JSON body.
	digest := hmac.New(sha1.New, salt)
	digest.Write(message)
	sum := digest.Sum(nil)
	return fmt.Sprintf("sha1=%x", sum)
}

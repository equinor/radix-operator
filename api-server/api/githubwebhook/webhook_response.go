package githubwebhook

import (
	"net/http"

	"github.com/rs/zerolog/log"
)

type WebhookResponse struct {
	Ok      bool   `json:"ok"`
	Event   string `json:"event"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

func (c *githubController) writeErrorResponse(w http.ResponseWriter, r *http.Request, statusCode int, err error, event string) {
	log.Ctx(r.Context()).Warn().Err(err).Msg("Error handling GitHub webhook event")
	c.JSONResponseWithCode(w, r, statusCode, WebhookResponse{
		Ok:    false,
		Event: event,
		Error: err.Error(),
	})
}

func (c *githubController) writeSuccessResponse(w http.ResponseWriter, r *http.Request, statusCode int, message string, event string) {
	log.Ctx(r.Context()).Info().Msg(message)
	c.JSONResponseWithCode(w, r, statusCode, WebhookResponse{
		Ok:      true,
		Event:   event,
		Message: message,
	})
}

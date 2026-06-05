package githubwebhook

import (
	"errors"
	"net/http"

	"github.com/equinor/radix-operator/api-server/api/utils/tlsvalidation"
	"github.com/equinor/radix-operator/api-server/models"
)

const rootPath = "/webhook/github"

type githubController struct {
	*models.DefaultController
	tlsValidator tlsvalidation.Validator
}

// NewGithubWebhookController Constructor
func NewGithubWebhookController(tlsValidator tlsvalidation.Validator) models.Controller {
	return &githubController{
		tlsValidator: tlsValidator,
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
	// swagger:operation POST /webhook/github githubwebhook handleGithubWebhook
	// ---
	// summary: Handle GitHub webhook events
	// description: This endpoint receives GitHub webhook events and processes them accordingly.
	// responses:
	//   '200':
	//     description: Webhook event processed successfully
	//   '400':
	//     description: Bad request, e.g., invalid payload or signature
	//   '500':
	//     description: Internal server error while processing the webhook event

	// /api/v1/webhooks/github?appName=XXXX
	c.ErrorResponse(w, r, errors.New("not implemented"))
}

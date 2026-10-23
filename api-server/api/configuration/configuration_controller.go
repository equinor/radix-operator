package configuration

import (
	"net/http"

	"github.com/equinor/radix-operator/api-server/internal/accounts"
	"github.com/equinor/radix-operator/api-server/internal/controller"
)

const rootPath = "/configuration"

type configurationController struct {
	*controller.DefaultController

	handler ConfigurationHandler
}

// NewConfigurationController Constructor
func NewConfigurationController(handler ConfigurationHandler) controller.Controller {
	return &configurationController{
		handler: handler,
	}
}

// GetRoutes List the supported routes of this handler
func (c *configurationController) GetRoutes() controller.Routes {
	routes := controller.Routes{
		controller.Route{
			Path:                      rootPath,
			Method:                    "GET",
			HandlerFunc:               c.GetClusterConfiguration,
			AllowUnauthenticatedUsers: false,
		},
	}

	return routes
}

// GetClusterConfiguration reveals the settings for the selected cluster environment
func (c *configurationController) GetClusterConfiguration(accounts accounts.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /configuration Configuration getConfiguration
	// ---
	// summary: Show the cluster environment
	// responses:
	//   "200":
	//     description: "Successful operation"
	//     schema:
	//        "$ref": "#/definitions/ClusterConfiguration"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "500":
	//     description: "Internal Server Error"

	s, err := c.handler.GetClusterConfiguration(r.Context())
	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, s)
}

package environmentvariables

import (
	"encoding/json"
	"net/http"

	envvarsmodels "github.com/equinor/radix-operator/api-server/api/environmentvariables/models"
	"github.com/equinor/radix-operator/api-server/models"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

const rootPath = "/applications/{appName}"

type envVarsController struct {
	*models.DefaultController
	handlerFactory envVarsHandlerFactory
}

// NewEnvVarsController Constructor
func NewEnvVarsController() models.Controller {
	return &envVarsController{
		handlerFactory: &defaultEnvVarsHandlerFactory{},
	}
}

func (controller *envVarsController) withHandlerFactory(factory envVarsHandlerFactory) *envVarsController {
	controller.handlerFactory = factory
	return controller
}

// GetRoutes List the supported routes of this handler
func (controller *envVarsController) GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:        rootPath + "/environments/{envName}/components/{componentName}/envvars",
			Method:      "GET",
			HandlerFunc: controller.GetComponentEnvVars,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/components/{componentName}/envvars",
			Method:      "PATCH",
			HandlerFunc: controller.ChangeEnvVar,
		},
	}

	return routes
}

// GetComponentEnvVars Get log from a scheduled job
func (controller *envVarsController) GetComponentEnvVars(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/components/{componentName}/envvars component envVars
	// ---
	// summary: Get environment variables for component
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: envName
	//   in: path
	//   description: Name of environment
	//   type: string
	//   required: true
	// - name: componentName
	//   in: path
	//   description: Name of component
	//   type: string
	//   required: true
	// - name: Impersonate-User
	//   in: header
	//   description: Works only with custom setup of cluster. Allow impersonation of test users (Required if Impersonate-Group is set)
	//   type: string
	//   required: false
	// - name: Impersonate-Group
	//   in: header
	//   description: Works only with custom setup of cluster. Allow impersonation of a comma-separated list of test groups (Required if Impersonate-User is set)
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     description: "environment variables"
	//     schema:
	//        type: "array"
	//        items:
	//           "$ref": "#/definitions/EnvVar"
	//   "404":
	//     description: "Not found"
	appName, envName, componentName := mux.Vars(r)["appName"], mux.Vars(r)["envName"], mux.Vars(r)["componentName"]

	eh := controller.handlerFactory.createHandler(accounts)
	envVars, err := eh.GetComponentEnvVars(r.Context(), appName, envName, componentName)

	if err != nil {
		controller.ErrorResponse(w, r, err)
		return
	}

	controller.JSONResponse(w, r, envVars)
}

// ChangeEnvVar Modifies an environment variable
func (controller *envVarsController) ChangeEnvVar(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation PATCH /applications/{appName}/environments/{envName}/components/{componentName}/envvars component changeEnvVar
	// ---
	// summary: Update an environment variable
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: envName
	//   in: path
	//   description: environment of Radix application
	//   type: string
	//   required: true
	// - name: componentName
	//   in: path
	//   description: environment component of Radix application
	//   type: string
	//   required: true
	// - name: EnvVarParameter
	//   in: body
	//   description: Environment variables new values and metadata
	//   required: true
	//   schema:
	//      type: array
	//      items:
	//       "$ref": "#/definitions/EnvVarParameter"
	// - name: Impersonate-User
	//   in: header
	//   description: Works only with custom setup of cluster. Allow impersonation of test users (Required if Impersonate-Group is set)
	//   type: string
	//   required: false
	// - name: Impersonate-Group
	//   in: header
	//   description: Works only with custom setup of cluster. Allow impersonation of a comma-separated list of test groups (Required if Impersonate-User is set)
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     description: success
	//   "400":
	//     description: "Invalid application"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"
	//   "409":
	//     description: "Conflict"
	//   "500":
	//     description: "Internal server error"

	appName, envName, componentName := mux.Vars(r)["appName"], mux.Vars(r)["envName"], mux.Vars(r)["componentName"]
	var envVarParameters []envvarsmodels.EnvVarParameter
	if err := json.NewDecoder(r.Body).Decode(&envVarParameters); err != nil {
		controller.ErrorResponse(w, r, err)
		return
	}

	log.Ctx(r.Context()).Debug().Msgf("Update %d environment variables for app: %s, env: %s, component: %s", len(envVarParameters), appName, envName, componentName)

	envVarsHandler := controller.handlerFactory.createHandler(accounts)

	err := envVarsHandler.ChangeEnvVar(r.Context(), appName, envName, componentName, envVarParameters)
	if err != nil {
		controller.ErrorResponse(w, r, err)
		return
	}

	controller.JSONResponse(w, r, "Success")
}

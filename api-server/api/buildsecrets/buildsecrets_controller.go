package buildsecrets

import (
	"encoding/json"
	"net/http"

	environmentModels "github.com/equinor/radix-operator/api-server/api/secrets/models"
	"github.com/equinor/radix-operator/api-server/models"
	"github.com/gorilla/mux"
)

const rootPath = "/applications/{appName}"

type buildSecretsController struct {
	*models.DefaultController
}

// NewBuildSecretsController Constructor
func NewBuildSecretsController() models.Controller {
	return &buildSecretsController{}
}

// GetRoutes List the supported routes of this handler
func (dc *buildSecretsController) GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:        rootPath + "/buildsecrets",
			Method:      "GET",
			HandlerFunc: dc.GetBuildSecrets,
		},
		models.Route{
			Path:        rootPath + "/buildsecrets/{secretName}",
			Method:      "PUT",
			HandlerFunc: dc.ChangeBuildSecret,
		},
	}

	return routes
}

// GetBuildSecrets Lists build secrets
func (dc *buildSecretsController) GetBuildSecrets(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/buildsecrets application getBuildSecrets
	// ---
	// summary: Lists the application build secrets
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
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
	//     description: "Successful operation"
	//     schema:
	//        type: "array"
	//        items:
	//           "$ref": "#/definitions/BuildSecret"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]

	buildSecretsHandler := Init(accounts)
	buildSecrets, err := buildSecretsHandler.GetBuildSecrets(r.Context(), appName)

	if err != nil {
		dc.ErrorResponse(w, r, err)
		return
	}

	dc.JSONResponse(w, r, buildSecrets)
}

// ChangeBuildSecret Modifies an application build secret
func (dc *buildSecretsController) ChangeBuildSecret(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation PUT /applications/{appName}/buildsecrets/{secretName} application updateBuildSecretsSecretValue
	// ---
	// summary: Update an application build secret
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: secretName
	//   in: path
	//   description: name of secret
	//   type: string
	//   required: true
	// - name: secretValue
	//   in: body
	//   description: New secret value
	//   required: true
	//   schema:
	//       "$ref": "#/definitions/SecretParameters"
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
	appName := mux.Vars(r)["appName"]
	secretName := mux.Vars(r)["secretName"]

	var secretParameters environmentModels.SecretParameters
	if err := json.NewDecoder(r.Body).Decode(&secretParameters); err != nil {
		dc.ErrorResponse(w, r, err)
		return
	}

	buildSecretsHandler := Init(accounts)
	err := buildSecretsHandler.ChangeBuildSecret(r.Context(), appName, secretName, secretParameters.SecretValue)

	if err != nil {
		dc.ErrorResponse(w, r, err)
		return
	}

	dc.JSONResponse(w, r, "Success")
}

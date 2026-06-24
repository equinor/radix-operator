package alerting

import (
	"encoding/json"
	"net/http"

	alertingModels "github.com/equinor/radix-operator/api-server/api/alerting/models"
	"github.com/equinor/radix-operator/api-server/models"
	crdutils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/gorilla/mux"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const appPath = "/applications/{appName}"
const envPath = appPath + "/environments/{envName}"

type alertingController struct {
	*models.DefaultController
}

// NewAlertingController Constructor
func NewAlertingController() models.Controller {
	return &alertingController{}
}

// GetRoutes List the supported routes of this handler
func (ec *alertingController) GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:        envPath + "/alerting",
			Method:      "PUT",
			HandlerFunc: ec.EnvironmentRouteAccessCheck(ec.UpdateEnvironmentAlertingConfig),
		},
		models.Route{
			Path:        envPath + "/alerting",
			Method:      http.MethodGet,
			HandlerFunc: ec.EnvironmentRouteAccessCheck(ec.GetEnvironmentAlertingConfig),
		},
		models.Route{
			Path:        envPath + "/alerting/enable",
			Method:      http.MethodPost,
			HandlerFunc: ec.EnvironmentRouteAccessCheck(ec.EnableEnvironmentAlerting),
		},
		models.Route{
			Path:        envPath + "/alerting/disable",
			Method:      http.MethodPost,
			HandlerFunc: ec.EnvironmentRouteAccessCheck(ec.DisableEnvironmentAlerting),
		},
		models.Route{
			Path:        appPath + "/alerting",
			Method:      "PUT",
			HandlerFunc: ec.UpdateApplicationAlertingConfig,
		},
		models.Route{
			Path:        appPath + "/alerting",
			Method:      http.MethodGet,
			HandlerFunc: ec.GetApplicationAlertingConfig,
		},
		models.Route{
			Path:        appPath + "/alerting/enable",
			Method:      http.MethodPost,
			HandlerFunc: ec.EnableApplicationAlerting,
		},
		models.Route{
			Path:        appPath + "/alerting/disable",
			Method:      http.MethodPost,
			HandlerFunc: ec.DisableApplicationAlerting,
		},
	}

	return routes
}

// EnvironmentRouteAccessCheck gets appName and envName from route and verifies that environment exists
// Returns 404 NotFound if environment is not defined, otherwise calls handler
func (ec *alertingController) EnvironmentRouteAccessCheck(handler models.RadixHandlerFunc) models.RadixHandlerFunc {
	return func(a models.Accounts, rw http.ResponseWriter, r *http.Request) {
		appName := mux.Vars(r)["appName"]
		envName := mux.Vars(r)["envName"]
		envNamespace := crdutils.GetEnvironmentNamespace(appName, envName)

		if _, err := a.ServiceAccount.RadixClient.RadixV1().RadixEnvironments().Get(r.Context(), envNamespace, v1.GetOptions{}); err != nil {
			ec.ErrorResponse(rw, r, err)
			return
		}

		handler(a, rw, r)
	}
}

// UpdateEnvironmentAlertingConfig Configures alert settings
func (ec *alertingController) UpdateEnvironmentAlertingConfig(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation PUT /applications/{appName}/environments/{envName}/alerting environment updateEnvironmentAlertingConfig
	// ---
	// summary: Update alerts configuration for an environment
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
	// - name: alertsConfig
	//   in: body
	//   description: Alerts configuration
	//   required: true
	//   schema:
	//       "$ref": "#/definitions/UpdateAlertingConfig"
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
	//     description: Successful alerts config update
	//     schema:
	//        "$ref": "#/definitions/AlertingConfig"
	//   "400":
	//     description: "Invalid configuration"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"
	//   "500":
	//     description: "Internal server error"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]

	var updateAlertingConfig alertingModels.UpdateAlertingConfig
	if err := json.NewDecoder(r.Body).Decode(&updateAlertingConfig); err != nil {
		ec.ErrorResponse(w, r, err)
		return
	}

	alertHandler := NewEnvironmentHandler(accounts, appName, envName)
	alertsConfig, err := alertHandler.UpdateAlertingConfig(r.Context(), updateAlertingConfig)

	if err != nil {
		ec.ErrorResponse(w, r, err)
		return
	}

	ec.JSONResponse(w, r, alertsConfig)
}

// GetEnvironmentAlertingConfig returns alerts configuration
func (ec *alertingController) GetEnvironmentAlertingConfig(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/alerting environment getEnvironmentAlertingConfig
	// ---
	// summary: Get alerts configuration for an environment
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
	//     description: Successful get alerts config
	//     schema:
	//        "$ref": "#/definitions/AlertingConfig"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"
	//   "500":
	//     description: "Internal server error"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]

	alertHandler := NewEnvironmentHandler(accounts, appName, envName)
	alertsConfig, err := alertHandler.GetAlertingConfig(r.Context())

	if err != nil {
		ec.ErrorResponse(w, r, err)
		return
	}

	ec.JSONResponse(w, r, alertsConfig)
}

// EnableEnvironmentAlerting enables alerting for application environment
func (ec *alertingController) EnableEnvironmentAlerting(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/alerting/enable environment enableEnvironmentAlerting
	// ---
	// summary: Enable alerting for an environment
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
	//     description: Successful enable alerting
	//     schema:
	//        "$ref": "#/definitions/AlertingConfig"
	//   "400":
	//     description: "Alerting already enabled"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"
	//   "500":
	//     description: "Internal server error"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]

	alertHandler := NewEnvironmentHandler(accounts, appName, envName)
	alertsConfig, err := alertHandler.EnableAlerting(r.Context())

	if err != nil {
		ec.ErrorResponse(w, r, err)
		return
	}

	ec.JSONResponse(w, r, alertsConfig)
}

// DisableEnvironmentAlerting disables alerting for application environment
func (ec *alertingController) DisableEnvironmentAlerting(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/alerting/disable environment disableEnvironmentAlerting
	// ---
	// summary: Disable alerting for an environment
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
	//     description: Successful disable alerting
	//     schema:
	//        "$ref": "#/definitions/AlertingConfig"
	//   "400":
	//     description: "Alerting already enabled"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"
	//   "500":
	//     description: "Internal server error"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]

	alertHandler := NewEnvironmentHandler(accounts, appName, envName)
	alertsConfig, err := alertHandler.DisableAlerting(r.Context())
	if err != nil {
		ec.ErrorResponse(w, r, err)
		return
	}

	ec.JSONResponse(w, r, alertsConfig)
}

// UpdateApplicationAlertingConfig Configures alert settings
func (ec *alertingController) UpdateApplicationAlertingConfig(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation PUT /applications/{appName}/alerting application updateApplicationAlertingConfig
	// ---
	// summary: Update alerts configuration for application namespace
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: alertsConfig
	//   in: body
	//   description: Alerts configuration
	//   required: true
	//   schema:
	//       "$ref": "#/definitions/UpdateAlertingConfig"
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
	//     description: Successful alerts config update
	//     schema:
	//        "$ref": "#/definitions/AlertingConfig"
	//   "400":
	//     description: "Invalid configuration"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"
	//   "500":
	//     description: "Internal server error"

	appName := mux.Vars(r)["appName"]

	var updateAlertingConfig alertingModels.UpdateAlertingConfig
	if err := json.NewDecoder(r.Body).Decode(&updateAlertingConfig); err != nil {
		ec.ErrorResponse(w, r, err)
		return
	}

	alertHandler := NewApplicationHandler(accounts, appName)
	alertsConfig, err := alertHandler.UpdateAlertingConfig(r.Context(), updateAlertingConfig)

	if err != nil {
		ec.ErrorResponse(w, r, err)
		return
	}

	ec.JSONResponse(w, r, alertsConfig)
}

// GetApplicationAlertingConfig returns alerts configuration
func (ec *alertingController) GetApplicationAlertingConfig(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/alerting application getApplicationAlertingConfig
	// ---
	// summary: Get alerts configuration for application namespace
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
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
	//     description: Successful get alerts config
	//     schema:
	//        "$ref": "#/definitions/AlertingConfig"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"
	//   "500":
	//     description: "Internal server error"
	appName := mux.Vars(r)["appName"]

	alertHandler := NewApplicationHandler(accounts, appName)
	alertsConfig, err := alertHandler.GetAlertingConfig(r.Context())

	if err != nil {
		ec.ErrorResponse(w, r, err)
		return
	}

	ec.JSONResponse(w, r, alertsConfig)
}

// EnableApplicationAlerting enables alerting for application
func (ec *alertingController) EnableApplicationAlerting(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/alerting/enable application enableApplicationAlerting
	// ---
	// summary: Enable alerting for application namespace
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
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
	//     description: Successful enable alerting
	//     schema:
	//        "$ref": "#/definitions/AlertingConfig"
	//   "400":
	//     description: "Alerting already enabled"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"
	//   "500":
	//     description: "Internal server error"
	appName := mux.Vars(r)["appName"]

	alertHandler := NewApplicationHandler(accounts, appName)
	alertsConfig, err := alertHandler.EnableAlerting(r.Context())

	if err != nil {
		ec.ErrorResponse(w, r, err)
		return
	}

	ec.JSONResponse(w, r, alertsConfig)
}

// DisableApplicationAlerting disables alerting for application
func (ec *alertingController) DisableApplicationAlerting(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/alerting/disable application disableApplicationAlerting
	// ---
	// summary: Disable alerting for application namespace
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
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
	//     description: Successful disable alerting
	//     schema:
	//        "$ref": "#/definitions/AlertingConfig"
	//   "400":
	//     description: "Alerting already enabled"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"
	//   "500":
	//     description: "Internal server error"
	appName := mux.Vars(r)["appName"]

	alertHandler := NewApplicationHandler(accounts, appName)
	alertsConfig, err := alertHandler.DisableAlerting(r.Context())
	if err != nil {
		ec.ErrorResponse(w, r, err)
		return
	}

	ec.JSONResponse(w, r, alertsConfig)
}

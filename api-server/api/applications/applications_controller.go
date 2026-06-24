package applications

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	applicationModels "github.com/equinor/radix-operator/api-server/api/applications/models"
	"github.com/equinor/radix-operator/api-server/api/metrics"
	"github.com/equinor/radix-operator/api-server/models"
	"github.com/gorilla/mux"
)

const rootPath = ""
const appPath = rootPath + "/applications/{appName}"

type applicationController struct {
	*models.DefaultController
	hasAccessToRR
	applicationHandlerFactory ApplicationHandlerFactory
	metricsHandler            *metrics.Handler
}

// NewApplicationController Constructor
func NewApplicationController(hasAccessTo hasAccessToRR, applicationHandlerFactory ApplicationHandlerFactory, metricsHandler *metrics.Handler) models.Controller {
	if hasAccessTo == nil {
		hasAccessTo = hasAccess
	}

	return &applicationController{
		hasAccessToRR:             hasAccessTo,
		applicationHandlerFactory: applicationHandlerFactory,
		metricsHandler:            metricsHandler,
	}
}

// GetRoutes List the supported routes of this controller
func (ac *applicationController) GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:        rootPath + "/applications",
			Method:      "POST",
			HandlerFunc: ac.RegisterApplication,
		},
		models.Route{
			Path:        appPath,
			Method:      "PUT",
			HandlerFunc: ac.ChangeRegistrationDetails,
		},
		models.Route{
			Path:        appPath,
			Method:      "PATCH",
			HandlerFunc: ac.ModifyRegistrationDetails,
		},
		models.Route{
			Path:        rootPath + "/applications",
			Method:      "GET",
			HandlerFunc: ac.ShowApplications,
			KubeApiConfig: models.KubeApiConfig{
				QPS:   50,
				Burst: 100,
			},
		},
		models.Route{
			Path:        rootPath + "/applications/_search",
			Method:      "GET",
			HandlerFunc: ac.SearchApplications,
			KubeApiConfig: models.KubeApiConfig{
				QPS:   100,
				Burst: 100,
			},
		},
		models.Route{
			Path:        appPath,
			Method:      "GET",
			HandlerFunc: ac.GetApplication,
		},
		models.Route{
			Path:        appPath,
			Method:      "DELETE",
			HandlerFunc: ac.DeleteApplication,
		},
		models.Route{
			Path:        appPath + "/pipelines",
			Method:      "GET",
			HandlerFunc: ac.ListPipelines,
		},
		models.Route{
			Path:        appPath + "/pipelines/build",
			Method:      "POST",
			HandlerFunc: ac.TriggerPipelineBuild,
		},
		models.Route{
			Path:        appPath + "/pipelines/build-deploy",
			Method:      "POST",
			HandlerFunc: ac.TriggerPipelineBuildDeploy,
		},
		models.Route{
			Path:        appPath + "/pipelines/promote",
			Method:      "POST",
			HandlerFunc: ac.TriggerPipelinePromote,
		},
		models.Route{
			Path:        appPath + "/pipelines/deploy",
			Method:      "POST",
			HandlerFunc: ac.TriggerPipelineDeploy,
		},
		models.Route{
			Path:        appPath + "/pipelines/apply-config",
			Method:      "POST",
			HandlerFunc: ac.TriggerPipelineApplyConfig,
		},
		models.Route{
			Path:        appPath + "/deploykey-valid",
			Method:      "GET",
			HandlerFunc: ac.IsDeployKeyValidHandler,
		},
		models.Route{
			Path:        appPath + "/deploy-key-and-secret",
			Method:      "GET",
			HandlerFunc: ac.GetDeployKeyAndSecret,
		},
		models.Route{
			Path:        appPath + "/regenerate-deploy-key",
			Method:      "POST",
			HandlerFunc: ac.RegenerateDeployKeyHandler,
		},
		models.Route{
			Path:        appPath + "/regenerate-shared-secret",
			Method:      "POST",
			HandlerFunc: ac.RegenerateSharedSecretHandler,
		},
		models.Route{
			Path:        appPath + "/utilization",
			Method:      "GET",
			HandlerFunc: ac.GetApplicationResourcesUtilization,
		},
		models.Route{
			Path:        appPath + "/environments/{envName}/utilization",
			Method:      "GET",
			HandlerFunc: ac.GetEnvironmentResourcesUtilization,
		},
	}

	return routes
}

// ShowApplications Lists applications
func (ac *applicationController) ShowApplications(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications platform showApplications
	//
	// ---
	// summary: Lists the applications. NOTE - doesn't get applicationSummary.latestJob.Environments
	// parameters:
	// - name: sshRepo
	//   in: query
	//   description: ssh repo to identify Radix application if exists
	//   type: string
	//   required: false
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
	//           "$ref": "#/definitions/ApplicationSummary"
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

	matcher := applicationModels.MatchAll
	sshRepo := strings.TrimSpace(r.FormValue("sshRepo"))
	if len(sshRepo) > 0 {
		matcher = applicationModels.MatchBySSHRepoFunc(sshRepo)
	}

	handler := ac.applicationHandlerFactory.Create(accounts)
	appRegistrations, err := handler.GetApplications(r.Context(), matcher, ac.hasAccessToRR, GetApplicationsOptions{})

	if err != nil {
		ac.ErrorResponse(w, r, err)
		return
	}

	ac.JSONResponse(w, r, appRegistrations)
}

// SearchApplications Gets applications by list of application names
func (ac *applicationController) SearchApplications(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/_search platform getSearchApplications
	//
	// ---
	// summary: Get applications by name. NOTE - doesn't get applicationSummary.latestJob.Environments
	// parameters:
	// - name: apps
	//   in: query
	//   description: Comma separated list of application names to search for
	//   required: true
	//   type: string
	// - name: includeLatestJobSummary
	//   in: query
	//   description: true to include latest job summary
	//   required: false
	//   type: boolean
	// - name: includeEnvironments
	//   in: query
	//   description: true to include environments
	//   required: false
	//   type: boolean
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
	//           "$ref": "#/definitions/ApplicationSummary"
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

	var appNamesRequest applicationModels.ApplicationsSearchRequest

	switch r.Method {
	case http.MethodGet:
		appNamesRequest.Names = strings.Split(r.FormValue("apps"), ",")

		if includeLatestJobSummary, _ := strconv.ParseBool(r.FormValue("includeLatestJobSummary")); includeLatestJobSummary {
			appNamesRequest.IncludeFields.LatestJobSummary = true
		}
		if includeEnvActiveComponents, _ := strconv.ParseBool(r.FormValue("includeEnvironments")); includeEnvActiveComponents {
			appNamesRequest.IncludeFields.Environments = true
		}
	case http.MethodPost:
		if err := json.NewDecoder(r.Body).Decode(&appNamesRequest); err != nil {
			ac.ErrorResponse(w, r, err)
			return
		}
	default:
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// No need to perform search if names in request is empty. Just return empty list
	if len(appNamesRequest.Names) == 0 {
		ac.JSONResponse(w, r, []interface{}{})
		return
	}

	handler := ac.applicationHandlerFactory.Create(accounts)
	matcher := applicationModels.MatchByNamesFunc(appNamesRequest.Names)

	appRegistrations, err := handler.GetApplications(
		r.Context(),
		matcher,
		ac.hasAccessToRR,
		GetApplicationsOptions{
			IncludeLatestJobSummary: appNamesRequest.IncludeFields.LatestJobSummary,
			IncludeEnvironments:     appNamesRequest.IncludeFields.Environments,
		},
	)
	if err != nil {
		ac.ErrorResponse(w, r, err)
		return
	}

	ac.JSONResponse(w, r, appRegistrations)
}

// GetApplication Gets application by application name
func (ac *applicationController) GetApplication(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName} application getApplication
	// ---
	// summary: Gets the application by name
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
	//     description: Successful get application
	//     schema:
	//       "$ref": "#/definitions/Application"
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

	appName := mux.Vars(r)["appName"]

	handler := ac.applicationHandlerFactory.Create(accounts)

	application, err := handler.GetApplication(r.Context(), appName)

	if err != nil {
		ac.ErrorResponse(w, r, err)
		return
	}

	ac.JSONResponse(w, r, &application)
}

// IsDeployKeyValidHandler validates deploy key for radix application found for application name
func (ac *applicationController) IsDeployKeyValidHandler(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/deploykey-valid application isDeployKeyValid
	// ---
	// summary: Checks if the deploy key is correctly setup for application by cloning the repository
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
	//     description: "Deploy key is valid"
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

	ac.ErrorResponse(w, r, errors.New("obsolete method"))
}

// RegenerateDeployKeyHandler Regenerates deploy key and secret and returns the new key
func (ac *applicationController) RegenerateDeployKeyHandler(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/regenerate-deploy-key application regenerateDeployKey
	// ---
	// summary: Regenerates deploy key
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of application
	//   type: string
	//   required: true
	// - name: regenerateDeployKeyAndSecretData
	//   in: body
	//   description: Regenerate deploy key and secret data
	//   required: true
	//   schema:
	//       "$ref": "#/definitions/RegenerateDeployKeyData"
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
	//   "204":
	//     description: Successfully regenerated deploy key and set shared secret
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	//   "409":
	//     description: "Conflict"
	appName := mux.Vars(r)["appName"]
	handler := ac.applicationHandlerFactory.Create(accounts)
	var sharedSecretAndPrivateKey applicationModels.RegenerateDeployKeyData
	if err := json.NewDecoder(r.Body).Decode(&sharedSecretAndPrivateKey); err != nil {
		ac.ErrorResponse(w, r, err)
		return
	}
	err := handler.RegenerateDeployKey(r.Context(), appName, sharedSecretAndPrivateKey)

	if err != nil {
		ac.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RegenerateSharedSecretHandler Regenerates shared secret
func (ac *applicationController) RegenerateSharedSecretHandler(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/regenerate-shared-secret application regenerateSharedSecret
	// ---
	// summary: Regenerates shared secret
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of application
	//   type: string
	//   required: true
	// - name: regenerateRegenerateSharedSecretData
	//   in: body
	//   description: Regenerate shared secret and secret data
	//   required: true
	//   schema:
	//       "$ref": "#/definitions/RegenerateSharedSecretData"
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
	//   "204":
	//     description: Successfully regenerated and set shared secret
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	//   "409":
	//     description: "Conflict"
	appName := mux.Vars(r)["appName"]
	handler := ac.applicationHandlerFactory.Create(accounts)
	var sharedSecret applicationModels.RegenerateSharedSecretData
	if err := json.NewDecoder(r.Body).Decode(&sharedSecret); err != nil {
		ac.ErrorResponse(w, r, err)
		return
	}
	err := handler.RegenerateSharedSecret(r.Context(), appName, sharedSecret)

	if err != nil {
		ac.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (ac *applicationController) GetDeployKeyAndSecret(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/deploy-key-and-secret application getDeployKeyAndSecret
	// ---
	// summary: Get deploy key and secret
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of application
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
	//     description: Successful get deploy key and secret
	//     schema:
	//       "$ref": "#/definitions/DeployKeyAndSecret"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	handler := ac.applicationHandlerFactory.Create(accounts)
	deployKeyAndSecret, err := handler.GetDeployKeyAndSecret(r.Context(), appName)

	if err != nil {
		ac.ErrorResponse(w, r, err)
		return
	}

	ac.JSONResponse(w, r, &deployKeyAndSecret)

}

// RegisterApplication Creates new application registration
func (ac *applicationController) RegisterApplication(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications platform registerApplication
	// ---
	// summary: Create an application registration
	// parameters:
	// - name: applicationRegistration
	//   in: body
	//   description: Request for an Application to register
	//   required: true
	//   schema:
	//       "$ref": "#/definitions/ApplicationRegistrationRequest"
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
	//     description: Application registration operation details
	//     schema:
	//       "$ref": "#/definitions/ApplicationRegistrationUpsertResponse"
	//   "400":
	//     description: "Invalid application registration"
	//   "401":
	//     description: "Unauthorized"
	//   "409":
	//     description: "Conflict"
	var applicationRegistrationRequest applicationModels.ApplicationRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&applicationRegistrationRequest); err != nil {
		ac.ErrorResponse(w, r, err)
		return
	}

	// Need in cluster Radix client in order to validate registration using sufficient privileges
	handler := ac.applicationHandlerFactory.Create(accounts)
	appRegistrationUpsertResponse, err := handler.RegisterApplication(r.Context(), applicationRegistrationRequest)
	if err != nil {
		ac.ErrorResponse(w, r, err)
		return
	}

	ac.JSONResponse(w, r, &appRegistrationUpsertResponse)
}

// ChangeRegistrationDetails Updates application registration
func (ac *applicationController) ChangeRegistrationDetails(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation PUT /applications/{appName} application changeRegistrationDetails
	// ---
	// summary: Update application registration
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: applicationRegistration
	//   in: body
	//   description: request for Application to change
	//   required: true
	//   schema:
	//       "$ref": "#/definitions/ApplicationRegistrationRequest"
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
	//     description: Change registration operation result
	//     schema:
	//       "$ref": "#/definitions/ApplicationRegistrationUpsertResponse"
	//   "400":
	//     description: "Invalid application"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	//   "409":
	//     description: "Conflict"
	appName := mux.Vars(r)["appName"]

	var applicationRegistrationRequest applicationModels.ApplicationRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&applicationRegistrationRequest); err != nil {
		ac.ErrorResponse(w, r, err)
		return
	}

	// Need in cluster Radix client in order to validate registration using sufficient privileges
	handler := ac.applicationHandlerFactory.Create(accounts)
	appRegistrationUpsertResponse, err := handler.ChangeRegistrationDetails(r.Context(), appName, applicationRegistrationRequest)
	if err != nil {
		ac.ErrorResponse(w, r, err)
		return
	}

	ac.JSONResponse(w, r, &appRegistrationUpsertResponse)
}

// ModifyRegistrationDetails Updates specific field(s) of an application registration
func (ac *applicationController) ModifyRegistrationDetails(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation PATCH /applications/{appName} application modifyRegistrationDetails
	// ---
	// summary: Updates specific field(s) of an application registration
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: patchRequest
	//   in: body
	//   description: Request for Application to patch
	//   required: true
	//   schema:
	//       "$ref": "#/definitions/ApplicationRegistrationPatchRequest"
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
	//     description: Modifying registration operation details
	//     schema:
	//       "$ref": "#/definitions/ApplicationRegistrationUpsertResponse"
	//   "400":
	//     description: "Invalid application"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	//   "409":
	//     description: "Conflict"
	appName := mux.Vars(r)["appName"]

	var applicationRegistrationPatchRequest applicationModels.ApplicationRegistrationPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&applicationRegistrationPatchRequest); err != nil {
		ac.ErrorResponse(w, r, err)
		return
	}

	// Need in cluster Radix client in order to validate registration using sufficient privileges
	handler := ac.applicationHandlerFactory.Create(accounts)
	appRegistrationUpsertResponse, err := handler.ModifyRegistrationDetails(r.Context(), appName, applicationRegistrationPatchRequest)
	if err != nil {
		ac.ErrorResponse(w, r, err)
		return
	}

	ac.JSONResponse(w, r, &appRegistrationUpsertResponse)
}

// DeleteApplication Deletes application
func (ac *applicationController) DeleteApplication(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation DELETE /applications/{appName} application deleteApplication
	// ---
	// summary: Delete application
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of application
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
	//     description: "Application deleted ok"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]

	handler := ac.applicationHandlerFactory.Create(accounts)
	err := handler.DeleteApplication(r.Context(), appName)

	if err != nil {
		ac.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// ListPipelines Lists supported pipelines
func (ac *applicationController) ListPipelines(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/pipelines application listPipelines
	// ---
	// summary: Lists the supported pipelines
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     description: "Successful operation"
	//     schema:
	//        type: array
	//        items:
	//           type: string

	// It was suggested to keep this under /applications/{appName} endpoint, but for now this will be the same for all applications
	handler := ac.applicationHandlerFactory.Create(accounts)
	supportedPipelines := handler.GetSupportedPipelines()
	ac.JSONResponse(w, r, supportedPipelines)
}

// TriggerPipelineBuild creates a build pipeline job for the application
func (ac *applicationController) TriggerPipelineBuild(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/pipelines/build application triggerPipelineBuild
	// ---
	// summary: Run a build pipeline for a given application and branch
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: PipelineParametersBuild
	//   description: Pipeline parameters
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/PipelineParametersBuild"
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
	//     description: Successful trigger pipeline
	//     schema:
	//       "$ref": "#/definitions/JobSummary"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	handler := ac.applicationHandlerFactory.Create(accounts)
	jobSummary, err := handler.TriggerPipelineBuild(r.Context(), appName, r)

	if err != nil {
		ac.ErrorResponse(w, r, err)
		return
	}

	ac.JSONResponse(w, r, &jobSummary)
}

// TriggerPipelineBuildDeploy creates a build-deploy pipeline job for the application
func (ac *applicationController) TriggerPipelineBuildDeploy(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/pipelines/build-deploy application triggerPipelineBuildDeploy
	// ---
	// summary: Run a build-deploy pipeline for a given application and branch
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: PipelineParametersBuild
	//   description: Pipeline parameters
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/PipelineParametersBuild"
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
	//     description: Successful trigger pipeline
	//     schema:
	//       "$ref": "#/definitions/JobSummary"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]

	handler := ac.applicationHandlerFactory.Create(accounts)
	jobSummary, err := handler.TriggerPipelineBuildDeploy(r.Context(), appName, r)

	if err != nil {
		ac.ErrorResponse(w, r, err)
		return
	}

	ac.JSONResponse(w, r, &jobSummary)
}

// TriggerPipelineDeploy creates a deploy pipeline job for the application
func (ac *applicationController) TriggerPipelineDeploy(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/pipelines/deploy application triggerPipelineDeploy
	// ---
	// summary: Run a deploy pipeline for a given application and environment
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: PipelineParametersDeploy
	//   description: Pipeline parameters
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/PipelineParametersDeploy"
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
	//     description: Successful trigger pipeline
	//     schema:
	//       "$ref": "#/definitions/JobSummary"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]

	handler := ac.applicationHandlerFactory.Create(accounts)
	jobSummary, err := handler.TriggerPipelineDeploy(r.Context(), appName, r)

	if err != nil {
		ac.ErrorResponse(w, r, err)
		return
	}

	ac.JSONResponse(w, r, &jobSummary)
}

// TriggerPipelineApplyConfig creates an apply config pipeline job for the application
func (ac *applicationController) TriggerPipelineApplyConfig(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/pipelines/apply-config application triggerPipelineApplyConfig
	// ---
	// summary: Run a apply config pipeline for a given application
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: PipelineParametersApplyConfig
	//   description: Pipeline parameters
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/PipelineParametersApplyConfig"
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
	//     description: Successful trigger pipeline
	//     schema:
	//       "$ref": "#/definitions/JobSummary"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]

	handler := ac.applicationHandlerFactory.Create(accounts)
	jobSummary, err := handler.TriggerPipelineApplyConfig(r.Context(), appName, r)

	if err != nil {
		ac.ErrorResponse(w, r, err)
		return
	}

	ac.JSONResponse(w, r, &jobSummary)
}

// TriggerPipelinePromote creates a promote pipeline job for the application
func (ac *applicationController) TriggerPipelinePromote(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/pipelines/promote application triggerPipelinePromote
	// ---
	// summary: Run a promote pipeline for a given application and branch
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: PipelineParametersPromote
	//   description: Pipeline parameters
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/PipelineParametersPromote"
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
	//     description: Successful trigger pipeline
	//     schema:
	//       "$ref": "#/definitions/JobSummary"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]

	handler := ac.applicationHandlerFactory.Create(accounts)
	jobSummary, err := handler.TriggerPipelinePromote(r.Context(), appName, r)

	if err != nil {
		ac.ErrorResponse(w, r, err)
		return
	}

	ac.JSONResponse(w, r, &jobSummary)
}

// GetApplicationResourcesUtilization Gets used resources for the application
func (ac *applicationController) GetApplicationResourcesUtilization(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/utilization application GetApplicationResourcesUtilization
	// ---
	// summary: Gets max resources used by the application
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of the application
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
	//     description: Successful trigger pipeline
	//     schema:
	//       "$ref": "#/definitions/ReplicaResourcesUtilizationResponse"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]

	utilization, err := ac.metricsHandler.GetReplicaResourcesUtilization(r.Context(), accounts.UserAccount.RadixClient, appName, "")
	if err != nil {
		ac.ErrorResponse(w, r, err)
		return
	}

	ac.JSONResponse(w, r, &utilization)
}

// GetEnvironmentResourcesUtilization Gets used resources for the application
func (ac *applicationController) GetEnvironmentResourcesUtilization(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/utilization environment GetEnvironmentResourcesUtilization
	// ---
	// summary: Gets max resources used by the application
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of the application
	//   type: string
	//   required: true
	// - name: envName
	//   in: path
	//   description: Name of the application environment
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
	//     description: Successful trigger pipeline
	//     schema:
	//       "$ref": "#/definitions/ReplicaResourcesUtilizationResponse"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]

	utilization, err := ac.metricsHandler.GetReplicaResourcesUtilization(r.Context(), accounts.UserAccount.RadixClient, appName, envName)
	if err != nil {
		ac.ErrorResponse(w, r, err)
		return
	}

	ac.JSONResponse(w, r, &utilization)
}

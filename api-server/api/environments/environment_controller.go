package environments

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/equinor/radix-operator/api-server/api/deployments"
	environmentsModels "github.com/equinor/radix-operator/api-server/api/environments/models"
	"github.com/equinor/radix-operator/api-server/api/utils/logs"
	"github.com/equinor/radix-operator/api-server/models"
	"github.com/gorilla/mux"
)

const rootPath = "/applications/{appName}"

type environmentController struct {
	*models.DefaultController
	environmentHandlerFactory EnvironmentHandlerFactory
}

// NewEnvironmentController Constructor
func NewEnvironmentController(environmentHandlerFactory EnvironmentHandlerFactory) models.Controller {
	return &environmentController{
		environmentHandlerFactory: environmentHandlerFactory,
	}
}

// GetRoutes List the supported routes of this handler
func (c *environmentController) GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:        rootPath + "/environments/{envName}/deployments",
			Method:      http.MethodGet,
			HandlerFunc: c.GetApplicationEnvironmentDeployments,
		},
		models.Route{
			Path:        rootPath + "/environments",
			Method:      http.MethodGet,
			HandlerFunc: c.GetEnvironmentSummary,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}",
			Method:      http.MethodGet,
			HandlerFunc: c.GetEnvironment,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}",
			Method:      http.MethodPost,
			HandlerFunc: c.CreateEnvironment,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}",
			Method:      http.MethodDelete,
			HandlerFunc: c.DeleteEnvironment,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/events",
			Method:      http.MethodGet,
			HandlerFunc: c.GetEnvironmentEvents,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/events/components/{componentName}",
			Method:      http.MethodGet,
			HandlerFunc: c.GetComponentEvents,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/events/components/{componentName}/replicas/{podName}",
			Method:      http.MethodGet,
			HandlerFunc: c.GetPodEvents,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/components/{componentName}/stop",
			Method:      http.MethodPost,
			HandlerFunc: c.StopComponent,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/components/{componentName}/start",
			Method:      http.MethodPost,
			HandlerFunc: c.ResetScaledComponent,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/components/{componentName}/reset-scale",
			Method:      http.MethodPost,
			HandlerFunc: c.ResetScaledComponent,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/components/{componentName}/restart",
			Method:      http.MethodPost,
			HandlerFunc: c.RestartComponent,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/components/{componentName}/aux/{type}/restart",
			Method:      http.MethodPost,
			HandlerFunc: c.RestartOAuthAuxiliaryResource,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/stop",
			Method:      http.MethodPost,
			HandlerFunc: c.StopEnvironment,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/start",
			Method:      http.MethodPost,
			HandlerFunc: c.ResetManuallyStoppedComponentsInEnvironment,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/reset-scale",
			Method:      http.MethodPost,
			HandlerFunc: c.ResetManuallyStoppedComponentsInEnvironment,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/restart",
			Method:      http.MethodPost,
			HandlerFunc: c.RestartEnvironment,
		},
		models.Route{
			Path:        rootPath + "/stop",
			Method:      http.MethodPost,
			HandlerFunc: c.StopApplication,
		},
		models.Route{
			Path:        rootPath + "/start",
			Method:      http.MethodPost,
			HandlerFunc: c.ResetManuallyScaledComponentsInApplication,
		},
		models.Route{
			Path:        rootPath + "/reset-scale",
			Method:      http.MethodPost,
			HandlerFunc: c.ResetManuallyScaledComponentsInApplication,
		},
		models.Route{
			Path:        rootPath + "/restart",
			Method:      http.MethodPost,
			HandlerFunc: c.RestartApplication,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/components/{componentName}/replicas/{podName}/logs",
			Method:      http.MethodGet,
			HandlerFunc: c.GetPodLog,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/scheduledjobs/{scheduledJobName}/logs",
			Method:      http.MethodGet,
			HandlerFunc: c.GetScheduledJobLog,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/components/{componentName}/aux/{type}/replicas/{podName}/logs",
			Method:      http.MethodGet,
			HandlerFunc: c.GetOAuthAuxiliaryResourcePodLog,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/deployments",
			Method:      http.MethodGet,
			HandlerFunc: c.GetJobComponentDeployments,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/jobs",
			Method:      http.MethodGet,
			HandlerFunc: c.GetJobs,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/jobs/{jobName}",
			Method:      http.MethodGet,
			HandlerFunc: c.GetJob,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/jobs/{jobName}/restart",
			Method:      http.MethodPost,
			HandlerFunc: c.RestartJob,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/jobs/{jobName}/copy",
			Method:      http.MethodPost,
			HandlerFunc: c.CopyJob,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/jobs/{jobName}",
			Method:      http.MethodDelete,
			HandlerFunc: c.DeleteJob,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/jobs/{jobName}/payload",
			Method:      http.MethodGet,
			HandlerFunc: c.GetJobPayload,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/batches",
			Method:      http.MethodGet,
			HandlerFunc: c.GetBatches,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/batches/{batchName}",
			Method:      http.MethodGet,
			HandlerFunc: c.GetBatch,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/batches/{batchName}/restart",
			Method:      http.MethodPost,
			HandlerFunc: c.RestartBatch,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/batches/{batchName}/copy",
			Method:      http.MethodPost,
			HandlerFunc: c.CopyBatch,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/batches/{batchName}",
			Method:      http.MethodDelete,
			HandlerFunc: c.DeleteBatch,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/components/{componentName}/scale/{replicas}",
			Method:      http.MethodPost,
			HandlerFunc: c.ScaleComponent,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/jobs/{jobName}/stop",
			Method:      http.MethodPost,
			HandlerFunc: c.StopJob,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/jobs/stop",
			Method:      http.MethodPost,
			HandlerFunc: c.StopAllJobs,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/batches/{batchName}/stop",
			Method:      http.MethodPost,
			HandlerFunc: c.StopBatch,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/batches/stop",
			Method:      http.MethodPost,
			HandlerFunc: c.StopAllBatches,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/stop",
			Method:      http.MethodPost,
			HandlerFunc: c.StopAllBatchesAndJobsForJobComponent,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/stop",
			Method:      http.MethodPost,
			HandlerFunc: c.StopAllBatchesAndJobsForEnvironment,
		},
	}
	return routes
}

// GetApplicationEnvironmentDeployments Lists the application environment deployments
func (c *environmentController) GetApplicationEnvironmentDeployments(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/deployments environment getApplicationEnvironmentDeployments
	// ---
	// summary: Lists the application environment deployments
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// - name: envName
	//   in: path
	//   description: environment of Radix application
	//   type: string
	//   required: true
	// - name: latest
	//   in: query
	//   description: indicator to allow only listing the latest
	//   type: boolean
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
	//           "$ref": "#/definitions/DeploymentSummary"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	latest := r.FormValue("latest")

	var err error
	var useLatest = false
	if strings.TrimSpace(latest) != "" {
		useLatest, err = strconv.ParseBool(r.FormValue("latest"))
		if err != nil {
			c.ErrorResponse(w, r, err)
			return
		}
	}

	deploymentHandler := deployments.Init(accounts)

	appEnvironmentDeployments, err := deploymentHandler.GetDeploymentsForApplicationEnvironment(r.Context(), appName, envName, useLatest)
	if err != nil {
		c.ErrorResponse(w, r, err)
	}

	c.JSONResponse(w, r, appEnvironmentDeployments)
}

// CreateEnvironment Creates a new environment
func (c *environmentController) CreateEnvironment(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName} environment createEnvironment
	// ---
	// summary: Creates application environment
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// - name: envName
	//   in: path
	//   description: name of environment
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
	//     description: "Environment created ok"
	//   "401":
	//     description: "Unauthorized"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]

	// Need in cluster client in order to delete namespace using sufficient privileges
	environmentHandler := c.environmentHandlerFactory(accounts)
	_, err := environmentHandler.CreateEnvironment(r.Context(), appName, envName)

	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetEnvironment Get details for an application environment
func (c *environmentController) GetEnvironment(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName} environment getEnvironment
	// ---
	// summary: Get details for an application environment
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// - name: envName
	//   in: path
	//   description: name of environment
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
	//     description: "Successful get environment"
	//     schema:
	//        "$ref": "#/definitions/Environment"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]

	environmentHandler := c.environmentHandlerFactory(accounts)
	appEnvironment, err := environmentHandler.GetEnvironment(r.Context(), appName, envName)

	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, appEnvironment)

}

// DeleteEnvironment Deletes environment
func (c *environmentController) DeleteEnvironment(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation DELETE /applications/{appName}/environments/{envName} environment deleteEnvironment
	// ---
	// summary: Deletes application environment
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// - name: envName
	//   in: path
	//   description: name of environment
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
	//     description: "Environment deleted ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]

	environmentHandler := c.environmentHandlerFactory(accounts)
	err := environmentHandler.DeleteEnvironment(r.Context(), appName, envName)

	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetEnvironmentSummary Lists the environments for an application
func (c *environmentController) GetEnvironmentSummary(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments environment getEnvironmentSummary
	// ---
	// summary: Lists the environments for an application
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
	//           "$ref": "#/definitions/EnvironmentSummary"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]

	environmentHandler := c.environmentHandlerFactory(accounts)
	appEnvironments, err := environmentHandler.GetEnvironmentSummary(r.Context(), appName)

	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, appEnvironments)
}

// GetEnvironmentEvents Get events for an application environment
func (c *environmentController) GetEnvironmentEvents(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/events environment getEnvironmentEvents
	// ---
	// summary: Lists events for an application environment
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// - name: envName
	//   in: path
	//   description: name of environment
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
	//     description: "Successful get environment events"
	//     schema:
	//        type: "array"
	//        items:
	//          "$ref": "#/definitions/Event"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]

	environmentHandler := c.environmentHandlerFactory(accounts)
	events, err := environmentHandler.eventHandler.GetEnvironmentEvents(r.Context(), appName, envName)

	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, events)

}

// GetComponentEvents Get events for an application environment component
func (c *environmentController) GetComponentEvents(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/events/components/{componentName} environment getComponentEvents
	// ---
	// summary: Lists events for an application environment
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// - name: envName
	//   in: path
	//   description: name of environment
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
	//     description: "Successful get environment events"
	//     schema:
	//        type: "array"
	//        items:
	//          "$ref": "#/definitions/Event"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	componentName := mux.Vars(r)["componentName"]

	environmentHandler := c.environmentHandlerFactory(accounts)
	events, err := environmentHandler.eventHandler.GetComponentEvents(r.Context(), appName, envName, componentName)

	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, events)
}

// GetPodEvents Get events for an application environment component
func (c *environmentController) GetPodEvents(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/events/components/{componentName}/replicas/{podName} environment getReplicaEvents
	// ---
	// summary: Lists events for an application environment
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// - name: envName
	//   in: path
	//   description: name of environment
	//   type: string
	//   required: true
	// - name: componentName
	//   in: path
	//   description: Name of component
	//   type: string
	//   required: true
	// - name: podName
	//   in: path
	//   description: Name of pod
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
	//     description: "Successful get environment events"
	//     schema:
	//        type: "array"
	//        items:
	//          "$ref": "#/definitions/Event"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	componentName := mux.Vars(r)["componentName"]
	podName := mux.Vars(r)["podName"]

	environmentHandler := c.environmentHandlerFactory(accounts)
	events, err := environmentHandler.eventHandler.GetPodEvents(r.Context(), appName, envName, componentName, podName)

	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, events)
}

// StopComponent Stops job
func (c *environmentController) StopComponent(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/components/{componentName}/stop component stopComponent
	// ---
	// summary: Stops component
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
	//     description: "Component stopped ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	componentName := mux.Vars(r)["componentName"]

	environmentHandler := c.environmentHandlerFactory(accounts)
	err := environmentHandler.StopComponent(r.Context(), appName, envName, componentName, false)

	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, "Success")
}

// ResetScaledComponent reset manually scaled component and resumes normal operation
func (c *environmentController) ResetScaledComponent(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/components/{componentName}/reset-scale component resetScaledComponent
	// ---
	// summary: Reset manually scaled component and resumes normal operation
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
	//     description: "Component started ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	componentName := mux.Vars(r)["componentName"]

	environmentHandler := c.environmentHandlerFactory(accounts)
	err := environmentHandler.ResetScaledComponent(r.Context(), appName, envName, componentName, false)

	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, "Success")
}

// StartComponent Starts job
func (c *environmentController) StartComponent(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/components/{componentName}/start component startComponent
	// ---
	// summary: Deprecated Start component. Use reset-scale instead. This does the same thing, but naming is wrong. This endpoint will be removed after 1. september 2025.
	// deprecated: true
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
	//     description: "Component started ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	componentName := mux.Vars(r)["componentName"]

	environmentHandler := c.environmentHandlerFactory(accounts)
	err := environmentHandler.ResetScaledComponent(r.Context(), appName, envName, componentName, false)

	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, "Success")
}

// RestartComponent Restarts job
func (c *environmentController) RestartComponent(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/components/{componentName}/restart component restartComponent
	// ---
	// summary: |
	//   Restart a component
	//     - Stops running the component container
	//     - Pulls new image from image hub in radix configuration
	//     - Starts the container again using an up-to-date image
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
	//     description: "Component started ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	componentName := mux.Vars(r)["componentName"]

	environmentHandler := c.environmentHandlerFactory(accounts)
	err := environmentHandler.RestartComponent(r.Context(), appName, envName, componentName, false)

	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, "Success")
}

// StopEnvironment  all components in the environment
func (c *environmentController) StopEnvironment(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/stop environment stopEnvironment
	// ---
	// summary: Stops all components in the environment
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
	//     description: "Environment stopped ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]

	environmentHandler := c.environmentHandlerFactory(accounts)
	err := environmentHandler.StopEnvironment(r.Context(), appName, envName)

	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, "Success")
}

// ResetManuallyStoppedComponentsInEnvironment Reset all manually scaled component and resumes normal operation in environment
func (c *environmentController) ResetManuallyStoppedComponentsInEnvironment(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/reset-scale environment resetManuallyScaledComponentsInEnvironment
	// ---
	// summary: Reset all manually scaled component and resumes normal operation in environment
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
	//     description: "Environment started ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]

	environmentHandler := c.environmentHandlerFactory(accounts)
	err := environmentHandler.ResetManuallyStoppedComponentsInEnvironment(r.Context(), appName, envName)

	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, "Success")
}

// StartEnvironment Starts all components in the environment
func (c *environmentController) StartEnvironment(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/start environment startEnvironment
	// ---
	// summary: Deprecated. Use reset-scale instead that does the same thing, but with better naming. This method will be removed after 1. september 2025.
	// deprecated: true
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
	//     description: "Environment started ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]

	environmentHandler := c.environmentHandlerFactory(accounts)
	err := environmentHandler.ResetManuallyStoppedComponentsInEnvironment(r.Context(), appName, envName)

	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, "Success")
}

// RestartEnvironment Restarts all components in the environment
func (c *environmentController) RestartEnvironment(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/restart environment restartEnvironment
	// ---
	// summary: |
	//   Restart all components in the environment
	//     - Stops all running components in the environment
	//     - Pulls new images from image hub in radix configuration
	//     - Starts all components in the environment again using up-to-date image
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
	//     description: "Environment started ok"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]

	environmentHandler := c.environmentHandlerFactory(accounts)
	err := environmentHandler.RestartEnvironment(r.Context(), appName, envName)

	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, "Success")
}

// StopApplication  all components in all environments of the application
func (c *environmentController) StopApplication(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/stop application stopApplication
	// ---
	// summary: Stops all components in the environment
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
	//     description: "Application stopped ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]

	environmentHandler := c.environmentHandlerFactory(accounts)
	err := environmentHandler.StopApplication(r.Context(), appName)

	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, "Success")
}

// ResetManuallyScaledComponentsInApplication Resets and resumes normal opperation for all manually stopped components in all environments of the application
func (c *environmentController) ResetManuallyScaledComponentsInApplication(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/reset-scale application resetManuallyScaledComponentsInApplication
	// ---
	// summary: Resets and resumes normal opperation for all manually stopped components in all environments of the application
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
	//     description: "Application started ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]

	environmentHandler := c.environmentHandlerFactory(accounts)
	err := environmentHandler.StartApplication(r.Context(), appName)

	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, "Success")
}

// StartApplication Starts all components in all environments of the application
func (c *environmentController) StartApplication(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/start application startApplication
	// ---
	// summary: Deprecated. Use reset scale that does the same thing instead. This will be removed after 1. september 2025.
	// deprecated: true
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
	//     description: "Application started ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]

	environmentHandler := c.environmentHandlerFactory(accounts)
	err := environmentHandler.StartApplication(r.Context(), appName)

	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, "Success")
}

// RestartApplication Restarts all components in all environments of the application
func (c *environmentController) RestartApplication(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/restart application restartApplication
	// ---
	// summary: |
	//   Restart all components in all environments of the application
	//     - Stops all running components in all environments of the application
	//     - Pulls new images from image hub in radix configuration
	//     - Starts all components in all environments of the application again using up-to-date image
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
	//     description: "Application started ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]

	environmentHandler := c.environmentHandlerFactory(accounts)
	err := environmentHandler.RestartApplication(r.Context(), appName)

	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, "Success")
}

// RestartOAuthAuxiliaryResource Restarts oauth auxiliary resource for a component
func (c *environmentController) RestartOAuthAuxiliaryResource(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/components/{componentName}/aux/{type}/restart component restartOAuthAuxiliaryResource
	// ---
	// summary: Restarts an auxiliary resource for a component
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
	// - name: type
	//   in: path
	//   description: Type of auxiliary resource (oauth|oauth-redis)
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
	//     description: "Auxiliary resource restarted ok"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "409":
	//     description: "Conflict"
	//   "404":
	//     description: "Not found"
	//   "500":
	//     description: "Internal server error"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	componentName := mux.Vars(r)["componentName"]
	auxType := mux.Vars(r)["type"]

	environmentHandler := c.environmentHandlerFactory(accounts)
	err := environmentHandler.RestartComponentAuxiliaryResource(r.Context(), appName, envName, componentName, auxType)

	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, "Success")
}

// GetPodLog Get logs of a single pod
func (c *environmentController) GetPodLog(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/components/{componentName}/replicas/{podName}/logs component replicaLog
	// ---
	// summary: Get logs from a deployed pod
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
	// - name: podName
	//   in: path
	//   description: Name of pod
	//   type: string
	//   required: true
	// - name: sinceTime
	//   in: query
	//   description: Get log only from sinceTime (example 2020-03-18T07:20:41+00:00)
	//   type: string
	//   format: date-time
	//   required: false
	// - name: lines
	//   in: query
	//   description: Get log lines (example 1000)
	//   type: string
	//   format: number
	//   required: false
	// - name: file
	//   in: query
	//   description: Get log as a file if true
	//   type: string
	//   format: boolean
	//   required: false
	// - name: follow
	//   in: query
	//   description: Get log as a server-sent event stream if true
	//   type: string
	//   format: boolean
	//   required: false
	// - name: previous
	//   in: query
	//   description: Get previous container log if true
	//   type: string
	//   format: boolean
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
	//     description: "pod log"
	//     schema:
	//        type: "string"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	podName := mux.Vars(r)["podName"]

	since, asFile, asFollow, logLines, err, previousLog := logs.GetLogParams(r)
	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	eh := c.environmentHandlerFactory(accounts)
	logs, err := eh.GetLogs(r.Context(), appName, envName, podName, &since, logLines, previousLog, asFollow)
	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}
	defer func() {
		_ = logs.Close()
	}()

	if asFile {
		fileName := fmt.Sprintf("%s.log", time.Now().Format("20060102150405"))
		c.ReaderFileResponse(w, r, logs, fileName, "text/plain; charset=utf-8")
	} else if asFollow {
		c.ReaderEventStreamResponse(w, r, logs)
	} else {
		c.ReaderResponse(w, r, logs, "text/plain; charset=utf-8")
	}
}

// GetScheduledJobLog Get log from a scheduled job
func (c *environmentController) GetScheduledJobLog(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/scheduledjobs/{scheduledJobName}/logs job jobLog
	// ---
	// summary: Get log from a scheduled job
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
	//   type: string
	//   required: true
	// - name: scheduledJobName
	//   in: path
	//   description: Name of scheduled job
	//   type: string
	//   required: true
	// - name: replicaName
	//   in: query
	//   description: Name of the job replica
	//   type: string
	//   required: false
	// - name: sinceTime
	//   in: query
	//   description: Get log only from sinceTime (example 2020-03-18T07:20:41+00:00)
	//   type: string
	//   format: date-time
	//   required: false
	// - name: lines
	//   in: query
	//   description: Get log lines (example 1000)
	//   type: string
	//   format: number
	//   required: false
	// - name: file
	//   in: query
	//   description: Get log as a file if true
	//   type: string
	//   format: boolean
	//   required: false
	// - name: follow
	//   in: query
	//   description: Get log as a server-sent event stream if true
	//   type: string
	//   format: boolean
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
	//     description: "scheduled job log"
	//     schema:
	//        type: "string"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	scheduledJobName := mux.Vars(r)["scheduledJobName"]
	replicaName := r.FormValue("replicaName")

	since, asFile, asFollow, logLines, err, _ := logs.GetLogParams(r)
	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	eh := c.environmentHandlerFactory(accounts)
	logs, err := eh.GetScheduledJobLogs(r.Context(), appName, envName, scheduledJobName, replicaName, &since, logLines, asFollow)
	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}
	defer func() { _ = logs.Close() }()

	if asFile {
		fileName := fmt.Sprintf("%s.log", time.Now().Format("20060102150405"))
		c.ReaderFileResponse(w, r, logs, fileName, "text/plain; charset=utf-8")
	} else if asFollow {
		c.ReaderEventStreamResponse(w, r, logs)
	} else {
		c.ReaderResponse(w, r, logs, "text/plain; charset=utf-8")
	}
}

// GetJobComponentDeployments Get list of deployments for the job component
func (c *environmentController) GetJobComponentDeployments(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/deployments job GetJobComponentDeployments
	// ---
	// summary: Get list of deployments for the job component
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
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
	//     description: "Radix deployments"
	//     schema:
	//        type: array
	//        items:
	//          "$ref": "#/definitions/DeploymentItem"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	jobComponentName := mux.Vars(r)["jobComponentName"]

	eh := c.environmentHandlerFactory(accounts)
	jobComponentDeployments, err := eh.deployHandler.GetJobComponentDeployments(r.Context(), appName, envName, jobComponentName)

	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, jobComponentDeployments)
}

// GetJobs Get list of scheduled jobs
func (c *environmentController) GetJobs(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/jobs job getJobs
	// ---
	// summary: Get list of scheduled jobs
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
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
	//     description: "scheduled jobs"
	//     schema:
	//        type: array
	//        items:
	//          "$ref": "#/definitions/ScheduledJobSummary"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	jobComponentName := mux.Vars(r)["jobComponentName"]

	eh := c.environmentHandlerFactory(accounts)
	jobSummaries, err := eh.GetJobs(r.Context(), appName, envName, jobComponentName)

	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, jobSummaries)
}

// GetJob Get a scheduled job
func (c *environmentController) GetJob(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/jobs/{jobName} job getJob
	// ---
	// summary: Get list of scheduled jobs
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
	//   type: string
	//   required: true
	// - name: jobName
	//   in: path
	//   description: Name of job
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
	//     description: "scheduled job"
	//     schema:
	//        "$ref": "#/definitions/ScheduledJobSummary"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	jobComponentName := mux.Vars(r)["jobComponentName"]
	jobName := mux.Vars(r)["jobName"]

	eh := c.environmentHandlerFactory(accounts)
	jobSummary, err := eh.GetJob(r.Context(), appName, envName, jobComponentName, jobName)

	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, jobSummary)
}

// StopJob Stop a scheduled job
func (c *environmentController) StopJob(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/jobs/{jobName}/stop job stopJob
	// ---
	// summary: Stop scheduled job
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
	//   type: string
	//   required: true
	// - name: jobName
	//   in: path
	//   description: Name of job
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
	//   "204":
	//     description: "Success"
	//   "400":
	//     description: "Invalid job"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	jobComponentName := mux.Vars(r)["jobComponentName"]
	jobName := mux.Vars(r)["jobName"]

	eh := c.environmentHandlerFactory(accounts)
	err := eh.StopJob(r.Context(), appName, envName, jobComponentName, jobName)
	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// StopAllJobs Stop all scheduled jobs
func (c *environmentController) StopAllJobs(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/jobs/stop job stopAllJobs
	// ---
	// summary: Stop all scheduled jobs
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
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
	//   "204":
	//     description: "Success"
	//   "400":
	//     description: "Invalid job"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	jobComponentName := mux.Vars(r)["jobComponentName"]

	eh := c.environmentHandlerFactory(accounts)
	err := eh.StopAllJobs(r.Context(), appName, envName, jobComponentName)
	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RestartJob Start a running or stopped scheduled job
func (c *environmentController) RestartJob(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/jobs/{jobName}/restart job restartJob
	// ---
	// summary: Restart a running or stopped scheduled job
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
	//   type: string
	//   required: true
	// - name: jobName
	//   in: path
	//   description: Name of job
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
	//   "204":
	//     description: "Success"
	//   "400":
	//     description: "Invalid job"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	jobComponentName := mux.Vars(r)["jobComponentName"]
	jobName := mux.Vars(r)["jobName"]

	eh := c.environmentHandlerFactory(accounts)
	err := eh.RestartJob(r.Context(), appName, envName, jobComponentName, jobName)
	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeleteJob Delete a job
func (c *environmentController) DeleteJob(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation DELETE /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/jobs/{jobName} job deleteJob
	// ---
	// summary: Delete job
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
	//   type: string
	//   required: true
	// - name: jobName
	//   in: path
	//   description: Name of job
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
	//   "204":
	//     description: "Success"
	//   "400":
	//     description: "Invalid job"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	jobComponentName := mux.Vars(r)["jobComponentName"]
	jobName := mux.Vars(r)["jobName"]

	eh := c.environmentHandlerFactory(accounts)
	err := eh.DeleteJob(r.Context(), appName, envName, jobComponentName, jobName)
	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetBatches Get list of scheduled batches
func (c *environmentController) GetBatches(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/batches job getBatches
	// ---
	// summary: Get list of scheduled batches
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
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
	//     description: "scheduled batches"
	//     schema:
	//        type: array
	//        items:
	//          "$ref": "#/definitions/ScheduledBatchSummary"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	jobComponentName := mux.Vars(r)["jobComponentName"]

	eh := c.environmentHandlerFactory(accounts)
	batchSummaries, err := eh.GetBatches(r.Context(), appName, envName, jobComponentName)

	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, batchSummaries)
}

// GetBatch Get a scheduled batch
func (c *environmentController) GetBatch(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/batches/{batchName} job getBatch
	// ---
	// summary: Get list of scheduled batches
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
	//   type: string
	//   required: true
	// - name: batchName
	//   in: path
	//   description: Name of batch
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
	//     description: "scheduled batch"
	//     schema:
	//        "$ref": "#/definitions/ScheduledBatchSummary"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	jobComponentName := mux.Vars(r)["jobComponentName"]
	batchName := mux.Vars(r)["batchName"]

	eh := c.environmentHandlerFactory(accounts)
	batchSummary, err := eh.GetBatch(r.Context(), appName, envName, jobComponentName, batchName)

	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, batchSummary)
}

// StopBatch Stop a scheduled batch
func (c *environmentController) StopBatch(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/batches/{batchName}/stop job stopBatch
	// ---
	// summary: Stop scheduled batch
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
	//   type: string
	//   required: true
	// - name: batchName
	//   in: path
	//   description: Name of batch
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
	//   "204":
	//     description: "Success"
	//   "400":
	//     description: "Invalid batch"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	jobComponentName := mux.Vars(r)["jobComponentName"]
	batchName := mux.Vars(r)["batchName"]

	eh := c.environmentHandlerFactory(accounts)
	err := eh.StopBatch(r.Context(), appName, envName, jobComponentName, batchName)
	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// StopAllBatches Stop all scheduled batches
func (c *environmentController) StopAllBatches(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/batches/stop job stopAllBatches
	// ---
	// summary: Stop scheduled batch
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
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
	//   "204":
	//     description: "Success"
	//   "400":
	//     description: "Invalid batch"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	jobComponentName := mux.Vars(r)["jobComponentName"]

	eh := c.environmentHandlerFactory(accounts)
	err := eh.StopAllBatches(r.Context(), appName, envName, jobComponentName)
	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// StopAllBatchesAndJobsForJobComponent Stop all scheduled batches in the job-component
func (c *environmentController) StopAllBatchesAndJobsForJobComponent(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/stop job stopAllBatchesAndJobsForJobComponent
	// ---
	// summary: Stop all scheduled batches for the job-component
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
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
	//   "204":
	//     description: "Success"
	//   "400":
	//     description: "Invalid batch"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	jobComponentName := mux.Vars(r)["jobComponentName"]

	eh := c.environmentHandlerFactory(accounts)
	err := eh.StopAllBatchesAndJobsForJobComponent(r.Context(), appName, envName, jobComponentName)
	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// StopAllBatchesAndJobsForEnvironment Stop all scheduled batches and jobs in the environment
func (c *environmentController) StopAllBatchesAndJobsForEnvironment(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/jobcomponents/stop job stopAllBatchesAndJobsForEnvironment
	// ---
	// summary: Stop all scheduled batches and jobs in the environment
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
	//   "204":
	//     description: "Success"
	//   "400":
	//     description: "Invalid batch"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]

	eh := c.environmentHandlerFactory(accounts)
	err := eh.StopAllBatchesAndJobsForEnvironment(r.Context(), appName, envName)
	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RestartBatch Restart a scheduled or stopped batch
func (c *environmentController) RestartBatch(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/batches/{batchName}/restart job restartBatch
	// ---
	// summary: Restart a scheduled or stopped batch
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
	//   type: string
	//   required: true
	// - name: batchName
	//   in: path
	//   description: Name of batch
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
	//   "204":
	//     description: "Success"
	//   "400":
	//     description: "Invalid batch"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	jobComponentName := mux.Vars(r)["jobComponentName"]
	batchName := mux.Vars(r)["batchName"]

	eh := c.environmentHandlerFactory(accounts)
	err := eh.RestartBatch(r.Context(), appName, envName, jobComponentName, batchName)
	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CopyBatch Create a copy of existing scheduled batch with optional changes
func (c *environmentController) CopyBatch(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/batches/{batchName}/copy job copyBatch
	// ---
	// summary: Create a copy of existing scheduled batch with optional changes
	// parameters:
	// - name: scheduledBatchRequest
	//   in: body
	//   description: Request for creating a scheduled batch
	//   required: true
	//   schema:
	//       "$ref": "#/definitions/ScheduledBatchRequest"
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
	//   type: string
	//   required: true
	// - name: batchName
	//   in: path
	//   description: Name of batch to be copied
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
	//     description: "Success"
	//     schema:
	//        "$ref": "#/definitions/ScheduledBatchSummary"
	//   "400":
	//     description: "Invalid batch"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	jobComponentName := mux.Vars(r)["jobComponentName"]
	batchName := mux.Vars(r)["batchName"]
	var scheduledBatchRequest environmentsModels.ScheduledBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&scheduledBatchRequest); err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	eh := c.environmentHandlerFactory(accounts)
	batchSummary, err := eh.CopyBatch(r.Context(), appName, envName, jobComponentName, batchName, scheduledBatchRequest)
	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, batchSummary)
}

// CopyJob Create a copy of existing scheduled job with optional changes
func (c *environmentController) CopyJob(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/jobs/{jobName}/copy job copyJob
	// ---
	// summary: Create a copy of existing scheduled job with optional changes
	// parameters:
	// - name: scheduledJobRequest
	//   in: body
	//   description: Request for creating a scheduled job
	//   required: true
	//   schema:
	//       "$ref": "#/definitions/ScheduledJobRequest"
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
	//   type: string
	//   required: true
	// - name: jobName
	//   in: path
	//   description: Name of job to be copied
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
	//     description: "Success"
	//     schema:
	//        "$ref": "#/definitions/ScheduledJobSummary"
	//   "400":
	//     description: "Invalid batch"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	jobComponentName := mux.Vars(r)["jobComponentName"]
	jobName := mux.Vars(r)["jobName"]
	var scheduledJobRequest environmentsModels.ScheduledJobRequest
	if err := json.NewDecoder(r.Body).Decode(&scheduledJobRequest); err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	eh := c.environmentHandlerFactory(accounts)
	jobSummary, err := eh.CopyJob(r.Context(), appName, envName, jobComponentName, jobName, scheduledJobRequest)
	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, jobSummary)
}

// DeleteBatch Delete a batch
func (c *environmentController) DeleteBatch(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation DELETE /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/batches/{batchName} job deleteBatch
	// ---
	// summary: Delete batch
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
	//   type: string
	//   required: true
	// - name: batchName
	//   in: path
	//   description: Name of batch
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
	//   "204":
	//     description: "Success"
	//   "400":
	//     description: "Invalid batch"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	jobComponentName := mux.Vars(r)["jobComponentName"]
	batchName := mux.Vars(r)["batchName"]

	eh := c.environmentHandlerFactory(accounts)
	err := eh.DeleteBatch(r.Context(), appName, envName, jobComponentName, batchName)
	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetOAuthAuxiliaryResourcePodLog Get log for a single auxiliary resource pod
func (c *environmentController) GetOAuthAuxiliaryResourcePodLog(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/components/{componentName}/aux/{type}/replicas/{podName}/logs component getOAuthPodLog
	// ---
	// summary: Get logs for an oauth auxiliary resource pod
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
	// - name: type
	//   in: path
	//   description: Type of auxiliary resource (oauth|oauth-redis)
	//   type: string
	//   required: true
	// - name: podName
	//   in: path
	//   description: Name of pod
	//   type: string
	//   required: true
	// - name: sinceTime
	//   in: query
	//   description: Get log only from sinceTime (example 2020-03-18T07:20:41+00:00)
	//   type: string
	//   format: date-time
	//   required: false
	// - name: lines
	//   in: query
	//   description: Get log lines (example 1000)
	//   type: string
	//   format: number
	//   required: false
	// - name: file
	//   in: query
	//   description: Get log as a file if true
	//   type: string
	//   format: boolean
	//   required: false
	// - name: follow
	//   in: query
	//   description: Get log as a server-sent event stream if true
	//   type: string
	//   format: boolean
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
	//     description: "pod log"
	//     schema:
	//        type: "string"
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
	auxType := mux.Vars(r)["type"]
	componentName := mux.Vars(r)["componentName"]
	podName := mux.Vars(r)["podName"]

	since, asFile, asFollow, logLines, err, _ := logs.GetLogParams(r)
	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	eh := c.environmentHandlerFactory(accounts)
	logs, err := eh.GetAuxiliaryResourcePodLog(r.Context(), appName, envName, componentName, auxType, podName, &since, logLines, asFollow)
	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}
	defer func() { _ = logs.Close() }()

	if asFile {
		fileName := fmt.Sprintf("%s.log", time.Now().Format("20060102150405"))
		c.ReaderFileResponse(w, r, logs, fileName, "text/plain; charset=utf-8")
	} else if asFollow {
		c.ReaderEventStreamResponse(w, r, logs)
	} else {
		c.ReaderResponse(w, r, logs, "text/plain; charset=utf-8")
	}
}

// GetJobPayload Get a scheduled job payload
func (c *environmentController) GetJobPayload(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/jobs/{jobName}/payload job getJobPayload
	// ---
	// summary: Get payload of a scheduled job
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
	//   type: string
	//   required: true
	// - name: jobName
	//   in: path
	//   description: Name of job
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
	//     description: "scheduled job payload"
	//     schema:
	//        type: "string"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	jobComponentName := mux.Vars(r)["jobComponentName"]
	jobName := mux.Vars(r)["jobName"]

	eh := c.environmentHandlerFactory(accounts)
	payload, err := eh.GetJobPayload(r.Context(), appName, envName, jobComponentName, jobName)

	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.ReaderResponse(w, r, payload, "text/plain; charset=utf-8")
}

// ScaleComponent Scale component replicas
func (c *environmentController) ScaleComponent(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/components/{componentName}/scale/{replicas} component scaleComponent
	// ---
	// summary: Scale a component replicas
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
	// - name: replicas
	//   in: path
	//   description: New desired number of replicas
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
	//   "204":
	//     description: "Success"
	//   "400":
	//     description: "Invalid component"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	componentName := mux.Vars(r)["componentName"]
	replicas, err := strconv.Atoi(mux.Vars(r)["replicas"])
	if err != nil {
		c.ErrorResponse(w, r, fmt.Errorf("invalid new desired number of replicas argument"))
		return
	}

	eh := c.environmentHandlerFactory(accounts)
	err = eh.ScaleComponent(r.Context(), appName, envName, componentName, replicas)
	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

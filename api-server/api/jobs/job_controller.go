package jobs

import (
	"fmt"
	"net/http"
	"time"

	"github.com/equinor/radix-operator/api-server/api/deployments"
	"github.com/equinor/radix-operator/api-server/api/utils/logs"
	"github.com/equinor/radix-operator/api-server/models"
	"github.com/gorilla/mux"
)

const rootPath = "/applications/{appName}"

type jobController struct {
	*models.DefaultController
}

// NewJobController Constructor
func NewJobController() models.Controller {
	return &jobController{}
}

// GetRoutes List the supported routes of this handler
func (jc *jobController) GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:        rootPath + "/jobs",
			Method:      "GET",
			HandlerFunc: jc.GetApplicationJobs,
		},
		models.Route{
			Path:        rootPath + "/jobs/{jobName}",
			Method:      "GET",
			HandlerFunc: jc.GetApplicationJob,
		},
		models.Route{
			Path:        rootPath + "/jobs/{jobName}/stop",
			Method:      "POST",
			HandlerFunc: jc.StopApplicationJob,
		},
		models.Route{
			Path:        rootPath + "/jobs/{jobName}/rerun",
			Method:      "POST",
			HandlerFunc: jc.RerunApplicationJob,
		},
		models.Route{
			Path:        rootPath + "/jobs/{jobName}/pipelineruns",
			Method:      "GET",
			HandlerFunc: jc.GetTektonPipelineRuns,
		},
		models.Route{
			Path:        rootPath + "/jobs/{jobName}/pipelineruns/{pipelineRunName}",
			Method:      "GET",
			HandlerFunc: jc.GetTektonPipelineRun,
		},
		models.Route{
			Path:        rootPath + "/jobs/{jobName}/pipelineruns/{pipelineRunName}/tasks",
			Method:      "GET",
			HandlerFunc: jc.GetTektonPipelineRunTasks,
		},
		models.Route{
			Path:        rootPath + "/jobs/{jobName}/pipelineruns/{pipelineRunName}/tasks/{taskName}",
			Method:      "GET",
			HandlerFunc: jc.GetTektonPipelineRunTask,
		},
		models.Route{
			Path:        rootPath + "/jobs/{jobName}/pipelineruns/{pipelineRunName}/tasks/{taskName}/steps",
			Method:      "GET",
			HandlerFunc: jc.GetTektonPipelineRunTaskSteps,
		},
		models.Route{
			Path:        rootPath + "/jobs/{jobName}/pipelineruns/{pipelineRunName}/tasks/{taskName}/step/{stepName}",
			Method:      "GET",
			HandlerFunc: jc.GetTektonPipelineRunTaskStep,
		},
		models.Route{
			Path:        rootPath + "/jobs/{jobName}/pipelineruns/{pipelineRunName}/tasks/{taskName}/logs/{stepName}",
			Method:      "GET",
			HandlerFunc: jc.GetTektonPipelineRunTaskStepLogs,
		},
		models.Route{
			Path:        rootPath + "/jobs/{jobName}/logs/{stepName}",
			Method:      "GET",
			HandlerFunc: jc.GetPipelineJobStepLogs,
		},
	}

	return routes
}

// GetApplicationJobs gets pipeline-job summaries
func (jc *jobController) GetApplicationJobs(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/jobs pipeline-job getApplicationJobs
	// ---
	// summary: Gets the summary of jobs for a given application
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
	//           "$ref": "#/definitions/JobSummary"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]

	handler := Init(accounts, deployments.Init(accounts))
	jobSummaries, err := handler.GetApplicationJobs(r.Context(), appName)

	if err != nil {
		jc.ErrorResponse(w, r, err)
		return
	}

	jc.JSONResponse(w, r, jobSummaries)
}

// GetApplicationJob gets specific pipeline-job details
func (jc *jobController) GetApplicationJob(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/jobs/{jobName} pipeline-job getApplicationJob
	// ---
	// summary: Gets the detail of a given pipeline-job for a given application
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// - name: jobName
	//   in: path
	//   description: name of job
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
	//     description: "Successful get job"
	//     schema:
	//        "$ref": "#/definitions/Job"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	jobName := mux.Vars(r)["jobName"]

	handler := Init(accounts, deployments.Init(accounts))
	jobDetail, err := handler.GetApplicationJob(r.Context(), appName, jobName)

	if err != nil {
		jc.ErrorResponse(w, r, err)
		return
	}

	jc.JSONResponse(w, r, jobDetail)
}

// StopApplicationJob Stops job
func (jc *jobController) StopApplicationJob(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/jobs/{jobName}/stop pipeline-job stopApplicationJob
	// ---
	// summary: Stops job
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of application
	//   type: string
	//   required: true
	// - name: jobName
	//   in: path
	//   description: name of job
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
	//     description: "Job stopped ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	jobName := mux.Vars(r)["jobName"]

	handler := Init(accounts, deployments.Init(accounts))
	err := handler.StopJob(r.Context(), appName, jobName)

	if err != nil {
		jc.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RerunApplicationJob Reruns the pipeline job
func (jc *jobController) RerunApplicationJob(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/jobs/{jobName}/rerun pipeline-job rerunApplicationJob
	// ---
	// summary: Reruns the pipeline job
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of application
	//   type: string
	//   required: true
	// - name: jobName
	//   in: path
	//   description: name of job
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
	//     description: "Job rerun ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	jobName := mux.Vars(r)["jobName"]
	handler := Init(accounts, deployments.Init(accounts))
	err := handler.RerunJob(r.Context(), appName, jobName)

	if err != nil {
		jc.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetTektonPipelineRuns Get the Tekton pipeline runs overview
func (jc *jobController) GetTektonPipelineRuns(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/jobs/{jobName}/pipelineruns pipeline-job getTektonPipelineRuns
	// ---
	// summary: Gets list of pipeline runs for a pipeline-job
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// - name: jobName
	//   in: path
	//   description: Name of pipeline job
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
	//     description: "List of PipelineRun-s"
	//     schema:
	//        type: "array"
	//        items:
	//           "$ref": "#/definitions/PipelineRun"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	jobName := mux.Vars(r)["jobName"]

	handler := Init(accounts, deployments.Init(accounts))
	tektonPipelineRuns, err := handler.GetTektonPipelineRuns(r.Context(), appName, jobName)

	if err != nil {
		jc.ErrorResponse(w, r, err)
		return
	}

	jc.JSONResponse(w, r, tektonPipelineRuns)
}

// GetTektonPipelineRun Get the Tekton pipeline run overview
func (jc *jobController) GetTektonPipelineRun(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/jobs/{jobName}/pipelineruns/{pipelineRunName} pipeline-job getTektonPipelineRun
	// ---
	// summary: Gets a pipeline run for a pipeline-job
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// - name: jobName
	//   in: path
	//   description: Name of pipeline job
	//   type: string
	//   required: true
	// - name: pipelineRunName
	//   in: path
	//   description: Name of pipeline run
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
	//     description: "List of Pipeline Runs"
	//     schema:
	//       "$ref": "#/definitions/PipelineRun"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	jobName := mux.Vars(r)["jobName"]
	pipelineRunName := mux.Vars(r)["pipelineRunName"]

	handler := Init(accounts, deployments.Init(accounts))
	tektonPipelineRun, err := handler.GetTektonPipelineRun(r.Context(), appName, jobName, pipelineRunName)

	if err != nil {
		jc.ErrorResponse(w, r, err)
		return
	}

	jc.JSONResponse(w, r, tektonPipelineRun)
}

// GetTektonPipelineRunTasks Get the Tekton task list of a pipeline run
func (jc *jobController) GetTektonPipelineRunTasks(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/jobs/{jobName}/pipelineruns/{pipelineRunName}/tasks pipeline-job getTektonPipelineRunTasks
	// ---
	// summary: Gets list of pipeline run tasks of a pipeline-job
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// - name: jobName
	//   in: path
	//   description: Name of pipeline job
	//   type: string
	//   required: true
	// - name: pipelineRunName
	//   in: path
	//   description: Name of pipeline run
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
	//     description: "List of Pipeline Run Tasks"
	//     schema:
	//        type: "array"
	//        items:
	//           "$ref": "#/definitions/PipelineRunTask"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	jobName := mux.Vars(r)["jobName"]
	pipelineRunName := mux.Vars(r)["pipelineRunName"]

	handler := Init(accounts, deployments.Init(accounts))
	tektonTasks, err := handler.GetTektonPipelineRunTasks(r.Context(), appName, jobName, pipelineRunName)

	if err != nil {
		jc.ErrorResponse(w, r, err)
		return
	}

	jc.JSONResponse(w, r, tektonTasks)
}

// GetTektonPipelineRunTask Get the Tekton task of a pipeline run
func (jc *jobController) GetTektonPipelineRunTask(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/jobs/{jobName}/pipelineruns/{pipelineRunName}/tasks/{taskName} pipeline-job getTektonPipelineRunTask
	// ---
	// summary: Gets list of pipeline run task of a pipeline-job
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// - name: jobName
	//   in: path
	//   description: Name of pipeline job
	//   type: string
	//   required: true
	// - name: pipelineRunName
	//   in: path
	//   description: Name of pipeline run
	//   type: string
	//   required: true
	// - name: taskName
	//   in: path
	//   description: Name of pipeline run task
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
	//     description: "Pipeline Run Task"
	//     schema:
	//        $ref: "#/definitions/PipelineRunTask"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	jobName := mux.Vars(r)["jobName"]
	pipelineRunName := mux.Vars(r)["pipelineRunName"]
	taskName := mux.Vars(r)["taskName"]

	handler := Init(accounts, deployments.Init(accounts))
	tektonTasks, err := handler.GetTektonPipelineRunTask(r.Context(), appName, jobName, pipelineRunName, taskName)

	if err != nil {
		jc.ErrorResponse(w, r, err)
		return
	}

	jc.JSONResponse(w, r, tektonTasks)
}

// GetTektonPipelineRunTaskSteps Get the Tekton task step list of a pipeline run
func (jc *jobController) GetTektonPipelineRunTaskSteps(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/jobs/{jobName}/pipelineruns/{pipelineRunName}/tasks/{taskName}/steps pipeline-job getTektonPipelineRunTaskSteps
	// ---
	// summary: Gets list of steps for a pipeline run task of a pipeline-job
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// - name: jobName
	//   in: path
	//   description: Name of pipeline job
	//   type: string
	//   required: true
	// - name: pipelineRunName
	//   in: path
	//   description: Name of pipeline run
	//   type: string
	//   required: true
	// - name: taskName
	//   in: path
	//   description: Name of pipeline run task
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
	//     description: "List of Pipeline Run Task Steps"
	//     schema:
	//        type: "array"
	//        items:
	//           "$ref": "#/definitions/PipelineRunTaskStep"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	jobName := mux.Vars(r)["jobName"]
	pipelineRunName := mux.Vars(r)["pipelineRunName"]
	taskName := mux.Vars(r)["taskName"]

	handler := Init(accounts, deployments.Init(accounts))
	tektonTaskSteps, err := handler.GetTektonPipelineRunTaskSteps(r.Context(), appName, jobName, pipelineRunName, taskName)

	if err != nil {
		jc.ErrorResponse(w, r, err)
		return
	}

	jc.JSONResponse(w, r, tektonTaskSteps)
}

// GetTektonPipelineRunTaskStep Get the Tekton task step of a pipeline run
func (jc *jobController) GetTektonPipelineRunTaskStep(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/jobs/{jobName}/pipelineruns/{pipelineRunName}/tasks/{taskName}/step/{stepName} pipeline-job getTektonPipelineRunTaskStep
	// ---
	// summary: Gets a step for a pipeline run task of a pipeline-job
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// - name: jobName
	//   in: path
	//   description: Name of pipeline job
	//   type: string
	//   required: true
	// - name: pipelineRunName
	//   in: path
	//   description: Name of pipeline run
	//   type: string
	//   required: true
	// - name: taskName
	//   in: path
	//   description: Name of pipeline run task
	//   type: string
	//   required: true
	// - name: stepName
	//   in: path
	//   description: Name of pipeline run task step
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
	//     description: "List of Pipeline Run Task Steps"
	//     schema:
	//        "$ref": "#/definitions/Step"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	jobName := mux.Vars(r)["jobName"]
	pipelineRunName := mux.Vars(r)["pipelineRunName"]
	taskName := mux.Vars(r)["taskName"]
	stepName := mux.Vars(r)["stepName"]

	handler := Init(accounts, deployments.Init(accounts))
	taskStep, err := handler.GetTektonPipelineRunTaskStep(r.Context(), appName, jobName, pipelineRunName, taskName, stepName)

	if err != nil {
		jc.ErrorResponse(w, r, err)
		return
	}

	jc.JSONResponse(w, r, taskStep)
}

// GetTektonPipelineRunTaskStepLogs Get step logs of a pipeline run task for a pipeline job
func (jc *jobController) GetTektonPipelineRunTaskStepLogs(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/jobs/{jobName}/pipelineruns/{pipelineRunName}/tasks/{taskName}/logs/{stepName} pipeline-job getTektonPipelineRunTaskStepLogs
	// ---
	// summary: Gets logs of pipeline runs for a pipeline-job
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// - name: jobName
	//   in: path
	//   description: Name of pipeline job
	//   type: string
	//   required: true
	// - name: pipelineRunName
	//   in: path
	//   description: Name of pipeline run
	//   type: string
	//   required: true
	// - name: taskName
	//   in: path
	//   description: Name of pipeline run task
	//   type: string
	//   required: true
	// - name: stepName
	//   in: path
	//   description: Name of pipeline run task step
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
	//     description: "Task step log"
	//     schema:
	//        type: "string"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	jobName := mux.Vars(r)["jobName"]
	pipelineRunName := mux.Vars(r)["pipelineRunName"]
	taskName := mux.Vars(r)["taskName"]
	stepName := mux.Vars(r)["stepName"]
	since, asFile, asFollow, logLines, err, _ := logs.GetLogParams(r)
	if err != nil {
		jc.ErrorResponse(w, r, err)
		return
	}

	handler := Init(accounts, deployments.Init(accounts))
	log, err := handler.GetTektonPipelineRunTaskStepLogs(r.Context(), appName, jobName, pipelineRunName, taskName, stepName, &since, logLines, asFollow)
	if err != nil {
		jc.ErrorResponse(w, r, err)
		return
	}
	defer func() { _ = log.Close() }()

	if asFile {
		fileName := fmt.Sprintf("%s.log", time.Now().Format("20060102150405"))
		jc.ReaderFileResponse(w, r, log, fileName, "text/plain; charset=utf-8")
	} else if asFollow {
		jc.ReaderEventStreamResponse(w, r, log)
	} else {
		jc.ReaderResponse(w, r, log, "text/plain; charset=utf-8")
	}
}

// GetPipelineJobStepLogs Get log of a pipeline job step
func (jc *jobController) GetPipelineJobStepLogs(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/jobs/{jobName}/logs/{stepName} pipeline-job getPipelineJobStepLogs
	// ---
	// summary: Gets logs of a pipeline job step
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// - name: jobName
	//   in: path
	//   description: Name of the pipeline job
	//   type: string
	//   required: true
	// - name: stepName
	//   in: path
	//   description: Name of the pipeline job step
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
	//     description: "Job step log"
	//     schema:
	//        type: "string"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	jobName := mux.Vars(r)["jobName"]
	stepName := mux.Vars(r)["stepName"]
	since, asFile, asFollow, logLines, err, _ := logs.GetLogParams(r)
	if err != nil {
		jc.ErrorResponse(w, r, err)
		return
	}

	handler := Init(accounts, deployments.Init(accounts))
	log, err := handler.GetPipelineJobStepLogs(r.Context(), appName, jobName, stepName, &since, logLines, asFollow)
	if err != nil {
		jc.ErrorResponse(w, r, err)
		return
	}
	defer func() { _ = log.Close() }()

	if asFile {
		fileName := fmt.Sprintf("%s.log", time.Now().Format("20060102150405"))
		jc.ReaderFileResponse(w, r, log, fileName, "text/plain; charset=utf-8")
	} else if asFollow {
		jc.ReaderEventStreamResponse(w, r, log)
	} else {
		jc.ReaderResponse(w, r, log, "text/plain; charset=utf-8")
	}
}

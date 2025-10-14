package jobs

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/equinor/radix-operator/job-scheduler/api/v1/controllers"
	handlers "github.com/equinor/radix-operator/job-scheduler/api/v1/handlers/jobs"
	models "github.com/equinor/radix-operator/job-scheduler/models/common"
	apiErrors "github.com/equinor/radix-operator/job-scheduler/pkg/errors"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const jobNameParam = "jobName"

type jobController struct {
	*controllers.ControllerBase
	handler handlers.JobHandler
}

// New create a new job controller
func New(handler handlers.JobHandler) controllers.Controller {
	return &jobController{
		handler: handler,
	}
}

// GetRoutes List the supported routes of this controller
func (controller *jobController) GetRoutes() []controllers.Route {
	routes := []controllers.Route{
		{
			Path:    "/jobs",
			Method:  http.MethodPost,
			Handler: controller.CreateJob,
		},
		{
			Path:    "/jobs",
			Method:  http.MethodGet,
			Handler: controller.GetJobs,
		},
		{
			Path:    fmt.Sprintf("/jobs/:%s", jobNameParam),
			Method:  http.MethodGet,
			Handler: controller.GetJob,
		},
		{
			Path:    fmt.Sprintf("/jobs/:%s", jobNameParam),
			Method:  http.MethodDelete,
			Handler: controller.DeleteJob,
		},
		{
			Path:    fmt.Sprintf("/jobs/:%s/stop", jobNameParam),
			Method:  http.MethodPost,
			Handler: controller.StopJob,
		},
		{
			Path:    "/jobs/stop",
			Method:  http.MethodPost,
			Handler: controller.StopAllJobs,
		},
	}
	return routes
}

// CreateJob Create a new job
func (controller *jobController) CreateJob(c *gin.Context) {
	// swagger:operation POST /jobs Job createJob
	// ---
	// summary: Create job
	// parameters:
	// - name: jobCreation
	//   in: body
	//   description: Job to create
	//   required: true
	//   schema:
	//   	 "$ref": "#/definitions/JobScheduleDescription"
	//
	// responses:
	//	 "200":
	//	   description: "Successful create job"
	//	   schema:
	//	     "$ref": "#/definitions/JobStatus"
	//	 "400":
	//	   description: "Bad request"
	//	   schema:
	//	     "$ref": "#/definitions/Status"
	//	 "404":
	//	   description: "Not found"
	//	   schema:
	//	     "$ref": "#/definitions/Status"
	//	 "422":
	//	   description: "Invalid data in request"
	//	   schema:
	//	     "$ref": "#/definitions/Status"
	//	 "500":
	//	   description: "Internal server error"
	//	   schema:
	//	     "$ref": "#/definitions/Status"
	logger := log.Ctx(c.Request.Context())
	logger.Info().Msg("Create Job")
	logger.Debug().Msgf("Read the request body. Request content length %d", c.Request.ContentLength)

	var jobScheduleDescription models.JobScheduleDescription
	if body, _ := io.ReadAll(c.Request.Body); len(body) > 0 {
		logger.Debug().Msgf("Read %d bytes", len(body))

		if err := json.Unmarshal(body, &jobScheduleDescription); err != nil {
			_ = c.Error(err)
			controller.HandleError(c, apiErrors.NewInvalid("payload"))
			return
		}
	}

	jobState, err := controller.handler.CreateJob(c.Request.Context(), &jobScheduleDescription)
	if err != nil {
		controller.HandleError(c, err)
		return
	}

	logger.Info().Msgf("Job %s has been created", jobState.Name)

	c.JSON(http.StatusOK, jobState)
}

// GetJobs Get all jobs
func (controller *jobController) GetJobs(c *gin.Context) {
	// swagger:operation GET /jobs/ Job getJobs
	// ---
	// summary: Gets jobs
	// responses:
	//	 "200":
	//	   description: "Successful get jobs"
	//	   schema:
	//	     type: "array"
	//	     items:
	//	        "$ref": "#/definitions/JobStatus"
	//	 "500":
	//	   description: "Internal server error"
	//	   schema:
	//	     "$ref": "#/definitions/Status"
	logger := log.Ctx(c.Request.Context())
	logger.Info().Msg("Get job list")
	jobs, err := controller.handler.GetJobs(c.Request.Context())
	if err != nil {
		controller.HandleError(c, err)
		return
	}
	logger.Debug().Msgf("Found %d jobs", len(jobs))
	c.JSON(http.StatusOK, jobs)
}

// GetJob Gets job
func (controller *jobController) GetJob(c *gin.Context) {
	// swagger:operation GET /jobs/{jobName} Job getJob
	// ---
	// summary: Gets job
	// parameters:
	// - name: jobName
	//   in: path
	//   description: Name of job
	//   type: string
	//   required: true
	//
	// responses:
	//	 "200":
	//	   description: "Successful get job"
	//	   schema:
	//	     "$ref": "#/definitions/JobStatus"
	//	 "404":
	//	   description: "Not found"
	//	   schema:
	//	     "$ref": "#/definitions/Status"
	//	 "500":
	//	   description: "Internal server error"
	//	   schema:
	//	     "$ref": "#/definitions/Status"
	jobName := c.Param(jobNameParam)
	logger := log.Ctx(c.Request.Context())
	logger.Info().Msgf("Get job %s", jobName)
	job, err := controller.handler.GetJob(c.Request.Context(), jobName)
	if err != nil {
		controller.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, job)
}

// DeleteJob Delete job
func (controller *jobController) DeleteJob(c *gin.Context) {
	// swagger:operation DELETE /jobs/{jobName} Job deleteJob
	// ---
	// summary: Delete job
	// parameters:
	// - name: jobName
	//   in: path
	//   description: Name of job
	//   type: string
	//   required: true
	//
	// responses:
	//	 "200":
	//	   description: "Successful delete job"
	//	   schema:
	//	     "$ref": "#/definitions/Status"
	//	 "404":
	//	   description: "Not found"
	//	   schema:
	//	     "$ref": "#/definitions/Status"
	//	 "500":
	//	   description: "Internal server error"
	//	   schema:
	//	     "$ref": "#/definitions/Status"
	jobName := c.Param(jobNameParam)
	logger := log.Ctx(c.Request.Context())
	logger.Info().Msgf("Delete job %s", jobName)
	err := controller.handler.DeleteJob(c.Request.Context(), jobName)
	if err != nil {
		controller.HandleError(c, err)
		return
	}

	logger.Info().Msgf("Job %s has been deleted", jobName)
	status := models.Status{
		Status:  models.StatusSuccess,
		Code:    http.StatusOK,
		Message: fmt.Sprintf("job %s successfully deleted", jobName),
	}
	c.JSON(http.StatusOK, &status)
}

// StopJob Stop job
func (controller *jobController) StopJob(c *gin.Context) {
	// swagger:operation POST /jobs/{jobName}/stop Job stopJob
	// ---
	// summary: Stop job
	// parameters:
	// - name: jobName
	//   in: path
	//   description: Name of job
	//   type: string
	//   required: true
	//
	// responses:
	//	 "200":
	//	   description: "Successful delete job"
	//	   schema:
	//	     "$ref": "#/definitions/Status"
	//	 "400":
	//	   description: "Bad request"
	//	   schema:
	//	     "$ref": "#/definitions/Status"
	//	 "404":
	//	   description: "Not found"
	//	   schema:
	//	     "$ref": "#/definitions/Status"
	//	 "500":
	//	   description: "Internal server error"
	//	   schema:
	//	     "$ref": "#/definitions/Status"
	jobName := c.Param(jobNameParam)
	logger := log.Ctx(c.Request.Context())
	logger.Info().Msgf("Stop the job %s", jobName)

	err := controller.handler.StopJob(c.Request.Context(), jobName)
	if err != nil {
		controller.HandleError(c, err)
		return
	}

	logger.Info().Msgf("Job %s has been stopped", jobName)
	status := models.Status{
		Status:  models.StatusSuccess,
		Code:    http.StatusOK,
		Message: fmt.Sprintf("job %s was successfully stopped", jobName),
	}
	c.JSON(http.StatusOK, &status)
}

// StopAllJobs Stop all jobs
func (controller *jobController) StopAllJobs(c *gin.Context) {
	// swagger:operation POST /jobs/stop Job stopAllJobs
	// ---
	// summary: Stop all jobs
	// responses:
	//	 "200":
	//	   description: "Successfully stopped jobs"
	//	   schema:
	//	     "$ref": "#/definitions/Status"
	//	 "400":
	//	   description: "Bad request"
	//	   schema:
	//	     "$ref": "#/definitions/Status"
	//	 "404":
	//	   description: "Not found"
	//	   schema:
	//	     "$ref": "#/definitions/Status"
	//	 "500":
	//	   description: "Internal server error"
	//	   schema:
	//	     "$ref": "#/definitions/Status"
	logger := log.Ctx(c.Request.Context())

	err := controller.handler.StopAllJobs(c.Request.Context())
	if err != nil {
		controller.HandleError(c, err)
		return
	}

	logger.Info().Msg("All jobs have been stopped")
	status := models.Status{
		Status:  models.StatusSuccess,
		Code:    http.StatusOK,
		Message: "all jobs where successfully stopped",
	}
	c.JSON(http.StatusOK, &status)
}

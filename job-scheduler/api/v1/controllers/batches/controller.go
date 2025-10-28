package batch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/equinor/radix-operator/job-scheduler/api/v1/controllers"
	handlers "github.com/equinor/radix-operator/job-scheduler/api/v1/handlers/batches"
	models "github.com/equinor/radix-operator/job-scheduler/models/common"
	apiErrors "github.com/equinor/radix-operator/job-scheduler/pkg/errors"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const (
	batchNameParam = "batchName"
	jobNameParam   = "jobName"
)

type batchController struct {
	*controllers.ControllerBase
	handler handlers.BatchHandler
}

// New create a new batch controller
func New(handler handlers.BatchHandler) controllers.Controller {
	return &batchController{
		handler: handler,
	}
}

// GetRoutes List the supported routes of this controller
func (controller *batchController) GetRoutes() []controllers.Route {
	routes := []controllers.Route{
		{
			Path:    "/batches",
			Method:  http.MethodPost,
			Handler: controller.CreateBatch,
		},
		{
			Path:    "/batches",
			Method:  http.MethodGet,
			Handler: controller.GetBatches,
		},
		{
			Path:    fmt.Sprintf("/batches/:%s", batchNameParam),
			Method:  http.MethodGet,
			Handler: controller.GetBatch,
		},
		{
			Path:    fmt.Sprintf("/batches/:%s/jobs/:%s", batchNameParam, jobNameParam),
			Method:  http.MethodGet,
			Handler: controller.GetBatchJob,
		},
		{
			Path:    fmt.Sprintf("/batches/:%s", batchNameParam),
			Method:  http.MethodDelete,
			Handler: controller.DeleteBatch,
		},
		{
			Path:    fmt.Sprintf("/batches/:%s/stop", batchNameParam),
			Method:  http.MethodPost,
			Handler: controller.StopBatch,
		},
		{
			Path:    "/batches/stop",
			Method:  http.MethodPost,
			Handler: controller.StopAllBatches,
		},
		{
			Path:    fmt.Sprintf("/batches/:%s/jobs/:%s/stop", batchNameParam, jobNameParam),
			Method:  http.MethodPost,
			Handler: controller.StopBatchJob,
		},
	}
	return routes
}

// CreateBatch Create batch
func (controller *batchController) CreateBatch(c *gin.Context) {
	// swagger:operation POST /batches Batch createBatch
	// ---
	// summary: Create batch
	// parameters:
	// - name: batchCreation
	//   in: body
	//   description: Batch to create
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/BatchScheduleDescription"
	//
	// responses:
	//	 "200":
	//	   description: "Successful create batch"
	//	   schema:
	//	     "$ref": "#/definitions/BatchStatus"
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
	logger.Info().Msg("Create Batch")

	var batchScheduleDescription models.BatchScheduleDescription
	logger.Debug().Msgf("Read the request body. Request content length %d", c.Request.ContentLength)
	if body, _ := io.ReadAll(c.Request.Body); len(body) > 0 {
		logger.Debug().Msgf("Read %d bytes", len(body))
		if err := json.Unmarshal(body, &batchScheduleDescription); err != nil {
			_ = c.Error(err)
			controller.HandleError(c, apiErrors.NewInvalid("BatchScheduleDescription"))
			return
		}
	}

	batchState, err := controller.handler.CreateBatch(c.Request.Context(), &batchScheduleDescription)
	if err != nil {
		controller.HandleError(c, err)
		return
	}
	logger.Info().Msgf("Batch %s has been created", batchState.Name)

	c.JSON(http.StatusOK, batchState)
}

// GetBatches Get batches
func (controller *batchController) GetBatches(c *gin.Context) {
	// swagger:operation GET /batches/ Batch getBatches
	// ---
	// summary: Gets batches
	// parameters:
	// responses:
	//	 "200":
	//	   description: "Successful get batches"
	//	   schema:
	//	    type: "array"
	//	    items:
	//	      "$ref": "#/definitions/BatchStatus"
	//	 "500":
	//	   description: "Internal server error"
	//	   schema:
	//	     "$ref": "#/definitions/Status"
	logger := log.Ctx(c.Request.Context())
	logger.Info().Msg("Get batch list")
	batches, err := controller.handler.GetBatches(c.Request.Context())
	if err != nil {
		controller.HandleError(c, err)
		return
	}
	logger.Debug().Msgf("Found %d batches", len(batches))
	c.JSON(http.StatusOK, batches)
}

// GetBatch Gets batch
func (controller *batchController) GetBatch(c *gin.Context) {
	// swagger:operation GET /batches/{batchName} Batch getBatch
	// ---
	// summary: Gets batch
	// parameters:
	// - name: batchName
	//   in: path
	//   description: Name of batch
	//   type: string
	//   required: true
	//
	// responses:
	//	 "200":
	//	   description: "Successful get batch"
	//	   schema:
	//	     "$ref": "#/definitions/BatchStatus"
	//	 "404":
	//	   description: "Not found"
	//	   schema:
	//	     "$ref": "#/definitions/Status"
	//	 "500":
	//	   description: "Internal server error"
	//	   schema:
	//	     "$ref": "#/definitions/Status"
	batchName := c.Param(batchNameParam)
	logger := log.Ctx(c.Request.Context())
	logger.Info().Msgf("Get batch %s", batchName)
	batch, err := controller.handler.GetBatch(c.Request.Context(), batchName)
	if err != nil {
		controller.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, batch)
}

// GetBatchJob Gets batch job
func (controller *batchController) GetBatchJob(c *gin.Context) {
	// swagger:operation GET /batches/{batchName}/jobs/{jobName} Batch getBatchJob
	// ---
	// summary: Gets batch job
	// parameters:
	// - name: batchName
	//   in: path
	//   description: Name of batch
	//   type: string
	//   required: true
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
	batchName := c.Param(batchNameParam)
	jobName := c.Param(jobNameParam)
	logger := log.Ctx(c.Request.Context())
	logger.Info().Msgf("Get job %s from the batch %s", jobName, batchName)
	job, err := controller.handler.GetBatchJob(c.Request.Context(), batchName, jobName)
	if err != nil {
		controller.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, job)
}

// DeleteBatch Delete batch
func (controller *batchController) DeleteBatch(c *gin.Context) {
	// swagger:operation DELETE /batches/{batchName} Batch deleteBatch
	// ---
	// summary: Delete batch
	// parameters:
	// - name: batchName
	//   in: path
	//   description: Name of batch
	//   type: string
	//   required: true
	//
	// responses:
	//	 "200":
	//	   description: "Successful delete batch"
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
	batchName := c.Param(batchNameParam)
	logger := log.Ctx(c.Request.Context())
	logger.Info().Msgf("Delete batch %s", batchName)
	err := controller.handler.DeleteBatch(c.Request.Context(), batchName)
	if err != nil {
		controller.HandleError(c, err)
		return
	}

	logger.Info().Msgf("Batch %s has been deleted", batchName)
	status := models.Status{
		Status:  models.StatusSuccess,
		Code:    http.StatusOK,
		Message: fmt.Sprintf("batch %s successfully deleted", batchName),
	}
	c.JSON(http.StatusOK, &status)
}

// StopBatch Stop batch
func (controller *batchController) StopBatch(c *gin.Context) {
	// swagger:operation POST /batches/{batchName}/stop Batch stopBatch
	// ---
	// summary: Stop batch
	// parameters:
	// - name: batchName
	//   in: path
	//   description: Name of batch
	//   type: string
	//   required: true
	//
	// responses:
	//	 "200":
	//	   description: "Successful stop batch"
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
	batchName := c.Param(batchNameParam)
	logger := log.Ctx(c.Request.Context())
	logger.Info().Msgf("Stop Batch %s", batchName)
	err := controller.handler.StopBatch(c.Request.Context(), batchName)
	if err != nil {
		controller.HandleError(c, err)
		return
	}

	logger.Info().Msgf("Batch %s has been stopped", batchName)
	status := models.Status{
		Status:  models.StatusSuccess,
		Code:    http.StatusOK,
		Message: fmt.Sprintf("batch %s successfully stopped", batchName),
	}
	c.JSON(http.StatusOK, &status)
}

// StopBatchJob Stop batch job
func (controller *batchController) StopBatchJob(c *gin.Context) {
	// swagger:operation POST /batches/{batchName}/jobs/{jobName}/stop Batch stopBatchJob
	// ---
	// summary: Stop batch job
	// parameters:
	// - name: batchName
	//   in: path
	//   description: Name of batch
	//   type: string
	//   required: true
	// - name: jobName
	//   in: path
	//   description: Name of job
	//   type: string
	//   required: true
	// responses:
	//	 "200":
	//	   description: "Successful stop batch job"
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
	batchName := c.Param(batchNameParam)
	jobName := c.Param(jobNameParam)
	logger := log.Ctx(c.Request.Context())

	logger.Info().Msgf("Stop the job %s in the batch %s ", jobName, batchName)
	err := controller.handler.StopBatchJob(c.Request.Context(), batchName, jobName)
	if err != nil {
		controller.HandleError(c, err)
		return
	}

	logger.Info().Msgf("Job %s in the batch %s has been stopped", jobName, batchName)
	status := models.Status{
		Status:  models.StatusSuccess,
		Code:    http.StatusOK,
		Message: fmt.Sprintf("job %s in the batch %s successfully stopped", jobName, batchName),
	}
	c.JSON(http.StatusOK, &status)
}

// StopAllBatches Stop all batches
func (controller *batchController) StopAllBatches(c *gin.Context) {
	// swagger:operation POST /batches/stop Batch stopAllBatches
	// ---
	// summary: Stop all batches
	// responses:
	//	 "200":
	//	   description: "Successfully stopped batches"
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
	logger.Info().Msg("Stop all batches")
	err := controller.handler.StopAllBatches(c.Request.Context())
	if err != nil {
		controller.HandleError(c, err)
		return
	}

	logger.Info().Msg("Batches have been stopped")
	status := models.Status{
		Status:  models.StatusSuccess,
		Code:    http.StatusOK,
		Message: "batches successfully stopped",
	}
	c.JSON(http.StatusOK, &status)
}

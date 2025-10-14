package jobs

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/job-scheduler/api/v1/handlers/jobs"
	"github.com/equinor/radix-operator/job-scheduler/api/v1/handlers/jobs/mock"
	"github.com/equinor/radix-operator/job-scheduler/internal/test"
	models "github.com/equinor/radix-operator/job-scheduler/models/common"
	modelsV1 "github.com/equinor/radix-operator/job-scheduler/models/v1"
	apiErrors "github.com/equinor/radix-operator/job-scheduler/pkg/errors"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(handler jobs.JobHandler) *test.ControllerTestUtils {
	jobController := jobController{handler: handler}
	controllerTestUtils := test.NewControllerTestUtils(&jobController)
	return &controllerTestUtils
}

func TestGetJobs(t *testing.T) {
	t.Run("Get jobs - success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		jobHandler := mock.NewMockJobHandler(ctrl)
		jobState := modelsV1.JobStatus{
			Name:    "jobname",
			Started: pointers.Ptr(time.Now()),
			Ended:   pointers.Ptr(time.Now().Add(1 * time.Minute)),
			Status:  "jobstatus",
		}
		ctx := context.Background()
		jobHandler.
			EXPECT().
			GetJobs(test.RequestContextMatcher{}).
			Return([]modelsV1.JobStatus{jobState}, nil).
			Times(1)

		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodGet, "api/v1/jobs")
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
			var returnedJobs []modelsV1.JobStatus
			err := test.GetResponseBody(response, &returnedJobs)
			require.NoError(t, err)
			assert.Len(t, returnedJobs, 1)
			assert.Equal(t, jobState.Name, returnedJobs[0].Name)
			assert.Equal(t, "", returnedJobs[0].BatchName)
			assert.WithinDuration(t, *jobState.Started, *returnedJobs[0].Started, 1)
			assert.WithinDuration(t, *jobState.Ended, *returnedJobs[0].Ended, 1)
			assert.Equal(t, jobState.Status, returnedJobs[0].Status)
		}
	})

	t.Run("Get jobs - status code 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		jobHandler := mock.NewMockJobHandler(ctrl)
		ctx := context.Background()
		jobHandler.
			EXPECT().
			GetJobs(test.RequestContextMatcher{}).
			Return(nil, errors.New("unhandled error")).
			Times(1)

		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodGet, "api/v1/jobs")
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
			var returnedStatus models.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusInternalServerError, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonUnknown, returnedStatus.Reason)
		}
	})
}

func TestGetJob(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		jobName := "jobname"
		jobHandler := mock.NewMockJobHandler(ctrl)
		jobState := modelsV1.JobStatus{
			Name:    jobName,
			Started: pointers.Ptr(time.Now()),
			Ended:   pointers.Ptr(time.Now().Add(1 * time.Minute)),
			Status:  "jobstatus",
		}
		ctx := context.Background()
		jobHandler.
			EXPECT().
			GetJob(test.RequestContextMatcher{}, jobName).
			Return(&jobState, nil).
			Times(1)

		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v1/jobs/%s", jobName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
			var returnedJob modelsV1.JobStatus
			err := test.GetResponseBody(response, &returnedJob)
			require.NoError(t, err)
			assert.Equal(t, jobState.Name, returnedJob.Name)
			assert.Equal(t, "", returnedJob.BatchName)
			assert.WithinDuration(t, *jobState.Started, *returnedJob.Started, 1)
			assert.WithinDuration(t, *jobState.Ended, *returnedJob.Ended, 1)
			assert.Equal(t, jobState.Status, returnedJob.Status)
		}
	})

	t.Run("not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		jobName, kind := "anyjob", "job"
		jobHandler := mock.NewMockJobHandler(ctrl)
		ctx := context.Background()
		jobHandler.
			EXPECT().
			GetJob(test.RequestContextMatcher{}, gomock.Any()).
			Return(nil, apiErrors.NewNotFound(kind, jobName)).
			Times(1)

		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v1/jobs/%s", jobName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusNotFound, response.StatusCode)
			var returnedStatus models.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusNotFound, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonNotFound, returnedStatus.Reason)
			assert.Equal(t, apiErrors.NotFoundMessage(kind, jobName), returnedStatus.Message)
		}
	})

	t.Run("internal error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		jobHandler := mock.NewMockJobHandler(ctrl)
		ctx := context.Background()
		jobHandler.
			EXPECT().
			GetJob(test.RequestContextMatcher{}, gomock.Any()).
			Return(nil, errors.New("unhandled error")).
			Times(1)

		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v1/jobs/%s", "anyjob"))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
			var returnedStatus models.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusInternalServerError, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonUnknown, returnedStatus.Reason)
		}
	})
}

func TestCreateJob(t *testing.T) {
	t.Run("empty body - successful", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		jobScheduleDescription := models.JobScheduleDescription{}
		createdJob := modelsV1.JobStatus{
			Name:    "newjob",
			Started: pointers.Ptr(time.Now()),
			Ended:   pointers.Ptr(time.Now().Add(1 * time.Minute)),
			Status:  "jobstatus",
		}
		jobHandler := mock.NewMockJobHandler(ctrl)
		ctx := context.Background()
		jobHandler.
			EXPECT().
			CreateJob(test.RequestContextMatcher{}, &jobScheduleDescription).
			Return(&createdJob, nil).
			Times(1)
		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequestWithBody(ctx, http.MethodPost, "/api/v1/jobs", nil)
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
			var returnedJob modelsV1.JobStatus
			err := test.GetResponseBody(response, &returnedJob)
			require.NoError(t, err)
			assert.Equal(t, createdJob.Name, returnedJob.Name)
			assert.Equal(t, "", returnedJob.BatchName)
			assert.WithinDuration(t, *createdJob.Started, *returnedJob.Started, 1)
			assert.WithinDuration(t, *createdJob.Ended, *returnedJob.Ended, 1)
			assert.Equal(t, createdJob.Status, returnedJob.Status)
		}
	})

	t.Run("valid payload body - successful", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		jobScheduleDescription := models.JobScheduleDescription{
			Payload: "a_payload",
			RadixJobComponentConfig: models.RadixJobComponentConfig{
				Resources: &models.Resources{
					Requests: models.ResourceList{
						"cpu":    "20m",
						"memory": "256M",
					},
					Limits: models.ResourceList{
						"cpu":    "10m",
						"memory": "128M",
					},
				},
				Node: &models.Node{
					Gpu:      "nvidia",
					GpuCount: "6",
				},
			},
		}
		createdJob := modelsV1.JobStatus{
			Name:    "newjob",
			Started: pointers.Ptr(time.Now()),
			Ended:   pointers.Ptr(time.Now().Add(1 * time.Minute)),
			Status:  "jobstatus",
		}
		jobHandler := mock.NewMockJobHandler(ctrl)
		ctx := context.Background()
		jobHandler.
			EXPECT().
			CreateJob(test.RequestContextMatcher{}, &jobScheduleDescription).
			Return(&createdJob, nil).
			Times(1)
		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequestWithBody(ctx, http.MethodPost, "/api/v1/jobs", jobScheduleDescription)
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
			var returnedJob modelsV1.JobStatus
			err := test.GetResponseBody(response, &returnedJob)
			require.NoError(t, err)
			assert.Equal(t, createdJob.Name, returnedJob.Name)
			assert.Equal(t, "", returnedJob.BatchName)
			assert.WithinDuration(t, *createdJob.Started, *returnedJob.Started, 1)
			assert.WithinDuration(t, *createdJob.Ended, *returnedJob.Ended, 1)
			assert.Equal(t, createdJob.Status, returnedJob.Status)
		}
	})

	t.Run("invalid request body - unprocessable", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		jobHandler := mock.NewMockJobHandler(ctrl)
		ctx := context.Background()
		jobHandler.
			EXPECT().
			CreateJob(test.RequestContextMatcher{}, gomock.Any()).
			Times(0)
		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequestWithBody(ctx, http.MethodPost, "/api/v1/jobs", struct{ Payload interface{} }{Payload: struct{}{}})
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
			var returnedStatus models.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusUnprocessableEntity, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonInvalid, returnedStatus.Reason)
			assert.Equal(t, apiErrors.InvalidMessage("payload", ""), returnedStatus.Message)
		}
	})

	t.Run("handler returning NotFound error - 404 not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		jobScheduleDescription := models.JobScheduleDescription{}
		jobHandler := mock.NewMockJobHandler(ctrl)
		anyKind, anyName := "anyKind", "anyName"
		ctx := context.Background()
		jobHandler.
			EXPECT().
			CreateJob(test.RequestContextMatcher{}, &jobScheduleDescription).
			Return(nil, apiErrors.NewNotFound(anyKind, anyName)).
			Times(1)
		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodPost, "/api/v1/jobs")
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusNotFound, response.StatusCode)
			var returnedStatus models.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusNotFound, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonNotFound, returnedStatus.Reason)
			assert.Equal(t, apiErrors.NotFoundMessage(anyKind, anyName), returnedStatus.Message)
		}
	})

	t.Run("handler returning unhandled error - 500 internal server error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		jobScheduleDescription := models.JobScheduleDescription{}
		jobHandler := mock.NewMockJobHandler(ctrl)
		ctx := context.Background()
		jobHandler.
			EXPECT().
			CreateJob(test.RequestContextMatcher{}, &jobScheduleDescription).
			Return(nil, errors.New("any error")).
			Times(1)
		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodPost, "/api/v1/jobs")
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
			var returnedStatus models.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusInternalServerError, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonUnknown, returnedStatus.Reason)
		}
	})
}

func TestDeleteJob(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		jobName := "anyjob"
		jobHandler := mock.NewMockJobHandler(ctrl)
		ctx := context.Background()
		jobHandler.
			EXPECT().
			DeleteJob(test.RequestContextMatcher{}, jobName).
			Return(nil).
			Times(1)
		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/jobs/%s", jobName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
			var returnedStatus models.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, returnedStatus.Code)
			assert.Equal(t, models.StatusSuccess, returnedStatus.Status)
			assert.Empty(t, returnedStatus.Reason)
		}
	})

	t.Run("handler returning not found - 404 not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		jobName := "anyjob"
		jobHandler := mock.NewMockJobHandler(ctrl)
		ctx := context.Background()
		jobHandler.
			EXPECT().
			DeleteJob(test.RequestContextMatcher{}, jobName).
			Return(apiErrors.NewNotFound("job", jobName)).
			Times(1)
		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/jobs/%s", jobName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusNotFound, response.StatusCode)
			var returnedStatus models.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusNotFound, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonNotFound, returnedStatus.Reason)
			assert.Equal(t, apiErrors.NotFoundMessage("job", jobName), returnedStatus.Message)
		}
	})

	t.Run("handler returning unhandled error - 500 internal server error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		jobName := "anyjob"
		jobHandler := mock.NewMockJobHandler(ctrl)
		ctx := context.Background()
		jobHandler.
			EXPECT().
			DeleteJob(test.RequestContextMatcher{}, jobName).
			Return(errors.New("any error")).
			Times(1)
		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/jobs/%s", jobName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
			var returnedStatus models.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusInternalServerError, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonUnknown, returnedStatus.Reason)
		}
	})
}

func TestStopJob(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		jobName := "anyjob"
		jobHandler := mock.NewMockJobHandler(ctrl)
		ctx := context.Background()
		jobHandler.
			EXPECT().
			StopJob(test.RequestContextMatcher{}, jobName).
			Return(nil).
			Times(1)
		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodPost, fmt.Sprintf("/api/v1/jobs/%s/stop", jobName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
			var returnedStatus models.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, returnedStatus.Code)
			assert.Equal(t, models.StatusSuccess, returnedStatus.Status)
			assert.Empty(t, returnedStatus.Reason)
		}
	})

	t.Run("handler returning not found - 404 not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		jobName := "anyjob"
		jobHandler := mock.NewMockJobHandler(ctrl)
		ctx := context.Background()
		jobHandler.
			EXPECT().
			StopJob(test.RequestContextMatcher{}, jobName).
			Return(apiErrors.NewNotFound("job", jobName)).
			Times(1)
		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodPost, fmt.Sprintf("/api/v1/jobs/%s/stop", jobName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusNotFound, response.StatusCode)
			var returnedStatus models.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusNotFound, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonNotFound, returnedStatus.Reason)
			assert.Equal(t, apiErrors.NotFoundMessage("job", jobName), returnedStatus.Message)
		}
	})

	t.Run("handler returning unhandled error - 500 internal server error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		jobName := "anyjob"
		jobHandler := mock.NewMockJobHandler(ctrl)
		ctx := context.Background()
		jobHandler.
			EXPECT().
			StopJob(test.RequestContextMatcher{}, jobName).
			Return(errors.New("any error")).
			Times(1)
		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodPost, fmt.Sprintf("/api/v1/jobs/%s/stop", jobName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
			var returnedStatus models.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusInternalServerError, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonUnknown, returnedStatus.Reason)
		}
	})
}

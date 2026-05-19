package jobs

import (
	"context"
	"testing"
	"time"

	certclientfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	deployMock "github.com/equinor/radix-operator/api-server/api/deployments/mock"
	deploymentModels "github.com/equinor/radix-operator/api-server/api/deployments/models"
	jobModels "github.com/equinor/radix-operator/api-server/api/jobs/models"
	"github.com/equinor/radix-operator/api-server/models"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	tektonclientfake "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	"go.uber.org/mock/gomock"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

type JobHandlerTestSuite struct {
	suite.Suite
	accounts             models.Accounts
	kubeClient           *kubefake.Clientset
	radixClient          *radixfake.Clientset
	secretProviderClient *secretproviderfake.Clientset
	certClient           *certclientfake.Clientset
	kedaClient           *kedafake.Clientset
	tektonClient         *tektonclientfake.Clientset
}

type jobCreatedScenario struct {
	scenarioName      string
	jobName           string
	jobStatusCreated  metav1.Time
	creationTimestamp metav1.Time
	expectedCreated   string
}

type jobStatusScenario struct {
	scenarioName   string
	jobName        string
	condition      radixv1.RadixJobCondition
	stop           bool
	expectedStatus string
}

type jobProperties struct {
	name      string
	condition radixv1.RadixJobCondition
	stop      bool
}

type jobRerunScenario struct {
	scenarioName   string
	existingJob    *jobProperties
	jobNameToRerun string
	expectedError  error
}

func TestRunJobHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(JobHandlerTestSuite))
}

func (s *JobHandlerTestSuite) SetupTest() {
	s.setupTest()
}

func (s *JobHandlerTestSuite) setupTest() {
	s.kubeClient, s.radixClient, s.kedaClient, s.secretProviderClient, s.certClient, s.tektonClient = s.getUtils()
	accounts := models.NewAccounts(s.kubeClient, s.radixClient, s.kedaClient, s.secretProviderClient, s.tektonClient, s.certClient, s.kubeClient, s.radixClient, s.kedaClient, s.secretProviderClient, s.tektonClient, s.certClient)
	s.accounts = accounts
}

func (s *JobHandlerTestSuite) getUtils() (*kubefake.Clientset, *radixfake.Clientset, *kedafake.Clientset, *secretproviderfake.Clientset, *certclientfake.Clientset, *tektonclientfake.Clientset) {
	kubeClient := kubefake.NewSimpleClientset()   //nolint:staticcheck
	radixClient := radixfake.NewSimpleClientset() //nolint:staticcheck
	kedaClient := kedafake.NewSimpleClientset()
	tektonClient := tektonclientfake.NewSimpleClientset() //nolint:staticcheck
	secretProviderClient := secretproviderfake.NewSimpleClientset()
	certClient := certclientfake.NewSimpleClientset()
	return kubeClient, radixClient, kedaClient, secretProviderClient, certClient, tektonClient
}

func (s *JobHandlerTestSuite) Test_GetApplicationJob() {
	jobName, appName, someTag, commitId, pipeline, triggeredBy := "a_job", "an_app", "a_tag", "a_commitid", radixv1.BuildDeploy, "a_user"
	started, ended := metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local)), metav1.NewTime(time.Date(2020, 1, 2, 0, 0, 0, 0, time.Local))
	step1Name, step1Pod, step1Condition, step1Started, step1Ended, step1Components := "step1_name", "step1_pod", radixv1.JobRunning, metav1.Now(), metav1.NewTime(time.Now().Add(1*time.Hour)), []string{"step1_comp1", "step1_comp2"}
	step2Name := "step2_name"

	rj := &radixv1.RadixJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: utils.GetAppNamespace(appName),
		},
		Spec: radixv1.RadixJobSpec{
			Build: radixv1.RadixBuildSpec{
				GitRef:                someTag,
				GitRefType:            radixv1.GitRefTag,
				CommitID:              commitId,
				OverrideUseBuildCache: pointers.Ptr(true),
				RefreshBuildCache:     pointers.Ptr(true),
			},
			PipeLineType: pipeline,
			TriggeredBy:  triggeredBy,
		},
		Status: radixv1.RadixJobStatus{
			Started: &started,
			Ended:   &ended,
			Steps: []radixv1.RadixJobStep{
				{Name: step1Name, PodName: step1Pod, Condition: step1Condition, Started: &step1Started, Ended: &step1Ended, Components: step1Components},
				{Name: step2Name},
			},
		},
	}
	_, err := s.radixClient.RadixV1().RadixJobs(rj.Namespace).Create(context.Background(), rj, metav1.CreateOptions{})
	s.NoError(err)

	deploymentName := "a_deployment"
	comp1Name, comp1Type, comp1Image := "comp1", "type1", "image1"
	comp2Name, comp2Type, comp2Image := "comp2", "type2", "image2"
	deploySummary := deploymentModels.DeploymentSummary{
		Name:        deploymentName,
		Environment: "any_env",
		ActiveFrom:  time.Time{},
		ActiveTo:    &time.Time{},
		Components: []*deploymentModels.ComponentSummary{
			{Name: comp1Name, Image: comp1Image, Type: comp1Type},
			{Name: comp2Name, Image: comp2Image, Type: comp2Type},
		},
		DeploymentSummaryPipelineJobInfo: deploymentModels.DeploymentSummaryPipelineJobInfo{
			CreatedByJob: "any_job",
		},
	}

	s.Run("radixjob does not exist", func() {
		ctrl := gomock.NewController(s.T())
		defer ctrl.Finish()
		dh := deployMock.NewMockDeployHandler(ctrl)
		h := Init(s.accounts, dh)
		actualJob, err := h.GetApplicationJob(context.Background(), appName, "missing_job")
		s.True(k8serrors.IsNotFound(err))
		s.Nil(actualJob)
	})

	s.Run("deployHandle.GetDeploymentsForPipelineJob return error", func() {
		ctrl := gomock.NewController(s.T())
		defer ctrl.Finish()

		dh := deployMock.NewMockDeployHandler(ctrl)
		dh.EXPECT().GetDeploymentsForPipelineJob(context.Background(), appName, jobName).Return(nil, assert.AnError).Times(1)
		h := Init(s.accounts, dh)

		actualJob, actualErr := h.GetApplicationJob(context.Background(), appName, jobName)
		s.Equal(assert.AnError, actualErr)
		s.Nil(actualJob)
	})

	s.Run("valid jobSummary", func() {
		ctrl := gomock.NewController(s.T())
		defer ctrl.Finish()

		deployList := []*deploymentModels.DeploymentSummary{&deploySummary}
		dh := deployMock.NewMockDeployHandler(ctrl)
		dh.EXPECT().GetDeploymentsForPipelineJob(context.Background(), appName, jobName).Return(deployList, nil).Times(1)
		h := Init(s.accounts, dh)

		actualJob, actualErr := h.GetApplicationJob(context.Background(), appName, jobName)
		s.NoError(actualErr)
		s.Equal(jobName, actualJob.Name)
		s.Equal(someTag, actualJob.GitRef)
		s.Equal(string(radixv1.GitRefTag), actualJob.GitRefType)
		s.Equal(commitId, actualJob.CommitID)
		s.NotNil(actualJob.OverrideUseBuildCache)
		s.True(*actualJob.OverrideUseBuildCache)
		s.NotNil(actualJob.RefreshBuildCache)
		s.True(*actualJob.RefreshBuildCache)
		s.Equal(triggeredBy, actualJob.TriggeredBy)
		s.Equal(radixutils.FormatTime(&started), actualJob.Started)
		s.Equal(radixutils.FormatTime(&ended), actualJob.Ended)
		s.Equal(string(pipeline), actualJob.Pipeline)
		s.ElementsMatch(deployList, actualJob.Deployments)

		expectedComponents := []deploymentModels.ComponentSummary{
			{Name: comp1Name, Type: comp1Type, Image: comp1Image},
			{Name: comp2Name, Type: comp2Type, Image: comp2Image},
		}

		//nolint:staticcheck
		//lint:ignore SA1019 we want to make sure that Components is populated for backward compatibility (at least for a while)
		s.ElementsMatch(slice.PointersOf(expectedComponents), actualJob.Components)
		expectedSteps := []jobModels.Step{
			{Name: step1Name, PodName: step1Pod, Status: string(step1Condition), Started: &step1Started.Time, Ended: &step1Ended.Time, Components: step1Components},
			{Name: step2Name},
		}
		s.ElementsMatch(expectedSteps, actualJob.Steps)
	})
}

func (s *JobHandlerTestSuite) Test_GetApplicationJob_Created() {
	appName, emptyTime := "any_app", metav1.Time{}
	scenarios := []jobCreatedScenario{
		{scenarioName: "both creation time and status.Created is empty", jobName: "job1", expectedCreated: ""},
		{scenarioName: "use CreationTimeStamp", jobName: "job2", expectedCreated: "2020-01-01T00:00:00Z", creationTimestamp: metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))},
		{scenarioName: "use Created from Status", jobName: "job3", expectedCreated: "2020-01-02T00:00:00Z", creationTimestamp: metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)), jobStatusCreated: metav1.NewTime(time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC))},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.scenarioName, func() {
			ctrl := gomock.NewController(s.T())
			defer ctrl.Finish()

			dh := deployMock.NewMockDeployHandler(ctrl)
			dh.EXPECT().GetDeploymentsForPipelineJob(context.Background(), gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			h := Init(s.accounts, dh)
			rj := radixv1.RadixJob{ObjectMeta: metav1.ObjectMeta{Name: scenario.jobName, Namespace: utils.GetAppNamespace(appName), CreationTimestamp: scenario.creationTimestamp}}
			if scenario.jobStatusCreated != emptyTime {
				rj.Status.Created = &scenario.jobStatusCreated
			}
			_, err := s.radixClient.RadixV1().RadixJobs(rj.Namespace).Create(context.Background(), &rj, metav1.CreateOptions{})
			s.NoError(err)
			actualJob, err := h.GetApplicationJob(context.Background(), appName, scenario.jobName)
			s.NoError(err)
			s.Equal(scenario.expectedCreated, actualJob.Created)
		})
	}
}

func (s *JobHandlerTestSuite) Test_GetApplicationJob_Status() {
	appName := "any_app"
	scenarios := []jobStatusScenario{
		{scenarioName: "status is set to condition when stop is false", jobName: "job1", condition: radixv1.JobFailed, stop: false, expectedStatus: jobModels.Failed.String()},
		{scenarioName: "status is Stopping when stop is true and condition is not Stopped", jobName: "job2", condition: radixv1.JobRunning, stop: true, expectedStatus: jobModels.Stopping.String()},
		{scenarioName: "status is Stopped when stop is true and condition is Stopped", jobName: "job3", condition: radixv1.JobStopped, stop: true, expectedStatus: jobModels.Stopped.String()},
		{scenarioName: "status is JobStoppedNoChanges when there is no changes", jobName: "job4", condition: radixv1.JobStoppedNoChanges, stop: false, expectedStatus: jobModels.StoppedNoChanges.String()},
		{scenarioName: "status is Waiting when condition is empty", jobName: "job5", expectedStatus: jobModels.Waiting.String()},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.scenarioName, func() {
			ctrl := gomock.NewController(s.T())
			defer ctrl.Finish()

			dh := deployMock.NewMockDeployHandler(ctrl)
			dh.EXPECT().GetDeploymentsForPipelineJob(context.Background(), gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			h := Init(s.accounts, dh)
			rj := radixv1.RadixJob{
				ObjectMeta: metav1.ObjectMeta{Name: scenario.jobName, Namespace: utils.GetAppNamespace(appName)},
				Spec:       radixv1.RadixJobSpec{Stop: scenario.stop},
				Status:     radixv1.RadixJobStatus{Condition: scenario.condition},
			}

			_, err := s.radixClient.RadixV1().RadixJobs(rj.Namespace).Create(context.Background(), &rj, metav1.CreateOptions{})
			s.NoError(err)
			actualJob, err := h.GetApplicationJob(context.Background(), appName, scenario.jobName)
			s.NoError(err)
			s.Equal(scenario.expectedStatus, actualJob.Status)
		})
	}
}

func (s *JobHandlerTestSuite) TestJobHandler_RerunJob() {
	appName := "anyApp"
	namespace := utils.GetAppNamespace(appName)
	tests := []jobRerunScenario{
		{scenarioName: "existing failed job", existingJob: &jobProperties{name: "job1", condition: radixv1.JobFailed}, jobNameToRerun: "job1", expectedError: nil},
		{scenarioName: "existing stopped job", existingJob: &jobProperties{name: "job1", condition: radixv1.JobStopped, stop: true}, jobNameToRerun: "job1", expectedError: nil},
		{scenarioName: "existing running job", existingJob: &jobProperties{name: "job1", condition: radixv1.JobRunning}, jobNameToRerun: "job1", expectedError: jobModels.JobHasInvalidConditionToRerunError(appName, "job1", radixv1.JobRunning)},
		{scenarioName: "existing stopped-no-changes job", existingJob: &jobProperties{name: "job1", condition: radixv1.JobStoppedNoChanges}, jobNameToRerun: "job1", expectedError: jobModels.JobHasInvalidConditionToRerunError(appName, "job1", radixv1.JobStoppedNoChanges)},
		{scenarioName: "existing queued job", existingJob: &jobProperties{name: "job1", condition: radixv1.JobQueued}, jobNameToRerun: "job1", expectedError: jobModels.JobHasInvalidConditionToRerunError(appName, "job1", radixv1.JobQueued)},
		{scenarioName: "existing succeeded job", existingJob: &jobProperties{name: "job1", condition: radixv1.JobSucceeded}, jobNameToRerun: "job1", expectedError: jobModels.JobHasInvalidConditionToRerunError(appName, "job1", radixv1.JobSucceeded)},
		{scenarioName: "existing waiting job", existingJob: &jobProperties{name: "job1", condition: radixv1.JobWaiting}, jobNameToRerun: "job1", expectedError: jobModels.JobHasInvalidConditionToRerunError(appName, "job1", radixv1.JobWaiting)},
		{scenarioName: "not existing job", existingJob: nil, jobNameToRerun: "job1", expectedError: jobModels.PipelineNotFoundError(appName, "job1")},
	}
	for _, tt := range tests {
		s.T().Run(tt.scenarioName, func(t *testing.T) {
			s.setupTest()
			ctrl := gomock.NewController(s.T())
			defer ctrl.Finish()
			dh := deployMock.NewMockDeployHandler(ctrl)
			jh := s.getJobHandler(dh)

			if tt.existingJob != nil {
				_, err := s.accounts.UserAccount.RadixClient.RadixV1().RadixJobs(namespace).Create(context.Background(), &radixv1.RadixJob{
					ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: tt.existingJob.name},
					Spec:       radixv1.RadixJobSpec{Stop: tt.existingJob.stop},
					Status:     radixv1.RadixJobStatus{Condition: tt.existingJob.condition},
				}, metav1.CreateOptions{})
				s.NoError(err)
			}

			err := jh.RerunJob(context.Background(), appName, tt.jobNameToRerun)
			s.Equal(tt.expectedError, err)
		})
	}
}

func (s *JobHandlerTestSuite) TestJobHandler_StopJob() {
	appName := "anyApp"
	namespace := utils.GetAppNamespace(appName)
	tests := []jobRerunScenario{
		{scenarioName: "existing failed job", existingJob: &jobProperties{name: "job1", condition: radixv1.JobFailed}, jobNameToRerun: "job1", expectedError: jobModels.JobHasInvalidConditionToStopError(appName, "job1", radixv1.JobFailed)},
		{scenarioName: "existing stopped job", existingJob: &jobProperties{name: "job1", condition: radixv1.JobStopped, stop: true}, jobNameToRerun: "job1", expectedError: jobModels.JobAlreadyRequestedToStopError(appName, "job1")},
		{scenarioName: "existing running job with stop in spec", existingJob: &jobProperties{name: "job1", condition: radixv1.JobRunning, stop: true}, jobNameToRerun: "job1", expectedError: jobModels.JobAlreadyRequestedToStopError(appName, "job1")},
		{scenarioName: "existing running job", existingJob: &jobProperties{name: "job1", condition: radixv1.JobRunning}, jobNameToRerun: "job1", expectedError: nil},
		{scenarioName: "existing stopped-no-changes job", existingJob: &jobProperties{name: "job1", condition: radixv1.JobStoppedNoChanges}, jobNameToRerun: "job1", expectedError: jobModels.JobHasInvalidConditionToStopError(appName, "job1", radixv1.JobStoppedNoChanges)},
		{scenarioName: "existing queued job", existingJob: &jobProperties{name: "job1", condition: radixv1.JobQueued}, jobNameToRerun: "job1", expectedError: nil},
		{scenarioName: "existing succeeded job", existingJob: &jobProperties{name: "job1", condition: radixv1.JobSucceeded}, jobNameToRerun: "job1", expectedError: nil},
		{scenarioName: "existing waiting job", existingJob: &jobProperties{name: "job1", condition: radixv1.JobWaiting}, jobNameToRerun: "job1", expectedError: nil},
		{scenarioName: "not existing job", existingJob: nil, jobNameToRerun: "job1", expectedError: jobModels.PipelineNotFoundError(appName, "job1")},
	}
	for _, tt := range tests {
		s.T().Run(tt.scenarioName, func(t *testing.T) {
			s.setupTest()
			ctrl := gomock.NewController(s.T())
			defer ctrl.Finish()
			dh := deployMock.NewMockDeployHandler(ctrl)
			jh := s.getJobHandler(dh)
			if tt.existingJob != nil {
				_, err := s.accounts.UserAccount.RadixClient.RadixV1().RadixJobs(namespace).Create(context.Background(), &radixv1.RadixJob{
					ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: tt.existingJob.name},
					Spec:       radixv1.RadixJobSpec{Stop: tt.existingJob.stop},
					Status:     radixv1.RadixJobStatus{Condition: tt.existingJob.condition},
				}, metav1.CreateOptions{})
				s.NoError(err)
			}

			err := jh.StopJob(context.Background(), appName, tt.jobNameToRerun)
			s.Equal(tt.expectedError, err)
		})
	}
}

func (s *JobHandlerTestSuite) getJobHandler(dh *deployMock.MockDeployHandler) JobHandler {
	return JobHandler{
		accounts:       s.accounts,
		userAccount:    s.accounts.UserAccount,
		serviceAccount: s.accounts.ServiceAccount,
		deploy:         dh,
	}
}

package job

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	kubernetes "k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

type RadixJobTestSuiteBase struct {
	suite.Suite
	testUtils   *test.Utils
	kubeClient  kubeclient.Interface
	kubeUtils   *kube.Kube
	radixClient radixclient.Interface
	config      struct {
		clusterName    string
		egressIps      string
		tektonImage    string
		builderImage   string
		buildahImage   string
		buildahSecComp string
		radixZone      string
		clusterType    string
		registry       string
		appRegistry    string
		subscriptionID string
	}
}

func (s *RadixJobTestSuiteBase) SetupSuite() {
	s.config = struct {
		clusterName    string
		egressIps      string
		tektonImage    string
		builderImage   string
		buildahImage   string
		buildahSecComp string
		radixZone      string
		clusterType    string
		registry       string
		appRegistry    string
		subscriptionID string
	}{
		clusterName:    "AnyClusterName",
		egressIps:      "0.0.0.0",
		tektonImage:    "tekton:any",
		builderImage:   "builder:any",
		buildahImage:   "buildah:any",
		buildahSecComp: "anyseccomp",
		radixZone:      "anyzone",
		clusterType:    "anyclustertype",
		registry:       "anyregistry",
		appRegistry:    "anyAppRegistry",
		subscriptionID: "anysubid",
	}
}

func (s *RadixJobTestSuiteBase) SetupTest() {
	s.setupTest()
}

func (s *RadixJobTestSuiteBase) TearDownTest() {
	s.teardownTest()
}

func (s *RadixJobTestSuiteBase) setupTest() {
	// Setup
	kubeClient := kubernetes.NewSimpleClientset()
	radixClient := radix.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	kubeUtil, _ := kube.New(kubeClient, radixClient, secretproviderclient)
	handlerTestUtils := test.NewTestUtils(kubeClient, radixClient, secretproviderclient)
	handlerTestUtils.CreateClusterPrerequisites(s.config.clusterName, s.config.egressIps, s.config.subscriptionID)
	s.testUtils, s.kubeClient, s.kubeUtils, s.radixClient = &handlerTestUtils, kubeClient, kubeUtil, radixClient

	s.T().Setenv(defaults.OperatorClusterTypeEnvironmentVariable, s.config.clusterType)
	s.T().Setenv(defaults.RadixZoneEnvironmentVariable, s.config.radixZone)
	s.T().Setenv(defaults.ContainerRegistryEnvironmentVariable, s.config.registry)
	s.T().Setenv(defaults.AppContainerRegistryEnvironmentVariable, s.config.appRegistry)
	s.T().Setenv(defaults.RadixTektonPipelineImageEnvironmentVariable, s.config.tektonImage)
	s.T().Setenv(defaults.RadixImageBuilderEnvironmentVariable, s.config.builderImage)
	s.T().Setenv(defaults.RadixBuildahImageBuilderEnvironmentVariable, s.config.buildahImage)
	s.T().Setenv(defaults.SeccompProfileFileNameEnvironmentVariable, s.config.buildahSecComp)
}

func (s *RadixJobTestSuiteBase) teardownTest() {
	// Cleanup setup
	os.Unsetenv(defaults.OperatorRollingUpdateMaxUnavailable)
	os.Unsetenv(defaults.OperatorRollingUpdateMaxSurge)
	os.Unsetenv(defaults.OperatorReadinessProbeInitialDelaySeconds)
	os.Unsetenv(defaults.OperatorReadinessProbePeriodSeconds)
	os.Unsetenv(defaults.ActiveClusternameEnvironmentVariable)
	os.Unsetenv(defaults.OperatorTenantIdEnvironmentVariable)
}

func (s *RadixJobTestSuiteBase) applyJobWithSync(jobBuilder utils.JobBuilder, config *Config) (*radixv1.RadixJob, error) {
	rj, err := s.testUtils.ApplyJob(jobBuilder)
	if err != nil {
		return nil, err
	}

	err = s.runSync(rj, config)
	if err != nil {
		return nil, err
	}

	return s.radixClient.RadixV1().RadixJobs(rj.GetNamespace()).Get(context.TODO(), rj.Name, metav1.GetOptions{})
}

func (s *RadixJobTestSuiteBase) runSync(rj *radixv1.RadixJob, config *Config) error {
	job := NewJob(s.kubeClient, s.kubeUtils, s.radixClient, rj, config)
	return job.OnSync()
}

func TestRadixJobTestSuite(t *testing.T) {
	suite.Run(t, new(RadixJobTestSuite))
}

type RadixJobTestSuite struct {
	RadixJobTestSuiteBase
}

func (s *RadixJobTestSuite) TestObjectSynced_StatusMissing_StatusFromAnnotation() {
	config := getConfigWithPipelineJobsHistoryLimit(3)

	appName := "anyApp"
	completedJobStatus := utils.ACompletedJobStatus()

	// Test
	job, err := s.applyJobWithSync(utils.NewJobBuilder().
		WithRadixApplication(utils.ARadixApplication().WithAppName(appName)).
		WithStatusOnAnnotation(completedJobStatus).
		WithAppName(appName).
		WithJobName("anyJob"), config)

	s.Require().NoError(err)

	expectedStatus := completedJobStatus.Build()
	actualStatus := job.Status
	s.assertStatusEqual(expectedStatus, actualStatus)
}

func (s *RadixJobTestSuite) TestObjectSynced_PipelineJobCreated() {
	appName, jobName, branch, deploymentName, commitID, imageTag, pipelineTag := "anyapp", "anyjobname", "anybranch", "anydeploy", "anycommit", "anyimagetag", "anypipelinetag"
	config := getConfigWithPipelineJobsHistoryLimit(3)
	rj, err := s.applyJobWithSync(utils.NewJobBuilder().
		WithJobName(jobName).
		WithAppName(appName).
		WithBranch(branch).
		WithCommitID(commitID).
		WithPushImage(true).
		WithImageTag(imageTag).
		WithDeploymentName(deploymentName).
		WithPipelineType(radixv1.BuildDeploy).
		WithPipelineImageTag(pipelineTag), config)
	s.Require().NoError(err)
	jobs, _ := s.kubeClient.BatchV1().Jobs(utils.GetAppNamespace(appName)).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(jobs.Items, 1)
	job := jobs.Items[0]
	s.Equal(GetOwnerReference(rj), job.OwnerReferences)
	expectedJobLabels := map[string]string{kube.RadixJobNameLabel: jobName, "radix-pipeline": string(radixv1.BuildDeploy), kube.RadixJobTypeLabel: kube.RadixJobTypeJob, kube.RadixAppLabel: appName, kube.RadixCommitLabel: commitID, kube.RadixImageTagLabel: imageTag}
	s.Equal(expectedJobLabels, job.Labels)
	expectedJobAnnotations := map[string]string{kube.RadixBranchAnnotation: branch}
	s.Equal(expectedJobAnnotations, job.Annotations)
	podTemplate := job.Spec.Template
	s.Equal(annotations.ForClusterAutoscalerSafeToEvict(false), podTemplate.Annotations)

	s.Equal(corev1.RestartPolicyNever, podTemplate.Spec.RestartPolicy)
	expectedTolerations := []corev1.Toleration{{Key: kube.NodeTaintJobsKey, Effect: corev1.TaintEffectNoSchedule, Operator: corev1.TolerationOpExists}}
	s.ElementsMatch(expectedTolerations, podTemplate.Spec.Tolerations)
	expectedAffinity := &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{{Key: kube.RadixJobNodeLabel, Operator: corev1.NodeSelectorOpExists}}}}}}}
	s.Equal(expectedAffinity, podTemplate.Spec.Affinity)
	expectedSecurityCtx := &corev1.PodSecurityContext{FSGroup: pointers.Ptr[int64](1000), RunAsNonRoot: pointers.Ptr(true), SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}}
	s.Equal(expectedSecurityCtx, podTemplate.Spec.SecurityContext)

	expectedContainers := []corev1.Container{
		{
			Name:            "radix-pipeline",
			Image:           fmt.Sprintf("%s/radix-pipeline:%s", s.config.registry, pipelineTag),
			ImagePullPolicy: corev1.PullAlways,
			Args: []string{
				fmt.Sprintf("--RADIX_APP=%s", appName),
				fmt.Sprintf("--JOB_NAME=%s", jobName),
				fmt.Sprintf("--PIPELINE_TYPE=%s", radixv1.BuildDeploy),
				"--RADIXOPERATOR_APP_BUILDER_RESOURCES_REQUESTS_MEMORY=1000Mi",
				"--RADIXOPERATOR_APP_BUILDER_RESOURCES_REQUESTS_CPU=100m",
				"--RADIXOPERATOR_APP_BUILDER_RESOURCES_LIMITS_MEMORY=2000Mi",
				fmt.Sprintf("--RADIX_TEKTON_IMAGE=%s", s.config.tektonImage),
				fmt.Sprintf("--RADIX_IMAGE_BUILDER=%s", s.config.builderImage),
				fmt.Sprintf("--RADIX_BUILDAH_IMAGE_BUILDER=%s", s.config.buildahImage),
				fmt.Sprintf("--SECCOMP_PROFILE_FILENAME=%s", s.config.buildahSecComp),
				fmt.Sprintf("--RADIX_CLUSTER_TYPE=%s", s.config.clusterType),
				fmt.Sprintf("--RADIX_ZONE=%s", s.config.radixZone),
				fmt.Sprintf("--RADIX_CLUSTERNAME=%s", s.config.clusterName),
				fmt.Sprintf("--RADIX_CONTAINER_REGISTRY=%s", s.config.registry),
				fmt.Sprintf("--RADIX_APP_CONTAINER_REGISTRY=%s", s.config.appRegistry),
				fmt.Sprintf("--AZURE_SUBSCRIPTION_ID=%s", s.config.subscriptionID),
				fmt.Sprintf("--IMAGE_TAG=%s", imageTag),
				fmt.Sprintf("--BRANCH=%s", branch),
				fmt.Sprintf("--COMMIT_ID=%s", commitID),
				"--PUSH_IMAGE=1",
				"--USE_CACHE=",
				"--RADIX_FILE_NAME=/workspace/radixconfig.yaml",
			},
			SecurityContext: &corev1.SecurityContext{
				Privileged:               pointers.Ptr(false),
				AllowPrivilegeEscalation: pointers.Ptr(false),
				RunAsNonRoot:             pointers.Ptr(true),
				RunAsUser:                pointers.Ptr[int64](1000),
				RunAsGroup:               pointers.Ptr[int64](1000),
				Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
				SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
		},
	}

	expectedPodLabels := map[string]string{kube.RadixJobNameLabel: jobName}
	s.Equal(expectedPodLabels, podTemplate.Labels)
	expectedPodAnnotations := annotations.ForClusterAutoscalerSafeToEvict(false)
	s.Equal(expectedPodAnnotations, podTemplate.Annotations)
	expectedPodSpec := corev1.PodSpec{
		RestartPolicy:      corev1.RestartPolicyNever,
		Tolerations:        expectedTolerations,
		Affinity:           expectedAffinity,
		ServiceAccountName: "radix-pipeline",
		SecurityContext:    expectedSecurityCtx,
		Containers:         expectedContainers,
	}
	s.Equal(expectedPodSpec, podTemplate.Spec)
}

func (s *RadixJobTestSuite) TestObjectSynced_FirstJobRunning_SecondJobQueued() {
	config := getConfigWithPipelineJobsHistoryLimit(3)
	// Setup
	firstJob, err := s.testUtils.ApplyJob(utils.AStartedBuildDeployJob().WithJobName("FirstJob").WithBranch("master"))
	s.Require().NoError(err)

	// Test
	secondJob, err := s.applyJobWithSync(utils.ARadixBuildDeployJob().WithJobName("SecondJob").WithBranch("master"), config)
	s.Require().NoError(err)
	s.Equal(radixv1.JobQueued, secondJob.Status.Condition)

	// Stopping first job should set second job to running
	firstJob.Spec.Stop = true
	s.radixClient.RadixV1().RadixJobs(firstJob.ObjectMeta.Namespace).Update(context.TODO(), firstJob, metav1.UpdateOptions{})
	err = s.runSync(firstJob, config)
	s.Require().NoError(err)

	secondJob, _ = s.radixClient.RadixV1().RadixJobs(secondJob.ObjectMeta.Namespace).Get(context.TODO(), secondJob.Name, metav1.GetOptions{})
	s.Equal(radixv1.JobRunning, secondJob.Status.Condition)
}

func (s *RadixJobTestSuite) TestObjectSynced_FirstJobWaiting_SecondJobQueued() {
	config := getConfigWithPipelineJobsHistoryLimit(3)
	// Setup
	firstJob, err := s.testUtils.ApplyJob(utils.ARadixBuildDeployJob().WithStatus(utils.NewJobStatusBuilder().WithCondition(radixv1.JobWaiting)).WithJobName("FirstJob").WithBranch("master"))
	s.Require().NoError(err)

	// Test
	secondJob, err := s.applyJobWithSync(utils.ARadixBuildDeployJob().WithJobName("SecondJob").WithBranch("master"), config)
	s.Require().NoError(err)
	s.Equal(radixv1.JobQueued, secondJob.Status.Condition)

	// Stopping first job should set second job to running
	firstJob.Spec.Stop = true
	s.radixClient.RadixV1().RadixJobs(firstJob.ObjectMeta.Namespace).Update(context.TODO(), firstJob, metav1.UpdateOptions{})
	err = s.runSync(firstJob, config)
	s.Require().NoError(err)

	secondJob, _ = s.radixClient.RadixV1().RadixJobs(secondJob.ObjectMeta.Namespace).Get(context.TODO(), secondJob.Name, metav1.GetOptions{})
	s.Equal(radixv1.JobRunning, secondJob.Status.Condition)
}

func (s *RadixJobTestSuite) TestObjectSynced_MultipleJobs_MissingRadixApplication() {
	config := getConfigWithPipelineJobsHistoryLimit(3)
	// Setup
	firstJob, err := s.testUtils.ApplyJob(utils.AStartedBuildDeployJob().WithRadixApplication(nil).WithJobName("FirstJob").WithBranch("master"))
	s.Require().NoError(err)

	// Test
	secondJob, err := s.applyJobWithSync(utils.ARadixBuildDeployJob().WithRadixApplication(nil).WithJobName("SecondJob").WithBranch("master"), config)
	s.Require().NoError(err)
	s.Equal(radixv1.JobQueued, secondJob.Status.Condition)

	// Third job differen branch
	thirdJob, err := s.applyJobWithSync(utils.ARadixBuildDeployJob().WithRadixApplication(nil).WithJobName("ThirdJob").WithBranch("qa"), config)
	s.Require().NoError(err)
	s.Equal(radixv1.JobWaiting, thirdJob.Status.Condition)

	// Stopping first job should set second job to running
	firstJob.Spec.Stop = true
	s.radixClient.RadixV1().RadixJobs(firstJob.ObjectMeta.Namespace).Update(context.TODO(), firstJob, metav1.UpdateOptions{})
	err = s.runSync(firstJob, config)
	s.Require().NoError(err)

	secondJob, _ = s.radixClient.RadixV1().RadixJobs(secondJob.ObjectMeta.Namespace).Get(context.TODO(), secondJob.Name, metav1.GetOptions{})
	s.Equal(radixv1.JobRunning, secondJob.Status.Condition)
}

func (s *RadixJobTestSuite) TestObjectSynced_MultipleJobsDifferentBranch_SecondJobRunning() {
	config := getConfigWithPipelineJobsHistoryLimit(3)
	// Setup
	_, err := s.testUtils.ApplyJob(utils.AStartedBuildDeployJob().WithJobName("FirstJob").WithBranch("master"))
	s.Require().NoError(err)

	// Test
	secondJob, err := s.applyJobWithSync(utils.ARadixBuildDeployJob().WithJobName("SecondJob").WithBranch("release"), config)
	s.Require().NoError(err)

	s.Equal(radixv1.JobWaiting, secondJob.Status.Condition)
}

type radixDeploymentJob struct {
	// jobName Radix pipeline job
	jobName string
	// rdName RadixDeployment name
	rdName string
	// env of environments, where RadixDeployments are deployed
	env string
	// branch to build
	branch string
	// type of the pipeline
	pipelineType radixv1.RadixPipelineType
	// jobStatus Status of the job
	jobStatus radixv1.RadixJobCondition
}

func (s *RadixJobTestSuite) TestHistoryLimit_EachEnvHasOwnHistory() {
	appName := "anyApp"
	appNamespace := utils.GetAppNamespace(appName)
	const envDev = "dev"
	const envQa = "qa"
	const envProd = "prod"
	const branchDevQa = "main"
	const branchProd = "release"
	raBuilder := utils.ARadixApplication().WithAppName(appName).
		WithEnvironment(envDev, branchDevQa).
		WithEnvironment(envQa, branchDevQa).
		WithEnvironment(envProd, branchProd)

	type jobScenario struct {
		// name Scenario name
		name string
		// existingRadixDeploymentJobs List of RadixDeployments and its RadixJobs, setup before test
		existingRadixDeploymentJobs []radixDeploymentJob
		// testingRadixDeploymentJob RadixDeployments and its RadixJobs under test
		testingRadixDeploymentJob radixDeploymentJob
		// jobsHistoryLimitPerBranchAndStatus Limit, defined in the env-var   RADIX_PIPELINE_JOBS_HISTORY_LIMIT
		jobsHistoryLimit int
		// expectedJobNames Name of jobs, expected to exist on finishing test
		expectedJobNames []string
	}

	scenarios := []jobScenario{
		{
			name:             "All jobs are successful and running - no deleted job",
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobSucceeded},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j3", rdName: "rd3", env: envDev, jobStatus: radixv1.JobRunning,
			},
			expectedJobNames: []string{"j1", "j2", "j3"},
		},
		{
			name:             "All jobs are successful and queued - no deleted job",
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobSucceeded},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j3", rdName: "rd3", env: envDev, jobStatus: radixv1.JobQueued,
			},
			expectedJobNames: []string{"j1", "j2", "j3"},
		},
		{
			name:             "All jobs are successful and waiting - no deleted job",
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobSucceeded},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j3", rdName: "rd3", env: envDev, jobStatus: radixv1.JobWaiting,
			},
			expectedJobNames: []string{"j1", "j2", "j3"},
		},
		{
			name:             "Stopped job within the limit - not deleted",
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobStopped,
			},
			expectedJobNames: []string{"j1", "j2"},
		},
		{
			name:             "Stopped job out of the limit - old deleted",
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobStopped},
				{jobName: "j3", rdName: "rd3", env: envDev, jobStatus: radixv1.JobStopped},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j4", rdName: "rd4", env: envDev, jobStatus: radixv1.JobStopped,
			},
			expectedJobNames: []string{"j1", "j3", "j4"},
		},
		{
			name:             "Failed job within the limit - not deleted",
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobFailed,
			},
			expectedJobNames: []string{"j1", "j2"},
		},
		{
			name:             "Failed job out of the limit - old deleted",
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobFailed},
				{jobName: "j3", rdName: "rd3", env: envDev, jobStatus: radixv1.JobFailed},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j4", rdName: "rd4", env: envDev, jobStatus: radixv1.JobFailed,
			},
			expectedJobNames: []string{"j1", "j3", "j4"},
		},
		{
			name:             "StoppedNoChanges job within the limit - not deleted",
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobStoppedNoChanges,
			},
			expectedJobNames: []string{"j1", "j2"},
		},
		{
			name:             "StoppedNoChanges job out of the limit - old deleted",
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobStoppedNoChanges},
				{jobName: "j3", rdName: "rd3", env: envDev, jobStatus: radixv1.JobStoppedNoChanges},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j4", rdName: "rd4", env: envDev, jobStatus: radixv1.JobStoppedNoChanges,
			},
			expectedJobNames: []string{"j1", "j3", "j4"},
		},
		{
			name:             "Stopped and failed jobs within the limit - not deleted",
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobStopped},
				{jobName: "j3", rdName: "rd3", env: envDev, jobStatus: radixv1.JobFailed},
				{jobName: "j4", rdName: "rd4", env: envDev, jobStatus: radixv1.JobStoppedNoChanges},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j5", rdName: "rd5", env: envDev, jobStatus: radixv1.JobStopped,
			},
			expectedJobNames: []string{"j1", "j2", "j3", "j4", "j5"},
		},
		{
			name:             "Stopped job out of the limit - old stopped deleted",
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobStopped},
				{jobName: "j3", rdName: "rd3", env: envDev, jobStatus: radixv1.JobFailed},
				{jobName: "j4", rdName: "rd4", env: envDev, jobStatus: radixv1.JobStoppedNoChanges},
				{jobName: "j5", rdName: "rd5", env: envDev, jobStatus: radixv1.JobStopped},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j6", rdName: "rd6", env: envDev, jobStatus: radixv1.JobStopped,
			},
			expectedJobNames: []string{"j1", "j3", "j4", "j5", "j6"},
		},
		{
			name:             "Failed job out of the limit - old falsed deleted",
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobStopped},
				{jobName: "j3", rdName: "rd3", env: envDev, jobStatus: radixv1.JobFailed},
				{jobName: "j4", rdName: "rd4", env: envDev, jobStatus: radixv1.JobStoppedNoChanges},
				{jobName: "j5", rdName: "rd5", env: envDev, jobStatus: radixv1.JobFailed},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j6", rdName: "rd6", env: envDev, jobStatus: radixv1.JobFailed,
			},
			expectedJobNames: []string{"j1", "j2", "j4", "j5", "j6"},
		},
		{
			name:             "StoppedNoChanges job out of the limit - old stopped-no-changes deleted",
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobStopped},
				{jobName: "j3", rdName: "rd3", env: envDev, jobStatus: radixv1.JobFailed},
				{jobName: "j4", rdName: "rd4", env: envDev, jobStatus: radixv1.JobStoppedNoChanges},
				{jobName: "j5", rdName: "rd5", env: envDev, jobStatus: radixv1.JobStoppedNoChanges},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j6", rdName: "rd6", env: envDev, jobStatus: radixv1.JobStoppedNoChanges,
			},
			expectedJobNames: []string{"j1", "j2", "j3", "j5", "j6"},
		},
		{
			name:             "Failed job is within the limit on env - not deleted",
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobFailed},
				{jobName: "j3", rdName: "rd3", env: envQa, jobStatus: radixv1.JobFailed},
				{jobName: "j4", rdName: "rd4", env: envQa, jobStatus: radixv1.JobFailed},
				{jobName: "j5", rdName: "rd5", env: envProd, jobStatus: radixv1.JobFailed},
				{jobName: "j6", rdName: "rd6", env: envProd, jobStatus: radixv1.JobFailed},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j7", rdName: "rd7", env: envDev, jobStatus: radixv1.JobFailed,
			},
			expectedJobNames: []string{"j1", "j2", "j3", "j4", "j5", "j6", "j7"},
		},
		{
			name:             "Failed job out of the limit on env - old failed deleted",
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobFailed},
				{jobName: "j3", rdName: "rd3", env: envQa, jobStatus: radixv1.JobFailed},
				{jobName: "j4", rdName: "rd4", env: envQa, jobStatus: radixv1.JobFailed},
				{jobName: "j5", rdName: "rd5", env: envProd, jobStatus: radixv1.JobFailed},
				{jobName: "j6", rdName: "rd6", env: envDev, jobStatus: radixv1.JobFailed},
				{jobName: "j7", rdName: "rd7", env: envProd, jobStatus: radixv1.JobFailed},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j8", rdName: "rd8", env: envDev, jobStatus: radixv1.JobFailed,
			},
			expectedJobNames: []string{"j1", "j3", "j4", "j5", "j6", "j7", "j8"},
		},
		{
			name:             "Stopped job is within the limit on env - not deleted",
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobStopped},
				{jobName: "j3", rdName: "rd3", env: envQa, jobStatus: radixv1.JobStopped},
				{jobName: "j4", rdName: "rd4", env: envQa, jobStatus: radixv1.JobStopped},
				{jobName: "j5", rdName: "rd5", env: envProd, jobStatus: radixv1.JobStopped},
				{jobName: "j6", rdName: "rd6", env: envProd, jobStatus: radixv1.JobStopped},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j7", rdName: "rd7", env: envDev, jobStatus: radixv1.JobStopped,
			},
			expectedJobNames: []string{"j1", "j2", "j3", "j4", "j5", "j6", "j7"},
		},
		{
			name:             "Stopped job out of the limit on env - old stopped deleted",
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobStopped},
				{jobName: "j3", rdName: "rd3", env: envQa, jobStatus: radixv1.JobStopped},
				{jobName: "j4", rdName: "rd4", env: envQa, jobStatus: radixv1.JobStopped},
				{jobName: "j5", rdName: "rd5", env: envProd, jobStatus: radixv1.JobStopped},
				{jobName: "j6", rdName: "rd6", env: envDev, jobStatus: radixv1.JobStopped},
				{jobName: "j7", rdName: "rd7", env: envProd, jobStatus: radixv1.JobStopped},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j8", rdName: "rd8", env: envDev, jobStatus: radixv1.JobStopped,
			},
			expectedJobNames: []string{"j1", "j3", "j4", "j5", "j6", "j7", "j8"},
		},
		{
			name:             "StoppedNoChanges job is within the limit on env - not deleted",
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobStoppedNoChanges},
				{jobName: "j3", rdName: "rd3", env: envQa, jobStatus: radixv1.JobStoppedNoChanges},
				{jobName: "j4", rdName: "rd4", env: envQa, jobStatus: radixv1.JobStoppedNoChanges},
				{jobName: "j5", rdName: "rd5", env: envProd, jobStatus: radixv1.JobStoppedNoChanges},
				{jobName: "j6", rdName: "rd6", env: envProd, jobStatus: radixv1.JobStoppedNoChanges},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j7", rdName: "rd7", env: envDev, jobStatus: radixv1.JobStoppedNoChanges,
			},
			expectedJobNames: []string{"j1", "j2", "j3", "j4", "j5", "j6", "j7"},
		},
		{
			name:             "StoppedNoChanges job out of the limit on env - old StoppedNoChanges deleted",
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobStoppedNoChanges},
				{jobName: "j3", rdName: "rd3", env: envQa, jobStatus: radixv1.JobStoppedNoChanges},
				{jobName: "j4", rdName: "rd4", env: envQa, jobStatus: radixv1.JobStoppedNoChanges},
				{jobName: "j5", rdName: "rd5", env: envProd, jobStatus: radixv1.JobStoppedNoChanges},
				{jobName: "j6", rdName: "rd6", env: envDev, jobStatus: radixv1.JobStoppedNoChanges},
				{jobName: "j7", rdName: "rd7", env: envProd, jobStatus: radixv1.JobStoppedNoChanges},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j8", rdName: "rd8", env: envDev, jobStatus: radixv1.JobStoppedNoChanges,
			},
			expectedJobNames: []string{"j1", "j3", "j4", "j5", "j6", "j7", "j8"},
		},
	}

	for _, scenario := range scenarios {
		s.T().Run(scenario.name, func(t *testing.T) {
			defer s.teardownTest()
			s.setupTest()
			config := getConfigWithPipelineJobsHistoryLimit(scenario.jobsHistoryLimit)
			testTime := time.Now().Add(time.Hour * -100)
			for _, rdJob := range scenario.existingRadixDeploymentJobs {
				_, err := s.testUtils.ApplyDeployment(utils.ARadixDeployment().
					WithAppName(appName).WithDeploymentName(rdJob.rdName).WithEnvironment(rdJob.env).WithJobName(rdJob.jobName).
					WithActiveFrom(testTime))
				s.NoError(err)
				s.applyJobWithSyncFor(raBuilder, appName, rdJob, config)
				testTime = testTime.Add(time.Hour)
			}

			_, err := s.testUtils.ApplyDeployment(utils.ARadixDeployment().
				WithAppName(appName).WithDeploymentName(scenario.testingRadixDeploymentJob.rdName).
				WithEnvironment(scenario.testingRadixDeploymentJob.env).
				WithActiveFrom(testTime))
			s.NoError(err)
			s.applyJobWithSyncFor(raBuilder, appName, scenario.testingRadixDeploymentJob, config)

			radixJobList, err := s.radixClient.RadixV1().RadixJobs(appNamespace).List(context.TODO(), metav1.ListOptions{})
			s.NoError(err)
			s.assertExistRadixJobsWithNames(radixJobList, scenario.expectedJobNames)
		})
	}
}

type jobConditions map[string]radixv1.RadixJobCondition

func (s *RadixJobTestSuite) Test_WildCardJobs() {
	appName := "anyApp"
	appNamespace := utils.GetAppNamespace(appName)
	const (
		envTest            = "test"
		envQa              = "qa"
		branchQa           = "qa"
		branchTest         = "test"
		branchTestWildCard = "test*"
		branchTest1        = "test1"
		branchTest2        = "test2"
	)

	type jobScenario struct {
		// name Scenario name
		name      string
		raBuilder utils.ApplicationBuilder
		// existingRadixDeploymentJobs List of RadixDeployments and its RadixJobs, setup before test
		existingRadixDeploymentJobs []radixDeploymentJob
		// testingRadixJobBuilder RadixDeployments and its RadixJobs under test
		testingRadixJobBuilder utils.JobBuilder
		// expectedJobConditions State of jobs, mapped by job name, expected to exist on finishing test
		expectedJobConditions jobConditions
	}

	scenarios := []jobScenario{
		{
			name: "One job is running",
			raBuilder: getRadixApplicationBuilder(appName).
				WithEnvironment(envTest, branchTest),
			existingRadixDeploymentJobs: nil,
			testingRadixJobBuilder: utils.NewJobBuilder().WithPipelineType(radixv1.BuildDeploy).
				WithJobName("j-new").WithBranch(branchTest),
			expectedJobConditions: jobConditions{"j-new": radixv1.JobWaiting},
		},
		{
			name: "One job is running, new is queuing on same branch",
			raBuilder: getRadixApplicationBuilder(appName).
				WithEnvironment(envTest, branchTest),
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", env: envTest, branch: branchTest, jobStatus: radixv1.JobRunning},
			},
			testingRadixJobBuilder: utils.NewJobBuilder().WithPipelineType(radixv1.BuildDeploy).
				WithJobName("j-new").WithBranch(branchTest),
			expectedJobConditions: jobConditions{"j1": radixv1.JobRunning, "j-new": radixv1.JobQueued},
		},
		{
			name: "One job is running, new is running on another branch",
			raBuilder: getRadixApplicationBuilder(appName).
				WithEnvironment(envTest, branchTest).
				WithEnvironment(envQa, branchQa),
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", env: envTest, branch: branchTest, jobStatus: radixv1.JobRunning},
			},
			testingRadixJobBuilder: utils.NewJobBuilder().WithPipelineType(radixv1.BuildDeploy).
				WithJobName("j-new").WithBranch(branchQa),
			expectedJobConditions: jobConditions{"j1": radixv1.JobRunning, "j-new": radixv1.JobWaiting},
		},
		{
			name: "One job is running, new is queuing on same branch",
			raBuilder: getRadixApplicationBuilder(appName).
				WithEnvironment(envTest, branchTestWildCard),
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", env: envTest, branch: branchTest1, jobStatus: radixv1.JobRunning},
			},
			testingRadixJobBuilder: utils.NewJobBuilder().WithPipelineType(radixv1.BuildDeploy).
				WithJobName("j-new").WithBranch(branchTest2),
			expectedJobConditions: jobConditions{"j1": radixv1.JobRunning, "j-new": radixv1.JobQueued},
		},
		{
			name: "One job is running, new is queuing on same branch",
			raBuilder: getRadixApplicationBuilder(appName).
				WithEnvironment(envTest, branchTestWildCard),
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", env: envTest, branch: branchTest1, jobStatus: radixv1.JobRunning},
			},
			testingRadixJobBuilder: utils.NewJobBuilder().WithPipelineType(radixv1.BuildDeploy).
				WithJobName("j-new").WithBranch(branchTest2),
			expectedJobConditions: jobConditions{"j1": radixv1.JobRunning, "j-new": radixv1.JobQueued},
		},
		{
			name: "Multiple non-running, new is running on same branch",
			raBuilder: getRadixApplicationBuilder(appName).
				WithEnvironment(envTest, branchTestWildCard),
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", env: envTest, branch: branchTest1, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", env: envTest, branch: branchTest1, jobStatus: radixv1.JobFailed},
				{jobName: "j3", env: envTest, branch: branchTest1, jobStatus: radixv1.JobStopped},
				{jobName: "j4", env: envTest, branch: branchTest1, jobStatus: radixv1.JobStoppedNoChanges},
				{jobName: "j5", env: envTest, branch: branchTest1, jobStatus: radixv1.JobQueued},
				{jobName: "j6", env: envTest, branch: branchTest1, jobStatus: radixv1.JobWaiting},
			},
			testingRadixJobBuilder: utils.NewJobBuilder().WithPipelineType(radixv1.BuildDeploy).
				WithJobName("j-new").WithBranch(branchTest1),
			expectedJobConditions: jobConditions{
				"j1":    radixv1.JobSucceeded,
				"j2":    radixv1.JobFailed,
				"j3":    radixv1.JobStopped,
				"j4":    radixv1.JobStoppedNoChanges,
				"j5":    radixv1.JobQueued,
				"j6":    radixv1.JobWaiting,
				"j-new": radixv1.JobQueued,
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			defer s.teardownTest()
			s.setupTest()
			config := getConfigWithPipelineJobsHistoryLimit(10)
			testTime := time.Now().Add(time.Hour * -100)
			raBuilder := scenario.raBuilder.WithAppName(appName)
			s.testUtils.ApplyApplication(raBuilder)
			for _, rdJob := range scenario.existingRadixDeploymentJobs {
				if rdJob.jobStatus == radixv1.JobSucceeded {
					_, err := s.testUtils.ApplyDeployment(utils.ARadixDeployment().
						WithRadixApplication(nil).
						WithAppName(appName).
						WithDeploymentName(fmt.Sprintf("%s-deployment", rdJob.jobName)).
						WithJobName(rdJob.jobName).
						WithActiveFrom(testTime))
					s.Require().NoError(err)
				}
				rjBuilder := utils.NewJobBuilder().WithAppName(appName).WithPipelineType(rdJob.pipelineType).
					WithJobName(rdJob.jobName).WithBranch(rdJob.branch).WithCreated(testTime.Add(time.Minute * 2)).
					WithStatus(utils.NewJobStatusBuilder().WithCondition(rdJob.jobStatus))
				_, err := s.testUtils.ApplyJob(rjBuilder)
				s.Require().NoError(err)
				testTime = testTime.Add(time.Hour)
			}

			testingRadixJob, err := s.testUtils.ApplyJob(scenario.testingRadixJobBuilder.WithAppName(appName))
			s.Require().NoError(err)
			err = s.runSync(testingRadixJob, config)
			s.NoError(err)

			radixJobList, err := s.radixClient.RadixV1().RadixJobs(appNamespace).List(context.TODO(), metav1.ListOptions{})
			s.Require().NoError(err)
			s.assertExistRadixJobsWithConditions(radixJobList, scenario.expectedJobConditions)
		})
	}
}

func getRadixApplicationBuilder(appName string) utils.ApplicationBuilder {
	return utils.NewRadixApplicationBuilder().
		WithRadixRegistration(utils.NewRegistrationBuilder().WithName(appName))
}
func (s *RadixJobTestSuite) assertExistRadixJobsWithConditions(radixJobList *radixv1.RadixJobList, expectedJobConditions jobConditions) {
	resultJobConditions := make(jobConditions)
	for _, rj := range radixJobList.Items {
		rj := rj
		resultJobConditions[rj.GetName()] = rj.Status.Condition
		if _, ok := expectedJobConditions[rj.GetName()]; !ok {
			s.Fail(fmt.Sprintf("unexpected job exists %s", rj.GetName()))
		}
	}
	for expectedJobName, expectedJobCondition := range expectedJobConditions {
		resultJobCondition, ok := resultJobConditions[expectedJobName]
		s.True(ok, "missing job %s", expectedJobName)
		s.Equal(expectedJobCondition, resultJobCondition, "unexpected job condition")
	}
}

func (s *RadixJobTestSuite) assertExistRadixJobsWithNames(radixJobList *radixv1.RadixJobList, expectedJobNames []string) {
	resultJobNames := make(map[string]bool)
	for _, rj := range radixJobList.Items {
		resultJobNames[rj.GetName()] = true
	}
	for _, jobName := range expectedJobNames {
		_, ok := resultJobNames[jobName]
		s.True(ok, "missing job %s", jobName)
	}
}

func (s *RadixJobTestSuite) applyJobWithSyncFor(raBuilder utils.ApplicationBuilder, appName string, rdJob radixDeploymentJob, config *Config) error {
	_, err := s.applyJobWithSync(utils.ARadixBuildDeployJob().
		WithRadixApplication(raBuilder).
		WithAppName(appName).
		WithJobName(rdJob.jobName).
		WithDeploymentName(rdJob.rdName).
		WithBranch(rdJob.env).
		WithStatus(utils.NewJobStatusBuilder().
			WithCondition(rdJob.jobStatus)), config)
	return err
}

func (s *RadixJobTestSuite) TestTargetEnvironmentIsSetWhenRadixApplicationExist() {
	config := getConfigWithPipelineJobsHistoryLimit(3)

	expectedEnvs := []string{"test"}
	job, err := s.applyJobWithSync(utils.ARadixBuildDeployJob().WithJobName("test").WithBranch("master"), config)
	s.Require().NoError(err)
	// Master maps to Test env
	s.Equal(job.Spec.Build.Branch, "master")
	s.Equal(expectedEnvs, job.Status.TargetEnvs)
}

func (s *RadixJobTestSuite) TestTargetEnvironmentEmptyWhenRadixApplicationMissing() {
	config := getConfigWithPipelineJobsHistoryLimit(3)

	job, err := s.applyJobWithSync(utils.ARadixBuildDeployJob().WithRadixApplication(nil).WithJobName("test").WithBranch("master"), config)
	s.Require().NoError(err)
	// Master maps to Test env
	s.Equal(job.Spec.Build.Branch, "master")
	s.Empty(job.Status.TargetEnvs)
}

func (s *RadixJobTestSuite) assertStatusEqual(expectedStatus, actualStatus radixv1.RadixJobStatus) {
	getTimestamp := func(t time.Time) string {
		return t.Format(time.RFC3339)
	}

	s.Equal(getTimestamp(expectedStatus.Started.Time), getTimestamp(actualStatus.Started.Time))
	s.Equal(getTimestamp(expectedStatus.Ended.Time), getTimestamp(actualStatus.Ended.Time))
	s.Equal(expectedStatus.Condition, actualStatus.Condition)
	s.Equal(expectedStatus.TargetEnvs, actualStatus.TargetEnvs)

	for index, expectedStep := range expectedStatus.Steps {
		s.Equal(expectedStep.Name, actualStatus.Steps[index].Name)
		s.Equal(expectedStep.Condition, actualStatus.Steps[index].Condition)
		s.Equal(getTimestamp(expectedStep.Started.Time), getTimestamp(actualStatus.Steps[index].Started.Time))
		s.Equal(getTimestamp(expectedStep.Ended.Time), getTimestamp(actualStatus.Steps[index].Ended.Time))
		s.Equal(expectedStep.Components, actualStatus.Steps[index].Components)
		s.Equal(expectedStep.PodName, actualStatus.Steps[index].PodName)
	}
}

func (s *RadixJobTestSuite) TestObjectSynced_UseBuildKid_HasResourcesArgs() {
	scenarios := []struct {
		name                                      string
		config                                    *Config
		expectedAppBuilderResourcesRequestsCPU    string
		expectedAppBuilderResourcesRequestsMemory string
		expectedAppBuilderResourcesLimitsMemory   string
		expectedError                             string
	}{
		{
			name: "Configured AppBuilderResources",
			config: &Config{
				PipelineJobsHistoryLimit:          3,
				AppBuilderResourcesRequestsCPU:    pointers.Ptr(resource.MustParse("123m")),
				AppBuilderResourcesRequestsMemory: pointers.Ptr(resource.MustParse("1234Mi")),
				AppBuilderResourcesLimitsMemory:   pointers.Ptr(resource.MustParse("2345Mi")),
			},
			expectedError:                             "",
			expectedAppBuilderResourcesRequestsCPU:    "123m",
			expectedAppBuilderResourcesRequestsMemory: "1234Mi",
			expectedAppBuilderResourcesLimitsMemory:   "2345Mi",
		},
		{
			name: "Missing config for ResourcesRequestsCPU",
			config: &Config{
				AppBuilderResourcesRequestsMemory: pointers.Ptr(resource.MustParse("1234Mi")),
				AppBuilderResourcesLimitsMemory:   pointers.Ptr(resource.MustParse("2345Mi")),
			},
			expectedError: "invalid or missing app builder resources",
		},
		{
			name: "Missing config for ResourcesRequestsMemory",
			config: &Config{
				AppBuilderResourcesRequestsCPU:  pointers.Ptr(resource.MustParse("123m")),
				AppBuilderResourcesLimitsMemory: pointers.Ptr(resource.MustParse("2345Mi")),
			},
			expectedError: "invalid or missing app builder resources",
		},
		{
			name: "Missing config for ResourcesLimitsMemory",
			config: &Config{
				AppBuilderResourcesRequestsCPU:    pointers.Ptr(resource.MustParse("123m")),
				AppBuilderResourcesRequestsMemory: pointers.Ptr(resource.MustParse("1234Mi")),
			},
			expectedError: "invalid or missing app builder resources",
		},
	}
	for _, scenario := range scenarios {
		s.setupTest()
		_, err := s.applyJobWithSync(utils.ARadixBuildDeployJobWithAppBuilder(func(m utils.ApplicationBuilder) {
			m.WithBuildKit(pointers.Ptr(true))
		}).WithJobName("job1").WithBranch("master"), scenario.config)
		switch {
		case len(scenario.expectedError) > 0 && err == nil:
			assert.Fail(s.T(), fmt.Sprintf("Missing expected error '%s'", scenario.expectedError))
			continue
		case len(scenario.expectedError) == 0 && err != nil:
			assert.Fail(s.T(), fmt.Sprintf("Unexpected error %v", err))
			continue
		case len(scenario.expectedError) > 0 && err != nil:
			assert.Equal(s.T(), scenario.expectedError, err.Error(), fmt.Sprintf("Expected error '%s' but got '%s'", scenario.expectedError, err.Error()))
			continue
		}
		s.Require().NoError(err)

		jobList, err := s.testUtils.GetKubeUtil().ListJobs(utils.GetAppNamespace("some-app"))
		s.Require().NoError(err)

		assert.Len(s.T(), jobList, 1)
		job := jobList[0]
		assert.Equal(s.T(), scenario.expectedAppBuilderResourcesRequestsCPU, getJobContainerArgument(job.Spec.Template.Spec.Containers[0], defaults.OperatorAppBuilderResourcesRequestsCPUEnvironmentVariable), "Invalid or missing AppBuilderResourcesRequestsCPU")
		assert.Equal(s.T(), scenario.expectedAppBuilderResourcesRequestsMemory, getJobContainerArgument(job.Spec.Template.Spec.Containers[0], defaults.OperatorAppBuilderResourcesRequestsMemoryEnvironmentVariable), "Invalid or missing AppBuilderResourcesRequestsMemory")
		assert.Equal(s.T(), scenario.expectedAppBuilderResourcesLimitsMemory, getJobContainerArgument(job.Spec.Template.Spec.Containers[0], defaults.OperatorAppBuilderResourcesLimitsMemoryEnvironmentVariable), "Invalid or missing AppBuilderResourcesLimitsMemory")
	}
}

func getJobContainerArgument(container corev1.Container, variableName string) string {
	for _, arg := range container.Args {
		argPrefix := fmt.Sprintf("--%s=", variableName)
		if strings.HasPrefix(arg, argPrefix) {
			return arg[len(argPrefix):]
		}
	}
	return ""
}

func getConfigWithPipelineJobsHistoryLimit(historyLimit int) *Config {
	return &Config{
		PipelineJobsHistoryLimit:          historyLimit,
		AppBuilderResourcesRequestsCPU:    pointers.Ptr(resource.MustParse("100m")),
		AppBuilderResourcesRequestsMemory: pointers.Ptr(resource.MustParse("1000Mi")),
		AppBuilderResourcesLimitsMemory:   pointers.Ptr(resource.MustParse("2000Mi")),
	}
}

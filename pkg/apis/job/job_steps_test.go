package job

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RadixJobStepTestSuite struct {
	RadixJobTestSuiteBase
}

type setStatusOfJobTestScenarioExpected struct {
	steps        []v1.RadixJobStep
	returnsError bool
}

type setStatusOfJobTestScenario struct {
	name       string
	radixjob   *v1.RadixJob
	jobs       []*batchv1.Job
	pods       []*corev1.Pod
	configMaps []*corev1.ConfigMap
	expected   setStatusOfJobTestScenarioExpected
}

type getJobStepWithContainerNameScenario struct {
	name            string
	podName         string
	containerName   string
	containerStatus *corev1.ContainerStatus
	components      []string
	jobStepOutput   *v1.RadixJobStepOutput
	expected        v1.RadixJobStep
}

func TestRadixJobStepTestSuite(t *testing.T) {
	suite.Run(t, new(RadixJobStepTestSuite))
}

func (s *RadixJobStepTestSuite) TestIt() {
	startedAt := metav1.NewTime(time.Date(2020, 1, 1, 1, 0, 0, 0, time.Local))
	finishedAt := metav1.NewTime(time.Date(2020, 1, 1, 1, 1, 0, 0, time.Local))
	testStepOutput := v1.RadixJobStepOutput{}

	scenarios := []getJobStepWithContainerNameScenario{
		{name: "test status waiting when containerStatus is nil", expected: v1.RadixJobStep{Condition: v1.JobWaiting}},
		{
			name:            "test status waiting when containerStatus is waiting",
			containerStatus: &corev1.ContainerStatus{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{}}},
			expected:        v1.RadixJobStep{Condition: v1.JobWaiting},
		},
		{
			name:            "test status running when containerStatus is running",
			containerStatus: &corev1.ContainerStatus{State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: startedAt}}},
			expected:        v1.RadixJobStep{Condition: v1.JobRunning, Started: &startedAt},
		},
		{
			name: "test status succeeded when containerStatus is terminated and exitCode is 0",
			containerStatus: &corev1.ContainerStatus{
				State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{StartedAt: startedAt, FinishedAt: finishedAt, ExitCode: 0}},
			},
			expected: v1.RadixJobStep{Condition: v1.JobSucceeded, Started: &startedAt, Ended: &finishedAt},
		},
		{
			name: "test status failed when containerStatus is terminated and exitCode is not 0",
			containerStatus: &corev1.ContainerStatus{
				State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{StartedAt: startedAt, FinishedAt: finishedAt, ExitCode: 1}},
			},
			expected: v1.RadixJobStep{Condition: v1.JobFailed, Started: &startedAt, Ended: &finishedAt},
		},
		{
			name:          "test podName, containerName and componenets set",
			podName:       "a_pod",
			containerName: "a_container",
			components:    []string{"comp1", "comp2"},
			expected:      v1.RadixJobStep{Condition: v1.JobWaiting, Name: "a_container", PodName: "a_pod", Components: []string{"comp1", "comp2"}},
		},
		{
			name:          "test jobStepOutputFunc called correctly",
			jobStepOutput: &testStepOutput,
			expected:      v1.RadixJobStep{Condition: v1.JobWaiting, Output: &testStepOutput},
		},
	}

	for _, scenario := range scenarios {
		var output *v1.RadixJobStepOutput
		if scenario.jobStepOutput != nil {
			output = scenario.jobStepOutput
		}
		actual := getJobStepWithContainerName(scenario.podName, scenario.containerName, scenario.containerStatus, scenario.components, output)
		assert.Equal(s.T(), scenario.expected, actual, scenario.name)
	}
}

func (s *RadixJobStepTestSuite) Test_StatusSteps_NoPipelineJob() {
	scenario := setStatusOfJobTestScenario{

		name:     "missing pipeline job",
		radixjob: s.getBuildDeployJob("job-1", "app-1").BuildRJ(),
	}

	s.testSetStatusOfJobTestScenario(&scenario)
}

func (s *RadixJobStepTestSuite) Test_StatusSteps_NoPipelinePod() {
	scenario := setStatusOfJobTestScenario{
		name:     "missing pipeline pod",
		radixjob: s.getBuildDeployJob("job-2", "app-2").BuildRJ(),
		jobs: []*batchv1.Job{
			s.getPipelineJob("job-2", "app-2", ""),
		},
	}

	s.testSetStatusOfJobTestScenario(&scenario)
}

func (s *RadixJobStepTestSuite) Test_StatusSteps_NoEmptyCloneSteps() {
	scenario := setStatusOfJobTestScenario{
		name:     "no empty steps when clone config has not been created",
		radixjob: s.getBuildDeployJob("job-3", "app-3").BuildRJ(),
		jobs: []*batchv1.Job{
			s.getPipelineJob("job-3", "app-3", ""),
		},
		pods: []*corev1.Pod{
			s.appendJobPodContainerStatus(
				s.getJobPod("pipeline-pod-3", "job-3", utils.GetAppNamespace("app-3")),
				s.getWaitingContainerStatus("radix-pipeline")),
		},
		expected: setStatusOfJobTestScenarioExpected{
			steps: []v1.RadixJobStep{{Condition: v1.JobWaiting, Name: "radix-pipeline", PodName: "pipeline-pod-3"}},
		},
	}

	s.testSetStatusOfJobTestScenario(&scenario)
}

func (s *RadixJobStepTestSuite) Test_StatusSteps_CorrectCloneStepsSequence() {
	scenario := setStatusOfJobTestScenario{
		name:     "clone and apply config steps added before pipeline step",
		radixjob: s.getBuildDeployJob("job-4", "app-4").BuildRJ(),
		jobs: []*batchv1.Job{
			s.getPipelineJob("job-4", "app-4", "a_tag"),
			s.getCloneConfigJob("clone-job-4", "job-4", "app-4", "a_tag"),
		},
		pods: []*corev1.Pod{
			s.appendJobPodContainerStatus(
				s.getJobPod("pipeline-pod-4", "job-4", utils.GetAppNamespace("app-4")),
				s.getWaitingContainerStatus("radix-pipeline")),
			s.appendJobPodInitContainerStatus(
				s.appendJobPodContainerStatus(
					s.getJobPod("clone-pod-4", "clone-job-4", utils.GetAppNamespace("app-4")),
					s.getWaitingContainerStatus("apply-config")),
				s.getWaitingContainerStatus("clone-config")),
		},
		expected: setStatusOfJobTestScenarioExpected{
			steps: []v1.RadixJobStep{
				{Condition: v1.JobWaiting, Name: "clone-config", PodName: "clone-pod-4"},
				{Condition: v1.JobWaiting, Name: "apply-config", PodName: "clone-pod-4"},
				{Condition: v1.JobWaiting, Name: "radix-pipeline", PodName: "pipeline-pod-4"},
			},
		},
	}

	s.testSetStatusOfJobTestScenario(&scenario)
}

func (s *RadixJobStepTestSuite) Test_StatusSteps_BuildSteps() {
	scenario := setStatusOfJobTestScenario{
		name:     "pipeline with build steps",
		radixjob: s.getBuildDeployJob("job-5", "app-5").BuildRJ(),
		jobs: []*batchv1.Job{
			s.getPipelineJob("job-5", "app-5", "a_tag"),
			s.getCloneConfigJob("clone-job-5", "job-5", "app-5", "a_tag"),
			s.getBuildJob("build-job-5", "job-5", "app-5", "a_tag", map[string]pipeline.ComponentImage{
				"comp":   {ContainerName: "build-app"},
				"multi1": {ContainerName: "build-multi"},
				"multi3": {ContainerName: "build-multi"},
				"multi2": {ContainerName: "build-multi"},
			}),
		},
		pods: []*corev1.Pod{
			s.appendJobPodContainerStatus(
				s.getJobPod("pipeline-pod-5", "job-5", utils.GetAppNamespace("app-5")),
				s.getWaitingContainerStatus("radix-pipeline")),
			s.appendJobPodInitContainerStatus(
				s.appendJobPodContainerStatus(
					s.getJobPod("clone-pod-5", "clone-job-5", utils.GetAppNamespace("app-5")),
					s.getWaitingContainerStatus("apply-config")),
				s.getWaitingContainerStatus("clone-config")),
			s.appendJobPodContainerStatus(
				s.getJobPod("build-pod-5", "build-job-5", utils.GetAppNamespace("app-5")),
				s.getWaitingContainerStatus("build-app"),
				s.getWaitingContainerStatus("build-multi"),
			),
		},
		expected: setStatusOfJobTestScenarioExpected{
			steps: []v1.RadixJobStep{
				{Condition: v1.JobWaiting, Name: "clone-config", PodName: "clone-pod-5"},
				{Condition: v1.JobWaiting, Name: "apply-config", PodName: "clone-pod-5"},
				{Condition: v1.JobWaiting, Name: "radix-pipeline", PodName: "pipeline-pod-5"},
				{Condition: v1.JobWaiting, Name: "build-app", PodName: "build-pod-5", Components: []string{"comp"}},
				{Condition: v1.JobWaiting, Name: "build-multi", PodName: "build-pod-5", Components: []string{"multi1", "multi2", "multi3"}},
			},
		},
	}

	s.testSetStatusOfJobTestScenario(&scenario)
}

func (s *RadixJobStepTestSuite) Test_StatusSteps_InitContainers() {
	scenario := setStatusOfJobTestScenario{
		name:     "steps with init containers",
		radixjob: s.getBuildDeployJob("job-1", "app-1").BuildRJ(),
		jobs: []*batchv1.Job{
			s.getPipelineJob("job-1", "app-1", "a_tag"),
			s.getCloneConfigJob("clone-job-1", "job-1", "app-1", "a_tag"),
			s.getBuildJob("build-job-1", "job-1", "app-1", "a_tag", map[string]pipeline.ComponentImage{}),
		},
		pods: []*corev1.Pod{
			s.appendJobPodContainerStatus(
				s.getJobPod("pipeline-pod-1", "job-1", utils.GetAppNamespace("app-1")),
				s.getWaitingContainerStatus("radix-pipeline")),
			s.appendJobPodInitContainerStatus(
				s.appendJobPodContainerStatus(
					s.getJobPod("clone-pod-1", "clone-job-1", utils.GetAppNamespace("app-1")),
					s.getWaitingContainerStatus("apply-config")),
				s.getWaitingContainerStatus("clone-config")),
			s.appendJobPodInitContainerStatus(
				s.getJobPod("build-pod-1", "build-job-1", utils.GetAppNamespace("app-1")),
				s.getWaitingContainerStatus("build-init1"),
				s.getWaitingContainerStatus("build-init2"),
				s.getWaitingContainerStatus("internal-build-init"),
			),
		},
		expected: setStatusOfJobTestScenarioExpected{
			steps: []v1.RadixJobStep{
				{Condition: v1.JobWaiting, Name: "clone-config", PodName: "clone-pod-1"},
				{Condition: v1.JobWaiting, Name: "apply-config", PodName: "clone-pod-1"},
				{Condition: v1.JobWaiting, Name: "radix-pipeline", PodName: "pipeline-pod-1"},
				{Condition: v1.JobWaiting, Name: "build-init1", PodName: "build-pod-1"},
				{Condition: v1.JobWaiting, Name: "build-init2", PodName: "build-pod-1"},
			},
		},
	}

	s.testSetStatusOfJobTestScenario(&scenario)
}

func (s *RadixJobStepTestSuite) Test_StatusSteps_ScanStepsSteps() {
	startedAt, finishedAt := metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local)), metav1.NewTime(time.Date(2020, 1, 1, 1, 0, 0, 0, time.Local))
	vulnerabilityMap := v1.VulnerabilityMap{"critical": 2, "medium": 3}

	scenarios := []setStatusOfJobTestScenario{
		{
			name:     "scan steps component info correctly set",
			radixjob: s.getBuildDeployJob("job-1", "app-1").BuildRJ(),
			jobs: []*batchv1.Job{
				s.getPipelineJob("job-1", "app-1", "a_tag"),
				s.getCloneConfigJob("clone-job-1", "job-1", "app-1", "a_tag"),
				s.getScanJob("scan-job-1", "job-1", "app-1", "a_tag",
					map[string]pipeline.ComponentImage{
						"single": {ContainerName: "single-container"},
						"multi1": {ContainerName: "multi-container"},
						"multi2": {ContainerName: "multi-container"},
					}, pipeline.ContainerOutputName{}),
			},
			pods: []*corev1.Pod{
				s.appendJobPodContainerStatus(
					s.getJobPod("pipeline-pod-1", "job-1", utils.GetAppNamespace("app-1")),
					s.getWaitingContainerStatus("radix-pipeline")),
				s.appendJobPodInitContainerStatus(
					s.appendJobPodContainerStatus(
						s.getJobPod("clone-pod-1", "clone-job-1", utils.GetAppNamespace("app-1")),
						s.getWaitingContainerStatus("apply-config")),
					s.getWaitingContainerStatus("clone-config")),
				s.appendJobPodContainerStatus(
					s.getJobPod("scan-pod-1", "scan-job-1", utils.GetAppNamespace("app-1")),
					s.getWaitingContainerStatus("single-container"),
					s.getWaitingContainerStatus("multi-container"),
				),
			},
			expected: setStatusOfJobTestScenarioExpected{
				steps: []v1.RadixJobStep{
					{Condition: v1.JobWaiting, Name: "clone-config", PodName: "clone-pod-1"},
					{Condition: v1.JobWaiting, Name: "apply-config", PodName: "clone-pod-1"},
					{Condition: v1.JobWaiting, Name: "radix-pipeline", PodName: "pipeline-pod-1"},
					{Condition: v1.JobWaiting, Name: "single-container", PodName: "scan-pod-1", Components: []string{"single"}},
					{Condition: v1.JobWaiting, Name: "multi-container", PodName: "scan-pod-1", Components: []string{"multi1", "multi2"}},
				},
			},
		},
		{
			name:     "only read scan results for terminated scan steps",
			radixjob: s.getBuildDeployJob("job-2", "app-2").BuildRJ(),
			jobs: []*batchv1.Job{
				s.getPipelineJob("job-2", "app-2", "a_tag"),
				s.getCloneConfigJob("clone-job-2", "job-2", "app-2", "a_tag"),
				s.getScanJob("scan-job-2", "job-2", "app-2", "a_tag",
					map[string]pipeline.ComponentImage{},
					pipeline.ContainerOutputName{
						"scan-waiting":           "cm-2-scan",
						"scan-running":           "cm-2-scan",
						"scan-terminated":        "cm-2-scan",
						"scan-terminated-failed": "cm-2-scan",
					}),
			},
			configMaps: []*corev1.ConfigMap{
				s.getScanConfigMapOutput(
					"cm-2-scan",
					utils.GetAppNamespace("app-2"),
					defaults.RadixPipelineScanStepVulnerabilityCountKey,
					vulnerabilityMap,
				),
			},
			pods: []*corev1.Pod{
				s.appendJobPodContainerStatus(
					s.getJobPod("pipeline-pod-2", "job-2", utils.GetAppNamespace("app-2")),
					s.getWaitingContainerStatus("radix-pipeline")),
				s.appendJobPodInitContainerStatus(
					s.appendJobPodContainerStatus(
						s.getJobPod("clone-pod-2", "clone-job-2", utils.GetAppNamespace("app-2")),
						s.getWaitingContainerStatus("apply-config")),
					s.getWaitingContainerStatus("clone-config")),
				s.appendJobPodContainerStatus(
					s.getJobPod("scan-pod-2", "scan-job-2", utils.GetAppNamespace("app-2")),
					s.getWaitingContainerStatus("scan-waiting"),
					s.getRunningContainerStatus("scan-running", startedAt),
					s.getTerminatedContainerStatus("scan-terminated", startedAt, finishedAt, 0),
					s.getTerminatedContainerStatus("scan-terminated-failed", startedAt, finishedAt, 1),
				),
			},
			expected: setStatusOfJobTestScenarioExpected{
				steps: []v1.RadixJobStep{
					{Condition: v1.JobWaiting, Name: "clone-config", PodName: "clone-pod-2"},
					{Condition: v1.JobWaiting, Name: "apply-config", PodName: "clone-pod-2"},
					{Condition: v1.JobWaiting, Name: "radix-pipeline", PodName: "pipeline-pod-2"},
					{Condition: v1.JobWaiting, Name: "scan-waiting", PodName: "scan-pod-2", Components: []string{}},
					{Condition: v1.JobRunning, Name: "scan-running", PodName: "scan-pod-2", Started: &startedAt, Components: []string{}},
					{
						Condition:  v1.JobSucceeded,
						Name:       "scan-terminated",
						PodName:    "scan-pod-2",
						Started:    &startedAt,
						Ended:      &finishedAt,
						Components: []string{},
						Output: &v1.RadixJobStepOutput{
							Scan: &v1.RadixJobStepScanOutput{
								Status:                     v1.ScanSuccess,
								Vulnerabilities:            vulnerabilityMap,
								VulnerabilityListConfigMap: "cm-2-scan",
								VulnerabilityListKey:       defaults.RadixPipelineScanStepVulnerabilityListKey,
							},
						},
					},
					{
						Condition:  v1.JobFailed,
						Name:       "scan-terminated-failed",
						PodName:    "scan-pod-2",
						Started:    &startedAt,
						Ended:      &finishedAt,
						Components: []string{},
						Output: &v1.RadixJobStepOutput{
							Scan: &v1.RadixJobStepScanOutput{
								Status:                     v1.ScanSuccess,
								Vulnerabilities:            vulnerabilityMap,
								VulnerabilityListConfigMap: "cm-2-scan",
								VulnerabilityListKey:       defaults.RadixPipelineScanStepVulnerabilityListKey,
							},
						},
					},
				},
			},
		},
		{
			name:     "handle missing output configmap and incorrect/missing key in cm",
			radixjob: s.getBuildDeployJob("job-3", "app-3").BuildRJ(),
			jobs: []*batchv1.Job{
				s.getPipelineJob("job-3", "app-3", "a_tag"),
				s.getCloneConfigJob("clone-job-3", "job-3", "app-3", "a_tag"),
				s.getScanJob("scan-job-3", "job-3", "app-3", "a_tag",
					map[string]pipeline.ComponentImage{},
					pipeline.ContainerOutputName{
						"scan-missing-cm":      "cm-3-missing",
						"scan-missing-cm-key":  "cm-3-missing-key",
						"scan-cm-invalid-data": "cm-3-invalid-data",
					}),
			},
			configMaps: []*corev1.ConfigMap{
				s.getScanConfigMapOutput(
					"cm-3-missing-key",
					utils.GetAppNamespace("app-3"),
					"incorrect-key",
					vulnerabilityMap,
				),
				s.getScanConfigMapOutput(
					"cm-3-invalid-data",
					utils.GetAppNamespace("app-3"),
					defaults.RadixPipelineScanStepVulnerabilityCountKey,
					"invalid-data",
				),
			},
			pods: []*corev1.Pod{
				s.appendJobPodContainerStatus(
					s.getJobPod("pipeline-pod-3", "job-3", utils.GetAppNamespace("app-3")),
					s.getWaitingContainerStatus("radix-pipeline")),
				s.appendJobPodInitContainerStatus(
					s.appendJobPodContainerStatus(
						s.getJobPod("clone-pod-3", "clone-job-3", utils.GetAppNamespace("app-3")),
						s.getWaitingContainerStatus("apply-config")),
					s.getWaitingContainerStatus("clone-config")),
				s.appendJobPodContainerStatus(
					s.getJobPod("scan-pod-3", "scan-job-3", utils.GetAppNamespace("app-3")),
					s.getTerminatedContainerStatus("scan-missing-annotation", startedAt, finishedAt, 0),
					s.getTerminatedContainerStatus("scan-missing-cm", startedAt, finishedAt, 0),
					s.getTerminatedContainerStatus("scan-missing-cm-key", startedAt, finishedAt, 0),
					s.getTerminatedContainerStatus("scan-cm-invalid-data", startedAt, finishedAt, 0),
				),
			},
			expected: setStatusOfJobTestScenarioExpected{
				steps: []v1.RadixJobStep{
					{Condition: v1.JobWaiting, Name: "clone-config", PodName: "clone-pod-3"},
					{Condition: v1.JobWaiting, Name: "apply-config", PodName: "clone-pod-3"},
					{Condition: v1.JobWaiting, Name: "radix-pipeline", PodName: "pipeline-pod-3"},
					{
						Condition:  v1.JobSucceeded,
						Name:       "scan-missing-annotation",
						PodName:    "scan-pod-3",
						Components: []string{},
						Started:    &startedAt,
						Ended:      &finishedAt,
						Output: &v1.RadixJobStepOutput{
							Scan: &v1.RadixJobStepScanOutput{
								Status: v1.ScanMissing,
								Reason: ScanStatusReasonNotRequested,
							},
						},
					},
					{
						Condition:  v1.JobSucceeded,
						Name:       "scan-missing-cm",
						PodName:    "scan-pod-3",
						Components: []string{},
						Started:    &startedAt,
						Ended:      &finishedAt,
						Output: &v1.RadixJobStepOutput{
							Scan: &v1.RadixJobStepScanOutput{
								Status: v1.ScanMissing,
								Reason: ScanStatusReasonOutputDeleted,
							},
						},
					},
					{
						Condition:  v1.JobSucceeded,
						Name:       "scan-missing-cm-key",
						PodName:    "scan-pod-3",
						Components: []string{},
						Started:    &startedAt,
						Ended:      &finishedAt,
						Output: &v1.RadixJobStepOutput{
							Scan: &v1.RadixJobStepScanOutput{
								Status: v1.ScanMissing,
								Reason: ScanStatusReasonResultMissing,
							},
						},
					},
					{
						Condition:  v1.JobSucceeded,
						Name:       "scan-cm-invalid-data",
						PodName:    "scan-pod-3",
						Components: []string{},
						Started:    &startedAt,
						Ended:      &finishedAt,
						Output: &v1.RadixJobStepOutput{
							Scan: &v1.RadixJobStepScanOutput{
								Status: v1.ScanMissing,
								Reason: ScanStatusReasonResultParseError,
							},
						},
					},
				},
			},
		},
	}

	for _, scenario := range scenarios {
		s.testSetStatusOfJobTestScenario(&scenario)
	}
}

func (s *RadixJobStepTestSuite) testSetStatusOfJobTestScenario(scenario *setStatusOfJobTestScenario) {
	if err := s.initScenario(scenario); err != nil {
		assert.FailNowf(s.T(), err.Error(), "scenario %s", scenario.name)
	}

	job := NewJob(s.kubeClient, s.kubeUtils, s.radixClient, scenario.radixjob)
	if err := job.setStatusOfJob(); err != nil {
		assert.FailNowf(s.T(), err.Error(), "scenario %s", scenario.name)
	}

	actualRj, err := s.radixClient.RadixV1().RadixJobs(scenario.radixjob.Namespace).Get(context.Background(), scenario.radixjob.Name, metav1.GetOptions{})
	if err != nil {
		assert.FailNowf(s.T(), err.Error(), "scenario %s", scenario.name)
	}

	assert.Equal(s.T(), scenario.expected.returnsError, err != nil, scenario.name)
	assert.ElementsMatch(s.T(), scenario.expected.steps, actualRj.Status.Steps, scenario.name)
}

func (s *RadixJobStepTestSuite) initScenario(scenario *setStatusOfJobTestScenario) error {
	if scenario.radixjob != nil {
		if _, err := s.radixClient.RadixV1().RadixJobs(scenario.radixjob.Namespace).Create(context.Background(), scenario.radixjob, metav1.CreateOptions{}); err != nil {
			return err
		}
	}

	for _, job := range scenario.jobs {
		if _, err := s.kubeClient.BatchV1().Jobs(job.Namespace).Create(context.Background(), job, metav1.CreateOptions{}); err != nil {
			return err
		}
	}

	for _, pod := range scenario.pods {
		if _, err := s.kubeClient.CoreV1().Pods(pod.Namespace).Create(context.Background(), pod, metav1.CreateOptions{}); err != nil {
			return err
		}
	}

	for _, cm := range scenario.configMaps {
		if _, err := s.kubeClient.CoreV1().ConfigMaps(cm.Namespace).Create(context.Background(), cm, metav1.CreateOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (s *RadixJobStepTestSuite) getPipelineJob(name, appName, imageTag string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: utils.GetAppNamespace(appName),
			Labels: map[string]string{
				kube.RadixJobNameLabel:  name,
				kube.RadixJobTypeLabel:  kube.RadixJobTypeJob,
				kube.RadixImageTagLabel: imageTag,
				kube.RadixAppLabel:      appName,
			},
		},
	}
}

func (s *RadixJobStepTestSuite) getCloneConfigJob(name, radixJobName, appName, imageTag string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: utils.GetAppNamespace(appName),
			Labels: map[string]string{
				kube.RadixJobNameLabel:  radixJobName,
				kube.RadixJobTypeLabel:  kube.RadixJobTypeCloneConfig,
				kube.RadixImageTagLabel: imageTag,
				kube.RadixAppLabel:      appName,
			},
		},
	}
}

func (s *RadixJobStepTestSuite) getBuildJob(name, radixJobName, appName, imageTag string, componentImages map[string]pipeline.ComponentImage) *batchv1.Job {
	componentImageAnnontation, _ := json.Marshal(&componentImages)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: utils.GetAppNamespace(appName),
			Labels: map[string]string{
				kube.RadixJobNameLabel:  radixJobName,
				kube.RadixBuildLabel:    fmt.Sprintf("%s-%s", appName, imageTag),
				kube.RadixAppLabel:      appName,
				kube.RadixImageTagLabel: imageTag,
				kube.RadixJobTypeLabel:  kube.RadixJobTypeBuild,
			},
			Annotations: map[string]string{
				kube.RadixComponentImagesAnnotation: string(componentImageAnnontation),
			},
		},
	}
}

func (s *RadixJobStepTestSuite) getScanJob(name, radixJobName, appName, imageTag string, componentImages map[string]pipeline.ComponentImage, containerOutput pipeline.ContainerOutputName) *batchv1.Job {
	componentImageAnnontation, _ := json.Marshal(&componentImages)
	containerOutputAnnontation, _ := json.Marshal(&containerOutput)
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: utils.GetAppNamespace(appName),
			Labels: map[string]string{
				kube.RadixJobNameLabel:  radixJobName,
				kube.RadixAppLabel:      appName,
				kube.RadixImageTagLabel: imageTag,
				kube.RadixJobTypeLabel:  kube.RadixJobTypeScan,
			},
			Annotations: map[string]string{
				kube.RadixComponentImagesAnnotation: string(componentImageAnnontation),
				kube.RadixContainerOutputAnnotation: string(containerOutputAnnontation),
			},
		},
	}
}

func (s *RadixJobStepTestSuite) getScanConfigMapOutput(name, namespace, vulnerabilityKey string, vulnerabilities interface{}) *corev1.ConfigMap {
	vulnerabilityBytes, _ := json.Marshal(&vulnerabilities)

	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{
			vulnerabilityKey: string(vulnerabilityBytes),
		},
	}

	return &cm
}

func (s *RadixJobStepTestSuite) getJobPod(podName, jobName, namespace string) *corev1.Pod {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Labels:    map[string]string{"job-name": jobName},
		},
	}

	return &pod
}

func (s *RadixJobStepTestSuite) appendJobPodContainerStatus(pod *corev1.Pod, containerStatus ...corev1.ContainerStatus) *corev1.Pod {
	pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, containerStatus...)
	return pod
}

func (s *RadixJobStepTestSuite) appendJobPodInitContainerStatus(pod *corev1.Pod, containerStatus ...corev1.ContainerStatus) *corev1.Pod {
	pod.Status.InitContainerStatuses = append(pod.Status.InitContainerStatuses, containerStatus...)
	return pod
}

func (s *RadixJobStepTestSuite) getBuildDeployJob(jobName, appName string) utils.JobBuilder {
	jb := utils.NewJobBuilder().
		WithJobName(jobName).
		WithAppName(appName).
		WithPipeline(v1.BuildDeploy).
		WithStatus(
			utils.NewJobStatusBuilder().
				WithCondition(v1.JobRunning),
		)

	return jb
}

func (s *RadixJobStepTestSuite) getWaitingContainerStatus(containerName string) corev1.ContainerStatus {
	return s.getContainerStatus(containerName, corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{}})
}

func (s *RadixJobStepTestSuite) getRunningContainerStatus(containerName string, startedAt metav1.Time) corev1.ContainerStatus {
	return s.getContainerStatus(containerName, corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: startedAt}})
}

func (s *RadixJobStepTestSuite) getTerminatedContainerStatus(containerName string, startedAt, finishedAt metav1.Time, exitCode int32) corev1.ContainerStatus {
	return s.getContainerStatus(containerName, corev1.ContainerState{
		Terminated: &corev1.ContainerStateTerminated{StartedAt: startedAt, FinishedAt: finishedAt, ExitCode: exitCode},
	})
}

func (s *RadixJobStepTestSuite) getContainerStatus(containerName string, state corev1.ContainerState) corev1.ContainerStatus {
	status := corev1.ContainerStatus{
		Name:  containerName,
		State: state,
	}

	return status
}

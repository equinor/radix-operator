package job

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	radixJob   *v1.RadixJob
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
	expected        v1.RadixJobStep
}

func TestRadixJobStepTestSuite(t *testing.T) {
	suite.Run(t, new(RadixJobStepTestSuite))
}

func (s *RadixJobStepTestSuite) TestIt() {
	startedAt := metav1.NewTime(time.Date(2020, 1, 1, 1, 0, 0, 0, time.Local))
	finishedAt := metav1.NewTime(time.Date(2020, 1, 1, 1, 1, 0, 0, time.Local))

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
			name:          "test podName, containerName and components set",
			podName:       "a_pod",
			containerName: "a_container",
			components:    []string{"comp1", "comp2"},
			expected:      v1.RadixJobStep{Condition: v1.JobWaiting, Name: "a_container", PodName: "a_pod", Components: []string{"comp1", "comp2"}},
		},
	}

	for _, scenario := range scenarios {
		actual := getJobStepWithContainerName(scenario.podName, scenario.containerName, scenario.containerStatus, scenario.components)
		assert.Equal(s.T(), scenario.expected, actual, scenario.name)
	}
}

func (s *RadixJobStepTestSuite) Test_StatusSteps_NoPipelineJob() {
	scenario := setStatusOfJobTestScenario{

		name:     "missing pipeline job",
		radixJob: s.getBuildDeployJob("job-1", "app-1").BuildRJ(),
	}

	s.testSetStatusOfJobTestScenario(&scenario)
}

func (s *RadixJobStepTestSuite) Test_StatusSteps_NoPipelinePod() {
	scenario := setStatusOfJobTestScenario{
		name:     "missing pipeline pod",
		radixJob: s.getBuildDeployJob("job-2", "app-2").BuildRJ(),
		jobs: []*batchv1.Job{
			s.getPipelineJob("job-2", "app-2", ""),
		},
	}

	s.testSetStatusOfJobTestScenario(&scenario)
}

func (s *RadixJobStepTestSuite) Test_StatusSteps_NoEmptyCloneSteps() {
	scenario := setStatusOfJobTestScenario{
		name:     "no empty steps when clone config has not been created",
		radixJob: s.getBuildDeployJob("job-3", "app-3").BuildRJ(),
		jobs: []*batchv1.Job{
			s.getPipelineJob("job-3", "app-3", ""),
		},
		pods: []*corev1.Pod{
			s.appendJobPodContainerWithStatus(
				s.getJobPod("pipeline-pod-3", "job-3", "job-3", utils.GetAppNamespace("app-3")),
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
		radixJob: s.getBuildDeployJob("job-4", "app-4").BuildRJ(),
		jobs: []*batchv1.Job{
			s.getPipelineJob("job-4", "app-4", "a_tag"),
			s.getPreparePipelineJob("prepare-pipeline-4", "job-4", "app-4", "a_tag"),
			s.getRunPipelineJob("run-pipeline-job-4", "job-4", "app-4", "a_tag"),
		},
		pods: []*corev1.Pod{
			s.appendJobPodContainerWithStatus(
				s.getJobPod("pipeline-pod-4", "job-4", "job-4", utils.GetAppNamespace("app-4")),
				s.getWaitingContainerStatus("radix-pipeline")),
			s.appendJobPodInitContainerStatus(
				s.appendJobPodContainerWithStatus(
					s.getJobPod("prepare-pipeline-pod-4", "job-4", "prepare-pipeline-4", utils.GetAppNamespace("app-4")),
					s.getWaitingContainerStatus("prepare-pipeline")),
				s.getWaitingContainerStatus("clone-config")),
			s.appendJobPodContainerWithStatus(
				s.getJobPod("run-pipeline-pod-4", "job-4", "run-pipeline-job-4", utils.GetAppNamespace("app-4")),
				s.getWaitingContainerStatus("run-pipeline")),
		},
		expected: setStatusOfJobTestScenarioExpected{
			steps: []v1.RadixJobStep{
				{Condition: v1.JobWaiting, Name: "clone-config", PodName: "prepare-pipeline-pod-4"},
				{Condition: v1.JobWaiting, Name: "prepare-pipeline", PodName: "prepare-pipeline-pod-4"},
				{Condition: v1.JobWaiting, Name: "radix-pipeline", PodName: "pipeline-pod-4"},
				{Condition: v1.JobWaiting, Name: "run-pipeline", PodName: "run-pipeline-pod-4", Components: []string{}},
			},
		},
	}

	s.testSetStatusOfJobTestScenario(&scenario)
}

func (s *RadixJobStepTestSuite) Test_StatusSteps_BuildSteps() {
	scenario := setStatusOfJobTestScenario{
		name:     "pipeline with build steps",
		radixJob: s.getBuildDeployJob("job-5", "app-5").BuildRJ(),
		jobs: []*batchv1.Job{
			s.getPipelineJob("job-5", "app-5", "a_tag"),
			s.getPreparePipelineJob("prepare-pipeline-5", "job-5", "app-5", "a_tag"),
			s.getBuildJob("build-job-5", "job-5", "app-5", "a_tag", []pipeline.BuildComponentImage{
				{ComponentName: "comp", ContainerName: "build-app"},
				{ComponentName: "multi1", ContainerName: "build-multi"},
				{ComponentName: "multi3", ContainerName: "build-multi"},
				{ComponentName: "multi2", ContainerName: "build-multi"},
			}),
			s.getRunPipelineJob("run-pipeline-5", "job-5", "app-5", "a_tag"),
		},
		pods: []*corev1.Pod{
			s.appendJobPodContainerWithStatus(
				s.getJobPod("pipeline-pod-5", "job-5", "job-5", utils.GetAppNamespace("app-5")),
				s.getWaitingContainerStatus("radix-pipeline")),
			s.appendJobPodInitContainerStatus(
				s.appendJobPodContainerWithStatus(
					s.getJobPod("prepare-pipeline-pod-5", "job-5", "prepare-pipeline-5", utils.GetAppNamespace("app-5")),
					s.getWaitingContainerStatus("prepare-pipeline")),
				s.getWaitingContainerStatus("clone-config")),
			s.appendJobPodContainerWithStatus(
				s.getJobPod("build-pod-5", "job-5", "build-job-5", utils.GetAppNamespace("app-5")),
				s.getWaitingContainerStatus("build-app"),
				s.getWaitingContainerStatus("build-multi"),
			),
			s.appendJobPodContainerWithStatus(
				s.getJobPod("run-pipeline-pod-5", "job-5", "run-pipeline-5", utils.GetAppNamespace("app-5")),
				s.getWaitingContainerStatus("run-pipeline")),
		},
		expected: setStatusOfJobTestScenarioExpected{
			steps: []v1.RadixJobStep{
				{Condition: v1.JobWaiting, Name: "clone-config", PodName: "prepare-pipeline-pod-5"},
				{Condition: v1.JobWaiting, Name: "prepare-pipeline", PodName: "prepare-pipeline-pod-5"},
				{Condition: v1.JobWaiting, Name: "radix-pipeline", PodName: "pipeline-pod-5"},
				{Condition: v1.JobWaiting, Name: "build-app", PodName: "build-pod-5", Components: []string{"comp"}},
				{Condition: v1.JobWaiting, Name: "build-multi", PodName: "build-pod-5", Components: []string{"multi1", "multi2", "multi3"}},
				{Condition: v1.JobWaiting, Name: "run-pipeline", PodName: "run-pipeline-pod-5", Components: []string{}},
			},
		},
	}

	s.testSetStatusOfJobTestScenario(&scenario)
}

func (s *RadixJobStepTestSuite) Test_StatusSteps_InitContainers() {
	scenario := setStatusOfJobTestScenario{
		name:     "steps with init containers",
		radixJob: s.getBuildDeployJob("job-1", "app-1").BuildRJ(),
		jobs: []*batchv1.Job{
			s.getPipelineJob("job-1", "app-1", "a_tag"),
			s.getPreparePipelineJob("prepare-pipeline-1", "job-1", "app-1", "a_tag"),
			s.getBuildJob("build-job-1", "job-1", "app-1", "a_tag", []pipeline.BuildComponentImage{}),
			s.getRunPipelineJob("run-pipeline-1", "job-1", "app-1", "a_tag"),
		},
		pods: []*corev1.Pod{
			s.appendJobPodContainerWithStatus(
				s.getJobPod("pipeline-pod-1", "job-1", "job-1", utils.GetAppNamespace("app-1")),
				s.getWaitingContainerStatus("radix-pipeline")),
			s.appendJobPodInitContainerStatus(
				s.appendJobPodContainerWithStatus(
					s.getJobPod("prepare-pipeline-pod-1", "job-1", "prepare-pipeline-1", utils.GetAppNamespace("app-1")),
					s.getWaitingContainerStatus("prepare-pipeline")),
				s.getWaitingContainerStatus("clone-config")),
			s.appendJobPodInitContainerStatus(
				s.getJobPod("build-pod-1", "job-1", "build-job-1", utils.GetAppNamespace("app-1")),
				s.getWaitingContainerStatus("build-init1"),
				s.getWaitingContainerStatus("build-init2"),
				s.getWaitingContainerStatus("internal-build-init"),
			),
			s.appendJobPodContainerWithStatus(
				s.getJobPod("run-pipeline-pod-1", "job-1", "run-pipeline-1", utils.GetAppNamespace("app-1")),
				s.getWaitingContainerStatus("run-pipeline")),
		},
		expected: setStatusOfJobTestScenarioExpected{
			steps: []v1.RadixJobStep{
				{Condition: v1.JobWaiting, Name: "clone-config", PodName: "prepare-pipeline-pod-1"},
				{Condition: v1.JobWaiting, Name: "prepare-pipeline", PodName: "prepare-pipeline-pod-1"},
				{Condition: v1.JobWaiting, Name: "radix-pipeline", PodName: "pipeline-pod-1"},
				{Condition: v1.JobWaiting, Name: "build-init1", PodName: "build-pod-1"},
				{Condition: v1.JobWaiting, Name: "build-init2", PodName: "build-pod-1"},
				{Condition: v1.JobWaiting, Name: "run-pipeline", PodName: "run-pipeline-pod-1", Components: []string{}},
			},
		},
	}

	s.testSetStatusOfJobTestScenario(&scenario)
}

func (s *RadixJobStepTestSuite) testSetStatusOfJobTestScenario(scenario *setStatusOfJobTestScenario) {
	err := s.initScenario(scenario)
	require.NoError(s.T(), err, "scenario %s", scenario.name)

	job := NewJob(s.kubeClient, s.kubeUtils, s.radixClient, scenario.radixJob, nil)
	err = job.setStatusOfJob(context.Background())
	require.NoError(s.T(), err, "scenario %s", scenario.name)

	actualRj, err := s.radixClient.RadixV1().RadixJobs(scenario.radixJob.Namespace).Get(context.Background(), scenario.radixJob.Name, metav1.GetOptions{})
	require.NoError(s.T(), err, "scenario %s", scenario.name)

	assert.Equal(s.T(), scenario.expected.returnsError, err != nil, scenario.name)
	assert.ElementsMatch(s.T(), scenario.expected.steps, actualRj.Status.Steps, scenario.name)
}

func (s *RadixJobStepTestSuite) initScenario(scenario *setStatusOfJobTestScenario) error {
	if scenario.radixJob != nil {
		if _, err := s.radixClient.RadixV1().RadixJobs(scenario.radixJob.Namespace).Create(context.Background(), scenario.radixJob, metav1.CreateOptions{}); err != nil {
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

func (s *RadixJobStepTestSuite) getPreparePipelineJob(name, radixJobName, appName, imageTag string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: utils.GetAppNamespace(appName),
			Labels: map[string]string{
				kube.RadixJobNameLabel:  radixJobName,
				kube.RadixJobTypeLabel:  kube.RadixJobTypePreparePipelines,
				kube.RadixImageTagLabel: imageTag,
				kube.RadixAppLabel:      appName,
			},
		},
	}
}

func (s *RadixJobStepTestSuite) getRunPipelineJob(name, radixJobName, appName, imageTag string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: utils.GetAppNamespace(appName),
			Labels: map[string]string{
				kube.RadixJobNameLabel:  radixJobName,
				kube.RadixJobTypeLabel:  kube.RadixJobTypeRunPipelines,
				kube.RadixImageTagLabel: imageTag,
				kube.RadixAppLabel:      appName,
			},
		},
	}
}

func (s *RadixJobStepTestSuite) getBuildJob(name, radixJobName, appName, imageTag string, componentImages []pipeline.BuildComponentImage) *batchv1.Job {
	componentImageAnnotation, _ := json.Marshal(&componentImages)

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
				kube.RadixBuildComponentsAnnotation: string(componentImageAnnotation),
			},
		},
	}
}

func (s *RadixJobStepTestSuite) getJobPod(podName, radixJobName, jobName, namespace string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Labels:    map[string]string{kube.RadixJobNameLabel: radixJobName, jobNameLabel: jobName},
		},
	}
}

func (s *RadixJobStepTestSuite) appendJobPodContainerWithStatus(pod *corev1.Pod, containerStatuses ...corev1.ContainerStatus) *corev1.Pod {
	for _, containerStatus := range containerStatuses {
		pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{Name: containerStatus.Name})
	}
	pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, containerStatuses...)
	return pod
}

func (s *RadixJobStepTestSuite) appendJobPodInitContainerStatus(pod *corev1.Pod, containerStatus ...corev1.ContainerStatus) *corev1.Pod {
	pod.Status.InitContainerStatuses = append(pod.Status.InitContainerStatuses, containerStatus...)
	return pod
}

func (s *RadixJobStepTestSuite) getBuildDeployJob(jobName, appName string) utils.JobBuilder {
	return utils.NewJobBuilder().
		WithJobName(jobName).
		WithAppName(appName).
		WithPipelineType(v1.BuildDeploy).
		WithStatus(
			utils.NewJobStatusBuilder().
				WithCondition(v1.JobRunning),
		)
}

func (s *RadixJobStepTestSuite) getWaitingContainerStatus(containerName string) corev1.ContainerStatus {
	return s.getContainerStatus(containerName, corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{}})
}

func (s *RadixJobStepTestSuite) getContainerStatus(containerName string, state corev1.ContainerState) corev1.ContainerStatus {
	return corev1.ContainerStatus{
		Name:  containerName,
		State: state,
	}
}

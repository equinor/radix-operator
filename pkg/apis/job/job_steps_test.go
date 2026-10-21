package job

import (
	"context"
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/random"
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
	name              string
	radixJob          *v1.RadixJob
	radixRegistration *v1.RadixRegistration
	jobs              []*batchv1.Job
	pods              []*corev1.Pod
	configMaps        []*corev1.ConfigMap
	expected          setStatusOfJobTestScenarioExpected
}

func TestRadixJobStepTestSuite(t *testing.T) {
	suite.Run(t, new(RadixJobStepTestSuite))
}

func (s *RadixJobStepTestSuite) Test_StatusJobSteps() {
	startedAt := metav1.NewTime(time.Date(2020, 1, 1, 1, 0, 0, 0, time.Local))
	finishedAt := metav1.NewTime(time.Date(2020, 1, 1, 1, 1, 0, 0, time.Local))

	scenarios := map[string]struct {
		podName         string
		containerName   string
		stepName        string
		stepType        v1.RadixJobStepType
		containerStatus *corev1.ContainerStatus
		components      []string
		environment     []string
		expected        v1.RadixJobStep
	}{
		"test status waiting when containerStatus is nil": {
			expected: v1.RadixJobStep{Condition: v1.JobWaiting},
		},
		"test status waiting when containerStatus is waiting": {
			containerStatus: &corev1.ContainerStatus{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{}}},
			expected:        v1.RadixJobStep{Condition: v1.JobWaiting},
		},
		"test status running when containerStatus is running": {
			containerStatus: &corev1.ContainerStatus{State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: startedAt}}},
			expected:        v1.RadixJobStep{Condition: v1.JobRunning, Started: &startedAt},
		},
		"test status succeeded when containerStatus is terminated and exitCode is 0": {
			containerStatus: &corev1.ContainerStatus{
				State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{StartedAt: startedAt, FinishedAt: finishedAt, ExitCode: 0}},
			},
			expected: v1.RadixJobStep{Condition: v1.JobSucceeded, Started: &startedAt, Ended: &finishedAt},
		},
		"test status failed when containerStatus is terminated and exitCode is not 0": {
			containerStatus: &corev1.ContainerStatus{
				State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{StartedAt: startedAt, FinishedAt: finishedAt, ExitCode: 1}},
			},
			expected: v1.RadixJobStep{Condition: v1.JobFailed, Started: &startedAt, Ended: &finishedAt},
		},
		"test podName, containerName and components set": {
			podName:       "a_pod",
			containerName: "a_container",
			stepName:      "a_step",
			components:    []string{"comp1", "comp2"},
			environment:   []string{"env1", "env2"},
			stepType:      v1.RadixJobStepTypeBuild,
			expected:      v1.RadixJobStep{Condition: v1.JobWaiting, Name: "a_step", Type: v1.RadixJobStepTypeBuild, PodName: "a_pod", ContainerName: "a_container", Components: []string{"comp1", "comp2"}, Environments: []string{"env1", "env2"}},
		},
	}

	for name, scenario := range scenarios {
		s.Run(name, func() {
			actual := getJobStep(scenario.podName, scenario.containerName, scenario.stepName, scenario.stepType, scenario.components, scenario.environment, scenario.containerStatus)
			assert.Equal(s.T(), scenario.expected, actual)
		})
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

func (s *RadixJobStepTestSuite) Test_StatusSteps_HasPipelinePod() {
	scenario := setStatusOfJobTestScenario{
		name:     "no empty steps when clone config has not been created",
		radixJob: s.getBuildDeployJob("job-3", "app-3").BuildRJ(),
		jobs: []*batchv1.Job{
			s.getPipelineJob("job-3", "app-3", "anytag"),
		},
		pods: []*corev1.Pod{
			s.appendJobPodInitContainerStatus(
				s.appendJobPodContainerWithStatus(
					s.getJobPod("pipeline-pod-3", "job-3", "job-3", utils.GetAppNamespace("app-3")),
					s.getWaitingContainerStatus("radix-pipeline"),
				),
				s.getWaitingContainerStatus("clone-config"),
			),
		},
		expected: setStatusOfJobTestScenarioExpected{
			steps: []v1.RadixJobStep{
				{Condition: v1.JobWaiting, Name: "clone-config", Type: v1.RadixJobStepTypeCloneConfig, PodName: "pipeline-pod-3", ContainerName: "clone-config"},
				{Condition: v1.JobWaiting, Name: "radix-pipeline", Type: v1.RadixJobStepTypePipelineRunner, PodName: "pipeline-pod-3", ContainerName: "radix-pipeline"},
			},
		},
	}

	s.testSetStatusOfJobTestScenario(&scenario)
}

func (s *RadixJobStepTestSuite) Test_StatusSteps_BuildAndCloneSteps() {
	scenario := setStatusOfJobTestScenario{
		name:     "pipeline with build steps",
		radixJob: s.getBuildDeployJob("job-5", "app-5").BuildRJ(),
		jobs: []*batchv1.Job{
			s.getPipelineJob("job-5", "app-5", "a_tag"),
			s.getBuildJob("build-job-5", "job-5", "app-5", "env1", "comp1"),
			s.getBuildJob("build-job-6", "job-5", "app-5", "env2", "comp2"),
		},
		pods: []*corev1.Pod{
			s.appendJobPodInitContainerStatus(
				s.appendJobPodContainerWithStatus(
					s.getJobPod("pipeline-pod-5", "job-5", "job-5", utils.GetAppNamespace("app-5")),
					s.getWaitingContainerStatus("radix-pipeline"),
				),
				s.getWaitingContainerStatus("clone-config"),
			),
			s.appendJobPodInitContainerStatus(
				s.appendJobPodContainerWithStatus(
					s.getJobPod("build-pod-5", "job-5", "build-job-5", utils.GetAppNamespace("app-5")),
					s.getWaitingContainerStatus("build"),
				),
				s.getWaitingContainerStatus("clone"),
			),
			s.appendJobPodInitContainerStatus(
				s.appendJobPodContainerWithStatus(
					s.getJobPod("build-pod-6", "job-5", "build-job-6", utils.GetAppNamespace("app-5")),
					s.getWaitingContainerStatus("build"),
				),
				s.getWaitingContainerStatus("clone"),
			),
		},
		expected: setStatusOfJobTestScenarioExpected{
			steps: []v1.RadixJobStep{
				{Condition: v1.JobWaiting, Name: "clone-config", Type: radixv1.RadixJobStepTypeCloneConfig, PodName: "pipeline-pod-5", ContainerName: "clone-config"},
				{Condition: v1.JobWaiting, Name: "radix-pipeline", Type: radixv1.RadixJobStepTypePipelineRunner, PodName: "pipeline-pod-5", ContainerName: "radix-pipeline"},
				{Condition: v1.JobWaiting, Name: "clone-comp1-env1-" + random.RandStringStrSeed(5, "build-pod-5"), Type: radixv1.RadixJobStepTypeClone, PodName: "build-pod-5", ContainerName: "clone", Components: []string{"comp1"}, Environments: []string{"env1"}},
				{Condition: v1.JobWaiting, Name: "build-comp1-env1-" + random.RandStringStrSeed(5, "build-pod-5"), Type: radixv1.RadixJobStepTypeBuild, PodName: "build-pod-5", ContainerName: "build", Components: []string{"comp1"}, Environments: []string{"env1"}},
				{Condition: v1.JobWaiting, Name: "clone-comp2-env2-" + random.RandStringStrSeed(5, "build-pod-6"), Type: radixv1.RadixJobStepTypeClone, PodName: "build-pod-6", ContainerName: "clone", Components: []string{"comp2"}, Environments: []string{"env2"}},
				{Condition: v1.JobWaiting, Name: "build-comp2-env2-" + random.RandStringStrSeed(5, "build-pod-6"), Type: radixv1.RadixJobStepTypeBuild, PodName: "build-pod-6", ContainerName: "build", Components: []string{"comp2"}, Environments: []string{"env2"}},
			},
		},
	}

	s.testSetStatusOfJobTestScenario(&scenario)
}

func (s *RadixJobStepTestSuite) Test_StatusSteps_UnknownContainerNamesAreIgnored() {
	scenario := setStatusOfJobTestScenario{
		name:     "pipeline with build steps",
		radixJob: s.getBuildDeployJob("job-5", "app-5").BuildRJ(),
		jobs: []*batchv1.Job{
			s.getPipelineJob("job-5", "app-5", "a_tag"),
			s.getBuildJob("build-job-5", "job-5", "app-5", "env1", "comp1"),
		},
		pods: []*corev1.Pod{
			s.appendJobPodInitContainerStatus(
				s.appendJobPodContainerWithStatus(
					s.getJobPod("pipeline-pod-5", "job-5", "job-5", utils.GetAppNamespace("app-5")),
					s.getWaitingContainerStatus("radix-pipeline"),
					s.getWaitingContainerStatus("radix-pipeline2"),
				),
				s.getWaitingContainerStatus("clone-config"),
				s.getWaitingContainerStatus("clone-config2"),
			),
			s.appendJobPodInitContainerStatus(
				s.appendJobPodContainerWithStatus(
					s.getJobPod("build-pod-5", "job-5", "build-job-5", utils.GetAppNamespace("app-5")),
					s.getWaitingContainerStatus("build"),
					s.getWaitingContainerStatus("ignored-prefix-build"),
				),
				s.getWaitingContainerStatus("clone"),
				s.getWaitingContainerStatus("ignoredß-prefix-clone"),
			),
		},
		expected: setStatusOfJobTestScenarioExpected{
			steps: []v1.RadixJobStep{
				{Condition: v1.JobWaiting, Name: "clone-config", Type: radixv1.RadixJobStepTypeCloneConfig, PodName: "pipeline-pod-5", ContainerName: "clone-config"},
				{Condition: v1.JobWaiting, Name: "radix-pipeline", Type: radixv1.RadixJobStepTypePipelineRunner, PodName: "pipeline-pod-5", ContainerName: "radix-pipeline"},
				{Condition: v1.JobWaiting, Name: "clone-comp1-env1-" + random.RandStringStrSeed(5, "build-pod-5"), Type: radixv1.RadixJobStepTypeClone, PodName: "build-pod-5", ContainerName: "clone", Components: []string{"comp1"}, Environments: []string{"env1"}},
				{Condition: v1.JobWaiting, Name: "build-comp1-env1-" + random.RandStringStrSeed(5, "build-pod-5"), Type: radixv1.RadixJobStepTypeBuild, PodName: "build-pod-5", ContainerName: "build", Components: []string{"comp1"}, Environments: []string{"env1"}},
			},
		},
	}

	s.testSetStatusOfJobTestScenario(&scenario)
}

// TODO: Remove this test once all running RadixJobs withdeprecated build container names are in a completed state.
func (s *RadixJobStepTestSuite) Test_StatusSteps_DeprecatedBuildContainerNames() {
	scenario := setStatusOfJobTestScenario{
		name:     "pipeline with build steps",
		radixJob: s.getBuildDeployJob("job-5", "app-5").BuildRJ(),
		jobs: []*batchv1.Job{
			s.getPipelineJob("job-5", "app-5", "a_tag"),
			s.getBuildJob("build-job-5", "job-5", "app-5", "env1", "comp1"),
			s.getBuildJob("build-job-6", "job-5", "app-5", "env2", "comp2"),
		},
		pods: []*corev1.Pod{
			s.appendJobPodInitContainerStatus(
				s.appendJobPodContainerWithStatus(
					s.getJobPod("pipeline-pod-5", "job-5", "job-5", utils.GetAppNamespace("app-5")),
					s.getWaitingContainerStatus("radix-pipeline"),
				),
				s.getWaitingContainerStatus("clone-config"),
			),
			s.appendJobPodInitContainerStatus(
				s.appendJobPodContainerWithStatus(
					s.getJobPod("build-pod-5", "job-5", "build-job-5", utils.GetAppNamespace("app-5")),
					s.getWaitingContainerStatus("build"),
				),
				s.getWaitingContainerStatus("clone"),
			),
			s.appendJobPodInitContainerStatus(
				s.appendJobPodContainerWithStatus(
					s.getJobPod("build-pod-6", "job-5", "build-job-6", utils.GetAppNamespace("app-5")),
					s.getWaitingContainerStatus("build-custom-name"),
				),
				s.getWaitingContainerStatus("clone"),
			),
		},
		expected: setStatusOfJobTestScenarioExpected{
			steps: []v1.RadixJobStep{
				{Condition: v1.JobWaiting, Name: "clone-config", Type: radixv1.RadixJobStepTypeCloneConfig, PodName: "pipeline-pod-5", ContainerName: "clone-config"},
				{Condition: v1.JobWaiting, Name: "radix-pipeline", Type: radixv1.RadixJobStepTypePipelineRunner, PodName: "pipeline-pod-5", ContainerName: "radix-pipeline"},
				{Condition: v1.JobWaiting, Name: "clone-comp1-env1-" + random.RandStringStrSeed(5, "build-pod-5"), Type: radixv1.RadixJobStepTypeClone, PodName: "build-pod-5", ContainerName: "clone", Components: []string{"comp1"}, Environments: []string{"env1"}},
				{Condition: v1.JobWaiting, Name: "build-comp1-env1-" + random.RandStringStrSeed(5, "build-pod-5"), Type: radixv1.RadixJobStepTypeBuild, PodName: "build-pod-5", ContainerName: "build", Components: []string{"comp1"}, Environments: []string{"env1"}},
				{Condition: v1.JobWaiting, Name: "clone-comp2-env2-" + random.RandStringStrSeed(5, "build-pod-6"), Type: radixv1.RadixJobStepTypeClone, PodName: "build-pod-6", ContainerName: "clone", Components: []string{"comp2"}, Environments: []string{"env2"}},
				{Condition: v1.JobWaiting, Name: "build-custom-name", Type: radixv1.RadixJobStepTypeBuild, PodName: "build-pod-6", ContainerName: "build-custom-name", Components: []string{"comp2"}, Environments: []string{"env2"}},
			},
		},
	}

	s.testSetStatusOfJobTestScenario(&scenario)
}

func (s *RadixJobStepTestSuite) testSetStatusOfJobTestScenario(scenario *setStatusOfJobTestScenario) {
	err := s.initScenario(scenario)
	require.NoError(s.T(), err, "scenario %s", scenario.name)

	job := NewJob(s.kubeClient, s.kubeUtils, s.radixClient, s.dynamicClient, scenario.radixRegistration, scenario.radixJob, s.cfg)
	err = job.syncStatus(context.Background(), nil)
	require.NoError(s.T(), err, "scenario %s", scenario.name)

	actualRj, err := s.radixClient.RadixV1().RadixJobs(scenario.radixJob.Namespace).Get(context.Background(), scenario.radixJob.Name, metav1.GetOptions{})
	require.NoError(s.T(), err, "scenario %s", scenario.name)

	assert.Equal(s.T(), scenario.expected.returnsError, err != nil, scenario.name)
	assert.ElementsMatch(s.T(), scenario.expected.steps, actualRj.Status.Steps, scenario.name)
}

func (s *RadixJobStepTestSuite) initScenario(scenario *setStatusOfJobTestScenario) error {
	rr := utils.ARadixRegistration().WithName(scenario.radixJob.Spec.AppName).BuildRR()
	newRa, err := s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	scenario.radixRegistration = newRa

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
		Name:      name,
		Namespace: utils.GetAppNamespace(appName),
		Labels: map[string]string{
			kube.RadixJobNameLabel:  name,
			kube.RadixJobTypeLabel:  kube.RadixJobTypeJob,
			kube.RadixImageTagLabel: imageTag,
			kube.RadixAppLabel:      appName,
		},
	}
}

func (s *RadixJobStepTestSuite) getBuildJob(name, radixJobName, appName, envName, componentName string) *batchv1.Job {
	return &batchv1.Job{
		Name:      name,
		Namespace: utils.GetAppNamespace(appName),
		Labels: map[string]string{
			kube.RadixJobNameLabel:   radixJobName,
			kube.RadixAppLabel:       appName,
			kube.RadixJobTypeLabel:   kube.RadixJobTypeBuild,
			kube.RadixEnvLabel:       envName,
			kube.RadixComponentLabel: componentName,
		},
	}
}

func (s *RadixJobStepTestSuite) getJobPod(podName, radixJobName, jobName, namespace string) *corev1.Pod {
	return &corev1.Pod{
		Name:      podName,
		Namespace: namespace,
		Labels:    map[string]string{kube.RadixJobNameLabel: radixJobName, jobNameLabel: jobName},
	}
}

func (s *RadixJobStepTestSuite) appendJobPodContainerWithStatus(pod *corev1.Pod, containerStatuses ...corev1.ContainerStatus) *corev1.Pod {
	for _, containerStatus := range containerStatuses {
		pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{Name: containerStatus.Name})
	}
	pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, containerStatuses...)
	return pod
}

func (s *RadixJobStepTestSuite) appendJobPodInitContainerStatus(pod *corev1.Pod, containerStatuses ...corev1.ContainerStatus) *corev1.Pod {
	for _, containerStatus := range containerStatuses {
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{Name: containerStatus.Name})
	}
	pod.Status.InitContainerStatuses = append(pod.Status.InitContainerStatuses, containerStatuses...)
	return pod
}

func (s *RadixJobStepTestSuite) getBuildDeployJob(jobName, appName string) utils.JobBuilder {
	return utils.NewJobBuilder().
		WithJobName(jobName).
		WithPipelineType(v1.BuildDeploy).
		WithAppName(appName).
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

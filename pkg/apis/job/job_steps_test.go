package job

import (
	"context"
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/kube"
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

type radixJobStepTestScenarioExpected struct {
	steps        []v1.RadixJobStep
	returnsError bool
}

type radixJobStepTestScenario struct {
	name           string
	radixjob       *v1.RadixJob
	pipelineJob    *batchv1.Job
	pipelinePod    *corev1.Pod
	cloneConfigJob *batchv1.Job
	cloneConfigPod *corev1.Pod
	expected       radixJobStepTestScenarioExpected
}

func TestRadixJobStepTestSuite(t *testing.T) {
	suite.Run(t, new(RadixJobStepTestSuite))
}

func (s *RadixJobStepTestSuite) Test_setStatusOfJob() {
	scenarios := []radixJobStepTestScenario{
		{
			name:     "missing pipeline job",
			radixjob: s.getBuildDeployJob("job-1", "app-1").BuildRJ(),
		},
		{
			name:        "missing pipeline pod",
			radixjob:    s.getBuildDeployJob("job-2", "app-2").BuildRJ(),
			pipelineJob: s.getPipelineJob("job-2", utils.GetAppNamespace("app-2")),
		},
		{
			name:        "no empty steps when clone config has not been created",
			radixjob:    s.getBuildDeployJob("job-3", "app-3").BuildRJ(),
			pipelineJob: s.getPipelineJob("job-3", utils.GetAppNamespace("app-3")),
			pipelinePod: s.appendJobPodContainerStatus(
				s.getJobPod("pipeline-pod-3", "job-3", utils.GetAppNamespace("app-3")),
				s.getWaitingContainerStatus("radix-pipeline")),
			expected: radixJobStepTestScenarioExpected{
				steps: []v1.RadixJobStep{{Condition: v1.JobWaiting, Name: "radix-pipeline", PodName: "pipeline-pod-3"}},
			},
		},
		{
			name:        "no empty steps when clone config has not been created",
			radixjob:    s.getBuildDeployJob("job-4", "app-4").BuildRJ(),
			pipelineJob: s.getPipelineJob("job-4", utils.GetAppNamespace("app-4")),
			pipelinePod: s.appendJobPodContainerStatus(
				s.getJobPod("pipeline-pod-4", "job-4", utils.GetAppNamespace("app-4")),
				s.getWaitingContainerStatus("radix-pipeline")),
			cloneConfigJob: s.getCloneConfigJob("clone-job-4", "job-4", utils.GetAppNamespace("app-4")),
			cloneConfigPod: s.appendJobPodInitContainerStatus(
				s.appendJobPodContainerStatus(
					s.getJobPod("clone-pod-4", "clone-job-4", utils.GetAppNamespace("app-4")),
					s.getWaitingContainerStatus("apply-config")),
				s.getWaitingContainerStatus("clone-config")),
			expected: radixJobStepTestScenarioExpected{
				steps: []v1.RadixJobStep{
					{Condition: v1.JobWaiting, Name: "clone-config", PodName: "clone-pod-4"},
					{Condition: v1.JobWaiting, Name: "apply-config", PodName: "clone-pod-4"},
					{Condition: v1.JobWaiting, Name: "radix-pipeline", PodName: "pipeline-pod-4"},
				},
			},
		},
	}

	for _, scenario := range scenarios {
		if err := s.initScenario(&scenario); err != nil {
			assert.FailNowf(s.T(), err.Error(), "scenario %s", scenario.name)
		}

		job := NewJob(s.kubeClient, s.kubeUtils, s.radixClient, scenario.radixjob)
		err := job.setStatusOfJob()

		actualRj, err := s.radixClient.RadixV1().RadixJobs(scenario.radixjob.Namespace).Get(context.Background(), scenario.radixjob.Name, metav1.GetOptions{})
		if err != nil {
			assert.FailNowf(s.T(), err.Error(), "scenario %s", scenario.name)
		}

		assert.Equal(s.T(), scenario.expected.returnsError, err != nil, scenario.name)
		assert.ElementsMatch(s.T(), scenario.expected.steps, actualRj.Status.Steps, scenario.name)
	}

}

func (s *RadixJobStepTestSuite) initScenario(scenario *radixJobStepTestScenario) error {
	if scenario.radixjob != nil {
		if _, err := s.radixClient.RadixV1().RadixJobs(scenario.radixjob.Namespace).Create(context.Background(), scenario.radixjob, metav1.CreateOptions{}); err != nil {
			return err
		}
	}

	if scenario.pipelineJob != nil {
		if _, err := s.kubeClient.BatchV1().Jobs(scenario.pipelineJob.Namespace).Create(context.Background(), scenario.pipelineJob, metav1.CreateOptions{}); err != nil {
			return err
		}
	}

	if scenario.pipelinePod != nil {
		if _, err := s.kubeClient.CoreV1().Pods(scenario.pipelinePod.Namespace).Create(context.Background(), scenario.pipelinePod, metav1.CreateOptions{}); err != nil {
			return err
		}
	}

	if scenario.cloneConfigJob != nil {
		if _, err := s.kubeClient.BatchV1().Jobs(scenario.cloneConfigJob.Namespace).Create(context.Background(), scenario.cloneConfigJob, metav1.CreateOptions{}); err != nil {
			return err
		}
	}

	if scenario.cloneConfigPod != nil {
		if _, err := s.kubeClient.CoreV1().Pods(scenario.cloneConfigPod.Namespace).Create(context.Background(), scenario.cloneConfigPod, metav1.CreateOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (s *RadixJobStepTestSuite) getPipelineJob(name, namespace string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func (s *RadixJobStepTestSuite) getCloneConfigJob(name, radixJobName, namespace string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{kube.RadixJobNameLabel: radixJobName, kube.RadixJobTypeLabel: kube.RadixJobTypeCloneConfig},
		},
	}
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
	for _, s := range containerStatus {
		pod.Status.ContainerStatuses = append(pod.Status.InitContainerStatuses, s)
	}

	return pod
}

func (s *RadixJobStepTestSuite) appendJobPodInitContainerStatus(pod *corev1.Pod, containerStatus ...corev1.ContainerStatus) *corev1.Pod {
	for _, s := range containerStatus {
		pod.Status.InitContainerStatuses = append(pod.Status.InitContainerStatuses, s)
	}

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

func (s *RadixJobStepTestSuite) getRunningContainerStatus(containerName string, startedAt time.Time) corev1.ContainerStatus {
	return s.getContainerStatus(containerName, corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(startedAt)}})
}

func (s *RadixJobStepTestSuite) getTerminatedContainerStatus(containerName string, startedAt, finishedAt time.Time, exitCode int32) corev1.ContainerStatus {
	return s.getContainerStatus(containerName, corev1.ContainerState{
		Terminated: &corev1.ContainerStateTerminated{StartedAt: metav1.NewTime(startedAt), FinishedAt: metav1.NewTime(finishedAt), ExitCode: exitCode},
	})
}

func (s *RadixJobStepTestSuite) getContainerStatus(containerName string, state corev1.ContainerState) corev1.ContainerStatus {
	status := corev1.ContainerStatus{
		Name:  containerName,
		State: state,
	}

	return status
}

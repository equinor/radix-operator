package batch

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/equinor/radix-common/utils/numbers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/securitycontext"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/suite"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

type syncerTestSuite struct {
	suite.Suite
	kubeClient  *fake.Clientset
	radixClient *fakeradix.Clientset
	kubeUtil    *kube.Kube
	promClient  *prometheusfake.Clientset
}

func TestSyncerTestSuite(t *testing.T) {
	suite.Run(t, new(syncerTestSuite))
}

func (s *syncerTestSuite) createSyncer(forJob *radixv1.RadixBatch) Syncer {
	return &syncer{kubeclient: s.kubeClient, kubeutil: s.kubeUtil, radixclient: s.radixClient, batch: forJob}
}

func (s *syncerTestSuite) applyRadixDeploymentEnvVarsConfigMaps(kubeUtil *kube.Kube, rd *radixv1.RadixDeployment) map[string]*corev1.ConfigMap {
	envVarConfigMapsMap := map[string]*corev1.ConfigMap{}
	for _, deployComponent := range rd.Spec.Components {
		envVarConfigMapsMap[deployComponent.GetName()] = s.ensurePopulatedEnvVarsConfigMaps(kubeUtil, rd, &deployComponent)
	}
	for _, deployJoyComponent := range rd.Spec.Jobs {
		envVarConfigMapsMap[deployJoyComponent.GetName()] = s.ensurePopulatedEnvVarsConfigMaps(kubeUtil, rd, &deployJoyComponent)
	}
	return envVarConfigMapsMap
}

func (s *syncerTestSuite) ensurePopulatedEnvVarsConfigMaps(kubeUtil *kube.Kube, rd *radixv1.RadixDeployment, deployComponent radixv1.RadixCommonDeployComponent) *corev1.ConfigMap {
	initialEnvVarsConfigMap, _, _ := kubeUtil.GetOrCreateEnvVarsConfigMapAndMetadataMap(rd.GetNamespace(),
		rd.Spec.AppName, deployComponent.GetName())
	desiredConfigMap := initialEnvVarsConfigMap.DeepCopy()
	for envVarName, envVarValue := range deployComponent.GetEnvironmentVariables() {
		if strings.HasPrefix(envVarName, "RADIX_") {
			continue
		}
		desiredConfigMap.Data[envVarName] = envVarValue
	}
	kubeUtil.ApplyConfigMap(rd.GetNamespace(), initialEnvVarsConfigMap, desiredConfigMap)
	return desiredConfigMap
}

func (s *syncerTestSuite) SetupTest() {
	s.kubeClient = fake.NewSimpleClientset()
	s.radixClient = fakeradix.NewSimpleClientset()
	s.promClient = prometheusfake.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, secretproviderfake.NewSimpleClientset())
	s.T().Setenv(defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable, "1500Mi")
	s.T().Setenv(defaults.OperatorEnvLimitDefaultCPUEnvironmentVariable, "2000m")
	s.T().Setenv(defaults.OperatorRollingUpdateMaxUnavailable, "25%")
	s.T().Setenv(defaults.OperatorRollingUpdateMaxSurge, "25%")
}

func (s *syncerTestSuite) Test_RestoreStatus() {
	created, started, ended := metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local)), metav1.NewTime(time.Date(2020, 1, 2, 0, 0, 0, 0, time.Local)), metav1.NewTime(time.Date(2020, 1, 3, 0, 0, 0, 0, time.Local))
	expectedStatus := radixv1.RadixBatchStatus{
		Condition: radixv1.RadixBatchCondition{
			Type:           radixv1.BatchConditionTypeCompleted,
			Reason:         "any reson",
			Message:        "any message",
			ActiveTime:     &started,
			CompletionTime: &ended,
		},
		JobStatuses: []radixv1.RadixBatchJobStatus{
			{
				Name:         "job1",
				Phase:        radixv1.BatchJobPhaseSucceeded,
				Reason:       "any-reason1",
				Message:      "any-message1",
				CreationTime: &created,
				StartTime:    &started,
				EndTime:      &ended,
			},
			{
				Name:         "job1",
				Phase:        radixv1.BatchJobPhaseFailed,
				Reason:       "any-reason2",
				Message:      "any-message2",
				CreationTime: &created,
				StartTime:    &started,
				EndTime:      &ended,
			},
		},
	}
	statusBytes, err := json.Marshal(&expectedStatus)
	s.Require().NoError(err)

	jobName, namespace := "any-job", "any-ns"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: jobName, Annotations: map[string]string{kube.RestoredStatusAnnotation: string(statusBytes)}},
	}
	batch, err = s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	sut := s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	batch, err = s.radixClient.RadixV1().RadixBatches(namespace).Get(context.Background(), jobName, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal(expectedStatus, batch.Status)
}

func (s *syncerTestSuite) Test_RestoreStatusWithInvalidAnnotationValueShouldReturnErrorAndSkipReconcile() {
	jobName, namespace := "any-job", "any-ns"
	appName, batchName, componentName, namespace, rdName := "any-app", "any-batch", "compute", "any-ns", "any-rd"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName, Annotations: map[string]string{kube.RestoredStatusAnnotation: "invalid data"}},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  componentName,
			},
			Jobs: []radixv1.RadixBatchJob{
				{Name: jobName},
			},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: appName,
			Jobs: []radixv1.RadixDeployJobComponent{
				{
					Name:  componentName,
					Ports: []radixv1.ComponentPort{{Name: "port1", Port: 8000}, {Name: "port2", Port: 9000}},
				},
			},
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)
	sut := s.createSyncer(batch)
	err = sut.OnSync()
	s.Require().Error(err)
	s.Contains(err.Error(), "unable to restore status for batch")
	batch, err = s.radixClient.RadixV1().RadixBatches(namespace).Get(context.Background(), batchName, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Empty(batch.Status)
	jobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Len(jobs.Items, 0)
	services, _ := s.kubeClient.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{})
	s.Len(services.Items, 0)
}

func (s *syncerTestSuite) Test_ShouldRestoreStatusFromAnnotationWhenStatusEmpty() {
	created, started, ended := metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local)), metav1.NewTime(time.Date(2020, 1, 2, 0, 0, 0, 0, time.Local)), metav1.NewTime(time.Date(2020, 1, 3, 0, 0, 0, 0, time.Local))
	expectedStatus := radixv1.RadixBatchStatus{
		Condition: radixv1.RadixBatchCondition{
			Type:    radixv1.BatchConditionTypeCompleted,
			Reason:  "any reson",
			Message: "any message",
		},
		JobStatuses: []radixv1.RadixBatchJobStatus{
			{
				Name:         "job1",
				Phase:        radixv1.BatchJobPhaseSucceeded,
				Reason:       "any-reason1",
				Message:      "any-message1",
				CreationTime: &created,
				StartTime:    &started,
				EndTime:      &ended,
			},
			{
				Name:         "job1",
				Phase:        radixv1.BatchJobPhaseFailed,
				Reason:       "any-reason2",
				Message:      "any-message2",
				CreationTime: &created,
				StartTime:    &started,
				EndTime:      &ended,
			},
		},
	}
	statusBytes, err := json.Marshal(&expectedStatus)
	s.Require().NoError(err)

	jobName, namespace := "any-job", "any-ns"
	job := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: jobName, Annotations: map[string]string{kube.RestoredStatusAnnotation: string(statusBytes)}},
		Status:     radixv1.RadixBatchStatus{},
	}
	job, err = s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), job, metav1.CreateOptions{})
	s.Require().NoError(err)
	sut := s.createSyncer(job)
	s.Require().NoError(sut.OnSync())
	job, err = s.radixClient.RadixV1().RadixBatches(namespace).Get(context.Background(), jobName, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal(expectedStatus, job.Status)
}

func (s *syncerTestSuite) Test_ShouldNotRestoreStatusFromAnnotationWhenStatusNotEmpty() {
	annotationStatus := radixv1.RadixBatchStatus{
		Condition: radixv1.RadixBatchCondition{
			Type:    radixv1.BatchConditionTypeActive,
			Reason:  "annotation reson",
			Message: "annotation message",
		},
	}
	statusBytes, err := json.Marshal(&annotationStatus)
	s.Require().NoError(err)

	jobName, namespace := "any-job", "any-ns"
	expectedStatus := radixv1.RadixBatchStatus{
		Condition: radixv1.RadixBatchCondition{
			Type:    radixv1.BatchConditionTypeCompleted,
			Reason:  "any reson",
			Message: "any message",
		},
	}
	job := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: jobName, Annotations: map[string]string{kube.RestoredStatusAnnotation: string(statusBytes)}},
		Status:     expectedStatus,
	}
	job, err = s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), job, metav1.CreateOptions{})
	s.Require().NoError(err)
	sut := s.createSyncer(job)
	s.Require().NoError(sut.OnSync())
	job, err = s.radixClient.RadixV1().RadixBatches(namespace).Get(context.Background(), jobName, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal(expectedStatus, job.Status)
}

func (s *syncerTestSuite) Test_ShouldSkipReconcileResourcesWhenBatchConditionIsDone() {
	doneConditions := []radixv1.RadixBatchConditionType{radixv1.BatchConditionTypeCompleted}

	for i, conditionType := range doneConditions {
		s.Run(string(conditionType), func() {
			jobName, namespace := fmt.Sprintf("any-job-%d", i), "any-ns"
			expectedStatus := radixv1.RadixBatchStatus{Condition: radixv1.RadixBatchCondition{Type: conditionType}}
			batch := &radixv1.RadixBatch{
				ObjectMeta: metav1.ObjectMeta{Name: jobName},
				Status:     expectedStatus,
			}
			batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
			s.Require().NoError(err)
			sut := s.createSyncer(batch)
			s.Require().NoError(sut.OnSync())
			batch, err = s.radixClient.RadixV1().RadixBatches(namespace).Get(context.Background(), jobName, metav1.GetOptions{})
			s.Require().NoError(err)
			s.Equal(expectedStatus, batch.Status)
			jobs, err := s.kubeClient.BatchV1().Jobs(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
			s.Require().NoError(err)
			s.Len(jobs.Items, 0)
			services, err := s.kubeClient.CoreV1().Services(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
			s.Require().NoError(err)
			s.Len(services.Items, 0)
		})
	}

}

func (s *syncerTestSuite) Test_ServiceCreated() {
	appName, batchName, componentName, namespace, rdName := "any-app", "any-batch", "compute", "any-ns", "any-rd"
	job1Name, job2Name := "job1", "job2"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  componentName,
			},
			Jobs: []radixv1.RadixBatchJob{
				{Name: job1Name},
				{Name: job2Name},
			},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: appName,
			Jobs: []radixv1.RadixDeployJobComponent{
				{
					Name:  componentName,
					Ports: []radixv1.ComponentPort{{Name: "port1", Port: 8000}, {Name: "port2", Port: 9000}},
				},
			},
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)

	sut := s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	allServices, _ := s.kubeClient.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{})
	s.Len(allServices.Items, 2)
	for _, jobName := range []string{job1Name, job2Name} {
		jobServices := slice.FindAll(allServices.Items, func(svc corev1.Service) bool { return svc.Name == getKubeServiceName(batchName, jobName) })
		s.Len(jobServices, 1)
		service := jobServices[0]
		expectedServiceLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: componentName, kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule, kube.RadixBatchNameLabel: batchName, kube.RadixBatchJobNameLabel: jobName}
		s.Equal(expectedServiceLabels, service.Labels, "service labels")
		s.Equal(ownerReference(batch), service.OwnerReferences)
		expectedSelectorLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: componentName, kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule, kube.RadixBatchNameLabel: batchName, kube.RadixBatchJobNameLabel: jobName}
		s.Equal(expectedSelectorLabels, service.Spec.Selector, "selector")
		s.ElementsMatch([]corev1.ServicePort{{Name: "port1", Port: 8000, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(8000)}, {Name: "port2", Port: 9000, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(9000)}}, service.Spec.Ports)
	}
}

func (s *syncerTestSuite) Test_ServiceNotCreatedWhenPortsIsEmpty() {
	appName, batchName, componentName, namespace, rdName := "any-app", "any-batch", "compute", "any-ns", "any-rd"
	job1Name := "job1"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  componentName,
			},
			Jobs: []radixv1.RadixBatchJob{
				{Name: job1Name},
			},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: appName,
			Jobs: []radixv1.RadixDeployJobComponent{
				{
					Name: componentName,
				},
			},
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)

	sut := s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	allServices, _ := s.kubeClient.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{})
	s.Len(allServices.Items, 0)
}

func (s *syncerTestSuite) Test_ServiceNotCreatedForJobWithPhaseDone() {
	appName, batchName, componentName, namespace, rdName := "any-app", "any-batch", "compute", "any-ns", "any-rd"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  componentName,
			},
			Jobs: []radixv1.RadixBatchJob{
				{Name: "job1"},
				{Name: "job2"},
				{Name: "job3"},
				{Name: "job4"},
				{Name: "job5"},
			},
		},
		Status: radixv1.RadixBatchStatus{
			JobStatuses: []radixv1.RadixBatchJobStatus{
				{Name: "job1", Phase: radixv1.BatchJobPhaseSucceeded},
				{Name: "job2", Phase: radixv1.BatchJobPhaseFailed},
				{Name: "job3", Phase: radixv1.BatchJobPhaseStopped},
				{Name: "job4", Phase: radixv1.BatchJobPhaseWaiting},
				{Name: "job5", Phase: radixv1.BatchJobPhaseActive},
			},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: appName,
			Jobs: []radixv1.RadixDeployJobComponent{
				{
					Name:  componentName,
					Ports: []radixv1.ComponentPort{{Name: "port1", Port: 8000}, {Name: "port2", Port: 9000}},
				},
			},
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)

	sut := s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	allServices, _ := s.kubeClient.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{})
	s.ElementsMatch([]string{getKubeServiceName(batchName, "job4"), getKubeServiceName(batchName, "job5")}, slice.Map(allServices.Items, func(svc corev1.Service) string { return svc.GetName() }))
}

func (s *syncerTestSuite) Test_BatchStaticConfiguration() {
	appName, batchName, componentName, namespace, rdName, imageName := "any-app", "any-batch", "compute", "any-ns", "any-rd", "any-image"
	job1Name, job2Name := "job1", "job2"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  componentName,
			},
			Jobs: []radixv1.RadixBatchJob{
				{Name: job1Name},
				{Name: job2Name},
			},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: appName,
			Jobs: []radixv1.RadixDeployJobComponent{
				{
					Name:                 componentName,
					Image:                imageName,
					EnvironmentVariables: radixv1.EnvVarsMap{"VAR1": "any-val", "VAR2": "any-val"},
					Secrets:              []string{"SECRET1", "SECRET2"},
				},
			},
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	rd, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.applyRadixDeploymentEnvVarsConfigMaps(s.kubeUtil, rd)
	sut := s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	allJobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Len(allJobs.Items, 2)
	for _, jobName := range []string{job1Name, job2Name} {
		jobKubeJobs := slice.FindAll(allJobs.Items, func(job batchv1.Job) bool { return job.Name == getKubeJobName(batchName, jobName) })
		s.Len(jobKubeJobs, 1)
		kubejob := jobKubeJobs[0]
		expectedJobLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: componentName, kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule, kube.RadixBatchNameLabel: batchName, kube.RadixBatchJobNameLabel: jobName}
		s.Equal(expectedJobLabels, kubejob.Labels, "job labels")
		expectedPodLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: componentName, kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule, kube.RadixBatchNameLabel: batchName, kube.RadixBatchJobNameLabel: jobName}
		s.Equal(expectedPodLabels, kubejob.Spec.Template.Labels, "pod labels")
		s.Equal(ownerReference(batch), kubejob.OwnerReferences)
		s.Equal(numbers.Int32Ptr(0), kubejob.Spec.BackoffLimit)
		s.Equal(corev1.RestartPolicyNever, kubejob.Spec.Template.Spec.RestartPolicy)
		s.Equal(securitycontext.Pod(securitycontext.WithPodSeccompProfile(corev1.SeccompProfileTypeRuntimeDefault)), kubejob.Spec.Template.Spec.SecurityContext)
		s.Equal(imageName, kubejob.Spec.Template.Spec.Containers[0].Image)
		s.Equal(securitycontext.Container(securitycontext.WithContainerSeccompProfileType(corev1.SeccompProfileTypeRuntimeDefault)), kubejob.Spec.Template.Spec.Containers[0].SecurityContext)
		s.Len(kubejob.Spec.Template.Spec.Containers[0].Resources.Limits, 0)
		s.Len(kubejob.Spec.Template.Spec.Containers[0].Resources.Requests, 0)
		s.Len(kubejob.Spec.Template.Spec.Containers[0].Env, 5)
		s.True(slice.Any(kubejob.Spec.Template.Spec.Containers[0].Env, func(env corev1.EnvVar) bool {
			return env.Name == "VAR1" && env.ValueFrom.ConfigMapKeyRef.Key == "VAR1" && env.ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name == kube.GetEnvVarsConfigMapName(componentName)
		}))
		s.True(slice.Any(kubejob.Spec.Template.Spec.Containers[0].Env, func(env corev1.EnvVar) bool {
			return env.Name == "VAR2" && env.ValueFrom.ConfigMapKeyRef.Key == "VAR2" && env.ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name == kube.GetEnvVarsConfigMapName(componentName)
		}))
		s.True(slice.Any(kubejob.Spec.Template.Spec.Containers[0].Env, func(env corev1.EnvVar) bool {
			return env.Name == "SECRET1" && env.ValueFrom.SecretKeyRef.Key == "SECRET1" && env.ValueFrom.SecretKeyRef.LocalObjectReference.Name == utils.GetComponentSecretName(componentName)
		}))
		s.True(slice.Any(kubejob.Spec.Template.Spec.Containers[0].Env, func(env corev1.EnvVar) bool {
			return env.Name == "SECRET2" && env.ValueFrom.SecretKeyRef.Key == "SECRET2" && env.ValueFrom.SecretKeyRef.LocalObjectReference.Name == utils.GetComponentSecretName(componentName)
		}))
		s.True(slice.Any(kubejob.Spec.Template.Spec.Containers[0].Env, func(env corev1.EnvVar) bool {
			return env.Name == defaults.RadixScheduleJobNameEnvironmentVariable && env.Value == kubejob.GetName()
		}))
		s.Equal(corev1.PullAlways, kubejob.Spec.Template.Spec.Containers[0].ImagePullPolicy)
		s.Equal("default", kubejob.Spec.Template.Spec.ServiceAccountName)
		s.Equal(utils.BoolPtr(false), kubejob.Spec.Template.Spec.AutomountServiceAccountToken)
		s.Len(kubejob.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms, 1)
		s.Equal(corev1.NodeSelectorRequirement{
			Key:      kube.RadixNodeScheduledJobLabel,
			Operator: corev1.NodeSelectorOpExists,
		}, kubejob.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0])
		s.Len(kubejob.Spec.Template.Spec.Tolerations, 1)
		s.Equal(corev1.Toleration{
			Key:      kube.NodeTaintScheduledKey,
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		}, kubejob.Spec.Template.Spec.Tolerations[0])
		s.Len(kubejob.Spec.Template.Spec.Volumes, 0)
		s.Len(kubejob.Spec.Template.Spec.Containers[0].VolumeMounts, 0)
		services, err := s.kubeClient.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{})
		s.Require().NoError(err)
		s.Len(services.Items, 0)
	}
}

func (s *syncerTestSuite) Test_JobNotCreatedForJobWithPhaseDone() {
	appName, batchName, componentName, namespace, rdName := "any-app", "any-batch", "compute", "any-ns", "any-rd"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  componentName,
			},
			Jobs: []radixv1.RadixBatchJob{
				{Name: "job1"},
				{Name: "job2"},
				{Name: "job3"},
				{Name: "job4"},
				{Name: "job5"},
			},
		},
		Status: radixv1.RadixBatchStatus{
			JobStatuses: []radixv1.RadixBatchJobStatus{
				{Name: "job1", Phase: radixv1.BatchJobPhaseSucceeded},
				{Name: "job2", Phase: radixv1.BatchJobPhaseFailed},
				{Name: "job3", Phase: radixv1.BatchJobPhaseStopped},
				{Name: "job4", Phase: radixv1.BatchJobPhaseWaiting},
				{Name: "job5", Phase: radixv1.BatchJobPhaseActive},
			},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: appName,
			Jobs: []radixv1.RadixDeployJobComponent{
				{
					Name: componentName,
				},
			},
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)

	sut := s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	allJobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.ElementsMatch([]string{getKubeJobName(batchName, "job4"), getKubeJobName(batchName, "job5")}, slice.Map(allJobs.Items, func(job batchv1.Job) string { return job.GetName() }))
}

func (s *syncerTestSuite) Test_BatchJobTimeLimitSeconds() {
	appName, batchName, componentName, namespace, rdName := "any-app", "any-batch", "compute", "any-ns", "any-rd"
	job1Name, job2Name := "job1", "job2"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  componentName,
			},
			Jobs: []radixv1.RadixBatchJob{
				{Name: job1Name},
				{Name: job2Name, TimeLimitSeconds: numbers.Int64Ptr(234)},
			},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: appName,
			Jobs: []radixv1.RadixDeployJobComponent{
				{
					Name:             componentName,
					TimeLimitSeconds: numbers.Int64Ptr(123),
				},
			},
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)
	sut := s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	allJobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(allJobs.Items, 2)
	job1 := slice.FindAll(allJobs.Items, func(job batchv1.Job) bool { return job.GetName() == getKubeJobName(batchName, job1Name) })[0]
	s.Equal(numbers.Int64Ptr(123), job1.Spec.Template.Spec.ActiveDeadlineSeconds)
	job2 := slice.FindAll(allJobs.Items, func(job batchv1.Job) bool { return job.GetName() == getKubeJobName(batchName, job2Name) })[0]
	s.Equal(numbers.Int64Ptr(234), job2.Spec.Template.Spec.ActiveDeadlineSeconds)
}

func (s *syncerTestSuite) Test_BatchJobBackoffLimit_WithJobComponentDefault() {
	appName, batchName, componentName, namespace, rdName := "any-app", "any-batch", "compute", "any-ns", "any-rd"
	job1Name, job2Name := "job1", "job2"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  componentName,
			},
			Jobs: []radixv1.RadixBatchJob{
				{Name: job1Name},
				{Name: job2Name, BackoffLimit: numbers.Int32Ptr(5)},
			},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: appName,
			Jobs: []radixv1.RadixDeployJobComponent{
				{
					Name:         componentName,
					BackoffLimit: numbers.Int32Ptr(4),
				},
			},
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)
	sut := s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	allJobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(allJobs.Items, 2)
	job1 := slice.FindAll(allJobs.Items, func(job batchv1.Job) bool { return job.GetName() == getKubeJobName(batchName, job1Name) })[0]
	s.Equal(numbers.Int32Ptr(4), job1.Spec.BackoffLimit)
	job2 := slice.FindAll(allJobs.Items, func(job batchv1.Job) bool { return job.GetName() == getKubeJobName(batchName, job2Name) })[0]
	s.Equal(numbers.Int32Ptr(5), job2.Spec.BackoffLimit)
}

func (s *syncerTestSuite) Test_BatchJobBackoffLimit_WithoutJobComponentDefault() {
	appName, batchName, componentName, namespace, rdName := "any-app", "any-batch", "compute", "any-ns", "any-rd"
	job1Name, job2Name := "job1", "job2"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  componentName,
			},
			Jobs: []radixv1.RadixBatchJob{
				{Name: job1Name},
				{Name: job2Name, BackoffLimit: numbers.Int32Ptr(5)},
			},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: appName,
			Jobs: []radixv1.RadixDeployJobComponent{
				{
					Name: componentName,
				},
			},
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)
	sut := s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	allJobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(allJobs.Items, 2)
	job1 := slice.FindAll(allJobs.Items, func(job batchv1.Job) bool { return job.GetName() == getKubeJobName(batchName, job1Name) })[0]
	s.Equal(numbers.Int32Ptr(0), job1.Spec.BackoffLimit)
	job2 := slice.FindAll(allJobs.Items, func(job batchv1.Job) bool { return job.GetName() == getKubeJobName(batchName, job2Name) })[0]
	s.Equal(numbers.Int32Ptr(5), job2.Spec.BackoffLimit)
}

func (s *syncerTestSuite) Test_JobWithIdentity() {
	appName, batchName, componentName, namespace, rdName := "any-app", "any-batch", "compute", "any-ns", "any-rd"
	jobName := "any-job"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  componentName,
			},
			Jobs: []radixv1.RadixBatchJob{{Name: jobName}},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: appName,
			Jobs: []radixv1.RadixDeployJobComponent{
				{
					Name:     componentName,
					Identity: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "a-client-id"}},
				},
			},
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)

	sut := s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	jobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(jobs.Items, 1)
	expectedPodLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: componentName, kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule, kube.RadixBatchNameLabel: batchName, kube.RadixBatchJobNameLabel: jobName, "azure.workload.identity/use": "true"}
	s.Equal(expectedPodLabels, jobs.Items[0].Spec.Template.Labels)
	s.Equal(utils.GetComponentServiceAccountName(componentName), jobs.Items[0].Spec.Template.Spec.ServiceAccountName)
	s.Equal(utils.BoolPtr(false), jobs.Items[0].Spec.Template.Spec.AutomountServiceAccountToken)
}

func (s *syncerTestSuite) Test_JobWithPayload() {
	appName, batchName, componentName, namespace, rdName, payloadPath, secretName, secretKey := "any-app", "any-job", "compute", "any-ns", "any-rd", "/mnt/path", "any-payload-secret", "any-payload-key"
	jobName := "any-job"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  componentName,
			},
			Jobs: []radixv1.RadixBatchJob{
				{
					Name: jobName,
					PayloadSecretRef: &radixv1.PayloadSecretKeySelector{
						LocalObjectReference: radixv1.LocalObjectReference{Name: secretName},
						Key:                  secretKey,
					},
				},
			},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: appName,
			Jobs: []radixv1.RadixDeployJobComponent{
				{
					Name:    componentName,
					Payload: &radixv1.RadixJobComponentPayload{Path: payloadPath},
				},
			},
		},
	}
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: radixlabels.Merge(radixlabels.ForJobScheduleJobType(),
				radixlabels.ForApplicationName(appName),
				radixlabels.ForComponentName(componentName), radixlabels.ForBatchName(batchName)),
		},
		Data: map[string][]byte{secretKey: []byte("any-payload")},
	}

	_, err := s.kubeClient.CoreV1().Secrets(namespace).Create(context.Background(), &secret, metav1.CreateOptions{})
	s.Require().NoError(err)
	batch, err = s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)

	sut := s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	jobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(jobs.Items, 1)
	s.Equal(getKubeJobName(batchName, jobName), jobs.Items[0].Name)
	s.Require().Len(jobs.Items[0].Spec.Template.Spec.Volumes, 1)
	s.Equal(corev1.Volume{
		Name: jobPayloadVolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
				Items:      []corev1.KeyToPath{{Key: secretKey, Path: "payload"}},
			},
		},
	}, jobs.Items[0].Spec.Template.Spec.Volumes[0])
	s.Len(jobs.Items[0].Spec.Template.Spec.Containers[0].VolumeMounts, 1)
	s.Equal(corev1.VolumeMount{Name: jobPayloadVolumeName, ReadOnly: true, MountPath: payloadPath}, jobs.Items[0].Spec.Template.Spec.Containers[0].VolumeMounts[0])
}

func (s *syncerTestSuite) Test_JobWithResources() {
	appName, batchName, componentName, namespace, rdName := "any-app", "any-job", "compute", "any-ns", "any-rd"
	job1Name, job2Name := "job1", "job2"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  componentName,
			},
			Jobs: []radixv1.RadixBatchJob{
				{Name: job1Name},
				{Name: job2Name, Resources: &radixv1.ResourceRequirements{
					Limits:   radixv1.ResourceList{"cpu": "700m", "memory": "701M"},
					Requests: radixv1.ResourceList{"cpu": "300m", "memory": "301M"},
				}},
			},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: appName,
			Jobs: []radixv1.RadixDeployJobComponent{
				{
					Name: componentName,
					Resources: radixv1.ResourceRequirements{
						Limits:   radixv1.ResourceList{"cpu": "800m", "memory": "801M"},
						Requests: radixv1.ResourceList{"cpu": "400m", "memory": "401M"},
					},
				},
			},
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)

	sut := s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	jobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(jobs.Items, 2)
	job1 := slice.FindAll(jobs.Items, func(job batchv1.Job) bool { return job.GetName() == getKubeJobName(batchName, job1Name) })[0]
	s.Equal(int64(800), job1.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().MilliValue())
	s.Equal(int64(400), job1.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().MilliValue())
	s.Equal(int64(801), job1.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().ScaledValue(resource.Mega))
	s.Equal(int64(401), job1.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().ScaledValue(resource.Mega))
	job2 := slice.FindAll(jobs.Items, func(job batchv1.Job) bool { return job.GetName() == getKubeJobName(batchName, job2Name) })[0]
	s.Equal(int64(700), job2.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().MilliValue())
	s.Equal(int64(300), job2.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().MilliValue())
	s.Equal(int64(701), job2.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().ScaledValue(resource.Mega))
	s.Equal(int64(301), job2.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().ScaledValue(resource.Mega))
}

func (s *syncerTestSuite) Test_JobWithVolumeMounts() {
	appName, batchName, componentName, namespace, rdName := "any-app", "any-job", "compute", "any-ns", "any-rd"
	jobName := "any-job"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  componentName,
			},
			Jobs: []radixv1.RadixBatchJob{{Name: jobName}},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: appName,
			Jobs: []radixv1.RadixDeployJobComponent{
				{
					Name: componentName,
					VolumeMounts: []radixv1.RadixVolumeMount{
						{Name: "azureblob2name", Path: "/azureblob2path", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Protocol: radixv1.BlobFuse2ProtocolFuse2, Container: "azureblob2container"}},
						{Name: "azurenfsname", Path: "/azurenfspath", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Protocol: radixv1.BlobFuse2ProtocolNfs, Container: "azurenfscontainer"}},
						{Name: "azurefilename", Path: "/azurefilepath", AzureFile: &radixv1.RadixAzureFileVolumeMount{Share: "azurefilecontainer"}},
					},
				},
			},
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)

	sut := s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	jobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(jobs.Items, 1)
	job := slice.FindAll(jobs.Items, func(job batchv1.Job) bool { return job.GetName() == getKubeJobName(batchName, jobName) })[0]
	s.Require().Len(job.Spec.Template.Spec.Volumes, 3)
	s.Require().Len(job.Spec.Template.Spec.Containers[0].VolumeMounts, 3)
	s.Equal(job.Spec.Template.Spec.Volumes[0].Name, job.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name)
	s.Equal(job.Spec.Template.Spec.Volumes[1].Name, job.Spec.Template.Spec.Containers[0].VolumeMounts[1].Name)
	s.Equal(job.Spec.Template.Spec.Volumes[2].Name, job.Spec.Template.Spec.Containers[0].VolumeMounts[2].Name)
	s.Equal("/azureblob2path", job.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath)
	s.Equal("/azurenfspath", job.Spec.Template.Spec.Containers[0].VolumeMounts[1].MountPath)
	s.Equal("/azurefilepath", job.Spec.Template.Spec.Containers[0].VolumeMounts[2].MountPath)
}

func (s *syncerTestSuite) Test_JobWithVolumeMounts_Deprecated() {
	appName, batchName, componentName, namespace, rdName := "any-app", "any-job", "compute", "any-ns", "any-rd"
	jobName := "any-job"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  componentName,
			},
			Jobs: []radixv1.RadixBatchJob{{Name: jobName}},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: appName,
			Jobs: []radixv1.RadixDeployJobComponent{
				{
					Name: componentName,
					VolumeMounts: []radixv1.RadixVolumeMount{
						{Type: "blob", Name: "blobname", Container: "blobcontainer", Path: "/blobpath"},
						{Type: "azure-blob", Name: "azureblobname", Storage: "azureblobcontainer", Path: "/azureblobpath"},
						{Type: "azure-file", Name: "azurefilename", Storage: "azurefilecontainer", Path: "/azurefilepath"},
					},
				},
			},
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)

	sut := s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	jobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(jobs.Items, 1)
	job := slice.FindAll(jobs.Items, func(job batchv1.Job) bool { return job.GetName() == getKubeJobName(batchName, jobName) })[0]
	s.Require().Len(job.Spec.Template.Spec.Volumes, 3)
	s.Require().Len(job.Spec.Template.Spec.Containers[0].VolumeMounts, 3)
	s.Equal(job.Spec.Template.Spec.Volumes[0].Name, job.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name)
	s.Equal(job.Spec.Template.Spec.Volumes[1].Name, job.Spec.Template.Spec.Containers[0].VolumeMounts[1].Name)
	s.Equal(job.Spec.Template.Spec.Volumes[2].Name, job.Spec.Template.Spec.Containers[0].VolumeMounts[2].Name)
	s.Equal("/blobpath", job.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath)
	s.Equal("/azureblobpath", job.Spec.Template.Spec.Containers[0].VolumeMounts[1].MountPath)
	s.Equal("/azurefilepath", job.Spec.Template.Spec.Containers[0].VolumeMounts[2].MountPath)
}

func (s *syncerTestSuite) Test_JobWithAzureSecretRefs() {
	appName, batchName, componentName, namespace, rdName := "any-app", "any-job", "compute", "any-app-dev", "any-rd"
	jobName := "any-job"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  componentName,
			},
			Jobs: []radixv1.RadixBatchJob{{Name: jobName}},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName:     appName,
			Environment: "dev",
			Jobs: []radixv1.RadixDeployJobComponent{
				{
					Name: componentName,
					SecretRefs: radixv1.RadixSecretRefs{
						AzureKeyVaults: []radixv1.RadixAzureKeyVault{
							{Name: "kv1", Path: utils.StringPtr("/mnt/kv1"), Items: []radixv1.RadixAzureKeyVaultItem{{Name: "secret", EnvVar: "SECRET1"}}},
							{Name: "kv2", Path: utils.StringPtr("/mnt/kv2"), Items: []radixv1.RadixAzureKeyVaultItem{{Name: "secret", EnvVar: "SECRET2"}}},
						},
					},
				},
			},
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	rd, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)
	deploySyncer := deployment.NewDeployment(s.kubeClient, s.kubeUtil, s.radixClient, s.promClient, utils.NewRegistrationBuilder().WithName(appName).BuildRR(), rd, "", 0, nil, nil)
	s.Require().NoError(deploySyncer.OnSync())

	sut := s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	jobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(jobs.Items, 1)
	job := slice.FindAll(jobs.Items, func(job batchv1.Job) bool { return job.GetName() == getKubeJobName(batchName, jobName) })[0]
	s.Require().Len(job.Spec.Template.Spec.Volumes, 2)
	s.Require().Len(job.Spec.Template.Spec.Containers[0].VolumeMounts, 2)
	s.True(slice.Any(job.Spec.Template.Spec.Containers[0].Env, func(env corev1.EnvVar) bool { return env.Name == "SECRET1" && env.ValueFrom.SecretKeyRef != nil }))
	s.True(slice.Any(job.Spec.Template.Spec.Containers[0].Env, func(env corev1.EnvVar) bool { return env.Name == "SECRET2" && env.ValueFrom.SecretKeyRef != nil }))
}

func (s *syncerTestSuite) Test_JobWithGpuNode() {
	appName, batchName, componentName, namespace, rdName := "any-app", "any-job", "compute", "any-ns", "any-rd"
	job1Name, job2Name := "job1", "job2"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  componentName,
			},
			Jobs: []radixv1.RadixBatchJob{
				{Name: job1Name},
				{Name: job2Name, Node: &radixv1.RadixNode{Gpu: "gpu3, gpu4", GpuCount: "8"}},
			},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: appName,
			Jobs: []radixv1.RadixDeployJobComponent{
				{
					Name: componentName,
					Node: radixv1.RadixNode{Gpu: "gpu1, gpu2", GpuCount: "4"},
				},
			},
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)

	sut := s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	jobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(jobs.Items, 2)

	job1 := slice.FindAll(jobs.Items, func(job batchv1.Job) bool { return job.GetName() == getKubeJobName(batchName, job1Name) })[0]
	job1NodeSelectorTerms := job1.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
	s.Len(job1NodeSelectorTerms, 2)
	s.Equal(corev1.NodeSelectorRequirement{
		Key:      kube.RadixGpuLabel,
		Operator: corev1.NodeSelectorOpIn,
		Values:   []string{"gpu1", "gpu2"},
	}, job1NodeSelectorTerms[0].MatchExpressions[0])
	s.Equal(corev1.NodeSelectorRequirement{
		Key:      kube.RadixGpuCountLabel,
		Operator: corev1.NodeSelectorOpGt,
		Values:   []string{"3"},
	}, job1NodeSelectorTerms[0].MatchExpressions[1])
	s.Equal(corev1.NodeSelectorRequirement{
		Key:      kube.RadixNodeScheduledJobLabel,
		Operator: corev1.NodeSelectorOpExists,
	}, job1NodeSelectorTerms[1].MatchExpressions[0])

	tolerations := job1.Spec.Template.Spec.Tolerations
	s.Len(tolerations, 2)
	s.Equal(corev1.Toleration{Key: kube.RadixGpuCountLabel, Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}, tolerations[0])
	s.Equal(corev1.Toleration{Key: kube.NodeTaintScheduledKey, Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}, tolerations[1])

	job2 := slice.FindAll(jobs.Items, func(job batchv1.Job) bool { return job.GetName() == getKubeJobName(batchName, job2Name) })[0]
	job2NodeSelectorTerms := job2.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
	s.Len(job2NodeSelectorTerms, 2)
	s.Equal(corev1.NodeSelectorRequirement{
		Key:      kube.RadixGpuLabel,
		Operator: corev1.NodeSelectorOpIn,
		Values:   []string{"gpu3", "gpu4"},
	}, job2NodeSelectorTerms[0].MatchExpressions[0])
	s.Equal(corev1.NodeSelectorRequirement{
		Key:      kube.RadixGpuCountLabel,
		Operator: corev1.NodeSelectorOpGt,
		Values:   []string{"7"},
	}, job2NodeSelectorTerms[0].MatchExpressions[1])
	s.Equal(corev1.NodeSelectorRequirement{
		Key:      kube.RadixNodeScheduledJobLabel,
		Operator: corev1.NodeSelectorOpExists,
	}, job2NodeSelectorTerms[1].MatchExpressions[0])
}

func (s *syncerTestSuite) Test_StopJob() {
	appName, batchName, componentName, namespace, rdName := "any-app", "any-batch", "compute", "any-ns", "any-rd"
	job1Name, job2Name := "job1", "job2"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  componentName,
			},
			Jobs: []radixv1.RadixBatchJob{
				{Name: job1Name},
				{Name: job2Name},
			},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: appName,
			Jobs: []radixv1.RadixDeployJobComponent{
				{
					Name: componentName,
				},
			},
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)
	sut := s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	allJobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(allJobs.Items, 2)

	batch.Spec.Jobs[0].Stop = utils.BoolPtr(true)
	sut = s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	allJobs, _ = s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(allJobs.Items, 1)
	s.Equal(getKubeJobName(batchName, job2Name), allJobs.Items[0].GetName())

}

func (s *syncerTestSuite) Test_SyncErrorWhenJobMissingInRadixDeployment() {
	appName, batchName, componentName, namespace, rdName, missingComponentName := "any-app", "any-batch", "compute", "any-ns", "any-rd", "incorrect-job"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  missingComponentName,
			},
			Jobs: []radixv1.RadixBatchJob{{Name: "any-job-name"}},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: appName,
			Jobs: []radixv1.RadixDeployJobComponent{
				{
					Name: componentName,
				},
			},
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)
	sut := s.createSyncer(batch)
	err = sut.OnSync()
	s.Equal(err, newReconcileRadixDeploymentJobSpecNotFoundError(rdName, missingComponentName))
	var target reconcileStatus
	s.ErrorAs(err, &target)
	s.Equal(radixv1.BatchConditionTypeWaiting, batch.Status.Condition.Type)
	s.Equal(invalidDeploymentReferenceReason, batch.Status.Condition.Reason)
	s.Equal(err.Error(), batch.Status.Condition.Message)
}

func (s *syncerTestSuite) Test_SyncErrorWhenRadixDeploymentDoesNotExist() {
	batchName, namespace, rdName, missingComponentName := "any-batch", "any-ns", "any-rd", "incorrect-job"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  missingComponentName,
			},
			Jobs: []radixv1.RadixBatchJob{{Name: "any-job-name"}},
		},
	}

	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	sut := s.createSyncer(batch)
	err = sut.OnSync()
	s.Equal(err, newReconcileRadixDeploymentNotFoundError(rdName))
	var target reconcileStatus
	s.ErrorAs(err, &target)
	s.Equal(radixv1.BatchConditionTypeWaiting, batch.Status.Condition.Type)
	s.Equal(invalidDeploymentReferenceReason, batch.Status.Condition.Reason)
	s.Equal(err.Error(), batch.Status.Condition.Message)
}

func (s *syncerTestSuite) Test_BatchStatusCondition() {
	batchName, namespace, rdName := "any-batch", "any-ns", "any-rd"
	job1Name, job2Name, job3Name := "job1", "job2", "job3"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  "any-job",
			},
			Jobs: []radixv1.RadixBatchJob{{Name: job1Name}, {Name: job2Name}, {Name: job3Name}},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: "any-app",
			Jobs: []radixv1.RadixDeployJobComponent{
				{
					Name: "any-job",
				},
			},
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)
	sut := s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	allJobs, err := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().NoError(err)
	s.Require().Len(allJobs.Items, 3)
	s.Require().ElementsMatch([]string{getKubeJobName(batchName, job1Name), getKubeJobName(batchName, job2Name), getKubeJobName(batchName, job3Name)}, slice.Map(allJobs.Items, func(job batchv1.Job) string { return job.GetName() }))

	// Initial condition is Waiting when all jobs are waiting
	s.Equal(radixv1.BatchConditionTypeWaiting, batch.Status.Condition.Type)
	s.Nil(batch.Status.Condition.ActiveTime)
	s.Nil(batch.Status.Condition.CompletionTime)

	// Set job1 status.active to 1 => batch condition is Running
	s.updateKubeJobStatus(getKubeJobName(batchName, job1Name), namespace)(func(status *batchv1.JobStatus) {
		status.Active = 1
	})
	sut = s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	s.Equal(radixv1.BatchConditionTypeActive, batch.Status.Condition.Type)
	s.NotNil(batch.Status.Condition.ActiveTime)
	s.Nil(batch.Status.Condition.CompletionTime)

	// Set job2 condition to failed => batch condition is Running
	s.updateKubeJobStatus(getKubeJobName(batchName, job2Name), namespace)(func(status *batchv1.JobStatus) {
		status.Conditions = []batchv1.JobCondition{
			{Type: batchv1.JobFailed, Status: corev1.ConditionTrue},
		}
	})
	sut = s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	s.Equal(radixv1.BatchConditionTypeActive, batch.Status.Condition.Type)
	s.NotNil(batch.Status.Condition.ActiveTime)
	s.Nil(batch.Status.Condition.CompletionTime)

	// Set job3 to stopped => batch condition is Running
	batch.Spec.Jobs[2].Stop = utils.BoolPtr(true)
	sut = s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	s.Equal(radixv1.BatchConditionTypeActive, batch.Status.Condition.Type)
	s.NotNil(batch.Status.Condition.ActiveTime)
	s.Nil(batch.Status.Condition.CompletionTime)

	// Set job1 condition to failed => batch condition is Completed
	s.updateKubeJobStatus(getKubeJobName(batchName, job1Name), namespace)(func(status *batchv1.JobStatus) {
		status.Active = 0
		status.Conditions = []batchv1.JobCondition{
			{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
		}
	})
	sut = s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	s.Equal(radixv1.BatchConditionTypeCompleted, batch.Status.Condition.Type)
	s.NotNil(batch.Status.Condition.ActiveTime)
	s.NotNil(batch.Status.Condition.CompletionTime)
}

func (s *syncerTestSuite) Test_BatchJobStatusWaitingToSucceeded() {
	batchName, namespace, rdName := "any-batch", "any-ns", "any-rd"
	jobName := "myjob"
	jobStartTime, jobCompletionTime := metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local)), metav1.NewTime(time.Date(2020, 1, 2, 0, 0, 0, 0, time.Local))
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  "any-job",
			},
			Jobs: []radixv1.RadixBatchJob{{Name: jobName}},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: "any-app",
			Jobs: []radixv1.RadixDeployJobComponent{
				{
					Name: "any-job",
				},
			},
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)
	sut := s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	allJobs, err := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().NoError(err)
	s.Require().Len(allJobs.Items, 1)
	s.Equal(getKubeJobName(batchName, jobName), allJobs.Items[0].GetName())

	// Initial phase is Waiting
	s.Require().Len(batch.Status.JobStatuses, 1)
	s.Equal(jobName, batch.Status.JobStatuses[0].Name)
	s.Equal(radixv1.BatchJobPhaseWaiting, batch.Status.JobStatuses[0].Phase)
	s.Equal(int32(0), batch.Status.JobStatuses[0].Failed)
	s.Empty(batch.Status.JobStatuses[0].Reason)
	s.Empty(batch.Status.JobStatuses[0].Message)
	s.Equal(&allJobs.Items[0].CreationTimestamp, batch.Status.JobStatuses[0].CreationTime)
	s.Nil(batch.Status.JobStatuses[0].StartTime)
	s.Nil(batch.Status.JobStatuses[0].EndTime)

	// Set job status.active to 1 => phase is Active
	s.updateKubeJobStatus(getKubeJobName(batchName, jobName), namespace)(func(status *batchv1.JobStatus) {
		status.Active = 1
		status.StartTime = &jobStartTime
	})
	sut = s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	s.Require().Len(batch.Status.JobStatuses, 1)
	s.Equal(jobName, batch.Status.JobStatuses[0].Name)
	s.Equal(radixv1.BatchJobPhaseActive, batch.Status.JobStatuses[0].Phase)
	s.Equal(int32(0), batch.Status.JobStatuses[0].Failed)
	s.Empty(batch.Status.JobStatuses[0].Reason)
	s.Empty(batch.Status.JobStatuses[0].Message)
	s.Equal(&allJobs.Items[0].CreationTimestamp, batch.Status.JobStatuses[0].CreationTime)
	s.Equal(&jobStartTime, batch.Status.JobStatuses[0].StartTime)
	s.Nil(batch.Status.JobStatuses[0].EndTime)

	// Set job status.failed to 2
	s.updateKubeJobStatus(getKubeJobName(batchName, jobName), namespace)(func(status *batchv1.JobStatus) {
		status.Failed = 2
		status.StartTime = &jobStartTime
	})
	sut = s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	s.Require().Len(batch.Status.JobStatuses, 1)
	s.Equal(jobName, batch.Status.JobStatuses[0].Name)
	s.Equal(radixv1.BatchJobPhaseActive, batch.Status.JobStatuses[0].Phase)
	s.Equal(int32(2), batch.Status.JobStatuses[0].Failed)
	s.Empty(batch.Status.JobStatuses[0].Reason)
	s.Empty(batch.Status.JobStatuses[0].Message)
	s.Equal(&allJobs.Items[0].CreationTimestamp, batch.Status.JobStatuses[0].CreationTime)
	s.Equal(&jobStartTime, batch.Status.JobStatuses[0].StartTime)
	s.Nil(batch.Status.JobStatuses[0].EndTime)

	// Set job status.conditions to complete => phase is Succeeded
	s.updateKubeJobStatus(getKubeJobName(batchName, jobName), namespace)(func(status *batchv1.JobStatus) {
		status.Conditions = []batchv1.JobCondition{
			{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
		}
		status.StartTime = &jobStartTime
		status.CompletionTime = &jobCompletionTime
	})
	sut = s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	s.Require().Len(batch.Status.JobStatuses, 1)
	s.Equal(jobName, batch.Status.JobStatuses[0].Name)
	s.Equal(radixv1.BatchJobPhaseSucceeded, batch.Status.JobStatuses[0].Phase)
	s.Equal(int32(2), batch.Status.JobStatuses[0].Failed)
	s.Empty(batch.Status.JobStatuses[0].Reason)
	s.Empty(batch.Status.JobStatuses[0].Message)
	s.Equal(&allJobs.Items[0].CreationTimestamp, batch.Status.JobStatuses[0].CreationTime)
	s.Equal(&jobStartTime, batch.Status.JobStatuses[0].StartTime)
	s.Equal(&jobCompletionTime, batch.Status.JobStatuses[0].EndTime)
}

func (s *syncerTestSuite) Test_BatchJobStatusWaitingToFailed() {
	batchName, namespace, rdName := "any-batch", "any-ns", "any-rd"
	jobName := "myjob"
	jobStartTime, jobFailedTime := metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local)), metav1.NewTime(time.Date(2020, 1, 2, 0, 0, 0, 0, time.Local))
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  "any-job",
			},
			Jobs: []radixv1.RadixBatchJob{{Name: jobName}},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: "any-app",
			Jobs: []radixv1.RadixDeployJobComponent{
				{
					Name: "any-job",
				},
			},
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)
	sut := s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	allJobs, err := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().NoError(err)
	s.Require().Len(allJobs.Items, 1)
	s.Equal(getKubeJobName(batchName, jobName), allJobs.Items[0].GetName())

	// Initial phase is Waiting
	s.Require().Len(batch.Status.JobStatuses, 1)
	s.Equal(jobName, batch.Status.JobStatuses[0].Name)
	s.Equal(radixv1.BatchJobPhaseWaiting, batch.Status.JobStatuses[0].Phase)
	s.Empty(batch.Status.JobStatuses[0].Reason)
	s.Empty(batch.Status.JobStatuses[0].Message)
	s.Equal(&allJobs.Items[0].CreationTimestamp, batch.Status.JobStatuses[0].CreationTime)
	s.Nil(batch.Status.JobStatuses[0].StartTime)
	s.Nil(batch.Status.JobStatuses[0].EndTime)

	// Set job status.active to 1 => phase is Active
	s.updateKubeJobStatus(getKubeJobName(batchName, jobName), namespace)(func(status *batchv1.JobStatus) {
		status.Active = 1
		status.StartTime = &jobStartTime
	})
	sut = s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	s.Require().Len(batch.Status.JobStatuses, 1)
	s.Equal(jobName, batch.Status.JobStatuses[0].Name)
	s.Equal(radixv1.BatchJobPhaseActive, batch.Status.JobStatuses[0].Phase)
	s.Empty(batch.Status.JobStatuses[0].Reason)
	s.Empty(batch.Status.JobStatuses[0].Message)
	s.Equal(&allJobs.Items[0].CreationTimestamp, batch.Status.JobStatuses[0].CreationTime)
	s.Equal(&jobStartTime, batch.Status.JobStatuses[0].StartTime)
	s.Nil(batch.Status.JobStatuses[0].EndTime)

	// Set job status.conditions to failed => phase is Failed
	s.updateKubeJobStatus(getKubeJobName(batchName, jobName), namespace)(func(status *batchv1.JobStatus) {
		status.Conditions = []batchv1.JobCondition{
			{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, LastTransitionTime: jobFailedTime},
		}
		status.StartTime = &jobStartTime
	})
	sut = s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	s.Require().Len(batch.Status.JobStatuses, 1)
	s.Equal(jobName, batch.Status.JobStatuses[0].Name)
	s.Equal(radixv1.BatchJobPhaseFailed, batch.Status.JobStatuses[0].Phase)
	s.Empty(batch.Status.JobStatuses[0].Reason)
	s.Empty(batch.Status.JobStatuses[0].Message)
	s.Equal(&allJobs.Items[0].CreationTimestamp, batch.Status.JobStatuses[0].CreationTime)
	s.Equal(&jobStartTime, batch.Status.JobStatuses[0].StartTime)
	s.Equal(&jobFailedTime, batch.Status.JobStatuses[0].EndTime)
}

func (s *syncerTestSuite) Test_BatchJobStatusWaitingToStopped() {
	batchName, namespace, rdName := "any-batch", "any-ns", "any-rd"
	jobName := "myjob"
	jobStartTime := metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local))
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  "any-job",
			},
			Jobs: []radixv1.RadixBatchJob{{Name: jobName}},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: "any-app",
			Jobs: []radixv1.RadixDeployJobComponent{
				{
					Name: "any-job",
				},
			},
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)
	sut := s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	allJobs, err := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().NoError(err)
	s.Require().Len(allJobs.Items, 1)
	s.Equal(getKubeJobName(batchName, jobName), allJobs.Items[0].GetName())

	// Initial phase is Waiting
	s.Require().Len(batch.Status.JobStatuses, 1)
	s.Equal(jobName, batch.Status.JobStatuses[0].Name)
	s.Equal(radixv1.BatchJobPhaseWaiting, batch.Status.JobStatuses[0].Phase)
	s.Empty(batch.Status.JobStatuses[0].Reason)
	s.Empty(batch.Status.JobStatuses[0].Message)
	s.Equal(&allJobs.Items[0].CreationTimestamp, batch.Status.JobStatuses[0].CreationTime)
	s.Nil(batch.Status.JobStatuses[0].StartTime)
	s.Nil(batch.Status.JobStatuses[0].EndTime)

	// Set job status.active to 1 => phase is Active
	s.updateKubeJobStatus(getKubeJobName(batchName, jobName), namespace)(func(status *batchv1.JobStatus) {
		status.Active = 1
		status.StartTime = &jobStartTime
	})
	sut = s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	s.Require().Len(batch.Status.JobStatuses, 1)
	s.Equal(jobName, batch.Status.JobStatuses[0].Name)
	s.Equal(radixv1.BatchJobPhaseActive, batch.Status.JobStatuses[0].Phase)
	s.Empty(batch.Status.JobStatuses[0].Reason)
	s.Empty(batch.Status.JobStatuses[0].Message)
	s.Equal(&allJobs.Items[0].CreationTimestamp, batch.Status.JobStatuses[0].CreationTime)
	s.Equal(&jobStartTime, batch.Status.JobStatuses[0].StartTime)
	s.Nil(batch.Status.JobStatuses[0].EndTime)

	// Set job status.conditions to failed => phase is Failed
	batch.Spec.Jobs[0].Stop = utils.BoolPtr(true)
	sut = s.createSyncer(batch)
	s.Require().NoError(sut.OnSync())
	s.Require().Len(batch.Status.JobStatuses, 1)
	s.Equal(jobName, batch.Status.JobStatuses[0].Name)
	s.Equal(radixv1.BatchJobPhaseStopped, batch.Status.JobStatuses[0].Phase)
	s.Empty(batch.Status.JobStatuses[0].Reason)
	s.Empty(batch.Status.JobStatuses[0].Message)
	s.Equal(&allJobs.Items[0].CreationTimestamp, batch.Status.JobStatuses[0].CreationTime)
	s.Equal(&jobStartTime, batch.Status.JobStatuses[0].StartTime)
	s.NotNil(batch.Status.JobStatuses[0].EndTime)
}

func (s *syncerTestSuite) updateKubeJobStatus(jobName, namespace string) func(updater func(status *batchv1.JobStatus)) {
	job, err := s.kubeClient.BatchV1().Jobs(namespace).Get(context.Background(), jobName, metav1.GetOptions{})
	if err != nil {
		s.FailNow(err.Error())
	}
	return func(updater func(status *batchv1.JobStatus)) {
		updater(&job.Status)
		_, err := s.kubeClient.BatchV1().Jobs(namespace).Update(context.Background(), job, metav1.UpdateOptions{})
		if err != nil {
			s.FailNow(err.Error())
		}
	}
}

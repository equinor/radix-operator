package batch

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	certfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	commonutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/numbers"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/config/containerregistry"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/securitycontext"
	_ "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	certClient  *certfake.Clientset
	kedaClient  *kedafake.Clientset
}

func TestSyncerTestSuite(t *testing.T) {
	suite.Run(t, new(syncerTestSuite))
}

func (s *syncerTestSuite) createSyncer(forJob *radixv1.RadixBatch, config *config.Config, options ...SyncerOption) Syncer {
	defaultRR := utils.ARadixRegistration().BuildRR()

	return NewSyncer(s.kubeClient, s.kubeUtil, s.radixClient, defaultRR, forJob, config, options...)
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
	initialEnvVarsConfigMap, _, _ := kubeUtil.GetOrCreateEnvVarsConfigMapAndMetadataMap(context.Background(), rd.GetNamespace(),
		rd.Spec.AppName, deployComponent.GetName())
	desiredConfigMap := initialEnvVarsConfigMap.DeepCopy()
	for envVarName, envVarValue := range deployComponent.GetEnvironmentVariables() {
		if strings.HasPrefix(envVarName, "RADIX_") {
			continue
		}
		desiredConfigMap.Data[envVarName] = envVarValue
	}
	err := kubeUtil.ApplyConfigMap(context.Background(), rd.GetNamespace(), initialEnvVarsConfigMap, desiredConfigMap)
	s.Require().NoError(err)

	return desiredConfigMap
}

func (s *syncerTestSuite) SetupTest() {
	s.setupTest()
}

func (s *syncerTestSuite) SetupSubTest() {
	s.setupTest()
}

func (s *syncerTestSuite) setupTest() {
	s.kubeClient = fake.NewSimpleClientset()
	s.radixClient = fakeradix.NewSimpleClientset()
	s.kedaClient = kedafake.NewSimpleClientset()
	s.promClient = prometheusfake.NewSimpleClientset()
	s.certClient = certfake.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, s.kedaClient, secretproviderfake.NewSimpleClientset())
	s.T().Setenv(defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable, "1500Mi")
	s.T().Setenv(defaults.OperatorRollingUpdateMaxUnavailable, "25%")
	s.T().Setenv(defaults.OperatorRollingUpdateMaxSurge, "25%")
	s.T().Setenv(defaults.OperatorDefaultUserGroupEnvironmentVariable, "any-group")
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
	sut := s.createSyncer(batch, nil)
	s.Require().NoError(sut.OnSync(context.Background()))
	batch, err = s.radixClient.RadixV1().RadixBatches(namespace).Get(context.Background(), jobName, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal(expectedStatus, batch.Status)
}

func (s *syncerTestSuite) Test_RestoreStatusWithInvalidAnnotationValueShouldReturnErrorAndSkipReconcile() {
	jobName := "any-job"
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
	sut := s.createSyncer(batch, nil)
	err = sut.OnSync(context.Background())
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
	sut := s.createSyncer(job, nil)
	s.Require().NoError(sut.OnSync(context.Background()))
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
	sut := s.createSyncer(job, nil)
	s.Require().NoError(sut.OnSync(context.Background()))
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
			sut := s.createSyncer(batch, nil)
			s.Require().NoError(sut.OnSync(context.Background()))
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
		ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
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

	sut := s.createSyncer(batch, nil)
	s.Require().NoError(sut.OnSync(context.Background()))
	allServices, _ := s.kubeClient.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{})
	s.Len(allServices.Items, 2)
	for _, jobName := range []string{job1Name, job2Name} {
		jobServices := slice.FindAll(allServices.Items, func(svc corev1.Service) bool { return svc.Name == getKubeServiceName(batchName, jobName) })
		s.Len(jobServices, 1)
		service := jobServices[0]
		expectedServiceLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixAppIDLabel: "00000000000000000000000001", kube.RadixComponentLabel: componentName, kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule, kube.RadixBatchNameLabel: batchName, kube.RadixBatchJobNameLabel: jobName}
		s.Equal(expectedServiceLabels, service.Labels, "service labels")
		s.Equal(ownerReference(batch), service.OwnerReferences)
		expectedSelectorLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixAppIDLabel: "00000000000000000000000001", kube.RadixComponentLabel: componentName, kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule, kube.RadixBatchNameLabel: batchName, kube.RadixBatchJobNameLabel: jobName}
		s.Equal(expectedSelectorLabels, service.Spec.Selector, "selector")
		s.ElementsMatch([]corev1.ServicePort{{Name: "port1", Port: 8000, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(8000)}, {Name: "port2", Port: 9000, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(9000)}}, service.Spec.Ports)
	}
}

func (s *syncerTestSuite) Test_ServiceNotCreatedWhenPortsIsEmpty() {
	appName, batchName, componentName, namespace, rdName := "any-app", "any-batch", "compute", "any-ns", "any-rd"
	job1Name := "job1"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
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

	sut := s.createSyncer(batch, nil)
	s.Require().NoError(sut.OnSync(context.Background()))
	allServices, _ := s.kubeClient.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{})
	s.Len(allServices.Items, 0)
}

func (s *syncerTestSuite) Test_ServiceNotCreatedForJobWithPhaseDone() {
	appName, batchName, componentName, namespace, rdName := "any-app", "any-batch", "compute", "any-ns", "any-rd"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
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

	sut := s.createSyncer(batch, nil)
	s.Require().NoError(sut.OnSync(context.Background()))
	allServices, _ := s.kubeClient.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{})
	s.ElementsMatch([]string{getKubeServiceName(batchName, "job4"), getKubeServiceName(batchName, "job5")}, slice.Map(allServices.Items, func(svc corev1.Service) string { return svc.GetName() }))
}

func (s *syncerTestSuite) Test_BatchStaticConfiguration() {
	appName, batchName, componentName, namespace, rdName, imageName := "any-app", "any-batch", "compute", "any-ns", "any-rd", "any-image"
	job1Name, job2Name := "job1", "job2"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
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

	sut := s.createSyncer(batch, nil)
	s.Require().NoError(sut.OnSync(context.Background()))

	allJobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Len(allJobs.Items, 2)
	for _, jobName := range []string{job1Name, job2Name} {
		jobKubeJobs := slice.FindAll(allJobs.Items, func(job batchv1.Job) bool { return job.Name == getKubeJobName(batchName, jobName) })
		s.Len(jobKubeJobs, 1)
		kubejob := jobKubeJobs[0]
		expectedJobLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixAppIDLabel: "00000000000000000000000001", kube.RadixComponentLabel: componentName, kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule, kube.RadixBatchNameLabel: batchName, kube.RadixBatchJobNameLabel: jobName}
		s.Equal(expectedJobLabels, kubejob.Labels, "job labels")
		expectedPodLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixAppIDLabel: "00000000000000000000000001", kube.RadixComponentLabel: componentName, kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule, kube.RadixBatchNameLabel: batchName, kube.RadixBatchJobNameLabel: jobName}
		s.Equal(expectedPodLabels, kubejob.Spec.Template.Labels, "pod labels")
		expectedPodAnnotations := map[string]string{"cluster-autoscaler.kubernetes.io/safe-to-evict": "false"}
		s.Equal(expectedPodAnnotations, kubejob.Spec.Template.Annotations)
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
		s.Equal(pointers.Ptr(false), kubejob.Spec.Template.Spec.AutomountServiceAccountToken)
		expectedAffinity := &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{
			{Key: kube.RadixJobNodeLabel, Operator: corev1.NodeSelectorOpExists},
			{Key: corev1.LabelOSStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorOS}},
			{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorArchitecture}},
		}}}}}}
		s.Equal(expectedAffinity, kubejob.Spec.Template.Spec.Affinity)
		s.Len(kubejob.Spec.Template.Spec.Tolerations, 1)
		expectedTolerations := []corev1.Toleration{{Key: kube.NodeTaintJobsKey, Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}}
		s.ElementsMatch(expectedTolerations, kubejob.Spec.Template.Spec.Tolerations)
		s.Len(kubejob.Spec.Template.Spec.Volumes, 0)
		s.Len(kubejob.Spec.Template.Spec.Containers[0].VolumeMounts, 0)
		services, err := s.kubeClient.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{})
		s.Require().NoError(err)
		s.Len(services.Items, 0)
	}
}

func (s *syncerTestSuite) Test_Batch_AffinityFromRuntime() {
	appName, batchName, componentName, namespace, rdName, imageName := "any-app", "any-batch", "compute", "any-ns", "any-rd", "any-image"
	jobName, runtimeArch := "job1", "customarch"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
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
					Name:                 componentName,
					Image:                imageName,
					EnvironmentVariables: radixv1.EnvVarsMap{"VAR1": "any-val", "VAR2": "any-val"},
					Secrets:              []string{"SECRET1", "SECRET2"},
					Runtime: &radixv1.Runtime{
						Architecture: radixv1.RuntimeArchitecture(runtimeArch),
					},
				},
			},
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)

	sut := s.createSyncer(batch, nil)
	s.Require().NoError(sut.OnSync(context.Background()))

	allJobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(allJobs.Items, 1)
	kubejob := allJobs.Items[0]
	expectedAffinity := &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{
		{Key: kube.RadixJobNodeLabel, Operator: corev1.NodeSelectorOpExists},
		{Key: corev1.LabelOSStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorOS}},
		{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{runtimeArch}},
	}}}}}}
	s.Equal(expectedAffinity, kubejob.Spec.Template.Spec.Affinity, "affinity should use arch from runtime")
	s.Len(kubejob.Spec.Template.Spec.Tolerations, 1)
	expectedTolerations := []corev1.Toleration{{Key: kube.NodeTaintJobsKey, Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}}
	s.ElementsMatch(expectedTolerations, kubejob.Spec.Template.Spec.Tolerations)
}

func (s *syncerTestSuite) Test_Batch_ImagePullSecrets() {
	tests := map[string]struct {
		rdImagePullSecrets        []corev1.LocalObjectReference
		defaultRegistryAuthSecret string
		expectedImagePullSecrets  []corev1.LocalObjectReference
	}{
		"none defined => empty": {
			rdImagePullSecrets:        nil,
			defaultRegistryAuthSecret: "",
			expectedImagePullSecrets:  nil,
		},
		"rd defined": {
			rdImagePullSecrets:        []corev1.LocalObjectReference{{Name: "secret1"}, {Name: "secret2"}},
			defaultRegistryAuthSecret: "",
			expectedImagePullSecrets:  []corev1.LocalObjectReference{{Name: "secret1"}, {Name: "secret2"}},
		},
		"default defined": {
			rdImagePullSecrets:        nil,
			defaultRegistryAuthSecret: "default-auth",
			expectedImagePullSecrets:  []corev1.LocalObjectReference{{Name: "default-auth"}},
		},
		"both defined": {
			rdImagePullSecrets:        []corev1.LocalObjectReference{{Name: "secret1"}, {Name: "secret2"}},
			defaultRegistryAuthSecret: "default-auth",
			expectedImagePullSecrets:  []corev1.LocalObjectReference{{Name: "secret1"}, {Name: "secret2"}, {Name: "default-auth"}},
		},
	}

	for testName, test := range tests {
		s.Run(testName, func() {
			appName, envName, batchName, jobComponentName, rdName, jobName := "any-app", "any-env", "any-batch", "compute", "any-rd", "any-jobname"
			namespace := utils.GetEnvironmentNamespace(appName, envName)
			batch := &radixv1.RadixBatch{
				ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
				Spec: radixv1.RadixBatchSpec{
					RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
						LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
						Job:                  jobComponentName,
					},
					Jobs: []radixv1.RadixBatchJob{
						{Name: jobName},
					},
				},
			}

			rd := utils.NewDeploymentBuilder().
				WithAppName(appName).
				WithEnvironment(envName).
				WithDeploymentName(rdName).
				WithImagePullSecrets(test.rdImagePullSecrets).
				WithJobComponents(
					utils.NewDeployJobComponentBuilder().WithName(jobComponentName),
				).BuildRD()
			batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
			s.Require().NoError(err)
			_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
			s.Require().NoError(err)

			cfg := &config.Config{ContainerRegistryConfig: containerregistry.Config{ExternalRegistryAuthSecret: test.defaultRegistryAuthSecret}}
			sut := s.createSyncer(batch, cfg)
			s.Require().NoError(sut.OnSync(context.Background()))
			allJobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
			s.Require().Len(allJobs.Items, 1)
			kubejob := allJobs.Items[0]
			s.ElementsMatch(test.expectedImagePullSecrets, kubejob.Spec.Template.Spec.ImagePullSecrets)
		})
	}
}

func (s *syncerTestSuite) Test_JobNotCreatedForJobWithPhaseDone() {
	appName, batchName, componentName, namespace, rdName := "any-app", "any-batch", "compute", "any-ns", "any-rd"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
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

	sut := s.createSyncer(batch, nil)
	s.Require().NoError(sut.OnSync(context.Background()))
	allJobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.ElementsMatch([]string{getKubeJobName(batchName, "job4"), getKubeJobName(batchName, "job5")}, slice.Map(allJobs.Items, func(job batchv1.Job) string { return job.GetName() }))
}

func (s *syncerTestSuite) Test_BatchJobTimeLimitSeconds() {
	appName, batchName, componentName, namespace, rdName := "any-app", "any-batch", "compute", "any-ns", "any-rd"
	job1Name, job2Name := "job1", "job2"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
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
	sut := s.createSyncer(batch, nil)
	s.Require().NoError(sut.OnSync(context.Background()))
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
		ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
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
	sut := s.createSyncer(batch, nil)
	s.Require().NoError(sut.OnSync(context.Background()))
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
		ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
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
	sut := s.createSyncer(batch, nil)
	s.Require().NoError(sut.OnSync(context.Background()))
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
		ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
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

	sut := s.createSyncer(batch, nil)
	s.Require().NoError(sut.OnSync(context.Background()))
	jobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(jobs.Items, 1)
	expectedPodLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixAppIDLabel: "00000000000000000000000001", kube.RadixComponentLabel: componentName, kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule, kube.RadixBatchNameLabel: batchName, kube.RadixBatchJobNameLabel: jobName, "azure.workload.identity/use": "true"}
	s.Equal(expectedPodLabels, jobs.Items[0].Spec.Template.Labels)
	s.Equal(utils.GetComponentServiceAccountName(componentName), jobs.Items[0].Spec.Template.Spec.ServiceAccountName)
	s.Equal(pointers.Ptr(false), jobs.Items[0].Spec.Template.Spec.AutomountServiceAccountToken)
}

func (s *syncerTestSuite) Test_JobWithPayload() {
	appName, batchName, componentName, namespace, rdName, payloadPath, secretName, secretKey := "any-app", "any-job", "compute", "any-ns", "any-rd", "/mnt/path", "any-payload-secret", "any-payload-key"
	jobName := "any-job"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
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

	sut := s.createSyncer(batch, nil)
	s.Require().NoError(sut.OnSync(context.Background()))
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

func (s *syncerTestSuite) Test_ReadOnlyFileSystem() {
	appName, batchName, namespace, rdName := "any-app", "any-job", "any-ns", "any-rd"
	type scenarioSpec struct {
		readOnlyFileSystem         *bool
		expectedReadOnlyFileSystem *bool
	}
	tests := map[string]scenarioSpec{
		"notSet": {readOnlyFileSystem: nil, expectedReadOnlyFileSystem: nil},
		"false":  {readOnlyFileSystem: pointers.Ptr(false), expectedReadOnlyFileSystem: pointers.Ptr(false)},
		"true":   {readOnlyFileSystem: pointers.Ptr(true), expectedReadOnlyFileSystem: pointers.Ptr(true)},
	}
	for name, test := range tests {
		s.Run(name, func() {
			batch := &radixv1.RadixBatch{
				ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
				Spec: radixv1.RadixBatchSpec{
					RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
						LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
						Job:                  "anyjob",
					},
					Jobs: []radixv1.RadixBatchJob{
						{Name: "any"},
					},
				},
			}

			rd := &radixv1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: rdName},
				Spec: radixv1.RadixDeploymentSpec{
					AppName: appName,
					Jobs: []radixv1.RadixDeployJobComponent{
						{
							Name:               "anyjob",
							ReadOnlyFileSystem: test.readOnlyFileSystem,
						},
					},
				},
			}
			batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
			s.Require().NoError(err)
			_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
			s.Require().NoError(err)

			sut := s.createSyncer(batch, nil)
			s.Require().NoError(sut.OnSync(context.Background()))
			jobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
			s.Require().Len(jobs.Items, 1)
			job1 := jobs.Items[0]
			s.Equal(test.expectedReadOnlyFileSystem, job1.Spec.Template.Spec.Containers[0].SecurityContext.ReadOnlyRootFilesystem)
		})
	}
}

func (s *syncerTestSuite) Test_JobWithResources() {
	appName, batchName, componentName, namespace, rdName := "any-app", "any-job", "compute", "any-ns", "any-rd"
	job1Name, job2Name := "job1", "job2"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
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

	sut := s.createSyncer(batch, nil)
	s.Require().NoError(sut.OnSync(context.Background()))
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
		ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
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
					},
				},
			},
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)

	sut := s.createSyncer(batch, nil)
	s.Require().NoError(sut.OnSync(context.Background()))
	jobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(jobs.Items, 1)
	job := slice.FindAll(jobs.Items, func(job batchv1.Job) bool { return job.GetName() == getKubeJobName(batchName, jobName) })[0]
	s.Require().Len(job.Spec.Template.Spec.Volumes, 1)
	s.Require().Len(job.Spec.Template.Spec.Containers[0].VolumeMounts, 1)
	s.Equal(job.Spec.Template.Spec.Volumes[0].Name, job.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name)
	s.Equal("/azureblob2path", job.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath)
}

func (s *syncerTestSuite) Test_JobWithVolumeMounts_Deprecated() {
	appName, batchName, componentName, namespace, rdName := "any-app", "any-job", "compute", "any-ns", "any-rd"
	jobName := "any-job"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
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
						{Type: "azure-blob", Name: "azureblobname", Storage: "azureblobcontainer", Path: "/azureblobpath"},
					},
				},
			},
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)

	sut := s.createSyncer(batch, nil)
	s.Require().NoError(sut.OnSync(context.Background()))
	jobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(jobs.Items, 1)
	job := slice.FindAll(jobs.Items, func(job batchv1.Job) bool { return job.GetName() == getKubeJobName(batchName, jobName) })[0]
	s.Require().Len(job.Spec.Template.Spec.Volumes, 1)
	s.Require().Len(job.Spec.Template.Spec.Containers[0].VolumeMounts, 1)
	s.Equal(job.Spec.Template.Spec.Volumes[0].Name, job.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name)
	s.Equal("/azureblobpath", job.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath)
}

func (s *syncerTestSuite) Test_JobWithAzureSecretRefs() {
	appName, batchName, componentName, namespace, rdName := "any-app", "any-job", "compute", "any-app-dev", "any-rd"
	jobName := "any-job"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
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
	deploySyncer := deployment.NewDeploymentSyncer(s.kubeClient, s.kubeUtil, s.radixClient, s.promClient, s.certClient, utils.NewRegistrationBuilder().WithName(appName).BuildRR(), rd, nil, nil, &config.Config{})
	s.Require().NoError(deploySyncer.OnSync(context.Background()))

	sut := s.createSyncer(batch, nil)
	s.Require().NoError(sut.OnSync(context.Background()))
	jobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(jobs.Items, 1)
	job := slice.FindAll(jobs.Items, func(job batchv1.Job) bool { return job.GetName() == getKubeJobName(batchName, jobName) })[0]
	s.Require().Len(job.Spec.Template.Spec.Volumes, 2)
	s.Require().Len(job.Spec.Template.Spec.Containers[0].VolumeMounts, 2)
	s.True(slice.Any(job.Spec.Template.Spec.Containers[0].Env, func(env corev1.EnvVar) bool { return env.Name == "SECRET1" && env.ValueFrom.SecretKeyRef != nil }))
	s.True(slice.Any(job.Spec.Template.Spec.Containers[0].Env, func(env corev1.EnvVar) bool { return env.Name == "SECRET2" && env.ValueFrom.SecretKeyRef != nil }))
}

func (s *syncerTestSuite) Test_JobWithGpuNode() {
	appName, batchName, jobComponentName, namespace, rdName := "any-app", "any-job", "compute", "any-ns", "any-rd"
	job1Name, job2Name := "job1", "job2"
	arch := "customarch"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  jobComponentName,
			},
			Jobs: []radixv1.RadixBatchJob{
				{Name: job1Name, Node: &radixv1.RadixNode{}},
				{Name: job2Name, Node: &radixv1.RadixNode{Gpu: "gpu3 , gpu4 ", GpuCount: "8"}},
			},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: appName,
			Jobs: []radixv1.RadixDeployJobComponent{
				{
					Name: jobComponentName,
					Node: radixv1.RadixNode{Gpu: " gpu1, gpu2", GpuCount: "4"},
					Runtime: &radixv1.Runtime{
						Architecture: radixv1.RuntimeArchitecture(arch),
					},
				},
			},
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)

	sut := s.createSyncer(batch, nil)
	s.Require().NoError(sut.OnSync(context.Background()))
	jobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(jobs.Items, 2)

	job1 := slice.FindAll(jobs.Items, func(job batchv1.Job) bool { return job.GetName() == getKubeJobName(batchName, job1Name) })[0]
	expectedAffinity := &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{
		{Key: kube.RadixJobNodeLabel, Operator: corev1.NodeSelectorOpExists},
		{Key: corev1.LabelOSStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorOS}},
		{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{arch}},
	}}}}}}
	s.Equal(expectedAffinity, job1.Spec.Template.Spec.Affinity)

	tolerations := job1.Spec.Template.Spec.Tolerations
	s.Require().Len(tolerations, 1)
	s.Equal(corev1.Toleration{Key: kube.NodeTaintJobsKey, Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}, tolerations[0])

	job2 := slice.FindAll(jobs.Items, func(job batchv1.Job) bool { return job.GetName() == getKubeJobName(batchName, job2Name) })[0]
	expectedAffinity = &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{
		{Key: kube.RadixGpuCountLabel, Operator: corev1.NodeSelectorOpGt, Values: []string{"7"}},
		{Key: kube.RadixGpuLabel, Operator: corev1.NodeSelectorOpIn, Values: []string{"gpu3", "gpu4"}},
	}}}}}}
	s.Equal(expectedAffinity, job2.Spec.Template.Spec.Affinity, "job with gpu should ignore runtime architecture")
	tolerations = job2.Spec.Template.Spec.Tolerations
	s.Require().Len(tolerations, 1)
	s.Equal(corev1.Toleration{Key: kube.RadixGpuCountLabel, Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}, tolerations[0])
}

func (s *syncerTestSuite) Test_StopJob() {
	appName, batchName, componentName, namespace, rdName := "any-app", "any-batch", "compute", "any-ns", "any-rd"
	job1Name, job2Name := "job1", "job2"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
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

	// Run initial sync to ensure k8s jobs are created
	sut := s.createSyncer(batch, nil)
	s.Require().NoError(sut.OnSync(context.Background()))
	allJobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(allJobs.Items, 2)

	// Stop first job and check that k8s job is deleted
	batch.Spec.Jobs[0].Stop = pointers.Ptr(true)
	sut = s.createSyncer(batch, nil)
	s.Require().NoError(sut.OnSync(context.Background()))
	allJobs, _ = s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(allJobs.Items, 1)
	s.Equal(getKubeJobName(batchName, job2Name), allJobs.Items[0].GetName())

}

func (s *syncerTestSuite) Test_SyncErrorWhenJobMissingInRadixDeployment() {
	appName, batchName, componentName, namespace, rdName, missingComponentName := "any-app", "any-batch", "compute", "any-ns", "any-rd", "incorrect-job"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
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
	sut := s.createSyncer(batch, nil)
	err = sut.OnSync(context.Background())
	s.Equal(err, newReconcileRadixDeploymentJobSpecNotFoundError(rdName, missingComponentName))
	var target reconcileStatus
	s.ErrorAs(err, &target)
	batch, _ = s.radixClient.RadixV1().RadixBatches(namespace).Get(context.Background(), batch.GetName(), metav1.GetOptions{})
	s.Equal(radixv1.BatchConditionTypeWaiting, batch.Status.Condition.Type)
	s.Equal(invalidDeploymentReferenceReason, batch.Status.Condition.Reason)
	s.Equal(err.Error(), batch.Status.Condition.Message)
}

func (s *syncerTestSuite) Test_SyncErrorWhenRadixDeploymentDoesNotExist() {
	batchName, namespace, missingRdName, anyJobComponentName := "any-batch", "any-ns", "missing-rd", "any-job"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: missingRdName},
				Job:                  anyJobComponentName,
			},
			Jobs: []radixv1.RadixBatchJob{{Name: "any-job-name"}},
		},
	}

	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	sut := s.createSyncer(batch, nil)
	err = sut.OnSync(context.Background())
	s.Equal(err, newReconcileRadixDeploymentNotFoundError(missingRdName))
	var target reconcileStatus
	s.ErrorAs(err, &target)
	batch, _ = s.radixClient.RadixV1().RadixBatches(namespace).Get(context.Background(), batch.GetName(), metav1.GetOptions{})
	s.Equal(radixv1.BatchConditionTypeWaiting, batch.Status.Condition.Type)
	s.Equal(invalidDeploymentReferenceReason, batch.Status.Condition.Reason)
}

func (s *syncerTestSuite) Test_HandleJobStopWhenMissingRadixDeploymentConfig() {
	batchName, namespace, missingRdName, anyJobComponentName := "any-batch", "any-ns", "missing-rd", "any-job"
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: missingRdName},
				Job:                  anyJobComponentName,
			},
			Jobs: []radixv1.RadixBatchJob{
				{Name: "job1"},
				{Name: "job2"},
			},
		},
	}

	type expectedJobStatusSpec struct {
		name  string
		phase radixv1.RadixBatchJobPhase
	}
	type scenarioSpec struct {
		testName                  string
		stopStatus                map[string]bool
		expectedSyncErr           error
		expectedType              radixv1.RadixBatchConditionType
		expectedReason            string
		expectedMessage           string
		expectedJobStatuses       []expectedJobStatusSpec
		expectedCompletionTimeSet bool
	}

	scenarios := []scenarioSpec{
		{
			testName:                  "stop flag not set",
			expectedSyncErr:           newReconcileRadixDeploymentNotFoundError(missingRdName),
			expectedType:              radixv1.BatchConditionTypeWaiting,
			expectedReason:            invalidDeploymentReferenceReason,
			expectedMessage:           newReconcileRadixDeploymentNotFoundError(missingRdName).Error(),
			expectedCompletionTimeSet: false,
			expectedJobStatuses:       []expectedJobStatusSpec{{name: "job1", phase: radixv1.BatchJobPhaseWaiting}, {name: "job2", phase: radixv1.BatchJobPhaseWaiting}},
		},
		{
			testName:                  "stop flag set to false for both jobs",
			stopStatus:                map[string]bool{"job1": false, "job2": false},
			expectedSyncErr:           newReconcileRadixDeploymentNotFoundError(missingRdName),
			expectedType:              radixv1.BatchConditionTypeWaiting,
			expectedReason:            invalidDeploymentReferenceReason,
			expectedMessage:           newReconcileRadixDeploymentNotFoundError(missingRdName).Error(),
			expectedCompletionTimeSet: false,
			expectedJobStatuses:       []expectedJobStatusSpec{{name: "job1", phase: radixv1.BatchJobPhaseWaiting}, {name: "job2", phase: radixv1.BatchJobPhaseWaiting}},
		},
		{
			testName:                  "stop flag set to true for job1",
			stopStatus:                map[string]bool{"job1": true, "job2": false},
			expectedSyncErr:           newReconcileRadixDeploymentNotFoundError(missingRdName),
			expectedType:              radixv1.BatchConditionTypeWaiting,
			expectedReason:            invalidDeploymentReferenceReason,
			expectedMessage:           newReconcileRadixDeploymentNotFoundError(missingRdName).Error(),
			expectedCompletionTimeSet: false,
			expectedJobStatuses:       []expectedJobStatusSpec{{name: "job1", phase: radixv1.BatchJobPhaseStopped}, {name: "job2", phase: radixv1.BatchJobPhaseWaiting}},
		},
		{
			testName:                  "stop flag set to true for both jobs",
			stopStatus:                map[string]bool{"job1": true, "job2": true},
			expectedSyncErr:           nil,
			expectedType:              radixv1.BatchConditionTypeCompleted,
			expectedReason:            "",
			expectedMessage:           "",
			expectedCompletionTimeSet: true,
			expectedJobStatuses:       []expectedJobStatusSpec{{name: "job1", phase: radixv1.BatchJobPhaseStopped}, {name: "job2", phase: radixv1.BatchJobPhaseStopped}},
		},
	}

	for _, scenario := range scenarios {
		scenario := scenario
		s.Run(scenario.testName, func() {
			batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
			s.Require().NoError(err)
			for jobName, stop := range scenario.stopStatus {
				i := slice.FindIndex(batch.Spec.Jobs, func(j radixv1.RadixBatchJob) bool { return j.Name == jobName })
				batch.Spec.Jobs[i].Stop = pointers.Ptr(stop)
			}
			sut := s.createSyncer(batch, nil)
			err = sut.OnSync(context.Background())
			s.Equal(scenario.expectedSyncErr, err)
			batch, _ = s.radixClient.RadixV1().RadixBatches(namespace).Get(context.Background(), batch.GetName(), metav1.GetOptions{})
			s.Equal(scenario.expectedType, batch.Status.Condition.Type)
			s.Equal(scenario.expectedReason, batch.Status.Condition.Reason)
			s.Equal(scenario.expectedMessage, batch.Status.Condition.Message)
			s.Equal(scenario.expectedCompletionTimeSet, batch.Status.Condition.CompletionTime != nil)
			actualJobStatus := slice.Map(batch.Status.JobStatuses, func(s radixv1.RadixBatchJobStatus) expectedJobStatusSpec {
				return expectedJobStatusSpec{name: s.Name, phase: s.Phase}
			})
			s.ElementsMatch(scenario.expectedJobStatuses, actualJobStatus)
		})
	}

}

func (s *syncerTestSuite) Test_BatchJobStatus() {
	const (
		namespace        = "any-ns"
		appName          = "any-app"
		rdName           = "any-rd"
		jobComponentName = "any-job"
		batchName        = "any-rb"
		batchJobName     = "any-batch-job"
	)

	var (
		now time.Time = time.Date(2024, 1, 20, 8, 00, 00, 0, time.Local)
	)

	type kubeJobSpec struct {
		creationTimestamp metav1.Time
		status            batchv1.JobStatus
	}

	type jobSpec struct {
		stop             bool
		kubeJob          *kubeJobSpec
		currentJobStatus *radixv1.RadixBatchJobStatus
	}

	tests := map[string]struct {
		job            jobSpec
		expectedStatus radixv1.RadixBatchJobStatus
	}{
		"kubejob not active and no current status": {
			job: jobSpec{
				kubeJob: &kubeJobSpec{
					creationTimestamp: metav1.Time{Time: now.Add(-5 * time.Hour)},
					status: batchv1.JobStatus{
						Failed:     5,
						StartTime:  &metav1.Time{Time: now.Add(-6 * time.Hour)},
						Conditions: []batchv1.JobCondition{{Type: "any-condition-type", Status: corev1.ConditionTrue, Reason: "any-condition-reason", Message: "any-condition-message", LastTransitionTime: metav1.Now()}},
					}}},
			expectedStatus: radixv1.RadixBatchJobStatus{
				Phase:        radixv1.BatchJobPhaseWaiting,
				CreationTime: &metav1.Time{Time: now.Add(-5 * time.Hour)},
				StartTime:    &metav1.Time{Time: now.Add(-6 * time.Hour)},
				Failed:       5,
			},
		},
		"kubejob not active and current status is set": {
			job: jobSpec{
				currentJobStatus: &radixv1.RadixBatchJobStatus{
					Restart:      "any-current-restart",
					CreationTime: &metav1.Time{Time: now.Add(-15 * time.Hour)},
					StartTime:    &metav1.Time{Time: now.Add(-16 * time.Hour)},
					EndTime:      &metav1.Time{Time: now.Add(-17 * time.Hour)},
					Phase:        "any-current-phase",
					Reason:       "any-current-reason",
					Message:      "any-current-message",
					Failed:       100,
				},
				kubeJob: &kubeJobSpec{
					creationTimestamp: metav1.Time{Time: now.Add(-5 * time.Hour)},
					status: batchv1.JobStatus{
						Failed:     5,
						StartTime:  &metav1.Time{Time: now.Add(-6 * time.Hour)},
						Conditions: []batchv1.JobCondition{{Type: "any-condition-type", Status: corev1.ConditionTrue, Reason: "any-condition-reason", Message: "any-condition-message", LastTransitionTime: metav1.Now()}},
					}}},
			expectedStatus: radixv1.RadixBatchJobStatus{
				Phase:        "any-current-phase",
				CreationTime: &metav1.Time{Time: now.Add(-5 * time.Hour)},
				StartTime:    &metav1.Time{Time: now.Add(-6 * time.Hour)},
				Failed:       5,
				Restart:      "any-current-restart",
			},
		},

		"job stop set and no current status": {
			job: jobSpec{
				stop: true,
			},
			expectedStatus: radixv1.RadixBatchJobStatus{
				Phase:   radixv1.BatchJobPhaseStopped,
				EndTime: &metav1.Time{Time: now},
			},
		},
		"job stopped and current status is set": {
			job: jobSpec{
				stop: true,
				currentJobStatus: &radixv1.RadixBatchJobStatus{
					Restart:      "any-current-restart",
					CreationTime: &metav1.Time{Time: now.Add(-15 * time.Hour)},
					StartTime:    &metav1.Time{Time: now.Add(-16 * time.Hour)},
					EndTime:      &metav1.Time{Time: now.Add(-17 * time.Hour)},
					Phase:        "any-current-phase",
					Reason:       "any-current-reason",
					Message:      "any-current-message",
					Failed:       100,
				},
			},
			expectedStatus: radixv1.RadixBatchJobStatus{
				Phase:        radixv1.BatchJobPhaseStopped,
				CreationTime: &metav1.Time{Time: now.Add(-15 * time.Hour)},
				StartTime:    &metav1.Time{Time: now.Add(-16 * time.Hour)},
				EndTime:      &metav1.Time{Time: now},
				Restart:      "any-current-restart",
				Failed:       100,
			},
		},
		"kubejob active and no current status": {
			job: jobSpec{
				kubeJob: &kubeJobSpec{
					creationTimestamp: metav1.Time{Time: now.Add(-5 * time.Hour)},
					status: batchv1.JobStatus{
						Failed:     5,
						Active:     1,
						StartTime:  &metav1.Time{Time: now.Add(-6 * time.Hour)},
						Conditions: []batchv1.JobCondition{{Type: "any-condition-type", Status: corev1.ConditionTrue, Reason: "any-condition-reason", Message: "any-condition-message", LastTransitionTime: metav1.Now()}},
					}}},
			expectedStatus: radixv1.RadixBatchJobStatus{
				Phase:        radixv1.BatchJobPhaseActive,
				CreationTime: &metav1.Time{Time: now.Add(-5 * time.Hour)},
				StartTime:    &metav1.Time{Time: now.Add(-6 * time.Hour)},
				Failed:       5,
			},
		},
		"kubejob active and current status is set": {
			job: jobSpec{
				kubeJob: &kubeJobSpec{
					creationTimestamp: metav1.Time{Time: now.Add(-5 * time.Hour)},
					status: batchv1.JobStatus{
						Failed:     5,
						Active:     1,
						StartTime:  &metav1.Time{Time: now.Add(-6 * time.Hour)},
						Conditions: []batchv1.JobCondition{{Type: "any-condition-type", Status: corev1.ConditionTrue, Reason: "any-condition-reason", Message: "any-condition-message", LastTransitionTime: metav1.Now()}},
					}},
				currentJobStatus: &radixv1.RadixBatchJobStatus{
					Restart:      "any-current-restart",
					CreationTime: &metav1.Time{Time: now.Add(-15 * time.Hour)},
					StartTime:    &metav1.Time{Time: now.Add(-16 * time.Hour)},
					EndTime:      &metav1.Time{Time: now.Add(-17 * time.Hour)},
					Phase:        "any-current-phase",
					Reason:       "any-current-reason",
					Message:      "any-current-message",
					Failed:       100,
				},
			},
			expectedStatus: radixv1.RadixBatchJobStatus{
				Phase:        radixv1.BatchJobPhaseActive,
				CreationTime: &metav1.Time{Time: now.Add(-5 * time.Hour)},
				StartTime:    &metav1.Time{Time: now.Add(-6 * time.Hour)},
				Restart:      "any-current-restart",
				Failed:       5,
			},
		},
		"kubejob active, ready and no current status": {
			job: jobSpec{
				kubeJob: &kubeJobSpec{
					creationTimestamp: metav1.Time{Time: now.Add(-5 * time.Hour)},
					status: batchv1.JobStatus{
						Failed:     5,
						Active:     1,
						Ready:      pointers.Ptr[int32](1),
						StartTime:  &metav1.Time{Time: now.Add(-6 * time.Hour)},
						Conditions: []batchv1.JobCondition{{Type: "any-condition-type", Status: corev1.ConditionTrue, Reason: "any-condition-reason", Message: "any-condition-message", LastTransitionTime: metav1.Now()}},
					}}},
			expectedStatus: radixv1.RadixBatchJobStatus{
				Phase:        radixv1.BatchJobPhaseRunning,
				CreationTime: &metav1.Time{Time: now.Add(-5 * time.Hour)},
				StartTime:    &metav1.Time{Time: now.Add(-6 * time.Hour)},
				Failed:       5,
			},
		},
		"kubejob active, ready and current status is set": {
			job: jobSpec{
				kubeJob: &kubeJobSpec{
					creationTimestamp: metav1.Time{Time: now.Add(-5 * time.Hour)},
					status: batchv1.JobStatus{
						Failed:     5,
						Active:     1,
						Ready:      pointers.Ptr[int32](1),
						StartTime:  &metav1.Time{Time: now.Add(-6 * time.Hour)},
						Conditions: []batchv1.JobCondition{{Type: "any-condition-type", Status: corev1.ConditionTrue, Reason: "any-condition-reason", Message: "any-condition-message", LastTransitionTime: metav1.Now()}},
					}},
				currentJobStatus: &radixv1.RadixBatchJobStatus{
					Restart:      "any-current-restart",
					CreationTime: &metav1.Time{Time: now.Add(-15 * time.Hour)},
					StartTime:    &metav1.Time{Time: now.Add(-16 * time.Hour)},
					EndTime:      &metav1.Time{Time: now.Add(-17 * time.Hour)},
					Phase:        "any-current-phase",
					Reason:       "any-current-reason",
					Message:      "any-current-message",
					Failed:       100,
				},
			},
			expectedStatus: radixv1.RadixBatchJobStatus{
				Phase:        radixv1.BatchJobPhaseRunning,
				CreationTime: &metav1.Time{Time: now.Add(-5 * time.Hour)},
				StartTime:    &metav1.Time{Time: now.Add(-6 * time.Hour)},
				Restart:      "any-current-restart",
				Failed:       5,
			},
		},
		"kubejob Complete condition and no current status": {
			job: jobSpec{
				kubeJob: &kubeJobSpec{
					creationTimestamp: metav1.Time{Time: now.Add(-5 * time.Hour)},
					status: batchv1.JobStatus{
						Failed:    5,
						Active:    1,
						Ready:     pointers.Ptr[int32](1),
						StartTime: &metav1.Time{Time: now.Add(-6 * time.Hour)},
						Conditions: []batchv1.JobCondition{{
							Type:               batchv1.JobComplete,
							Status:             corev1.ConditionTrue,
							Reason:             "any-condition-reason",
							Message:            "any-condition-message",
							LastTransitionTime: metav1.Time{Time: now.Add(-4 * time.Hour)}},
						},
					}}},
			expectedStatus: radixv1.RadixBatchJobStatus{
				Phase:        radixv1.BatchJobPhaseSucceeded,
				CreationTime: &metav1.Time{Time: now.Add(-5 * time.Hour)},
				StartTime:    &metav1.Time{Time: now.Add(-6 * time.Hour)},
				EndTime:      &metav1.Time{Time: now.Add(-4 * time.Hour)},
				Reason:       "any-condition-reason",
				Message:      "any-condition-message",
				Failed:       5,
			},
		},
		"kubejob Complete condition and current status is set": {
			job: jobSpec{
				kubeJob: &kubeJobSpec{
					creationTimestamp: metav1.Time{Time: now.Add(-5 * time.Hour)},
					status: batchv1.JobStatus{
						Failed:    5,
						Active:    1,
						Ready:     pointers.Ptr[int32](1),
						StartTime: &metav1.Time{Time: now.Add(-6 * time.Hour)},
						Conditions: []batchv1.JobCondition{{
							Type:               batchv1.JobComplete,
							Status:             corev1.ConditionTrue,
							Reason:             "any-condition-reason",
							Message:            "any-condition-message",
							LastTransitionTime: metav1.Time{Time: now.Add(-4 * time.Hour)}},
						},
					}},
				currentJobStatus: &radixv1.RadixBatchJobStatus{
					Restart:      "any-current-restart",
					CreationTime: &metav1.Time{Time: now.Add(-15 * time.Hour)},
					StartTime:    &metav1.Time{Time: now.Add(-16 * time.Hour)},
					EndTime:      &metav1.Time{Time: now.Add(-17 * time.Hour)},
					Phase:        "any-current-phase",
					Reason:       "any-current-reason",
					Message:      "any-current-message",
					Failed:       100,
				},
			},
			expectedStatus: radixv1.RadixBatchJobStatus{
				Phase:        radixv1.BatchJobPhaseSucceeded,
				CreationTime: &metav1.Time{Time: now.Add(-5 * time.Hour)},
				StartTime:    &metav1.Time{Time: now.Add(-6 * time.Hour)},
				EndTime:      &metav1.Time{Time: now.Add(-4 * time.Hour)},
				Reason:       "any-condition-reason",
				Message:      "any-condition-message",
				Restart:      "any-current-restart",
				Failed:       5,
			},
		},
		"kubejob SuccessCriteriaMet condition and no current status": {
			job: jobSpec{
				kubeJob: &kubeJobSpec{
					creationTimestamp: metav1.Time{Time: now.Add(-5 * time.Hour)},
					status: batchv1.JobStatus{
						Failed:    5,
						Active:    1,
						Ready:     pointers.Ptr[int32](1),
						StartTime: &metav1.Time{Time: now.Add(-6 * time.Hour)},
						Conditions: []batchv1.JobCondition{{
							Type:               batchv1.JobSuccessCriteriaMet,
							Status:             corev1.ConditionTrue,
							Reason:             "any-condition-reason",
							Message:            "any-condition-message",
							LastTransitionTime: metav1.Time{Time: now.Add(-4 * time.Hour)}},
						},
					}}},
			expectedStatus: radixv1.RadixBatchJobStatus{
				Phase:        radixv1.BatchJobPhaseSucceeded,
				CreationTime: &metav1.Time{Time: now.Add(-5 * time.Hour)},
				StartTime:    &metav1.Time{Time: now.Add(-6 * time.Hour)},
				EndTime:      &metav1.Time{Time: now.Add(-4 * time.Hour)},
				Reason:       "any-condition-reason",
				Message:      "any-condition-message",
				Failed:       5,
			},
		},
		"kubejob SuccessCriteriaMet condition and current status is set": {
			job: jobSpec{
				kubeJob: &kubeJobSpec{
					creationTimestamp: metav1.Time{Time: now.Add(-5 * time.Hour)},
					status: batchv1.JobStatus{
						Failed:    5,
						Active:    1,
						Ready:     pointers.Ptr[int32](1),
						StartTime: &metav1.Time{Time: now.Add(-6 * time.Hour)},
						Conditions: []batchv1.JobCondition{{
							Type:               batchv1.JobSuccessCriteriaMet,
							Status:             corev1.ConditionTrue,
							Reason:             "any-condition-reason",
							Message:            "any-condition-message",
							LastTransitionTime: metav1.Time{Time: now.Add(-4 * time.Hour)}},
						},
					}},
				currentJobStatus: &radixv1.RadixBatchJobStatus{
					Restart:      "any-current-restart",
					CreationTime: &metav1.Time{Time: now.Add(-15 * time.Hour)},
					StartTime:    &metav1.Time{Time: now.Add(-16 * time.Hour)},
					EndTime:      &metav1.Time{Time: now.Add(-17 * time.Hour)},
					Phase:        "any-current-phase",
					Reason:       "any-current-reason",
					Message:      "any-current-message",
					Failed:       100,
				},
			},
			expectedStatus: radixv1.RadixBatchJobStatus{
				Phase:        radixv1.BatchJobPhaseSucceeded,
				CreationTime: &metav1.Time{Time: now.Add(-5 * time.Hour)},
				StartTime:    &metav1.Time{Time: now.Add(-6 * time.Hour)},
				EndTime:      &metav1.Time{Time: now.Add(-4 * time.Hour)},
				Reason:       "any-condition-reason",
				Message:      "any-condition-message",
				Restart:      "any-current-restart",
				Failed:       5,
			},
		},
		"kubejob Failed condition and no current status": {
			job: jobSpec{
				kubeJob: &kubeJobSpec{
					creationTimestamp: metav1.Time{Time: now.Add(-5 * time.Hour)},
					status: batchv1.JobStatus{
						Failed:    5,
						Active:    1,
						Ready:     pointers.Ptr[int32](1),
						StartTime: &metav1.Time{Time: now.Add(-6 * time.Hour)},
						Conditions: []batchv1.JobCondition{{
							Type:               batchv1.JobFailed,
							Status:             corev1.ConditionTrue,
							Reason:             "any-condition-reason",
							Message:            "any-condition-message",
							LastTransitionTime: metav1.Time{Time: now.Add(-4 * time.Hour)}},
						},
					}}},
			expectedStatus: radixv1.RadixBatchJobStatus{
				Phase:        radixv1.BatchJobPhaseFailed,
				CreationTime: &metav1.Time{Time: now.Add(-5 * time.Hour)},
				StartTime:    &metav1.Time{Time: now.Add(-6 * time.Hour)},
				EndTime:      &metav1.Time{Time: now.Add(-4 * time.Hour)},
				Reason:       "any-condition-reason",
				Message:      "any-condition-message",
				Failed:       5,
			},
		},
		"kubejob Failed condition and current status is set": {
			job: jobSpec{
				kubeJob: &kubeJobSpec{
					creationTimestamp: metav1.Time{Time: now.Add(-5 * time.Hour)},
					status: batchv1.JobStatus{
						Failed:    5,
						Active:    1,
						Ready:     pointers.Ptr[int32](1),
						StartTime: &metav1.Time{Time: now.Add(-6 * time.Hour)},
						Conditions: []batchv1.JobCondition{{
							Type:               batchv1.JobFailed,
							Status:             corev1.ConditionTrue,
							Reason:             "any-condition-reason",
							Message:            "any-condition-message",
							LastTransitionTime: metav1.Time{Time: now.Add(-4 * time.Hour)}},
						},
					}},
				currentJobStatus: &radixv1.RadixBatchJobStatus{
					Restart:      "any-current-restart",
					CreationTime: &metav1.Time{Time: now.Add(-15 * time.Hour)},
					StartTime:    &metav1.Time{Time: now.Add(-16 * time.Hour)},
					EndTime:      &metav1.Time{Time: now.Add(-17 * time.Hour)},
					Phase:        "any-current-phase",
					Reason:       "any-current-reason",
					Message:      "any-current-message",
					Failed:       100,
				},
			},
			expectedStatus: radixv1.RadixBatchJobStatus{
				Phase:        radixv1.BatchJobPhaseFailed,
				CreationTime: &metav1.Time{Time: now.Add(-5 * time.Hour)},
				StartTime:    &metav1.Time{Time: now.Add(-6 * time.Hour)},
				EndTime:      &metav1.Time{Time: now.Add(-4 * time.Hour)},
				Reason:       "any-condition-reason",
				Message:      "any-condition-message",
				Restart:      "any-current-restart",
				Failed:       5,
			},
		},
	}

	jobLabelsFunc := func(jobName string) labels.Set {
		return radixlabels.Merge(
			radixlabels.ForApplicationName(appName),
			radixlabels.ForComponentName(jobComponentName),
			radixlabels.ForBatchName(batchName),
			radixlabels.ForJobScheduleJobType(),
			radixlabels.ForBatchJobName(jobName),
		)
	}

	for testName, test := range tests {
		s.Run(testName, func() {

			rd := &radixv1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: rdName},
				Spec:       radixv1.RadixDeploymentSpec{AppName: appName, Jobs: []radixv1.RadixDeployJobComponent{{Name: jobComponentName}}},
			}
			_, err := s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
			s.Require().NoError(err)

			batch := &radixv1.RadixBatch{
				ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
				Spec: radixv1.RadixBatchSpec{
					RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{LocalObjectReference: radixv1.LocalObjectReference{Name: rdName}, Job: jobComponentName},
					Jobs:                  []radixv1.RadixBatchJob{{Name: batchJobName, Stop: &test.job.stop}},
				},
			}
			if currentStatus := test.job.currentJobStatus; currentStatus != nil {
				batch.Status.JobStatuses = []radixv1.RadixBatchJobStatus{*currentStatus}
				batch.Status.JobStatuses[0].Name = batchJobName
			}
			batch, err = s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
			s.Require().NoError(err)

			// Create k8s jobs required for building batch status
			if kubeJob := test.job.kubeJob; kubeJob != nil {
				j := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:              getKubeJobName(batchName, batchJobName),
						Namespace:         namespace,
						Labels:            jobLabelsFunc(batchJobName),
						CreationTimestamp: kubeJob.creationTimestamp,
					},
					Status: kubeJob.status,
				}
				_, err = s.kubeClient.BatchV1().Jobs(namespace).Create(context.Background(), j, metav1.CreateOptions{})
				s.Require().NoError(err)
			}

			// Run test
			sut := s.createSyncer(batch, nil, WithClock(commonutils.NewFakeClock(now)))
			s.Require().NoError(sut.OnSync(context.Background()))
			batch, err = s.radixClient.RadixV1().RadixBatches(namespace).Get(context.Background(), batchName, metav1.GetOptions{})
			s.Require().NoError(err)

			expectedStatus := test.expectedStatus
			expectedStatus.Name = batchJobName
			s.Equal([]radixv1.RadixBatchJobStatus{expectedStatus}, batch.Status.JobStatuses)
		})
	}
}

func (s *syncerTestSuite) Test_BatchStatusCondition() {
	const (
		namespace        = "any-ns"
		appName          = "any-app"
		rdName           = "any-rd"
		jobComponentName = "any-job"
		batchName        = "any-rb"
	)

	var (
		now time.Time = time.Date(2024, 1, 20, 8, 00, 00, 0, time.Local)
	)

	type jobSpec struct {
		stop          bool
		kubeJobStatus batchv1.JobStatus
	}

	type partialBatchStatus struct {
		jobPhases []radixv1.RadixBatchJobPhase
		condition radixv1.RadixBatchCondition
	}

	waitingJob := jobSpec{kubeJobStatus: batchv1.JobStatus{}}
	activeJob := jobSpec{kubeJobStatus: batchv1.JobStatus{Active: 1}}
	runningJob := jobSpec{kubeJobStatus: batchv1.JobStatus{Active: 1, Ready: pointers.Ptr[int32](1)}}
	succeededJob := jobSpec{kubeJobStatus: batchv1.JobStatus{Conditions: []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}}}
	failedJob := jobSpec{kubeJobStatus: batchv1.JobStatus{Conditions: []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: corev1.ConditionTrue}}}}
	stoppedJob := jobSpec{stop: true}

	tests := map[string]struct {
		jobs           map[string]jobSpec
		expectedStatus partialBatchStatus
	}{
		"Waiting condition when all jobs Waiting": {
			jobs: map[string]jobSpec{
				"job1": waitingJob,
				"job2": waitingJob,
			},
			expectedStatus: partialBatchStatus{
				jobPhases: []radixv1.RadixBatchJobPhase{radixv1.BatchJobPhaseWaiting, radixv1.BatchJobPhaseWaiting},
				condition: radixv1.RadixBatchCondition{
					Type: radixv1.BatchConditionTypeWaiting,
				},
			},
		},
		"Completed condition when all jobs in done phase": {
			jobs: map[string]jobSpec{
				"job1": stoppedJob,
				"job2": succeededJob,
				"job3": failedJob,
			},
			expectedStatus: partialBatchStatus{
				jobPhases: []radixv1.RadixBatchJobPhase{radixv1.BatchJobPhaseStopped, radixv1.BatchJobPhaseSucceeded, radixv1.BatchJobPhaseFailed},
				condition: radixv1.RadixBatchCondition{
					Type:           radixv1.BatchConditionTypeCompleted,
					ActiveTime:     &metav1.Time{Time: now},
					CompletionTime: &metav1.Time{Time: now},
				},
			},
		},
	}

	// Build test spec for all job status phases that will lead to an Active condition
	activeJobsMap := map[radixv1.RadixBatchJobPhase]jobSpec{
		radixv1.BatchJobPhaseActive:  activeJob,
		radixv1.BatchJobPhaseRunning: runningJob,
	}
	allJobsMap := map[radixv1.RadixBatchJobPhase]jobSpec{
		radixv1.BatchJobPhaseWaiting:   waitingJob,
		radixv1.BatchJobPhaseActive:    activeJob,
		radixv1.BatchJobPhaseRunning:   runningJob,
		radixv1.BatchJobPhaseStopped:   stoppedJob,
		radixv1.BatchJobPhaseSucceeded: succeededJob,
		radixv1.BatchJobPhaseFailed:    failedJob,
	}
	for activeStatus, activeJob := range activeJobsMap {
		for anyStatus, anyJob := range allJobsMap {
			tests[fmt.Sprintf("Completed condition when jobs %s and %s", activeStatus, anyStatus)] = struct {
				jobs           map[string]jobSpec
				expectedStatus partialBatchStatus
			}{
				jobs: map[string]jobSpec{
					"job1": activeJob,
					"job2": anyJob,
				},
				expectedStatus: partialBatchStatus{
					jobPhases: []radixv1.RadixBatchJobPhase{activeStatus, anyStatus},
					condition: radixv1.RadixBatchCondition{
						Type:       radixv1.BatchConditionTypeActive,
						ActiveTime: &metav1.Time{Time: now},
					},
				},
			}
		}
	}

	jobLabelsFunc := func(jobName string) labels.Set {
		return radixlabels.Merge(
			radixlabels.ForApplicationName(appName),
			radixlabels.ForComponentName(jobComponentName),
			radixlabels.ForBatchName(batchName),
			radixlabels.ForJobScheduleJobType(),
			radixlabels.ForBatchJobName(jobName),
		)
	}

	for testName, test := range tests {
		s.Run(testName, func() {

			rd := &radixv1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: rdName},
				Spec:       radixv1.RadixDeploymentSpec{AppName: appName, Jobs: []radixv1.RadixDeployJobComponent{{Name: jobComponentName}}},
			}
			_, err := s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
			s.Require().NoError(err)

			batch := &radixv1.RadixBatch{
				ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
				Spec: radixv1.RadixBatchSpec{
					RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{LocalObjectReference: radixv1.LocalObjectReference{Name: rdName}, Job: jobComponentName},
					Jobs:                  []radixv1.RadixBatchJob{},
				},
			}
			for batchJobName, job := range test.jobs {
				batch.Spec.Jobs = append(batch.Spec.Jobs, radixv1.RadixBatchJob{Name: batchJobName, Stop: &job.stop})

				j := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getKubeJobName(batchName, batchJobName),
						Namespace: namespace,
						Labels:    jobLabelsFunc(batchJobName),
					},
					Status: job.kubeJobStatus,
				}
				_, err = s.kubeClient.BatchV1().Jobs(namespace).Create(context.Background(), j, metav1.CreateOptions{})
				s.Require().NoError(err)
			}
			batch, err = s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
			s.Require().NoError(err)

			// Run test
			sut := s.createSyncer(batch, nil, WithClock(commonutils.NewFakeClock(now)))
			s.Require().NoError(sut.OnSync(context.Background()))
			batch, err = s.radixClient.RadixV1().RadixBatches(namespace).Get(context.Background(), batchName, metav1.GetOptions{})
			s.Require().NoError(err)

			actualJobPhases := slice.Map(batch.Status.JobStatuses, func(s radixv1.RadixBatchJobStatus) radixv1.RadixBatchJobPhase { return s.Phase })
			s.ElementsMatch(test.expectedStatus.jobPhases, actualJobPhases)
			s.Equal(test.expectedStatus.condition, batch.Status.Condition)
		})
	}
}

func (s *syncerTestSuite) Test_ShouldRestartBatchJobNotRestartedBefore() {
	const (
		appName          = "any-app"
		rdName           = "any-rd"
		namespace        = "any-ns"
		jobComponentName = "any-job"
		batchName        = "any-rb"
		batchJob1Name    = "job1"
		batchJob2Name    = "job2"
		restartTime      = "2020-01-01T08:00:00Z"
	)
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  jobComponentName,
			},
			Jobs: []radixv1.RadixBatchJob{
				{Name: batchJob1Name, Restart: restartTime},
				{Name: batchJob2Name},
			},
		},
		Status: radixv1.RadixBatchStatus{
			Condition: radixv1.RadixBatchCondition{
				Type: radixv1.BatchConditionTypeCompleted,
			},
			JobStatuses: []radixv1.RadixBatchJobStatus{
				{Name: batchJob1Name, Phase: radixv1.BatchJobPhaseSucceeded},
				{Name: batchJob2Name, Phase: radixv1.BatchJobPhaseSucceeded},
			},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: appName,
			Jobs:    []radixv1.RadixDeployJobComponent{{Name: jobComponentName}},
		},
	}
	kubeJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "any-previous-job-name",
			Labels: radixlabels.Merge(radixlabels.ForBatchName(batchName), radixlabels.ForBatchJobName(batchJob1Name)),
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.kubeClient.BatchV1().Jobs(namespace).Create(context.Background(), kubeJob, metav1.CreateOptions{})
	s.Require().NoError(err)
	sut := s.createSyncer(batch, nil)
	s.Require().NoError(sut.OnSync(context.Background()))
	batch, err = s.radixClient.RadixV1().RadixBatches(namespace).Get(context.Background(), batchName, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal(restartTime, batch.Status.JobStatuses[0].Restart)
	s.Equal(radixv1.BatchJobPhaseWaiting, batch.Status.JobStatuses[0].Phase)
	s.Equal(radixv1.BatchConditionTypeActive, batch.Status.Condition.Type)
	jobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(jobs.Items, 1)
	s.Equal(getKubeJobName(batchName, batchJob1Name), jobs.Items[0].GetName())
}

func (s *syncerTestSuite) Test_ShouldRestartBatchJobWithNewRestartTimestamp() {
	const (
		appName          = "any-app"
		rdName           = "any-rd"
		namespace        = "any-ns"
		jobComponentName = "any-job"
		batchName        = "any-rb"
		batchJob1Name    = "job1"
		batchJob2Name    = "job2"
		newRestartTime   = "2020-01-01T08:00:00Z"
		oldRestartTime   = "2020-01-01T07:00:00Z"
	)
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  jobComponentName,
			},
			Jobs: []radixv1.RadixBatchJob{
				{Name: batchJob1Name, Restart: newRestartTime},
				{Name: batchJob2Name},
			},
		},
		Status: radixv1.RadixBatchStatus{
			Condition: radixv1.RadixBatchCondition{
				Type: radixv1.BatchConditionTypeCompleted,
			},
			JobStatuses: []radixv1.RadixBatchJobStatus{
				{Name: batchJob1Name, Phase: radixv1.BatchJobPhaseSucceeded, Restart: oldRestartTime},
				{Name: batchJob2Name, Phase: radixv1.BatchJobPhaseSucceeded},
			},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: appName,
			Jobs:    []radixv1.RadixDeployJobComponent{{Name: jobComponentName}},
		},
	}
	kubeJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "any-previous-job-name",
			Labels: radixlabels.Merge(radixlabels.ForBatchName(batchName), radixlabels.ForBatchJobName(batchJob1Name)),
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.kubeClient.BatchV1().Jobs(namespace).Create(context.Background(), kubeJob, metav1.CreateOptions{})
	s.Require().NoError(err)
	sut := s.createSyncer(batch, nil)
	s.Require().NoError(sut.OnSync(context.Background()))
	batch, err = s.radixClient.RadixV1().RadixBatches(namespace).Get(context.Background(), batchName, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal(newRestartTime, batch.Status.JobStatuses[0].Restart)
	s.Equal(radixv1.BatchJobPhaseWaiting, batch.Status.JobStatuses[0].Phase)
	s.Equal(radixv1.BatchConditionTypeActive, batch.Status.Condition.Type)
	jobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(jobs.Items, 1)
	s.Equal(getKubeJobName(batchName, batchJob1Name), jobs.Items[0].GetName())
}

func (s *syncerTestSuite) Test_ShouldNotRestartBatchJobWhenAlreadyRestarted() {
	const (
		appName            = "any-app"
		rdName             = "any-rd"
		namespace          = "any-ns"
		jobComponentName   = "any-job"
		batchName          = "any-rb"
		batchJob1Name      = "job1"
		batchJob2Name      = "job2"
		restartTime        = "2020-01-01T08:00:00Z"
		exitingKubeJobName = "any-exiting-kube-job"
	)
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  jobComponentName,
			},
			Jobs: []radixv1.RadixBatchJob{
				{Name: batchJob1Name, Restart: restartTime},
				{Name: batchJob2Name},
			},
		},
		Status: radixv1.RadixBatchStatus{
			Condition: radixv1.RadixBatchCondition{
				Type: radixv1.BatchConditionTypeCompleted,
			},
			JobStatuses: []radixv1.RadixBatchJobStatus{
				{Name: batchJob1Name, Phase: radixv1.BatchJobPhaseSucceeded, Restart: restartTime},
				{Name: batchJob2Name, Phase: radixv1.BatchJobPhaseSucceeded},
			},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: appName,
			Jobs:    []radixv1.RadixDeployJobComponent{{Name: jobComponentName}},
		},
	}
	kubeJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:   exitingKubeJobName,
			Labels: radixlabels.Merge(radixlabels.ForBatchName(batchName), radixlabels.ForBatchJobName(batchJob1Name)),
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.kubeClient.BatchV1().Jobs(namespace).Create(context.Background(), kubeJob, metav1.CreateOptions{})
	s.Require().NoError(err)
	sut := s.createSyncer(batch, nil)
	s.Require().NoError(sut.OnSync(context.Background()))
	batch, err = s.radixClient.RadixV1().RadixBatches(namespace).Get(context.Background(), batchName, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal(restartTime, batch.Status.JobStatuses[0].Restart)
	s.Equal(radixv1.BatchJobPhaseSucceeded, batch.Status.JobStatuses[0].Phase)
	s.Equal(radixv1.BatchConditionTypeCompleted, batch.Status.Condition.Type)
	jobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(jobs.Items, 1)
	s.Equal(exitingKubeJobName, jobs.Items[0].GetName())
}

func (s *syncerTestSuite) Test_ShouldKeepRestartStatusOnSync() {
	const (
		appName            = "any-app"
		rdName             = "any-rd"
		namespace          = "any-ns"
		jobComponentName   = "any-job"
		batchName          = "any-rb"
		batchJob1Name      = "job1"
		batchJob2Name      = "job2"
		restartTime        = "2020-01-01T08:00:00Z"
		exitingKubeJobName = "any-exiting-kube-job"
	)
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  jobComponentName,
			},
			Jobs: []radixv1.RadixBatchJob{
				{Name: batchJob1Name, Restart: restartTime},
				{Name: batchJob2Name},
			},
		},
		Status: radixv1.RadixBatchStatus{
			Condition: radixv1.RadixBatchCondition{
				Type: radixv1.BatchConditionTypeActive,
			},
			JobStatuses: []radixv1.RadixBatchJobStatus{
				{Name: batchJob1Name, Phase: radixv1.BatchJobPhaseWaiting, Restart: restartTime},
				{Name: batchJob2Name, Phase: radixv1.BatchJobPhaseSucceeded},
			},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: appName,
			Jobs:    []radixv1.RadixDeployJobComponent{{Name: jobComponentName}},
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)
	sut := s.createSyncer(batch, nil)
	s.Require().NoError(sut.OnSync(context.Background()))
	batch, err = s.radixClient.RadixV1().RadixBatches(namespace).Get(context.Background(), batchName, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal(restartTime, batch.Status.JobStatuses[0].Restart)
}

func (s *syncerTestSuite) Test_RestartCorrectlyHandledWithIntermediateStatusUpdates() {
	const (
		appName          = "any-app"
		rdName           = "any-rd"
		namespace        = "any-ns"
		jobComponentName = "any-job"
		batchName        = "any-rb"
		batchJobName     = "thejob"
		restartTime      = "2020-01-01T08:00:00Z"
	)
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  jobComponentName,
			},
			Jobs: func() []radixv1.RadixBatchJob {
				var jobs []radixv1.RadixBatchJob
				for i := range syncStatusForEveryNumberOfBatchJobsReconciled {
					jobs = append(jobs, radixv1.RadixBatchJob{Name: fmt.Sprintf("anyjob%v", i)})
				}
				jobs = append(jobs, radixv1.RadixBatchJob{Name: batchJobName, Restart: restartTime})
				return jobs
			}(),
		},
		Status: radixv1.RadixBatchStatus{
			Condition: radixv1.RadixBatchCondition{
				Type: radixv1.BatchConditionTypeCompleted,
			},
			JobStatuses: func() []radixv1.RadixBatchJobStatus {
				var jobs []radixv1.RadixBatchJobStatus
				for i := range syncStatusForEveryNumberOfBatchJobsReconciled {
					jobs = append(jobs, radixv1.RadixBatchJobStatus{Name: fmt.Sprintf("anyjob%v", i), Phase: radixv1.BatchJobPhaseSucceeded})
				}
				jobs = append(jobs, radixv1.RadixBatchJobStatus{Name: batchJobName, Phase: radixv1.BatchJobPhaseSucceeded})
				return jobs
			}(),
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: appName,
			Jobs:    []radixv1.RadixDeployJobComponent{{Name: jobComponentName}},
		},
	}
	kubeJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "any-previous-job-name",
			Labels: radixlabels.Merge(radixlabels.ForBatchName(batchName), radixlabels.ForBatchJobName(batchJobName)),
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.kubeClient.BatchV1().Jobs(namespace).Create(context.Background(), kubeJob, metav1.CreateOptions{})
	s.Require().NoError(err)
	sut := s.createSyncer(batch, nil)
	s.Require().NoError(sut.OnSync(context.Background()))
	batch, err = s.radixClient.RadixV1().RadixBatches(namespace).Get(context.Background(), batchName, metav1.GetOptions{})
	s.Require().NoError(err)
	batchJobStatus, foundBatchJobStatus := slice.FindFirst(batch.Status.JobStatuses, func(s radixv1.RadixBatchJobStatus) bool { return s.Name == batchJobName })
	s.Require().True(foundBatchJobStatus)
	s.Equal(restartTime, batchJobStatus.Restart)
	s.Equal(radixv1.BatchJobPhaseWaiting, batchJobStatus.Phase)
	s.Equal(radixv1.BatchConditionTypeActive, batch.Status.Condition.Type)
	jobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(jobs.Items, 1)
	s.Equal(getKubeJobName(batchName, batchJobName), jobs.Items[0].GetName())
}

func (s *syncerTestSuite) Test_FailurePolicy() {
	appName, batchName, componentName, namespace, rdName := "any-app", "any-batch", "compute", "any-ns", "any-rd"
	jobName1, jobName2 := "any-job1", "any-job2"
	rdFailurePolicy := &radixv1.RadixJobComponentFailurePolicy{
		Rules: []radixv1.RadixJobComponentFailurePolicyRule{
			{
				Action: radixv1.RadixJobComponentFailurePolicyActionFailJob,
				OnExitCodes: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes{
					Operator: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOpIn,
					Values:   []int32{5, 3, 4},
				},
			},
		},
	}
	batchJobFailurePolicy := &radixv1.RadixJobComponentFailurePolicy{
		Rules: []radixv1.RadixJobComponentFailurePolicyRule{
			{
				Action: radixv1.RadixJobComponentFailurePolicyActionIgnore,
				OnExitCodes: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes{
					Operator: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOpNotIn,
					Values:   []int32{4, 2, 3},
				},
			},
		},
	}
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  componentName,
			},
			Jobs: []radixv1.RadixBatchJob{
				{Name: jobName1},
				{Name: jobName2, FailurePolicy: batchJobFailurePolicy},
			},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			AppName: appName,
			Jobs: []radixv1.RadixDeployJobComponent{
				{
					Name:          componentName,
					FailurePolicy: rdFailurePolicy,
				},
			},
		},
	}
	batch, err := s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)

	sut := s.createSyncer(batch, nil)
	s.Require().NoError(sut.OnSync(context.Background()))
	jobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(jobs.Items, 2)

	findJobByName := func(name string) func(j batchv1.Job) bool {
		return func(j batchv1.Job) bool { return j.ObjectMeta.Name == getKubeJobName(batchName, name) }
	}

	job1, found := slice.FindFirst(jobs.Items, findJobByName(jobName1))
	s.Require().True(found)
	expectedPodFailurePolicy := utils.GetPodFailurePolicy(rd.Spec.Jobs[0].FailurePolicy)
	s.Equal(expectedPodFailurePolicy, job1.Spec.PodFailurePolicy)

	job2, found := slice.FindFirst(jobs.Items, findJobByName(jobName2))
	s.Require().True(found)
	expectedPodFailurePolicy = utils.GetPodFailurePolicy(batch.Spec.Jobs[1].FailurePolicy)
	s.Equal(expectedPodFailurePolicy, job2.Spec.PodFailurePolicy)
}

func (s *syncerTestSuite) Test_CommandAndArgs() {
	const (
		appName, batchName, jobComponentName, jobName1, env1 = "any-app", "any-batch", "job1", "any-job1", "env1"
	)
	namespace := utils.GetEnvironmentNamespace(appName, env1)
	type scenario struct {
		command []string
		args    []string
	}
	scenarios := map[string]scenario{
		"command and args are not set": {
			command: nil,
			args:    nil,
		},
		"single command is set": {
			command: []string{"bash"},
			args:    nil,
		},
		"command with arguments is set": {
			command: []string{"sh", "-c", "echo hello"},
			args:    nil,
		},
		"command is set and args are set": {
			command: []string{"sh", "-c"},
			args:    []string{"echo hello"},
		},
		"only args are set": {
			command: nil,
			args:    []string{"--verbose", "--output=json"},
		},
	}

	for name, ts := range scenarios {
		s.T().Run(name, func(t *testing.T) {
			s.SetupTest()

			job1Builder := utils.AnApplicationJobComponent().WithName(jobComponentName).
				WithImage("radixdev.azurecr.io/job:imagetag").WithSchedulerPort(numbers.Int32Ptr(8080)).
				WithCommand(ts.command).WithArgs(ts.args)
			raBuilder := utils.NewRadixApplicationBuilder().WithAppName(appName).
				WithEnvironment(env1, "master").WithJobComponents(job1Builder)

			rd := utils.NewDeploymentBuilder().
				WithRadixApplication(raBuilder).
				WithAppName(appName).
				WithImageTag("imagetag").
				WithEnvironment(env1).
				WithJobComponent(utils.NewDeployJobComponentBuilder().
					WithName(jobComponentName).
					WithImage("radixdev.azurecr.io/job:imagetag").
					WithSchedulerPort(numbers.Int32Ptr(8080)).
					WithCommand(ts.command).WithArgs(ts.args)).
				BuildRD()

			batch := &radixv1.RadixBatch{
				ObjectMeta: metav1.ObjectMeta{Name: batchName, Labels: radixlabels.ForJobScheduleJobType()},
				Spec: radixv1.RadixBatchSpec{
					RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
						LocalObjectReference: radixv1.LocalObjectReference{Name: rd.GetName()},
						Job:                  jobComponentName,
					},
					Jobs: []radixv1.RadixBatchJob{
						{Name: jobName1},
					},
				},
			}
			_, err := s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
			s.Require().NoError(err)
			batch, err = s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
			s.Require().NoError(err)

			sut := s.createSyncer(batch, nil)
			s.T().Setenv(defaults.OperatorDNSZoneEnvironmentVariable, "dev")
			s.T().Setenv(defaults.RadixClusterTypeEnvironmentVariable, "development")
			s.T().Setenv(defaults.ContainerRegistryEnvironmentVariable, "dev-acr")
			s.T().Setenv(defaults.OperatorRadixJobSchedulerEnvironmentVariable, "radix-job-scheduler:main-latest")

			s.Require().NoError(sut.OnSync(context.Background()))

			s.T().Cleanup(func() {})

			jobs, err := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err, "failed to list jobs")
			if !s.Assert().Len(jobs.Items, 1, "expected one job to be created") {
				return
			}
			kubeJobList, err := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err, "failed to list jobs")

			kubeJobContainer := kubeJobList.Items[0].Spec.Template.Spec.Containers[0]
			assert.Equal(t, ts.command, kubeJobContainer.Command, "command in job should match in RadixDeployment")
			assert.Equal(t, ts.args, kubeJobContainer.Args, "args in job should match in RadixDeployment")
		})
	}
}

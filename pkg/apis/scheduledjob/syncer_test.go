package scheduledjob

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
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
}

func TestSyncerTestSuite(t *testing.T) {
	suite.Run(t, new(syncerTestSuite))
}

func (s *syncerTestSuite) createSyncer(forJob *radixv1.RadixScheduledJob) Syncer {
	return NewSyncer(s.kubeClient, s.kubeUtil, s.radixClient, forJob)
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
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, secretproviderfake.NewSimpleClientset())
	s.T().Setenv("RADIXOPERATOR_APP_ENV_LIMITS_DEFAULT_MEMORY", "1500Mi")
	s.T().Setenv("RADIXOPERATOR_APP_ENV_LIMITS_DEFAULT_CPU", "2000m")
}

func (s *syncerTestSuite) Test_RestoreStatus() {
	created, started, ended := metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local)), metav1.NewTime(time.Date(2020, 1, 2, 0, 0, 0, 0, time.Local)), metav1.NewTime(time.Date(2020, 1, 3, 0, 0, 0, 0, time.Local))
	expectedStatus := radixv1.RadixScheduledJobStatus{
		Phase:   radixv1.ScheduledJobPhaseSucceeded,
		Reason:  "any-reason",
		Message: "any-message",
		Created: &created,
		Started: &started,
		Ended:   &ended,
	}
	statusBytes, err := json.Marshal(&expectedStatus)
	s.Require().NoError(err)

	jobName, namespace := "any-job", "any-ns"
	job := &radixv1.RadixScheduledJob{
		ObjectMeta: metav1.ObjectMeta{Name: jobName, Annotations: map[string]string{kube.RestoredStatusAnnotation: string(statusBytes)}},
	}
	job, err = s.radixClient.RadixV1().RadixScheduledJobs(namespace).Create(context.Background(), job, metav1.CreateOptions{})
	s.Require().NoError(err)
	sut := s.createSyncer(job)
	s.Require().NoError(sut.OnSync())
	job, err = s.radixClient.RadixV1().RadixScheduledJobs(namespace).Get(context.Background(), jobName, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal(expectedStatus, job.Status)
}

func (s *syncerTestSuite) Test_ShouldRestoreStatusFromAnnotationWhenStatusEmpty() {
	created, started, ended := metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local)), metav1.NewTime(time.Date(2020, 1, 2, 0, 0, 0, 0, time.Local)), metav1.NewTime(time.Date(2020, 1, 3, 0, 0, 0, 0, time.Local))
	expectedStatus := radixv1.RadixScheduledJobStatus{
		Phase:   radixv1.ScheduledJobPhaseSucceeded,
		Reason:  "any-reason",
		Message: "any-message",
		Created: &created,
		Started: &started,
		Ended:   &ended,
	}
	statusBytes, err := json.Marshal(&expectedStatus)
	s.Require().NoError(err)

	jobName, namespace := "any-job", "any-ns"
	job := &radixv1.RadixScheduledJob{
		ObjectMeta: metav1.ObjectMeta{Name: jobName, Annotations: map[string]string{kube.RestoredStatusAnnotation: string(statusBytes)}},
		Status:     radixv1.RadixScheduledJobStatus{},
	}
	job, err = s.radixClient.RadixV1().RadixScheduledJobs(namespace).Create(context.Background(), job, metav1.CreateOptions{})
	s.Require().NoError(err)
	sut := s.createSyncer(job)
	s.Require().NoError(sut.OnSync())
	job, err = s.radixClient.RadixV1().RadixScheduledJobs(namespace).Get(context.Background(), jobName, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal(expectedStatus, job.Status)
}

func (s *syncerTestSuite) Test_ShouldNotRestoreStatusFromAnnotationWhenStatusNotEmpty() {
	created, started, ended := metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local)), metav1.NewTime(time.Date(2020, 1, 2, 0, 0, 0, 0, time.Local)), metav1.NewTime(time.Date(2020, 1, 3, 0, 0, 0, 0, time.Local))
	annotationStatus := radixv1.RadixScheduledJobStatus{
		Phase:   radixv1.ScheduledJobPhaseSucceeded,
		Reason:  "any-reason",
		Message: "any-message",
		Created: &created,
		Started: &started,
		Ended:   &ended,
	}
	statusBytes, err := json.Marshal(&annotationStatus)
	s.Require().NoError(err)

	jobName, namespace := "any-job", "any-ns"
	expectedStatus := radixv1.RadixScheduledJobStatus{Phase: radixv1.ScheduledJobPhaseFailed}
	job := &radixv1.RadixScheduledJob{
		ObjectMeta: metav1.ObjectMeta{Name: jobName, Annotations: map[string]string{kube.RestoredStatusAnnotation: string(statusBytes)}},
		Status:     expectedStatus,
	}
	job, err = s.radixClient.RadixV1().RadixScheduledJobs(namespace).Create(context.Background(), job, metav1.CreateOptions{})
	s.Require().NoError(err)
	sut := s.createSyncer(job)
	s.Require().NoError(sut.OnSync())
	job, err = s.radixClient.RadixV1().RadixScheduledJobs(namespace).Get(context.Background(), jobName, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal(expectedStatus, job.Status)
}

func (s *syncerTestSuite) Test_ShouldSkipReconcileResourcesWhenJobStatusIsDone() {
	donePhases := []radixv1.RadixScheduledJobPhase{radixv1.ScheduledJobPhaseFailed, radixv1.ScheduledJobPhaseStopped, radixv1.ScheduledJobPhaseSucceeded}

	for i, phase := range donePhases {
		s.Run(string(phase), func() {
			jobName, namespace := fmt.Sprintf("any-job-%d", i), "any-ns"
			expectedStatus := radixv1.RadixScheduledJobStatus{Phase: phase}
			scheduledjob := &radixv1.RadixScheduledJob{
				ObjectMeta: metav1.ObjectMeta{Name: jobName},
				Status:     expectedStatus,
			}
			scheduledjob, err := s.radixClient.RadixV1().RadixScheduledJobs(namespace).Create(context.Background(), scheduledjob, metav1.CreateOptions{})
			s.Require().NoError(err)
			sut := s.createSyncer(scheduledjob)
			s.Require().NoError(sut.OnSync())
			scheduledjob, err = s.radixClient.RadixV1().RadixScheduledJobs(namespace).Get(context.Background(), jobName, metav1.GetOptions{})
			s.Require().NoError(err)
			s.Equal(expectedStatus, scheduledjob.Status)
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
	appName, rsjName, componentName, namespace, rdName := "any-app", "any-job", "compute", "any-ns", "any-rd"
	rsj := &radixv1.RadixScheduledJob{
		ObjectMeta: metav1.ObjectMeta{Name: rsjName},
		Spec: radixv1.RadixScheduledJobSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  componentName,
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
	rsj, err := s.radixClient.RadixV1().RadixScheduledJobs(namespace).Create(context.Background(), rsj, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)

	sut := s.createSyncer(rsj)
	err = sut.OnSync()
	s.Require().NoError(err)
	services, _ := s.kubeClient.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{})
	s.Len(services.Items, 1)
	s.Equal(rsjName, services.Items[0].Name)
	expectedServiceLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: componentName, kube.RadixJobNameLabel: rsjName, kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule}
	s.Equal(expectedServiceLabels, services.Items[0].Labels)
	s.Equal(ownerReference(rsj), services.Items[0].OwnerReferences)
	s.Equal(map[string]string{kube.RadixJobNameLabel: rsjName}, services.Items[0].Spec.Selector)
	s.ElementsMatch([]corev1.ServicePort{{Name: "port1", Port: 8000, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(8000)}, {Name: "port2", Port: 9000, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(9000)}}, services.Items[0].Spec.Ports)
}

func (s *syncerTestSuite) Test_JobStaticConfiguration() {
	appName, rsjName, componentName, namespace, rdName, imageName := "any-app", "any-job", "compute", "any-ns", "any-rd", "any-image"
	rsj := &radixv1.RadixScheduledJob{
		ObjectMeta: metav1.ObjectMeta{Name: rsjName},
		Spec: radixv1.RadixScheduledJobSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  componentName,
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
	rsj, err := s.radixClient.RadixV1().RadixScheduledJobs(namespace).Create(context.Background(), rsj, metav1.CreateOptions{})
	s.Require().NoError(err)
	rd, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.applyRadixDeploymentEnvVarsConfigMaps(s.kubeUtil, rd)
	sut := s.createSyncer(rsj)
	err = sut.OnSync()
	s.Require().NoError(err)
	jobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Len(jobs.Items, 1)
	s.Equal(rsjName, jobs.Items[0].Name)
	expectedJobLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: componentName, kube.RadixJobNameLabel: rsjName, kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule}
	s.Equal(expectedJobLabels, jobs.Items[0].Labels)
	expectedPodLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: componentName, kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule, kube.RadixJobNameLabel: rsjName}
	s.Equal(expectedPodLabels, jobs.Items[0].Spec.Template.Labels)
	s.Equal(ownerReference(rsj), jobs.Items[0].OwnerReferences)
	s.Equal(numbers.Int32Ptr(0), jobs.Items[0].Spec.BackoffLimit)
	s.Equal(corev1.RestartPolicyNever, jobs.Items[0].Spec.Template.Spec.RestartPolicy)
	s.Equal(imageName, jobs.Items[0].Spec.Template.Spec.Containers[0].Image)
	s.Len(jobs.Items[0].Spec.Template.Spec.Containers[0].Env, 5)

	s.True(slice.Any(jobs.Items[0].Spec.Template.Spec.Containers[0].Env, func(env corev1.EnvVar) bool {
		return env.Name == "VAR1" && env.ValueFrom.ConfigMapKeyRef.Key == "VAR1" && env.ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name == kube.GetEnvVarsConfigMapName(componentName)
	}))
	s.True(slice.Any(jobs.Items[0].Spec.Template.Spec.Containers[0].Env, func(env corev1.EnvVar) bool {
		return env.Name == "VAR2" && env.ValueFrom.ConfigMapKeyRef.Key == "VAR2" && env.ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name == kube.GetEnvVarsConfigMapName(componentName)
	}))
	s.True(slice.Any(jobs.Items[0].Spec.Template.Spec.Containers[0].Env, func(env corev1.EnvVar) bool {
		return env.Name == "SECRET1" && env.ValueFrom.SecretKeyRef.Key == "SECRET1" && env.ValueFrom.SecretKeyRef.LocalObjectReference.Name == utils.GetComponentSecretName(componentName)
	}))
	s.True(slice.Any(jobs.Items[0].Spec.Template.Spec.Containers[0].Env, func(env corev1.EnvVar) bool {
		return env.Name == "SECRET2" && env.ValueFrom.SecretKeyRef.Key == "SECRET2" && env.ValueFrom.SecretKeyRef.LocalObjectReference.Name == utils.GetComponentSecretName(componentName)
	}))
	s.True(slice.Any(jobs.Items[0].Spec.Template.Spec.Containers[0].Env, func(env corev1.EnvVar) bool {
		return env.Name == defaults.RadixScheduleJobNameEnvironmentVariable && env.Value == rsjName
	}))
	s.Equal(corev1.PullAlways, jobs.Items[0].Spec.Template.Spec.Containers[0].ImagePullPolicy)
	s.Equal("default", jobs.Items[0].Spec.Template.Spec.ServiceAccountName)
	s.Equal(utils.BoolPtr(false), jobs.Items[0].Spec.Template.Spec.AutomountServiceAccountToken)
	s.Nil(jobs.Items[0].Spec.Template.Spec.Affinity.NodeAffinity)
	s.Len(jobs.Items[0].Spec.Template.Spec.Tolerations, 0)
	s.Len(jobs.Items[0].Spec.Template.Spec.Volumes, 0)
	s.Len(jobs.Items[0].Spec.Template.Spec.Containers[0].VolumeMounts, 0)
	services, err := s.kubeClient.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{})
	s.Require().NoError(err)
	s.Len(services.Items, 0)
}

func (s *syncerTestSuite) Test_JobWithIdentity() {
	appName, rsjName, componentName, namespace, rdName := "any-app", "any-job", "compute", "any-ns", "any-rd"
	rsj := &radixv1.RadixScheduledJob{
		ObjectMeta: metav1.ObjectMeta{Name: rsjName},
		Spec: radixv1.RadixScheduledJobSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  componentName,
			},
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
	rsj, err := s.radixClient.RadixV1().RadixScheduledJobs(namespace).Create(context.Background(), rsj, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)

	sut := s.createSyncer(rsj)
	err = sut.OnSync()
	s.Require().NoError(err)
	jobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Len(jobs.Items, 1)
	expectedPodLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: componentName, kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule, kube.RadixJobNameLabel: rsjName, "azure.workload.identity/use": "true"}
	s.Equal(expectedPodLabels, jobs.Items[0].Spec.Template.Labels)
	s.Equal(utils.GetComponentServiceAccountName(componentName), jobs.Items[0].Spec.Template.Spec.ServiceAccountName)
	s.Equal(utils.BoolPtr(false), jobs.Items[0].Spec.Template.Spec.AutomountServiceAccountToken)
}

func (s *syncerTestSuite) Test_JobWithPayload() {
	appName, rsjName, componentName, namespace, rdName, payloadPath, secretName, secretKey := "any-app", "any-job", "compute", "any-ns", "any-rd", "/mnt/path", "any-payload-secret", "any-payload-key"
	rsj := &radixv1.RadixScheduledJob{
		ObjectMeta: metav1.ObjectMeta{Name: rsjName},
		Spec: radixv1.RadixScheduledJobSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  componentName,
			},
			PayloadSecretRef: &radixv1.PayloadSecretKeySelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: secretName},
				Key:                  secretKey,
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
	rsj, err := s.radixClient.RadixV1().RadixScheduledJobs(namespace).Create(context.Background(), rsj, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	s.Require().NoError(err)

	sut := s.createSyncer(rsj)
	err = sut.OnSync()
	s.Require().NoError(err)
	jobs, _ := s.kubeClient.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.Len(jobs.Items, 1)
	s.Equal(rsjName, jobs.Items[0].Name)
	s.Len(jobs.Items[0].Spec.Template.Spec.Volumes, 1)
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

// All resources (jobs and services) created according to spec - split job features (volumes, keyvaults etc)
// Phase Waiting
// Phase Running
// Phase Completed
// Phase Failed
// Reason and Status from latest Pod when Phase in Failed or Waiting

// Delete Job when stop is set to true
// Missing RD => status pending
// RD exist but missing job

//

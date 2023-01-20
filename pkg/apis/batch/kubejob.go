package batch

import (
	"context"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/securitycontext"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	jobPayloadVolumeName = "job-payload"
)

func (s *syncer) reconcileJob(rd *radixv1.RadixDeployment, jobComponent *radixv1.RadixDeployJobComponent) error {
	existingJobs, err := s.kubeutil.ListJobsWithSelector(s.batch.GetNamespace(), s.batchIdentifierLabel().String())
	if err != nil {
		return err
	}

	for _, batchjob := range s.batch.Spec.Jobs {
		if isBatchJobStopRequested(batchjob) {
			for _, jobToDelete := range slice.FindAll(existingJobs, func(job *batchv1.Job) bool { return isResourceLabeledWithBatchJobName(batchjob.Name, job) }) {
				err := s.kubeclient.BatchV1().Jobs(jobToDelete.GetNamespace()).Delete(context.Background(), jobToDelete.GetName(), metav1.DeleteOptions{PropagationPolicy: pointers.Ptr(metav1.DeletePropagationBackground)})
				if err != nil && !errors.IsNotFound(err) {
					return err
				}
			}
			continue
		}

		if !slice.Any(existingJobs, func(job *batchv1.Job) bool { return isResourceLabeledWithBatchJobName(batchjob.Name, job) }) {
			job, err := s.buildJob(batchjob, jobComponent, rd)
			if err != nil {
				return err
			}

			if _, err := s.kubeclient.BatchV1().Jobs(s.batch.GetNamespace()).Create(context.TODO(), job, metav1.CreateOptions{}); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *syncer) buildJob(batchJob radixv1.RadixBatchJob, jobComponent *radixv1.RadixDeployJobComponent, rd *radixv1.RadixDeployment) (*batchv1.Job, error) {
	podLabels := radixlabels.Merge(
		s.batchIdentifierLabel(),
		radixlabels.ForBatchJobName("jobname"),
		radixlabels.ForPodWithRadixIdentity(jobComponent.Identity),
		radixlabels.ForBatchJobName(batchJob.Name),
	)
	jobLabels := radixlabels.Merge(
		s.batchIdentifierLabel(),
		radixlabels.ForBatchJobName(batchJob.Name),
	)

	volumes, err := s.getVolumes(rd.GetNamespace(), rd.Spec.Environment, batchJob, jobComponent, rd.Name)
	if err != nil {
		return nil, err
	}

	containers, err := s.getContainers(rd, jobComponent, batchJob)
	if err != nil {
		return nil, err
	}

	timeLimitSeconds := jobComponent.GetTimeLimitSeconds()
	if batchJob.TimeLimitSeconds != nil {
		timeLimitSeconds = batchJob.TimeLimitSeconds
	}

	node := jobComponent.GetNode()
	if batchJob.Node != nil {
		node = batchJob.Node
	}

	affinity := operatorUtils.GetPodSpecAffinity(node, rd.Spec.AppName, jobComponent.GetName())
	tolerations := operatorUtils.GetPodSpecTolerations(node)
	serviceAccountSpec := deployment.NewServiceAccountSpec(rd, jobComponent)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            getKubeJobName(s.batch.GetName(), batchJob.Name),
			Labels:          jobLabels,
			OwnerReferences: ownerReference(s.batch),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: numbers.Int32Ptr(0),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
				Spec: corev1.PodSpec{
					Containers:                   containers,
					Volumes:                      volumes,
					SecurityContext:              securitycontext.Pod(securitycontext.WithPodSeccompProfile(corev1.SeccompProfileTypeRuntimeDefault)),
					RestartPolicy:                corev1.RestartPolicyNever,
					ImagePullSecrets:             rd.Spec.ImagePullSecrets,
					Affinity:                     affinity,
					Tolerations:                  tolerations,
					ActiveDeadlineSeconds:        timeLimitSeconds,
					ServiceAccountName:           serviceAccountSpec.ServiceAccountName(),
					AutomountServiceAccountToken: serviceAccountSpec.AutomountServiceAccountToken(),
				},
			},
		},
	}

	return job, nil
}

func (s *syncer) getVolumes(namespace, environment string, batchJob radixv1.RadixBatchJob, radixJobComponent *radixv1.RadixDeployJobComponent, radixDeploymentName string) ([]corev1.Volume, error) {
	volumes, err := deployment.GetVolumes(s.kubeclient, s.kubeutil, namespace, environment, radixJobComponent, radixDeploymentName)
	if err != nil {
		return nil, err
	}

	if radixJobComponent.Payload != nil && batchJob.PayloadSecretRef != nil {
		volumes = append(volumes, corev1.Volume{
			Name: jobPayloadVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: batchJob.PayloadSecretRef.Name,
					Items: []corev1.KeyToPath{
						{Key: batchJob.PayloadSecretRef.Key, Path: "payload"},
					},
				},
			},
		})
	}

	return volumes, nil
}

func (s *syncer) getContainers(rd *radixv1.RadixDeployment, jobComponent *radixv1.RadixDeployJobComponent, batchJob radixv1.RadixBatchJob) ([]corev1.Container, error) {
	volumeMounts, err := s.getContainerVolumeMounts(batchJob, jobComponent, rd.GetName())
	if err != nil {
		return nil, err
	}
	environmentVariables, err := s.getContainerEnvironmentVariables(rd, jobComponent)
	if err != nil {
		return nil, err
	}
	ports := getContainerPorts(jobComponent)
	resources := s.getContainerResources(jobComponent, batchJob)

	container := corev1.Container{
		Name:            jobComponent.Name,
		Image:           jobComponent.Image,
		ImagePullPolicy: corev1.PullAlways,
		Env:             environmentVariables,
		Ports:           ports,
		VolumeMounts:    volumeMounts,
		SecurityContext: securitycontext.Container(),
		Resources:       resources,
	}

	return []corev1.Container{container}, nil
}

func (s *syncer) getContainerEnvironmentVariables(rd *radixv1.RadixDeployment, jobComponent *radixv1.RadixDeployJobComponent) ([]corev1.EnvVar, error) {
	environmentVariables, err := deployment.GetEnvironmentVariablesForRadixOperator(s.kubeutil, rd.Spec.AppName, rd, jobComponent)
	if err != nil {
		return nil, err
	}
	environmentVariables = append(environmentVariables, corev1.EnvVar{Name: defaults.RadixScheduleJobNameEnvironmentVariable, Value: s.batch.GetName()})
	return environmentVariables, nil
}

func (s *syncer) getContainerResources(jobComponent *radixv1.RadixDeployJobComponent, batchJob radixv1.RadixBatchJob) corev1.ResourceRequirements {
	if batchJob.Resources != nil {
		return operatorUtils.BuildResourceRequirement(batchJob.Resources)
	} else {
		return operatorUtils.GetResourceRequirements(jobComponent)
	}
}

func getContainerPorts(radixJobComponent *radixv1.RadixDeployJobComponent) []corev1.ContainerPort {
	var ports []corev1.ContainerPort
	for _, v := range radixJobComponent.Ports {
		containerPort := corev1.ContainerPort{
			Name:          v.Name,
			ContainerPort: v.Port,
		}
		ports = append(ports, containerPort)
	}
	return ports
}

func (s *syncer) getContainerVolumeMounts(batchJob radixv1.RadixBatchJob, radixJobComponent *radixv1.RadixDeployJobComponent, radixDeploymentName string) ([]corev1.VolumeMount, error) {
	volumeMounts, err := deployment.GetRadixDeployComponentVolumeMounts(radixJobComponent, radixDeploymentName)
	if err != nil {
		return nil, err
	}
	if radixJobComponent.Payload != nil && batchJob.PayloadSecretRef != nil {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      jobPayloadVolumeName,
			ReadOnly:  true,
			MountPath: radixJobComponent.Payload.Path,
		})
	}

	return volumeMounts, nil
}

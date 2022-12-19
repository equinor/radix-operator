package scheduledjob

import (
	"context"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/securitycontext"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	jobPayloadVolumeName = "job-payload"
)

func (s *syncer) reconcileJob() error {
	existingJobs, err := s.kubeclient.BatchV1().Jobs(s.radixScheduledJob.GetNamespace()).List(context.TODO(), metav1.ListOptions{LabelSelector: s.scheduledJobLabelIdentifier().String()})
	if err != nil {
		return err
	}
	if len(existingJobs.Items) > 0 {
		return nil
	}
	rd, jobComponent, err := s.getRadixDeploymentAndJobComponent()
	if err != nil {
		return err
	}
	job, err := s.buildJob(jobComponent, rd)
	if err != nil {
		return err
	}
	_, err = s.kubeclient.BatchV1().Jobs(s.radixScheduledJob.GetNamespace()).Create(context.TODO(), job, metav1.CreateOptions{})
	return err
}

func (s *syncer) buildJob(jobComponent *radixv1.RadixDeployJobComponent, rd *radixv1.RadixDeployment) (*batchv1.Job, error) {
	podLabels := radixlabels.ForPodWithRadixIdentity(jobComponent.Identity)
	jobLabels := radixlabels.Merge(
		s.scheduledJobLabelIdentifier(),
		radixlabels.ForApplicationName(rd.Spec.AppName),
		radixlabels.ForComponentName(jobComponent.Name),
		radixlabels.ForJobType(kube.RadixJobTypeJobSchedule),
		radixlabels.ForJobId(s.radixScheduledJob.Spec.JobId),
	)

	volumes, err := s.getVolumes(rd.GetNamespace(), rd.Spec.Environment, jobComponent, rd.Name)
	if err != nil {
		return nil, err
	}

	containers, err := s.getContainers(rd, jobComponent)
	if err != nil {
		return nil, err
	}

	timeLimitSeconds := jobComponent.GetTimeLimitSeconds()
	if s.radixScheduledJob.Spec.TimeLimitSeconds != nil {
		timeLimitSeconds = s.radixScheduledJob.Spec.TimeLimitSeconds
	}

	node := jobComponent.GetNode()
	if s.radixScheduledJob.Spec.Node != nil {
		node = s.radixScheduledJob.Spec.Node
	}

	affinity := operatorUtils.GetPodSpecAffinity(node, rd.Spec.AppName, jobComponent.GetName())
	tolerations := operatorUtils.GetPodSpecTolerations(node)
	serviceAccountSpec := deployment.NewServiceAccountSpec(rd, jobComponent)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            s.radixScheduledJob.GetName(),
			Labels:          jobLabels,
			OwnerReferences: ownerReference(s.radixScheduledJob),
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

func (s *syncer) getVolumes(namespace, environment string, radixJobComponent *radixv1.RadixDeployJobComponent, radixDeploymentName string) ([]corev1.Volume, error) {
	volumes, err := deployment.GetVolumes(s.kubeclient, s.kubeutil, namespace, environment, radixJobComponent, radixDeploymentName)
	if err != nil {
		return nil, err
	}

	if s.radixScheduledJob.Spec.PayloadSecretRef != nil {
		volumes = append(volumes, corev1.Volume{
			Name: jobPayloadVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: s.radixScheduledJob.Spec.PayloadSecretRef.Name,
					Items: []corev1.KeyToPath{
						{Key: s.radixScheduledJob.Spec.PayloadSecretRef.Key, Path: "payload"},
					},
				},
			},
		})
	}

	return volumes, nil
}

func (s *syncer) getContainers(rd *radixv1.RadixDeployment, jobComponent *radixv1.RadixDeployJobComponent) ([]corev1.Container, error) {
	volumeMounts, err := s.getContainerVolumeMounts(jobComponent, rd.GetName())
	if err != nil {
		return nil, err
	}
	environmentVariables, err := s.getContainerEnvironmentVariables(rd, jobComponent)
	if err != nil {
		return nil, err
	}
	ports := getContainerPorts(jobComponent)
	resources := s.getContainerResources(jobComponent)

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
	environmentVariables = append(environmentVariables, corev1.EnvVar{Name: defaults.RadixScheduleJobNameEnvironmentVariable, Value: s.radixScheduledJob.GetName()})
	return environmentVariables, nil
}

func (s *syncer) getContainerResources(jobComponent *radixv1.RadixDeployJobComponent) corev1.ResourceRequirements {
	if s.radixScheduledJob.Spec.Resources != nil {
		return operatorUtils.BuildResourceRequirement(s.radixScheduledJob.Spec.Resources)
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

func (s *syncer) getContainerVolumeMounts(radixJobComponent *radixv1.RadixDeployJobComponent, radixDeploymentName string) ([]corev1.VolumeMount, error) {
	volumeMounts, err := deployment.GetRadixDeployComponentVolumeMounts(radixJobComponent, radixDeploymentName)
	if err != nil {
		return nil, err
	}
	if s.radixScheduledJob.Spec.PayloadSecretRef != nil {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      jobPayloadVolumeName,
			ReadOnly:  true,
			MountPath: radixJobComponent.Payload.Path,
		})
	}

	return volumeMounts, nil
}

func jobNameLabelSelector(jobName string) labels.Set {
	return labels.Set{kubernetesJobNameLabel: jobName}
}

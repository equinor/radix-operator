package batch

import (
	"context"
	"fmt"
	"strings"

	"github.com/equinor/radix-common/utils/numbers"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/runtime"
	"github.com/equinor/radix-operator/pkg/apis/securitycontext"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/pkg/apis/volumemount"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/retry"
)

const (
	jobPayloadVolumeName = "job-payload"
)

func (s *syncer) reconcileKubeJob(ctx context.Context, batchJob *radixv1.RadixBatchJob, rd *radixv1.RadixDeployment, jobComponent *radixv1.RadixDeployJobComponent, existingJobs []*batchv1.Job, actualVolumesGetter func() ([]corev1.Volume, error)) error {
	batchJobKubeJobs := slice.FindAll(existingJobs, isKubeJobForBatchJob(batchJob))

	if isBatchJobStopRequested(batchJob) {
		// Delete existing k8s job if stop is requested for batch job
		return s.deleteJobs(ctx, batchJobKubeJobs)
	}

	requiresRestart := s.jobRequiresRestart(*batchJob)
	if !requiresRestart && (s.isBatchJobDone(batchJob.Name) || len(batchJobKubeJobs) > 0) {
		return nil
	}

	if requiresRestart {
		s.restartedJobs[batchJob.Name] = *batchJob
		if err := s.deleteJobs(ctx, batchJobKubeJobs); err != nil {
			return err
		}
	}

	if err := s.validatePayloadSecretReference(ctx, batchJob, jobComponent); err != nil {
		return err
	}

	volumes, err := actualVolumesGetter()
	if err != nil {
		return err
	}
	job, err := s.buildJob(ctx, batchJob, jobComponent, rd, volumes)
	if err != nil {
		return err
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err = s.kubeClient.BatchV1().Jobs(s.radixBatch.GetNamespace()).Create(ctx, job, metav1.CreateOptions{})
		return err
	})
}

func (s *syncer) validatePayloadSecretReference(ctx context.Context, batchJob *radixv1.RadixBatchJob, jobComponent *radixv1.RadixDeployJobComponent) error {
	if batchJob.PayloadSecretRef == nil {
		return nil
	}
	payloadSecret, err := s.kubeClient.CoreV1().Secrets(s.radixBatch.GetNamespace()).Get(ctx, batchJob.PayloadSecretRef.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if !radixlabels.GetRadixBatchDescendantsSelector(jobComponent.GetName()).Matches(labels.Set(payloadSecret.GetLabels())) {
		return fmt.Errorf("secret %s, referenced in the job %s of the batch %s is not valid payload secret", batchJob.PayloadSecretRef.Name, batchJob.Name, s.radixBatch.GetName())
	}
	if len(payloadSecret.Data) == 0 {
		return fmt.Errorf("payload secret %s, in the job %s of the batch %s is empty", batchJob.PayloadSecretRef.Name, batchJob.Name, s.radixBatch.GetName())
	}
	if _, ok := payloadSecret.Data[batchJob.PayloadSecretRef.Key]; !ok {
		return fmt.Errorf("payload secret %s, in the job %s of the batch %s has no entry %s for the job", batchJob.PayloadSecretRef.Name, batchJob.Name, s.radixBatch.GetName(), batchJob.PayloadSecretRef.Key)
	}
	return nil
}

func (s *syncer) deleteJobs(ctx context.Context, jobsToDelete []*batchv1.Job) error {
	for _, jobToDelete := range jobsToDelete {
		err := s.kubeClient.BatchV1().Jobs(jobToDelete.GetNamespace()).Delete(ctx, jobToDelete.GetName(), metav1.DeleteOptions{PropagationPolicy: pointers.Ptr(metav1.DeletePropagationBackground)})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (s *syncer) buildJob(ctx context.Context, batchJob *radixv1.RadixBatchJob, jobComponent *radixv1.RadixDeployJobComponent, rd *radixv1.RadixDeployment, volumes []corev1.Volume) (*batchv1.Job, error) {
	jobLabels := s.batchJobIdentifierLabel(batchJob.Name, rd.Spec.AppName)
	podLabels := radixlabels.Merge(
		jobLabels,
		radixlabels.ForPodWithRadixIdentity(jobComponent.Identity),
	)
	podAnnotations := annotations.ForClusterAutoscalerSafeToEvict(false)

	kubeJobName := getKubeJobName(s.radixBatch.GetName(), batchJob.Name)
	containers, err := s.getContainers(ctx, rd, jobComponent, batchJob, kubeJobName)
	if err != nil {
		return nil, err
	}

	timeLimitSeconds := jobComponent.TimeLimitSeconds
	if batchJob.TimeLimitSeconds != nil {
		timeLimitSeconds = batchJob.TimeLimitSeconds
	}

	node := jobComponent.GetNode()
	if batchJob.Node != nil { // nolint:staticcheck // SA1019: Ignore linting deprecated fields
		node = batchJob.Node // nolint:staticcheck // SA1019: Ignore linting deprecated fields
	}
	jobRuntime := jobComponent.GetRuntime()
	if batchJob.Runtime != nil {
		jobRuntime = batchJob.Runtime
	}
	nodeType := jobRuntime.GetNodeType()
	nodeArch := runtime.GetArchitectureFromRuntimeOrDefault(jobRuntime)

	backoffLimit := jobComponent.BackoffLimit
	if batchJob.BackoffLimit != nil {
		backoffLimit = batchJob.BackoffLimit
	}
	if backoffLimit == nil {
		backoffLimit = numbers.Int32Ptr(0)
	}

	failurePolicy := operatorUtils.GetPodFailurePolicy(jobComponent.FailurePolicy)
	if batchJob.FailurePolicy != nil {
		failurePolicy = operatorUtils.GetPodFailurePolicy(batchJob.FailurePolicy)
	}

	serviceAccountSpec := deployment.NewServiceAccountSpec(rd, jobComponent)
	volumes = s.appendPayloadSecretVolumes(batchJob, jobComponent, volumes)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            kubeJobName,
			Labels:          jobLabels,
			OwnerReferences: ownerReference(s.radixBatch),
			Annotations:     annotations.ForKubernetesDeploymentObservedGeneration(rd),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:     backoffLimit,
			PodFailurePolicy: failurePolicy,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      podLabels,
					Annotations: podAnnotations,
				},
				Spec: corev1.PodSpec{
					Containers:                   containers,
					Volumes:                      volumes,
					SecurityContext:              securitycontext.Pod(securitycontext.WithPodSeccompProfile(corev1.SeccompProfileTypeRuntimeDefault)),
					RestartPolicy:                corev1.RestartPolicyNever,
					ImagePullSecrets:             s.getJobPodImagePullSecrets(rd),
					Affinity:                     operatorUtils.GetAffinityForBatchJob(ctx, node, nodeType, nodeArch),
					Tolerations:                  operatorUtils.GetScheduledJobPodSpecTolerations(node, nodeType),
					ActiveDeadlineSeconds:        timeLimitSeconds,
					ServiceAccountName:           serviceAccountSpec.ServiceAccountName(),
					AutomountServiceAccountToken: serviceAccountSpec.AutomountServiceAccountToken(),
				},
			},
			TTLSecondsAfterFinished: pointers.Ptr(int32(86400)), // delete completed job after 24 hours
		},
	}
	return job, nil
}

func (s *syncer) getJobPodImagePullSecrets(rd *radixv1.RadixDeployment) []corev1.LocalObjectReference {
	imagePullSecrets := rd.Spec.ImagePullSecrets
	if s.config != nil {
		imagePullSecrets = append(imagePullSecrets, s.config.ContainerRegistryConfig.ImagePullSecretsFromExternalRegistryAuth()...)
	}
	return imagePullSecrets
}

func (s *syncer) appendPayloadSecretVolumes(batchJob *radixv1.RadixBatchJob, radixJobComponent *radixv1.RadixDeployJobComponent, volumes []corev1.Volume) []corev1.Volume {
	if radixJobComponent.Payload == nil || batchJob.PayloadSecretRef == nil {
		return volumes
	}
	return append(volumes, corev1.Volume{
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

func (s *syncer) getContainers(ctx context.Context, rd *radixv1.RadixDeployment, jobComponent *radixv1.RadixDeployJobComponent, batchJob *radixv1.RadixBatchJob, kubeJobName string) ([]corev1.Container, error) {
	volumeMounts, err := s.getContainerVolumeMounts(batchJob, jobComponent, rd.GetName())
	if err != nil {
		return nil, err
	}
	environmentVariables, err := s.getContainerEnvironmentVariables(ctx, rd, jobComponent, batchJob, kubeJobName)
	if err != nil {
		return nil, err
	}
	ports := getContainerPorts(jobComponent)
	resources, err := s.getContainerResources(batchJob, jobComponent)
	if err != nil {
		return nil, err
	}

	image := getJobImage(jobComponent, batchJob)
	securityContext := securitycontext.Container(securitycontext.WithContainerSeccompProfileType(corev1.SeccompProfileTypeRuntimeDefault), securitycontext.WithReadOnlyRootFileSystem(jobComponent.GetReadOnlyFileSystem()))
	container := corev1.Container{
		Name:            jobComponent.Name,
		Image:           image,
		ImagePullPolicy: corev1.PullAlways,
		Env:             environmentVariables,
		Ports:           ports,
		VolumeMounts:    volumeMounts,
		SecurityContext: securityContext,
		Resources:       resources,
		Command:         getJobCommand(jobComponent, batchJob),
		Args:            getJobArgs(jobComponent, batchJob),
	}

	return []corev1.Container{container}, nil
}

func getJobArgs(jobComponent *radixv1.RadixDeployJobComponent, batchJob *radixv1.RadixBatchJob) []string {
	if batchJob.Args != nil {
		return *batchJob.Args
	}
	return jobComponent.Args
}

func getJobCommand(jobComponent *radixv1.RadixDeployJobComponent, batchJob *radixv1.RadixBatchJob) []string {
	if batchJob.Command != nil {
		return *batchJob.Command
	}
	return jobComponent.Command
}

func getJobImage(jobComponent *radixv1.RadixDeployJobComponent, batchJob *radixv1.RadixBatchJob) string {
	image := jobComponent.Image
	if batchJob.Image != "" {
		image = batchJob.Image
	}
	if batchJob.ImageTagName == "" {
		return image
	}
	tagSeparatorIndex := strings.LastIndex(image, ":")
	lastSlashIndex := strings.LastIndex(image, "/")
	if tagSeparatorIndex > 0 && (lastSlashIndex < 0 || lastSlashIndex < tagSeparatorIndex) {
		image = image[:tagSeparatorIndex]
	}
	return fmt.Sprintf("%s:%s", image, batchJob.ImageTagName)
}

func (s *syncer) getContainerEnvironmentVariables(ctx context.Context, rd *radixv1.RadixDeployment, jobComponent *radixv1.RadixDeployJobComponent, batchJob *radixv1.RadixBatchJob, kubeJobName string) ([]corev1.EnvVar, error) {
	environmentVariables, err := deployment.GetEnvironmentVariablesForRadixOperator(ctx, s.kubeUtil, rd.Spec.AppName, rd, jobComponent)
	if err != nil {
		return nil, err
	}
	environmentVariables = append(environmentVariables, corev1.EnvVar{Name: defaults.RadixScheduleJobNameEnvironmentVariable, Value: kubeJobName})
	environmentVariables = applyBatchJobEnvironmentVariables(batchJob, environmentVariables)
	return environmentVariables, nil
}

func applyBatchJobEnvironmentVariables(batchJob *radixv1.RadixBatchJob, componentEnvVars []corev1.EnvVar) []corev1.EnvVar {
	if len(batchJob.Variables) == 0 {
		return componentEnvVars
	}
	appliedJobEnvVarNames := make(map[string]struct{})
	for i, componentEnvVar := range componentEnvVars {
		if jobEnvVarValue, ok := batchJob.Variables[componentEnvVar.Name]; ok {
			componentEnvVars[i].Value = jobEnvVarValue
			appliedJobEnvVarNames[componentEnvVar.Name] = struct{}{}
		}
	}
	for varName, varValue := range batchJob.Variables {
		if _, ok := appliedJobEnvVarNames[varName]; !ok {
			componentEnvVars = append(componentEnvVars, corev1.EnvVar{Name: varName, Value: varValue})
		}
	}
	return componentEnvVars
}

func (s *syncer) getContainerResources(batchJob *radixv1.RadixBatchJob, jobComponent *radixv1.RadixDeployJobComponent) (corev1.ResourceRequirements, error) {
	if batchJob.Resources != nil {
		return operatorUtils.BuildResourceRequirement(batchJob.Resources)
	}

	return operatorUtils.GetResourceRequirements(jobComponent)
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

func (s *syncer) getContainerVolumeMounts(batchJob *radixv1.RadixBatchJob, radixJobComponent *radixv1.RadixDeployJobComponent, radixDeploymentName string) ([]corev1.VolumeMount, error) {
	volumeMounts, err := volumemount.GetRadixDeployComponentVolumeMounts(radixJobComponent, radixDeploymentName)
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

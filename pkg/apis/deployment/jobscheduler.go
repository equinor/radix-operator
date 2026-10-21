package deployment

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	"github.com/equinor/radix-operator/pkg/apis/utils/kubemerge"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO:
// - Check Deployment Labels, Pod Labels and Selector labels for scheduelr and aux
// - Check Deployment Volumes and VolumeMounts for aux

const (
	auxContainerName       = "aux"
	schedulerContainerName = "scheduler"
)

func (deploy *Deployment) reconcileJobSchedulerComponent(ctx context.Context, jobComp v1.RadixDeployJobComponent) error {

	// Create service account to be used by batch job, if nessecary
	if err := deploy.createOrUpdateServiceAccount(ctx, &jobComp); err != nil {
		return fmt.Errorf("failed to create service account: %w", err)
	}

	if err := deploy.createOrUpdateJobSchedulerDeployment(ctx, jobComp); err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	if err := deploy.createOrUpdateServiceForJobScheduler(ctx, jobComp); err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	// Delete service for batch jobs if no longer defined in the spec
	if err := deploy.garbageCollectServiceAccountNoLongerInSpecForComponent(ctx, &jobComp); err != nil {
		return fmt.Errorf("failed to garbage collect service account: %w", err)
	}

	return nil
}

func (deploy *Deployment) createOrUpdateJobSchedulerDeployment(ctx context.Context, component v1.RadixDeployJobComponent) error {
	current, desired, err := deploy.getCurrentAndDesiredJobSchedulerDeployment(ctx, component)
	if err != nil {
		return err
	}

	return deploy.kubeutil.ApplyDeployment(ctx, deploy.radixDeployment.Namespace, current, desired)
}

func (deploy *Deployment) getCurrentAndDesiredJobSchedulerDeployment(ctx context.Context, component v1.RadixDeployJobComponent) (*appsv1.Deployment, *appsv1.Deployment, error) {
	currentDeployment, err := deploy.kubeutil.GetDeployment(ctx, deploy.radixDeployment.Namespace, component.GetName())
	if err != nil && !kubeerrors.IsNotFound(err) {
		return nil, nil, err
	}
	desiredDeployment, err := deploy.getDesiredJobSchedulerDeployment(component)
	if err != nil {
		return nil, nil, err
	}

	return currentDeployment, desiredDeployment, nil
}

func (deploy *Deployment) getDesiredJobSchedulerDeployment(component v1.RadixDeployJobComponent) (*appsv1.Deployment, error) {

	var imagePullSecrets []corev1.LocalObjectReference
	if deploy.cfg.Common.ExternalRegistryAuthSecret != "" {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: deploy.cfg.Common.ExternalRegistryAuthSecret})
	}

	env := []corev1.EnvVar{
		{
			Name:  "RADIX_APP",
			Value: deploy.registration.Name,
		},
		{
			Name:  "RADIX_COMPONENT",
			Value: component.Name,
		},
		{
			Name:  "RADIX_DEPLOYMENT",
			Value: deploy.radixDeployment.Name,
		},
		{
			Name:  "RADIX_ENVIRONMENT",
			Value: deploy.radixDeployment.Spec.Environment,
		},
	}
	if restartComponentValue, ok := component.GetEnvironmentVariables()[defaults.RadixRestartEnvironmentVariable]; ok {
		env = append(env, corev1.EnvVar{Name: defaults.RadixRestartEnvironmentVariable, Value: restartComponentValue})
	}

	podTemplate, err := kubemerge.MergePodTemplate(deploy.cfg.Runtime.PipelineRunnerTemplate.Base, deploy.cfg.Runtime.PipelineRunnerTemplate.Overlay, corev1.PodTemplateSpec{
		Labels: map[string]string{
			"app":       deploy.registration.Name,
			"component": component.Name,
		},
		Spec: corev1.PodSpec{
			ImagePullSecrets:             imagePullSecrets,
			ServiceAccountName:           defaults.RadixJobSchedulerServiceName,
			AutomountServiceAccountToken: new(true),
			Containers: []corev1.Container{
				{
					Name: schedulerContainerName,
					Ports: []corev1.ContainerPort{{
						ContainerPort: *component.SchedulerPort,
					}},
					Env: env,
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to merge pod spec for job scheduler %s: %w", component.Name, err)
	}

	desiredDeployment := &appsv1.Deployment{
		Name:            component.GetName(),
		Annotations:     annotations.ForKubernetesDeploymentObservedGeneration(deploy.radixDeployment),
		OwnerReferences: []metav1.OwnerReference{getOwnerReferenceOfDeployment(deploy.radixDeployment)},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
			Selector: &metav1.LabelSelector{
				// TODO:	MatchLabels: radixlabels.ForAuxOAuthProxyComponent(deploy.registration.Name(), component), //nolint:staticcheck
			},
			Template: podTemplate,
		},
	}

	return desiredDeployment, nil
}

func (deploy *Deployment) garbageCollectScheduledJobsNoLongerInSpec(ctx context.Context) error {
	jobs, err := deploy.kubeutil.ListJobs(ctx, deploy.radixDeployment.GetNamespace())
	if err != nil {
		return err
	}

	for _, job := range jobs {
		componentName, ok := RadixComponentNameFromComponentLabel(job)
		if !ok {
			continue
		}

		jobType, ok := NewRadixJobTypeFromObjectLabels(job)
		if !ok {
			continue
		}

		// Delete job if it originates from job-scheduler and is no longed defined in RD jobs section
		if jobType.IsJobScheduler() && !componentName.ExistInDeploymentSpecJobList(deploy.radixDeployment) {
			propagationPolicy := metav1.DeletePropagationBackground
			err = deploy.kubeclient.BatchV1().Jobs(deploy.radixDeployment.GetNamespace()).Delete(
				ctx,
				job.Name,
				metav1.DeleteOptions{
					PropagationPolicy: &propagationPolicy,
				})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) createOrUpdateServiceForJobScheduler(ctx context.Context, job v1.RadixDeployJobComponent) error {

	if job.SchedulerPort == nil {
		return nil
	}

	service := &corev1.Service{
		Name: job.GetName(),
		Labels: map[string]string{
			kube.RadixAppLabel:       deploy.registration.GetName(),
			kube.RadixComponentLabel: job.GetName(),
		},
		OwnerReferences: []metav1.OwnerReference{
			getOwnerReferenceOfDeployment(deploy.radixDeployment),
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				kube.RadixComponentLabel:         job.GetName(),
				kube.RadixPodIsJobSchedulerLabel: "true",
			},
			Ports: utils.GetServicePorts([]v1.ComponentPort{{Port: *job.SchedulerPort}}),
		},
	}
	return deploy.kubeutil.ApplyService(ctx, deploy.radixDeployment.Namespace, service)
}

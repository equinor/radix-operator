package deployment

import (
	"context"
	stderrors "errors"
	"fmt"
	"maps"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/securitycontext"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	radixannotations "github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	"github.com/equinor/radix-operator/pkg/apis/utils/kubemerge"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/rs/zerolog/log"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (deploy *Deployment) reconcileJobSchedulerComponent(ctx context.Context, component v1.RadixDeployJobComponent) error {

	// Create service account to be used by batch job, if nessecary
	if err := deploy.createOrUpdateServiceAccount(ctx, &component); err != nil {
		return fmt.Errorf("failed to create service account: %w", err)
	}

	if err := deploy.createOrUpdateJobSchedulerDeployment(ctx, component); err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	if err := deploy.createOrUpdateServiceForJobScheduler(ctx, component); err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	// Delete service for batch jobs if no longer defined in the spec
	if err := deploy.garbageCollectServiceAccountNoLongerInSpecForComponent(ctx, &component); err != nil {
		return fmt.Errorf("failed to garbage collect service account: %w", err)
	}

	if err := deploy.handleJobAuxDeployment(ctx, component); err != nil {
		return err
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
					Name: "scheduler",
					Ports: []corev1.ContainerPort{
						corev1.ContainerPort{
							ContainerPort: *component.SchedulerPort,
						},
					},
					Env: []corev1.EnvVar{
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
					},
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

func (deploy *Deployment) garbageCollectScheduledJobAuxDeploymentsNoLongerInSpec(ctx context.Context) error {
	jobAuxDeployments, err := deploy.kubeutil.ListDeploymentsWithSelector(ctx, deploy.radixDeployment.GetNamespace(), labels.IsJobAuxObjectSelector(kube.RadixJobTypeManagerAux).String())
	if err != nil {
		return err
	}
	var errs []error
	for _, deployment := range jobAuxDeployments {
		componentName, ok := RadixComponentNameFromAuxComponentLabel(deployment)
		if !ok {
			continue
		}

		// Delete job aux deployment if a job is no longed defined in RD job section
		if !componentName.ExistInDeploymentSpecJobList(deploy.radixDeployment) {
			err = deploy.kubeutil.DeleteDeployment(ctx, deploy.radixDeployment.GetNamespace(), deployment.Name)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	return stderrors.Join(errs...)
}

func (deploy *Deployment) createOrUpdateServiceForJobScheduler(ctx context.Context, job v1.RadixDeployJobComponent) error {

	if job.SchedulerPort == nil {
		return nil
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: job.GetName(),
			Labels: map[string]string{
				kube.RadixAppLabel:       deploy.registration.GetName(),
				kube.RadixComponentLabel: job.GetName(),
			},
			OwnerReferences: []metav1.OwnerReference{
				getOwnerReferenceOfDeployment(deploy.radixDeployment),
			},
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

func (deploy *Deployment) handleJobAuxDeployment(ctx context.Context, deployComponent v1.RadixDeployJobComponent) error {

	//TODO: Find acutall volumes for job aux deployment
	panic("HELLLO@!!!!")
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount

	currentJobAuxDeployment, desiredJobAuxDeployment, err := deploy.getCurrentAndDesiredJobAuxDeployment(ctx, deployComponent, volumes, volumeMounts)
	if err != nil {
		return err
	}

	if currentJobAuxDeployment != nil && !maps.Equal(currentJobAuxDeployment.Spec.Selector.MatchLabels, desiredJobAuxDeployment.Spec.Selector.MatchLabels) {
		log.Ctx(ctx).Info().Msgf("Deleting outdated deployment (label selector does not match) %s", currentJobAuxDeployment.GetName())
		if err = deploy.kubeutil.DeleteDeployment(ctx, deploy.radixDeployment.Namespace, currentJobAuxDeployment.Name); err != nil {
			return err
		}

		currentJobAuxDeployment = nil
	}

	return deploy.kubeutil.ApplyDeployment(ctx, deploy.radixDeployment.Namespace, currentJobAuxDeployment, desiredJobAuxDeployment)
}

func (deploy *Deployment) getCurrentAndDesiredJobAuxDeployment(ctx context.Context, deployComponent v1.RadixCommonDeployComponent, volumes []corev1.Volume, volumeMounts []corev1.VolumeMount) (*appsv1.Deployment, *appsv1.Deployment, error) {
	jobAuxKubeDeploymentName := defaults.GetJobAuxKubeDeployName(deployComponent.GetName())
	var imagePullSecrets []corev1.LocalObjectReference

	if deploy.cfg.Common.ExternalRegistryAuthSecret != "" {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: deploy.cfg.Common.ExternalRegistryAuthSecret})
	}

	var env []corev1.EnvVar
	if restartComponentValue, ok := deployComponent.GetEnvironmentVariables()[defaults.RadixRestartEnvironmentVariable]; ok {
		env = append(env, corev1.EnvVar{Name: defaults.RadixRestartEnvironmentVariable, Value: restartComponentValue})
	}

	desiredJobAuxDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            jobAuxKubeDeploymentName,
			OwnerReferences: []metav1.OwnerReference{getOwnerReferenceOfDeployment(deploy.radixDeployment)},
			Labels: radixlabels.Merge(
				radixlabels.ForApplicationName(deploy.registration.Name),
				radixlabels.ForJobAuxObject(deployComponent.GetName(), kube.RadixJobTypeManagerAux),
			),
			Annotations: radixannotations.ForKubernetesDeploymentObservedGeneration(deploy.radixDeployment),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: radixlabels.Merge(
					radixlabels.ForApplicationName(deploy.registration.Name),
					radixlabels.ForApplicationID(deploy.registration.Spec.AppID),
					radixlabels.ForJobAuxObject(deployComponent.GetName(), kube.RadixJobTypeManagerAux),
				),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: radixlabels.Merge(
						radixlabels.ForApplicationName(deploy.registration.Name),
						radixlabels.ForApplicationID(deploy.registration.Spec.AppID),
						radixlabels.ForPodWithRadixIdentity(deployComponent.GetIdentity()),
						radixlabels.ForJobAuxObject(deployComponent.GetName(), kube.RadixJobTypeManagerAux),
					),
				},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken: new(false),
					SecurityContext:              securitycontext.Pod(),
					ImagePullSecrets:             imagePullSecrets,
					ServiceAccountName:           (&radixComponentServiceAccountSpec{component: deployComponent}).ServiceAccountName(),
					Affinity:                     utils.GetAffinityForJobAPIAuxComponent(),
					Volumes:                      volumes,
					Containers: []corev1.Container{
						{
							Name:            jobAuxKubeDeploymentName,
							Image:           deploy.cfg.Operator.JobSchedulerAuxImage.String(),
							ImagePullPolicy: corev1.PullIfNotPresent,
							SecurityContext: securitycontext.Container(
								securitycontext.WithReadOnlyRootFileSystem(new(true)),
								securitycontext.WithContainerRunAsUser(1000),
							),
							Command:      []string{"sh"},
							Args:         []string{"-c", "echo 'start'; while true; do echo $(date);sleep 3600; done; echo 'exit'"},
							Resources:    getJobAuxResources(),
							VolumeMounts: volumeMounts,
							Env:          env,
						}},
				},
			},
		},
	}

	currentJobAuxDeployment, err := deploy.kubeutil.KubeClient().AppsV1().Deployments(deploy.radixDeployment.Namespace).Get(ctx, jobAuxKubeDeploymentName, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil, desiredJobAuxDeployment, nil
		}
		return nil, nil, err
	}
	return currentJobAuxDeployment, desiredJobAuxDeployment, nil
}

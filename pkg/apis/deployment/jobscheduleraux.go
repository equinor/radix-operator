package deployment

import (
	"context"
	stderrors "errors"
	"fmt"
	"maps"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixannotations "github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	"github.com/equinor/radix-operator/pkg/apis/utils/kubemerge"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/pkg/apis/volumemount"
	"github.com/rs/zerolog/log"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (deploy *Deployment) reconcileJobSchedulerAuxComponent(ctx context.Context, jobComp v1.RadixDeployJobComponent) error {

	volumes, err := volumemount.CreateOrUpdatePVCVolumeResourcesForDeployComponent(ctx, deploy.kubeutil.KubeClient(), deploy.radixDeployment, &jobComp, []corev1.Volume{})
	if err != nil {
		return err
	}

	mounts, err := volumemount.GetRadixDeployComponentVolumeMounts(&jobComp, deploy.radixDeployment.Name)
	if err != nil {
		return err
	}

	if err := deploy.handleJobAuxDeployment(ctx, jobComp, volumes, mounts); err != nil {
		return err
	}

	if err = volumemount.GarbageCollectCsiAzureVolumeResourcesForDeployComponent(ctx, deploy.kubeutil.KubeClient(), deploy.radixDeployment, deploy.radixDeployment.GetNamespace()); err != nil {
		return fmt.Errorf("failed to garbage collect persistent volumes or persistent volume claims: %w", err)
	}

	return nil
}

func (deploy *Deployment) garbageCollectScheduledJobAuxDeploymentsNoLongerInSpec(ctx context.Context) error {
	jobAuxDeployments, err := deploy.kubeutil.ListDeploymentsWithSelector(ctx, deploy.radixDeployment.GetNamespace(), radixlabels.IsJobAuxObjectSelector(kube.RadixJobTypeManagerAux).String())
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

func (deploy *Deployment) handleJobAuxDeployment(ctx context.Context, jobComp v1.RadixDeployJobComponent, volumes []corev1.Volume, mounts []corev1.VolumeMount) error {

	currentJobAuxDeployment, desiredJobAuxDeployment, err := deploy.getCurrentAndDesiredJobAuxDeployment(ctx, jobComp, volumes, mounts)
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

func (deploy *Deployment) getCurrentAndDesiredJobAuxDeployment(ctx context.Context, jobComp v1.RadixDeployJobComponent, volumes []corev1.Volume, mounts []corev1.VolumeMount) (*appsv1.Deployment, *appsv1.Deployment, error) {
	jobAuxKubeDeploymentName := defaults.GetJobAuxKubeDeployName(jobComp.GetName())

	var imagePullSecrets []corev1.LocalObjectReference
	if deploy.cfg.Common.ExternalRegistryAuthSecret != "" {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: deploy.cfg.Common.ExternalRegistryAuthSecret})
	}

	var env []corev1.EnvVar
	if restartComponentValue, ok := jobComp.GetEnvironmentVariables()[defaults.RadixRestartEnvironmentVariable]; ok {
		env = append(env, corev1.EnvVar{Name: defaults.RadixRestartEnvironmentVariable, Value: restartComponentValue})
	}

	podTemplate, err := kubemerge.MergePodTemplate(deploy.cfg.Runtime.PipelineRunnerTemplate.Base, deploy.cfg.Runtime.PipelineRunnerTemplate.Overlay, corev1.PodTemplateSpec{
		Labels: radixlabels.Merge(
			radixlabels.ForApplicationName(deploy.registration.Name),
			radixlabels.ForApplicationID(deploy.registration.Spec.AppID),
			radixlabels.ForPodWithRadixIdentity(jobComp.GetIdentity()),
			radixlabels.ForJobAuxObject(jobComp.GetName(), kube.RadixJobTypeManagerAux),
		),
		Spec: corev1.PodSpec{
			ImagePullSecrets: imagePullSecrets,
			Volumes:          volumes,
			Containers: []corev1.Container{
				{
					Name:         auxContainerName,
					VolumeMounts: mounts,
					Env:          env,
				}},
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to merge pod spec for job scheduler aux %s: %w", jobComp.GetName(), err)
	}

	desiredDeployment := &appsv1.Deployment{
		Name:            jobAuxKubeDeploymentName,
		OwnerReferences: []metav1.OwnerReference{getOwnerReferenceOfDeployment(deploy.radixDeployment)},
		Labels: radixlabels.Merge(
			radixlabels.ForApplicationName(deploy.registration.Name),
			radixlabels.ForJobAuxObject(jobComp.GetName(), kube.RadixJobTypeManagerAux),
		),
		Annotations: radixannotations.ForKubernetesDeploymentObservedGeneration(deploy.radixDeployment),
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: radixlabels.Merge(
					radixlabels.ForApplicationName(deploy.registration.Name),
					radixlabels.ForApplicationID(deploy.registration.Spec.AppID),
					radixlabels.ForJobAuxObject(jobComp.GetName(), kube.RadixJobTypeManagerAux),
				),
			},
			Template: podTemplate,
		},
	}

	currentJobAuxDeployment, err := deploy.kubeutil.KubeClient().AppsV1().Deployments(deploy.radixDeployment.Namespace).Get(ctx, jobAuxKubeDeploymentName, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil, desiredDeployment, nil
		}
		return nil, nil, err
	}
	return currentJobAuxDeployment, desiredDeployment, nil
}

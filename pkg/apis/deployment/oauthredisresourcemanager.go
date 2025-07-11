package deployment

import (
	"context"
	"fmt"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/securitycontext"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	oauthutil "github.com/equinor/radix-operator/pkg/apis/utils/oauth"
	"github.com/equinor/radix-operator/pkg/apis/utils/resources"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/cache"
)

const (
	redisPasswordEnvironmentVariable = "REDIS_PASSWORD"
)

// NewOAuthRedisResourceManager creates a new RedisResourceManager
func NewOAuthRedisResourceManager(rd *v1.RadixDeployment, rr *v1.RadixRegistration, kubeutil *kube.Kube, oauth2RedisDockerImage string) AuxiliaryResourceManager {
	return &oauthRedisResourceManager{
		rd:                    rd,
		rr:                    rr,
		kubeutil:              kubeutil,
		oauthRedisDockerImage: oauth2RedisDockerImage,
		logger:                log.Logger.With().Str("resource_kind", v1.KindRadixDeployment).Str("resource_name", cache.MetaObjectToName(&rd.ObjectMeta).String()).Str("aux", "oauth-redis").Logger(),
	}
}

type oauthRedisResourceManager struct {
	rd                    *v1.RadixDeployment
	rr                    *v1.RadixRegistration
	kubeutil              *kube.Kube
	oauthRedisDockerImage string
	logger                zerolog.Logger
}

func (o *oauthRedisResourceManager) Sync(ctx context.Context) error {
	for _, component := range o.rd.Spec.Components {
		if err := o.syncComponent(ctx, &component); err != nil {
			return fmt.Errorf("failed to sync oauth redis for component %s: %w", component.Name, err)
		}
	}
	return nil
}

func (o *oauthRedisResourceManager) syncComponent(ctx context.Context, component *v1.RadixDeployComponent) error {
	if auth := component.GetAuthentication(); component.IsPublic() && auth.GetOAuth2().IsSessionStoreTypeSystemManaged() {
		o.logger.Debug().Msgf("Sync system managed redis for the component %s", component.GetName())
		return o.install(ctx, component.DeepCopy())
	}
	return o.uninstall(ctx, component)
}

func (o *oauthRedisResourceManager) GarbageCollect(ctx context.Context) error {
	if err := o.garbageCollect(ctx); err != nil {
		return fmt.Errorf("failed to garbage collect redis: %w", err)
	}
	return nil
}

func (o *oauthRedisResourceManager) garbageCollect(ctx context.Context) error {
	if err := o.garbageCollectDeployment(ctx); err != nil {
		return fmt.Errorf("failed to garbage collect deployment: %w", err)
	}
	if err := o.garbageCollectServices(ctx); err != nil {
		return fmt.Errorf("failed to garbage collect services: %w", err)
	}
	return nil
}

func (o *oauthRedisResourceManager) garbageCollectDeployment(ctx context.Context) error {
	deployments, err := o.kubeutil.ListDeployments(ctx, o.rd.Namespace)
	if err != nil {
		return err
	}

	for _, deployment := range deployments {
		if o.isEligibleForGarbageCollection(deployment) {
			if err := o.kubeutil.KubeClient().AppsV1().Deployments(deployment.Namespace).Delete(ctx, deployment.Name, metav1.DeleteOptions{}); err != nil && !kubeerrors.IsNotFound(err) {
				return err
			}
			o.logger.Info().Msgf("Deleted deployment: %s in namespace %s", deployment.GetName(), deployment.Namespace)
		}
	}
	return nil
}

func (o *oauthRedisResourceManager) garbageCollectServices(ctx context.Context) error {
	services, err := o.kubeutil.ListServices(ctx, o.rd.Namespace)
	if err != nil {
		return err
	}

	for _, service := range services {
		if o.isEligibleForGarbageCollection(service) {
			if err := o.kubeutil.KubeClient().CoreV1().Services(service.Namespace).Delete(ctx, service.Name, metav1.DeleteOptions{}); err != nil && !kubeerrors.IsNotFound(err) {
				return err
			}
		}
	}
	return nil
}

func (o *oauthRedisResourceManager) isEligibleForGarbageCollection(object metav1.Object) bool {
	if appName := object.GetLabels()[kube.RadixAppLabel]; appName != o.rd.Spec.AppName {
		return false
	}
	if auxType := object.GetLabels()[kube.RadixAuxiliaryComponentTypeLabel]; auxType != v1.OAuthRedisAuxiliaryComponentType {
		return false
	}
	auxTargetComponentName, nameExist := RadixComponentNameFromAuxComponentLabel(object)
	if !nameExist {
		return false
	}
	return !auxTargetComponentName.ExistInDeploymentSpec(o.rd)
}

func (o *oauthRedisResourceManager) install(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	o.logger.Debug().Msgf("install the Redis for the component %s", component.GetName())
	if err := o.createOrUpdateService(ctx, component); err != nil {
		return err
	}
	return o.createOrUpdateDeployment(ctx, component)
}

func (o *oauthRedisResourceManager) uninstall(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	o.logger.Debug().Msgf("uninstall oauth redis for the component %s", component.GetName())
	if err := o.deleteDeployment(ctx, component); err != nil {
		return err
	}

	if err := o.deleteServices(ctx, component); err != nil {
		return err
	}
	return nil
}

func (o *oauthRedisResourceManager) deleteDeployment(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	selector := labels.SelectorFromValidatedSet(radixlabels.ForAuxOAuthRedisComponent(o.rd.Spec.AppName, component)).String()
	deployments, err := o.kubeutil.ListDeploymentsWithSelector(ctx, o.rd.Namespace, selector)
	if err != nil {
		return err
	}

	for _, deployment := range deployments {
		if err := o.kubeutil.KubeClient().AppsV1().Deployments(deployment.Namespace).Delete(ctx, deployment.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func (o *oauthRedisResourceManager) deleteServices(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	selector := labels.SelectorFromValidatedSet(radixlabels.ForAuxOAuthRedisComponent(o.rd.Spec.AppName, component)).String()
	services, err := o.kubeutil.ListServicesWithSelector(ctx, o.rd.Namespace, selector)
	if err != nil {
		return err
	}

	for _, service := range services {
		if err := o.kubeutil.KubeClient().CoreV1().Services(service.Namespace).Delete(ctx, service.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func (o *oauthRedisResourceManager) createOrUpdateService(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	service := o.buildServiceSpec(component)
	return o.kubeutil.ApplyService(ctx, o.rd.Namespace, service)
}

func (o *oauthRedisResourceManager) buildServiceSpec(component v1.RadixCommonDeployComponent) *corev1.Service {
	serviceName := utils.GetAuxOAuthRedisServiceName(component.GetName())
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            serviceName,
			OwnerReferences: []metav1.OwnerReference{getOwnerReferenceOfDeployment(o.rd)},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: radixlabels.ForAuxOAuthRedisComponent(o.rd.Spec.AppName, component),
			Ports: []corev1.ServicePort{
				{
					Port:       v1.OAuthRedisPortNumber,
					TargetPort: intstr.FromInt32(v1.OAuthRedisPortNumber),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	oauthutil.MergeAuxOAuthRedisComponentResourceLabels(service, o.rd.Spec.AppName, component)
	return service
}

func (o *oauthRedisResourceManager) createOrUpdateDeployment(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	current, desired, err := o.getCurrentAndDesiredDeployment(ctx, component)
	if err != nil {
		return err
	}

	return o.kubeutil.ApplyDeployment(ctx, o.rd.Namespace, current, desired, false)
}

func (o *oauthRedisResourceManager) getCurrentAndDesiredDeployment(ctx context.Context, component v1.RadixCommonDeployComponent) (*appsv1.Deployment, *appsv1.Deployment, error) {
	deploymentName := utils.GetAuxiliaryComponentDeploymentName(component.GetName(), v1.OAuthRedisAuxiliaryComponentSuffix)

	currentDeployment, err := o.kubeutil.GetDeployment(ctx, o.rd.Namespace, deploymentName)
	if err != nil && !kubeerrors.IsNotFound(err) {
		return nil, nil, err
	}
	desiredDeployment, err := o.getDesiredDeployment(component)
	if err != nil {
		return nil, nil, err
	}

	return currentDeployment, desiredDeployment, nil
}

func (o *oauthRedisResourceManager) getDesiredDeployment(component v1.RadixCommonDeployComponent) (*appsv1.Deployment, error) {
	componentName := component.GetName()
	deploymentName := utils.GetAuxiliaryComponentDeploymentName(componentName, v1.OAuthRedisAuxiliaryComponentSuffix)
	readinessProbe, err := getReadinessProbeWithDefaultsFromEnv(v1.OAuthRedisPortNumber)
	if err != nil {
		return nil, err
	}

	var replicas int32 = 1
	if isComponentStopped(component) || component.HasZeroReplicas() {
		replicas = 0
	}

	// Spec.Strategy defaults to RollingUpdate, ref https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#strategy
	const (
		volumeNameRedisData   = "redis-data"
		volumeNameRedisConfig = "redis-config"
		volumeNameRedisTmp    = "redis-tmp"
		volumeNameRedisRun    = "redis-run"
	)
	desiredDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            deploymentName,
			Annotations:     annotations.ForKubernetesDeploymentObservedGeneration(o.rd),
			OwnerReferences: []metav1.OwnerReference{getOwnerReferenceOfDeployment(o.rd)},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointers.Ptr(replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: radixlabels.ForAuxOAuthRedisComponent(o.rd.Spec.AppName, component),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: radixlabels.Merge(
						radixlabels.ForAuxOAuthRedisComponent(o.rd.Spec.AppName, component),
					),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            componentName,
							Image:           o.oauthRedisDockerImage,
							ImagePullPolicy: corev1.PullAlways,
							Env:             o.getEnvVars(component),
							Ports: []corev1.ContainerPort{
								{
									Name:          v1.OAuthRedisPortName,
									ContainerPort: v1.OAuthRedisPortNumber,
								},
							},
							ReadinessProbe: readinessProbe,
							SecurityContext: securitycontext.Container(
								securitycontext.WithContainerSeccompProfileType(corev1.SeccompProfileTypeRuntimeDefault),
								securitycontext.WithReadOnlyRootFileSystem(pointers.Ptr(true)),
								securitycontext.WithContainerRunAsUser(1001),
							),
							Resources: resources.New(resources.WithMemoryMega(100), resources.WithCPUMilli(10)),
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      volumeNameRedisData,
									MountPath: "/bitnami/redis/data",
								},
								{
									Name:      volumeNameRedisConfig,
									MountPath: "/opt/bitnami/redis/etc",
								},
								{
									Name:      volumeNameRedisTmp,
									MountPath: "/tmp",
								},
								{
									Name:      volumeNameRedisRun,
									MountPath: "/opt/bitnami/redis/tmp",
								},
							},
						},
					},
					SecurityContext: securitycontext.Pod(securitycontext.WithPodSeccompProfile(corev1.SeccompProfileTypeRuntimeDefault)),
					Affinity:        utils.GetAffinityForOAuthAuxComponent(),
					Volumes: []corev1.Volume{
						o.getEmptyDirVolume(volumeNameRedisData),
						o.getEmptyDirVolume(volumeNameRedisConfig),
						o.getEmptyDirVolume(volumeNameRedisTmp),
						o.getEmptyDirVolume(volumeNameRedisRun),
					},
				},
			},
		},
	}
	oauthutil.MergeAuxOAuthRedisComponentResourceLabels(desiredDeployment, o.rd.Spec.AppName, component)
	return desiredDeployment, nil
}

func (o *oauthRedisResourceManager) getEmptyDirVolume(name string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

func (o *oauthRedisResourceManager) getEnvVars(component v1.RadixCommonDeployComponent) []corev1.EnvVar {
	var envVars []corev1.EnvVar
	if v, ok := component.GetEnvironmentVariables()[defaults.RadixRestartEnvironmentVariable]; ok {
		envVars = append(envVars, corev1.EnvVar{Name: defaults.RadixRestartEnvironmentVariable, Value: v})
	}
	secretName := utils.GetAuxiliaryComponentSecretName(component.GetName(), v1.OAuthProxyAuxiliaryComponentSuffix)
	return append(envVars, o.createEnvVarWithSecretRef(redisPasswordEnvironmentVariable, secretName, defaults.OAuthRedisPasswordKeyName))
}

func (o *oauthRedisResourceManager) createEnvVarWithSecretRef(envVarName, secretName, key string) corev1.EnvVar {
	return corev1.EnvVar{
		Name: envVarName,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  key,
			},
		},
	}
}

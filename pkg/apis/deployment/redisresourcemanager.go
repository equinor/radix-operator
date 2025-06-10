package deployment

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"k8s.io/apimachinery/pkg/util/intstr"

	commonutils "github.com/equinor/radix-common/utils"
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
	"k8s.io/client-go/tools/cache"
)

const (
	redisRedisPasswordEnvironmentVariable = "OAUTH2_PROXY_REDIS_PASSWORD"
)

// NewRedisResourceManager creates a new RedisResourceManager
func NewRedisResourceManager(rd *v1.RadixDeployment, rr *v1.RadixRegistration, kubeutil *kube.Kube, redisDockerImage string) AuxiliaryResourceManager {
	return &redisResourceManager{
		rd:               rd,
		rr:               rr,
		kubeutil:         kubeutil,
		redisDockerImage: redisDockerImage,
		logger:           log.Logger.With().Str("resource_kind", v1.KindRadixDeployment).Str("resource_name", cache.MetaObjectToName(&rd.ObjectMeta).String()).Str("aux", "redis").Logger(),
	}
}

type redisResourceManager struct {
	rd               *v1.RadixDeployment
	rr               *v1.RadixRegistration
	kubeutil         *kube.Kube
	redisDockerImage string
	logger           zerolog.Logger
}

func (o *redisResourceManager) Sync(ctx context.Context) error {
	for _, component := range o.rd.Spec.Components {
		if err := o.syncComponent(ctx, &component); err != nil {
			return fmt.Errorf("failed to sync redis for component %s: %w", component.Name, err)
		}
	}
	return nil
}

func (o *redisResourceManager) syncComponent(ctx context.Context, component *v1.RadixDeployComponent) error {
	if auth := component.GetAuthentication(); component.IsPublic() && auth != nil && auth.OAuth2.SessionStoreTypeIsSystemManaged() {
		o.logger.Debug().Msgf("Sync system managed Redis for the component %s", component.GetName())
		return o.install(ctx, component.DeepCopy())
	}
	return o.uninstall(ctx, component)
}

func (o *redisResourceManager) GarbageCollect(ctx context.Context) error {
	if err := o.garbageCollect(ctx); err != nil {
		return fmt.Errorf("failed to garbage collect redis: %w", err)
	}
	return nil
}

func (o *redisResourceManager) garbageCollect(ctx context.Context) error {
	if err := o.garbageCollectDeployment(ctx); err != nil {
		return fmt.Errorf("failed to garbage collect deployment: %w", err)
	}

	if err := o.garbageCollectSecrets(ctx); err != nil {
		return fmt.Errorf("failed to garbage collect secrets: %w", err)
	}

	if err := o.garbageCollectRoles(ctx); err != nil {
		return fmt.Errorf("failed to garbage collect roles: %w", err)
	}

	if err := o.garbageCollectRoleBinding(ctx); err != nil {
		return fmt.Errorf("failed to garbage collect role bindings: %w", err)
	}

	return nil
}

func (o *redisResourceManager) garbageCollectDeployment(ctx context.Context) error {
	deployments, err := o.kubeutil.ListDeployments(ctx, o.rd.Namespace)
	if err != nil {
		return err
	}

	for _, deployment := range deployments {
		if o.isEligibleForGarbageCollection(deployment) {
			err := o.kubeutil.KubeClient().AppsV1().Deployments(deployment.Namespace).Delete(ctx, deployment.Name, metav1.DeleteOptions{})
			if err != nil && !kubeerrors.IsNotFound(err) {
				return err
			}
			o.logger.Info().Msgf("Deleted deployment: %s in namespace %s", deployment.GetName(), deployment.Namespace)
		}
	}

	return nil
}

func (o *redisResourceManager) garbageCollectSecrets(ctx context.Context) error {
	secrets, err := o.kubeutil.ListSecrets(ctx, o.rd.Namespace)
	if err != nil {
		return err
	}

	for _, secret := range secrets {
		if o.isEligibleForGarbageCollection(secret) {
			err := o.kubeutil.KubeClient().CoreV1().Secrets(secret.Namespace).Delete(ctx, secret.Name, metav1.DeleteOptions{})
			if err != nil && !kubeerrors.IsNotFound(err) {
				return err
			}
			o.logger.Info().Msgf("Deleted secret: %s in namespace %s", secret.GetName(), secret.Namespace)
		}
	}

	return nil
}

func (o *redisResourceManager) garbageCollectRoles(ctx context.Context) error {
	roles, err := o.kubeutil.ListRoles(ctx, o.rd.Namespace)
	if err != nil {
		return err
	}

	for _, role := range roles {
		if o.isEligibleForGarbageCollection(role) {
			err := o.kubeutil.KubeClient().RbacV1().Roles(role.Namespace).Delete(ctx, role.Name, metav1.DeleteOptions{})
			if err != nil && !kubeerrors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

func (o *redisResourceManager) garbageCollectRoleBinding(ctx context.Context) error {
	roleBindings, err := o.kubeutil.ListRoleBindings(ctx, o.rd.Namespace)
	if err != nil {
		return err
	}

	for _, rolebinding := range roleBindings {
		if o.isEligibleForGarbageCollection(rolebinding) {
			err := o.kubeutil.KubeClient().RbacV1().RoleBindings(rolebinding.Namespace).Delete(ctx, rolebinding.Name, metav1.DeleteOptions{})
			if err != nil && !kubeerrors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

func (o *redisResourceManager) isEligibleForGarbageCollection(object metav1.Object) bool {
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

func (o *redisResourceManager) install(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	o.logger.Debug().Msgf("install the Redis for the component %s", component.GetName())
	if err := o.createOrUpdateSecret(ctx, component); err != nil {
		return err
	}
	if err := o.createOrUpdateRbac(ctx, component); err != nil {
		return err
	}
	if err := o.createOrUpdateService(ctx, component); err != nil {
		return err
	}
	return o.createOrUpdateDeployment(ctx, component)
}

func (o *redisResourceManager) uninstall(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	o.logger.Debug().Msgf("uninstall redis for the component %s", component.GetName())
	if err := o.deleteDeployment(ctx, component); err != nil {
		return err
	}

	if err := o.deleteServices(ctx, component); err != nil {
		return err
	}

	if err := o.deleteSecretValue(ctx, component); err != nil {
		return err
	}

	if err := o.deleteRoleBindings(ctx, component); err != nil {
		return err
	}

	return o.deleteRoles(ctx, component)
}

func (o *redisResourceManager) deleteDeployment(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	selector := labels.SelectorFromValidatedSet(radixlabels.ForAuxRedisComponent(o.rd.Spec.AppName, component)).String()
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

func (o *redisResourceManager) deleteServices(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	selector := labels.SelectorFromValidatedSet(radixlabels.ForAuxRedisComponent(o.rd.Spec.AppName, component)).String()
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

func (o *redisResourceManager) deleteSecretValue(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	selector := labels.SelectorFromValidatedSet(radixlabels.ForAuxComponent(o.rd.Spec.AppName, component)).String()
	secrets, err := o.kubeutil.ListSecretsWithSelector(ctx, o.rd.Namespace, selector)
	if err != nil {
		return err
	}

	for _, existingSecret := range secrets {
		secret := existingSecret.DeepCopy()
		if _, ok := secret.Data[defaults.OAuthRedisPasswordKeyName]; ok {
			delete(secret.Data, defaults.OAuthRedisPasswordKeyName)
			_, err = o.kubeutil.UpdateSecret(ctx, existingSecret, secret)
		}
		o.logger.Info().Msgf("Deleted secret key: %s from secret %s in namespace %s", defaults.OAuthRedisPasswordKeyName, secret.GetName(), secret.Namespace)
	}

	return nil
}

func (o *redisResourceManager) deleteRoleBindings(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	selector := labels.SelectorFromValidatedSet(radixlabels.ForAuxRedisComponent(o.rd.Spec.AppName, component)).String()
	roleBindings, err := o.kubeutil.ListRoleBindingsWithSelector(ctx, o.rd.Namespace, selector)
	if err != nil {
		return err
	}

	for _, rolebinding := range roleBindings {
		if err := o.kubeutil.KubeClient().RbacV1().RoleBindings(rolebinding.Namespace).Delete(ctx, rolebinding.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (o *redisResourceManager) deleteRoles(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	selector := labels.SelectorFromValidatedSet(radixlabels.ForAuxRedisComponent(o.rd.Spec.AppName, component)).String()
	roles, err := o.kubeutil.ListRolesWithSelector(ctx, o.rd.Namespace, selector)
	if err != nil {
		return err
	}

	for _, role := range roles {
		if err := o.kubeutil.KubeClient().RbacV1().Roles(role.Namespace).Delete(ctx, role.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (o *redisResourceManager) createOrUpdateService(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	service := o.buildServiceSpec(component)
	return o.kubeutil.ApplyService(ctx, o.rd.Namespace, service)
}

func (o *redisResourceManager) createOrUpdateSecret(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	secretName := utils.GetAuxiliaryComponentSecretName(component.GetName(), v1.OAuthProxyAuxiliaryComponentSuffix)
	existingSecret, err := o.kubeutil.GetSecret(ctx, o.rd.Namespace, secretName)
	if err != nil {
		if !kubeerrors.IsNotFound(err) {
			return err
		}
		secret, err := o.buildSecret(o.rd.Spec.AppName, component)
		if err != nil {
			return err
		}
		_, err = o.kubeutil.CreateSecret(ctx, o.rd.Namespace, secret)
		return err
	}
	secret := existingSecret.DeepCopy()
	oauthutil.MergeAuxProxyComponentResourceLabels(secret, o.rd.Spec.AppName, component)
	if passwordSecret, ok := secret.Data[defaults.OAuthRedisPasswordKeyName]; !ok || len(passwordSecret) == 0 {
		passwordSecret, err = o.generateRandomPasswordSecret()
		if err != nil {
			return err
		}
		secret.Data[defaults.OAuthRedisPasswordKeyName] = passwordSecret
		_, err = o.kubeutil.UpdateSecret(ctx, existingSecret, secret)
	}
	return err
}

func (o *redisResourceManager) createOrUpdateRbac(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	if err := o.createOrUpdateAppAdminRbac(ctx, component); err != nil {
		return err
	}

	return o.createOrUpdateAppReaderRbac(ctx, component)
}

func (o *redisResourceManager) createOrUpdateAppAdminRbac(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	secretName := utils.GetAuxiliaryComponentSecretName(component.GetName(), v1.OAuthRedisAuxiliaryComponentSuffix)
	roleName := o.getRoleAndRoleBindingName("radix-app-adm", component.GetName())
	namespace := o.rd.Namespace

	// create role
	role := kube.CreateAppRole(
		o.rd.Spec.AppName,
		roleName,
		radixlabels.ForAuxRedisComponent(o.rd.Spec.AppName, component),
		kube.ManageSecretsRule([]string{secretName}),
	)

	err := o.kubeutil.ApplyRole(ctx, namespace, role)
	if err != nil {
		return err
	}

	// create rolebinding
	subjects, err := utils.GetAppAdminRbacSubjects(o.rr)
	if err != nil {
		return err
	}
	rolebinding := kube.GetRolebindingToRoleWithLabelsForSubjects(roleName, subjects, role.Labels)
	return o.kubeutil.ApplyRoleBinding(ctx, namespace, rolebinding)
}

func (o *redisResourceManager) createOrUpdateAppReaderRbac(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	secretName := utils.GetAuxiliaryComponentSecretName(component.GetName(), v1.OAuthRedisAuxiliaryComponentSuffix)
	roleName := o.getRoleAndRoleBindingName("radix-app-reader", component.GetName())
	namespace := o.rd.Namespace

	// create role
	role := kube.CreateAppRole(
		o.rd.Spec.AppName,
		roleName,
		radixlabels.ForAuxRedisComponent(o.rd.Spec.AppName, component),
		kube.ReadSecretsRule([]string{secretName}),
	)

	err := o.kubeutil.ApplyRole(ctx, namespace, role)
	if err != nil {
		return err
	}

	// create rolebinding
	subjects := utils.GetAppReaderRbacSubjects(o.rr)
	rolebinding := kube.GetRolebindingToRoleWithLabelsForSubjects(roleName, subjects, role.Labels)
	return o.kubeutil.ApplyRoleBinding(ctx, namespace, rolebinding)
}

func (o *redisResourceManager) getRoleAndRoleBindingName(prefix, componentName string) string {
	deploymentName := utils.GetAuxiliaryComponentDeploymentName(componentName, v1.OAuthRedisAuxiliaryComponentSuffix)
	return fmt.Sprintf("%s-%s", prefix, deploymentName)
}

func (o *redisResourceManager) buildSecret(appName string, component v1.RadixCommonDeployComponent) (*corev1.Secret, error) {
	secretName := utils.GetAuxiliaryComponentSecretName(component.GetName(), v1.OAuthProxyAuxiliaryComponentSuffix)

	secret := &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
		},
		Data: make(map[string][]byte),
	}
	oauthutil.MergeAuxComponentResourceLabels(secret, appName, component)
	passwordSecret, err := o.generateRandomPasswordSecret()
	if err != nil {
		return nil, err
	}
	secret.Data[defaults.OAuthRedisPasswordKeyName] = passwordSecret
	return secret, nil
}

func (o *redisResourceManager) buildServiceSpec(component v1.RadixCommonDeployComponent) *corev1.Service {
	serviceName := utils.GetAuxiliaryComponentServiceName(component.GetName(), v1.OAuthRedisAuxiliaryComponentSuffix)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            serviceName,
			OwnerReferences: []metav1.OwnerReference{getOwnerReferenceOfDeployment(o.rd)},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: radixlabels.ForAuxComponent(o.rd.Spec.AppName, component),
			Ports: []corev1.ServicePort{
				{
					Port:       v1.OAuthRedisPortNumber,
					TargetPort: intstr.FromInt32(v1.OAuthRedisPortNumber),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	oauthutil.MergeAuxProxyComponentResourceLabels(service, o.rd.Spec.AppName, component)
	return service
}

func (o *redisResourceManager) generateRandomPasswordSecret() ([]byte, error) {
	randomBytes := commonutils.GenerateRandomKey(32)
	// Extra check to make sure correct number of bytes are returned for the random key
	if len(randomBytes) != 32 {
		return nil, errors.New("failed to generator cookie secret with correct length")
	}
	encoding := base64.URLEncoding
	encodedBytes := make([]byte, encoding.EncodedLen(len(randomBytes)))
	encoding.Encode(encodedBytes, randomBytes)
	return encodedBytes, nil
}

func (o *redisResourceManager) createOrUpdateDeployment(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	current, desired, err := o.getCurrentAndDesiredDeployment(ctx, component)
	if err != nil {
		return err
	}

	if err := o.kubeutil.ApplyDeployment(ctx, o.rd.Namespace, current, desired); err != nil {
		return err
	}
	return nil
}

func (o *redisResourceManager) getCurrentAndDesiredDeployment(ctx context.Context, component v1.RadixCommonDeployComponent) (*appsv1.Deployment, *appsv1.Deployment, error) {
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

func (o *redisResourceManager) getDesiredDeployment(component v1.RadixCommonDeployComponent) (*appsv1.Deployment, error) {
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
	desiredDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            deploymentName,
			Annotations:     annotations.ForKubernetesDeploymentObservedGeneration(o.rd),
			OwnerReferences: []metav1.OwnerReference{getOwnerReferenceOfDeployment(o.rd)},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointers.Ptr(replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: radixlabels.ForAuxRedisComponent(o.rd.Spec.AppName, component),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: radixlabels.Merge(
						radixlabels.ForAuxRedisComponent(o.rd.Spec.AppName, component),
					),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            componentName,
							Image:           o.redisDockerImage,
							ImagePullPolicy: corev1.PullAlways,
							Env:             o.getEnvVars(component),
							Ports: []corev1.ContainerPort{
								{
									Name:          "redis",
									ContainerPort: v1.OAuthRedisPortNumber,
								},
							},
							ReadinessProbe: readinessProbe,
							SecurityContext: securitycontext.Container(
								securitycontext.WithContainerSeccompProfileType(corev1.SeccompProfileTypeRuntimeDefault),
								securitycontext.WithReadOnlyRootFileSystem(pointers.Ptr(true)),
							),
							Resources: resources.New(resources.WithMemoryMega(100), resources.WithCPUMilli(10)),
						},
					},
					SecurityContext: securitycontext.Pod(securitycontext.WithPodSeccompProfile(corev1.SeccompProfileTypeRuntimeDefault)),
					Affinity:        utils.GetAffinityForOAuthAuxComponent(),
				},
			},
		},
	}
	oauthutil.MergeAuxComponentResourceLabels(desiredDeployment, o.rd.Spec.AppName, component)
	return desiredDeployment, nil
}

func (o *redisResourceManager) getEnvVars(component v1.RadixCommonDeployComponent) []corev1.EnvVar {
	var envVars []corev1.EnvVar
	// Radix env-vars
	if v, ok := component.GetEnvironmentVariables()[defaults.RadixRestartEnvironmentVariable]; ok {
		envVars = append(envVars, corev1.EnvVar{Name: defaults.RadixRestartEnvironmentVariable, Value: v})
	}
	secretName := utils.GetAuxiliaryComponentSecretName(component.GetName(), v1.OAuthProxyAuxiliaryComponentSuffix)
	envVars = append(envVars, o.createEnvVarWithSecretRef(redisRedisPasswordEnvironmentVariable, secretName, defaults.OAuthRedisPasswordKeyName))
	return envVars
}

func (o *redisResourceManager) createEnvVarWithSecretRef(envVarName, secretName, key string) corev1.EnvVar {
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

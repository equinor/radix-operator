package deployment

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"reflect"

	commonutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/securitycontext"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	oauthutil "github.com/equinor/radix-operator/pkg/apis/utils/oauth"
	"github.com/equinor/radix-operator/pkg/apis/utils/resources"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/cache"
)

// NewOAuthProxyResourceManager creates a new OAuthProxyResourceManager
func NewOAuthProxyResourceManager(rd *v1.RadixDeployment, rr *v1.RadixRegistration, kubeutil *kube.Kube, oauth2DefaultConfig defaults.OAuth2Config, ingressAnnotationProviders []ingress.AnnotationProvider, oauth2ProxyDockerImage string) AuxiliaryResourceManager {

	return &oauthProxyResourceManager{
		rd:                         rd,
		rr:                         rr,
		kubeutil:                   kubeutil,
		ingressAnnotationProviders: ingressAnnotationProviders,
		oauth2DefaultConfig:        oauth2DefaultConfig,
		oauth2ProxyDockerImage:     oauth2ProxyDockerImage,
		logger:                     log.Logger.With().Str("resource_kind", v1.KindRadixDeployment).Str("resource_name", cache.MetaObjectToName(&rd.ObjectMeta).String()).Str("aux", "oauth2").Logger(),
	}
}

type oauthProxyResourceManager struct {
	rd                         *v1.RadixDeployment
	rr                         *v1.RadixRegistration
	kubeutil                   *kube.Kube
	ingressAnnotationProviders []ingress.AnnotationProvider
	oauth2DefaultConfig        defaults.OAuth2Config
	oauth2ProxyDockerImage     string
	logger                     zerolog.Logger
}

func (o *oauthProxyResourceManager) Sync() error {
	for _, component := range o.rd.Spec.Components {
		if err := o.syncComponent(&component); err != nil {
			return fmt.Errorf("failed to sync oauth proxy for component %s: %w", component.Name, err)
		}
	}
	return nil
}

func (o *oauthProxyResourceManager) syncComponent(component *v1.RadixDeployComponent) error {
	if auth := component.GetAuthentication(); component.IsPublic() && auth != nil && auth.OAuth2 != nil {
		o.logger.Debug().Msgf("Sync oauth proxy for the component %s", component.GetName())
		componentWithOAuthDefaults := component.DeepCopy()
		oauth, err := o.oauth2DefaultConfig.MergeWith(componentWithOAuthDefaults.Authentication.OAuth2)
		if err != nil {
			return err
		}
		componentWithOAuthDefaults.Authentication.OAuth2 = oauth
		return o.install(componentWithOAuthDefaults)
	}
	return o.uninstall(component)
}

func (o *oauthProxyResourceManager) GarbageCollect() error {
	if err := o.garbageCollect(); err != nil {
		return fmt.Errorf("failed to garbage collect oauth2 proxy: %w", err)
	}
	return nil
}

func (o *oauthProxyResourceManager) garbageCollect() error {
	if err := o.garbageCollectDeployment(); err != nil {
		return fmt.Errorf("failed to garbage collect deployment: %w", err)
	}

	if err := o.garbageCollectSecrets(); err != nil {
		return fmt.Errorf("failed to garbage collect secrets: %w", err)
	}

	if err := o.garbageCollectRoles(); err != nil {
		return fmt.Errorf("failed to garbage collect roles: %w", err)
	}

	if err := o.garbageCollectRoleBinding(); err != nil {
		return fmt.Errorf("failed to garbage collect role bindings: %w", err)
	}

	if err := o.garbageCollectServices(); err != nil {
		return fmt.Errorf("failed to garbage collect services: %w", err)
	}

	if err := o.garbageCollectIngresses(); err != nil {
		return fmt.Errorf("failed to garbage collect ingresses: %w", err)
	}

	return nil
}

func (o *oauthProxyResourceManager) garbageCollectDeployment() error {
	deployments, err := o.kubeutil.ListDeployments(o.rd.Namespace)
	if err != nil {
		return err
	}

	for _, deployment := range deployments {
		if o.isEligibleForGarbageCollection(deployment) {
			err := o.kubeutil.KubeClient().AppsV1().Deployments(deployment.Namespace).Delete(context.TODO(), deployment.Name, metav1.DeleteOptions{})
			if err != nil && !kubeerrors.IsNotFound(err) {
				return err
			}
			o.logger.Info().Msgf("Deleted deployment: %s in namespace %s", deployment.GetName(), deployment.Namespace)
		}
	}

	return nil
}

func (o *oauthProxyResourceManager) garbageCollectSecrets() error {
	secrets, err := o.kubeutil.ListSecrets(o.rd.Namespace)
	if err != nil {
		return err
	}

	for _, secret := range secrets {
		if o.isEligibleForGarbageCollection(secret) {
			err := o.kubeutil.KubeClient().CoreV1().Secrets(secret.Namespace).Delete(context.TODO(), secret.Name, metav1.DeleteOptions{})
			if err != nil && !kubeerrors.IsNotFound(err) {
				return err
			}
			o.logger.Info().Msgf("Deleted secret: %s in namespace %s", secret.GetName(), secret.Namespace)
		}
	}

	return nil
}

func (o *oauthProxyResourceManager) garbageCollectServices() error {
	services, err := o.kubeutil.ListServices(o.rd.Namespace)
	if err != nil {
		return err
	}

	for _, service := range services {
		if o.isEligibleForGarbageCollection(service) {
			err := o.kubeutil.KubeClient().CoreV1().Services(service.Namespace).Delete(context.TODO(), service.Name, metav1.DeleteOptions{})
			if err != nil && !kubeerrors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

func (o *oauthProxyResourceManager) garbageCollectIngresses() error {
	ingresses, err := o.kubeutil.ListIngresses(o.rd.Namespace)
	if err != nil {
		return err
	}

	for _, ing := range ingresses {
		if o.isEligibleForGarbageCollection(ing) {
			err := o.kubeutil.KubeClient().NetworkingV1().Ingresses(ing.Namespace).Delete(context.TODO(), ing.Name, metav1.DeleteOptions{})
			if err != nil && !kubeerrors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

func (o *oauthProxyResourceManager) garbageCollectRoles() error {
	roles, err := o.kubeutil.ListRoles(o.rd.Namespace)
	if err != nil {
		return err
	}

	for _, role := range roles {
		if o.isEligibleForGarbageCollection(role) {
			err := o.kubeutil.KubeClient().RbacV1().Roles(role.Namespace).Delete(context.TODO(), role.Name, metav1.DeleteOptions{})
			if err != nil && !kubeerrors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

func (o *oauthProxyResourceManager) garbageCollectRoleBinding() error {
	roleBindings, err := o.kubeutil.ListRoleBindings(o.rd.Namespace)
	if err != nil {
		return err
	}

	for _, rolebinding := range roleBindings {
		if o.isEligibleForGarbageCollection(rolebinding) {
			err := o.kubeutil.KubeClient().RbacV1().RoleBindings(rolebinding.Namespace).Delete(context.TODO(), rolebinding.Name, metav1.DeleteOptions{})
			if err != nil && !kubeerrors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

func (o *oauthProxyResourceManager) isEligibleForGarbageCollection(object metav1.Object) bool {
	if appName := object.GetLabels()[kube.RadixAppLabel]; appName != o.rd.Spec.AppName {
		return false
	}
	if auxType := object.GetLabels()[kube.RadixAuxiliaryComponentTypeLabel]; auxType != defaults.OAuthProxyAuxiliaryComponentType {
		return false
	}
	auxTargetComponentName, nameExist := RadixComponentNameFromAuxComponentLabel(object)
	if !nameExist {
		return false
	}
	return !auxTargetComponentName.ExistInDeploymentSpec(o.rd)
}

func (o *oauthProxyResourceManager) install(component v1.RadixCommonDeployComponent) error {
	o.logger.Debug().Msgf("install the oauth proxy for the component %s", component.GetName())
	if err := o.createOrUpdateSecret(component); err != nil {
		return err
	}

	if err := o.createOrUpdateRbac(component); err != nil {
		return err
	}

	if err := o.createOrUpdateService(component); err != nil {
		return err
	}

	if err := o.createOrUpdateIngresses(component); err != nil {
		return err
	}

	return o.createOrUpdateDeployment(component)
}

func (o *oauthProxyResourceManager) uninstall(component v1.RadixCommonDeployComponent) error {
	o.logger.Debug().Msgf("uninstall oauth proxy for the component %s", component.GetName())
	if err := o.deleteDeployment(component); err != nil {
		return err
	}

	if err := o.deleteIngresses(component); err != nil {
		return err
	}

	if err := o.deleteServices(component); err != nil {
		return err
	}

	if err := o.deleteSecrets(component); err != nil {
		return err
	}

	if err := o.deleteRoleBindings(component); err != nil {
		return err
	}

	return o.deleteRoles(component)
}

func (o *oauthProxyResourceManager) deleteDeployment(component v1.RadixCommonDeployComponent) error {
	selector := labels.SelectorFromValidatedSet(radixlabels.ForAuxComponent(o.rd.Spec.AppName, component)).String()
	deployments, err := o.kubeutil.ListDeploymentsWithSelector(o.rd.Namespace, selector)
	if err != nil {
		return err
	}

	for _, deployment := range deployments {
		if err := o.kubeutil.KubeClient().AppsV1().Deployments(deployment.Namespace).Delete(context.TODO(), deployment.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (o *oauthProxyResourceManager) deleteIngresses(component v1.RadixCommonDeployComponent) error {
	ingresses, err := o.getComponentAuxIngresses(component)
	if err != nil {
		return err
	}
	return o.kubeutil.DeleteIngresses(ingresses...)
}

func (o *oauthProxyResourceManager) deleteServices(component v1.RadixCommonDeployComponent) error {
	selector := labels.SelectorFromValidatedSet(radixlabels.ForAuxComponent(o.rd.Spec.AppName, component)).String()
	services, err := o.kubeutil.ListServicesWithSelector(o.rd.Namespace, selector)
	if err != nil {
		return err
	}

	for _, service := range services {
		if err := o.kubeutil.KubeClient().CoreV1().Services(service.Namespace).Delete(context.TODO(), service.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (o *oauthProxyResourceManager) deleteSecrets(component v1.RadixCommonDeployComponent) error {
	selector := labels.SelectorFromValidatedSet(radixlabels.ForAuxComponent(o.rd.Spec.AppName, component)).String()
	secrets, err := o.kubeutil.ListSecretsWithSelector(o.rd.Namespace, selector)
	if err != nil {
		return err
	}

	for _, secret := range secrets {
		if err := o.kubeutil.KubeClient().CoreV1().Secrets(secret.Namespace).Delete(context.TODO(), secret.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
		o.logger.Info().Msgf("Deleted secret: %s in namespace %s", secret.GetName(), secret.Namespace)
	}

	return nil
}

func (o *oauthProxyResourceManager) deleteRoleBindings(component v1.RadixCommonDeployComponent) error {
	selector := labels.SelectorFromValidatedSet(radixlabels.ForAuxComponent(o.rd.Spec.AppName, component)).String()
	roleBindings, err := o.kubeutil.ListRoleBindingsWithSelector(o.rd.Namespace, selector)
	if err != nil {
		return err
	}

	for _, rolebinding := range roleBindings {
		if err := o.kubeutil.KubeClient().RbacV1().RoleBindings(rolebinding.Namespace).Delete(context.TODO(), rolebinding.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (o *oauthProxyResourceManager) deleteRoles(component v1.RadixCommonDeployComponent) error {
	selector := labels.SelectorFromValidatedSet(radixlabels.ForAuxComponent(o.rd.Spec.AppName, component)).String()
	roles, err := o.kubeutil.ListRolesWithSelector(o.rd.Namespace, selector)
	if err != nil {
		return err
	}

	for _, role := range roles {
		if err := o.kubeutil.KubeClient().RbacV1().Roles(role.Namespace).Delete(context.TODO(), role.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (o *oauthProxyResourceManager) createOrUpdateIngresses(component v1.RadixCommonDeployComponent) error {
	namespace := o.rd.Namespace
	o.logger.Debug().Msgf("create of update ingresses for the component %s in the namespace %s", component.GetName(), namespace)
	ingresses, err := o.getComponentIngresses(component)
	if err != nil {
		return err
	}

	for _, ing := range ingresses {
		appName := o.rd.Spec.AppName
		auxIngress, err := ingress.BuildOAuthProxyIngressForComponentIngress(namespace, component, &ing, o.ingressAnnotationProviders)
		if err != nil {
			return err
		}
		if auxIngress == nil {
			continue
		}
		mergeAuxIngressLabels(appName, component, ing.GetLabels(), auxIngress)
		if err := o.kubeutil.ApplyIngress(namespace, auxIngress); err != nil {
			return err
		}
	}
	return nil
}

func mergeAuxIngressLabels(appName string, component v1.RadixCommonDeployComponent, componentLabels map[string]string, auxIngress *networkingv1.Ingress) {
	switch {
	case componentLabels[kube.RadixDefaultAliasLabel] == "true":
		oauthutil.MergeAuxComponentDefaultAliasIngressLabels(auxIngress, appName, component)
	case componentLabels[kube.RadixActiveClusterAliasLabel] == "true":
		oauthutil.MergeAuxComponentActiveClusterAliasIngressLabels(auxIngress, appName, component)
	case componentLabels[kube.RadixAppAliasLabel] == "true":
		oauthutil.MergeAuxComponentAppAliasIngressLabels(auxIngress, appName, component)
	case componentLabels[kube.RadixExternalAliasLabel] == "true":
		oauthutil.MergeAuxComponentExternalAliasIngressLabels(auxIngress, appName, component)
	}
}

func (o *oauthProxyResourceManager) getComponentIngresses(component v1.RadixCommonDeployComponent) ([]networkingv1.Ingress, error) {
	namespace := o.rd.Namespace
	return o.getIngressesForSelector(namespace,
		radixlabels.ForComponentDefaultAliasIngress(component),
		radixlabels.ForComponentActiveClusterAliasIngress(component),
		radixlabels.ForComponentAppAliasIngress(component),
		radixlabels.ForComponentExternalAliasIngress(component),
	)
}

func (o *oauthProxyResourceManager) getComponentAuxIngresses(component v1.RadixCommonDeployComponent) ([]networkingv1.Ingress, error) {
	appName := o.rd.Spec.AppName
	return o.getIngressesForSelector(o.rd.Namespace,
		radixlabels.ForAuxComponentDefaultIngress(appName, component),
		radixlabels.ForAuxComponentActiveClusterAliasIngress(appName, component),
		radixlabels.ForAuxComponentAppAliasIngress(appName, component),
		radixlabels.ForAuxComponentExternalAliasIngress(appName, component),
	)
}

func (o *oauthProxyResourceManager) getIngressesForSelector(namespace string, selectors ...labels.Set) ([]networkingv1.Ingress, error) {
	var ingresses []networkingv1.Ingress
	for _, selector := range selectors {
		ingressList, err := o.kubeutil.KubeClient().NetworkingV1().Ingresses(namespace).
			List(context.Background(), metav1.ListOptions{LabelSelector: selector.String()})
		if err != nil {
			return nil, err
		}
		ingresses = append(ingresses, ingressList.Items...)
	}
	return ingresses, nil
}

func (o *oauthProxyResourceManager) createOrUpdateService(component v1.RadixCommonDeployComponent) error {
	service := o.buildServiceSpec(component)
	return o.kubeutil.ApplyService(o.rd.Namespace, service)
}

func (o *oauthProxyResourceManager) createOrUpdateSecret(component v1.RadixCommonDeployComponent) error {
	secretName := utils.GetAuxiliaryComponentSecretName(component.GetName(), defaults.OAuthProxyAuxiliaryComponentSuffix)
	secret, err := o.kubeutil.GetSecret(o.rd.Namespace, secretName)

	if err != nil {
		if !kubeerrors.IsNotFound(err) {
			return err
		}
		secret, err = o.buildSecretSpec(component)
		if err != nil {
			return err
		}
	} else {
		oauthutil.MergeAuxComponentResourceLabels(secret, o.rd.Spec.AppName, component)
		if component.GetAuthentication().OAuth2.SessionStoreType != v1.SessionStoreRedis {
			if secret.Data != nil && len(secret.Data[defaults.OAuthRedisPasswordKeyName]) > 0 {
				delete(secret.Data, defaults.OAuthRedisPasswordKeyName)
			}
		}
	}

	_, err = o.kubeutil.ApplySecret(o.rd.Namespace, secret)
	return err
}

func (o *oauthProxyResourceManager) createOrUpdateRbac(component v1.RadixCommonDeployComponent) error {
	if err := o.createOrUpdateAppAdminRbac(component); err != nil {
		return err
	}

	return o.createOrUpdateAppReaderRbac(component)
}

func (o *oauthProxyResourceManager) createOrUpdateAppAdminRbac(component v1.RadixCommonDeployComponent) error {
	secretName := utils.GetAuxiliaryComponentSecretName(component.GetName(), defaults.OAuthProxyAuxiliaryComponentSuffix)
	deploymentName := utils.GetAuxiliaryComponentDeploymentName(component.GetName(), defaults.OAuthProxyAuxiliaryComponentSuffix)
	roleName := o.getRoleAndRoleBindingName("radix-app-adm", component.GetName())
	namespace := o.rd.Namespace

	// create role
	role := kube.CreateAppRole(
		o.rd.Spec.AppName,
		roleName,
		radixlabels.ForAuxComponent(o.rd.Spec.AppName, component),
		kube.ManageSecretsRule([]string{secretName}),
		kube.UpdateDeploymentsRule([]string{deploymentName}),
	)

	err := o.kubeutil.ApplyRole(namespace, role)
	if err != nil {
		return err
	}

	// create rolebinding
	adGroups, err := utils.GetAdGroups(o.rr)
	if err != nil {
		return err
	}

	subjects := kube.GetRoleBindingGroups(adGroups)
	rolebinding := kube.GetRolebindingToRoleWithLabelsForSubjects(roleName, subjects, role.Labels)
	return o.kubeutil.ApplyRoleBinding(namespace, rolebinding)
}

func (o *oauthProxyResourceManager) createOrUpdateAppReaderRbac(component v1.RadixCommonDeployComponent) error {
	secretName := utils.GetAuxiliaryComponentSecretName(component.GetName(), defaults.OAuthProxyAuxiliaryComponentSuffix)
	roleName := o.getRoleAndRoleBindingName("radix-app-reader", component.GetName())
	namespace := o.rd.Namespace

	// create role
	role := kube.CreateAppRole(
		o.rd.Spec.AppName,
		roleName,
		radixlabels.ForAuxComponent(o.rd.Spec.AppName, component),
		kube.ReadSecretsRule([]string{secretName}),
	)

	err := o.kubeutil.ApplyRole(namespace, role)
	if err != nil {
		return err
	}

	// create rolebinding
	subjects := kube.GetRoleBindingGroups(o.rr.Spec.ReaderAdGroups)
	rolebinding := kube.GetRolebindingToRoleWithLabelsForSubjects(roleName, subjects, role.Labels)
	return o.kubeutil.ApplyRoleBinding(namespace, rolebinding)
}

func (o *oauthProxyResourceManager) getRoleAndRoleBindingName(prefix, componentName string) string {
	deploymentName := utils.GetAuxiliaryComponentDeploymentName(componentName, defaults.OAuthProxyAuxiliaryComponentSuffix)
	return fmt.Sprintf("%s-%s", prefix, deploymentName)
}

func (o *oauthProxyResourceManager) buildSecretSpec(component v1.RadixCommonDeployComponent) (*corev1.Secret, error) {
	secretName := utils.GetAuxiliaryComponentSecretName(component.GetName(), defaults.OAuthProxyAuxiliaryComponentSuffix)

	secret := &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
		},
		Data: make(map[string][]byte),
	}
	oauthutil.MergeAuxComponentResourceLabels(secret, o.rd.Spec.AppName, component)
	cookieSecret, err := o.generateRandomCookieSecret()
	if err != nil {
		return nil, err
	}
	secret.Data[defaults.OAuthCookieSecretKeyName] = cookieSecret
	return secret, nil
}

func (o *oauthProxyResourceManager) buildServiceSpec(component v1.RadixCommonDeployComponent) *corev1.Service {
	serviceName := utils.GetAuxiliaryComponentServiceName(component.GetName(), defaults.OAuthProxyAuxiliaryComponentSuffix)

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
					Port:       defaults.OAuthProxyPortNumber,
					TargetPort: intstr.FromString(defaults.OAuthProxyPortName),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	oauthutil.MergeAuxComponentResourceLabels(service, o.rd.Spec.AppName, component)
	return service
}

func (o *oauthProxyResourceManager) generateRandomCookieSecret() ([]byte, error) {
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

func (o *oauthProxyResourceManager) createOrUpdateDeployment(component v1.RadixCommonDeployComponent) error {
	current, desired, err := o.getCurrentAndDesiredDeployment(component)
	if err != nil {
		return err
	}

	if err := o.kubeutil.ApplyDeployment(o.rd.Namespace, current, desired); err != nil {
		return err
	}
	return nil
}

func (o *oauthProxyResourceManager) getCurrentAndDesiredDeployment(component v1.RadixCommonDeployComponent) (*appsv1.Deployment, *appsv1.Deployment, error) {
	deploymentName := utils.GetAuxiliaryComponentDeploymentName(component.GetName(), defaults.OAuthProxyAuxiliaryComponentSuffix)

	currentDeployment, err := o.kubeutil.GetDeployment(o.rd.Namespace, deploymentName)
	if err != nil && !kubeerrors.IsNotFound(err) {
		return nil, nil, err
	}
	desiredDeployment, err := o.getDesiredDeployment(component)
	if err != nil {
		return nil, nil, err
	}

	return currentDeployment, desiredDeployment, nil
}

func (o *oauthProxyResourceManager) getDesiredDeployment(component v1.RadixCommonDeployComponent) (*appsv1.Deployment, error) {
	deploymentName := utils.GetAuxiliaryComponentDeploymentName(component.GetName(), defaults.OAuthProxyAuxiliaryComponentSuffix)
	readinessProbe, err := getReadinessProbeWithDefaultsFromEnv(defaults.OAuthProxyPortNumber)
	if err != nil {
		return nil, err
	}

	var replicas int32 = 1
	if isComponentStopped(component) {
		replicas = 0
	}

	// Spec.Strategy defaults to RollingUpdate, ref https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#strategy
	desiredDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            deploymentName,
			Annotations:     make(map[string]string),
			OwnerReferences: []metav1.OwnerReference{getOwnerReferenceOfDeployment(o.rd)},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointers.Ptr(replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: radixlabels.ForAuxComponent(o.rd.Spec.AppName, component),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: radixlabels.ForAuxComponent(o.rd.Spec.AppName, component),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            component.GetName(),
							Image:           o.oauth2ProxyDockerImage,
							ImagePullPolicy: corev1.PullAlways,
							Env:             o.getEnvVars(component),
							Ports: []corev1.ContainerPort{
								{
									Name:          defaults.OAuthProxyPortName,
									ContainerPort: defaults.OAuthProxyPortNumber,
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
				},
			},
		},
	}

	oauthutil.MergeAuxComponentResourceLabels(desiredDeployment, o.rd.Spec.AppName, component)
	return desiredDeployment, nil
}

func (o *oauthProxyResourceManager) getEnvVars(component v1.RadixCommonDeployComponent) []corev1.EnvVar {
	var envVars []corev1.EnvVar
	oauth := component.GetAuthentication().OAuth2

	addEnvVarIfSet := func(envVar string, value interface{}) {
		rval := reflect.ValueOf(value)
		if !rval.IsZero() {
			switch rval.Kind() {
			case reflect.String:
				envVars = append(envVars, corev1.EnvVar{Name: envVar, Value: fmt.Sprint(rval)})
			case reflect.Ptr:
				envVars = append(envVars, corev1.EnvVar{Name: envVar, Value: fmt.Sprint(rval.Elem())})
			}
		}
	}

	// Radix env-vars
	if v, ok := component.GetEnvironmentVariables()[defaults.RadixRestartEnvironmentVariable]; ok {
		envVars = append(envVars, corev1.EnvVar{Name: defaults.RadixRestartEnvironmentVariable, Value: v})
	}

	// oauth2-proxy env-vars
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_PROVIDER", Value: "oidc"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_COOKIE_HTTPONLY", Value: "true"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_COOKIE_SECURE", Value: "true"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_PASS_BASIC_AUTH", Value: "false"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_SKIP_PROVIDER_BUTTON", Value: "true"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_EMAIL_DOMAINS", Value: "*"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_SKIP_CLAIMS_FROM_PROFILE_URL", Value: "true"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_HTTP_ADDRESS", Value: fmt.Sprintf("%s://:%v", "http", defaults.OAuthProxyPortNumber)})
	secretName := utils.GetAuxiliaryComponentSecretName(component.GetName(), defaults.OAuthProxyAuxiliaryComponentSuffix)
	envVars = append(envVars, o.createEnvVarWithSecretRef("OAUTH2_PROXY_COOKIE_SECRET", secretName, defaults.OAuthCookieSecretKeyName))
	envVars = append(envVars, o.createEnvVarWithSecretRef("OAUTH2_PROXY_CLIENT_SECRET", secretName, defaults.OAuthClientSecretKeyName))
	if oauth.SessionStoreType == v1.SessionStoreRedis {
		envVars = append(envVars, o.createEnvVarWithSecretRef("OAUTH2_PROXY_REDIS_PASSWORD", secretName, defaults.OAuthRedisPasswordKeyName))
	}

	addEnvVarIfSet("OAUTH2_PROXY_CLIENT_ID", oauth.ClientID)
	addEnvVarIfSet("OAUTH2_PROXY_SCOPE", oauth.Scope)
	addEnvVarIfSet("OAUTH2_PROXY_SET_XAUTHREQUEST", oauth.SetXAuthRequestHeaders)
	addEnvVarIfSet("OAUTH2_PROXY_PASS_ACCESS_TOKEN", oauth.SetXAuthRequestHeaders)
	addEnvVarIfSet("OAUTH2_PROXY_SET_AUTHORIZATION_HEADER", oauth.SetAuthorizationHeader)
	addEnvVarIfSet("OAUTH2_PROXY_PROXY_PREFIX", oauthutil.SanitizePathPrefix(oauth.ProxyPrefix))
	addEnvVarIfSet("OAUTH2_PROXY_LOGIN_URL", oauth.LoginURL)
	addEnvVarIfSet("OAUTH2_PROXY_REDEEM_URL", oauth.RedeemURL)
	addEnvVarIfSet("OAUTH2_PROXY_SESSION_STORE_TYPE", oauth.SessionStoreType)

	if oidc := oauth.OIDC; oidc != nil {
		addEnvVarIfSet("OAUTH2_PROXY_OIDC_ISSUER_URL", oidc.IssuerURL)
		addEnvVarIfSet("OAUTH2_PROXY_OIDC_JWKS_URL", oidc.JWKSURL)
		addEnvVarIfSet("OAUTH2_PROXY_SKIP_OIDC_DISCOVERY", oidc.SkipDiscovery)
		addEnvVarIfSet("OAUTH2_PROXY_INSECURE_OIDC_SKIP_NONCE", oidc.InsecureSkipVerifyNonce)
	}

	if cookie := oauth.Cookie; cookie != nil {
		addEnvVarIfSet("OAUTH2_PROXY_COOKIE_NAME", cookie.Name)
		addEnvVarIfSet("OAUTH2_PROXY_COOKIE_EXPIRE", cookie.Expire)
		addEnvVarIfSet("OAUTH2_PROXY_COOKIE_REFRESH", cookie.Refresh)
		addEnvVarIfSet("OAUTH2_PROXY_COOKIE_SAMESITE", cookie.SameSite)
	}

	if cookieStore := oauth.CookieStore; cookieStore != nil {
		addEnvVarIfSet("OAUTH2_PROXY_SESSION_COOKIE_MINIMAL", cookieStore.Minimal)
	}

	if redisStore := oauth.RedisStore; redisStore != nil {
		addEnvVarIfSet("OAUTH2_PROXY_REDIS_CONNECTION_URL", redisStore.ConnectionURL)
	}

	return envVars
}

func (o *oauthProxyResourceManager) createEnvVarWithSecretRef(envVarName, secretName, key string) corev1.EnvVar {
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

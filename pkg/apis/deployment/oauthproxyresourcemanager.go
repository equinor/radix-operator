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
	networkingv1 "k8s.io/api/networking/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/cache"
)

const (
	oauth2ProxyClientSecretEnvironmentVariable              = "OAUTH2_PROXY_CLIENT_SECRET"
	oauth2ProxyCookieSecretEnvironmentVariable              = "OAUTH2_PROXY_COOKIE_SECRET"
	oauth2ProxyRedisPasswordEnvironmentVariable             = "OAUTH2_PROXY_REDIS_PASSWORD"
	oauth2ProxyEntraIdFederatedTokenAuthEnvironmentVariable = "OAUTH2_PROXY_ENTRA_ID_FEDERATED_TOKEN_AUTH"
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

func (o *oauthProxyResourceManager) Sync(ctx context.Context) error {
	for _, component := range o.rd.Spec.Components {
		if err := o.syncComponent(ctx, &component); err != nil {
			return fmt.Errorf("failed to sync oauth proxy for component %s: %w", component.Name, err)
		}
	}
	return nil
}

func (o *oauthProxyResourceManager) syncComponent(ctx context.Context, component *v1.RadixDeployComponent) error {
	if auth := component.GetAuthentication(); component.IsPublic() && auth != nil && auth.OAuth2 != nil {
		o.logger.Debug().Msgf("Sync oauth proxy for the component %s", component.GetName())
		componentWithOAuthDefaults := component.DeepCopy()
		oauth, err := o.oauth2DefaultConfig.MergeWith(componentWithOAuthDefaults.Authentication.OAuth2)
		if err != nil {
			return err
		}
		componentWithOAuthDefaults.Authentication.OAuth2 = oauth
		return o.install(ctx, componentWithOAuthDefaults)
	}
	return o.uninstall(ctx, component)
}

func (o *oauthProxyResourceManager) GarbageCollect(ctx context.Context) error {
	if err := o.garbageCollect(ctx); err != nil {
		return fmt.Errorf("failed to garbage collect oauth2 proxy: %w", err)
	}
	return nil
}

func (o *oauthProxyResourceManager) garbageCollect(ctx context.Context) error {
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

	if err := o.garbageCollectServices(ctx); err != nil {
		return fmt.Errorf("failed to garbage collect services: %w", err)
	}

	if err := o.garbageCollectIngresses(ctx); err != nil {
		return fmt.Errorf("failed to garbage collect ingresses: %w", err)
	}

	return nil
}

func (o *oauthProxyResourceManager) garbageCollectDeployment(ctx context.Context) error {
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

func (o *oauthProxyResourceManager) garbageCollectSecrets(ctx context.Context) error {
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

func (o *oauthProxyResourceManager) garbageCollectServices(ctx context.Context) error {
	services, err := o.kubeutil.ListServices(ctx, o.rd.Namespace)
	if err != nil {
		return err
	}

	for _, service := range services {
		if o.isEligibleForGarbageCollection(service) {
			err := o.kubeutil.KubeClient().CoreV1().Services(service.Namespace).Delete(ctx, service.Name, metav1.DeleteOptions{})
			if err != nil && !kubeerrors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

func (o *oauthProxyResourceManager) garbageCollectIngresses(ctx context.Context) error {
	ingresses, err := o.kubeutil.ListIngresses(ctx, o.rd.Namespace)
	if err != nil {
		return err
	}

	for _, ing := range ingresses {
		if o.isEligibleForGarbageCollection(ing) {
			err := o.kubeutil.KubeClient().NetworkingV1().Ingresses(ing.Namespace).Delete(ctx, ing.Name, metav1.DeleteOptions{})
			if err != nil && !kubeerrors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

func (o *oauthProxyResourceManager) garbageCollectRoles(ctx context.Context) error {
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

func (o *oauthProxyResourceManager) garbageCollectRoleBinding(ctx context.Context) error {
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

func (o *oauthProxyResourceManager) isEligibleForGarbageCollection(object metav1.Object) bool {
	if appName := object.GetLabels()[kube.RadixAppLabel]; appName != o.rd.Spec.AppName {
		return false
	}
	if auxType := object.GetLabels()[kube.RadixAuxiliaryComponentTypeLabel]; auxType != v1.OAuthProxyAuxiliaryComponentType {
		return false
	}
	auxTargetComponentName, nameExist := RadixComponentNameFromAuxComponentLabel(object)
	if !nameExist {
		return false
	}
	return !auxTargetComponentName.ExistInDeploymentSpec(o.rd)
}

func (o *oauthProxyResourceManager) install(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	o.logger.Debug().Msgf("install the oauth proxy for the component %s", component.GetName())
	if err := o.createOrUpdateSecret(ctx, component); err != nil {
		return err
	}

	if err := o.createOrUpdateRbac(ctx, component); err != nil {
		return err
	}

	if err := o.createOrUpdateService(ctx, component); err != nil {
		return err
	}

	if err := o.createOrUpdateIngresses(ctx, component); err != nil {
		return err
	}

	if err := createOrUpdateOAuthProxyServiceAccount(ctx, o.kubeutil, o.rd, component); err != nil {
		return fmt.Errorf("failed to create OAuth proxy service account: %w", err)
	}
	if err := garbageCollectServiceAccountNoLongerInSpecForOAuthProxyComponent(ctx, o.kubeutil, o.rd, component); err != nil {
		return fmt.Errorf("failed to garbage collect service account no longer in spec for OAuth proxy component: %w", err)
	}
	return o.createOrUpdateDeployment(ctx, component)
}

func (o *oauthProxyResourceManager) uninstall(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	o.logger.Debug().Msgf("uninstall oauth proxy for the component %s", component.GetName())
	if err := o.deleteDeployment(ctx, component); err != nil {
		return err
	}

	if err := o.deleteIngresses(ctx, component); err != nil {
		return err
	}

	if err := o.deleteServices(ctx, component); err != nil {
		return err
	}

	if err := deleteOAuthProxyServiceAccounts(ctx, o.kubeutil, o.rd.Namespace, component); err != nil {
		return err
	}

	if err := o.deleteSecrets(ctx, component); err != nil {
		return err
	}

	if err := o.deleteRoleBindings(ctx, component); err != nil {
		return err
	}

	return o.deleteRoles(ctx, component)
}

func (o *oauthProxyResourceManager) deleteDeployment(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	selector := labels.SelectorFromValidatedSet(radixlabels.ForAuxComponent(o.rd.Spec.AppName, component)).String()
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

func (o *oauthProxyResourceManager) deleteIngresses(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	ingresses, err := o.getComponentAuxIngresses(ctx, component)
	if err != nil {
		return err
	}
	return o.kubeutil.DeleteIngresses(ctx, ingresses...)
}

func (o *oauthProxyResourceManager) deleteServices(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	selector := labels.SelectorFromValidatedSet(radixlabels.ForAuxComponent(o.rd.Spec.AppName, component)).String()
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

func (o *oauthProxyResourceManager) deleteSecrets(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	selector := labels.SelectorFromValidatedSet(radixlabels.ForAuxComponent(o.rd.Spec.AppName, component)).String()
	secrets, err := o.kubeutil.ListSecretsWithSelector(ctx, o.rd.Namespace, selector)
	if err != nil {
		return err
	}

	for _, secret := range secrets {
		if err := o.kubeutil.KubeClient().CoreV1().Secrets(secret.Namespace).Delete(ctx, secret.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
		o.logger.Info().Msgf("Deleted secret: %s in namespace %s", secret.GetName(), secret.Namespace)
	}

	return nil
}

func (o *oauthProxyResourceManager) deleteRoleBindings(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	selector := labels.SelectorFromValidatedSet(radixlabels.ForAuxComponent(o.rd.Spec.AppName, component)).String()
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

func (o *oauthProxyResourceManager) deleteRoles(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	selector := labels.SelectorFromValidatedSet(radixlabels.ForAuxComponent(o.rd.Spec.AppName, component)).String()
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

func (o *oauthProxyResourceManager) createOrUpdateIngresses(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	namespace := o.rd.Namespace
	o.logger.Debug().Msgf("create of update ingresses for the component %s in the namespace %s", component.GetName(), namespace)
	ingresses, err := o.getComponentIngresses(ctx, component)
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
		if err := o.kubeutil.ApplyIngress(ctx, namespace, auxIngress); err != nil {
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

func (o *oauthProxyResourceManager) getComponentIngresses(ctx context.Context, component v1.RadixCommonDeployComponent) ([]networkingv1.Ingress, error) {
	namespace := o.rd.Namespace
	return o.getIngressesForSelector(ctx, namespace,
		radixlabels.ForComponentDefaultAliasIngress(component),
		radixlabels.ForComponentActiveClusterAliasIngress(component),
		radixlabels.ForComponentAppAliasIngress(component),
		radixlabels.ForComponentExternalAliasIngress(component),
	)
}

func (o *oauthProxyResourceManager) getComponentAuxIngresses(ctx context.Context, component v1.RadixCommonDeployComponent) ([]networkingv1.Ingress, error) {
	appName := o.rd.Spec.AppName
	return o.getIngressesForSelector(ctx, o.rd.Namespace,
		radixlabels.ForAuxComponentDefaultIngress(appName, component),
		radixlabels.ForAuxComponentActiveClusterAliasIngress(appName, component),
		radixlabels.ForAuxComponentAppAliasIngress(appName, component),
		radixlabels.ForAuxComponentExternalAliasIngress(appName, component),
	)
}

func (o *oauthProxyResourceManager) getIngressesForSelector(ctx context.Context, namespace string, selectors ...labels.Set) ([]networkingv1.Ingress, error) {
	var ingresses []networkingv1.Ingress
	for _, selector := range selectors {
		ingressList, err := o.kubeutil.KubeClient().NetworkingV1().Ingresses(namespace).
			List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
		if err != nil {
			return nil, err
		}
		ingresses = append(ingresses, ingressList.Items...)
	}
	return ingresses, nil
}

func (o *oauthProxyResourceManager) createOrUpdateService(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	service := o.buildServiceSpec(component)
	return o.kubeutil.ApplyService(ctx, o.rd.Namespace, service)
}

func (o *oauthProxyResourceManager) createOrUpdateSecret(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	secretName := utils.GetAuxiliaryComponentSecretName(component.GetName(), v1.OAuthProxyAuxiliaryComponentSuffix)
	existingSecret, err := o.kubeutil.GetSecret(ctx, o.rd.Namespace, secretName)
	if err != nil {
		if !kubeerrors.IsNotFound(err) {
			return err
		}
		secret, err := buildOAuthProxySecret(o.rd.Spec.AppName, component)
		if err != nil {
			return err
		}
		_, err = o.kubeutil.CreateSecret(ctx, o.rd.Namespace, secret)
		return err
	}

	secret := existingSecret.DeepCopy()
	oauthutil.MergeAuxComponentResourceLabels(secret, o.rd.Spec.AppName, component)
	if _, ok := secret.Data[defaults.OAuthRedisPasswordKeyName]; ok && component.GetAuthentication().GetOAuth2().GetSessionStoreType() != v1.SessionStoreRedis {
		delete(secret.Data, defaults.OAuthRedisPasswordKeyName)
	}
	if _, ok := secret.Data[defaults.OAuthClientSecretKeyName]; ok && component.GetAuthentication().GetOAuth2().GetUseAzureIdentity() {
		delete(secret.Data, defaults.OAuthClientSecretKeyName)
	}
	_, err = o.kubeutil.UpdateSecret(ctx, existingSecret, secret)
	return err
}

func (o *oauthProxyResourceManager) createOrUpdateRbac(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	if err := o.createOrUpdateAppAdminRbac(ctx, component); err != nil {
		return err
	}

	return o.createOrUpdateAppReaderRbac(ctx, component)
}

func (o *oauthProxyResourceManager) createOrUpdateAppAdminRbac(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	secretName := utils.GetAuxiliaryComponentSecretName(component.GetName(), v1.OAuthProxyAuxiliaryComponentSuffix)
	roleName := o.getRoleAndRoleBindingName("radix-app-adm", component.GetName())
	namespace := o.rd.Namespace

	// create role
	role := kube.CreateAppRole(
		o.rd.Spec.AppName,
		roleName,
		radixlabels.ForAuxComponent(o.rd.Spec.AppName, component),
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

func (o *oauthProxyResourceManager) createOrUpdateAppReaderRbac(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	secretName := utils.GetAuxiliaryComponentSecretName(component.GetName(), v1.OAuthProxyAuxiliaryComponentSuffix)
	roleName := o.getRoleAndRoleBindingName("radix-app-reader", component.GetName())
	namespace := o.rd.Namespace

	// create role
	role := kube.CreateAppRole(
		o.rd.Spec.AppName,
		roleName,
		radixlabels.ForAuxComponent(o.rd.Spec.AppName, component),
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

func (o *oauthProxyResourceManager) getRoleAndRoleBindingName(prefix, componentName string) string {
	deploymentName := utils.GetAuxiliaryComponentDeploymentName(componentName, v1.OAuthProxyAuxiliaryComponentSuffix)
	return fmt.Sprintf("%s-%s", prefix, deploymentName)
}

func buildOAuthProxySecret(appName string, component v1.RadixCommonDeployComponent) (*corev1.Secret, error) {
	secretName := utils.GetAuxiliaryComponentSecretName(component.GetName(), v1.OAuthProxyAuxiliaryComponentSuffix)

	secret := &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
		},
		Data: make(map[string][]byte),
	}
	oauthutil.MergeAuxComponentResourceLabels(secret, appName, component)
	cookieSecret, err := generateRandomCookieSecret()
	if err != nil {
		return nil, err
	}
	secret.Data[defaults.OAuthCookieSecretKeyName] = cookieSecret
	return secret, nil
}

func (o *oauthProxyResourceManager) buildServiceSpec(component v1.RadixCommonDeployComponent) *corev1.Service {
	serviceName := utils.GetAuxiliaryComponentServiceName(component.GetName(), v1.OAuthProxyAuxiliaryComponentSuffix)

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

func generateRandomCookieSecret() ([]byte, error) {
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

func (o *oauthProxyResourceManager) createOrUpdateDeployment(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	current, desired, err := o.getCurrentAndDesiredDeployment(ctx, component)
	if err != nil {
		return err
	}

	if err := o.kubeutil.ApplyDeployment(ctx, o.rd.Namespace, current, desired); err != nil {
		return err
	}
	return nil
}

func (o *oauthProxyResourceManager) getCurrentAndDesiredDeployment(ctx context.Context, component v1.RadixCommonDeployComponent) (*appsv1.Deployment, *appsv1.Deployment, error) {
	deploymentName := utils.GetAuxiliaryComponentDeploymentName(component.GetName(), v1.OAuthProxyAuxiliaryComponentSuffix)

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

func (o *oauthProxyResourceManager) getDesiredDeployment(component v1.RadixCommonDeployComponent) (*appsv1.Deployment, error) {
	componentName := component.GetName()
	deploymentName := utils.GetAuxiliaryComponentDeploymentName(componentName, v1.OAuthProxyAuxiliaryComponentSuffix)
	oauth2 := component.GetAuthentication().GetOAuth2()
	readinessProbe, err := getReadinessProbeWithDefaultsFromEnv(defaults.OAuthProxyPortNumber)
	if err != nil {
		return nil, err
	}

	var replicas int32 = 1
	if isComponentStopped(component) || componentHasZeroReplicas(component) {
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
				MatchLabels: radixlabels.ForAuxComponent(o.rd.Spec.AppName, component),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: radixlabels.Merge(
						radixlabels.ForAuxComponent(o.rd.Spec.AppName, component),
						radixlabels.ForOAuthProxyPodWithRadixIdentity(oauth2),
					),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            componentName,
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
					SecurityContext:    securitycontext.Pod(securitycontext.WithPodSeccompProfile(corev1.SeccompProfileTypeRuntimeDefault)),
					Affinity:           utils.GetAffinityForOAuthAuxComponent(),
					ServiceAccountName: oauth2.GetServiceAccountName(componentName),
				},
			},
		},
	}
	oauthutil.MergeAuxComponentResourceLabels(desiredDeployment, o.rd.Spec.AppName, component)
	return desiredDeployment, nil
}

func componentHasZeroReplicas(component v1.RadixCommonDeployComponent) bool {
	return component.GetReplicas() != nil && *component.GetReplicas() == 0
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
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_PROVIDER", Value: getOAuthProxyProvider(oauth)})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_COOKIE_HTTPONLY", Value: "true"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_COOKIE_SECURE", Value: "true"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_PASS_BASIC_AUTH", Value: "false"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_SKIP_PROVIDER_BUTTON", Value: "true"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_EMAIL_DOMAINS", Value: "*"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_SKIP_CLAIMS_FROM_PROFILE_URL", Value: "true"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_HTTP_ADDRESS", Value: fmt.Sprintf("%s://:%v", "http", defaults.OAuthProxyPortNumber)})
	secretName := utils.GetAuxiliaryComponentSecretName(component.GetName(), v1.OAuthProxyAuxiliaryComponentSuffix)
	envVars = append(envVars, o.createEnvVarWithSecretRef(oauth2ProxyCookieSecretEnvironmentVariable, secretName, defaults.OAuthCookieSecretKeyName))

	if oauth.GetUseAzureIdentity() {
		envVars = append(envVars, corev1.EnvVar{Name: oauth2ProxyEntraIdFederatedTokenAuthEnvironmentVariable, Value: "true"})
	} else {
		envVars = append(envVars, o.createEnvVarWithSecretRef(oauth2ProxyClientSecretEnvironmentVariable, secretName, defaults.OAuthClientSecretKeyName))
	}

	if oauth.SessionStoreType == v1.SessionStoreRedis {
		envVars = append(envVars, o.createEnvVarWithSecretRef(oauth2ProxyRedisPasswordEnvironmentVariable, secretName, defaults.OAuthRedisPasswordKeyName))
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

func getOAuthProxyProvider(oauth *v1.OAuth2) string {
	if oauth.GetUseAzureIdentity() {
		return defaults.OAuthProxyProviderEntraId
	}
	return defaults.OAuthProxyProviderOIDC
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

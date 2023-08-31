package deployment

import (
	"context"
	"encoding/base64"
	"fmt"
	"reflect"

	commonutils "github.com/equinor/radix-common/utils"
	radixmaps "github.com/equinor/radix-common/utils/maps"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/securitycontext"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	oauthutil "github.com/equinor/radix-operator/pkg/apis/utils/oauth"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	oauthProxyPortName                  = "http"
	oauthProxyPortNumber          int32 = 4180
	authUrlAnnotation                   = "nginx.ingress.kubernetes.io/auth-url"
	authSigninAnnotation                = "nginx.ingress.kubernetes.io/auth-signin"
	authResponseHeadersAnnotation       = "nginx.ingress.kubernetes.io/auth-response-headers"
)

// NewOAuthProxyResourceManager creates a new OAuthProxyResourceManager
func NewOAuthProxyResourceManager(rd *v1.RadixDeployment, rr *v1.RadixRegistration, kubeutil *kube.Kube, oauth2DefaultConfig defaults.OAuth2Config, ingressAnnotationProviders []IngressAnnotationProvider, oauth2ProxyDockerImage string) AuxiliaryResourceManager {
	return &oauthProxyResourceManager{
		rd:                         rd,
		rr:                         rr,
		kubeutil:                   kubeutil,
		ingressAnnotationProviders: ingressAnnotationProviders,
		oauth2DefaultConfig:        oauth2DefaultConfig,
		oauth2ProxyDockerImage:     oauth2ProxyDockerImage,
	}
}

type oauthProxyResourceManager struct {
	rd                         *v1.RadixDeployment
	rr                         *v1.RadixRegistration
	kubeutil                   *kube.Kube
	ingressAnnotationProviders []IngressAnnotationProvider
	oauth2DefaultConfig        defaults.OAuth2Config
	oauth2ProxyDockerImage     string
}

func (o *oauthProxyResourceManager) Sync() error {
	for _, component := range o.rd.Spec.Components {
		if err := o.syncComponent(&component); err != nil {
			return fmt.Errorf("failed to sync oauth proxy: %v", err)
		}
	}
	return nil
}

func (o *oauthProxyResourceManager) syncComponent(component *v1.RadixDeployComponent) error {
	if auth := component.GetAuthentication(); component.IsPublic() && auth != nil && auth.OAuth2 != nil {
		componentWithOAuthDefaults := component.DeepCopy()
		oauth, err := o.oauth2DefaultConfig.MergeWith(componentWithOAuthDefaults.Authentication.OAuth2)
		if err != nil {
			return err
		}
		componentWithOAuthDefaults.Authentication.OAuth2 = oauth
		return o.install(componentWithOAuthDefaults)
	} else {
		return o.uninstall(component)
	}
}

func (o *oauthProxyResourceManager) GarbageCollect() error {
	if err := o.garbageCollect(); err != nil {
		return fmt.Errorf("failed to garbage collect oauth proxy: %v", err)
	}
	return nil
}

func (o *oauthProxyResourceManager) garbageCollect() error {
	if err := o.garbageCollectDeployment(); err != nil {
		return err
	}

	if err := o.garbageCollectSecrets(); err != nil {
		return err
	}

	if err := o.garbageCollectRoles(); err != nil {
		return err
	}

	if err := o.garbageCollectRoleBinding(); err != nil {
		return err
	}

	if err := o.garbageCollectServices(); err != nil {
		return err
	}

	return o.garbageCollectIngresses()
}

func (o *oauthProxyResourceManager) garbageCollectDeployment() error {
	deployments, err := o.kubeutil.ListDeployments(o.rd.Namespace)
	if err != nil {
		return err
	}

	for _, deployment := range deployments {
		if o.isEligibleForGarbageCollection(deployment) {
			err := o.kubeutil.KubeClient().AppsV1().Deployments(deployment.Namespace).Delete(context.TODO(), deployment.Name, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
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
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
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
			if err != nil && !errors.IsNotFound(err) {
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

	for _, ingress := range ingresses {
		if o.isEligibleForGarbageCollection(ingress) {
			err := o.kubeutil.KubeClient().NetworkingV1().Ingresses(ingress.Namespace).Delete(context.TODO(), ingress.Name, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
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
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

func (o *oauthProxyResourceManager) garbageCollectRoleBinding() error {
	rolebindings, err := o.kubeutil.ListRoleBindings(o.rd.Namespace)
	if err != nil {
		return err
	}

	for _, rolebinding := range rolebindings {
		if o.isEligibleForGarbageCollection(rolebinding) {
			err := o.kubeutil.KubeClient().RbacV1().RoleBindings(rolebinding.Namespace).Delete(context.TODO(), rolebinding.Name, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
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
	if err := o.createOrUpdateSecret(component); err != nil {
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
	selector := labels.SelectorFromValidatedSet(o.getLabelsForAuxComponent(component)).String()
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
	selector := labels.SelectorFromValidatedSet(o.getLabelsForAuxComponent(component)).String()
	ingresses, err := o.kubeutil.ListIngressesWithSelector(o.rd.Namespace, selector)
	if err != nil {
		return err
	}

	for _, ingress := range ingresses {
		if err := o.kubeutil.KubeClient().NetworkingV1().Ingresses(ingress.Namespace).Delete(context.TODO(), ingress.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (o *oauthProxyResourceManager) deleteServices(component v1.RadixCommonDeployComponent) error {
	selector := labels.SelectorFromValidatedSet(o.getLabelsForAuxComponent(component)).String()
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
	selector := labels.SelectorFromValidatedSet(o.getLabelsForAuxComponent(component)).String()
	secrets, err := o.kubeutil.ListSecretsWithSelector(o.rd.Namespace, selector)
	if err != nil {
		return err
	}

	for _, secret := range secrets {
		if err := o.kubeutil.KubeClient().CoreV1().Secrets(secret.Namespace).Delete(context.TODO(), secret.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (o *oauthProxyResourceManager) deleteRoleBindings(component v1.RadixCommonDeployComponent) error {
	selector := labels.SelectorFromValidatedSet(o.getLabelsForAuxComponent(component)).String()
	rolebindings, err := o.kubeutil.ListRoleBindingsWithSelector(o.rd.Namespace, selector)
	if err != nil {
		return err
	}

	for _, rolebinding := range rolebindings {
		if err := o.kubeutil.KubeClient().RbacV1().RoleBindings(rolebinding.Namespace).Delete(context.TODO(), rolebinding.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (o *oauthProxyResourceManager) deleteRoles(component v1.RadixCommonDeployComponent) error {
	selector := labels.SelectorFromValidatedSet(o.getLabelsForAuxComponent(component)).String()
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
	listOptions := metav1.ListOptions{LabelSelector: getLabelSelectorForComponent(component)}
	ingresses, err := o.kubeutil.KubeClient().NetworkingV1().Ingresses(o.rd.Namespace).List(context.TODO(), listOptions)
	if err != nil {
		return err
	}

	for _, ingress := range ingresses.Items {
		auxIngress, err := o.buildOAuthProxyIngressForComponentIngress(component, ingress)
		if err != nil {
			return err
		}
		if auxIngress != nil {
			if err := o.kubeutil.ApplyIngress(o.rd.Namespace, auxIngress); err != nil {
				return err
			}
		}
	}

	return nil
}

func (o *oauthProxyResourceManager) buildOAuthProxyIngressForComponentIngress(component v1.RadixCommonDeployComponent, componentIngress networkingv1.Ingress) (*networkingv1.Ingress, error) {
	if len(componentIngress.Spec.Rules) == 0 {
		return nil, nil
	}
	sourceHost := componentIngress.Spec.Rules[0]
	pathType := networkingv1.PathTypeImplementationSpecific
	annotations := map[string]string{}

	for _, ia := range o.ingressAnnotationProviders {
		providedAnnotations, err := ia.GetAnnotations(component)
		if err != nil {
			return nil, err
		}
		annotations = radixmaps.MergeMaps(annotations, providedAnnotations)
	}

	var tls []networkingv1.IngressTLS
	for _, sourceTls := range componentIngress.Spec.TLS {
		tls = append(tls, *sourceTls.DeepCopy())
	}

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:            o.getIngressName(componentIngress.GetName()),
			Annotations:     annotations,
			OwnerReferences: o.getOwnerReferenceOfIngress(&componentIngress),
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: componentIngress.Spec.IngressClassName,
			TLS:              tls,
			Rules: []networkingv1.IngressRule{
				{
					Host: sourceHost.Host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     oauthutil.SanitizePathPrefix(component.GetAuthentication().OAuth2.ProxyPrefix),
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: utils.GetAuxiliaryComponentServiceName(component.GetName(), defaults.OAuthProxyAuxiliaryComponentSuffix),
											Port: networkingv1.ServiceBackendPort{
												Number: oauthProxyPortNumber,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	o.mergeAuxComponentResourceLabels(ingress, component)
	return ingress, nil
}

func (o *oauthProxyResourceManager) getOwnerReferenceOfIngress(ingress *networkingv1.Ingress) []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			APIVersion: "networking.k8s.io/v1",
			Kind:       "Ingress",
			Name:       ingress.Name,
			UID:        ingress.UID,
			Controller: utils.BoolPtr(true),
		},
	}
}

func (o *oauthProxyResourceManager) createOrUpdateService(component v1.RadixCommonDeployComponent) error {
	service := o.buildServiceSpec(component)
	return o.kubeutil.ApplyService(o.rd.Namespace, service)
}

func (o *oauthProxyResourceManager) createOrUpdateSecret(component v1.RadixCommonDeployComponent) error {
	secretName := utils.GetAuxiliaryComponentSecretName(component.GetName(), defaults.OAuthProxyAuxiliaryComponentSuffix)
	secret, err := o.kubeutil.GetSecret(o.rd.Namespace, secretName)

	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		secret, err = o.buildSecretSpec(component)
		if err != nil {
			return err
		}
	} else {
		o.mergeAuxComponentResourceLabels(secret, component)
		if component.GetAuthentication().OAuth2.SessionStoreType != v1.SessionStoreRedis {
			if secret.Data != nil && len(secret.Data[defaults.OAuthRedisPasswordKeyName]) > 0 {
				delete(secret.Data, defaults.OAuthRedisPasswordKeyName)
			}
		}
	}

	if _, err := o.kubeutil.ApplySecret(o.rd.Namespace, secret); err != nil {
		return err
	}

	return o.grantAccessToSecret(component)
}

func (o *oauthProxyResourceManager) mergeAuxComponentResourceLabels(object metav1.Object, component v1.RadixCommonDeployComponent) {
	object.SetLabels(labels.Merge(object.GetLabels(), o.getLabelsForAuxComponent(component)))
}

func (o *oauthProxyResourceManager) grantAccessToSecret(component v1.RadixCommonDeployComponent) error {
	secretName := utils.GetAuxiliaryComponentSecretName(component.GetName(), defaults.OAuthProxyAuxiliaryComponentSuffix)
	deploymentName := utils.GetAuxiliaryComponentDeploymentName(component.GetName(), defaults.OAuthProxyAuxiliaryComponentSuffix)
	roleName := o.getAppAdminRoleAndRoleBindingName(component.GetName())
	namespace := o.rd.Namespace

	// create role
	role := kube.CreateAppRole(
		o.rd.Spec.AppName,
		roleName,
		o.getLabelsForAuxComponent(component),
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

	// Add machine user to subjects
	if o.rr.Spec.MachineUser {
		subjects = append(subjects, rbacv1.Subject{
			Kind:      "ServiceAccount",
			Name:      defaults.GetMachineUserRoleName(o.rr.Name),
			Namespace: utils.GetAppNamespace(o.rr.Name),
		})
	}

	rolebinding := kube.GetRolebindingToRoleWithLabelsForSubjects(roleName, subjects, role.Labels)
	return o.kubeutil.ApplyRoleBinding(namespace, rolebinding)
}

func (o *oauthProxyResourceManager) getAppAdminRoleAndRoleBindingName(componentName string) string {
	deploymentName := utils.GetAuxiliaryComponentDeploymentName(componentName, defaults.OAuthProxyAuxiliaryComponentSuffix)
	return fmt.Sprintf("radix-app-adm-%s", deploymentName)
}

func (o *oauthProxyResourceManager) getIngressName(sourceIngressName string) string {
	return fmt.Sprintf("%s-%s", sourceIngressName, defaults.OAuthProxyAuxiliaryComponentSuffix)
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
	o.mergeAuxComponentResourceLabels(secret, component)
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
			Selector: o.getLabelsForAuxComponent(component),
			Ports: []corev1.ServicePort{
				{
					Port:       oauthProxyPortNumber,
					TargetPort: intstr.FromString(oauthProxyPortName),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	o.mergeAuxComponentResourceLabels(service, component)
	return service
}

func (o *oauthProxyResourceManager) getLabelsForAuxComponent(component v1.RadixCommonDeployComponent) map[string]string {
	return map[string]string{
		kube.RadixAppLabel:                    o.rd.Spec.AppName,
		kube.RadixAuxiliaryComponentLabel:     component.GetName(),
		kube.RadixAuxiliaryComponentTypeLabel: defaults.OAuthProxyAuxiliaryComponentType,
	}
}

func (o *oauthProxyResourceManager) generateRandomCookieSecret() ([]byte, error) {
	randomBytes := commonutils.GenerateRandomKey(32)
	// Extra check to make sure correct number of bytes are returned for the random key
	if len(randomBytes) != 32 {
		return nil, fmt.Errorf("failed to generator cookie secret")
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
	if err != nil && !errors.IsNotFound(err) {
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
	readinessProbe, err := getReadinessProbeWithDefaultsFromEnv(oauthProxyPortNumber)
	if err != nil {
		return nil, err
	}

	// Spec.Strategy defaults to RollingUpdate, ref https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#strategy
	desiredDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            deploymentName,
			Annotations:     make(map[string]string),
			OwnerReferences: []metav1.OwnerReference{getOwnerReferenceOfDeployment(o.rd)},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(DefaultReplicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: o.getLabelsForAuxComponent(component),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: o.getLabelsForAuxComponent(component),
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
									Name:          oauthProxyPortName,
									ContainerPort: oauthProxyPortNumber,
								},
							},
							ReadinessProbe:  readinessProbe,
							SecurityContext: securitycontext.Container(securitycontext.WithContainerSeccompProfileType(corev1.SeccompProfileTypeRuntimeDefault)),
						},
					},
					SecurityContext: securitycontext.Pod(securitycontext.WithPodSeccompProfile(corev1.SeccompProfileTypeRuntimeDefault)),
				},
			},
		},
	}

	o.mergeAuxComponentResourceLabels(desiredDeployment, component)
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

	// Add fixed envvars
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_PROVIDER", Value: "oidc"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_COOKIE_HTTPONLY", Value: "true"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_COOKIE_SECURE", Value: "true"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_PASS_BASIC_AUTH", Value: "false"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_SKIP_PROVIDER_BUTTON", Value: "true"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_EMAIL_DOMAINS", Value: "*"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_HTTP_ADDRESS", Value: fmt.Sprintf("http://:%v", oauthProxyPortNumber)})
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

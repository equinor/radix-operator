package deployment

import (
	"context"
	"encoding/base64"
	"fmt"
	"reflect"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/imdario/mergo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	oauthProxyImage               = "quay.io/oauth2-proxy/oauth2-proxy:v7.2.0"
	oauthProxyPortName            = "http"
	oauthProxyPortNumber          = 4180
	authUrlAnnotation             = "nginx.ingress.kubernetes.io/auth-url"
	authSigninAnnotation          = "nginx.ingress.kubernetes.io/auth-signin"
	authResponseHeadersAnnotation = "nginx.ingress.kubernetes.io/auth-response-headers"
)

// OAuthProxyResourceManager contains methods to configure oauth authentication for a component
type OAuthProxyResourceManager interface {
	// Sync creates, updates or removes resources to handle the oauth code flow
	Sync(component v1.RadixCommonDeployComponent) error
	// ConfigureRootIngress sets annotations required by nginx to handle authentication on the ingress resource
	// This method should be called with the ingress containing the root path ("/") as argument
	ConfigureRootIngress(ingress *networkingv1.Ingress, component v1.RadixCommonDeployComponent)
}

// NewOAuthProxyResourceManager creates a new OAuthProxyResourceManager
func NewOAuthProxyResourceManager(rd *v1.RadixDeployment, rr *v1.RadixRegistration, kubeutil *kube.Kube) OAuthProxyResourceManager {
	return &oauthProxyResourceManager{rd: rd, rr: rr, kubeutil: kubeutil}
}

type oauthProxyResourceManager struct {
	rd       *v1.RadixDeployment
	rr       *v1.RadixRegistration
	kubeutil *kube.Kube
}

func (o *oauthProxyResourceManager) Sync(component v1.RadixCommonDeployComponent) error {
	isPublic := component.GetPublicPort() != "" || component.IsPublic()

	if auth := component.GetAuthentication(); auth != nil && auth.OAuth2 != nil && isPublic {
		return o.install(component)
	} else {
		return o.uninstall(component)
	}
}

func (o *oauthProxyResourceManager) ConfigureRootIngress(ingress *networkingv1.Ingress, component v1.RadixCommonDeployComponent) {
	if auth := component.GetAuthentication(); auth != nil && auth.OAuth2 != nil {
		oauth := oauth2DefaultsWithSource(auth.OAuth2)
		o.setRootIngressAnnotations(ingress, oauth)
	}
}

func (o *oauthProxyResourceManager) setRootIngressAnnotations(ingress *networkingv1.Ingress, oauth *v1.OAuth2) {
	if ingress.Annotations == nil {
		ingress.Annotations = make(map[string]string)
	}

	ingress.Annotations[authUrlAnnotation] = fmt.Sprintf("https://$host%s/auth", oauth.ProxyPrefix)
	ingress.Annotations[authSigninAnnotation] = fmt.Sprintf("https://$host%s/start?rd=$escaped_request_uri", oauth.ProxyPrefix)

	var authResponseHeaders []string
	if oauth.SetXAuthRequestHeaders != nil && *oauth.SetXAuthRequestHeaders {
		authResponseHeaders = append(authResponseHeaders, "X-Auth-Request-Access-Token", "X-Auth-Request-User", "X-Auth-Request-Groups", "X-Auth-Request-Email", "X-Auth-Request-Preferred-Username")
	}
	if oauth.SetAuthorizationHeader != nil && *oauth.SetAuthorizationHeader {
		authResponseHeaders = append(authResponseHeaders, "Authorization")
	}
	if len(authResponseHeaders) > 0 {
		ingress.Annotations[authResponseHeadersAnnotation] = strings.Join(authResponseHeaders, ",")
	}
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
	return nil
}

func (o *oauthProxyResourceManager) createOrUpdateIngresses(component v1.RadixCommonDeployComponent) error {
	listOptions := metav1.ListOptions{LabelSelector: getLabelSelectorForComponent(component)}
	ingresses, err := o.kubeutil.KubeClient().NetworkingV1().Ingresses(o.rd.Namespace).List(context.TODO(), listOptions)
	if err != nil {
		return err
	}

	for _, ingress := range ingresses.Items {
		auxIngress := o.buildOAuthProxyIngressForComponentIngress(component, ingress)
		if err := o.kubeutil.ApplyIngress(o.rd.Namespace, auxIngress); err != nil {
			return err
		}
	}

	return nil
}

func (o *oauthProxyResourceManager) buildOAuthProxyIngressForComponentIngress(component v1.RadixCommonDeployComponent, componentIngress networkingv1.Ingress) *networkingv1.Ingress {
	oauth := oauth2DefaultsWithSource(component.GetAuthentication().OAuth2)
	sourceHost := componentIngress.Spec.Rules[0]
	pathType := networkingv1.PathTypeImplementationSpecific

	ingress := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", componentIngress.Name, defaults.OAuthProxyAuxiliaryComponentSuffix),
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: componentIngress.Spec.IngressClassName,
			TLS: []networkingv1.IngressTLS{
				*componentIngress.Spec.TLS[0].DeepCopy(),
			},
			Rules: []networkingv1.IngressRule{
				{
					Host: sourceHost.Host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     oauth.ProxyPrefix,
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
	return &ingress
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
		if component.GetAuthentication().OAuth2.SessionStoreType != v1.SessionStoreRedis {
			if secret.Data != nil && len(secret.Data[defaults.OAuthCookieSecretKeyName]) > 0 {
				delete(secret.Data, defaults.OAuthRedisPasswordKeyName)
			}
		}
	}

	if _, err := o.kubeutil.ApplySecret(o.rd.Namespace, secret); err != nil {
		return err
	}

	return o.grantAccessToSecret(component)
}

func (o *oauthProxyResourceManager) grantAccessToSecret(component v1.RadixCommonDeployComponent) error {
	secretName := utils.GetAuxiliaryComponentSecretName(component.GetName(), defaults.OAuthProxyAuxiliaryComponentSuffix)
	roleName := o.getRoleAndRoleBindingName(component)
	namespace := o.rd.Namespace

	// create role
	role := kube.CreateManageSecretRole(
		o.rd.Spec.AppName,
		roleName,
		[]string{secretName},
		o.getLabelsForAuxComponent(component),
	)

	err := o.kubeutil.ApplyRole(namespace, role)
	if err != nil {
		return err
	}

	// create rolebinding
	adGroups, err := application.GetAdGroups(o.rr)
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

func (o *oauthProxyResourceManager) getRoleAndRoleBindingName(component v1.RadixCommonDeployComponent) string {
	deploymentName := utils.GetAuxiliaryComponentDeploymentName(component.GetName(), defaults.OAuthProxyAuxiliaryComponentSuffix)
	return fmt.Sprintf("radix-app-adm-%s", deploymentName)
}

func (o *oauthProxyResourceManager) buildSecretSpec(component v1.RadixCommonDeployComponent) (*corev1.Secret, error) {
	secretName := utils.GetAuxiliaryComponentSecretName(component.GetName(), defaults.OAuthProxyAuxiliaryComponentSuffix)

	secret := &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name:   secretName,
			Labels: o.getLabelsForAuxComponent(component),
		},
		Data: make(map[string][]byte),
	}

	cookieSecret, err := o.generateRandomCookieSecret()
	if err != nil {
		return nil, err
	}
	secret.Data[defaults.OAuthCookieSecretKeyName] = cookieSecret
	return secret, nil
}

func (o *oauthProxyResourceManager) buildServiceSpec(component v1.RadixCommonDeployComponent) *corev1.Service {
	serviceName := utils.GetAuxiliaryComponentServiceName(component.GetName(), defaults.OAuthProxyAuxiliaryComponentSuffix)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   serviceName,
			Labels: o.getLabelsForAuxComponent(component),
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
}

func (o *oauthProxyResourceManager) getLabelsForAuxComponent(component v1.RadixCommonDeployComponent) map[string]string {
	return map[string]string{
		kube.RadixAppLabel:                    o.rd.Spec.AppName,
		kube.RadixAuxiliaryComponentLabel:     component.GetName(),
		kube.RadixAuxiliaryComponentTypeLabel: string(defaults.OAuthProxyAuxiliaryComponent),
	}
}

func (o *oauthProxyResourceManager) generateRandomCookieSecret() ([]byte, error) {
	randomBytes := utils.GenerateRandomKey(32)
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
	readinessProbe, err := getReadinessProbe(oauthProxyPortNumber)
	if err != nil {
		return nil, err
	}

	desiredDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            deploymentName,
			Labels:          o.getLabelsForAuxComponent(component),
			Annotations:     make(map[string]string),
			OwnerReferences: getOwnerReferenceOfDeployment(o.rd),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(DefaultReplicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: o.getLabelsForAuxComponent(component),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: o.getLabelsForAuxComponent(component),
					Annotations: map[string]string{
						"apparmor.security.beta.kubernetes.io/pod": "runtime/default",
						"seccomp.security.alpha.kubernetes.io/pod": "docker/default",
					},
				},
				Spec: corev1.PodSpec{Containers: []corev1.Container{
					{
						Name:            component.GetName(),
						Image:           oauthProxyImage,
						ImagePullPolicy: corev1.PullAlways,
						Env:             o.getEnvVars(component),
						Ports: []corev1.ContainerPort{
							{
								Name:          oauthProxyPortName,
								ContainerPort: oauthProxyPortNumber,
							},
						},
						ReadinessProbe: readinessProbe,
					},
				}},
			},
		},
	}

	return desiredDeployment, nil
}

func (o *oauthProxyResourceManager) getEnvVars(component v1.RadixCommonDeployComponent) []corev1.EnvVar {
	var envVars []corev1.EnvVar
	oauth := oauth2DefaultsWithSource(component.GetAuthentication().OAuth2)

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
	secretName := utils.GetAuxiliaryComponentSecretName(component.GetName(), defaults.OAuthProxyAuxiliaryComponentSuffix)

	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_PROVIDER", Value: "oidc"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_COOKIE_HTTPONLY", Value: "true"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_COOKIE_SECURE", Value: "true"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_PASS_BASIC_AUTH", Value: "false"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_SKIP_PROVIDER_BUTTON", Value: "true"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_EMAIL_DOMAINS", Value: "*"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_HTTP_ADDRESS", Value: fmt.Sprintf("http://:%v", oauthProxyPortNumber)})
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
	addEnvVarIfSet("OAUTH2_PROXY_PROXY_PREFIX", oauth.ProxyPrefix)
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
		addEnvVarIfSet("OAUTH2_PROXY_COOKIE_MINIMAL", cookieStore.Minimal)
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

func oauth2DefaultsWithSource(source *v1.OAuth2) *v1.OAuth2 {
	target := &v1.OAuth2{
		Scope:                  "openid profile email",
		SetXAuthRequestHeaders: utils.BoolPtr(false),
		SetAuthorizationHeader: utils.BoolPtr(false),
		ProxyPrefix:            "/oauth2",
		SessionStoreType:       "cookie",
		OIDC: &v1.OAuth2OIDC{
			IssuerURL:               "https://login.microsoftonline.com/3aa4a235-b6e2-48d5-9195-7fcf05b459b0/v2.0",
			InsecureSkipVerifyNonce: utils.BoolPtr(false),
		},
		Cookie: &v1.OAuth2Cookie{
			Name:    "_oauth2_proxy",
			Expire:  "168h0m0s",
			Refresh: "60m0s",
		},
	}

	mergo.Merge(target, source, mergo.WithOverride, mergo.WithTransformers(transformer))
	return target
}

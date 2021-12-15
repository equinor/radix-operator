package deployment

import (
	"encoding/base64"
	"fmt"
	"reflect"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/imdario/mergo"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	oauthProxyImage      = "quay.io/oauth2-proxy/oauth2-proxy:v7.2.0"
	oauthProxyPortName   = "http"
	oauthProxyPortNumber = 4180
)

type OAuthProxyResourceManager interface {
	Sync(component v1.RadixCommonDeployComponent) error
}

func NewOAuthProxyResourceManager(rd *v1.RadixDeployment, kubeutil *kube.Kube) OAuthProxyResourceManager {
	return &oauthProxyResourceManager{rd: rd, kubeutil: kubeutil}
}

type oauthProxyResourceManager struct {
	rd       *v1.RadixDeployment
	kubeutil *kube.Kube
}

func (o *oauthProxyResourceManager) Sync(component v1.RadixCommonDeployComponent) error {
	isPublic := component.GetPublicPort() != "" || component.IsPublic()

	if auth := component.GetAuthentication(); auth != nil && auth.OAuth2 != nil && isPublic {
		return o.install(component)
	} else {
		return o.uninstall(component)
	}

	// build secret+role+binding
	// build deployment
	// build service
	// build ingress

}

func (o *oauthProxyResourceManager) install(component v1.RadixCommonDeployComponent) error {
	if err := o.createOrUpdateSecret(component); err != nil {
		return err
	}
	return o.createOrUpdateDeployment(component)
}

func (o *oauthProxyResourceManager) uninstall(component v1.RadixCommonDeployComponent) error {
	return nil
}

func (o *oauthProxyResourceManager) createOrUpdateSecret(component v1.RadixCommonDeployComponent) error {
	secretName := utils.GetAuxiliaryComponentSecretName(component.GetName(), defaults.OAuthProxyAuxiliaryComponent)
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
		logrus.Debug("")
	}

	if _, err := o.kubeutil.ApplySecret(o.rd.Namespace, secret); err != nil {
		return err
	}

	return nil
}

func (o *oauthProxyResourceManager) buildSecretSpec(component v1.RadixCommonDeployComponent) (*corev1.Secret, error) {
	secretName := utils.GetAuxiliaryComponentSecretName(component.GetName(), defaults.OAuthProxyAuxiliaryComponent)

	secret := &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: map[string]string{
				kube.RadixAppLabel:               o.rd.Spec.AppName,
				kube.RadixComponentLabel:         component.GetName(),
				kube.RadixComponentAuxiliaryType: string(defaults.OAuthProxyAuxiliaryComponent),
			},
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
	//Auxiliary
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
	deploymentName := utils.GetAuxiliaryComponentDeploymentName(component.GetName(), defaults.OAuthProxyAuxiliaryComponent)

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
	deploymentName := utils.GetAuxiliaryComponentDeploymentName(component.GetName(), defaults.OAuthProxyAuxiliaryComponent)
	readinessProbe, err := getReadinessProbe(oauthProxyPortNumber)
	if err != nil {
		return nil, err
	}

	desiredDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploymentName,
			Labels: map[string]string{
				kube.RadixAppLabel:               o.rd.Spec.AppName,
				kube.RadixComponentLabel:         component.GetName(),
				kube.RadixComponentAuxiliaryType: string(defaults.OAuthProxyAuxiliaryComponent),
			},
			Annotations:     make(map[string]string),
			OwnerReferences: getOwnerReferenceOfDeployment(o.rd),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(DefaultReplicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					kube.RadixComponentLabel:         component.GetName(),
					kube.RadixComponentAuxiliaryType: string(defaults.OAuthProxyAuxiliaryComponent),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						kube.RadixAppLabel:               o.rd.Spec.AppName,
						kube.RadixComponentLabel:         component.GetName(),
						kube.RadixComponentAuxiliaryType: string(defaults.OAuthProxyAuxiliaryComponent),
					},
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
	secretName := utils.GetAuxiliaryComponentSecretName(component.GetName(), defaults.OAuthProxyAuxiliaryComponent)

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
		Scope:                  "openid profile email offline_acccess",
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

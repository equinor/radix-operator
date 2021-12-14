package deployment

import (
	"fmt"
	"reflect"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
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
	Install(component v1.RadixCommonDeployComponent) error
	Uninstall(componentName string) error
}

func NewOAuthProxyResourceManager(rd *v1.RadixDeployment, kubeutil *kube.Kube) OAuthProxyResourceManager {
	return &oauthProxyResourceManager{rd: rd, kubeutil: kubeutil}
}

type oauthProxyResourceManager struct {
	rd       *v1.RadixDeployment
	kubeutil *kube.Kube
}

func (o *oauthProxyResourceManager) Install(component v1.RadixCommonDeployComponent) error {
	// build secret+role+binding
	// build deployment
	// build service
	// build ingress
	return o.syncDeployment(component)
}

func (o *oauthProxyResourceManager) Uninstall(componentName string) error {
	return nil
}

func (o *oauthProxyResourceManager) syncDeployment(component v1.RadixCommonDeployComponent) error {
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
			Annotations: make(map[string]string),
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
			switch v := value.(type) {
			case *bool:
				envVars = append(envVars, corev1.EnvVar{Name: envVar, Value: fmt.Sprint(*v)})
			case string:
				envVars = append(envVars, corev1.EnvVar{Name: envVar, Value: fmt.Sprint(v)})
			}
		}
	}

	// Add fixed settings
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_PROVIDER", Value: "oidc"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_COOKIE_HTTPONLY", Value: "true"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_COOKIE_SECURE", Value: "true"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_PASS_BASIC_AUTH", Value: "false"})
	envVars = append(envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_SKIP_PROVIDER_BUTTON", Value: "true"})
	// OAUTH2_PROXY_COOKIE_SECRET and OAUTH2_PROXY_CLIENT_SECRET must be mapped to a secret

	/*
			func GenerateRandomKey(length int) []byte {
			k := make([]byte, length)
			if _, err := io.ReadFull(rand.Reader, k); err != nil {
				return nil
			}
			return k
		}

	*/

	addEnvVarIfSet("OAUTH2_PROXY_CLIENT_ID", oauth.ClientID)
	addEnvVarIfSet("OAUTH2_PROXY_SCOPE", oauth.Scope)
	addEnvVarIfSet("OAUTH2_PROXY_SET_XAUTHREQUEST", oauth.SetXAuthRequestHeaders)
	addEnvVarIfSet("OAUTH2_PROXY_SET_AUTHORIZATION_HEADER", oauth.SetAuthorizationHeader)
	addEnvVarIfSet("OAUTH2_PROXY_EMAIL_DOMAINS", oauth.EmailDomain)
	addEnvVarIfSet("OAUTH2_PROXY_PROXY_PREFIX", oauth.ProxyPrefix)
	addEnvVarIfSet("OAUTH2_PROXY_LOGIN_URL", oauth.LoginURL)
	addEnvVarIfSet("OAUTH2_PROXY_REDEEM_URL", oauth.RedeemURL)
	addEnvVarIfSet("OAUTH2_PROXY_SESSION_STORE_TYPE", oauth.SessionStoreType)

	if oidc := oauth.OIDC; oidc != nil {
		addEnvVarIfSet("OAUTH2_PROXY_OIDC_ISSUER_URL", oidc.IssuerURL)
		addEnvVarIfSet("OAUTH2_PROXY_OIDC_JWKS_URL", oidc.JWKSURL)
		addEnvVarIfSet("OAUTH2_PROXY_OIDC_EMAIL_CLAIM", oidc.EmailClaim)
		addEnvVarIfSet("OAUTH2_PROXY_OIDC_GROUPS_CLAIM", oidc.GroupsClaim)
		addEnvVarIfSet("OAUTH2_PROXY_SKIP_OIDC_DISCOVERY", oidc.SkipDiscovery)
		addEnvVarIfSet("OAUTH2_PROXY_INSECURE_OIDC_SKIP_NONCE", oidc.InsecureSkipVerifyNonce)
	}

	if cookie := oauth.Cookie; cookie != nil {
		addEnvVarIfSet("OAUTH2_PROXY_COOKIE_NAME", cookie.Name)
		addEnvVarIfSet("OAUTH2_PROXY_COOKIE_PATH", cookie.Path)
		addEnvVarIfSet("OAUTH2_PROXY_COOKIE_DOMAIN", cookie.Domain)
		addEnvVarIfSet("OAUTH2_PROXY_COOKIE_EXPIRE", cookie.Expire)
		addEnvVarIfSet("OAUTH2_PROXY_COOKIE_REFRESH", cookie.Refresh)
		addEnvVarIfSet("OAUTH2_PROXY_COOKIE_SAMESITE", cookie.SameSite)
	}

	return envVars
}

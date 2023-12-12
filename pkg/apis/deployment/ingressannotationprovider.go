package deployment

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	oauthutil "github.com/equinor/radix-operator/pkg/apis/utils/oauth"
)

const (
	minCertDuration    = 2160 * time.Hour
	minCertRenewBefore = 360 * time.Hour
)

type IngressAnnotationProvider interface {
	GetAnnotations(component v1.RadixCommonDeployComponent, namespace string) (map[string]string, error)
}

func NewForceSslRedirectAnnotationProvider() IngressAnnotationProvider {
	return &forceSslRedirectAnnotationProvider{}
}

type forceSslRedirectAnnotationProvider struct{}

func (forceSslRedirectAnnotationProvider) GetAnnotations(component v1.RadixCommonDeployComponent, namespace string) (map[string]string, error) {
	return map[string]string{"nginx.ingress.kubernetes.io/force-ssl-redirect": "true"}, nil
}

func NewIngressConfigurationAnnotationProvider(config IngressConfiguration) IngressAnnotationProvider {
	return &ingressConfigurationAnnotationProvider{config: config}
}

type ingressConfigurationAnnotationProvider struct {
	config IngressConfiguration
}

func (provider *ingressConfigurationAnnotationProvider) GetAnnotations(component v1.RadixCommonDeployComponent, namespace string) (map[string]string, error) {
	allAnnotations := make(map[string]string)

	for _, configuration := range component.GetIngressConfiguration() {
		annotations := provider.getAnnotationsFromConfiguration(configuration, provider.config)
		for key, value := range annotations {
			allAnnotations[key] = value
		}
	}

	return allAnnotations, nil
}

func (provider *ingressConfigurationAnnotationProvider) getAnnotationsFromConfiguration(name string, config IngressConfiguration) map[string]string {
	for _, ingressConfig := range config.AnnotationConfigurations {
		if strings.EqualFold(ingressConfig.Name, name) {
			return ingressConfig.Annotations
		}
	}

	return nil
}

func NewClientCertificateAnnotationProvider(certificateNamespace string) IngressAnnotationProvider {
	return &clientCertificateAnnotationProvider{namespace: certificateNamespace}
}

type clientCertificateAnnotationProvider struct {
	namespace string
}

func (provider *clientCertificateAnnotationProvider) GetAnnotations(component v1.RadixCommonDeployComponent, namespace string) (map[string]string, error) {
	result := make(map[string]string)
	if auth := component.GetAuthentication(); auth != nil {
		if clientCert := auth.ClientCertificate; clientCert != nil {
			if IsSecretRequiredForClientCertificate(clientCert) {
				result["nginx.ingress.kubernetes.io/auth-tls-secret"] = fmt.Sprintf("%s/%s", provider.namespace, utils.GetComponentClientCertificateSecretName(component.GetName()))
			}

			certificateConfig := parseClientCertificateConfiguration(*clientCert)
			result["nginx.ingress.kubernetes.io/auth-tls-verify-client"] = string(*certificateConfig.Verification)
			result["nginx.ingress.kubernetes.io/auth-tls-pass-certificate-to-upstream"] = utils.TernaryString(*certificateConfig.PassCertificateToUpstream, "true", "false")
		}
	}

	return result, nil
}

func NewOAuth2AnnotationProvider(oauth2DefaultConfig defaults.OAuth2Config) IngressAnnotationProvider {
	return &oauth2AnnotationProvider{oauth2DefaultConfig: oauth2DefaultConfig}
}

type oauth2AnnotationProvider struct {
	oauth2DefaultConfig defaults.OAuth2Config
}

func (provider *oauth2AnnotationProvider) GetAnnotations(component v1.RadixCommonDeployComponent, namespace string) (map[string]string, error) {
	annotations := make(map[string]string)

	if auth := component.GetAuthentication(); component.IsPublic() && auth != nil && auth.OAuth2 != nil {
		oauth, err := provider.oauth2DefaultConfig.MergeWith(auth.OAuth2)
		if err != nil {
			return nil, err
		}

		svcName := utils.GetAuxiliaryComponentServiceName(component.GetName(), defaults.OAuthProxyAuxiliaryComponentSuffix)

		// Documentation for Oauth2 proxy auth-request: https://oauth2-proxy.github.io/oauth2-proxy/docs/configuration/overview#configuring-for-use-with-the-nginx-auth_request-directive
		hostPath := fmt.Sprintf("https://$host%s", oauthutil.SanitizePathPrefix(oauth.ProxyPrefix))
		servicePath := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d%s", svcName, namespace, oauthProxyPortNumber, oauthutil.SanitizePathPrefix(oauth.ProxyPrefix))
		annotations[authUrlAnnotation] = fmt.Sprintf("%s/auth", servicePath)
		annotations[authSigninAnnotation] = fmt.Sprintf("%s/start?rd=$escaped_request_uri", hostPath)

		var authResponseHeaders []string
		if oauth.SetXAuthRequestHeaders != nil && *oauth.SetXAuthRequestHeaders {
			authResponseHeaders = append(authResponseHeaders, "X-Auth-Request-Access-Token", "X-Auth-Request-User", "X-Auth-Request-Groups", "X-Auth-Request-Email", "X-Auth-Request-Preferred-Username")
		}
		if oauth.SetAuthorizationHeader != nil && *oauth.SetAuthorizationHeader {
			authResponseHeaders = append(authResponseHeaders, "Authorization")
		}
		if len(authResponseHeaders) > 0 {
			annotations[authResponseHeadersAnnotation] = strings.Join(authResponseHeaders, ",")
		}
	}

	return annotations, nil
}

func NewExternalDNSAnnotationProvider(useAutomation bool, certAutomationConfig CertificateAutomationConfig) IngressAnnotationProvider {
	return &externalDNSAnnotationProvider{
		useAutomation:        useAutomation,
		certAutomationConfig: certAutomationConfig,
	}
}

type externalDNSAnnotationProvider struct {
	useAutomation        bool
	certAutomationConfig CertificateAutomationConfig
}

func (provider *externalDNSAnnotationProvider) GetAnnotations(_ v1.RadixCommonDeployComponent, _ string) (map[string]string, error) {
	annotations := map[string]string{kube.RadixExternalDNSUseAutomationAnnotation: strconv.FormatBool(provider.useAutomation)}

	if provider.useAutomation {
		if len(provider.certAutomationConfig.ClusterIssuer) == 0 {
			return nil, errors.New("cluster issuer not set in certificate automation config")
		}
		duration := provider.certAutomationConfig.Duration
		if duration < minCertDuration {
			duration = minCertDuration
		}
		renewBefore := provider.certAutomationConfig.RenewBefore
		if renewBefore < minCertRenewBefore {
			renewBefore = minCertRenewBefore
		}
		annotations["cert-manager.io/cluster-issuer"] = provider.certAutomationConfig.ClusterIssuer
		annotations["cert-manager.io/duration"] = duration.String()
		annotations["cert-manager.io/renew-before"] = renewBefore.String()
	}

	return annotations, nil
}

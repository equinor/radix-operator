package ingress

import (
	"fmt"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	oauthutil "github.com/equinor/radix-operator/pkg/apis/utils/oauth"
)

type AnnotationProvider interface {
	GetAnnotations(component radixv1.RadixCommonDeployComponent, namespace string) (map[string]string, error)
}

func NewForceSslRedirectAnnotationProvider() AnnotationProvider {
	return &forceSslRedirectAnnotationProvider{}
}

type forceSslRedirectAnnotationProvider struct{}

func (forceSslRedirectAnnotationProvider) GetAnnotations(_ radixv1.RadixCommonDeployComponent, _ string) (map[string]string, error) {
	return map[string]string{"nginx.ingress.kubernetes.io/force-ssl-redirect": "true"}, nil
}

func NewIngressConfigurationAnnotationProvider(config IngressConfiguration) AnnotationProvider {
	return &ingressConfigurationAnnotationProvider{config: config}
}

type ingressConfigurationAnnotationProvider struct {
	config IngressConfiguration
}

func (provider *ingressConfigurationAnnotationProvider) GetAnnotations(component radixv1.RadixCommonDeployComponent, _ string) (map[string]string, error) {
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

func NewClientCertificateAnnotationProvider(certificateNamespace string) AnnotationProvider {
	return &clientCertificateAnnotationProvider{namespace: certificateNamespace}
}

type ClientCertificateAnnotationProvider interface {
	AnnotationProvider
	GetNamespace() string
}

type clientCertificateAnnotationProvider struct {
	namespace string
}

func (provider *clientCertificateAnnotationProvider) GetNamespace() string {
	return provider.namespace
}

func (provider *clientCertificateAnnotationProvider) GetAnnotations(component radixv1.RadixCommonDeployComponent, _ string) (map[string]string, error) {
	annotations := make(map[string]string)
	if auth := component.GetAuthentication(); auth != nil {
		if clientCert := auth.ClientCertificate; clientCert != nil {
			if IsSecretRequiredForClientCertificate(clientCert) {
				annotations["nginx.ingress.kubernetes.io/auth-tls-secret"] = fmt.Sprintf("%s/%s", provider.namespace, utils.GetComponentClientCertificateSecretName(component.GetName()))
			}

			certificateConfig := ParseClientCertificateConfiguration(*clientCert)
			annotations["nginx.ingress.kubernetes.io/auth-tls-verify-client"] = string(*certificateConfig.Verification)
			annotations["nginx.ingress.kubernetes.io/auth-tls-pass-certificate-to-upstream"] = utils.TernaryString(*certificateConfig.PassCertificateToUpstream, "true", "false")
		}
	}

	return annotations, nil
}

func NewOAuth2AnnotationProvider(oauth2DefaultConfig defaults.OAuth2Config) AnnotationProvider {
	return &oauth2AnnotationProvider{oauth2DefaultConfig: oauth2DefaultConfig}
}

type oauth2AnnotationProvider struct {
	oauth2DefaultConfig defaults.OAuth2Config
}

func (provider *oauth2AnnotationProvider) GetAnnotations(component radixv1.RadixCommonDeployComponent, namespace string) (map[string]string, error) {
	annotations := make(map[string]string)

	if auth := component.GetAuthentication(); component.IsPublic() && auth != nil && auth.OAuth2 != nil {
		oauth, err := provider.oauth2DefaultConfig.MergeWith(auth.OAuth2)
		if err != nil {
			return nil, err
		}

		svcName := utils.GetAuxiliaryComponentServiceName(component.GetName(), defaults.OAuthProxyAuxiliaryComponentSuffix)

		// Documentation for OAuth2 proxy auth-request: https://oauth2-proxy.github.io/oauth2-proxy/docs/configuration/overview#configuring-for-use-with-the-nginx-auth_request-directive
		hostPath := fmt.Sprintf("https://$host%s", oauthutil.SanitizePathPrefix(oauth.ProxyPrefix))
		servicePath := fmt.Sprintf("%s://%s.%s.svc.cluster.local:%d%s", "http", svcName, namespace, defaults.OAuthProxyPortNumber, oauthutil.SanitizePathPrefix(oauth.ProxyPrefix))
		annotations[defaults.AuthUrlAnnotation] = fmt.Sprintf("%s/auth", servicePath)
		annotations[defaults.AuthSigninAnnotation] = fmt.Sprintf("%s/start?rd=$escaped_request_uri", hostPath)

		var authResponseHeaders []string
		if oauth.SetXAuthRequestHeaders != nil && *oauth.SetXAuthRequestHeaders {
			authResponseHeaders = append(authResponseHeaders, "X-Auth-Request-Access-Token", "X-Auth-Request-User", "X-Auth-Request-Groups", "X-Auth-Request-Email", "X-Auth-Request-Preferred-Username")
		}
		if oauth.SetAuthorizationHeader != nil && *oauth.SetAuthorizationHeader {
			authResponseHeaders = append(authResponseHeaders, "Authorization")
		}
		if len(authResponseHeaders) > 0 {
			annotations[defaults.AuthResponseHeadersAnnotation] = strings.Join(authResponseHeaders, ",")
		}
	}

	return annotations, nil
}

// NewIngressPublicAllowListAnnotationProvider provides Ingress annotations for allowing
// only public traffic from IP adresses defined in Network.Ingress.Public.Allow field
func NewIngressPublicAllowListAnnotationProvider() AnnotationProvider {
	return &ingressPublicAllowListAnnotationProvider{}
}

type ingressPublicAllowListAnnotationProvider struct{}

func (*ingressPublicAllowListAnnotationProvider) GetAnnotations(component radixv1.RadixCommonDeployComponent, _ string) (map[string]string, error) {
	if network := component.GetNetwork(); network == nil || network.Ingress == nil || network.Ingress.Public == nil || network.Ingress.Public.Allow == nil || len(*network.Ingress.Public.Allow) == 0 {
		return nil, nil
	}

	annotations := make(map[string]string, 1)
	var sb strings.Builder

	for i, addr := range *component.GetNetwork().Ingress.Public.Allow {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(string(addr))
	}
	annotations["nginx.ingress.kubernetes.io/whitelist-source-range"] = sb.String()

	return annotations, nil
}

package deployment

import (
	"fmt"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	oauthutil "github.com/equinor/radix-operator/pkg/apis/utils/oauth"
)

type IngressAnnotationProvider interface {
	GetAnnotations(component v1.RadixCommonDeployComponent) (map[string]string, error)
}

func NewForceSslRedirectAnnotationProvider() IngressAnnotationProvider {
	return &forceSslRedirectAnnotationProvider{}
}

type forceSslRedirectAnnotationProvider struct{}

func (forceSslRedirectAnnotationProvider) GetAnnotations(component v1.RadixCommonDeployComponent) (map[string]string, error) {
	return map[string]string{"nginx.ingress.kubernetes.io/force-ssl-redirect": "true"}, nil
}

func NewIngressConfigurationAnnotationProvider(config IngressConfiguration) IngressAnnotationProvider {
	return &ingressConfigurationAnnotationProvider{config: config}
}

type ingressConfigurationAnnotationProvider struct {
	config IngressConfiguration
}

func (o *ingressConfigurationAnnotationProvider) GetAnnotations(component v1.RadixCommonDeployComponent) (map[string]string, error) {
	allAnnotations := make(map[string]string)

	for _, configuration := range component.GetIngressConfiguration() {
		annotations := o.getAnnotationsFromConfiguration(configuration, o.config)
		for key, value := range annotations {
			allAnnotations[key] = value
		}
	}

	return allAnnotations, nil
}

func (o *ingressConfigurationAnnotationProvider) getAnnotationsFromConfiguration(name string, config IngressConfiguration) map[string]string {
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

func (o *clientCertificateAnnotationProvider) GetAnnotations(component v1.RadixCommonDeployComponent) (map[string]string, error) {
	result := make(map[string]string)
	if auth := component.GetAuthentication(); auth != nil {
		if clientCert := auth.ClientCertificate; clientCert != nil {
			if IsSecretRequiredForClientCertificate(clientCert) {
				result["nginx.ingress.kubernetes.io/auth-tls-secret"] = fmt.Sprintf("%s/%s", o.namespace, utils.GetComponentClientCertificateSecretName(component.GetName()))
			}

			certificateConfig := parseClientCertificateConfiguration(*clientCert)
			result["nginx.ingress.kubernetes.io/auth-tls-verify-client"] = string(*certificateConfig.Verification)
			result["nginx.ingress.kubernetes.io/auth-tls-pass-certificate-to-upstream"] = utils.TernaryString(*certificateConfig.PassCertificateToUpstream, "true", "false")
		}
	}

	return result, nil
}

func NewOAuth2AnnotationProvider(oauth2DefaultConfig defaults.OAuth2DefaultConfigApplier) IngressAnnotationProvider {
	return &oauth2AnnotationProvider{oauth2DefaultConfig: oauth2DefaultConfig}
}

type oauth2AnnotationProvider struct {
	oauth2DefaultConfig defaults.OAuth2DefaultConfigApplier
}

func (a *oauth2AnnotationProvider) GetAnnotations(component v1.RadixCommonDeployComponent) (map[string]string, error) {
	annotations := make(map[string]string)

	if auth := component.GetAuthentication(); component.IsPublic() && auth != nil && auth.OAuth2 != nil {
		oauth, err := a.oauth2DefaultConfig.ApplyTo(auth.OAuth2)
		if err != nil {
			return nil, err
		}

		rootPath := fmt.Sprintf("https://$host%s", oauthutil.SanitizePathPrefix(oauth.ProxyPrefix))
		annotations[authUrlAnnotation] = fmt.Sprintf("%s/auth", rootPath)
		annotations[authSigninAnnotation] = fmt.Sprintf("%s/start?rd=$escaped_request_uri", rootPath)

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

package deployment

import (
	"fmt"
	"strings"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	oauthutil "github.com/equinor/radix-operator/pkg/apis/utils/oauth"
)

type IngressAnnotations interface {
	GetAnnotations(component v1.RadixCommonDeployComponent) map[string]string
}

type forceSslRedirectAnnotations struct{}

func (forceSslRedirectAnnotations) GetAnnotations(component v1.RadixCommonDeployComponent) map[string]string {
	return map[string]string{"nginx.ingress.kubernetes.io/force-ssl-redirect": "true"}
}

type ingressConfigurationAnnotations struct {
	config IngressConfiguration
}

func (o *ingressConfigurationAnnotations) GetAnnotations(component v1.RadixCommonDeployComponent) map[string]string {
	allAnnotations := make(map[string]string)

	for _, configuration := range component.GetIngressConfiguration() {
		annotations := o.getAnnotationsFromConfiguration(configuration, o.config)
		for key, value := range annotations {
			allAnnotations[key] = value
		}
	}

	return allAnnotations
}

func (o *ingressConfigurationAnnotations) getAnnotationsFromConfiguration(name string, config IngressConfiguration) map[string]string {
	for _, ingressConfig := range config.AnnotationConfigurations {
		if strings.EqualFold(ingressConfig.Name, name) {
			return ingressConfig.Annotations
		}
	}

	return nil
}

type clientCertificateAnnotations struct {
	namespace string
}

func (o *clientCertificateAnnotations) GetAnnotations(component v1.RadixCommonDeployComponent) map[string]string {
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

	return result
}

type oauth2Annotations struct {
	oauth2Config OAuth2Config
}

func (a *oauth2Annotations) GetAnnotations(component v1.RadixCommonDeployComponent) map[string]string {
	annotations := make(map[string]string)

	if auth := component.GetAuthentication(); component.IsPublic() && auth != nil && auth.OAuth2 != nil {
		oauth := a.oauth2Config.MergeWithDefaults(auth.OAuth2)
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

	return annotations
}

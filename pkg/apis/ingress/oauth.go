package ingress

import (
	"github.com/equinor/radix-common/utils/maps"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/oauth"
	"github.com/sirupsen/logrus"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetAuxOAuthProxyAnnotationProviders Gets aux OAuth proxy annotation providers
func GetAuxOAuthProxyAnnotationProviders() []AnnotationProvider {
	return []AnnotationProvider{NewForceSslRedirectAnnotationProvider()}
}

// CreateOrUpdateOAuthProxyIngressForComponentIngress Creates or updates OAuth proxy ingress for RadixDeploy component ingress
func CreateOrUpdateOAuthProxyIngressForComponentIngress(kubeutil *kube.Kube, namespace string, appName string, component v1.RadixCommonDeployComponent, ingress *networkingv1.Ingress, ingressAnnotationProviders []AnnotationProvider) error {
	auxIngress, err := buildOAuthProxyIngressForComponentIngress(appName, ingressAnnotationProviders, component, ingress, namespace)
	if err != nil {
		return err
	}
	if auxIngress != nil {
		return kubeutil.ApplyIngress(namespace, auxIngress)
	}
	return nil
}

func buildOAuthProxyIngressForComponentIngress(appName string, ingressAnnotationProviders []AnnotationProvider, component v1.RadixCommonDeployComponent, componentIngress *networkingv1.Ingress, namespace string) (*networkingv1.Ingress, error) {
	if len(componentIngress.Spec.Rules) == 0 {
		logrus.Debugf("the component ingress %s in the namespace %s has no rules. Do not create an OAuth proxy ingress", componentIngress.GetName(), namespace)
		return nil, nil
	}
	oauthProxyIngressName := oauth.GetAuxAuthProxyIngressName(componentIngress.GetName())
	sourceHost := componentIngress.Spec.Rules[0]
	oAuthProxyPortNumber := defaults.OAuthProxyPortNumber
	logrus.Debugf("build the OAuth proxy ingress %s with the host %s, port %d for the component ingress %s in the namespace %s", oauthProxyIngressName, sourceHost, oAuthProxyPortNumber, componentIngress.GetName(), namespace)
	pathType := networkingv1.PathTypeImplementationSpecific
	annotations := map[string]string{}

	for _, ia := range ingressAnnotationProviders {
		providedAnnotations, err := ia.GetAnnotations(component, namespace)
		if err != nil {
			return nil, err
		}
		annotations = maps.MergeMaps(annotations, providedAnnotations)
	}

	var tls []networkingv1.IngressTLS
	for _, sourceTls := range componentIngress.Spec.TLS {
		tls = append(tls, *sourceTls.DeepCopy())
	}

	rulePath := oauth.SanitizePathPrefix(component.GetAuthentication().OAuth2.ProxyPrefix)
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:            oauthProxyIngressName,
			Annotations:     annotations,
			OwnerReferences: GetOwnerReferenceOfIngress(componentIngress),
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
									Path:     rulePath,
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: utils.GetAuxiliaryComponentServiceName(component.GetName(), defaults.OAuthProxyAuxiliaryComponentSuffix),
											Port: networkingv1.ServiceBackendPort{
												Number: oAuthProxyPortNumber,
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

	oauth.MergeAuxComponentResourceLabels(ingress, appName, component)
	return ingress, nil
}

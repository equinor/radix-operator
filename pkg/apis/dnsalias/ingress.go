package dnsalias

import (
	"context"
	"fmt"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/dnsalias/internal"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/pkg/apis/utils/oauth"
	"github.com/rs/zerolog/log"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// CreateRadixDNSAliasIngress Create an Ingress for a RadixDNSAlias
func CreateRadixDNSAliasIngress(ctx context.Context, kubeClient kubernetes.Interface, appName, envName string, ingress *networkingv1.Ingress) (*networkingv1.Ingress, error) {
	return kubeClient.NetworkingV1().Ingresses(utils.GetEnvironmentNamespace(appName, envName)).Create(ctx, ingress, metav1.CreateOptions{})
}

// GetDNSAliasIngressName Gets name of the ingress for the custom DNS alias
func GetDNSAliasIngressName(alias string) string {
	return fmt.Sprintf("%s.custom-alias", alias)
}

// GetDNSAliasHost Gets DNS alias host.
// Example for the alias "my-app" and the cluster "Playground": my-app.playground.radix.equinor.com
func GetDNSAliasHost(alias, dnsZone string) string {
	return fmt.Sprintf("%s.%s", alias, dnsZone)
}

func (s *syncer) syncIngress(ctx context.Context, namespace string, radixDeployComponent radixv1.RadixCommonDeployComponent) (*networkingv1.Ingress, error) {
	ingressName := GetDNSAliasIngressName(s.radixDNSAlias.GetName())
	newIngress, err := buildIngress(radixDeployComponent, s.radixDNSAlias, s.dnsZone, s.oauth2DefaultConfig, s.ingressConfiguration)
	if err != nil {
		return nil, err
	}
	existingIngress, err := s.kubeUtil.GetIngress(ctx, namespace, ingressName)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Ctx(ctx).Debug().Msgf("not found Ingress %s in the namespace %s. Create new.", ingressName, namespace)
			return s.createIngress(ctx, s.radixDNSAlias, newIngress)
		}
		return nil, err
	}
	log.Ctx(ctx).Debug().Msgf("found Ingress %s in the namespace %s.", ingressName, namespace)
	patchesIngress, err := s.applyIngress(ctx, namespace, existingIngress, newIngress)
	if err != nil {
		return nil, err
	}
	return patchesIngress, nil
}

func (s *syncer) applyIngress(ctx context.Context, namespace string, existingIngress *networkingv1.Ingress, ing *networkingv1.Ingress) (*networkingv1.Ingress, error) {
	patchesIngress, err := s.kubeUtil.PatchIngress(ctx, namespace, existingIngress, ing)
	if err != nil {
		return nil, fmt.Errorf("failed to patch an ingress %s: %w", ing.GetName(), err)
	}
	return patchesIngress, nil
}

func (s *syncer) createIngress(ctx context.Context, radixDNSAlias *radixv1.RadixDNSAlias, ing *networkingv1.Ingress) (*networkingv1.Ingress, error) {
	log.Ctx(ctx).Debug().Msgf("create an ingress %s for the RadixDNSAlias", ing.GetName())
	return CreateRadixDNSAliasIngress(ctx, s.kubeClient, radixDNSAlias.Spec.AppName, radixDNSAlias.Spec.Environment, ing)
}

func (s *syncer) syncOAuthProxyIngress(ctx context.Context, namespace string, ing *networkingv1.Ingress, deployComponent radixv1.RadixCommonDeployComponent) error {
	appName := s.radixDNSAlias.Spec.AppName
	authentication := deployComponent.GetAuthentication()
	oauthEnabled := authentication != nil && authentication.OAuth2 != nil
	if !oauthEnabled {
		selector := radixlabels.ForAuxComponentDNSAliasIngress(s.radixDNSAlias.Spec.AppName, deployComponent, s.radixDNSAlias.GetName())
		return s.deleteIngresses(ctx, selector)
	}
	oauth2, err := s.oauth2DefaultConfig.MergeWith(authentication.OAuth2)
	if err != nil {
		return err
	}
	authentication.OAuth2 = oauth2
	auxIngress, err := ingress.BuildOAuthProxyIngressForComponentIngress(namespace, deployComponent, ing, s.ingressAnnotationProviders)
	if err != nil {
		return err
	}
	if auxIngress == nil {
		return nil
	}
	oauth.MergeAuxComponentDNSAliasIngressResourceLabels(auxIngress, appName, deployComponent, s.radixDNSAlias.GetName())
	return s.kubeUtil.ApplyIngress(ctx, namespace, auxIngress)
}

func (s *syncer) deleteIngresses(ctx context.Context, selector labels.Set) error {
	aliasSpec := s.radixDNSAlias.Spec
	namespace := utils.GetEnvironmentNamespace(aliasSpec.AppName, aliasSpec.Environment)
	ingresses, err := s.kubeUtil.KubeClient().NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return err
	}
	return s.kubeUtil.DeleteIngresses(ctx, ingresses.Items...)
}

func buildIngress(radixDeployComponent radixv1.RadixCommonDeployComponent, radixDNSAlias *radixv1.RadixDNSAlias, dnsZone string, oauth2Config defaults.OAuth2Config, ingressConfiguration ingress.IngressConfiguration) (*networkingv1.Ingress, error) {
	publicPort := getComponentPublicPort(radixDeployComponent)
	if publicPort == nil {
		return nil, radixvalidators.ComponentForDNSAliasIsNotMarkedAsPublicError(radixDeployComponent.GetName())
	}
	aliasName := radixDNSAlias.GetName()
	aliasSpec := radixDNSAlias.Spec
	ingressName := GetDNSAliasIngressName(aliasName)
	hostName := GetDNSAliasHost(aliasName, dnsZone)
	ingressSpec := ingress.GetIngressSpec(hostName, aliasSpec.Component, defaults.TLSSecretName, publicPort.Port)

	namespace := utils.GetEnvironmentNamespace(aliasSpec.AppName, aliasSpec.Environment)
	ingressAnnotations := ingress.GetAnnotationProvider(ingressConfiguration, namespace, oauth2Config)
	ingressConfig, err := ingress.GetIngressConfig(namespace, aliasSpec.AppName, radixDeployComponent, ingressName, ingressSpec, ingressAnnotations, internal.GetOwnerReferences(radixDNSAlias, true))
	if err != nil {
		return nil, err
	}

	ingressConfig.ObjectMeta.Labels[kube.RadixAliasLabel] = aliasName
	return ingressConfig, nil
}

func getComponentPublicPort(component radixv1.RadixCommonDeployComponent) *radixv1.ComponentPort {
	if port, ok := slice.FindFirst(component.GetPorts(), func(p radixv1.ComponentPort) bool { return p.Name == component.GetPublicPort() }); ok {
		return &port
	}
	return nil
}

package dnsalias

import (
	"context"
	"fmt"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/dnsalias/internal"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/pkg/apis/utils/oauth"
	log "github.com/sirupsen/logrus"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CreateRadixDNSAliasIngress Create an Ingress for a RadixDNSAlias
func CreateRadixDNSAliasIngress(kubeClient kubernetes.Interface, appName, envName string, ingress *networkingv1.Ingress) (*networkingv1.Ingress, error) {
	return kubeClient.NetworkingV1().Ingresses(utils.GetEnvironmentNamespace(appName, envName)).Create(context.Background(), ingress, metav1.CreateOptions{})
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

func (s *syncer) syncIngress(namespace string, radixDeployComponent radixv1.RadixCommonDeployComponent) (*networkingv1.Ingress, error) {
	ingressName := GetDNSAliasIngressName(s.radixDNSAlias.GetName())
	newIngress, err := buildIngress(radixDeployComponent, s.radixDNSAlias, s.dnsConfig, s.oauth2DefaultConfig, s.ingressConfiguration)
	if err != nil {
		return nil, err
	}
	existingIngress, err := s.kubeUtil.GetIngress(namespace, ingressName)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Debugf("not found Ingress %s in the namespace %s. Create new.", ingressName, namespace)
			return s.createIngress(s.radixDNSAlias, newIngress)
		}
		return nil, err
	}
	log.Debugf("found Ingress %s in the namespace %s.", ingressName, namespace)
	patchesIngress, err := s.applyIngress(namespace, existingIngress, newIngress)
	if err != nil {
		return nil, err
	}
	return patchesIngress, nil
}

func (s *syncer) applyIngress(namespace string, existingIngress *networkingv1.Ingress, ing *networkingv1.Ingress) (*networkingv1.Ingress, error) {
	patchesIngress, err := s.kubeUtil.PatchIngress(namespace, existingIngress, ing)
	if err != nil {
		return nil, fmt.Errorf("failed to patch an ingress %s: %w", ing.GetName(), err)
	}
	return patchesIngress, nil
}

func (s *syncer) createIngress(radixDNSAlias *radixv1.RadixDNSAlias, ing *networkingv1.Ingress) (*networkingv1.Ingress, error) {
	log.Debugf("create an ingress %s for the RadixDNSAlias", ing.GetName())
	return CreateRadixDNSAliasIngress(s.kubeClient, radixDNSAlias.Spec.AppName, radixDNSAlias.Spec.Environment, ing)
}

func (s *syncer) syncOAuthProxyIngress(namespace string, ing *networkingv1.Ingress, deployComponent radixv1.RadixCommonDeployComponent) error {
	appName := s.radixDNSAlias.Spec.AppName
	authentication := deployComponent.GetAuthentication()
	oauthEnabled := authentication != nil && authentication.OAuth2 != nil
	if !oauthEnabled {
		return s.deleteOAuthAuxIngresses(deployComponent, namespace, appName)
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
	return s.kubeUtil.ApplyIngress(namespace, auxIngress)
}

func (s *syncer) deleteOAuthAuxIngresses(deployComponent radixv1.RadixCommonDeployComponent, namespace string, appName string) error {
	oauthAuxIngresses, err := s.kubeUtil.KubeClient().NetworkingV1().Ingresses(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: radixlabels.ForAuxComponentDNSAliasIngress(appName, deployComponent, s.radixDNSAlias.GetName()).String()})
	if err != nil {
		return err
	}
	return s.kubeUtil.DeleteIngresses(slice.PointersOf(oauthAuxIngresses.Items).([]*networkingv1.Ingress)...)
}

func buildIngress(radixDeployComponent radixv1.RadixCommonDeployComponent, radixDNSAlias *radixv1.RadixDNSAlias, dnsConfig *dnsalias.DNSConfig, oauth2Config defaults.OAuth2Config, ingressConfiguration ingress.IngressConfiguration) (*networkingv1.Ingress, error) {
	log.Debug("build an ingress for the RadixDNSAlias")
	publicPort := getComponentPublicPort(radixDeployComponent)
	if publicPort == nil {
		return nil, radixvalidators.ComponentForDNSAliasIsNotMarkedAsPublicError(radixDeployComponent.GetName())
	}
	aliasName := radixDNSAlias.GetName()
	aliasSpec := radixDNSAlias.Spec
	ingressName := GetDNSAliasIngressName(aliasName)
	hostName := GetDNSAliasHost(aliasName, dnsConfig.DNSZone)
	ingressSpec := ingress.GetIngressSpec(hostName, aliasSpec.Component, defaults.TLSSecretName, publicPort.Port)

	namespace := utils.GetEnvironmentNamespace(aliasSpec.AppName, aliasSpec.Environment)
	ingressAnnotations := ingress.GetAnnotationProvider(ingressConfiguration, namespace, oauth2Config)
	ingressConfig, err := ingress.GetIngressConfig(namespace, aliasSpec.AppName, radixDeployComponent, ingressName, ingressSpec, ingressAnnotations, internal.GetOwnerReferences(radixDNSAlias))
	if err != nil {
		return nil, err
	}

	ingressConfig.ObjectMeta.Annotations = annotations.Merge(ingressConfig.ObjectMeta.Annotations, annotations.ForManagedByRadixDNSAliasIngress(aliasName))
	ingressConfig.ObjectMeta.Labels[kube.RadixAliasLabel] = aliasName
	log.Debugf("built the Ingress %s in the environment %s with a host %s", ingressConfig.GetName(), namespace, hostName)
	return ingressConfig, nil
}

func (s *syncer) deletedIngresses() error {
	aliasSpec := s.radixDNSAlias.Spec
	namespace := utils.GetEnvironmentNamespace(aliasSpec.AppName, aliasSpec.Environment)
	dnsAliasIngressesSelector := radixlabels.ForDNSAliasIngress(aliasSpec.AppName, aliasSpec.Component, s.radixDNSAlias.GetName()).String()
	ingresses, err := s.kubeUtil.KubeClient().NetworkingV1().Ingresses(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: dnsAliasIngressesSelector})
	if err != nil {
		return err
	}
	return s.kubeUtil.DeleteIngresses(slice.PointersOf(ingresses.Items).([]*networkingv1.Ingress)...)
}

func getComponentPublicPort(component radixv1.RadixCommonDeployComponent) *radixv1.ComponentPort {
	if port, ok := slice.FindFirst(component.GetPorts(), func(p radixv1.ComponentPort) bool { return p.Name == component.GetPublicPort() }); ok {
		return &port
	}
	return nil
}

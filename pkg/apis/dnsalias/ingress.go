package dnsalias

import (
	"context"
	"fmt"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
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

func (s *syncer) syncIngress(namespace string, radixDeployComponent radixv1.RadixCommonDeployComponent, ingressName string) (*networkingv1.Ingress, error) {
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

func (s *syncer) syncOAuthProxyIngress(deployComponent radixv1.RadixCommonDeployComponent, namespace string, aliasSpec radixv1.RadixDNSAliasSpec, radixDeployComponent radixv1.RadixCommonDeployComponent, ing *networkingv1.Ingress) error {
	appName := aliasSpec.AppName
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

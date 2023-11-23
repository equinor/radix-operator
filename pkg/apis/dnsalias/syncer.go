package dnsalias

import (
	"fmt"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/dnsalias/internal"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/radix"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
)

// Syncer of  RadixDNSAliases
type Syncer interface {
	// OnSync Syncs RadixDNSAliases
	OnSync() error
}

// DNSAlias is the aggregate-root for manipulating RadixDNSAliases
type syncer struct {
	kubeClient                 kubernetes.Interface
	radixClient                radixclient.Interface
	kubeUtil                   *kube.Kube
	radixDNSAlias              *radixv1.RadixDNSAlias
	dnsConfig                  *dnsalias.DNSConfig
	ingressConfiguration       ingress.IngressConfiguration
	oauth2DefaultConfig        defaults.OAuth2Config
	ingressAnnotationProviders []ingress.AnnotationProvider
}

// NewSyncer is the constructor for RadixDNSAlias syncer
func NewSyncer(kubeClient kubernetes.Interface, kubeUtil *kube.Kube, radixClient radixclient.Interface, dnsConfig *dnsalias.DNSConfig, ingressConfiguration ingress.IngressConfiguration, oauth2Config defaults.OAuth2Config, ingressAnnotationProviders []ingress.AnnotationProvider, radixDNSAlias *radixv1.RadixDNSAlias) Syncer {
	return &syncer{
		kubeClient:                 kubeClient,
		radixClient:                radixClient,
		kubeUtil:                   kubeUtil,
		dnsConfig:                  dnsConfig,
		ingressConfiguration:       ingressConfiguration,
		oauth2DefaultConfig:        oauth2Config,
		ingressAnnotationProviders: ingressAnnotationProviders,
		radixDNSAlias:              radixDNSAlias,
	}
}

// OnSync is called by the handler when changes are applied and must be
// reconciled with current state.
func (s *syncer) OnSync() error {
	log.Debugf("OnSync RadixDNSAlias %s, application %s, environment %s, component %s, port %d", s.radixDNSAlias.GetName(), s.radixDNSAlias.Spec.AppName, s.radixDNSAlias.Spec.Environment, s.radixDNSAlias.Spec.Component, s.radixDNSAlias.Spec.Port)
	if err := s.restoreStatus(); err != nil {
		return fmt.Errorf("failed to update status on DNS alias %s: %v", s.radixDNSAlias.GetName(), err)
	}
	if err := s.syncAlias(); err != nil {
		return err
	}
	return s.syncStatus()

}

func (s *syncer) syncAlias() error {
	aliasName := s.radixDNSAlias.GetName()
	log.Debugf("syncAlias RadixDNSAlias %s", aliasName)

	radixDeployComponent, err := s.getRadixDeployComponent()
	if err != nil {
		return err
	}

	aliasSpec := s.radixDNSAlias.Spec
	ingressName := GetDNSAliasIngressName(aliasSpec.Component, aliasName)
	namespace := utils.GetEnvironmentNamespace(aliasSpec.AppName, aliasSpec.Environment)
	ing, err := s.createOrUpdateIngress(namespace, radixDeployComponent, ingressName)
	if err != nil {
		return err
	}
	return ingress.CreateOrUpdateOAuthProxyIngressForComponentIngress(s.kubeUtil, namespace, aliasSpec.AppName, radixDeployComponent, ing, s.ingressAnnotationProviders)
}

func (s *syncer) getRadixDeployComponent() (radixv1.RadixCommonDeployComponent, error) {
	aliasSpec := s.radixDNSAlias.Spec
	namespace := utils.GetEnvironmentNamespace(aliasSpec.AppName, aliasSpec.Environment)
	log.Debugf("get active deployment for the namespace %s", namespace)
	radixDeployment, err := s.kubeUtil.GetActiveDeployment(namespace)
	if err != nil {
		return nil, err
	}
	if radixDeployment == nil {
		return nil, errors.NewNotFound(schema.GroupResource{Group: radix.GroupName, Resource: radix.ResourceRadixDeployment}, "active")
	}
	log.Debugf("active deployment for the namespace %s is %s", namespace, radixDeployment.GetName())
	deployComponent := radixDeployment.GetCommonComponentByName(aliasSpec.Component)
	if commonUtils.IsNil(deployComponent) {
		return nil, DeployComponentNotFoundByName(aliasSpec.AppName, aliasSpec.Environment, aliasSpec.Component, radixDeployment.GetName())
	}
	return deployComponent, nil
}

func (s *syncer) createOrUpdateIngress(namespace string, radixDeployComponent radixv1.RadixCommonDeployComponent, ingressName string) (*networkingv1.Ingress, error) {
	existingIngress, err := s.kubeUtil.GetIngress(namespace, ingressName)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Debugf("not found Ingress %s in the namespace %s. Create new.", ingressName, namespace)
			return s.createIngress(radixDeployComponent)
		}
		return nil, err
	}
	log.Debugf("found Ingress %s in the namespace %s.", ingressName, namespace)
	updatedIngress, err := s.buildIngress(radixDeployComponent)
	if err != nil {
		return nil, err
	}
	ing, err := s.kubeUtil.PatchIngress(namespace, existingIngress, updatedIngress)
	if err != nil {
		return nil, fmt.Errorf("failed to patch an ingress %s: %w", ingressName, err)
	}
	return ing, nil
}

func (s *syncer) createIngress(radixDeployComponent radixv1.RadixCommonDeployComponent) (*networkingv1.Ingress, error) {
	ing, err := s.buildIngress(radixDeployComponent)
	if err != nil {
		return nil, err
	}
	aliasSpec := s.radixDNSAlias.Spec
	log.Debugf("create an ingress %s for the RadixDNSAlias", ing.GetName())
	return CreateRadixDNSAliasIngress(s.kubeClient, aliasSpec.AppName, aliasSpec.Environment, ing)
}

func (s *syncer) buildIngress(radixDeployComponent radixv1.RadixCommonDeployComponent) (*networkingv1.Ingress, error) {
	log.Debug("build an ingress for the RadixDNSAlias")
	aliasSpec := s.radixDNSAlias.Spec
	appName := aliasSpec.AppName
	envName := aliasSpec.Environment
	componentName := aliasSpec.Component
	namespace := utils.GetEnvironmentNamespace(appName, envName)
	ingressAnnotations := []ingress.AnnotationProvider{
		ingress.NewForceSslRedirectAnnotationProvider(),
		ingress.NewIngressConfigurationAnnotationProvider(s.ingressConfiguration),
		ingress.NewClientCertificateAnnotationProvider(namespace),
		ingress.NewOAuth2AnnotationProvider(s.oauth2DefaultConfig),
	}

	aliasName := s.radixDNSAlias.GetName()
	ingressName := GetDNSAliasIngressName(componentName, aliasName)
	hostName := GetDNSAliasHost(aliasName, s.dnsConfig.DNSZone)
	ingressSpec := ingress.GetIngressSpec(hostName, componentName, defaults.TLSSecretName, aliasSpec.Port)
	ingressConfig, err := ingress.GetIngressConfig(namespace, appName, radixDeployComponent, ingressName, ingressSpec, ingressAnnotations, ingress.DNSAlias, internal.GetOwnerReferences(s.radixDNSAlias))
	if err != nil {
		return nil, err
	}
	ingressConfig.ObjectMeta.Annotations = annotations.Merge(ingressConfig.ObjectMeta.Annotations, annotations.ForManagedByRadixDNSAliasIngress(aliasName))
	log.Debugf("built the Ingress %s in the environment %s with a host %s", ingressConfig.GetName(), namespace, hostName)
	return ingressConfig, nil
}

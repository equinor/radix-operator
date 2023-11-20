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
	kubeClient           kubernetes.Interface
	radixClient          radixclient.Interface
	kubeUtil             *kube.Kube
	radixDNSAlias        *radixv1.RadixDNSAlias
	dnsConfig            *dnsalias.DNSConfig
	ingressConfiguration ingress.IngressConfiguration
	oauth2DefaultConfig  defaults.OAuth2Config
}

// NewSyncer is the constructor for RadixDNSAlias syncer
func NewSyncer(kubeClient kubernetes.Interface, kubeUtil *kube.Kube, radixClient radixclient.Interface, dnsConfig *dnsalias.DNSConfig, ingressConfiguration ingress.IngressConfiguration, oauth2Config defaults.OAuth2Config, radixDNSAlias *radixv1.RadixDNSAlias) Syncer {
	return &syncer{
		kubeClient:           kubeClient,
		radixClient:          radixClient,
		kubeUtil:             kubeUtil,
		dnsConfig:            dnsConfig,
		ingressConfiguration: ingressConfiguration,
		oauth2DefaultConfig:  oauth2Config,
		radixDNSAlias:        radixDNSAlias,
	}
}

// OnSync is called by the handler when changes are applied and must be
// reconciled with current state.
func (s *syncer) OnSync() error {
	if err := s.restoreStatus(); err != nil {
		return fmt.Errorf("failed to update status on DNS alias %s: %v", s.radixDNSAlias.GetName(), err)
	}
	if err := s.syncAlias(); err != nil {
		return err
	}
	return s.syncStatus()

}

func (s *syncer) syncAlias() error {
	aliasSpec := s.radixDNSAlias.Spec
	aliasName := s.radixDNSAlias.GetName()
	envNamespace := utils.GetEnvironmentNamespace(aliasSpec.AppName, aliasSpec.Environment)
	ingressName := GetDNSAliasIngressName(aliasSpec.Component, aliasName)
	existingIngress, err := s.kubeUtil.GetIngress(envNamespace, ingressName)
	if err != nil {
		if errors.IsNotFound(err) {
			return s.createIngress()
		}
		return err
	}
	updatedIngress, err := s.buildIngress()
	if err != nil {
		return err
	}
	err = s.kubeUtil.PatchIngress(envNamespace, existingIngress, updatedIngress)
	if err != nil {
		return fmt.Errorf("failed to patch an ingress %s: %w", ingressName, err)
	}
	return nil
}

func (s *syncer) createIngress() error {
	ingress, err := s.buildIngress()
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	aliasSpec := s.radixDNSAlias.Spec
	_, err = CreateRadixDNSAliasIngress(s.kubeClient, aliasSpec.AppName, aliasSpec.Environment, ingress)
	return err
}

func (s *syncer) buildIngress() (*networkingv1.Ingress, error) {
	aliasSpec := s.radixDNSAlias.Spec
	appName := aliasSpec.AppName
	envName := aliasSpec.Environment
	componentName := aliasSpec.Component
	envNamespace := utils.GetEnvironmentNamespace(appName, envName)
	ingressAnnotations := []ingress.AnnotationProvider{
		ingress.NewForceSslRedirectAnnotationProvider(),
		ingress.NewIngressConfigurationAnnotationProvider(s.ingressConfiguration),
		ingress.NewClientCertificateAnnotationProvider(envNamespace),
		ingress.NewOAuth2AnnotationProvider(s.oauth2DefaultConfig),
	}
	radixDeployment, err := s.kubeUtil.GetActiveDeployment(envNamespace)
	if err != nil {
		return nil, err
	}
	if radixDeployment == nil {
		return nil, errors.NewNotFound(schema.GroupResource{Group: radix.GroupName, Resource: radix.ResourceRadixDeployment}, "active")
	}
	deployComponent := radixDeployment.GetCommonComponentByName(componentName)
	if commonUtils.IsNil(deployComponent) {
		return nil, DeployComponentNotFoundByName(appName, envName, componentName, radixDeployment.GetName())
	}

	aliasName := s.radixDNSAlias.GetName()
	ingressName := GetDNSAliasIngressName(componentName, aliasName)
	hostName := GetDNSAliasHost(aliasName, s.dnsConfig.DNSZone)
	ingressSpec := ingress.GetIngressSpec(hostName, componentName, defaults.TLSSecretName, aliasSpec.Port)
	ingressConfig, err := ingress.GetIngressConfig(envNamespace, appName, deployComponent, ingressName, ingressSpec, ingressAnnotations, ingress.DNSAlias, internal.GetOwnerReferences(s.radixDNSAlias))
	if err != nil {
		return nil, err
	}
	ingressConfig.ObjectMeta.Annotations = annotations.Merge(ingressConfig.ObjectMeta.Annotations, annotations.ForManagedByRadixDNSAliasIngress(aliasName))
	return ingressConfig, nil
}

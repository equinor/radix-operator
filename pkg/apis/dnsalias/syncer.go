package dnsalias

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
)

// Syncer of  RadixDNSAliases
type Syncer interface {
	// OnSync Syncs RadixDNSAliases
	OnSync() error
}

// DNSAlias is the aggregate-root for manipulating RadixDNSAliases
type syncer struct {
	kubeClient    kubernetes.Interface
	radixClient   radixclient.Interface
	kubeUtil      *kube.Kube
	radixDNSAlias *radixv1.RadixDNSAlias
	dnsConfig     *config.DNSConfig
}

// NewSyncer is the constructor for RadixDNSAlias syncer
func NewSyncer(kubeClient kubernetes.Interface, kubeUtil *kube.Kube, radixClient radixclient.Interface, dnsConfig *config.DNSConfig, radixDNSAlias *radixv1.RadixDNSAlias) Syncer {
	return &syncer{
		kubeClient:    kubeClient,
		radixClient:   radixClient,
		kubeUtil:      kubeUtil,
		dnsConfig:     dnsConfig,
		radixDNSAlias: radixDNSAlias,
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
	domainName := s.radixDNSAlias.GetName()
	envNamespace := utils.GetEnvironmentNamespace(aliasSpec.AppName, aliasSpec.Environment)
	ingressName := GetDNSAliasIngressName(aliasSpec.Component, domainName)
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
		return err
	}
	aliasSpec := s.radixDNSAlias.Spec
	_, err = CreateRadixDNSAliasIngress(s.kubeClient, aliasSpec.AppName, aliasSpec.Environment, ingress)
	return err
}

func (s *syncer) buildIngress() (*networkingv1.Ingress, error) {
	aliasSpec := s.radixDNSAlias.Spec
	domain := s.radixDNSAlias.GetName()
	return BuildRadixDNSAliasIngress(aliasSpec.AppName, domain, aliasSpec.Component, aliasSpec.Port, s.radixDNSAlias, s.dnsConfig), nil
}

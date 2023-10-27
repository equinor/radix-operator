package dnsalias

import (
	"context"
	"fmt"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/dnsalias/internal"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/radix-operator/config"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	clusterConfig *config.ClusterConfig
}

// NewSyncer is the constructor for RadixDNSAlias syncer
func NewSyncer(kubeClient kubernetes.Interface, kubeUtil *kube.Kube, radixClient radixclient.Interface, clusterConfig *config.ClusterConfig, radixDNSAlias *radixv1.RadixDNSAlias) Syncer {
	return &syncer{
		kubeClient:    kubeClient,
		radixClient:   radixClient,
		kubeUtil:      kubeUtil,
		clusterConfig: clusterConfig,
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
	appName := aliasSpec.AppName
	domainName := s.radixDNSAlias.GetName()
	radixApplication, err := s.radixClient.RadixV1().RadixApplications(utils.GetAppNamespace(appName)).
		Get(context.Background(), appName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("not found the Radix application %s for the DNS alias %s", appName, domainName)
		}
		return err
	}
	envNamespace := utils.GetEnvironmentNamespace(aliasSpec.AppName, aliasSpec.Environment)
	ingressName := internal.GetDNSAliasIngressName(aliasSpec.Component, domainName)
	ingress, err := s.kubeClient.NetworkingV1().Ingresses(envNamespace).Get(context.Background(), ingressName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return s.createIngress(radixApplication)
		}
		return err
	}
	// TODO
	fmt.Println(radixApplication.GetName())
	fmt.Println(ingress.GetName())
	return nil
}

func (s *syncer) createIngress(ra *radixv1.RadixApplication) error {
	aliasSpec := s.radixDNSAlias.Spec
	portNumber, err := getComponentPublicPortNumber(ra, aliasSpec.Component)
	if err != nil {
		return err
	}
	domain := s.radixDNSAlias.GetName()
	_, err = internal.CreateRadixDNSAliasIngress(s.kubeClient, aliasSpec.AppName, aliasSpec.Environment,
		internal.BuildRadixDNSAliasIngress(aliasSpec.AppName, domain, aliasSpec.Component, portNumber, s.radixDNSAlias, s.clusterConfig))
	return err
}

func getComponentPublicPortNumber(ra *radixv1.RadixApplication, componentName string) (int32, error) {
	component, componentExists := slice.FindFirst(ra.Spec.Components, func(c radixv1.RadixComponent) bool {
		return c.Name == componentName
	})
	if !componentExists {
		return 0, fmt.Errorf("not found component %s in the application %s for the DNS alias", componentName, ra.GetName())
	}
	componentPort, publicPortExists := slice.FindFirst(component.Ports, func(p radixv1.ComponentPort) bool {
		return p.Name == component.PublicPort
	})
	if !publicPortExists {
		return 0, fmt.Errorf("not found component %s in the application %s for the DNS alias", componentName, ra.GetName())
	}

	return componentPort.Port, nil
}

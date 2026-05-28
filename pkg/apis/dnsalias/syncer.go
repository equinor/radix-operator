package dnsalias

import (
	"context"
	"fmt"
	"sync"

	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Syncer of  RadixDNSAliases
type Syncer interface {
	// OnSync Syncs RadixDNSAliases
	OnSync(ctx context.Context) error
}

// DNSAlias is the aggregate-root for manipulating RadixDNSAliases
type syncer struct {
	radixClient         radixclient.Interface
	dynamicClient       client.Client
	kubeUtil            *kube.Kube
	radixDNSAlias       *radixv1.RadixDNSAlias
	oauth2DefaultConfig defaults.OAuth2Config
	rd                  *radixv1.RadixDeployment
	rr                  *radixv1.RadixRegistration
	component           *radixv1.RadixDeployComponent
	initMutex           sync.Mutex
	config              config.Config
}

// NewSyncer is the constructor for RadixDNSAlias syncer
func NewSyncer(radixDNSAlias *radixv1.RadixDNSAlias, kubeUtil *kube.Kube, radixClient radixclient.Interface, dynamicClient client.Client, config config.Config, oauth2Config defaults.OAuth2Config) Syncer {
	return &syncer{
		radixClient:         radixClient,
		dynamicClient:       dynamicClient,
		kubeUtil:            kubeUtil,
		config:              config,
		oauth2DefaultConfig: oauth2Config,
		radixDNSAlias:       radixDNSAlias,
	}
}

// OnSync is called by the handler when changes are applied and must be
// reconciled with current state.
func (s *syncer) OnSync(ctx context.Context) error {
	log.Ctx(ctx).Info().Msg("Syncing")
	s.initMutex.Lock()
	defer s.initMutex.Unlock()

	return s.syncStatus(ctx, s.reconcile(ctx))
}

func (s *syncer) reconcile(ctx context.Context) error {

	if err := s.init(ctx); err != nil {
		return fmt.Errorf("failed to init: %w", err)
	}

	if err := s.reconcileHTTPRoute(ctx); err != nil {
		return fmt.Errorf("failed to reconcile HTTPRoute: %w", err)
	}

	return nil
}

func (s *syncer) init(ctx context.Context) error {
	s.rr = nil
	s.rd = nil
	s.component = nil

	rr, err := s.kubeUtil.GetRegistration(ctx, s.radixDNSAlias.Spec.AppName)
	if err != nil {
		return fmt.Errorf("failed to get RadixRegistration: %w", err)
	}
	s.rr = rr

	ns := utils.GetEnvironmentNamespace(s.radixDNSAlias.Spec.AppName, s.radixDNSAlias.Spec.Environment)
	rd, err := kube.GetActiveDeployment(ctx, s.kubeUtil.RadixClient(), ns)
	if err != nil {
		return fmt.Errorf("failed to get active RadixDeployment: %w", err)
	}
	s.rd = rd

	if s.rd != nil {
		component := s.rd.GetComponentByName(s.radixDNSAlias.Spec.Component)

		if component != nil {
			component, err := s.buildComponentWithOAuthDefaults(component)
			if err != nil {
				return err
			}
			s.component = component
		}
	}

	return nil
}

func (s *syncer) buildComponentWithOAuthDefaults(component *radixv1.RadixDeployComponent) (*radixv1.RadixDeployComponent, error) {
	if component.GetAuthentication().GetOAuth2() == nil {
		return component, nil
	}
	componentWithOAuthDefaults := component.DeepCopy()
	oauth, err := s.oauth2DefaultConfig.MergeWith(componentWithOAuthDefaults.Authentication.OAuth2)
	if err != nil {
		return nil, err
	}
	componentWithOAuthDefaults.Authentication.OAuth2 = oauth
	return componentWithOAuthDefaults, nil
}

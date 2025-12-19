package dnsalias

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Syncer of  RadixDNSAliases
type Syncer interface {
	// OnSync Syncs RadixDNSAliases
	OnSync(ctx context.Context) error
}

// DNSAlias is the aggregate-root for manipulating RadixDNSAliases
type syncer struct {
	kubeClient                      kubernetes.Interface
	radixClient                     radixclient.Interface
	kubeUtil                        *kube.Kube
	radixDNSAlias                   *radixv1.RadixDNSAlias
	dnsZone                         string
	oauth2DefaultConfig             defaults.OAuth2Config
	componentIngressAnnotations     []ingress.AnnotationProvider
	oauthIngressAnnotations         []ingress.AnnotationProvider
	oauthProxyModeIngressAnnotation []ingress.AnnotationProvider
	rd                              *radixv1.RadixDeployment
	rr                              *radixv1.RadixRegistration
	component                       *radixv1.RadixDeployComponent
	initMutex                       sync.Mutex
}

// NewSyncer is the constructor for RadixDNSAlias syncer
func NewSyncer(radixDNSAlias *radixv1.RadixDNSAlias, kubeClient kubernetes.Interface, kubeUtil *kube.Kube, radixClient radixclient.Interface, dnsZone string, oauth2Config defaults.OAuth2Config, componentIngressAnnotations []ingress.AnnotationProvider, oauthIngressAnnotations []ingress.AnnotationProvider, oauthProxyModeIngressAnnotation []ingress.AnnotationProvider) Syncer {
	return &syncer{
		kubeClient:                      kubeClient,
		radixClient:                     radixClient,
		kubeUtil:                        kubeUtil,
		dnsZone:                         dnsZone,
		oauth2DefaultConfig:             oauth2Config,
		componentIngressAnnotations:     componentIngressAnnotations,
		oauthIngressAnnotations:         oauthIngressAnnotations,
		oauthProxyModeIngressAnnotation: oauthProxyModeIngressAnnotation,
		radixDNSAlias:                   radixDNSAlias,
	}
}

// OnSync is called by the handler when changes are applied and must be
// reconciled with current state.
func (s *syncer) OnSync(ctx context.Context) error {
	log.Ctx(ctx).Info().Msg("Syncing")
	s.initMutex.Lock()
	defer s.initMutex.Unlock()

	if s.radixDNSAlias.ObjectMeta.DeletionTimestamp != nil {
		return s.handleFinalizer(ctx)
	}

	if err := s.init(ctx); err != nil {
		return fmt.Errorf("failed to init: %w", err)
	}

	return s.syncStatus(ctx, s.syncAlias(ctx))

}

func (s *syncer) init(ctx context.Context) error {
	rr, err := s.kubeUtil.GetRegistration(ctx, s.radixDNSAlias.Spec.AppName)
	if err != nil {
		return fmt.Errorf("failed to get RadixRegistration: %w", err)
	}
	s.rr = rr

	ns := utils.GetEnvironmentNamespace(s.radixDNSAlias.Spec.AppName, s.radixDNSAlias.Spec.Environment)
	rd, err := s.kubeUtil.GetActiveDeployment(ctx, ns)
	if err != nil {
		return fmt.Errorf("failed to get active RadixDeployment: %w", err)
	}
	s.rd = rd

	if s.rd != nil {
		component := s.rd.GetComponentByName(s.radixDNSAlias.Spec.Component)
		component, err := s.buildComponentWithOAuthDefaults(component)
		if err != nil {
			return err
		}
		s.component = component
	}

	return nil
}

func (s *syncer) syncAlias(ctx context.Context) error {
	if err := s.syncIngresses(ctx); err != nil {
		return err
	}
	return s.syncRbac(ctx)
}

func (s *syncer) handleFinalizer(ctx context.Context) error {
	finalizerIndex := slices.Index(s.radixDNSAlias.ObjectMeta.Finalizers, kube.RadixDNSAliasFinalizer)
	if finalizerIndex < 0 {
		return nil
	}

	log.Ctx(ctx).Info().Msg("Process finalizer")

	if err := s.deleteIngresses(ctx); err != nil {
		return err
	}
	if err := s.deleteRbac(ctx); err != nil {
		return err
	}

	finalizers := append(s.radixDNSAlias.ObjectMeta.Finalizers[:finalizerIndex], s.radixDNSAlias.ObjectMeta.Finalizers[finalizerIndex+1:]...)
	s.radixDNSAlias.ObjectMeta.Finalizers = finalizers
	return s.kubeUtil.UpdateRadixDNSAlias(ctx, s.radixDNSAlias)
}

func (s *syncer) getExistingClusterRoleOwnerReferences(ctx context.Context, roleName string) ([]metav1.OwnerReference, error) {
	clusterRole, err := s.kubeUtil.GetClusterRole(ctx, roleName)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return clusterRole.GetOwnerReferences(), nil
}

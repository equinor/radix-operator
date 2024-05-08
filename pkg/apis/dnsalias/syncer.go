package dnsalias

import (
	"context"
	"fmt"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// Syncer of  RadixDNSAliases
type Syncer interface {
	// OnSync Syncs RadixDNSAliases
	OnSync(ctx context.Context) error
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
	logger                     zerolog.Logger
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
		logger:                     log.Logger.With().Str("resource_kind", radixv1.KindRadixDNSAlias).Str("resource_name", cache.MetaObjectToName(&radixDNSAlias.ObjectMeta).String()).Logger(),
	}
}

// OnSync is called by the handler when changes are applied and must be
// reconciled with current state.
func (s *syncer) OnSync(ctx context.Context) error {
	s.logger.Debug().Msgf("OnSync RadixDNSAlias %s, application %s, environment %s, component %s", s.radixDNSAlias.GetName(), s.radixDNSAlias.Spec.AppName, s.radixDNSAlias.Spec.Environment, s.radixDNSAlias.Spec.Component)
	if err := s.restoreStatus(ctx); err != nil {
		return fmt.Errorf("failed to update status on DNS alias %s: %v", s.radixDNSAlias.GetName(), err)
	}
	if s.radixDNSAlias.ObjectMeta.DeletionTimestamp != nil {
		return s.handleDeletedRadixDNSAlias(ctx)
	}
	return s.syncStatus(ctx, s.syncAlias(ctx))

}

func (s *syncer) syncAlias(ctx context.Context) error {
	s.logger.Debug().Msgf("syncAlias RadixDNSAlias %s", s.radixDNSAlias.GetName())
	if err := s.syncIngresses(ctx); err != nil {
		return err
	}
	return s.syncRbac(ctx)
}

func (s *syncer) syncIngresses(ctx context.Context) error {
	radixDeployComponent, err := s.getRadixDeployComponent(ctx)
	if err != nil {
		return err
	}
	if radixDeployComponent == nil {
		return nil // there is no any RadixDeployment (probably it is just created app). Do not sync, radixDeploymentInformer in the RadixDNSAlias controller will call the re-sync, when the RadixDeployment is added
	}

	aliasSpec := s.radixDNSAlias.Spec
	namespace := utils.GetEnvironmentNamespace(aliasSpec.AppName, aliasSpec.Environment)
	ing, err := s.syncIngress(ctx, namespace, radixDeployComponent)
	if err != nil {
		return err
	}
	return s.syncOAuthProxyIngress(ctx, namespace, ing, radixDeployComponent)
}

func (s *syncer) getRadixDeployComponent(ctx context.Context) (radixv1.RadixCommonDeployComponent, error) {
	aliasSpec := s.radixDNSAlias.Spec
	namespace := utils.GetEnvironmentNamespace(aliasSpec.AppName, aliasSpec.Environment)

	s.logger.Debug().Msgf("get active deployment for the namespace %s", namespace)
	radixDeployment, err := s.kubeUtil.GetActiveDeployment(ctx, namespace)
	if err != nil {
		return nil, err
	}
	if radixDeployment == nil {
		return nil, nil
	}
	s.logger.Debug().Msgf("active deployment for the namespace %s is %s", namespace, radixDeployment.GetName())

	deployComponent := radixDeployment.GetCommonComponentByName(aliasSpec.Component)
	if commonUtils.IsNil(deployComponent) {
		return nil, DeployComponentNotFoundByName(aliasSpec.AppName, aliasSpec.Environment, aliasSpec.Component, radixDeployment.GetName())
	}
	return deployComponent, nil
}

func (s *syncer) handleDeletedRadixDNSAlias(ctx context.Context) error {
	s.logger.Debug().Msgf("handle deleted RadixDNSAlias %s in the application %s", s.radixDNSAlias.Name, s.radixDNSAlias.Spec.AppName)
	finalizerIndex := slice.FindIndex(s.radixDNSAlias.ObjectMeta.Finalizers, func(val string) bool {
		return val == kube.RadixDNSAliasFinalizer
	})
	if finalizerIndex < 0 {
		s.logger.Info().Msgf("missing finalizer %s in the RadixDNSAlias %s. Exist finalizers: %d. Skip dependency handling",
			kube.RadixDNSAliasFinalizer, s.radixDNSAlias.Name, len(s.radixDNSAlias.ObjectMeta.Finalizers))
		return nil
	}

	selector := radixlabels.ForDNSAliasIngress(s.radixDNSAlias.Spec.AppName, s.radixDNSAlias.Spec.Component, s.radixDNSAlias.GetName())
	if err := s.deleteIngresses(ctx, selector); err != nil {
		return err
	}
	if err := s.deleteRbac(ctx); err != nil {
		return err
	}

	updatingAlias := s.radixDNSAlias.DeepCopy()
	updatingAlias.ObjectMeta.Finalizers = append(s.radixDNSAlias.ObjectMeta.Finalizers[:finalizerIndex], s.radixDNSAlias.ObjectMeta.Finalizers[finalizerIndex+1:]...)
	s.logger.Debug().Msgf("removed finalizer %s from the RadixDNSAlias %s for the application %s. Left finalizers: %d",
		kube.RadixEnvironmentFinalizer, updatingAlias.Name, updatingAlias.Spec.AppName, len(updatingAlias.ObjectMeta.Finalizers))

	return s.kubeUtil.UpdateRadixDNSAlias(ctx, updatingAlias)
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

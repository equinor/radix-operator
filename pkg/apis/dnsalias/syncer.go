package dnsalias

import (
	"fmt"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/dnsalias/internal"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
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
	if handled, err := s.handleDeletedRadixDNSAlias(); handled || err != nil {
		return err
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
	if radixDeployComponent == nil {
		return nil // there is no any RadixDeployment (probably it is just created app). Do not sync, radixDeploymentInformer in the RadixDNSAlias controller will call the re-sync, when the RadixDeployment is added
	}

	aliasSpec := s.radixDNSAlias.Spec
	ingressName := GetDNSAliasIngressName(aliasSpec.Component, aliasName)
	namespace := utils.GetEnvironmentNamespace(aliasSpec.AppName, aliasSpec.Environment)
	ing, err := s.createOrUpdateIngress(namespace, radixDeployComponent, ingressName)
	if err != nil {
		return err
	}
	return s.createOrUpdateOAuthProxyIngressForComponentIngress(radixDeployComponent.GetAuthentication(), namespace, aliasSpec, radixDeployComponent, ing)
}

func (s *syncer) createOrUpdateOAuthProxyIngressForComponentIngress(componentAuthentication *radixv1.Authentication, namespace string, aliasSpec radixv1.RadixDNSAliasSpec, radixDeployComponent radixv1.RadixCommonDeployComponent, ing *networkingv1.Ingress) error {
	if componentAuthentication == nil || componentAuthentication.OAuth2 == nil {
		return nil
	}
	oauth, err := s.oauth2DefaultConfig.MergeWith(componentAuthentication.OAuth2)
	if err != nil {
		return err
	}
	componentAuthentication.OAuth2 = oauth
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
		return nil, nil
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
			return s.createIngress(radixDeployComponent, s.radixDNSAlias, s.dnsConfig, s.oauth2DefaultConfig, s.ingressConfiguration)
		}
		return nil, err
	}
	log.Debugf("found Ingress %s in the namespace %s.", ingressName, namespace)
	updatedIngress, err := buildIngress(radixDeployComponent, s.radixDNSAlias, s.dnsConfig, s.oauth2DefaultConfig, s.ingressConfiguration)
	if err != nil {
		return nil, err
	}
	ing, err := s.kubeUtil.PatchIngress(namespace, existingIngress, updatedIngress)
	if err != nil {
		return nil, fmt.Errorf("failed to patch an ingress %s: %w", ingressName, err)
	}
	return ing, nil
}

func (s *syncer) createIngress(radixDeployComponent radixv1.RadixCommonDeployComponent, radixDNSAlias *radixv1.RadixDNSAlias, dnsConfig *dnsalias.DNSConfig, oauth2Config defaults.OAuth2Config, ingressConfiguration ingress.IngressConfiguration) (*networkingv1.Ingress, error) {
	ing, err := buildIngress(radixDeployComponent, radixDNSAlias, dnsConfig, oauth2Config, ingressConfiguration)
	if err != nil {
		return nil, err
	}
	aliasSpec := radixDNSAlias.Spec
	log.Debugf("create an ingress %s for the RadixDNSAlias", ing.GetName())
	return CreateRadixDNSAliasIngress(s.kubeClient, aliasSpec.AppName, aliasSpec.Environment, ing)
}

func (s *syncer) handleDeletedRadixDNSAlias() (bool, error) {
	if s.radixDNSAlias.ObjectMeta.DeletionTimestamp == nil {
		return false, nil
	}
	log.Debugf("handle deleted RadixDNSAlias %s in the application %s", s.radixDNSAlias.Name, s.radixDNSAlias.Spec.AppName)
	finalizerIndex := slice.FindIndex(s.radixDNSAlias.ObjectMeta.Finalizers, func(val string) bool {
		return val == kube.RadixDNSAliasFinalizer
	})
	if finalizerIndex < 0 {
		log.Infof("missing finalizer %s in the RadixDNSAlias %s. Exist finalizers: %d. Skip dependency handling",
			kube.RadixDNSAliasFinalizer, s.radixDNSAlias.Name, len(s.radixDNSAlias.ObjectMeta.Finalizers))
		return false, nil
	}
	if err := s.deletedIngressesForRadixDNSAlias(); err != nil {
		return true, err
	}
	updatingAlias := s.radixDNSAlias.DeepCopy()
	updatingAlias.ObjectMeta.Finalizers = append(s.radixDNSAlias.ObjectMeta.Finalizers[:finalizerIndex], s.radixDNSAlias.ObjectMeta.Finalizers[finalizerIndex+1:]...)
	log.Debugf("removed finalizer %s from the RadixDNSAlias %s for the application %s. Left finalizers: %d",
		kube.RadixEnvironmentFinalizer, updatingAlias.Name, updatingAlias.Spec.AppName, len(updatingAlias.ObjectMeta.Finalizers))
	if err := s.kubeUtil.UpdateRadixDNSAlias(updatingAlias); err != nil {
		return false, err
	}
	return true, nil
}

func (s *syncer) deletedIngressesForRadixDNSAlias() error {
	aliasSpec := s.radixDNSAlias.Spec
	ingresses, err := s.kubeUtil.GetIngressesWithSelector(utils.GetEnvironmentNamespace(aliasSpec.AppName, aliasSpec.Environment),
		radixlabels.Merge(radixlabels.ForApplicationName(aliasSpec.AppName), radixlabels.ForComponentName(aliasSpec.Component), radixlabels.ForDNSAlias()).String())
	if err != nil {
		return err
	}
	log.Debugf("delete %d Ingress(es)", len(ingresses))
	return s.kubeUtil.DeleteIngresses(true, ingresses...)
}

func buildIngress(radixDeployComponent radixv1.RadixCommonDeployComponent, radixDNSAlias *radixv1.RadixDNSAlias, dnsConfig *dnsalias.DNSConfig, oauth2Config defaults.OAuth2Config, ingressConfiguration ingress.IngressConfiguration) (*networkingv1.Ingress, error) {
	log.Debug("build an ingress for the RadixDNSAlias")
	aliasSpec := radixDNSAlias.Spec
	appName := aliasSpec.AppName
	envName := aliasSpec.Environment
	componentName := aliasSpec.Component
	namespace := utils.GetEnvironmentNamespace(appName, envName)
	aliasName := radixDNSAlias.GetName()
	ingressName := GetDNSAliasIngressName(componentName, aliasName)
	hostName := GetDNSAliasHost(aliasName, dnsConfig.DNSZone)

	ingressSpec := ingress.GetIngressSpec(hostName, componentName, defaults.TLSSecretName, aliasSpec.Port)
	ingressAnnotations := ingress.GetAnnotationProvider(ingressConfiguration, namespace, oauth2Config)
	ingressConfig, err := ingress.GetIngressConfig(namespace, appName, radixDeployComponent, ingressName, ingressSpec, ingressAnnotations, ingress.DNSAlias, internal.GetOwnerReferences(radixDNSAlias))
	if err != nil {
		return nil, err
	}

	ingressConfig.ObjectMeta.Annotations = annotations.Merge(ingressConfig.ObjectMeta.Annotations, annotations.ForManagedByRadixDNSAliasIngress(aliasName))
	log.Debugf("built the Ingress %s in the environment %s with a host %s", ingressConfig.GetName(), namespace, hostName)
	return ingressConfig, nil
}

package dnsalias

import (
	"context"
	"fmt"
	"regexp"

	commonUtils "github.com/equinor/radix-common/utils"
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
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	networkingv1 "k8s.io/api/networking/v1"
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
	kubeClient                 kubernetes.Interface
	radixClient                radixclient.Interface
	kubeUtil                   *kube.Kube
	radixDNSAlias              *radixv1.RadixDNSAlias
	dnsConfig                  *dnsalias.DNSConfig
	ingressConfiguration       ingress.IngressConfiguration
	oauth2DefaultConfig        defaults.OAuth2Config
	ingressAnnotationProviders []ingress.AnnotationProvider
}

var admissionWebhookErrorExpression *regexp.Regexp

// NewSyncer is the constructor for RadixDNSAlias syncer
func NewSyncer(kubeClient kubernetes.Interface, kubeUtil *kube.Kube, radixClient radixclient.Interface, dnsConfig *dnsalias.DNSConfig, ingressConfiguration ingress.IngressConfiguration, oauth2Config defaults.OAuth2Config, ingressAnnotationProviders []ingress.AnnotationProvider, radixDNSAlias *radixv1.RadixDNSAlias) Syncer {
	admissionWebhookErrorExpression = regexp.MustCompile(`admission webhook "validate.nginx.ingress.kubernetes.io" denied the request: host "(.*?)" and path "(.*?)" is already defined in ingress (.*?)/(.*?)$`)
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
	log.Debugf("OnSync RadixDNSAlias %s, application %s, environment %s, component %s", s.radixDNSAlias.GetName(), s.radixDNSAlias.Spec.AppName, s.radixDNSAlias.Spec.Environment, s.radixDNSAlias.Spec.Component)
	if err := s.restoreStatus(); err != nil {
		return fmt.Errorf("failed to update status on DNS alias %s: %v", s.radixDNSAlias.GetName(), err)
	}
	if s.radixDNSAlias.ObjectMeta.DeletionTimestamp != nil {
		return s.syncStatus(s.handleDeletedRadixDNSAlias())
	}
	return s.syncStatus(s.syncAlias())

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
	ingressName := GetDNSAliasIngressName(aliasName)
	namespace := utils.GetEnvironmentNamespace(aliasSpec.AppName, aliasSpec.Environment)

	ing, err := s.syncIngress(namespace, radixDeployComponent, ingressName)
	if err != nil {
		return s.getDNSAliasError(err)
	}

	return s.syncOAuthProxyIngress(radixDeployComponent, namespace, aliasSpec, radixDeployComponent, ing)
}

func (s *syncer) getDNSAliasError(err error) error {
	if admissionWebhookErrorMatcher := admissionWebhookErrorExpression.FindStringSubmatch(err.Error()); len(admissionWebhookErrorMatcher) == 5 {
		log.Error(err)
		return fmt.Errorf("DNS alias %s cannot be used, because the host %s with the path %s is already in use", s.radixDNSAlias.GetName(), admissionWebhookErrorMatcher[1], admissionWebhookErrorMatcher[2])
	}
	return err
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

func (s *syncer) handleDeletedRadixDNSAlias() error {
	log.Debugf("handle deleted RadixDNSAlias %s in the application %s", s.radixDNSAlias.Name, s.radixDNSAlias.Spec.AppName)
	finalizerIndex := slice.FindIndex(s.radixDNSAlias.ObjectMeta.Finalizers, func(val string) bool {
		return val == kube.RadixDNSAliasFinalizer
	})
	if finalizerIndex < 0 {
		log.Infof("missing finalizer %s in the RadixDNSAlias %s. Exist finalizers: %d. Skip dependency handling",
			kube.RadixDNSAliasFinalizer, s.radixDNSAlias.Name, len(s.radixDNSAlias.ObjectMeta.Finalizers))
		return nil
	}

	if err := s.deletedIngressesForRadixDNSAlias(); err != nil {
		return err
	}

	updatingAlias := s.radixDNSAlias.DeepCopy()
	updatingAlias.ObjectMeta.Finalizers = append(s.radixDNSAlias.ObjectMeta.Finalizers[:finalizerIndex], s.radixDNSAlias.ObjectMeta.Finalizers[finalizerIndex+1:]...)
	log.Debugf("removed finalizer %s from the RadixDNSAlias %s for the application %s. Left finalizers: %d",
		kube.RadixEnvironmentFinalizer, updatingAlias.Name, updatingAlias.Spec.AppName, len(updatingAlias.ObjectMeta.Finalizers))

	return s.kubeUtil.UpdateRadixDNSAlias(updatingAlias)
}

func (s *syncer) deletedIngressesForRadixDNSAlias() error {
	aliasSpec := s.radixDNSAlias.Spec
	namespace := utils.GetEnvironmentNamespace(aliasSpec.AppName, aliasSpec.Environment)
	dnsAliasIngressesSelector := radixlabels.ForDNSAliasIngress(aliasSpec.AppName, aliasSpec.Component, s.radixDNSAlias.GetName()).String()
	ingresses, err := s.kubeUtil.KubeClient().NetworkingV1().Ingresses(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: dnsAliasIngressesSelector})
	if err != nil {
		return err
	}
	return s.kubeUtil.DeleteIngresses(slice.PointersOf(ingresses.Items).([]*networkingv1.Ingress)...)
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

func getComponentPublicPort(component radixv1.RadixCommonDeployComponent) *radixv1.ComponentPort {
	if port, ok := slice.FindFirst(component.GetPorts(), func(p radixv1.ComponentPort) bool { return p.Name == component.GetPublicPort() }); ok {
		return &port
	}
	return nil
}

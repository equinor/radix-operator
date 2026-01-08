package dnsalias

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/ingress"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/pkg/apis/utils/oauth"
	networkingv1 "k8s.io/api/networking/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (s *syncer) syncIngresses(ctx context.Context) error {

	if err := s.garbageCollectOAuthIngress(ctx); err != nil {
		return fmt.Errorf("failed to garbage collect OAuth2 ingress: %w", err)
	}

	if err := s.garbageCollectComponentIngress(ctx); err != nil {
		return fmt.Errorf("failed to garbage collect ingress: %w", err)
	}

	if err := s.createOrUpdateComponentIngress(ctx); err != nil {
		return fmt.Errorf("failed to sync ingress: %w", err)
	}

	if err := s.createOrUpdateOAuthIngress(ctx); err != nil {
		return fmt.Errorf("failed to sync OAuth2 ingress: %w", err)
	}

	return nil
}

func (s *syncer) createOrUpdateComponentIngress(ctx context.Context) error {
	mustApply := func() bool {
		if s.component == nil {
			return false
		}

		if !s.component.IsPublic() {
			return false
		}

		if s.component.GetAuthentication().GetOAuth2() != nil && s.isProxyModeEnabled() {
			return false
		}

		return true
	}

	if !mustApply() {
		return nil
	}

	annotations, err := ingress.BuildAnnotationsFromProviders(s.component, s.componentIngressAnnotations)
	if err != nil {
		return fmt.Errorf("failed to build annotations: %w", err)
	}

	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        s.getIngressName(),
			Labels:      radixlabels.ForDNSAliasComponentIngress(s.radixDNSAlias),
			Annotations: annotations,
		},
		Spec: ingress.BuildIngressSpecForComponent(s.component, s.getHostName(), ""),
	}

	if err := controllerutil.SetControllerReference(s.radixDNSAlias, ing, scheme); err != nil {
		return fmt.Errorf("failed to set ownerreference: %w", err)
	}

	if err := s.kubeUtil.ApplyIngress(ctx, s.rd.Namespace, ing); err != nil {
		return fmt.Errorf("failed to create or update ingress: %w", err)
	}

	return nil
}

func (s *syncer) createOrUpdateOAuthIngress(ctx context.Context) error {
	mustApply := func() bool {
		if s.component == nil {
			return false
		}

		if !s.component.IsPublic() {
			return false
		}

		return s.component.GetAuthentication().GetOAuth2() != nil
	}

	if !mustApply() {
		return nil
	}

	annotationProviders := s.oauthIngressAnnotations
	if s.isProxyModeEnabled() {
		annotationProviders = s.oauthProxyModeIngressAnnotation
	}
	annotations, err := ingress.BuildAnnotationsFromProviders(s.component, annotationProviders)
	if err != nil {
		return fmt.Errorf("failed to build annotations: %w", err)
	}

	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        oauth.GetAuxOAuthProxyIngressName(s.getIngressName()),
			Labels:      radixlabels.ForDNSAliasComponentIngress(s.radixDNSAlias),
			Annotations: annotations,
		},
		Spec: ingress.BuildIngressSpecForOAuth2Component(s.component, s.getHostName(), "", s.isProxyModeEnabled()),
	}

	if err := controllerutil.SetControllerReference(s.radixDNSAlias, ing, scheme); err != nil {
		return fmt.Errorf("failed to set ownerreference: %w", err)
	}

	if err := s.kubeUtil.ApplyIngress(ctx, s.rd.Namespace, ing); err != nil {
		return fmt.Errorf("failed to create or update ingress: %w", err)
	}

	return nil
}

func (s *syncer) garbageCollectOAuthIngress(ctx context.Context) error {
	ingressName := oauth.GetAuxOAuthProxyIngressName(s.getIngressName())
	namespace := utils.GetEnvironmentNamespace(s.radixDNSAlias.Spec.AppName, s.radixDNSAlias.Spec.Environment)

	ing, err := s.kubeClient.NetworkingV1().Ingresses(namespace).Get(ctx, ingressName, metav1.GetOptions{})
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get ingress: %w", err)
	}

	mustDelete := func() bool {
		if s.component == nil {
			return true
		}

		if !s.component.IsPublic() {
			return true
		}

		if s.component.GetAuthentication().GetOAuth2() == nil {
			return true
		}

		expectedPath := ingress.BuildIngressSpecForOAuth2Component(s.component, "", "", s.isProxyModeEnabled()).Rules[0].HTTP.Paths[0].Path
		return ing.Spec.Rules[0].HTTP.Paths[0].Path != expectedPath
	}

	if !mustDelete() {
		return nil
	}

	if err := s.kubeClient.NetworkingV1().Ingresses(ing.Namespace).Delete(ctx, ing.Name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("failed to delete ingress: %w", err)
	}

	return nil
}

func (s *syncer) garbageCollectComponentIngress(ctx context.Context) error {
	namespace := utils.GetEnvironmentNamespace(s.radixDNSAlias.Spec.AppName, s.radixDNSAlias.Spec.Environment)
	ing, err := s.kubeClient.NetworkingV1().Ingresses(namespace).Get(ctx, s.getIngressName(), metav1.GetOptions{})
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get ingress: %w", err)
	}

	mustDelete := func() bool {
		if s.component == nil {
			return true
		}

		if !s.component.IsPublic() {
			return true
		}

		return s.component.GetAuthentication().GetOAuth2() != nil && s.isProxyModeEnabled()
	}

	if !mustDelete() {
		return nil
	}

	if err := s.kubeClient.NetworkingV1().Ingresses(ing.Namespace).Delete(ctx, ing.Name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("failed to delete ingress: %w", err)
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

func (s *syncer) isProxyModeEnabled() bool {
	var rdProxyMode, rrProxyMode bool

	if s.rd != nil {
		rdProxyMode = annotations.OAuth2ProxyModeEnabledForEnvironment(s.rd.Annotations, s.rd.Spec.Environment)
	}

	rrProxyMode = annotations.OAuth2ProxyModeEnabledForEnvironment(s.rr.Annotations, s.rd.Spec.Environment)

	return rdProxyMode || rrProxyMode
}

func (s *syncer) getHostName() string {
	return fmt.Sprintf("%s.%s", s.radixDNSAlias.Name, s.dnsZone)
}

func (s *syncer) getIngressName() string {
	return fmt.Sprintf("%s.custom-alias", s.radixDNSAlias.Name)
}

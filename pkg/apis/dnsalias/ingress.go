package dnsalias

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/ingress"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/pkg/apis/utils/oauth"
	networkingv1 "k8s.io/api/networking/v1"
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

		if s.component.GetAuthentication().GetOAuth2() != nil {
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

	annotations, err := ingress.BuildAnnotationsFromProviders(s.component, s.oauthProxyIngressAnnotations)
	if err != nil {
		return fmt.Errorf("failed to build annotations: %w", err)
	}

	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        oauth.GetAuxOAuthProxyIngressName(s.getIngressName()),
			Labels:      radixlabels.ForDNSAliasComponentIngress(s.radixDNSAlias),
			Annotations: annotations,
		},
		Spec: ingress.BuildIngressSpecForOAuth2Component(s.component, s.getHostName(), ""),
	}

	if err := controllerutil.SetControllerReference(s.radixDNSAlias, ing, scheme); err != nil {
		return fmt.Errorf("failed to set ownerreference: %w", err)
	}

	if err := s.kubeUtil.ApplyIngress(ctx, s.rd.Namespace, ing); err != nil {
		return fmt.Errorf("failed to create or update ingress: %w", err)
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

func (s *syncer) getHostName() string {
	return fmt.Sprintf("%s.%s", s.radixDNSAlias.Name, s.config.DNSZone)
}

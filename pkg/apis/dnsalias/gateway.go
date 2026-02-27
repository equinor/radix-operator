package dnsalias

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/gateway"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubelabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func (s *syncer) reconcileHTTPRoute(ctx context.Context) error {
	logger := log.Ctx(ctx)

	// When everyone is using proxy mode, or its enforced, cleanup this code (https://github.com/equinor/radix-platform/issues/1822)
	namespace := utils.GetEnvironmentNamespace(s.radixDNSAlias.Spec.AppName, s.radixDNSAlias.Spec.Environment)
	route := &gatewayapiv1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Name: s.getIngressName(), Namespace: namespace}}

	if _, hasPublicPort := s.component.GetPublicPortNumber(); !hasPublicPort {
		err := s.dynamicClient.Delete(ctx, route)
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to delete HTTPRoute %s: %w", route.Name, err)
		}
		if err == nil {
			logger.Info().Str("httproute", route.Name).Str("op", "delete").Msg("reconcile HTTPRoute")
		}

		return nil
	}

	oauth2enabled := s.component.GetAuthentication().GetOAuth2() != nil

	op, err := controllerutil.CreateOrUpdate(ctx, s.dynamicClient, route, func() error {
		parentRefs := []gatewayapiv1.ParentReference{{
			Group:       new(gatewayapiv1.Group(gatewayapiv1.GroupName)),
			Kind:        new(gatewayapiv1.Kind("Gateway")),
			Name:        gatewayapiv1.ObjectName(s.config.Gateway.Name),
			Namespace:   new(gatewayapiv1.Namespace(s.config.Gateway.Namespace)),
			SectionName: new(gatewayapiv1.SectionName(s.config.Gateway.SectionName)),
		}}

		route.Labels = kubelabels.Merge(route.Labels, labels.ForDNSAliasComponentGatewayResource(s.radixDNSAlias))
		if route.Annotations == nil {
			route.Annotations = map[string]string{}
		}

		if s.isGatewayAPIEnabled() {
			route.Annotations[annotations.PreviewGatewayModeAnnotation] = "true"
			route.Annotations["external-dns.alpha.kubernetes.io/ttl"] = "30"
		} else {
			delete(route.Annotations, annotations.PreviewGatewayModeAnnotation)
			delete(route.Annotations, "external-dns.alpha.kubernetes.io/ttl")
		}

		var backendRef gatewayapiv1.HTTPBackendRef
		if oauth2enabled {
			backendRef = gateway.BuildBackendRefForComponentOauth2Service(s.component)
		} else {
			var err error
			backendRef, err = gateway.BuildBackendRefForComponent(s.component)
			if err != nil {
				return fmt.Errorf("failed to build backend reference for component %s: %w", s.component.GetName(), err)
			}
		}

		route.Spec = gatewayapiv1.HTTPRouteSpec{
			Hostnames: []gatewayapiv1.Hostname{gatewayapiv1.Hostname(s.getHostName())},
			Rules: []gatewayapiv1.HTTPRouteRule{
				{
					BackendRefs: []gatewayapiv1.HTTPBackendRef{backendRef},
					Matches: []gatewayapiv1.HTTPRouteMatch{
						{
							Path: &gatewayapiv1.HTTPPathMatch{Type: new(gatewayapiv1.PathMatchPathPrefix), Value: new("/")},
						},
					},
					Filters: []gatewayapiv1.HTTPRouteFilter{
						{
							Type: gatewayapiv1.HTTPRouteFilterResponseHeaderModifier,
							ResponseHeaderModifier: &gatewayapiv1.HTTPHeaderFilter{
								Add: []gatewayapiv1.HTTPHeader{{Name: "Strict-Transport-Security", Value: "max-age=31536000; includeSubDomains; preload"}},
							},
						},
					},
				},
			},
			CommonRouteSpec: gatewayapiv1.CommonRouteSpec{ParentRefs: parentRefs},
		}

		return controllerutil.SetControllerReference(s.radixDNSAlias, route, s.dynamicClient.Scheme())
	})

	if err != nil {
		return fmt.Errorf("failed to create or update HTTPRoute '%s': %w", route.Name, err)
	}
	if op != controllerutil.OperationResultNone {
		log.Ctx(ctx).Info().Str("httproute", route.Name).Str("op", string(op)).Msg("reconcile HTTPRoute")
	}

	return nil
}

// isGatewayAPIEnabled checks if the gateway API is enabled for the deployment.
// TODO: Remove this when everything is using gateway API, or when it's enforced, and cleanup the code related to this (
func (s *syncer) isGatewayAPIEnabled() bool {
	return annotations.GatewayAPIEnabledForEnvironment(s.rd.Annotations, s.rd.Spec.Environment) ||
		annotations.GatewayAPIEnabledForEnvironment(s.rr.Annotations, s.rd.Spec.Environment)
}

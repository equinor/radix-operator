package deployment

import (
	"context"
	"fmt"
	"os"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/gateway"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubelabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayapixv1alpha1 "sigs.k8s.io/gateway-api/apisx/v1alpha1"
)

type dnsType string

func (dns dnsType) ToIngressLabels() kubelabels.Set {
	switch dns {
	case dnsTypeExternal:
		return kubelabels.Set{kube.RadixExternalAliasLabel: "true"}
	case dnsTypeAppAlias:
		return kubelabels.Set{kube.RadixAppAliasLabel: "true"}
	case dnsTypeClusterName:
		return kubelabels.Set{kube.RadixDefaultAliasLabel: "true"}
	case dnsTypeActiveCluster:
		return kubelabels.Set{kube.RadixActiveClusterAliasLabel: "true"}
	}

	return nil
}

const (
	dnsTypeExternal      dnsType = "external"
	dnsTypeAppAlias      dnsType = "app-alias"
	dnsTypeClusterName   dnsType = "cluster-name"
	dnsTypeActiveCluster dnsType = "active-cluster"
)

type dnsInfo struct {
	fqdn         string
	tlsSecret    string
	dnsType      dnsType
	resourceName string
}

/*
	TODO:
	 - Test Network Policy is using correct pod and namespace selector for Gateway resource
*/

func (deploy *Deployment) reconcileGatewayResources(ctx context.Context, component radixv1.RadixCommonDeployComponent) error {
	ls, err := deploy.reconcileListenerSet(ctx, component)
	if err != nil {
		return fmt.Errorf("failed to reconcile ListenerSet: %w", err)
	}

	if err := deploy.reconcileHTTPRoute(ctx, component, ls); err != nil {
		return fmt.Errorf("failed to reconcile HTTPRoute: %w", err)
	}

	return nil
}

func (deploy *Deployment) reconcileHTTPRoute(ctx context.Context, component radixv1.RadixCommonDeployComponent, parentListenerSet *gatewayapixv1alpha1.XListenerSet) error {
	logger := log.Ctx(ctx)
	var hosts []dnsInfo

	// When everyone is using proxy mode, or its enforced, cleanup this code (https://github.com/equinor/radix-platform/issues/1822)
	oauth2enabled := component.GetAuthentication().GetOAuth2() != nil
	logger.Debug().Msgf("Reconciling ingresses for component %s. OAuth2 enabled: %t", component.GetName(), oauth2enabled)

	if component.IsPublic() {
		hosts = getComponentDNSInfo(ctx, component, *deploy.radixDeployment, *deploy.kubeutil)
	}

	route := &gatewayapiv1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Name: component.GetName(), Namespace: deploy.radixDeployment.Namespace}}
	if len(hosts) == 0 {
		err := deploy.dynamicClient.Delete(ctx, route)
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to delete HTTPRoute %s: %w", route.Name, err)
		}
		if err == nil {
			logger.Info().Str("httproute", route.Name).Str("op", "delete").Msg("reconcile HTTPRoute")
		}

		return nil
	}

	op, err := controllerutil.CreateOrUpdate(ctx, deploy.dynamicClient, route, func() error {
		parentRefs := []gatewayapiv1.ParentReference{{
			Group:       new(gatewayapiv1.Group(gatewayapiv1.GroupName)),
			Kind:        new(gatewayapiv1.Kind("Gateway")),
			Name:        gatewayapiv1.ObjectName(deploy.config.Gateway.Name),
			Namespace:   new(gatewayapiv1.Namespace(deploy.config.Gateway.Namespace)),
			SectionName: new(gatewayapiv1.SectionName(deploy.config.Gateway.SectionName)),
		}}

		if parentListenerSet != nil {
			lsGVK, err := deploy.dynamicClient.GroupVersionKindFor(parentListenerSet)
			if err != nil {
				return fmt.Errorf("failed to get GVK for ListenerSet %s: %w", parentListenerSet.Name, err)
			}

			parentRefs = append(parentRefs, gatewayapiv1.ParentReference{
				Group:     new(gatewayapiv1.Group(lsGVK.Group)),
				Kind:      new(gatewayapiv1.Kind(lsGVK.Kind)),
				Name:      gatewayapiv1.ObjectName(parentListenerSet.Name),
				Namespace: new(gatewayapiv1.Namespace(parentListenerSet.Namespace)),
			})
		}
		route.Labels = kubelabels.Merge(route.Labels, labels.ForComponentGatewayResources(deploy.registration.Name, component))
		if route.Annotations == nil {
			route.Annotations = map[string]string{}
		}

		if deploy.isGatewayAPIEnabled() {
			route.Annotations[annotations.PreviewGatewayModeAnnotation] = "true"
			route.Annotations["external-dns.alpha.kubernetes.io/ttl"] = "30"
		} else {
			delete(route.Annotations, annotations.PreviewGatewayModeAnnotation)
			delete(route.Annotations, "external-dns.alpha.kubernetes.io/ttl")
		}

		var backendRef gatewayapiv1.HTTPBackendRef
		if oauth2enabled {
			backendRef = gateway.BuildBackendRefForComponentOauth2Service(component)
		} else {
			var err error
			backendRef, err = gateway.BuildBackendRefForComponent(component)
			if err != nil {
				return fmt.Errorf("failed to build backend reference for component %s: %w", component.GetName(), err)
			}
		}

		route.Spec = gatewayapiv1.HTTPRouteSpec{
			Hostnames: slice.Map(hosts, func(host dnsInfo) gatewayapiv1.Hostname { return gatewayapiv1.Hostname(host.fqdn) }),
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

		route.ObjectMeta.OwnerReferences = []metav1.OwnerReference{}
		return controllerutil.SetControllerReference(deploy.radixDeployment, route, deploy.dynamicClient.Scheme())
	})

	if err != nil {
		return fmt.Errorf("failed to create or update HTTPRoute '%s': %w", route.Name, err)
	}
	if op != controllerutil.OperationResultNone {
		log.Ctx(ctx).Info().Str("httproute", route.Name).Str("op", string(op)).Msg("reconcile HTTPRoute")
	}

	return nil
}

func (deploy *Deployment) reconcileListenerSet(ctx context.Context, component radixv1.RadixCommonDeployComponent) (*gatewayapixv1alpha1.XListenerSet, error) {
	logger := log.Ctx(ctx)
	var hosts []dnsInfo

	if component.IsPublic() {
		hosts = getComponentDNSInfo(ctx, component, *deploy.radixDeployment, *deploy.kubeutil)
	}

	externalDNSHosts := slice.FindAll(hosts, func(h dnsInfo) bool { return h.dnsType == dnsTypeExternal })
	ls := &gatewayapixv1alpha1.XListenerSet{ObjectMeta: metav1.ObjectMeta{Name: component.GetName(), Namespace: deploy.radixDeployment.Namespace}}

	if len(externalDNSHosts) == 0 {
		err := deploy.dynamicClient.Delete(ctx, ls)
		if client.IgnoreNotFound(err) != nil {
			return nil, fmt.Errorf("failed to delete ListenerSet %s: %w", ls.Name, err)
		}
		if err == nil {
			logger.Info().Str("xlisternerset", ls.Name).Str("op", "delete").Msg("reconcile XListenerSet")
		}

		return nil, nil
	}

	op, err := controllerutil.CreateOrUpdate(ctx, deploy.dynamicClient, ls, func() error {
		var listeners []gatewayapixv1alpha1.ListenerEntry
		for _, host := range externalDNSHosts {
			listeners = append(listeners, gatewayapixv1alpha1.ListenerEntry{
				Name:     gatewayapixv1alpha1.SectionName(host.fqdn),
				Hostname: new(gatewayapixv1alpha1.Hostname(host.fqdn)),
				Protocol: gatewayapiv1.HTTPSProtocolType,
				Port:     443,
				AllowedRoutes: &gatewayapiv1.AllowedRoutes{
					Namespaces: &gatewayapiv1.RouteNamespaces{From: new(gatewayapiv1.NamespacesFromSame)},
				},
				TLS: &gatewayapiv1.ListenerTLSConfig{
					Mode: new(gatewayapiv1.TLSModeTerminate),
					CertificateRefs: []gatewayapiv1.SecretObjectReference{
						{
							Group: new(gatewayapiv1.Group("")),
							Kind:  new(gatewayapiv1.Kind("Secret")),
							Name:  gatewayapiv1.ObjectName(host.tlsSecret),
						},
					},
				},
			})
		}

		ls.Labels = kubelabels.Merge(ls.Labels, labels.ForComponentGatewayResources(deploy.registration.Name, component))
		ls.Spec = gatewayapixv1alpha1.ListenerSetSpec{
			ParentRef: gatewayapixv1alpha1.ParentGatewayReference{
				Group:     new(gatewayapiv1.Group(gatewayapiv1.GroupName)),
				Kind:      new(gatewayapiv1.Kind("Gateway")),
				Name:      gatewayapiv1.ObjectName(deploy.config.Gateway.Name),
				Namespace: new(gatewayapiv1.Namespace(deploy.config.Gateway.Namespace)),
			},
			Listeners: listeners,
		}

		ls.ObjectMeta.OwnerReferences = []metav1.OwnerReference{}
		return controllerutil.SetControllerReference(deploy.radixDeployment, ls, deploy.dynamicClient.Scheme())
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create or update XListenerSet '%s': %w", ls.Name, err)
	}

	if op != controllerutil.OperationResultNone {
		logger.Info().Str("xlisternerset", ls.Name).Str("op", string(op)).Msg("reconcile XListenerSet")
	}

	return ls, nil
}

func getComponentDNSInfo(ctx context.Context, component radixv1.RadixCommonDeployComponent, rd radixv1.RadixDeployment, kubeutil kube.Kube) []dnsInfo {
	var info []dnsInfo

	if component.IsDNSAppAlias() {
		appAlias := os.Getenv(defaults.OperatorAppAliasBaseURLEnvironmentVariable) // .app.dev.radix.equinor.com in launch.json
		if appAlias != "" {
			info = append(info, dnsInfo{
				fqdn:         fmt.Sprintf("%s.%s", rd.Spec.AppName, appAlias),
				tlsSecret:    "",
				dnsType:      dnsTypeAppAlias,
				resourceName: getAppAliasIngressName(rd.Spec.AppName),
			})
		}
	}

	for _, externalDns := range component.GetExternalDNS() {
		info = append(info, dnsInfo{
			fqdn:         externalDns.FQDN,
			tlsSecret:    utils.GetExternalDnsTlsSecretName(externalDns),
			dnsType:      dnsTypeExternal,
			resourceName: externalDns.FQDN,
		})
	}

	if hostname := getActiveClusterHostName(component.GetName(), rd.Namespace); hostname != "" {
		info = append(info, dnsInfo{
			fqdn:         hostname,
			tlsSecret:    "",
			dnsType:      dnsTypeActiveCluster,
			resourceName: getActiveClusterIngressName(component.GetName()),
		})
	}

	if clustername, err := kubeutil.GetClusterName(ctx); err != nil {
		log.Ctx(ctx).Warn().Err(err).Msg("failed to read cluster name")
	} else {
		if hostname := getHostName(component.GetName(), rd.Namespace, clustername); hostname != "" {
			info = append(info, dnsInfo{
				fqdn:         hostname,
				tlsSecret:    "",
				dnsType:      dnsTypeClusterName,
				resourceName: getDefaultIngressName(component.GetName()),
			})
		}
	}

	return info
}

func (deploy *Deployment) garbageCollectGatewayResourcesNoLongerInSpec(ctx context.Context) error {
	if err := deploy.garbageCollectHTTPRoutesNoLongerInSpec(ctx); err != nil {
		return fmt.Errorf("failed to garbage collect HTTPRoutes: %w", err)
	}

	if err := deploy.garbageCollectListenerSetsNoLongerInSpec(ctx); err != nil {
		return fmt.Errorf("failed to garbage collect ListenerSets: %w", err)
	}

	return nil
}

func (deploy *Deployment) garbageCollectHTTPRoutesNoLongerInSpec(ctx context.Context) error {
	routes := &gatewayapiv1.HTTPRouteList{}
	//TODO: Do not list HTTPRoutes owned by DNSAlias
	if err := deploy.dynamicClient.List(ctx, routes, client.InNamespace(deploy.radixDeployment.Namespace), client.MatchingLabels(labels.ForApplicationName(deploy.registration.Name))); err != nil {
		return fmt.Errorf("failed to list HTTPRoutes: %w", err)
	}

	for _, route := range routes.Items {
		// skip if route owner is not RadixDeployment
		if c := metav1.GetControllerOf(&route); c == nil || c.Kind != radixv1.KindRadixDeployment {
			continue
		}

		componentName, ok := RadixComponentNameFromComponentLabel(&route)
		if !ok {
			continue
		}

		// HTTPRoutes should only exist for items in component list.
		if !componentName.ExistInDeploymentSpecComponentList(deploy.radixDeployment) {
			err := deploy.dynamicClient.Delete(ctx, &route)
			if client.IgnoreNotFound(err) != nil {
				return fmt.Errorf("failed to delete HTTPRoute %s: %w", route.Name, err)
			}
			if err == nil {
				log.Ctx(ctx).Info().Str("httproute", route.Name).Str("op", "delete").Msg("garbage collect HTTPRoute no longer in spec")
			}
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectListenerSetsNoLongerInSpec(ctx context.Context) error {
	lsList := &gatewayapixv1alpha1.XListenerSetList{}
	if err := deploy.dynamicClient.List(ctx, lsList, client.InNamespace(deploy.radixDeployment.Namespace), client.MatchingLabels(labels.ForApplicationName(deploy.registration.Name))); err != nil {
		return fmt.Errorf("failed to list XListenerSets: %w", err)
	}

	for _, ls := range lsList.Items {
		componentName, ok := RadixComponentNameFromComponentLabel(&ls)
		if !ok {
			continue
		}

		// ListenerSets should only exist for items in component list.
		if !componentName.ExistInDeploymentSpecComponentList(deploy.radixDeployment) {
			err := deploy.dynamicClient.Delete(ctx, &ls)
			if client.IgnoreNotFound(err) != nil {
				return fmt.Errorf("failed to delete XListenerSet %s: %w", ls.Name, err)
			}
			if err == nil {
				log.Ctx(ctx).Info().Str("xlisternerset", ls.Name).Str("op", "delete").Msg("garbage collect XListenerSet no longer in spec")
			}
		}
	}

	return nil
}

// isGatewayAPIEnabled checks if the gateway API is enabled for the deployment.
// TODO: Remove this when everything is using gateway API, or when it's enforced, and cleanup the code related to this (
func (deploy *Deployment) isGatewayAPIEnabled() bool {
	return annotations.GatewayAPIEnabledForEnvironment(deploy.radixDeployment.Annotations, deploy.radixDeployment.Spec.Environment) ||
		annotations.GatewayAPIEnabledForEnvironment(deploy.registration.Annotations, deploy.radixDeployment.Spec.Environment)
}

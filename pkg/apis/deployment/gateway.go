package deployment

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/gateway"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/rs/zerolog/log"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubelabels "k8s.io/apimachinery/pkg/labels"
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

func (deploy *Deployment) reconcileGatewayResources(ctx context.Context, component radixv1.RadixCommonDeployComponent) error {
	logger := log.Ctx(ctx)
	var hosts []dnsInfo

	// When everyone is using proxy mode, or its enforced, cleanup this code (https://github.com/equinor/radix-platform/issues/1822)
	oauth2enabled := component.GetAuthentication().GetOAuth2() != nil
	logger.Debug().Msgf("Reconciling ingresses for component %s. OAuth2 enabled: %t", component.GetName(), oauth2enabled)

	if component.IsPublic() {
		hosts = getComponentDNSInfo(ctx, component, *deploy.radixDeployment, *deploy.kubeutil)
	}

	if len(hosts) == 0 {
		// Garbage collect HTTP routes and listener sets
		return errors.New("Not implemented")
	}

	namespace := deploy.radixDeployment.Namespace
	componentName := component.GetName()
	route := &gatewayapiv1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Name: componentName, Namespace: namespace}}
	op, err := controllerutil.CreateOrUpdate(ctx, deploy.dynamicClient, route, func() error {
		route.Labels = kubelabels.Merge(route.Labels, labels.ForComponentHTTPRoute(deploy.registration.Name, component))

		var backendRef gatewayapiv1.HTTPBackendRef
		if oauth2enabled {
			backendRef = gateway.BuildBackendRefForComponentOauth2Service(component)
		} else {
			var err error
			backendRef, err = gateway.BuildBackendRefForComponent(component)
			if err != nil {
				return fmt.Errorf("failed to build backend reference for component %s: %w", componentName, err)
			}
		}

		parentRefs := []gatewayapiv1.ParentReference{
			{
				Group: new(gatewayapiv1.Group(gatewayapiv1.GroupName)),
				Kind:  new(gatewayapiv1.Kind("Gateway")),

				// TODO: Make this configurable
				Name:        "gateway",
				Namespace:   new(gatewayapiv1.Namespace("istio-system")),
				SectionName: new(gatewayapiv1.SectionName("https")),
			},
		}

		if slices.ContainsFunc(hosts, func(h dnsInfo) bool { return h.dnsType == dnsTypeExternal }) {
			parentRefs = append(parentRefs, gatewayapiv1.ParentReference{
				Group:     new(gatewayapiv1.Group(gatewayapixv1alpha1.GroupName)),
				Kind:      new(gatewayapiv1.Kind("XListenerSet")),
				Name:      gatewayapiv1.ObjectName(component.GetName()),
				Namespace: new(gatewayapiv1.Namespace(namespace)),
			})
		}

		route.Spec = gatewayapiv1.HTTPRouteSpec{
			Hostnames: slice.Map(hosts, func(host dnsInfo) gatewayapiv1.Hostname { return gatewayapiv1.Hostname(host.fqdn) }),
			Rules: []gatewayapiv1.HTTPRouteRule{
				{
					BackendRefs: []gatewayapiv1.HTTPBackendRef{backendRef},
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
		return fmt.Errorf("failed to create or update service monitor '%s': %w", componentName, err)
	}
	if op != controllerutil.OperationResultNone {
		log.Ctx(ctx).Info().Str("httproute", componentName).Str("op", string(op)).Msg("reconcile HTTP Route")
	}

	return nil
}

func (deploy *Deployment) garbageCollectGatewayResources(ctx context.Context, component radixv1.RadixCommonDeployComponent, hosts []dnsInfo) error {
	logger := log.Ctx(ctx)

	selector := fmt.Sprintf("%s,!%s", labels.ForComponentIngress(deploy.radixDeployment.Spec.AppName, component).AsSelector().String(), kube.RadixAliasLabel)
	existingIngresses, err := deploy.kubeclient.NetworkingV1().Ingresses(deploy.radixDeployment.GetNamespace()).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return fmt.Errorf("failed to list ingresses in garbageCollectIngresses: %w", err)
	}
	logger.Debug().Msgf("Garbage collecting ingresses for component %s. Found %d existing ingresses", component.GetName(), len(existingIngresses.Items))

	for _, ing := range existingIngresses.Items {
		// should exist in list of hosts
		found := false
		for _, host := range hosts {
			if len(ing.Spec.Rules) > 0 && ing.Spec.Rules[0].Host == host.fqdn {
				found = true
				break
			}
		}

		if !found {
			logger.Info().Msgf("Garbage collecting ingress %s for component %s", ing.Name, component.GetName())
			if err := deploy.kubeclient.NetworkingV1().Ingresses(deploy.radixDeployment.GetNamespace()).Delete(ctx, ing.Name, metav1.DeleteOptions{}); err != nil {
				return fmt.Errorf("failed to delete ingress %s: %w", ing.Name, err)
			}
		}
	}

	return nil
}

func (deploy *Deployment) createOrUpdateGatewayResources(ctx context.Context, component radixv1.RadixCommonDeployComponent, hosts []dnsInfo) error {
	if len(hosts) == 0 {
		return nil
	}

	owner := []metav1.OwnerReference{getOwnerReferenceOfDeployment(deploy.radixDeployment)}

	for _, host := range hosts {
		ingressSpec := ingress.BuildIngressSpecForComponent(component, host.fqdn, host.tlsSecret)
		annotations, err := ingress.BuildAnnotationsFromProviders(component, deploy.ingressAnnotationProviders)
		if err != nil {
			return fmt.Errorf("failed to build annotations: %w", err)
		}
		ing := &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:        host.resourceName,
				Annotations: annotations,
				Labels: labels.Merge(
					labels.ForComponentIngress(deploy.radixDeployment.Spec.AppName, component),
					host.dnsType.ToIngressLabels(),
				),
				OwnerReferences: owner,
			},
			Spec: ingressSpec,
		}

		if err := deploy.kubeutil.ApplyIngress(ctx, deploy.radixDeployment.Namespace, ing); err != nil {
			return fmt.Errorf("failed to reconcile ingress: %w", err)
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectGatewayResourcesNoLongerInSpec(ctx context.Context) error {
	ingresses, err := deploy.kubeutil.ListIngresses(ctx, deploy.radixDeployment.Namespace)
	if err != nil {
		return err
	}

	for _, ing := range ingresses {
		componentName, ok := RadixComponentNameFromComponentLabel(ing)
		if !ok {
			continue
		}

		// Ingresses should only exist for items in component list.
		if !componentName.ExistInDeploymentSpecComponentList(deploy.radixDeployment) {
			err = deploy.kubeclient.NetworkingV1().Ingresses(deploy.radixDeployment.GetNamespace()).Delete(ctx, ing.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func getAppAliasIngressName(appName string) string {
	return fmt.Sprintf("%s-url-alias", appName)
}

func getActiveClusterIngressName(componentName string) string {
	return fmt.Sprintf("%s-active-cluster-url-alias", componentName)
}

func getDefaultIngressName(componentName string) string {
	return componentName
}

func getActiveClusterHostName(componentName, namespace string) string {
	dnsZone := os.Getenv(defaults.OperatorDNSZoneEnvironmentVariable)
	if dnsZone == "" {
		return ""
	}
	return fmt.Sprintf("%s-%s.%s", componentName, namespace, dnsZone)
}

func getHostName(componentName, namespace, clustername string) string {
	dnsZone := os.Getenv(defaults.OperatorDNSZoneEnvironmentVariable)
	if dnsZone == "" {
		return ""
	}
	hostnameTemplate := "%s-%s.%s.%s"
	return fmt.Sprintf(hostnameTemplate, componentName, namespace, clustername, dnsZone)
}

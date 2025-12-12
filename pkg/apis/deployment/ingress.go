package deployment

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type dnsType string

func (dns dnsType) ToIngressLabels() labels.Set {
	switch dns {
	case dnsTypeExternal:
		return labels.Set{kube.RadixExternalAliasLabel: "true"}
	case dnsTypeAppAlias:
		return labels.Set{kube.RadixAppAliasLabel: "true"}
	case dnsTypeClusterName:
		return labels.Set{kube.RadixDefaultAliasLabel: "true"}
	case dnsTypeActiveCluster:
		return labels.Set{kube.RadixActiveClusterAliasLabel: "true"}
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

func getComponentDNSInfo(ctx context.Context, c radixv1.RadixCommonDeployComponent, rd radixv1.RadixDeployment, kubeutil kube.Kube) []dnsInfo {
	var info []dnsInfo

	if c.IsDNSAppAlias() {
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

	for _, externalDns := range c.GetExternalDNS() {
		info = append(info, dnsInfo{
			fqdn:         externalDns.FQDN,
			tlsSecret:    utils.GetExternalDnsTlsSecretName(externalDns),
			dnsType:      dnsTypeExternal,
			resourceName: externalDns.FQDN,
		})
	}

	if hostname := getActiveClusterHostName(c.GetName(), rd.Namespace); hostname != "" {
		info = append(info, dnsInfo{
			fqdn:         hostname,
			tlsSecret:    "",
			dnsType:      dnsTypeActiveCluster,
			resourceName: getActiveClusterIngressName(c.GetName()),
		})
	}

	if clustername, err := kubeutil.GetClusterName(ctx); err != nil {
		log.Ctx(ctx).Warn().Err(err).Msg("failed to read cluster name")
	} else {
		if hostname := getHostName(c.GetName(), rd.Namespace, clustername); hostname != "" {
			info = append(info, dnsInfo{
				fqdn:         hostname,
				tlsSecret:    "",
				dnsType:      dnsTypeClusterName,
				resourceName: getDefaultIngressName(c.GetName()),
			})
		}
	}

	return info
}

func (deploy *Deployment) reconcileIngresses(ctx context.Context, c radixv1.RadixCommonDeployComponent) error {
	logger := log.Ctx(ctx)
	var hosts []dnsInfo

	// When everyone is using proxy mode, or its enforced, cleanup this code (https://github.com/equinor/radix-platform/issues/1822)
	oauth2enabled := c.GetAuthentication().GetOAuth2() != nil
	hasProxyModeAnnotation := annotations.Oauth2PreviewModeEnabledForEnvironment(deploy.radixDeployment.Annotations, deploy.radixDeployment.Spec.Environment)
	oauth2PreviewProxyEnabled := oauth2enabled && hasProxyModeAnnotation
	logger.Debug().Msgf("Reconciling ingresses for component %s. OAuth2 enabled: %t, Proxy mode enabled: %t", c.GetName(), oauth2enabled, oauth2PreviewProxyEnabled)

	if c.IsPublic() && !oauth2PreviewProxyEnabled {
		hosts = getComponentDNSInfo(ctx, c, *deploy.radixDeployment, *deploy.kubeutil)
	}

	if err := deploy.garbageCollectIngresses(ctx, c, hosts); err != nil {
		return fmt.Errorf("failed to garbage collect ingresses: %w", err)
	}

	if err := deploy.createOrUpdateIngress(ctx, c, hosts); err != nil {
		return fmt.Errorf("failed to create ingress: %w", err)
	}

	// Garbage collect oauth2 proxy ingress
	// Garbage collect regular ingress

	// if public && oauth, create/update oauth2 proxy ingress
	// if (public && !oauth2 proxy mode) component, create/update ingress

	return nil
}

func (deploy *Deployment) garbageCollectIngresses(ctx context.Context, c radixv1.RadixCommonDeployComponent, hosts []dnsInfo) error {
	logger := log.Ctx(ctx)
	selector := fmt.Sprintf("%s=%s, !%s", kube.RadixComponentLabel, c.GetName(), kube.RadixAliasLabel)
	existingIngresses, err := deploy.kubeclient.NetworkingV1().Ingresses(deploy.radixDeployment.GetNamespace()).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return fmt.Errorf("failed to list ingresses in garbageCollectIngresses: %w", err)
	}
	logger.Debug().Msgf("Garbage collecting ingresses for component %s. Found %d existing ingresses", c.GetName(), len(existingIngresses.Items))

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
			logger.Info().Msgf("Garbage collecting ingress %s for component %s", ing.Name, c.GetName())
			if err := deploy.kubeclient.NetworkingV1().Ingresses(deploy.radixDeployment.GetNamespace()).Delete(ctx, ing.Name, metav1.DeleteOptions{}); err != nil {
				return fmt.Errorf("failed to delete ingress %s: %w", ing.Name, err)
			}
		}
	}

	return nil
}

func (deploy *Deployment) createOrUpdateIngress(ctx context.Context, c radixv1.RadixCommonDeployComponent, hosts []dnsInfo) error {
	if len(hosts) == 0 {
		return nil
	}

	publicPortNumber := getPublicPortForComponent(c)
	owner := []metav1.OwnerReference{getOwnerReferenceOfDeployment(deploy.radixDeployment)}

	for _, host := range hosts {
		tlsSecret := host.tlsSecret
		if tlsSecret == "" {
			tlsSecret = defaults.TLSSecretName
		}

		ingressSpec := ingress.GetIngressSpec(host.fqdn, c.GetName(), tlsSecret, publicPortNumber, "/")
		ingressConfig, err := ingress.GetIngressConfig(deploy.radixDeployment.Namespace, deploy.radixDeployment.Spec.AppName, c, host.resourceName, ingressSpec, deploy.ingressAnnotationProviders, owner)
		if err != nil {
			return fmt.Errorf("failed to create ingress config: %w", err)
		}
		ingressConfig.Labels = labels.Merge(ingressConfig.Labels, host.dnsType.ToIngressLabels())

		if err := deploy.kubeutil.ApplyIngress(ctx, deploy.radixDeployment.Namespace, ingressConfig); err != nil {
			return fmt.Errorf("failed to reconcile ingress: %w", err)
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectIngressesNoLongerInSpec(ctx context.Context) error {
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

func getPublicPortForComponent(deployComponent radixv1.RadixCommonDeployComponent) int32 {
	if deployComponent.GetPublicPort() == "" {
		// For backwards compatibility
		return deployComponent.GetPorts()[0].Port
	} else {
		if port, ok := slice.FindFirst(deployComponent.GetPorts(), func(cp radixv1.ComponentPort) bool {
			return strings.EqualFold(cp.Name, deployComponent.GetPublicPort())
		}); ok {
			return port.Port
		}
	}

	return 0
}

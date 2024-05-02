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
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (deploy *Deployment) createOrUpdateIngress(ctx context.Context, deployComponent radixv1.RadixCommonDeployComponent) error {
	if err := deploy.createOrUpdateAppAliasIngress(ctx, deployComponent); err != nil {
		return err
	}

	if err := deploy.createOrUpdateExternalDNSIngresses(ctx, deployComponent); err != nil {
		return err
	}

	if err := deploy.createOrUpdateActiveClusterIngress(ctx, deployComponent); err != nil {
		return err
	}

	return deploy.createOrUpdateClusterIngress(ctx, deployComponent)
}

func (deploy *Deployment) createOrUpdateClusterIngress(ctx context.Context, deployComponent radixv1.RadixCommonDeployComponent) error {
	clustername, err := deploy.kubeutil.GetClusterName()
	if err != nil {
		return err
	}

	namespace := deploy.radixDeployment.Namespace
	publicPortNumber := getPublicPortForComponent(deployComponent)

	ing, err := deploy.getDefaultIngressConfig(deploy.radixDeployment.Spec.AppName, []metav1.OwnerReference{getOwnerReferenceOfDeployment(deploy.radixDeployment)}, deployComponent, clustername, namespace, publicPortNumber)
	if err != nil {
		return err
	}

	return deploy.kubeutil.ApplyIngress(ctx, namespace, ing)
}

func (deploy *Deployment) createOrUpdateActiveClusterIngress(ctx context.Context, deployComponent radixv1.RadixCommonDeployComponent) error {
	clustername, err := deploy.kubeutil.GetClusterName()
	if err != nil {
		return err
	}

	if !isActiveCluster(clustername) {
		return deploy.garbageCollectNonActiveClusterIngress(ctx, deployComponent)
	}

	namespace := deploy.radixDeployment.Namespace
	publicPortNumber := getPublicPortForComponent(deployComponent)

	// Create fixed active cluster ingress for this component
	activeClusterAliasIngress, err := deploy.getActiveClusterAliasIngressConfig(deploy.radixDeployment.Spec.AppName, []metav1.OwnerReference{getOwnerReferenceOfDeployment(deploy.radixDeployment)}, deployComponent, namespace, publicPortNumber)
	if err != nil {
		return err
	}

	return deploy.kubeutil.ApplyIngress(ctx, namespace, activeClusterAliasIngress)
}

func (deploy *Deployment) createOrUpdateAppAliasIngress(ctx context.Context, deployComponent radixv1.RadixCommonDeployComponent) error {
	clustername, err := deploy.kubeutil.GetClusterName()
	if err != nil {
		return err
	}

	if !deployComponent.IsDNSAppAlias() || !isActiveCluster(clustername) {
		return deploy.garbageCollectAppAliasIngressNoLongerInSpecForComponent(ctx, deployComponent)
	}

	namespace := deploy.radixDeployment.Namespace
	publicPortNumber := getPublicPortForComponent(deployComponent)

	appAliasIngress, err := deploy.getAppAliasIngressConfig(deploy.radixDeployment.Spec.AppName, []metav1.OwnerReference{getOwnerReferenceOfDeployment(deploy.radixDeployment)}, deployComponent, namespace, publicPortNumber)
	if err != nil {
		return err
	}

	return deploy.kubeutil.ApplyIngress(ctx, namespace, appAliasIngress)
}

func (deploy *Deployment) createOrUpdateExternalDNSIngresses(ctx context.Context, deployComponent radixv1.RadixCommonDeployComponent) error {
	clustername, err := deploy.kubeutil.GetClusterName()
	if err != nil {
		return err
	}

	externalDNSList := deployComponent.GetExternalDNS()

	if len(externalDNSList) == 0 || !isActiveCluster(clustername) {
		return deploy.garbageCollectAllExternalAliasIngressesForComponent(ctx, deployComponent)
	}

	namespace := deploy.radixDeployment.Namespace
	publicPortNumber := getPublicPortForComponent(deployComponent)

	if err := deploy.garbageCollectIngressNoLongerInSpecForComponentAndExternalAlias(ctx, deployComponent); err != nil {
		return err
	}

	for _, externalDNS := range externalDNSList {
		ingress, err := deploy.getExternalAliasIngressConfig(deploy.radixDeployment.Spec.AppName, []metav1.OwnerReference{getOwnerReferenceOfDeployment(deploy.radixDeployment)}, externalDNS, deployComponent, namespace, publicPortNumber)
		if err != nil {
			return err
		}

		err = deploy.kubeutil.ApplyIngress(ctx, namespace, ingress)
		if err != nil {
			return err
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectIngressesNoLongerInSpec(ctx context.Context) error {
	ingresses, err := deploy.kubeutil.ListIngresses(deploy.radixDeployment.Namespace)
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

func (deploy *Deployment) garbageCollectAppAliasIngressNoLongerInSpecForComponent(ctx context.Context, component radixv1.RadixCommonDeployComponent) error {
	return deploy.garbageCollectIngressByLabelSelectorForComponent(ctx, fmt.Sprintf("%s=%s, %s=%s", kube.RadixComponentLabel, component.GetName(), kube.RadixAppAliasLabel, "true"))
}

func (deploy *Deployment) garbageCollectIngressNoLongerInSpecForComponent(ctx context.Context, component radixv1.RadixCommonDeployComponent) error {
	return deploy.garbageCollectIngressByLabelSelectorForComponent(ctx, getLabelSelectorForComponent(component))
}

func (deploy *Deployment) garbageCollectNonActiveClusterIngress(ctx context.Context, component radixv1.RadixCommonDeployComponent) error {
	return deploy.garbageCollectIngressByLabelSelectorForComponent(ctx, fmt.Sprintf("%s=%s, %s=%s", kube.RadixComponentLabel, component.GetName(), kube.RadixActiveClusterAliasLabel, "true"))
}

func (deploy *Deployment) garbageCollectIngressByLabelSelectorForComponent(ctx context.Context, labelSelector string) error {
	ingresses, err := deploy.kubeutil.ListIngressesWithSelector(deploy.radixDeployment.GetNamespace(), labelSelector)
	if err != nil {
		return err
	}

	if len(ingresses) > 0 {
		for n := range ingresses {
			err = deploy.kubeclient.NetworkingV1().Ingresses(deploy.radixDeployment.GetNamespace()).Delete(ctx, ingresses[n].Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectAllExternalAliasIngressesForComponent(ctx context.Context, component radixv1.RadixCommonDeployComponent) error {
	return deploy.garbageCollectIngressForComponentAndExternalAlias(ctx, component, true)
}

func (deploy *Deployment) garbageCollectIngressNoLongerInSpecForComponentAndExternalAlias(ctx context.Context, component radixv1.RadixCommonDeployComponent) error {
	return deploy.garbageCollectIngressForComponentAndExternalAlias(ctx, component, false)
}

func (deploy *Deployment) garbageCollectIngressForComponentAndExternalAlias(ctx context.Context, component radixv1.RadixCommonDeployComponent, all bool) error {
	labelSelector := getLabelSelectorForExternalAliasIngress(component)
	ingresses, err := deploy.kubeutil.ListIngressesWithSelector(deploy.radixDeployment.GetNamespace(), labelSelector)
	if err != nil {
		return err
	}

	for _, ingress := range ingresses {
		garbageCollectIngress := true

		if !all {
			externalAliasForIngress := ingress.Name
			for _, externalAlias := range component.GetExternalDNS() {
				if externalAlias.FQDN == externalAliasForIngress {
					garbageCollectIngress = false
				}
			}
		}

		if garbageCollectIngress {
			err = deploy.kubeclient.NetworkingV1().Ingresses(deploy.radixDeployment.GetNamespace()).Delete(ctx, ingress.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func getLabelSelectorForExternalAliasIngress(component radixv1.RadixCommonDeployComponent) string {
	return fmt.Sprintf("%s=%s, %s=%s", kube.RadixComponentLabel, component.GetName(), kube.RadixExternalAliasLabel, "true")
}

func (deploy *Deployment) getAppAliasIngressConfig(appName string, ownerReference []metav1.OwnerReference, component radixv1.RadixCommonDeployComponent, namespace string, publicPortNumber int32) (*networkingv1.Ingress, error) {
	appAlias := os.Getenv(defaults.OperatorAppAliasBaseURLEnvironmentVariable) // .app.dev.radix.equinor.com in launch.json
	if appAlias == "" {
		return nil, nil
	}

	hostname := fmt.Sprintf("%s.%s", appName, appAlias)
	ingressSpec := ingress.GetIngressSpec(hostname, component.GetName(), defaults.TLSSecretName, publicPortNumber)

	ingressConfig, err := ingress.GetIngressConfig(namespace, appName, component, getAppAliasIngressName(appName), ingressSpec, deploy.ingressAnnotationProviders, ownerReference)
	if err != nil {
		return nil, err
	}
	ingressConfig.ObjectMeta.Labels[kube.RadixAppAliasLabel] = "true"
	return ingressConfig, err
}

func (deploy *Deployment) getActiveClusterAliasIngressConfig(
	appName string,
	ownerReference []metav1.OwnerReference,
	component radixv1.RadixCommonDeployComponent,
	namespace string,
	publicPortNumber int32,
) (*networkingv1.Ingress, error) {
	hostname := getActiveClusterHostName(component.GetName(), namespace)
	if hostname == "" {
		return nil, nil
	}
	ingressSpec := ingress.GetIngressSpec(hostname, component.GetName(), defaults.TLSSecretName, publicPortNumber)
	ingressName := getActiveClusterIngressName(component.GetName())

	ingressConfig, err := ingress.GetIngressConfig(namespace, appName, component, ingressName, ingressSpec, deploy.ingressAnnotationProviders, ownerReference)
	if err != nil {
		return nil, err
	}
	ingressConfig.ObjectMeta.Labels[kube.RadixActiveClusterAliasLabel] = "true"
	return ingressConfig, err
}

func (deploy *Deployment) getDefaultIngressConfig(
	appName string,
	ownerReference []metav1.OwnerReference,
	component radixv1.RadixCommonDeployComponent,
	clustername, namespace string,
	publicPortNumber int32,
) (*networkingv1.Ingress, error) {
	dnsZone := os.Getenv(defaults.OperatorDNSZoneEnvironmentVariable)
	if dnsZone == "" {
		return nil, nil
	}
	hostname := getHostName(component.GetName(), namespace, clustername, dnsZone)
	ingressSpec := ingress.GetIngressSpec(hostname, component.GetName(), defaults.TLSSecretName, publicPortNumber)

	ingressConfig, err := ingress.GetIngressConfig(namespace, appName, component, getDefaultIngressName(component.GetName()), ingressSpec, deploy.ingressAnnotationProviders, ownerReference)
	if err != nil {
		return nil, err
	}
	ingressConfig.ObjectMeta.Labels[kube.RadixDefaultAliasLabel] = "true"
	return ingressConfig, err
}

func (deploy *Deployment) getExternalAliasIngressConfig(
	appName string,
	ownerReference []metav1.OwnerReference,
	externalAlias radixv1.RadixDeployExternalDNS,
	component radixv1.RadixCommonDeployComponent,
	namespace string,
	publicPortNumber int32,
) (*networkingv1.Ingress, error) {
	ingressSpec := ingress.GetIngressSpec(externalAlias.FQDN, component.GetName(), utils.GetExternalDnsTlsSecretName(externalAlias), publicPortNumber)
	ingressConfig, err := ingress.GetIngressConfig(namespace, appName, component, externalAlias.FQDN, ingressSpec, deploy.ingressAnnotationProviders, ownerReference)
	if err != nil {
		return nil, err
	}
	ingressConfig.ObjectMeta.Labels[kube.RadixExternalAliasLabel] = "true"
	return ingressConfig, err
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

func getHostName(componentName, namespace, clustername, dnsZone string) string {
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

func isActiveCluster(clustername string) bool {
	activeClustername := os.Getenv(defaults.ActiveClusternameEnvironmentVariable)
	return strings.EqualFold(clustername, activeClustername)
}

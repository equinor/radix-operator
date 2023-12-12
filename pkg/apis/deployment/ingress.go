package deployment

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (deploy *Deployment) createOrUpdateIngress(deployComponent radixv1.RadixCommonDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace
	clustername, err := deploy.kubeutil.GetClusterName()
	if err != nil {
		return err
	}

	var publicPortNumber int32
	// For backwards compatibility
	if deployComponent.GetPublicPort() == "" {
		publicPortNumber = deployComponent.GetPorts()[0].Port
	} else {
		publicPortNumber = getPublicPortNumber(deployComponent.GetPorts(), deployComponent.GetPublicPort())
	}

	ownerReference := []metav1.OwnerReference{
		getOwnerReferenceOfDeployment(deploy.radixDeployment),
	}

	// Only the active cluster should have the DNS alias, not to cause conflicts between clusters
	if deployComponent.IsDNSAppAlias() && isActiveCluster(clustername) {
		appAliasIngress, err := deploy.getAppAliasIngressConfig(deploy.radixDeployment.Spec.AppName, ownerReference, deployComponent, namespace, publicPortNumber)
		if err != nil {
			return err
		}

		if appAliasIngress != nil {
			err = deploy.kubeutil.ApplyIngress(namespace, appAliasIngress)
			if err != nil {
				return err
			}
		}
	} else {
		err := deploy.garbageCollectAppAliasIngressNoLongerInSpecForComponent(deployComponent)
		if err != nil {
			return err
		}
	}

	// Only the active cluster should have the DNS external alias, not to cause conflicts between clusters
	dnsExternalAlias := deployComponent.GetDNSExternalAlias()
	if len(dnsExternalAlias) > 0 && isActiveCluster(clustername) {
		err = deploy.garbageCollectIngressNoLongerInSpecForComponentAndExternalAlias(deployComponent)
		if err != nil {
			return err
		}

		for _, externalAlias := range dnsExternalAlias {
			externalAliasIngress, err := deploy.getExternalAliasIngressConfig(deploy.radixDeployment.Spec.AppName,
				ownerReference, externalAlias, deployComponent, namespace, publicPortNumber)
			if err != nil {
				return err
			}

			err = deploy.kubeutil.ApplyIngress(namespace, externalAliasIngress)
			if err != nil {
				return err
			}
		}
	} else {
		err = deploy.garbageCollectAllExternalAliasIngressesForComponent(deployComponent)
		if err != nil {
			return err
		}
	}

	if isActiveCluster(clustername) {
		// Create fixed active cluster ingress for this component
		activeClusterAliasIngress, err := deploy.getActiveClusterAliasIngressConfig(deploy.radixDeployment.Spec.AppName, ownerReference, deployComponent, namespace, publicPortNumber)
		if err != nil {
			return err
		}

		if activeClusterAliasIngress != nil {
			err = deploy.kubeutil.ApplyIngress(namespace, activeClusterAliasIngress)
			if err != nil {
				return err
			}
		}
	} else {
		// Remove existing fixed active cluster ingress for this component
		err = deploy.garbageCollectNonActiveClusterIngress(deployComponent)
		if err != nil {
			return err
		}
	}

	ing, err := deploy.getDefaultIngressConfig(deploy.radixDeployment.Spec.AppName, ownerReference, deployComponent, clustername, namespace, publicPortNumber)
	if err != nil {
		return err
	}

	return deploy.kubeutil.ApplyIngress(namespace, ing)
}

func isActiveCluster(clustername string) bool {
	activeClustername := os.Getenv(defaults.ActiveClusternameEnvironmentVariable)
	return strings.EqualFold(clustername, activeClustername)
}

func (deploy *Deployment) garbageCollectIngressesNoLongerInSpec() error {
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
			err = deploy.kubeclient.NetworkingV1().Ingresses(deploy.radixDeployment.GetNamespace()).Delete(context.TODO(), ing.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectAppAliasIngressNoLongerInSpecForComponent(component radixv1.RadixCommonDeployComponent) error {
	return deploy.garbageCollectIngressByLabelSelectorForComponent(fmt.Sprintf("%s=%s, %s=%s", kube.RadixComponentLabel, component.GetName(), kube.RadixAppAliasLabel, "true"))
}

func (deploy *Deployment) garbageCollectIngressNoLongerInSpecForComponent(component radixv1.RadixCommonDeployComponent) error {
	return deploy.garbageCollectIngressByLabelSelectorForComponent(getLabelSelectorForComponent(component))
}

func (deploy *Deployment) garbageCollectNonActiveClusterIngress(component radixv1.RadixCommonDeployComponent) error {
	return deploy.garbageCollectIngressByLabelSelectorForComponent(fmt.Sprintf("%s=%s, %s=%s", kube.RadixComponentLabel, component.GetName(), kube.RadixActiveClusterAliasLabel, "true"))
}

func (deploy *Deployment) garbageCollectIngressByLabelSelectorForComponent(labelSelector string) error {
	ingresses, err := deploy.kubeutil.ListIngressesWithSelector(deploy.radixDeployment.GetNamespace(), labelSelector)
	if err != nil {
		return err
	}

	if len(ingresses) > 0 {
		for n := range ingresses {
			err = deploy.kubeclient.NetworkingV1().Ingresses(deploy.radixDeployment.GetNamespace()).Delete(context.TODO(), ingresses[n].Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectAllExternalAliasIngressesForComponent(component radixv1.RadixCommonDeployComponent) error {
	return deploy.garbageCollectIngressForComponentAndExternalAlias(component, true)
}

func (deploy *Deployment) garbageCollectIngressNoLongerInSpecForComponentAndExternalAlias(component radixv1.RadixCommonDeployComponent) error {
	return deploy.garbageCollectIngressForComponentAndExternalAlias(component, false)
}

func (deploy *Deployment) garbageCollectIngressForComponentAndExternalAlias(component radixv1.RadixCommonDeployComponent, all bool) error {
	labelSelector := getLabelSelectorForExternalAlias(component)
	ingresses, err := deploy.kubeutil.ListIngressesWithSelector(deploy.radixDeployment.GetNamespace(), labelSelector)
	if err != nil {
		return err
	}

	for _, ing := range ingresses {
		garbageCollectIngress := true

		if !all {
			externalAliasForIngress := ing.Name
			for _, externalAlias := range component.GetDNSExternalAlias() {
				if externalAlias == externalAliasForIngress {
					garbageCollectIngress = false
				}
			}
		}

		if garbageCollectIngress {
			err = deploy.kubeclient.NetworkingV1().Ingresses(deploy.radixDeployment.GetNamespace()).Delete(context.TODO(), ing.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
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

func getAppAliasIngressName(appName string) string {
	return fmt.Sprintf("%s-url-alias", appName)
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

func getActiveClusterIngressName(componentName string) string {
	return fmt.Sprintf("%s-active-cluster-url-alias", componentName)
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
	return ingressConfig, err
}

func getDefaultIngressName(componentName string) string {
	return componentName
}

func (deploy *Deployment) getExternalAliasIngressConfig(
	appName string,
	ownerReference []metav1.OwnerReference,
	externalAlias string,
	component radixv1.RadixCommonDeployComponent,
	namespace string,
	publicPortNumber int32,
) (*networkingv1.Ingress, error) {
	ingressSpec := ingress.GetIngressSpec(externalAlias, component.GetName(), externalAlias, publicPortNumber)
	ingressConfig, err := ingress.GetIngressConfig(namespace, appName, component, externalAlias, ingressSpec, deploy.ingressAnnotationProviders, ownerReference)
	if err != nil {
		return nil, err
	}
	ingressConfig.ObjectMeta.Labels[kube.RadixExternalAliasLabel] = "true"
	return ingressConfig, err
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

func getPublicPortNumber(ports []radixv1.ComponentPort, publicPort string) int32 {
	for _, port := range ports {
		if strings.EqualFold(port.Name, publicPort) {
			return port.Port
		}
	}
	return 0
}

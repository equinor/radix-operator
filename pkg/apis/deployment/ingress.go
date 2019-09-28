package deployment

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	appAliasTLSSecretName       = "app-wildcard-tls-cert"
	clusterDefaultTLSSecretName = "cluster-wildcard-tls-cert"
	activeClusterTLSSecretName  = "active-cluster-wildcard-tls-cert"
)

func (deploy *Deployment) createIngress(deployComponent v1.RadixDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace
	clustername, err := deploy.kubeutil.GetClusterName()
	if err != nil {
		return err
	}

	var publicPortNumber int32
	// For backwards compatibility
	if deployComponent.PublicPort == "" {
		publicPortNumber = deployComponent.Ports[0].Port
	} else {
		publicPortNumber = getPublicPortNumber(deployComponent.Ports, deployComponent.PublicPort)
	}

	// Only the active cluster should have the DNS alias, not to cause conflics between clusters
	if deployComponent.DNSAppAlias && isActiveCluster(clustername) {
		appAliasIngress := getAppAliasIngressConfig(deployComponent.Name, deploy.radixDeployment, clustername, namespace, publicPortNumber)
		if appAliasIngress != nil {
			err = deploy.kubeutil.ApplyIngress(namespace, appAliasIngress)
			if err != nil {
				return err
			}
		}
	} else {
		deploy.garbageCollectAppAliasIngressNoLongerInSpecForComponent(deployComponent)
	}

	// Only the active cluster should have the DNS external alias, not to cause conflics between clusters
	if len(deployComponent.DNSExternalAlias) > 0 && isActiveCluster(clustername) {
		err = deploy.garbageCollectIngressNoLongerInSpecForComponentAndExternalAlias(deployComponent)
		if err != nil {
			return err
		}

		for _, externalAlias := range deployComponent.DNSExternalAlias {
			externalAliasIngress, err := deploy.getExternalAliasIngressConfig(externalAlias, deployComponent.Name, deploy.radixDeployment, namespace, publicPortNumber)
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
		activeClusterAliasIngress := getActiveClusterAliasIngressConfig(deployComponent.Name, deploy.radixDeployment, namespace, publicPortNumber)
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

	ingress := getDefaultIngressConfig(deployComponent.Name, deploy.radixDeployment, clustername, namespace, publicPortNumber)
	return deploy.kubeutil.ApplyIngress(namespace, ingress)
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

	for _, exisitingComponent := range ingresses {
		garbageCollect := true
		exisitingComponentName := exisitingComponent.ObjectMeta.Labels[kube.RadixComponentLabel]

		for _, component := range deploy.radixDeployment.Spec.Components {
			if strings.EqualFold(component.Name, exisitingComponentName) {
				garbageCollect = false
				break
			}
		}

		if garbageCollect {
			err = deploy.kubeclient.ExtensionsV1beta1().Ingresses(deploy.radixDeployment.GetNamespace()).Delete(exisitingComponent.Name, &metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectAppAliasIngressNoLongerInSpecForComponent(component v1.RadixDeployComponent) error {
	return deploy.garbageCollectIngressByLabelSelectorForComponent(component.Name, fmt.Sprintf("%s=%s, %s=%s", kube.RadixComponentLabel, component.Name, kube.RadixAppAliasLabel, "true"))
}

func (deploy *Deployment) garbageCollectExternalAliasIngressNoLongerInSpecForComponent(component v1.RadixDeployComponent) error {
	return deploy.garbageCollectIngressByLabelSelectorForComponent(component.Name, getLabelSelectorForExternalAlias(component))
}

func (deploy *Deployment) garbageCollectIngressNoLongerInSpecForComponent(component v1.RadixDeployComponent) error {
	return deploy.garbageCollectIngressByLabelSelectorForComponent(component.Name, getLabelSelectorForComponent(component))
}

func (deploy *Deployment) garbageCollectNonActiveClusterIngress(component v1.RadixDeployComponent) error {
	return deploy.garbageCollectIngressByLabelSelectorForComponent(component.Name, fmt.Sprintf("%s=%s, %s=%s", kube.RadixComponentLabel, component.Name, kube.RadixActiveClusterAliasLabel, "true"))
}

func (deploy *Deployment) garbageCollectIngressByLabelSelectorForComponent(componentName, labelSelector string) error {
	ingresses, err := deploy.kubeutil.ListIngressesWithSelector(deploy.radixDeployment.GetNamespace(), &labelSelector)
	if err != nil {
		return err
	}

	if len(ingresses) > 0 {
		for n := range ingresses {
			err = deploy.kubeclient.ExtensionsV1beta1().Ingresses(deploy.radixDeployment.GetNamespace()).Delete(ingresses[n].Name, &metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectAllExternalAliasIngressesForComponent(component radixv1.RadixDeployComponent) error {
	return deploy.garbageCollectIngressForComponentAndExternalAlias(component, true)
}

func (deploy *Deployment) garbageCollectIngressNoLongerInSpecForComponentAndExternalAlias(component radixv1.RadixDeployComponent) error {
	return deploy.garbageCollectIngressForComponentAndExternalAlias(component, false)
}

func (deploy *Deployment) garbageCollectIngressForComponentAndExternalAlias(component radixv1.RadixDeployComponent, all bool) error {
	labelSelector := getLabelSelectorForExternalAlias(component)
	ingresses, err := deploy.kubeutil.ListIngressesWithSelector(deploy.radixDeployment.GetNamespace(), &labelSelector)
	if err != nil {
		return err
	}

	for _, ingress := range ingresses {
		garbageCollectIngress := true

		if !all {
			externalAliasForIngress := ingress.Name
			for _, externalAlias := range component.DNSExternalAlias {
				if externalAlias == externalAliasForIngress {
					garbageCollectIngress = false
				}
			}
		}

		if garbageCollectIngress {
			err = deploy.kubeclient.ExtensionsV1beta1().Ingresses(deploy.radixDeployment.GetNamespace()).Delete(ingress.Name, &metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func getAppAliasIngressConfig(componentName string, radixDeployment *v1.RadixDeployment, clustername, namespace string, publicPortNumber int32) *v1beta1.Ingress {
	appAlias := os.Getenv(defaults.OperatorAppAliasBaseURLEnvironmentVariable) // .app.dev.radix.equinor.com in launch.json
	if appAlias == "" {
		return nil
	}

	hostname := fmt.Sprintf("%s.%s", radixDeployment.Spec.AppName, appAlias)
	ownerReference := getOwnerReferenceOfDeployment(radixDeployment)
	ingressSpec := getIngressSpec(hostname, componentName, appAliasTLSSecretName, publicPortNumber)

	return getIngressConfig(radixDeployment, componentName, fmt.Sprintf("%s-url-alias", radixDeployment.Spec.AppName), ownerReference, true, false, false, ingressSpec)
}

func getActiveClusterAliasIngressConfig(componentName string, radixDeployment *v1.RadixDeployment, namespace string, publicPortNumber int32) *v1beta1.Ingress {
	hostname := getActiveClusterHostName(componentName, namespace)
	if hostname == "" {
		return nil
	}
	ownerReference := getOwnerReferenceOfDeployment(radixDeployment)
	ingressSpec := getIngressSpec(hostname, componentName, activeClusterTLSSecretName, publicPortNumber)
	ingressName := fmt.Sprintf("%s-active-cluster-url-alias", componentName)

	return getIngressConfig(radixDeployment, componentName, ingressName, ownerReference, false, false, true, ingressSpec)
}

func getDefaultIngressConfig(componentName string, radixDeployment *v1.RadixDeployment, clustername, namespace string, publicPortNumber int32) *v1beta1.Ingress {
	dnsZone := os.Getenv(defaults.OperatorDNSZoneEnvironmentVariable)
	if dnsZone == "" {
		return nil
	}
	hostname := getHostName(componentName, namespace, clustername, dnsZone)
	ownerReference := getOwnerReferenceOfDeployment(radixDeployment)
	ingressSpec := getIngressSpec(hostname, componentName, clusterDefaultTLSSecretName, publicPortNumber)

	return getIngressConfig(radixDeployment, componentName, componentName, ownerReference, false, false, false, ingressSpec)
}

func (deploy *Deployment) getExternalAliasIngressConfig(externalAlias, componentName string, radixDeployment *v1.RadixDeployment, namespace string, publicPortNumber int32) (*v1beta1.Ingress, error) {
	ownerReference := getOwnerReferenceOfDeployment(radixDeployment)
	ingressSpec := getIngressSpec(externalAlias, componentName, externalAlias, publicPortNumber)
	return getIngressConfig(radixDeployment, componentName, externalAlias, ownerReference, false, true, false, ingressSpec), nil
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

func getIngressConfig(radixDeployment *v1.RadixDeployment, componentName, ingressName string, ownerReference []metav1.OwnerReference, isAlias, isExternalAlias, isActiveClusterAlias bool, ingressSpec v1beta1.IngressSpec) *v1beta1.Ingress {
	ingress := &v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: ingressName,
			Annotations: map[string]string{
				"kubernetes.io/ingress.class":              "nginx",
				"ingress.kubernetes.io/force-ssl-redirect": "true",
			},
			Labels: map[string]string{
				"radixApp":                        radixDeployment.Spec.AppName, // For backwards compatibility. Remove when cluster is migrated
				kube.RadixAppLabel:                radixDeployment.Spec.AppName,
				kube.RadixComponentLabel:          componentName,
				kube.RadixAppAliasLabel:           strconv.FormatBool(isAlias),
				kube.RadixExternalAliasLabel:      strconv.FormatBool(isExternalAlias),
				kube.RadixActiveClusterAliasLabel: strconv.FormatBool(isActiveClusterAlias),
			},
			OwnerReferences: ownerReference,
		},
		Spec: ingressSpec,
	}

	return ingress
}

func getIngressSpec(hostname, serviceName, tlsSecretName string, servicePort int32) v1beta1.IngressSpec {
	return v1beta1.IngressSpec{
		TLS: []v1beta1.IngressTLS{
			{
				Hosts: []string{
					hostname,
				},
				SecretName: tlsSecretName,
			},
		},
		Rules: []v1beta1.IngressRule{
			{
				Host: hostname,
				IngressRuleValue: v1beta1.IngressRuleValue{
					HTTP: &v1beta1.HTTPIngressRuleValue{
						Paths: []v1beta1.HTTPIngressPath{
							{
								Path: "/",
								Backend: v1beta1.IngressBackend{
									ServiceName: serviceName,
									ServicePort: intstr.IntOrString{
										IntVal: int32(servicePort),
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func getPublicPortNumber(ports []v1.ComponentPort, publicPort string) int32 {
	for _, port := range ports {
		if strings.EqualFold(port.Name, publicPort) {
			return port.Port
		}
	}
	return 0
}

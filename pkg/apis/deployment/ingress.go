package deployment

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	radixmaps "github.com/equinor/radix-common/utils/maps"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IngressConfiguration Holds all ingress annotation configurations
type IngressConfiguration struct {
	AnnotationConfigurations []AnnotationConfiguration `yaml:"configuration"`
}

// AnnotationConfiguration Holds annotations for a single configuration
type AnnotationConfiguration struct {
	Name        string
	Annotations map[string]string
}

type dnsAliasType int

const (
	dnsDefaultAlias = iota
	dnsActiveClusterAlias
	dnsAlias
	dnsAppAlias
	dnsExternalAlias
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

	ingress, err := deploy.getDefaultIngressConfig(deploy.radixDeployment.Spec.AppName, ownerReference, deployComponent, clustername, namespace, publicPortNumber)
	if err != nil {
		return err
	}

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

	for _, ingress := range ingresses {
		componentName, ok := RadixComponentNameFromComponentLabel(ingress)
		if !ok {
			continue
		}

		// Ingresses should only exist for items in component list.
		if !componentName.ExistInDeploymentSpecComponentList(deploy.radixDeployment) {
			err = deploy.kubeclient.NetworkingV1().Ingresses(deploy.radixDeployment.GetNamespace()).Delete(context.TODO(), ingress.Name, metav1.DeleteOptions{})
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

	for _, ingress := range ingresses {
		garbageCollectIngress := true

		if !all {
			externalAliasForIngress := ingress.Name
			for _, externalAlias := range component.GetDNSExternalAlias() {
				if externalAlias == externalAliasForIngress {
					garbageCollectIngress = false
				}
			}
		}

		if garbageCollectIngress {
			err = deploy.kubeclient.NetworkingV1().Ingresses(deploy.radixDeployment.GetNamespace()).Delete(context.TODO(), ingress.Name, metav1.DeleteOptions{})
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
	ingressSpec := getIngressSpec(hostname, component.GetName(), defaults.TLSSecretName, publicPortNumber)

	return deploy.getIngressConfig(appName, component, getAppAliasIngressName(appName), ownerReference, dnsAppAlias, ingressSpec, namespace)
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
	ingressSpec := getIngressSpec(hostname, component.GetName(), defaults.TLSSecretName, publicPortNumber)
	ingressName := getActiveClusterIngressName(component.GetName())

	return deploy.getIngressConfig(appName, component, ingressName, ownerReference, dnsActiveClusterAlias, ingressSpec, namespace)
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
	ingressSpec := getIngressSpec(hostname, component.GetName(), defaults.TLSSecretName, publicPortNumber)

	return deploy.getIngressConfig(appName, component, getDefaultIngressName(component.GetName()), ownerReference, dnsDefaultAlias, ingressSpec, namespace)
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
	ingressSpec := getIngressSpec(externalAlias, component.GetName(), externalAlias, publicPortNumber)
	return deploy.getIngressConfig(appName, component, externalAlias, ownerReference, dnsExternalAlias, ingressSpec, namespace)
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

func parseClientCertificateConfiguration(clientCertificate radixv1.ClientCertificate) (certificate radixv1.ClientCertificate) {
	verification := radixv1.VerificationTypeOff
	certificate = radixv1.ClientCertificate{
		Verification:              &verification,
		PassCertificateToUpstream: utils.BoolPtr(false),
	}

	if passUpstream := clientCertificate.PassCertificateToUpstream; passUpstream != nil {
		certificate.PassCertificateToUpstream = passUpstream
	}

	if verification := clientCertificate.Verification; verification != nil {
		certificate.Verification = verification
	}

	return
}

func (deploy *Deployment) getIngressConfig(
	appName string,
	component radixv1.RadixCommonDeployComponent,
	ingressName string,
	ownerReference []metav1.OwnerReference,
	aliasType dnsAliasType,
	ingressSpec networkingv1.IngressSpec,
	namespace string,
) (*networkingv1.Ingress, error) {
	annotations := map[string]string{}

	for _, ia := range deploy.ingressAnnotationProviders {
		providedAnnotations, err := ia.GetAnnotations(component, namespace)
		if err != nil {
			return nil, err
		}
		annotations = radixmaps.MergeMaps(annotations, providedAnnotations)
	}

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ingressName,
			Annotations: annotations,
			Labels: map[string]string{
				kube.RadixAppLabel:                appName,
				kube.RadixComponentLabel:          component.GetName(),
				kube.RadixAliasLabel:              strconv.FormatBool(aliasType == dnsAlias),
				kube.RadixAppAliasLabel:           strconv.FormatBool(aliasType == dnsAppAlias),
				kube.RadixExternalAliasLabel:      strconv.FormatBool(aliasType == dnsExternalAlias),
				kube.RadixActiveClusterAliasLabel: strconv.FormatBool(aliasType == dnsActiveClusterAlias),
			},
			OwnerReferences: ownerReference,
		},
		Spec: ingressSpec,
	}

	return ingress, nil
}

func getIngressSpec(hostname, serviceName, tlsSecretName string, servicePort int32) networkingv1.IngressSpec {
	pathType := networkingv1.PathTypeImplementationSpecific
	ingressClass := "nginx"

	return networkingv1.IngressSpec{
		IngressClassName: &ingressClass,
		TLS: []networkingv1.IngressTLS{
			{
				Hosts: []string{
					hostname,
				},
				SecretName: tlsSecretName,
			},
		},
		Rules: []networkingv1.IngressRule{
			{
				Host: hostname,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{
							{
								Path:     "/",
								PathType: &pathType,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: serviceName,
										Port: networkingv1.ServiceBackendPort{
											Number: servicePort,
										},
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

func getPublicPortNumber(ports []radixv1.ComponentPort, publicPort string) int32 {
	for _, port := range ports {
		if strings.EqualFold(port.Name, publicPort) {
			return port.Port
		}
	}
	return 0
}

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
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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

const (
	appAliasTLSSecretName       = "app-wildcard-tls-cert"
	clusterDefaultTLSSecretName = "cluster-wildcard-tls-cert"
	activeClusterTLSSecretName  = "active-cluster-wildcard-tls-cert"
	ingressConfigurationMap     = "radix-operator-ingress-configmap"
)

func (deploy *Deployment) createOrUpdateIngress(deployComponent v1.RadixCommonDeployComponent) error {
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

	config := loadIngressConfigFromMap(deploy.kubeutil)
	ownerReference := getOwnerReferenceOfDeployment(deploy.radixDeployment)

	// Only the active cluster should have the DNS alias, not to cause conflicts between clusters
	if deployComponent.IsDNSAppAlias() && isActiveCluster(clustername) {
		appAliasIngress := getAppAliasIngressConfig(deploy.radixDeployment.Spec.AppName, ownerReference, config, deployComponent, namespace, publicPortNumber)
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
	if dnsExternalAlias != nil && len(dnsExternalAlias) > 0 && isActiveCluster(clustername) {
		err = deploy.garbageCollectIngressNoLongerInSpecForComponentAndExternalAlias(deployComponent)
		if err != nil {
			return err
		}

		for _, externalAlias := range dnsExternalAlias {
			externalAliasIngress, err := deploy.getExternalAliasIngressConfig(deploy.radixDeployment.Spec.AppName,
				ownerReference, config, externalAlias, deployComponent, namespace, publicPortNumber)
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
		activeClusterAliasIngress := getActiveClusterAliasIngressConfig(deploy.radixDeployment.Spec.AppName,
			ownerReference, config, deployComponent, namespace, publicPortNumber)
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

	ingress := getDefaultIngressConfig(deploy.radixDeployment.Spec.AppName,
		ownerReference, config, deployComponent, clustername, namespace, publicPortNumber)
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
		componentName, ok := NewRadixComponentNameFromLabels(ingress)
		if !ok {
			continue
		}

		// Ingresses should only exist for items in component list.
		if !componentName.ExistInDeploymentSpecComponentList(deploy.radixDeployment) {
			err = deploy.kubeclient.NetworkingV1beta1().Ingresses(deploy.radixDeployment.GetNamespace()).Delete(context.TODO(), ingress.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectAppAliasIngressNoLongerInSpecForComponent(component v1.RadixCommonDeployComponent) error {
	return deploy.garbageCollectIngressByLabelSelectorForComponent(fmt.Sprintf("%s=%s, %s=%s", kube.RadixComponentLabel, component.GetName(), kube.RadixAppAliasLabel, "true"))
}

func (deploy *Deployment) garbageCollectExternalAliasIngressNoLongerInSpecForComponent(component v1.RadixCommonDeployComponent) error {
	return deploy.garbageCollectIngressByLabelSelectorForComponent(getLabelSelectorForExternalAlias(component))
}

func (deploy *Deployment) garbageCollectIngressNoLongerInSpecForComponent(component v1.RadixCommonDeployComponent) error {
	return deploy.garbageCollectIngressByLabelSelectorForComponent(getLabelSelectorForComponent(component))
}

func (deploy *Deployment) garbageCollectNonActiveClusterIngress(component v1.RadixCommonDeployComponent) error {
	return deploy.garbageCollectIngressByLabelSelectorForComponent(fmt.Sprintf("%s=%s, %s=%s", kube.RadixComponentLabel, component.GetName(), kube.RadixActiveClusterAliasLabel, "true"))
}

func (deploy *Deployment) garbageCollectIngressByLabelSelectorForComponent(labelSelector string) error {
	ingresses, err := deploy.kubeutil.ListIngressesWithSelector(deploy.radixDeployment.GetNamespace(), &labelSelector)
	if err != nil {
		return err
	}

	if len(ingresses) > 0 {
		for n := range ingresses {
			err = deploy.kubeclient.NetworkingV1beta1().Ingresses(deploy.radixDeployment.GetNamespace()).Delete(context.TODO(), ingresses[n].Name, metav1.DeleteOptions{})
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
	ingresses, err := deploy.kubeutil.ListIngressesWithSelector(deploy.radixDeployment.GetNamespace(), &labelSelector)
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
			err = deploy.kubeclient.NetworkingV1beta1().Ingresses(deploy.radixDeployment.GetNamespace()).Delete(context.TODO(), ingress.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func getAppAliasIngressConfig(appName string, ownerReference []metav1.OwnerReference, config IngressConfiguration, component v1.RadixCommonDeployComponent, namespace string, publicPortNumber int32) *networkingv1beta1.Ingress {
	appAlias := os.Getenv(defaults.OperatorAppAliasBaseURLEnvironmentVariable) // .app.dev.radix.equinor.com in launch.json
	if appAlias == "" {
		return nil
	}

	hostname := fmt.Sprintf("%s.%s", appName, appAlias)
	ingressSpec := getIngressSpec(hostname, component.GetName(), appAliasTLSSecretName, publicPortNumber)

	return getIngressConfig(appName, component, getAppAliasIngressName(appName), ownerReference, config, true, false, false, ingressSpec, namespace)
}

func getAppAliasIngressName(appName string) string {
	return fmt.Sprintf("%s-url-alias", appName)
}

func getActiveClusterAliasIngressConfig(
	appName string,
	ownerReference []metav1.OwnerReference,
	config IngressConfiguration,
	component v1.RadixCommonDeployComponent,
	namespace string,
	publicPortNumber int32,
) *networkingv1beta1.Ingress {
	hostname := getActiveClusterHostName(component.GetName(), namespace)
	if hostname == "" {
		return nil
	}
	ingressSpec := getIngressSpec(hostname, component.GetName(), activeClusterTLSSecretName, publicPortNumber)
	ingressName := getActiveClusterIngressName(component.GetName())

	return getIngressConfig(appName, component, ingressName, ownerReference, config, false, false, true, ingressSpec, namespace)
}

func getActiveClusterIngressName(componentName string) string {
	return fmt.Sprintf("%s-active-cluster-url-alias", componentName)
}

func getDefaultIngressConfig(
	appName string,
	ownerReference []metav1.OwnerReference,
	config IngressConfiguration,
	component v1.RadixCommonDeployComponent,
	clustername, namespace string,
	publicPortNumber int32,
) *networkingv1beta1.Ingress {
	dnsZone := os.Getenv(defaults.OperatorDNSZoneEnvironmentVariable)
	if dnsZone == "" {
		return nil
	}
	hostname := getHostName(component.GetName(), namespace, clustername, dnsZone)
	ingressSpec := getIngressSpec(hostname, component.GetName(), clusterDefaultTLSSecretName, publicPortNumber)

	return getIngressConfig(appName, component, getDefaultIngressName(component.GetName()), ownerReference, config, false, false, false, ingressSpec, namespace)
}

func getDefaultIngressName(componentName string) string {
	return componentName
}

func (deploy *Deployment) getExternalAliasIngressConfig(
	appName string,
	ownerReference []metav1.OwnerReference,
	config IngressConfiguration,
	externalAlias string,
	component v1.RadixCommonDeployComponent,
	namespace string,
	publicPortNumber int32,
) (*networkingv1beta1.Ingress, error) {
	ingressSpec := getIngressSpec(externalAlias, component.GetName(), externalAlias, publicPortNumber)
	return getIngressConfig(appName, component, externalAlias, ownerReference, config, false, true, false, ingressSpec, namespace), nil
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

func getAuthenticationAnnotationsFromConfiguration(authentication *v1.Authentication, componentName, namespace string) map[string]string {
	result := make(map[string]string)
	if authentication != nil {
		if authentication.ClientCertificate != nil {
			if IsSecretRequiredForClientCertificate(authentication.ClientCertificate) {
				result["nginx.ingress.kubernetes.io/auth-tls-secret"] = fmt.Sprintf("%s/%s", namespace, utils.GetComponentClientCertificateSecretName(componentName))
			}

			certificateConfig := parseClientCertificateConfiguration(*authentication.ClientCertificate)
			result["nginx.ingress.kubernetes.io/auth-tls-verify-client"] = string(*certificateConfig.Verification)
			result["nginx.ingress.kubernetes.io/auth-tls-pass-certificate-to-upstream"] = utils.TernaryString(*certificateConfig.PassCertificateToUpstream, "true", "false")
		}
	}

	return result
}

func parseClientCertificateConfiguration(clientCertificate v1.ClientCertificate) (certificate v1.ClientCertificate) {
	verification := v1.VerificationTypeOff
	certificate = v1.ClientCertificate{
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

func getIngressConfig(
	appName string,
	component v1.RadixCommonDeployComponent,
	ingressName string,
	ownerReference []metav1.OwnerReference,
	config IngressConfiguration,
	isAlias, isExternalAlias, isActiveClusterAlias bool,
	ingressSpec networkingv1beta1.IngressSpec,
	namespace string,
) *networkingv1beta1.Ingress {
	annotations := getAnnotationsFromConfigurations(config, component.GetIngressConfiguration()...)

	authentication := getAuthenticationAnnotationsFromConfiguration(component.GetAuthentication(), component.GetName(), namespace)
	annotations = radixmaps.MergeStringMaps(annotations, authentication)

	annotations["kubernetes.io/ingress.class"] = "nginx"
	annotations["ingress.kubernetes.io/force-ssl-redirect"] = "true"

	ingress := &networkingv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ingressName,
			Annotations: annotations,
			Labels: map[string]string{
				kube.RadixAppLabel:                appName,
				kube.RadixComponentLabel:          component.GetName(),
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

func getIngressSpec(hostname, serviceName, tlsSecretName string, servicePort int32) networkingv1beta1.IngressSpec {
	return networkingv1beta1.IngressSpec{
		TLS: []networkingv1beta1.IngressTLS{
			{
				Hosts: []string{
					hostname,
				},
				SecretName: tlsSecretName,
			},
		},
		Rules: []networkingv1beta1.IngressRule{
			{
				Host: hostname,
				IngressRuleValue: networkingv1beta1.IngressRuleValue{
					HTTP: &networkingv1beta1.HTTPIngressRuleValue{
						Paths: []networkingv1beta1.HTTPIngressPath{
							{
								Path: "/",
								Backend: networkingv1beta1.IngressBackend{
									ServiceName: serviceName,
									ServicePort: intstr.IntOrString{
										IntVal: servicePort,
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

func loadIngressConfigFromMap(kubeutil *kube.Kube) IngressConfiguration {

	configMap, err := kubeutil.GetConfigMap(corev1.NamespaceDefault, ingressConfigurationMap)
	if err != nil {
		return IngressConfiguration{}
	}

	return getConfigFromStringData(configMap.Data["ingressConfiguration"])
}

func getConfigFromStringData(data string) IngressConfiguration {
	config := IngressConfiguration{}
	err := yaml.Unmarshal([]byte(data), &config)
	if err != nil {
		return config
	}
	return config
}

func getAnnotationsFromConfigurations(config IngressConfiguration, configurations ...string) map[string]string {
	allAnnotations := make(map[string]string)

	if config.AnnotationConfigurations != nil &&
		len(config.AnnotationConfigurations) > 0 {
		for _, configuration := range configurations {
			annotations := getAnnotationsFromConfiguration(configuration, config)
			if annotations != nil {
				for key, value := range annotations {
					allAnnotations[key] = value
				}
			}
		}
	}

	return allAnnotations
}

func getAnnotationsFromConfiguration(name string, config IngressConfiguration) map[string]string {
	for _, ingressConfig := range config.AnnotationConfigurations {
		if strings.EqualFold(ingressConfig.Name, name) {
			return ingressConfig.Annotations
		}
	}

	return nil
}

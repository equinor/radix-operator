package deployment

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	appAliasTLSSecretName             = "app-wildcard-tls-cert"
	clusterDefaultTLSSecretName       = "cluster-wildcard-tls-cert"
	externalAliasIngressNamePattern   = "%s-external-%d"
	externalAliasTLSCertificateFormat = "%s-external-%d-tls-cert"
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

	if deployComponent.DNSAppAlias {
		appAliasIngress := getAppAliasIngressConfig(deployComponent.Name, deploy.radixDeployment, clustername, namespace, publicPortNumber)
		if appAliasIngress != nil {
			err = deploy.kubeutil.ApplyIngress(namespace, appAliasIngress)
			if err != nil {
				log.Errorf("Failed to create app alias ingress for app %s. Error was %s ", deploy.radixDeployment.Spec.AppName, err)
			}
		}
	} else {
		deploy.garbageCollectAppAliasIngressNoLongerInSpecForComponent(deployComponent)
	}

	if len(deployComponent.DNSExternalAlias) > 0 {
		for n, externalAlias := range deployComponent.DNSExternalAlias {
			externalAliasTLSCertificateName := fmt.Sprintf(externalAliasTLSCertificateFormat, deployComponent.Name, (n + 1))
			externalAliasIngressName := fmt.Sprintf(externalAliasIngressNamePattern, deploy.radixDeployment.Spec.AppName, (n + 1))
			externalAliasIngress := getExternalAliasIngressConfig(externalAlias, deployComponent.Name, deploy.radixDeployment, namespace, externalAliasIngressName, externalAliasTLSCertificateName, publicPortNumber)
			err = deploy.kubeutil.ApplyIngress(namespace, externalAliasIngress)
			if err != nil {
				log.Errorf("Failed to create external alias ingress for app %s. Error was %s ", deploy.radixDeployment.Spec.AppName, err)
			}
		}
	} else {
		deploy.garbageCollectExternalAliasIngressNoLongerInSpecForComponent(deployComponent)
	}

	ingress := getDefaultIngressConfig(deployComponent.Name, deploy.radixDeployment, clustername, namespace, publicPortNumber)
	return deploy.kubeutil.ApplyIngress(namespace, ingress)
}

func (deploy *Deployment) garbageCollectIngressesNoLongerInSpec() error {
	ingresses, err := deploy.kubeclient.ExtensionsV1beta1().Ingresses(deploy.radixDeployment.GetNamespace()).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, exisitingComponent := range ingresses.Items {
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
	return deploy.garbageCollectIngressByLabelSelectorForComponent(component.Name, fmt.Sprintf("%s=%s, %s=%s", kube.RadixComponentLabel, component.Name, kube.RadixExternalAliasLabel, "true"))
}

func (deploy *Deployment) garbageCollectIngressNoLongerInSpecForComponent(component v1.RadixDeployComponent) error {
	return deploy.garbageCollectIngressByLabelSelectorForComponent(component.Name, fmt.Sprintf("%s=%s", kube.RadixComponentLabel, component.Name))
}

func (deploy *Deployment) garbageCollectIngressByLabelSelectorForComponent(componentName, labelSelector string) error {
	ingresses, err := deploy.kubeclient.ExtensionsV1beta1().Ingresses(deploy.radixDeployment.GetNamespace()).List(metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return err
	}

	if len(ingresses.Items) > 0 {
		err = deploy.kubeclient.ExtensionsV1beta1().Ingresses(deploy.radixDeployment.GetNamespace()).Delete(ingresses.Items[0].Name, &metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func getAppAliasIngressConfig(componentName string, radixDeployment *v1.RadixDeployment, clustername, namespace string, publicPortNumber int32) *v1beta1.Ingress {
	appAlias := os.Getenv(OperatorAppAliasBaseURLEnvironmentVariable) // .app.dev.radix.equinor.com in launch.json
	if appAlias == "" {
		return nil
	}

	hostname := fmt.Sprintf("%s.%s", radixDeployment.Spec.AppName, appAlias)
	ownerReference := getOwnerReferenceOfDeployment(radixDeployment)
	ingressSpec := getIngressSpec(hostname, componentName, appAliasTLSSecretName, publicPortNumber)

	return getIngressConfig(radixDeployment, componentName, fmt.Sprintf("%s-url-alias", radixDeployment.Spec.AppName), ownerReference, true, false, ingressSpec)
}

func getDefaultIngressConfig(componentName string, radixDeployment *v1.RadixDeployment, clustername, namespace string, publicPortNumber int32) *v1beta1.Ingress {
	dnsZone := os.Getenv(OperatorDNSZoneEnvironmentVariable)
	if dnsZone == "" {
		return nil
	}
	hostname := getHostName(componentName, namespace, clustername, dnsZone)
	ownerReference := getOwnerReferenceOfDeployment(radixDeployment)
	ingressSpec := getIngressSpec(hostname, componentName, clusterDefaultTLSSecretName, publicPortNumber)

	return getIngressConfig(radixDeployment, componentName, componentName, ownerReference, false, false, ingressSpec)
}

func getExternalAliasIngressConfig(externalAlias, componentName string, radixDeployment *v1.RadixDeployment, namespace, ingressName, externalAliasTLSSecretName string, publicPortNumber int32) *v1beta1.Ingress {
	ownerReference := getOwnerReferenceOfDeployment(radixDeployment)
	ingressSpec := getIngressSpec(externalAlias, componentName, externalAliasTLSSecretName, publicPortNumber)

	return getIngressConfig(radixDeployment, componentName, ingressName, ownerReference, false, true, ingressSpec)
}

func getHostName(componentName, namespace, clustername, dnsZone string) string {
	hostnameTemplate := "%s-%s.%s.%s"
	return fmt.Sprintf(hostnameTemplate, componentName, namespace, clustername, dnsZone)
}

func getIngressConfig(radixDeployment *v1.RadixDeployment, componentName, ingressName string, ownerReference []metav1.OwnerReference, isAlias, isExternalAlias bool, ingressSpec v1beta1.IngressSpec) *v1beta1.Ingress {
	ingress := &v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: ingressName,
			Annotations: map[string]string{
				"kubernetes.io/ingress.class":              "nginx",
				"ingress.kubernetes.io/force-ssl-redirect": "true",
			},
			Labels: map[string]string{
				"radixApp":                   radixDeployment.Spec.AppName, // For backwards compatibility. Remove when cluster is migrated
				kube.RadixAppLabel:           radixDeployment.Spec.AppName,
				kube.RadixComponentLabel:     componentName,
				kube.RadixAppAliasLabel:      strconv.FormatBool(isAlias),
				kube.RadixExternalAliasLabel: strconv.FormatBool(isExternalAlias),
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

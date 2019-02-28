package deployment

import (
	"fmt"
	"os"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (deploy *Deployment) createIngress(deployComponent v1.RadixDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace
	clustername, err := deploy.kubeutil.GetClusterName()
	if err != nil {
		return err
	}

	if deployComponent.DNSAppAlias {
		appAliasIngress := getAppAliasIngressConfig(deployComponent.Name, deploy.radixDeployment, clustername, namespace, deployComponent.Ports)
		if appAliasIngress != nil {
			err = deploy.kubeutil.ApplyIngress(namespace, appAliasIngress)
			if err != nil {
				log.Errorf("Failed to create app alias ingress for app %s. Error was %s ", deploy.radixDeployment.Spec.AppName, err)
			}
		}
	}

	ingress := getDefaultIngressConfig(deployComponent.Name, deploy.radixDeployment, clustername, namespace, deployComponent.Ports)
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

func (deploy *Deployment) garbageCollectIngressNoLongerInSpecForComponent(component v1.RadixDeployComponent) error {
	ingresses, err := deploy.kubeclient.ExtensionsV1beta1().Ingresses(deploy.radixDeployment.GetNamespace()).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", kube.RadixComponentLabel, component.Name),
	})
	if err != nil {
		return err
	}

	if len(ingresses.Items) > 0 {
		err = deploy.kubeclient.ExtensionsV1beta1().Ingresses(deploy.radixDeployment.GetNamespace()).Delete(component.Name, &metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func getAppAliasIngressConfig(componentName string, radixDeployment *v1.RadixDeployment, clustername, namespace string, componentPorts []v1.ComponentPort) *v1beta1.Ingress {
	tlsSecretName := "app-wildcard-tls-cert"
	appAlias := os.Getenv(OperatorAppAliasBaseURLEnvironmentVariable) // .app.dev.radix.equinor.com in launch.json
	if appAlias == "" {
		return nil
	}

	hostname := fmt.Sprintf("%s.%s", radixDeployment.Spec.AppName, appAlias)
	ownerReference := getOwnerReferenceOfDeploymentWithName(componentName, radixDeployment)
	ingressSpec := getIngressSpec(hostname, componentName, tlsSecretName, componentPorts[0].Port)

	return getIngressConfig(radixDeployment, fmt.Sprintf("%s-url-alias", radixDeployment.Spec.AppName), ownerReference, ingressSpec)
}

func getDefaultIngressConfig(componentName string, radixDeployment *v1.RadixDeployment, clustername, namespace string, componentPorts []v1.ComponentPort) *v1beta1.Ingress {
	tlsSecretName := "cluster-wildcard-tls-cert"
	dnsZone := os.Getenv(OperatorDNSZoneEnvironmentVariable)
	if dnsZone == "" {
		return nil
	}
	hostname := getHostName(componentName, namespace, clustername, dnsZone)
	ownerReference := getOwnerReferenceOfDeploymentWithName(componentName, radixDeployment)
	ingressSpec := getIngressSpec(hostname, componentName, tlsSecretName, componentPorts[0].Port)

	return getIngressConfig(radixDeployment, componentName, ownerReference, ingressSpec)
}

func getHostName(componentName, namespace, clustername, dnsZone string) string {
	hostnameTemplate := "%s-%s.%s.%s"
	return fmt.Sprintf(hostnameTemplate, componentName, namespace, clustername, dnsZone)
}

func getIngressConfig(radixDeployment *v1.RadixDeployment, ingressName string, ownerReference []metav1.OwnerReference, ingressSpec v1beta1.IngressSpec) *v1beta1.Ingress {
	ingress := &v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: ingressName,
			Annotations: map[string]string{
				"kubernetes.io/ingress.class":              "nginx",
				"ingress.kubernetes.io/force-ssl-redirect": "true",
			},
			Labels: map[string]string{
				"radixApp":         radixDeployment.Spec.AppName, // For backwards compatibility. Remove when cluster is migrated
				kube.RadixAppLabel: radixDeployment.Spec.AppName,
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

package deployment

import (
	"fmt"
	"os"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Ingress
func (deploy *Deployment) createIngress(deployComponent v1.RadixDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace
	clustername, err := deploy.kubeutil.GetClusterName()
	if err != nil {
		return err
	}

	if deployComponent.DNSAppAlias {
		appAliasIngress := getAppAliasIngressConfig(deployComponent.Name, deploy.radixDeployment, clustername, namespace, deployComponent.Ports)
		if appAliasIngress != nil {
			err = deploy.applyIngress(namespace, appAliasIngress)
			if err != nil {
				log.Errorf("Failed to create app alias ingress for app %s. Error was %s ", deploy.radixDeployment.Spec.AppName, err)
			}
		}
	}

	ingress := getDefaultIngressConfig(deployComponent.Name, deploy.radixDeployment, clustername, namespace, deployComponent.Ports)
	return deploy.applyIngress(namespace, ingress)
}

// TOOD move to kube
func (deploy *Deployment) applyIngress(namespace string, ingress *v1beta1.Ingress) error {
	ingressName := ingress.GetName()
	log.Infof("Creating Ingress object %s in namespace %s", ingressName, namespace)

	_, err := deploy.kubeclient.ExtensionsV1beta1().Ingresses(namespace).Create(ingress)
	if errors.IsAlreadyExists(err) {
		log.Infof("Ingress object %s already exists in namespace %s, updating the object now", ingressName, namespace)
		_, err := deploy.kubeclient.ExtensionsV1beta1().Ingresses(namespace).Update(ingress)
		if err != nil {
			return fmt.Errorf("Failed to update Ingress object: %v", err)
		}
		log.Infof("Updated Ingress: %s in namespace %s", ingressName, namespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("Failed to create Ingress object: %v", err)
	}
	log.Infof("Created Ingress: %s in namespace %s", ingressName, namespace)
	return nil
}

func getAppAliasIngressConfig(componentName string, radixDeployment *v1.RadixDeployment, clustername, namespace string, componentPorts []v1.ComponentPort) *v1beta1.Ingress {
	appAlias := os.Getenv("APP_ALIAS_BASE_URL") // .app.dev.radix.equinor.com in launch.json
	if appAlias == "" {
		return nil
	}

	hostname := fmt.Sprintf("%s.%s", radixDeployment.Spec.AppName, appAlias)
	ownerReference := kube.GetOwnerReferenceOfDeploymentWithName(componentName, radixDeployment)
	ingressSpec := getIngressSpec(hostname, componentName, componentPorts[0].Port)

	return getIngressConfig(radixDeployment, fmt.Sprintf("%s-url-alias", radixDeployment.Spec.AppName), ownerReference, ingressSpec)
}

func getDefaultIngressConfig(componentName string, radixDeployment *v1.RadixDeployment, clustername, namespace string, componentPorts []v1.ComponentPort) *v1beta1.Ingress {
	dnsZone := os.Getenv("DNS_ZONE")
	if dnsZone == "" {
		return nil
	}
	hostname := getHostName(componentName, namespace, clustername, dnsZone)
	ownerReference := kube.GetOwnerReferenceOfDeploymentWithName(componentName, radixDeployment)
	ingressSpec := getIngressSpec(hostname, componentName, componentPorts[0].Port)

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
				"kubernetes.io/ingress.class": "nginx",
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

func getIngressSpec(hostname, serviceName string, servicePort int32) v1beta1.IngressSpec {
	tlsSecretName := "cluster-wildcard-tls-cert"
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

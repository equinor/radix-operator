package deployment

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	v1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

type RadixDeployHandler struct {
	kubeclient  kubernetes.Interface
	radixclient radixclient.Interface
}

func NewDeployHandler(kubeclient kubernetes.Interface, radixclient radixclient.Interface) RadixDeployHandler {
	handler := RadixDeployHandler{
		kubeclient:  kubeclient,
		radixclient: radixclient,
	}

	return handler
}

// Init handles any handler initialization
func (t *RadixDeployHandler) Init() error {
	log.Info("RadixDeployHandler.Init")
	return nil
}

// ObjectCreated is called when an object is created
func (t *RadixDeployHandler) ObjectCreated(obj interface{}) {
	log.Info("Deploy object created received.")
	radixDeploy, ok := obj.(*v1.RadixDeployment)
	if !ok {
		log.Errorf("Provided object was not a valid Radix Deployment; instead was %v", obj)
		return
	}

	log.Infof("Deploy name: %s", radixDeploy.Name)
	log.Infof("Application name: %s", radixDeploy.Spec.AppName)

	radixApplication, err := t.radixclient.RadixV1().RadixApplications("default").Get(radixDeploy.Spec.AppName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Failed to get RadixApplication object: %v", err)
		return
	} else {
		log.Infof("RadixApplication: %s", radixApplication)
	}

	appComponents := radixApplication.Spec.Components
	for i, v := range radixDeploy.Spec.Components {
		log.Infof("Deploy component %d:", i)
		log.Infof("Name: %s, Image: %s", v.Name, v.Image)
		for j, w := range appComponents {
			if v.Name != w.Name {
				continue
			}
			log.Infof("App component %d:", j)
			log.Infof("Name: %s, Public: %t", w.Name, w.Public)
			for k, x := range w.Ports {
				log.Infof("Port %d: %d", k, x)
			}
			err := t.createDeployment(radixDeploy, v, w)
			if err != nil {
				log.Errorf("Failed to create deployment: %v", err)
			}
			err = t.createService(radixDeploy, w)
			if err != nil {
				log.Errorf("Failed to create service: %v", err)
			}
			if w.Public {
				err = t.createIngress(radixDeploy, w)
				if err != nil {
					log.Errorf("Failed to create ingress: %v", err)
				}
			}
		}
	}
}

// ObjectDeleted is called when an object is deleted
func (t *RadixDeployHandler) ObjectDeleted(key string) {
	log.Info("RadixDeployment object deleted.")
}

// ObjectUpdated is called when an object is updated
func (t *RadixDeployHandler) ObjectUpdated(objOld, objNew interface{}) {
	log.Info("Deploy object updated received.")
}

func (t *RadixDeployHandler) createDeployment(radixDeploy *v1.RadixDeployment, deployComponent v1.RadixDeployComponent, appComponent v1.RadixComponent) error {
	deployment := getDeploymentConfig(radixDeploy.Name, radixDeploy.UID, deployComponent.Image, appComponent.Ports)
	log.Infof("Creating Deployment object %s", radixDeploy.ObjectMeta.Name)
	createdDeployment, err := t.kubeclient.ExtensionsV1beta1().Deployments("default").Create(deployment)
	if err != nil {
		return fmt.Errorf("Failed to create Deployment object: %v", err)
	}
	log.Infof("Created Deployment: %s", createdDeployment.Name)
	return nil
}

func (t *RadixDeployHandler) createService(radixDeploy *v1.RadixDeployment, appComponent v1.RadixComponent) error {
	service := getServiceConfig(radixDeploy.Name, radixDeploy.UID, appComponent.Ports)
	log.Infof("Creating Service object %s", radixDeploy.ObjectMeta.Name)
	createdService, err := t.kubeclient.CoreV1().Services("default").Create(service)
	if err != nil {
		return fmt.Errorf("Failed to create Service object: %v", err)
	}
	log.Infof("Created Service: %s", createdService.Name)
	return nil
}

func (t *RadixDeployHandler) createIngress(radixDeploy *v1.RadixDeployment, appComponent v1.RadixComponent) error {
	ingress := getIngressConfig(radixDeploy.Name, radixDeploy.UID, appComponent.Ports, appComponent.Name)
	log.Infof("Creating Ingress object %s", radixDeploy.ObjectMeta.Name)
	createdIngress, err := t.kubeclient.ExtensionsV1beta1().Ingresses("default").Create(ingress)
	if err != nil {
		return fmt.Errorf("Failed to create Ingress object: %v", err)
	}
	log.Infof("Created Ingress: %s", createdIngress.Name)
	return nil
}

func getDeploymentConfig(name string, uid types.UID, image string, componentPorts []int) *v1beta1.Deployment {
	trueVar := true
	deployment := &v1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"radixApp": name,
			},
			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "radix.equinor.com/v1", //need to hardcode these values for now - seems they are missing from the CRD in k8s 1.8
					Kind:       "RadixDeployment",
					Name:       name,
					UID:        uid,
					Controller: &trueVar,
				},
			},
		},
		Spec: v1beta1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"radixApp": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"radixApp": name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  name,
							Image: image,
						},
					},
				},
			},
		},
	}

	var ports []corev1.ContainerPort
	for _, v := range componentPorts {
		containerPort := corev1.ContainerPort{
			ContainerPort: int32(v),
		}
		ports = append(ports, containerPort)
	}
	deployment.Spec.Template.Spec.Containers[0].Ports = ports

	return deployment
}

func getServiceConfig(name string, uid types.UID, componentPorts []int) *corev1.Service {
	trueVar := true
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"radixApp": name,
			},
			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "radix.equinor.com/v1", //need to hardcode these values for now - seems they are missing from the CRD in k8s 1.8
					Kind:       "RadixDeployment",
					Name:       name,
					UID:        uid,
					Controller: &trueVar,
				},
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"radixApp": name,
			},
		},
	}

	var ports []corev1.ServicePort
	for _, v := range componentPorts {
		servicePort := corev1.ServicePort{
			Port: int32(v),
		}
		ports = append(ports, servicePort)
	}
	service.Spec.Ports = ports

	return service
}

func getIngressConfig(name string, uid types.UID, componentPorts []int, componentName string) *v1beta1.Ingress {
	trueVar := true
	ingress := &v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				"kubernetes.io/tls-acme":         "true",
				"traefik.frontend.rule.type":     "PathPrefixStrip",
				"traefik.backend.circuitbreaker": "NetworkErrorRatio() > 0.5",
			},
			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "radix.equinor.com/v1", //need to hardcode these values for now - seems they are missing from the CRD in k8s 1.8
					Kind:       "RadixDeployment",
					Name:       name,
					UID:        uid,
					Controller: &trueVar,
				},
			},
		},
		Spec: v1beta1.IngressSpec{
			TLS: []v1beta1.IngressTLS{
				{
					SecretName: "staas-tls",
				},
			},
			Rules: []v1beta1.IngressRule{
				{
					Host: "staas.ukwest.cloudapp.azure.com",
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: []v1beta1.HTTPIngressPath{
								{
									Path: "/" + componentName,
									Backend: v1beta1.IngressBackend{
										ServiceName: name,
										ServicePort: intstr.IntOrString{
											IntVal: int32(componentPorts[0]),
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

	return ingress
}

func int32Ptr(i int32) *int32 {
	return &i
}

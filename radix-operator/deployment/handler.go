package deployment

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	v1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

type RadixDeployHandler struct {
	kubeclient kubernetes.Interface
}

func NewDeployHandler(kubeclient kubernetes.Interface) RadixDeployHandler {
	handler := RadixDeployHandler{
		kubeclient: kubeclient,
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
	err := t.createDeployment(radixDeploy)
	if err != nil {
		log.Errorf("Failed to create deployment: %v", err)
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

func (t *RadixDeployHandler) createDeployment(app *v1.RadixDeployment) error {
	trueVar := true
	deployment := &v1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: app.Name,
			Labels: map[string]string{
				"radixApp": app.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "radix.equinor.com/v1", //need to hardcode these values for now - seems they are missing from the CRD in k8s 1.8
					Kind:       "RadixDeployment",
					Name:       app.Name,
					UID:        app.UID,
					Controller: &trueVar,
				},
			},
		},
		Spec: v1beta1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"radixApp": app.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"radixApp": app.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  app.Name,
							Image: app.Spec.Image,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 3000,
								},
							},
						},
					},
				},
			},
		},
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: app.Name,
			Labels: map[string]string{
				"radixApp": app.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "radix.equinor.com/v1", //need to hardcode these values for now - seems they are missing from the CRD in k8s 1.8
					Kind:       "RadixDeployment",
					Name:       app.Name,
					UID:        app.UID,
					Controller: &trueVar,
				},
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Port: 3000,
				},
			},
			Selector: map[string]string{
				"radixApp": app.Name,
			},
		},
	}

	ingress := &v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: app.Name,
			Annotations: map[string]string{
				"kubernetes.io/tls-acme":         "true",
				"traefik.frontend.rule.type":     "PathPrefixStrip",
				"traefik.backend.circuitbreaker": "NetworkErrorRatio() > 0.5",
			},
			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "radix.equinor.com/v1", //need to hardcode these values for now - seems they are missing from the CRD in k8s 1.8
					Kind:       "RadixDeployment",
					Name:       app.Name,
					UID:        app.UID,
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
									Path: "/" + app.Name,
									Backend: v1beta1.IngressBackend{
										ServiceName: app.Name,
										ServicePort: intstr.IntOrString{
											IntVal: 3000,
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

	log.Infof("Creating Deployment object %s", app.ObjectMeta.Name)
	createdDeployment, err := t.kubeclient.ExtensionsV1beta1().Deployments("default").Create(deployment)
	if err != nil {
		return fmt.Errorf("Failed to create Deployment object: %v", err)
	}
	log.Infof("Created Deployment: %s", createdDeployment.Name)

	log.Infof("Creating Service object %s", app.ObjectMeta.Name)
	createdService, err := t.kubeclient.CoreV1().Services("default").Create(service)
	if err != nil {
		return fmt.Errorf("Failed to create Service object: %v", err)
	}
	log.Infof("Created Service: %s", createdService.Name)

	log.Infof("Creating Ingress object %s", app.ObjectMeta.Name)
	createdIngress, err := t.kubeclient.ExtensionsV1beta1().Ingresses("default").Create(ingress)
	if err != nil {
		return fmt.Errorf("Failed to create Ingress object: %v", err)
	}
	log.Infof("Created Ingress: %s", createdIngress.Name)

	return nil
}

func int32Ptr(i int32) *int32 {
	return &i
}

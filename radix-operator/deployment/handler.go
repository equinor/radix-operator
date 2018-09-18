package deployment

import (
	"fmt"

	"encoding/json"

	"github.com/statoil/radix-operator/pkg/apis/kube"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	v1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
)

type RadixDeployHandler struct {
	kubeclient  kubernetes.Interface
	radixclient radixclient.Interface
	kubeutil    *kube.Kube
}

func NewDeployHandler(kubeclient kubernetes.Interface, radixclient radixclient.Interface) RadixDeployHandler {
	kube, _ := kube.New(kubeclient)

	handler := RadixDeployHandler{
		kubeclient:  kubeclient,
		radixclient: radixclient,
		kubeutil:    kube,
	}

	return handler
}

// Init handles any handler initialization
func (t *RadixDeployHandler) Init() error {
	logger.Info("RadixDeployHandler.Init")
	return nil
}

// ObjectCreated is called when an object is created
func (t *RadixDeployHandler) ObjectCreated(obj interface{}) error {
	logger.Info("Deploy object created received.")
	radixDeploy, ok := obj.(*v1.RadixDeployment)
	if !ok {
		return fmt.Errorf("Provided object was not a valid Radix Deployment; instead was %v", obj)
	}

	radixRegistration, err := t.radixclient.RadixV1().RadixRegistrations("default").Get(radixDeploy.Spec.AppName, metav1.GetOptions{})
	if err != nil {
		logger.Infof("Failed to get RadixRegistartion object: %v", err)
		return fmt.Errorf("Failed to get RadixRegistartion object: %v", err)
	} else {
		logger.Infof("RadixRegistartion %s exists", radixDeploy.Spec.AppName)
	}

	err = t.kubeutil.CreateSecrets(radixRegistration, radixDeploy.Spec.Environment)
	if err != nil {
		logger.Errorf("Failed to provision secrets: %v", err)
		return fmt.Errorf("Failed to provision secrets: %v", err)
	}

	for _, v := range radixDeploy.Spec.Components {
		// Deploy to current radixDeploy object's namespace
		err := t.createDeployment(radixRegistration, radixDeploy, v)
		if err != nil {
			logger.Infof("Failed to create deployment: %v", err)
			return fmt.Errorf("Failed to create deployment: %v", err)
		}
		err = t.createService(radixDeploy, v)
		if err != nil {
			logger.Infof("Failed to create service: %v", err)
			return fmt.Errorf("Failed to create service: %v", err)
		}
		if v.Public {
			err = t.createIngress(radixDeploy, v)
			if err != nil {
				logger.Infof("Failed to create ingress: %v", err)
				return fmt.Errorf("Failed to create ingress: %v", err)
			}
		}
	}

	err = t.applyRbacOnRd(radixDeploy, radixRegistration.Spec.AdGroups)
	if err != nil {
		logger.Infof("Failed to apply RBAC on RD: %v", err)
		return fmt.Errorf("Failed to apply RBAC on RD: %v", err)
	}

	return nil
}

// ObjectDeleted is called when an object is deleted
func (t *RadixDeployHandler) ObjectDeleted(key string) error {
	logger.Info("RadixDeployment object deleted.")
	return nil
}

// ObjectUpdated is called when an object is updated
func (t *RadixDeployHandler) ObjectUpdated(objOld, objNew interface{}) error {
	logger.Info("Deploy object updated received.")
	return nil
}

func (t *RadixDeployHandler) applyRbacOnRd(radixDeploy *v1.RadixDeployment, adGroups []string) error {
	logger.Infof("Applies rbac to rd %s on ns %s", radixDeploy.Name, radixDeploy.Namespace)
	role := kube.RdRole(radixDeploy, adGroups)
	rolebinding := kube.RdRoleBinding(radixDeploy, role.Name, adGroups)

	err := t.kubeutil.ApplyRole(radixDeploy.Namespace, role)
	if err != nil {
		return err
	}

	err = t.kubeutil.ApplyRoleBinding(radixDeploy.Namespace, rolebinding)
	if err != nil {
		return err
	}
	logger.Infof("Applied rbac to rd %s on ns %s", radixDeploy.Name, radixDeploy.Namespace)
	return nil
}

func (t *RadixDeployHandler) createDeployment(radixRegistration *v1.RadixRegistration, radixDeploy *v1.RadixDeployment, deployComponent v1.RadixDeployComponent) error {
	namespace := radixDeploy.Namespace
	appName := radixDeploy.Spec.AppName
	deployment := getDeploymentConfig(radixDeploy, deployComponent)

	if isRadixWebHook(appName) {
		serviceAccountName := "radix-webhook"
		serviceAccount, err := t.kubeutil.ApplyServiceAccount(serviceAccountName, namespace)
		if err != nil {
			logger.Warnf("Service account for running radix webhook not made. %v", err)
		} else {
			_ = t.kubeutil.ApplyClusterRoleToServiceAccount("radix-operator", radixRegistration, serviceAccount)
			deployment.Spec.Template.Spec.ServiceAccountName = serviceAccountName
		}
	}

	logger.Infof("Creating Deployment object %s in namespace %s", deployComponent.Name, namespace)
	createdDeployment, err := t.kubeclient.ExtensionsV1beta1().Deployments(namespace).Create(deployment)
	if errors.IsAlreadyExists(err) {
		logger.Infof("Deployment object %s already exists in namespace %s, updating the object now", deployComponent.Name, namespace)
		updatedDeployment, err := t.kubeclient.ExtensionsV1beta1().Deployments(namespace).Update(deployment)
		if err != nil {
			return fmt.Errorf("Failed to update Deployment object: %v", err)
		}
		logger.Infof("Updated Deployment: %s in namespace %s", updatedDeployment.Name, namespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("Failed to create Deployment object: %v", err)
	}
	logger.Infof("Created Deployment: %s in namespace %s", createdDeployment.Name, namespace)
	return nil
}

func isRadixWebHook(appName string) bool {
	return appName == "radix-webhook"
}

func (t *RadixDeployHandler) createService(radixDeploy *v1.RadixDeployment, deployComponent v1.RadixDeployComponent) error {
	namespace := radixDeploy.Namespace
	service := getServiceConfig(deployComponent.Name, radixDeploy.Spec.AppName, radixDeploy.UID, deployComponent.Ports)
	logger.Infof("Creating Service object %s in namespace %s", deployComponent.Name, namespace)
	createdService, err := t.kubeclient.CoreV1().Services(namespace).Create(service)
	if errors.IsAlreadyExists(err) {
		logger.Infof("Service object %s already exists in namespace %s, updating the object now", deployComponent.Name, namespace)
		oldService, err := t.kubeclient.CoreV1().Services(namespace).Get(deployComponent.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("Failed to get old Service object: %v", err)
		}
		newService := oldService.DeepCopy()
		ports := buildServicePorts(deployComponent.Ports)
		newService.Spec.Ports = ports

		oldServiceJson, err := json.Marshal(oldService)
		if err != nil {
			return fmt.Errorf("Failed to marshal old Service object: %v", err)
		}

		newServiceJson, err := json.Marshal(newService)
		if err != nil {
			return fmt.Errorf("Failed to marshal new Service object: %v", err)
		}

		patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldServiceJson, newServiceJson, corev1.Service{})
		if err != nil {
			return fmt.Errorf("Failed to create two way merge patch Service objects: %v", err)
		}

		patchedService, err := t.kubeclient.CoreV1().Services(namespace).Patch(deployComponent.Name, types.StrategicMergePatchType, patchBytes)
		if err != nil {
			return fmt.Errorf("Failed to patch Service object: %v", err)
		}
		logger.Infof("Patched Service: %s in namespace %s", patchedService.Name, namespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("Failed to create Service object: %v", err)
	}
	logger.Infof("Created Service: %s in namespace %s", createdService.Name, namespace)
	return nil
}

func (t *RadixDeployHandler) createIngress(radixDeploy *v1.RadixDeployment, deployComponent v1.RadixDeployComponent) error {
	namespace := radixDeploy.Namespace
	radixconfigmap, err := t.kubeclient.CoreV1().ConfigMaps("default").Get("radix-config", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Failed to get radix config map: %v", err)
	}
	clustername := radixconfigmap.Data["clustername"]
	logger.Infof("Cluster name: %s", clustername)
	ingress := getIngressConfig(deployComponent.Name, radixDeploy.Spec.AppName, clustername, namespace, radixDeploy.UID, deployComponent.Ports)
	logger.Infof("Creating Ingress object %s in namespace %s", deployComponent.Name, namespace)
	createdIngress, err := t.kubeclient.ExtensionsV1beta1().Ingresses(namespace).Create(ingress)
	if errors.IsAlreadyExists(err) {
		logger.Infof("Ingress object %s already exists in namespace %s, updating the object now", deployComponent.Name, namespace)
		updatedIngress, err := t.kubeclient.ExtensionsV1beta1().Ingresses(namespace).Update(ingress)
		if err != nil {
			return fmt.Errorf("Failed to update Ingress object: %v", err)
		}
		logger.Infof("Updated Ingress: %s in namespace %s", updatedIngress.Name, namespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("Failed to create Ingress object: %v", err)
	}
	logger.Infof("Created Ingress: %s in namespace %s", createdIngress.Name, namespace)
	return nil
}

func getDeploymentConfig(radixDeploy *v1.RadixDeployment, deployComponent v1.RadixDeployComponent) *v1beta1.Deployment {
	trueVar := true
	appName := radixDeploy.Spec.AppName
	uid := radixDeploy.UID
	environment := radixDeploy.Spec.Environment
	componentName := deployComponent.Name
	componentPorts := deployComponent.Ports
	replicas := deployComponent.Replicas
	deployment := &v1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: componentName,
			Labels: map[string]string{
				"radixApp":       appName,
				"radixComponent": componentName,
			},
			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "radix.equinor.com/v1", //need to hardcode these values for now - seems they are missing from the CRD in k8s 1.8
					Kind:       "RadixDeployment",
					Name:       componentName,
					UID:        uid,
					Controller: &trueVar,
				},
			},
		},
		Spec: v1beta1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"radixComponent": componentName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"radixApp":       appName,
						"radixComponent": componentName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  componentName,
							Image: deployComponent.Image,
						},
					},
					ImagePullSecrets: []corev1.LocalObjectReference{
						{
							Name: "radix-docker",
						},
					},
				},
			},
		},
	}

	var ports []corev1.ContainerPort
	for _, v := range componentPorts {
		containerPort := corev1.ContainerPort{
			Name:          v.Name,
			ContainerPort: int32(v.Port),
		}
		ports = append(ports, containerPort)
	}
	deployment.Spec.Template.Spec.Containers[0].Ports = ports

	if replicas > 0 {
		deployment.Spec.Replicas = int32Ptr(int32(replicas))
	}

	environmentVariables := getEnvironmentVariables(deployComponent.EnvironmentVariables, deployComponent.Secrets, radixDeploy.Name, environment, componentName)
	if environmentVariables != nil {
		deployment.Spec.Template.Spec.Containers[0].Env = environmentVariables
	}

	return deployment
}

func getEnvironmentVariables(radixEnvVars []v1.EnvVars, radixSecrets []string, radixDeployName, currentEnvironment, componentName string) []corev1.EnvVar {
	if radixEnvVars == nil && radixSecrets == nil {
		logger.Infof("No environment variable and secret is set for this RadixDeployment %s", radixDeployName)
		return nil
	}
	var environmentVariables []corev1.EnvVar
	// environmentVariables
	for _, v := range radixEnvVars {
		if v.Environment != currentEnvironment {
			continue
		}
		for key, value := range v.Variables {
			envVar := corev1.EnvVar{
				Name:  key,
				Value: value,
			}
			environmentVariables = append(environmentVariables, envVar)
		}
	}
	// secrets
	for _, v := range radixSecrets {
		secretKeySelector := corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: componentName,
			},
			Key: v,
		}
		envVarSource := corev1.EnvVarSource{
			SecretKeyRef: &secretKeySelector,
		}
		secretEnvVar := corev1.EnvVar{
			Name:      v,
			ValueFrom: &envVarSource,
		}
		environmentVariables = append(environmentVariables, secretEnvVar)
	}
	return environmentVariables
}

func getServiceConfig(componentName, appName string, uid types.UID, componentPorts []v1.ComponentPort) *corev1.Service {
	trueVar := true
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: componentName,
			Labels: map[string]string{
				"radixApp":       appName,
				"radixComponent": componentName,
			},
			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "radix.equinor.com/v1", //need to hardcode these values for now - seems they are missing from the CRD in k8s 1.8
					Kind:       "RadixDeployment",
					Name:       componentName,
					UID:        uid,
					Controller: &trueVar,
				},
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"radixComponent": componentName,
			},
		},
	}

	ports := buildServicePorts(componentPorts)
	service.Spec.Ports = ports

	return service
}

func getIngressConfig(componentName, appName, clustername, namespace string, uid types.UID, componentPorts []v1.ComponentPort) *v1beta1.Ingress {
	trueVar := true
	clusterUniqueChars := clustername[len(clustername)-5:] // TODO - make more robust solution
	hostname := fmt.Sprintf("%s-%s.%s.dev.radix.equinor.com", componentName, namespace, clusterUniqueChars)
	tlsSecretName := fmt.Sprintf("tls-cert-%s", componentName)
	ingress := &v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: componentName,
			Annotations: map[string]string{
				"kubernetes.io/tls-acme": "true",
			},
			Labels: map[string]string{
				"radixApp": appName,
			},
			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "radix.equinor.com/v1",
					Kind:       "RadixDeployment",
					Name:       componentName,
					UID:        uid,
					Controller: &trueVar,
				},
			},
		},
		Spec: v1beta1.IngressSpec{
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
										ServiceName: componentName,
										ServicePort: intstr.IntOrString{
											IntVal: int32(componentPorts[0].Port),
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

func buildServicePorts(componentPorts []v1.ComponentPort) []corev1.ServicePort {
	var ports []corev1.ServicePort
	for _, v := range componentPorts {
		servicePort := corev1.ServicePort{
			Name: v.Name,
			Port: int32(v.Port),
		}
		ports = append(ports, servicePort)
	}
	return ports
}

func int32Ptr(i int32) *int32 {
	return &i
}

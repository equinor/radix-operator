package deployment

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/statoil/radix-operator/pkg/apis/utils"

	monitoring "github.com/coreos/prometheus-operator/pkg/client/monitoring"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/client/monitoring/v1"
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

const (
	clusternameEnvironmentVariable     = "RADIX_CLUSTERNAME"
	environmentnameEnvironmentVariable = "RADIX_ENVIRONMENT"
	publicEndpointEnvironmentVariable  = "RADIX_PUBLIC_DOMAIN_NAME"
	radixAppEnvironmentVariable        = "RADIX_APP"
	radixComponentEnvironmentVariable  = "RADIX_COMPONENT"
	radixPortsEnvironmentVariable      = "RADIX_PORTS"
	prometheusInstanceLabel            = "LABEL_PROMETHEUS_INSTANCE"
	radixPortNamesEnvironmentVariable  = "RADIX_PORT_NAMES"
	hostnameTemplate                   = "%s-%s.%s.dev.radix.equinor.com"
	defaultReplicas                    = 1
)

// RadixDeployHandler Instance variables
type RadixDeployHandler struct {
	kubeclient              kubernetes.Interface
	radixclient             radixclient.Interface
	prometheusperatorclient monitoring.Interface
	kubeutil                *kube.Kube
}

// NewDeployHandler Constructor
func NewDeployHandler(kubeclient kubernetes.Interface, radixclient radixclient.Interface, prometheusperatorclient monitoring.Interface) RadixDeployHandler {
	kube, _ := kube.New(kubeclient)

	handler := RadixDeployHandler{
		kubeclient:              kubeclient,
		radixclient:             radixclient,
		prometheusperatorclient: prometheusperatorclient,
		kubeutil:                kube,
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

	err := t.processRadixDeployment(radixDeploy)
	if err != nil {
		return err
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
	radixDeploy, ok := objNew.(*v1.RadixDeployment)
	if !ok {
		return fmt.Errorf("Provided object was not a valid Radix Deployment; instead was %v", objNew)
	}

	err := t.processRadixDeployment(radixDeploy)
	if err != nil {
		return err
	}

	return nil
}

// TODO: Move this to the Deployment domain/package
func (t *RadixDeployHandler) processRadixDeployment(radixDeploy *v1.RadixDeployment) error {
	isLatest, err := t.isLatest(radixDeploy)
	if err != nil {
		return fmt.Errorf("Failed to check if RadixDeployment was latest. Error was %v", err)
	}

	if !isLatest {
		return fmt.Errorf("RadixDeployment %s was not the latest. Ignoring", radixDeploy.GetName())
	}

	radixRegistration, err := t.radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Get(radixDeploy.Spec.AppName, metav1.GetOptions{})
	if err != nil {
		logger.Infof("Failed to get RadixRegistartion object: %v", err)
		return fmt.Errorf("Failed to get RadixRegistartion object: %v", err)
	}

	logger.Infof("RadixRegistartion %s exists", radixDeploy.Spec.AppName)
	err = t.kubeutil.CreateSecrets(radixRegistration, radixDeploy)
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
		if v.Monitoring {
			err = t.createServiceMonitor(radixDeploy, v)
			if err != nil {
				logger.Infof("Failed to create service monitor: %v", err)
				return fmt.Errorf("Failed to create service monitor: %v", err)
			}
		}
		err = t.kubeutil.GrantAppAdminAccessToRuntimeSecrets(radixDeploy.Namespace, radixRegistration, &v)
		if err != nil {
			return fmt.Errorf("Failed to grant app admin access to own secrets. %v", err)
		}
	}

	err = t.kubeutil.GrantAppAdminAccessToNs(radixDeploy.Namespace, radixRegistration)
	if err != nil {
		logger.Infof("Failed to setup RBAC on namespace %s: %v", radixDeploy.Namespace, err)
		return fmt.Errorf("Failed to apply RBAC on namespace %s: %v", radixDeploy.Namespace, err)
	}

	return nil
}

func (t *RadixDeployHandler) isLatest(theOne *v1.RadixDeployment) (bool, error) {
	all, err := t.radixclient.RadixV1().RadixDeployments(theOne.GetNamespace()).List(metav1.ListOptions{})
	if err != nil {
		return false, err
	}

	for _, rd := range all.Items {
		if rd.GetName() != theOne.GetName() &&
			rd.CreationTimestamp.Time.After(theOne.CreationTimestamp.Time) {
			return false, nil
		}
	}

	return true, nil
}

func (t *RadixDeployHandler) createDeployment(radixRegistration *v1.RadixRegistration, radixDeploy *v1.RadixDeployment, deployComponent v1.RadixDeployComponent) error {
	namespace := radixDeploy.Namespace
	appName := radixDeploy.Spec.AppName
	deployment := t.getDeploymentConfig(radixDeploy, deployComponent)

	t.customRbacSettings(appName, namespace, radixRegistration, deployment)

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

func (t *RadixDeployHandler) customRbacSettings(appName, namespace string, radixRegistration *v1.RadixRegistration, deployment *v1beta1.Deployment) {
	if isRadixWebHook(radixRegistration.Namespace, appName) {
		serviceAccountName := "radix-github-webhook"
		serviceAccount, err := t.kubeutil.ApplyServiceAccount(serviceAccountName, namespace)
		if err != nil {
			logger.Warnf("Service account for running radix github webhook not made. %v", err)
		} else {
			_ = t.kubeutil.ApplyClusterRoleToServiceAccount("radix-operator", radixRegistration, serviceAccount)
			deployment.Spec.Template.Spec.ServiceAccountName = serviceAccountName
		}
	}
	if isRadixAPI(radixRegistration.Namespace, appName) {
		serviceAccountName := "radix-api"
		serviceAccount, err := t.kubeutil.ApplyServiceAccount(serviceAccountName, namespace)
		if err != nil {
			logger.Warnf("Error creating Service account for radix api. %v", err)
		} else {
			_ = t.kubeutil.ApplyClusterRoleToServiceAccount("radix-operator", radixRegistration, serviceAccount)
			deployment.Spec.Template.Spec.ServiceAccountName = serviceAccountName
		}
	}
}

func isRadixAPI(radixRegistrationNamespace, appName string) bool {
	return appName == "radix-api" && radixRegistrationNamespace == corev1.NamespaceDefault
}

func isRadixWebHook(radixRegistrationNamespace, appName string) bool {
	return appName == "radix-github-webhook" && radixRegistrationNamespace == corev1.NamespaceDefault
}

func (t *RadixDeployHandler) createService(radixDeploy *v1.RadixDeployment, deployComponent v1.RadixDeployComponent) error {
	namespace := radixDeploy.Namespace
	service := getServiceConfig(deployComponent.Name, radixDeploy, deployComponent.Ports)
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

		oldServiceJSON, err := json.Marshal(oldService)
		if err != nil {
			return fmt.Errorf("Failed to marshal old Service object: %v", err)
		}

		newServiceJSON, err := json.Marshal(newService)
		if err != nil {
			return fmt.Errorf("Failed to marshal new Service object: %v", err)
		}

		patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldServiceJSON, newServiceJSON, corev1.Service{})
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

func (t *RadixDeployHandler) createServiceMonitor(radixDeploy *v1.RadixDeployment, deployComponent v1.RadixDeployComponent) error {
	namespace := radixDeploy.Namespace
	serviceMonitor := getServiceMonitorConfig(deployComponent.Name, namespace, deployComponent.Ports)
	createdServiceMonitor, err := t.prometheusperatorclient.MonitoringV1().ServiceMonitors(namespace).Create(serviceMonitor)
	if errors.IsAlreadyExists(err) {
		logger.Infof("ServiceMonitor object %s already exists in namespace %s, updating the object now", deployComponent.Name, namespace)
		updatedServiceMonitor, err := t.prometheusperatorclient.MonitoringV1().ServiceMonitors(namespace).Update(serviceMonitor)
		if err != nil {
			return fmt.Errorf("Failed to update ServiceMonitor object: %v", err)
		}
		logger.Infof("Updated ServiceMonitor: %s in namespace %s", updatedServiceMonitor.Name, namespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("Failed to create ServiceMonitor object: %v", err)
	}
	logger.Infof("Created ServiceMonitor: %s in namespace %s", createdServiceMonitor.Name, namespace)
	return nil
}

func (t *RadixDeployHandler) getDeploymentConfig(radixDeploy *v1.RadixDeployment, deployComponent v1.RadixDeployComponent) *v1beta1.Deployment {
	appName := radixDeploy.Spec.AppName
	environment := radixDeploy.Spec.Environment
	componentName := deployComponent.Name
	componentPorts := deployComponent.Ports
	replicas := deployComponent.Replicas

	const branchKey, commitIDKey = "radix-branch", "radix-commit"
	rdLabels := radixDeploy.Labels
	var branch, commitID string
	if branchVal, exists := rdLabels[branchKey]; exists {
		branch = branchVal
	}
	if commitIDVal, exists := rdLabels[commitIDKey]; exists {
		commitID = commitIDVal
	}

	ownerReference := kube.GetOwnerReferenceOfDeploymentWithName(componentName, radixDeploy)

	deployment := &v1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: componentName,
			Labels: map[string]string{
				"radixApp":               appName, // For backwards compatibility. Remove when cluster is migrated
				kube.RadixAppLabel:       appName,
				kube.RadixComponentLabel: componentName,
				kube.RadixBranchLabel:    branch,
				kube.RadixCommitLabel:    commitID,
			},
			OwnerReferences: ownerReference,
		},
		Spec: v1beta1.DeploymentSpec{
			Replicas: int32Ptr(defaultReplicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					kube.RadixComponentLabel: componentName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"radixApp":               appName, // For backwards compatibility. Remove when cluster is migrated
						kube.RadixAppLabel:       appName,
						kube.RadixComponentLabel: componentName,
						kube.RadixBranchLabel:    branch,
						kube.RadixCommitLabel:    commitID,
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

	environmentVariables := t.getEnvironmentVariables(deployComponent.EnvironmentVariables, deployComponent.Secrets, deployComponent.Public, deployComponent.Ports, radixDeploy.Name, radixDeploy.Namespace, environment, appName, componentName)

	if environmentVariables != nil {
		deployment.Spec.Template.Spec.Containers[0].Env = environmentVariables
	}

	return deployment
}

func (t *RadixDeployHandler) getEnvironmentVariables(radixEnvVars v1.EnvVarsMap, radixSecrets []string, isPublic bool, ports []v1.ComponentPort, radixDeployName, namespace, currentEnvironment, appName, componentName string) []corev1.EnvVar {
	var environmentVariables []corev1.EnvVar
	if radixEnvVars != nil {
		// environmentVariables
		for key, value := range radixEnvVars {
			envVar := corev1.EnvVar{
				Name:  key,
				Value: value,
			}
			environmentVariables = append(environmentVariables, envVar)
		}
	} else {
		logger.Infof("No environment variable is set for this RadixDeployment %s", radixDeployName)
	}

	environmentVariables = t.appendDefaultVariables(currentEnvironment, environmentVariables, isPublic, namespace, appName, componentName, ports)

	// secrets
	if radixSecrets != nil {
		for _, v := range radixSecrets {
			componentSecretName := utils.GetComponentSecretName(componentName)
			secretKeySelector := corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: componentSecretName,
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
	} else {
		logger.Infof("No secret is set for this RadixDeployment %s", radixDeployName)
	}

	return environmentVariables
}

func (t *RadixDeployHandler) appendDefaultVariables(currentEnvironment string, environmentVariables []corev1.EnvVar, isPublic bool, namespace, appName, componentName string, ports []v1.ComponentPort) []corev1.EnvVar {
	clusterName, err := t.getClusterName()
	if err != nil {
		return environmentVariables
	}

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  clusternameEnvironmentVariable,
		Value: clusterName,
	})

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  environmentnameEnvironmentVariable,
		Value: currentEnvironment,
	})

	if isPublic {
		environmentVariables = append(environmentVariables, corev1.EnvVar{
			Name:  publicEndpointEnvironmentVariable,
			Value: fmt.Sprintf(hostnameTemplate, componentName, namespace, clusterName),
		})
	}

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  radixAppEnvironmentVariable,
		Value: appName,
	})

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  radixComponentEnvironmentVariable,
		Value: componentName,
	})

	if len(ports) > 0 {
		portNumbers, portNames := getPortNumbersAndNamesString(ports)
		environmentVariables = append(environmentVariables, corev1.EnvVar{
			Name:  radixPortsEnvironmentVariable,
			Value: portNumbers,
		})

		environmentVariables = append(environmentVariables, corev1.EnvVar{
			Name:  radixPortNamesEnvironmentVariable,
			Value: portNames,
		})
	}

	return environmentVariables
}

func getPortNumbersAndNamesString(ports []v1.ComponentPort) (string, string) {
	portNumbers := "("
	portNames := "("
	portsSize := len(ports)
	for i, portObj := range ports {
		if i < portsSize-1 {
			portNumbers += fmt.Sprint(portObj.Port) + " "
			portNames += fmt.Sprint(portObj.Name) + " "
		} else {
			portNumbers += fmt.Sprint(portObj.Port) + ")"
			portNames += fmt.Sprint(portObj.Name) + ")"
		}
	}
	return portNumbers, portNames
}

func getServiceConfig(componentName string, radixDeployment *v1.RadixDeployment, componentPorts []v1.ComponentPort) *corev1.Service {
	ownerReference := kube.GetOwnerReferenceOfDeploymentWithName(componentName, radixDeployment)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: componentName,
			Labels: map[string]string{
				"radixApp":               radixDeployment.Spec.AppName, // For backwards compatibility. Remove when cluster is migrated
				kube.RadixAppLabel:       radixDeployment.Spec.AppName,
				kube.RadixComponentLabel: componentName,
			},
			OwnerReferences: ownerReference,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				kube.RadixComponentLabel: componentName,
			},
		},
	}

	ports := buildServicePorts(componentPorts)
	service.Spec.Ports = ports

	return service
}

func getServiceMonitorConfig(componentName, namespace string, componentPorts []v1.ComponentPort) *monitoringv1.ServiceMonitor {
	serviceMonitor := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name: componentName,
			Labels: map[string]string{
				"prometheus": os.Getenv(prometheusInstanceLabel),
			},
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Endpoints: []monitoringv1.Endpoint{
				{
					Interval: "5s",
					Port:     componentPorts[0].Name,
				},
			},
			JobLabel: fmt.Sprintf("%s-%s", namespace, componentName),
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{
					namespace,
				},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"radix-component": componentName,
				},
			},
		},
	}
	return serviceMonitor
}

func (t *RadixDeployHandler) createIngress(radixDeploy *v1.RadixDeployment, deployComponent v1.RadixDeployComponent) error {
	namespace := radixDeploy.Namespace
	clustername, err := t.getClusterName()
	if err != nil {
		return err
	}

	if deployComponent.DNSAppAlias {
		appAliasIngress := getAppAliasIngressConfig(deployComponent.Name, radixDeploy, clustername, namespace, deployComponent.Ports)
		if appAliasIngress != nil {
			err = t.applyIngress(namespace, appAliasIngress)
			if err != nil {
				logger.Errorf("Failed to create app alias ingress for app %s. Error was %s ", radixDeploy.Spec.AppName, err)
			}
		}
	}

	ingress := getDefaultIngressConfig(deployComponent.Name, radixDeploy, clustername, namespace, deployComponent.Ports)
	return t.applyIngress(namespace, ingress)
}

func (t *RadixDeployHandler) applyIngress(namespace string, ingress *v1beta1.Ingress) error {
	ingressName := ingress.GetName()
	logger.Infof("Creating Ingress object %s in namespace %s", ingressName, namespace)

	_, err := t.kubeclient.ExtensionsV1beta1().Ingresses(namespace).Create(ingress)
	if errors.IsAlreadyExists(err) {
		logger.Infof("Ingress object %s already exists in namespace %s, updating the object now", ingressName, namespace)
		_, err := t.kubeclient.ExtensionsV1beta1().Ingresses(namespace).Update(ingress)
		if err != nil {
			return fmt.Errorf("Failed to update Ingress object: %v", err)
		}
		logger.Infof("Updated Ingress: %s in namespace %s", ingressName, namespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("Failed to create Ingress object: %v", err)
	}
	logger.Infof("Created Ingress: %s in namespace %s", ingressName, namespace)
	return nil
}

func (t *RadixDeployHandler) getClusterName() (string, error) {
	radixconfigmap, err := t.kubeclient.CoreV1().ConfigMaps(corev1.NamespaceDefault).Get("radix-config", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("Failed to get radix config map: %v", err)
	}
	clustername := radixconfigmap.Data["clustername"]
	logger.Infof("Cluster name: %s", clustername)
	return clustername, nil
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
	hostname := fmt.Sprintf(hostnameTemplate, componentName, namespace, clustername)
	ownerReference := kube.GetOwnerReferenceOfDeploymentWithName(componentName, radixDeployment)
	ingressSpec := getIngressSpec(hostname, componentName, componentPorts[0].Port)

	return getIngressConfig(radixDeployment, componentName, ownerReference, ingressSpec)
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

package deployment

import (
	"encoding/json"
	"fmt"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/coreos/prometheus-operator/pkg/client/monitoring"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/client/monitoring/v1"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
)

const (
	ClusternameEnvironmentVariable       = "RADIX_CLUSTERNAME"
	ContainerRegistryEnvironmentVariable = "RADIX_CONTAINER_REGISTRY"
	EnvironmentnameEnvironmentVariable   = "RADIX_ENVIRONMENT"
	PublicEndpointEnvironmentVariable    = "RADIX_PUBLIC_DOMAIN_NAME"
	RadixAppEnvironmentVariable          = "RADIX_APP"
	RadixComponentEnvironmentVariable    = "RADIX_COMPONENT"
	RadixPortsEnvironmentVariable        = "RADIX_PORTS"
	RadixPortNamesEnvironmentVariable    = "RADIX_PORT_NAMES"
	RadixDNSZoneEnvironmentVariable      = "RADIX_DNS_ZONE"
	DefaultReplicas                      = 1

	prometheusInstanceLabel = "LABEL_PROMETHEUS_INSTANCE"
)

// Deployment Instance variables
type Deployment struct {
	kubeclient              kubernetes.Interface
	radixclient             radixclient.Interface
	kubeutil                *kube.Kube
	prometheusperatorclient monitoring.Interface
	registration            *v1.RadixRegistration
	radixDeployment         *v1.RadixDeployment
}

// NewDeployment Constructor
func NewDeployment(kubeclient kubernetes.Interface, radixclient radixclient.Interface, prometheusperatorclient monitoring.Interface, registration *v1.RadixRegistration, radixDeployment *v1.RadixDeployment) (Deployment, error) {
	kubeutil, err := kube.New(kubeclient)
	if err != nil {
		return Deployment{}, err
	}

	return Deployment{
		kubeclient,
		radixclient,
		kubeutil, prometheusperatorclient, registration, radixDeployment}, nil
}

// ConstructForTargetEnvironments Will build a list of deployments for each target environment
func ConstructForTargetEnvironments(config *v1.RadixApplication, containerRegistry, jobName, imageTag, branch, commitID string, targetEnvs map[string]bool) ([]v1.RadixDeployment, error) {
	radixDeployments := []v1.RadixDeployment{}
	for _, env := range config.Spec.Environments {
		if _, contains := targetEnvs[env.Name]; !contains {
			continue
		}

		if !targetEnvs[env.Name] {
			// Target environment exists in config but should not be built
			continue
		}

		radixComponents := getRadixComponentsForEnv(config, containerRegistry, env.Name, imageTag)
		radixDeployment := createRadixDeployment(config.Name, env.Name, jobName, imageTag, branch, commitID, radixComponents)
		radixDeployments = append(radixDeployments, radixDeployment)
	}

	return radixDeployments, nil
}

// Apply Will make deployment effective
func (deploy *Deployment) Apply() error {
	log.Infof("Apply radix deployment %s on env %s", deploy.radixDeployment.ObjectMeta.Name, deploy.radixDeployment.ObjectMeta.Namespace)
	_, err := deploy.radixclient.RadixV1().RadixDeployments(deploy.radixDeployment.ObjectMeta.Namespace).Create(deploy.radixDeployment)
	if err != nil {
		return err
	}
	return nil
}

// IsLatestInTheEnvironment Checks if the deployment is the latest in the same namespace as specified in the deployment
func (deploy *Deployment) IsLatestInTheEnvironment() (bool, error) {
	all, err := deploy.radixclient.RadixV1().RadixDeployments(deploy.radixDeployment.GetNamespace()).List(metav1.ListOptions{})
	if err != nil {
		return false, err
	}

	for _, rd := range all.Items {
		if rd.GetName() != deploy.radixDeployment.GetName() &&
			rd.CreationTimestamp.Time.After(deploy.radixDeployment.CreationTimestamp.Time) {
			return false, nil
		}
	}

	return true, nil
}

// OnDeploy Process Radix deplyment
func (deploy *Deployment) OnDeploy() error {
	err := deploy.kubeutil.CreateSecrets(deploy.registration, deploy.radixDeployment)
	if err != nil {
		log.Errorf("Failed to provision secrets: %v", err)
		return fmt.Errorf("Failed to provision secrets: %v", err)
	}

	for _, v := range deploy.radixDeployment.Spec.Components {
		// Deploy to current radixDeploy object's namespace
		err := deploy.createDeployment(v)
		if err != nil {
			log.Infof("Failed to create deployment: %v", err)
			return fmt.Errorf("Failed to create deployment: %v", err)
		}
		err = deploy.createService(v)
		if err != nil {
			log.Infof("Failed to create service: %v", err)
			return fmt.Errorf("Failed to create service: %v", err)
		}
		if v.Public {
			err = deploy.createIngress(v)
			if err != nil {
				log.Infof("Failed to create ingress: %v", err)
				return fmt.Errorf("Failed to create ingress: %v", err)
			}
		}
		if v.Monitoring {
			err = deploy.createServiceMonitor(v)
			if err != nil {
				log.Infof("Failed to create service monitor: %v", err)
				return fmt.Errorf("Failed to create service monitor: %v", err)
			}
		}
	}

	return nil
}

func createRadixDeployment(appName, env, jobName, imageTag, branch, commitID string, components []v1.RadixDeployComponent) v1.RadixDeployment {
	deployName := utils.GetDeploymentName(appName, env, imageTag)
	radixDeployment := v1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployName,
			Namespace: utils.GetEnvironmentNamespace(appName, env),
			Labels: map[string]string{
				"radixApp":             appName, // For backwards compatibility. Remove when cluster is migrated
				kube.RadixAppLabel:     appName,
				kube.RadixEnvLabel:     env,
				kube.RadixBranchLabel:  branch,
				kube.RadixCommitLabel:  commitID,
				kube.RadixJobNameLabel: jobName,
			},
		},
		Spec: v1.RadixDeploymentSpec{
			AppName:     appName,
			Environment: env,
			Components:  components,
		},
	}
	return radixDeployment
}

func getRadixComponentsForEnv(radixApplication *v1.RadixApplication, containerRegistry, env, imageTag string) []v1.RadixDeployComponent {
	appName := radixApplication.Name
	dnsAppAlias := radixApplication.Spec.DNSAppAlias
	components := []v1.RadixDeployComponent{}

	for _, appComponent := range radixApplication.Spec.Components {
		componentName := appComponent.Name
		variables := getEnvironmentVariables(appComponent, env)

		deployComponent := v1.RadixDeployComponent{
			Name:                 componentName,
			Image:                utils.GetImagePath(containerRegistry, appName, componentName, imageTag),
			Replicas:             appComponent.Replicas,
			Public:               appComponent.Public,
			Ports:                appComponent.Ports,
			Secrets:              appComponent.Secrets,
			EnvironmentVariables: variables, // todo: use single EnvVars instead
			DNSAppAlias:          env == dnsAppAlias.Environment && componentName == dnsAppAlias.Component,
			Monitoring:           appComponent.Monitoring,
			Resources:            appComponent.Resources,
		}

		components = append(components, deployComponent)
	}
	return components
}

func getEnvironmentVariables(component v1.RadixComponent, env string) v1.EnvVarsMap {
	if component.EnvironmentVariables == nil {
		return v1.EnvVarsMap{}
	}

	for _, variables := range component.EnvironmentVariables {
		if variables.Environment == env {
			return variables.Variables
		}
	}
	return v1.EnvVarsMap{}
}

func (deploy *Deployment) createDeployment(deployComponent v1.RadixDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace
	appName := deploy.radixDeployment.Spec.AppName
	deployment := deploy.getDeploymentConfig(deployComponent)

	deploy.customSecuritySettings(appName, namespace, deployment)

	log.Infof("Creating Deployment object %s in namespace %s", deployComponent.Name, namespace)
	createdDeployment, err := deploy.kubeclient.ExtensionsV1beta1().Deployments(namespace).Create(deployment)
	if errors.IsAlreadyExists(err) {
		log.Infof("Deployment object %s already exists in namespace %s, updating the object now", deployComponent.Name, namespace)
		updatedDeployment, err := deploy.kubeclient.ExtensionsV1beta1().Deployments(namespace).Update(deployment)
		if err != nil {
			return fmt.Errorf("Failed to update Deployment object: %v", err)
		}
		log.Infof("Updated Deployment: %s in namespace %s", updatedDeployment.Name, namespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("Failed to create Deployment object: %v", err)
	}
	log.Infof("Created Deployment: %s in namespace %s", createdDeployment.Name, namespace)
	return nil
}

func (deploy *Deployment) customSecuritySettings(appName, namespace string, deployment *v1beta1.Deployment) {
	// need to be able to get serviceaccount token inside container
	automountServiceAccountToken := true
	if isRadixWebHook(deploy.registration.Namespace, appName) {
		serviceAccountName := "radix-github-webhook"
		serviceAccount, err := deploy.kubeutil.ApplyServiceAccount(serviceAccountName, namespace)
		if err != nil {
			log.Warnf("Service account for running radix github webhook not made. %v", err)
		} else {
			_ = deploy.kubeutil.ApplyClusterRoleToServiceAccount("radix-operator", deploy.registration, serviceAccount)
			deployment.Spec.Template.Spec.ServiceAccountName = serviceAccountName
		}
		deployment.Spec.Template.Spec.AutomountServiceAccountToken = &automountServiceAccountToken
	}
	if isRadixAPI(deploy.registration.Namespace, appName) {
		serviceAccountName := "radix-api"
		serviceAccount, err := deploy.kubeutil.ApplyServiceAccount(serviceAccountName, namespace)
		if err != nil {
			log.Warnf("Error creating Service account for radix api. %v", err)
		} else {
			_ = deploy.kubeutil.ApplyClusterRoleToServiceAccount("radix-operator", deploy.registration, serviceAccount)
			deployment.Spec.Template.Spec.ServiceAccountName = serviceAccountName
		}
		deployment.Spec.Template.Spec.AutomountServiceAccountToken = &automountServiceAccountToken
	}
}

func isRadixAPI(radixRegistrationNamespace, appName string) bool {
	return appName == "radix-api" && radixRegistrationNamespace == corev1.NamespaceDefault
}

func isRadixWebHook(radixRegistrationNamespace, appName string) bool {
	return appName == "radix-github-webhook" && radixRegistrationNamespace == corev1.NamespaceDefault
}

func (deploy *Deployment) createService(deployComponent v1.RadixDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace
	service := getServiceConfig(deployComponent.Name, deploy.radixDeployment, deployComponent.Ports)
	log.Infof("Creating Service object %s in namespace %s", deployComponent.Name, namespace)
	createdService, err := deploy.kubeclient.CoreV1().Services(namespace).Create(service)
	if errors.IsAlreadyExists(err) {
		log.Infof("Service object %s already exists in namespace %s, updating the object now", deployComponent.Name, namespace)
		oldService, err := deploy.kubeclient.CoreV1().Services(namespace).Get(deployComponent.Name, metav1.GetOptions{})
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

		patchedService, err := deploy.kubeclient.CoreV1().Services(namespace).Patch(deployComponent.Name, types.StrategicMergePatchType, patchBytes)
		if err != nil {
			return fmt.Errorf("Failed to patch Service object: %v", err)
		}
		log.Infof("Patched Service: %s in namespace %s", patchedService.Name, namespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("Failed to create Service object: %v", err)
	}
	log.Infof("Created Service: %s in namespace %s", createdService.Name, namespace)
	return nil
}

func (deploy *Deployment) createServiceMonitor(deployComponent v1.RadixDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace
	serviceMonitor := getServiceMonitorConfig(deployComponent.Name, namespace, deployComponent.Ports)
	createdServiceMonitor, err := deploy.prometheusperatorclient.MonitoringV1().ServiceMonitors(namespace).Create(serviceMonitor)
	if errors.IsAlreadyExists(err) {
		log.Infof("ServiceMonitor object %s already exists in namespace %s, updating the object now", deployComponent.Name, namespace)
		updatedServiceMonitor, err := deploy.prometheusperatorclient.MonitoringV1().ServiceMonitors(namespace).Update(serviceMonitor)
		if err != nil {
			return fmt.Errorf("Failed to update ServiceMonitor object: %v", err)
		}
		log.Infof("Updated ServiceMonitor: %s in namespace %s", updatedServiceMonitor.Name, namespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("Failed to create ServiceMonitor object: %v", err)
	}
	log.Infof("Created ServiceMonitor: %s in namespace %s", createdServiceMonitor.Name, namespace)
	return nil
}

func (deploy *Deployment) getDeploymentConfig(deployComponent v1.RadixDeployComponent) *v1beta1.Deployment {
	appName := deploy.radixDeployment.Spec.AppName
	environment := deploy.radixDeployment.Spec.Environment
	componentName := deployComponent.Name
	componentPorts := deployComponent.Ports
	replicas := deployComponent.Replicas
	automountServiceAccountToken := false

	const branchKey, commitIDKey = "radix-branch", "radix-commit"
	rdLabels := deploy.radixDeployment.Labels
	var branch, commitID string
	if branchVal, exists := rdLabels[branchKey]; exists {
		branch = branchVal
	}
	if commitIDVal, exists := rdLabels[commitIDKey]; exists {
		commitID = commitIDVal
	}

	ownerReference := kube.GetOwnerReferenceOfDeploymentWithName(componentName, deploy.radixDeployment)
	securityContext := getSecurityContextForContainer()

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
			Replicas: int32Ptr(DefaultReplicas),
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
					Annotations: map[string]string{
						"apparmor.security.beta.kubernetes.io/pod": "runtime/default",
						"seccomp.security.alpha.kubernetes.io/pod": "docker/default",
					},
				},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken: &automountServiceAccountToken,
					Containers: []corev1.Container{
						{
							Name:            componentName,
							Image:           deployComponent.Image,
							SecurityContext: securityContext,
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

	environmentVariables := deploy.getEnvironmentVariables(deployComponent.EnvironmentVariables, deployComponent.Secrets, deployComponent.Public, deployComponent.Ports, deploy.radixDeployment.Name, deploy.radixDeployment.Namespace, environment, appName, componentName)

	if environmentVariables != nil {
		deployment.Spec.Template.Spec.Containers[0].Env = environmentVariables
	}

	resourceRequirements := getResourceRequirements(deployComponent)

	if resourceRequirements != nil {
		deployment.Spec.Template.Spec.Containers[0].Resources = *resourceRequirements
	}

	return deployment
}

func getSecurityContextForContainer() *corev1.SecurityContext {
	allowPrivilegeEscalation := false
	// runAsNonRoot := true
	// runAsUser := int64(1000)

	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: &allowPrivilegeEscalation,
		// RunAsNonRoot:             &runAsNonRoot,
		// RunAsUser:                &runAsUser,
	}
}

func getResourceRequirements(deployComponent v1.RadixDeployComponent) *corev1.ResourceRequirements {
	// if you only set limit, it will use the same values for request
	limits := corev1.ResourceList{}
	requests := corev1.ResourceList{}
	for name, limit := range deployComponent.Resources.Limits {
		limits[corev1.ResourceName(name)], _ = resource.ParseQuantity(limit)
	}
	for name, req := range deployComponent.Resources.Requests {
		requests[corev1.ResourceName(name)], _ = resource.ParseQuantity(req)
	}

	if len(limits) <= 0 && len(requests) <= 0 {
		return nil
	}

	req := corev1.ResourceRequirements{
		Limits:   limits,
		Requests: requests,
	}

	return &req
}

func (deploy *Deployment) getEnvironmentVariables(radixEnvVars v1.EnvVarsMap, radixSecrets []string, isPublic bool, ports []v1.ComponentPort, radixDeployName, namespace, currentEnvironment, appName, componentName string) []corev1.EnvVar {
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
		log.Infof("No environment variable is set for this RadixDeployment %s", radixDeployName)
	}

	environmentVariables = deploy.appendDefaultVariables(currentEnvironment, environmentVariables, isPublic, namespace, appName, componentName, ports)

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
		log.Infof("No secret is set for this RadixDeployment %s", radixDeployName)
	}

	return environmentVariables
}

func (deploy *Deployment) appendDefaultVariables(currentEnvironment string, environmentVariables []corev1.EnvVar, isPublic bool, namespace, appName, componentName string, ports []v1.ComponentPort) []corev1.EnvVar {
	clusterName, err := deploy.kubeutil.GetClusterName()
	if err != nil {
		return environmentVariables
	}

	dnsZone := os.Getenv("DNS_ZONE")
	if dnsZone == "" {
		return nil
	}

	containerRegistry, err := deploy.kubeutil.GetContainerRegistry()
	if err != nil {
		return environmentVariables
	}

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  ContainerRegistryEnvironmentVariable,
		Value: containerRegistry,
	})

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  RadixDNSZoneEnvironmentVariable,
		Value: dnsZone,
	})

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  ClusternameEnvironmentVariable,
		Value: clusterName,
	})

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  EnvironmentnameEnvironmentVariable,
		Value: currentEnvironment,
	})

	if isPublic {
		environmentVariables = append(environmentVariables, corev1.EnvVar{
			Name:  PublicEndpointEnvironmentVariable,
			Value: getHostName(componentName, namespace, clusterName, dnsZone),
		})
	}

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  RadixAppEnvironmentVariable,
		Value: appName,
	})

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  RadixComponentEnvironmentVariable,
		Value: componentName,
	})

	if len(ports) > 0 {
		portNumbers, portNames := getPortNumbersAndNamesString(ports)
		environmentVariables = append(environmentVariables, corev1.EnvVar{
			Name:  RadixPortsEnvironmentVariable,
			Value: portNumbers,
		})

		environmentVariables = append(environmentVariables, corev1.EnvVar{
			Name:  RadixPortNamesEnvironmentVariable,
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

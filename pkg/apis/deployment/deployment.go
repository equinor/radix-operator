package deployment

import (
	"fmt"

	"github.com/coreos/prometheus-operator/pkg/client/monitoring"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	err := deploy.CreateSecrets(deploy.registration, deploy.radixDeployment)
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

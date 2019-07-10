package deployment

import (
	"fmt"
	"time"

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
	DefaultReplicas = 1

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
		radixDeployment := constructRadixDeployment(config.Name, env.Name, jobName, imageTag, branch, commitID, radixComponents)
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

// OnSync compares the actual state with the desired, and attempts to
// converge the two
func (deploy *Deployment) OnSync() error {
	if IsRadixDeploymentInactive(deploy.radixDeployment) {
		log.Warnf("Ignoring RadixDeployment %s/%s as it's inactive.", deploy.GetNamespace(), deploy.GetName())
		return nil
	}

	continueReconsiliation, err := deploy.syncStatuses()
	if err != nil {
		return err
	}
	if !continueReconsiliation {
		log.Infof("stopping reconsiliation, status updated, triggering new sync")
		return nil
	}

	return deploy.syncDeployment()
}

// GetNamespace gets the namespace of radixDeployment
func (deploy *Deployment) GetNamespace() string {
	return deploy.radixDeployment.GetNamespace()
}

// GetName gets the name of radixDeployment
func (deploy *Deployment) GetName() string {
	return deploy.radixDeployment.GetName()
}

// IsRadixDeploymentInactive checks if deployment is inactive
func IsRadixDeploymentInactive(rd *v1.RadixDeployment) bool {
	return rd == nil || rd.Status.Status == v1.DeploymentInactive
}

func (deploy *Deployment) syncStatuses() (continueReconsiliation bool, err error) {
	continueReconsiliation = true

	allRds, err := deploy.radixclient.RadixV1().RadixDeployments(deploy.GetNamespace()).List(metav1.ListOptions{})
	if err != nil {
		err = fmt.Errorf("Failed to get all RadixDeployments. Error was %v", err)
	}

	if deploy.isLatestInTheEnvironment(allRds.Items) {
		// only continue reconsiliation if status is already set
		continueReconsiliation = deploy.radixDeployment.Status.Status == v1.DeploymentActive
		err = deploy.setRdToActive()
		if err != nil {
			log.Errorf("Failed to set rd (%s) status to active", deploy.GetName())
			return false, err
		}
		err = deploy.setOldRdsToInactive(allRds.Items)
		if err != nil {
			// should this lead to new RD not being deployed?
			log.Warnf("Failed to set old rds statuses to inactive")
		}
	} else {
		// Should not be put back on queue
		continueReconsiliation = false
		log.Warnf("RadixDeployment %s was not the latest. Ignoring", deploy.GetName())
		// rd is inactive - reflect in status field
		err = deploy.setRdToInactive()
	}
	return
}

func (deploy *Deployment) syncDeployment() error {
	// can garbageCollectComponentsNoLongerInSpec be moved to syncComponents()?
	err := deploy.garbageCollectComponentsNoLongerInSpec()
	if err != nil {
		return fmt.Errorf("Failed to perform garbage collection of removed components: %v", err)
	}

	err = deploy.createSecrets(deploy.registration, deploy.radixDeployment)
	if err != nil {
		log.Errorf("Failed to provision secrets: %v", err)
		return fmt.Errorf("Failed to provision secrets: %v", err)
	}

	err = deploy.denyTrafficFromOtherNamespaces()
	if err != nil {
		errmsg := "Failed to setup NSP whitelist: "
		log.Errorf("%s%v", errmsg, err)
		return fmt.Errorf("%s%v", errmsg, err)
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
		if v.PublicPort != "" || v.Public {
			err = deploy.createIngress(v)
			if err != nil {
				log.Infof("Failed to create ingress: %v", err)
				return fmt.Errorf("Failed to create ingress: %v", err)
			}
		} else {
			err = deploy.garbageCollectIngressNoLongerInSpecForComponent(v)
			if err != nil {
				log.Infof("Failed to delete ingress: %v", err)
				return fmt.Errorf("Failed to delete ingress: %v", err)
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

func (deploy *Deployment) setRdToActive() error {
	if deploy.radixDeployment.Status.Status == v1.DeploymentActive {
		return nil
	}

	deploy.radixDeployment.Status.Status = v1.DeploymentActive
	deploy.radixDeployment.Status.ActiveFrom = metav1.NewTime(time.Now().UTC())
	return saveStatusRd(deploy.radixclient, deploy.radixDeployment)
}

func (deploy *Deployment) setRdToInactive() error {
	return setRdToInactive(deploy.radixclient, deploy.radixDeployment)
}

func setRdToInactive(radixClient radixclient.Interface, rd *v1.RadixDeployment) error {
	if rd.Status.Status == v1.DeploymentInactive {
		return nil
	}

	rd.Status.Status = v1.DeploymentInactive
	rd.Status.ActiveTo = metav1.NewTime(time.Now().UTC())
	return saveStatusRd(radixClient, rd)
}

func saveStatusRd(radixClient radixclient.Interface, rd *v1.RadixDeployment) error {
	_, err := radixClient.RadixV1().RadixDeployments(rd.GetNamespace()).UpdateStatus(rd)
	return err
}

func (deploy *Deployment) setOldRdsToInactive(allRds []v1.RadixDeployment) error {
	for _, rd := range allRds {
		if rd.GetName() != deploy.GetName() {
			err := setRdToInactive(deploy.radixclient, &rd)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// isLatestInTheEnvironment Checks if the deployment is the latest in the same namespace as specified in the deployment
func (deploy *Deployment) isLatestInTheEnvironment(allRds []v1.RadixDeployment) bool {
	for _, rd := range allRds {
		if rd.GetName() != deploy.GetName() && deploy.isCreatedBefore(rd) {
			return false
		}
	}

	return true
}

func (deploy *Deployment) isCreatedBefore(rd v1.RadixDeployment) bool {
	currentDeployActiveFrom := deploy.radixDeployment.Status.ActiveFrom
	compareDeployActiveFrom := rd.Status.ActiveFrom

	if currentDeployActiveFrom.IsZero() {
		currentDeployActiveFrom = deploy.radixDeployment.CreationTimestamp
	}
	if compareDeployActiveFrom.IsZero() {
		compareDeployActiveFrom = rd.CreationTimestamp
	}

	return currentDeployActiveFrom.Before(&compareDeployActiveFrom)
}

func (deploy *Deployment) garbageCollectComponentsNoLongerInSpec() error {
	err := deploy.garbageCollectDeploymentsNoLongerInSpec()
	if err != nil {
		return err
	}

	err = deploy.garbageCollectServicesNoLongerInSpec()
	if err != nil {
		return err
	}

	err = deploy.garbageCollectIngressesNoLongerInSpec()
	if err != nil {
		return err
	}

	err = deploy.garbageCollectSecretsNoLongerInSpec()
	if err != nil {
		return err
	}

	err = deploy.garbageCollectRoleBindingsNoLongerInSpec()
	if err != nil {
		return err
	}

	err = deploy.garbageCollectServiceMonitorsNoLongerInSpec()
	if err != nil {
		return err
	}

	return nil
}

func constructRadixDeployment(appName, env, jobName, imageTag, branch, commitID string, components []v1.RadixDeployComponent) v1.RadixDeployment {
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

func getLabelSelectorForComponent(component v1.RadixDeployComponent) string {
	return fmt.Sprintf("%s=%s", kube.RadixComponentLabel, component.Name)
}

func getLabelSelectorForExternalAlias(component v1.RadixDeployComponent) string {
	return fmt.Sprintf("%s=%s, %s=%s", kube.RadixComponentLabel, component.Name, kube.RadixExternalAliasLabel, "true")
}

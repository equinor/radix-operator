package deployment

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	monitoring "github.com/coreos/prometheus-operator/pkg/client/versioned"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	errorUtils "github.com/equinor/radix-operator/pkg/apis/utils/errors"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// DefaultReplicas Hold the default replicas for the deployment if nothing is stated in the radix config
	DefaultReplicas         = 1
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
func NewDeployment(kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	prometheusperatorclient monitoring.Interface,
	registration *v1.RadixRegistration,
	radixDeployment *v1.RadixDeployment) (Deployment, error) {

	return Deployment{
		kubeclient,
		radixclient,
		kubeutil, prometheusperatorclient, registration, radixDeployment}, nil
}

// GetDeploymentComponent Gets the index  of and the component given name
func GetDeploymentComponent(rd *v1.RadixDeployment, name string) (int, *v1.RadixDeployComponent) {
	for index, component := range rd.Spec.Components {
		if strings.EqualFold(component.Name, name) {
			return index, &component
		}
	}

	return -1, nil
}

// ConstructForTargetEnvironment Will build a deployment for target environment
func ConstructForTargetEnvironment(config *v1.RadixApplication, containerRegistry, jobName, imageTag, branch, commitID string, env string) (v1.RadixDeployment, error) {
	radixComponents := getRadixComponentsForEnv(config, containerRegistry, env, imageTag)
	radixDeployment := constructRadixDeployment(config, env, jobName, imageTag, branch, commitID, radixComponents)
	return radixDeployment, nil
}

// DeployToEnvironment Will return true/false depending on it has a mapping in config
func DeployToEnvironment(env v1.Environment, targetEnvs map[string]bool) bool {
	if _, contains := targetEnvs[env.Name]; contains && targetEnvs[env.Name] {
		return true
	}

	return false
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
	requeue := deploy.restoreStatus()

	if IsRadixDeploymentInactive(deploy.radixDeployment) {
		log.Warnf("Ignoring RadixDeployment %s/%s as it's inactive.", deploy.getNamespace(), deploy.getName())
		return nil
	}

	if requeue {
		// If this is an active deployment restored from status, it is important that the other inactive RDs are restored
		// before this is reprocessed, as the code will now skip OnSync if only a status has changed on the RD
		return fmt.Errorf("Requeue, status was restored for active deployment, %s, and we need to trigger a new sync", deploy.getName())
	}

	stopReconciliation, err := deploy.syncStatuses()
	if err != nil {
		return err
	}
	if stopReconciliation {
		log.Infof("stop reconciliation, status updated triggering new sync")
		return nil
	}

	err = deploy.syncDeployment()

	if err == nil {
		// Only remove old RDs if deployment is successful
		deploy.maintainHistoryLimit()
	}

	return err
}

// IsRadixDeploymentInactive checks if deployment is inactive
func IsRadixDeploymentInactive(rd *v1.RadixDeployment) bool {
	return rd == nil || rd.Status.Condition == v1.DeploymentInactive
}

// GetLatestDeploymentInNamespace Gets the last deployment in namespace
func GetLatestDeploymentInNamespace(radixclient radixclient.Interface, namespace string) (*v1.RadixDeployment, error) {
	allRDs, err := radixclient.RadixV1().RadixDeployments(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	if len(allRDs.Items) > 0 {
		for _, rd := range allRDs.Items {
			if isLatest(&rd, slice.PointersOf(allRDs.Items).([]*v1.RadixDeployment)) {
				return &rd, nil
			}
		}
	}

	return nil, nil
}

// getNamespace gets the namespace of radixDeployment
func (deploy *Deployment) getNamespace() string {
	return deploy.radixDeployment.GetNamespace()
}

// getName gets the name of radixDeployment
func (deploy *Deployment) getName() string {
	return deploy.radixDeployment.GetName()
}

// See https://github.com/equinor/radix-velero-plugin/blob/master/velero-plugins/deployment/restore.go
func (deploy *Deployment) restoreStatus() bool {
	requeue := false

	if restoredStatus, ok := deploy.radixDeployment.Annotations[kube.RestoredStatusAnnotation]; ok {
		if deploy.radixDeployment.Status.Condition == "" {
			var status v1.RadixDeployStatus
			err := json.Unmarshal([]byte(restoredStatus), &status)
			if err != nil {
				log.Error("Unable to get status from annotation", err)
				return false
			}

			deploy.radixDeployment.Status.Condition = status.Condition
			deploy.radixDeployment.Status.ActiveFrom = status.ActiveFrom
			deploy.radixDeployment.Status.ActiveTo = status.ActiveTo
			err = saveStatusRD(deploy.radixclient, deploy.radixDeployment)
			if err != nil {
				log.Error("Unable to restore status", err)
				return false
			}

			// Need to requeue
			requeue = true
		}
	}

	return requeue
}

func (deploy *Deployment) syncStatuses() (stopReconciliation bool, err error) {
	stopReconciliation = false

	allRDs, err := deploy.kubeutil.ListRadixDeployments(deploy.getNamespace())
	if err != nil {
		err = fmt.Errorf("Failed to get all RadixDeployments. Error was %v", err)
	}

	if deploy.isLatestInTheEnvironment(allRDs) {
		// Should always reconcile, because we now skip sync if only status on RD has been modified
		stopReconciliation = false
		err = deploy.updateStatusOnActiveDeployment()
		if err != nil {
			log.Errorf("Failed to set rd (%s) status to active", deploy.getName())
			return false, err
		}
		err = deploy.setOtherRDsToInactive(allRDs)
		if err != nil {
			// should this lead to new RD not being deployed?
			log.Warnf("Failed to set old rds statuses to inactive")
		}
	} else {
		// Inactive - Should not be put back on queue - stop reconciliation
		// Inactive status is updated when latest rd reconciliation is triggered
		stopReconciliation = true
		log.Warnf("RadixDeployment %s was not the latest. Ignoring", deploy.getName())
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

	errs := []error{}
	for _, v := range deploy.radixDeployment.Spec.Components {
		// Deploy to current radixDeploy object's namespace
		err := deploy.createDeployment(v)
		if err != nil {
			log.Infof("Failed to create deployment: %v", err)
			errs = append(errs, fmt.Errorf("Failed to create deployment: %v", err))
			continue
		}
		err = deploy.createService(v)
		if err != nil {
			log.Infof("Failed to create service: %v", err)
			errs = append(errs, fmt.Errorf("Failed to create service: %v", err))
			continue
		}
		if v.PublicPort != "" || v.Public {
			err = deploy.createIngress(v)
			if err != nil {
				log.Infof("Failed to create ingress: %v", err)
				errs = append(errs, fmt.Errorf("Failed to create ingress: %v", err))
				continue
			}
		} else {
			err = deploy.garbageCollectIngressNoLongerInSpecForComponent(v)
			if err != nil {
				log.Infof("Failed to delete ingress: %v", err)
				errs = append(errs, fmt.Errorf("Failed to delete ingress: %v", err))
				continue
			}
		}

		if v.Monitoring {
			err = deploy.createServiceMonitor(v)
			if err != nil {
				log.Infof("Failed to create service monitor: %v", err)
				errs = append(errs, fmt.Errorf("Failed to create service monitor: %v", err))
				continue
			}
		}
	}

	// If any error occured when syncing of components
	if len(errs) > 0 {
		return errorUtils.Concat(errs)
	}

	return nil
}

func (deploy *Deployment) updateStatusOnActiveDeployment() error {
	if deploy.radixDeployment.Status.Condition == v1.DeploymentActive {
		deploy.radixDeployment.Status.Reconciled = metav1.NewTime(time.Now().UTC())
	} else {
		deploy.radixDeployment.Status.Condition = v1.DeploymentActive
		deploy.radixDeployment.Status.ActiveFrom = metav1.NewTime(time.Now().UTC())
	}

	return saveStatusRD(deploy.radixclient, deploy.radixDeployment)
}

func setRDToInactive(radixClient radixclient.Interface, rd *v1.RadixDeployment, activeTo metav1.Time) error {
	if rd.Status.Condition == v1.DeploymentInactive {
		return nil
	}

	rd.Status.Condition = v1.DeploymentInactive
	rd.Status.ActiveTo = metav1.NewTime(activeTo.Time)
	rd.Status.ActiveFrom = getActiveFrom(rd)
	return saveStatusRD(radixClient, rd)
}

func saveStatusRD(radixClient radixclient.Interface, rd *v1.RadixDeployment) error {
	_, err := radixClient.RadixV1().RadixDeployments(rd.GetNamespace()).UpdateStatus(rd)
	return err
}

func (deploy *Deployment) setOtherRDsToInactive(allRDs []*v1.RadixDeployment) error {
	sortedRDs := sortRDsByActiveFromTimestampDesc(allRDs)
	prevRDActiveFrom := metav1.Time{}

	for _, rd := range sortedRDs {
		if rd.GetName() != deploy.getName() {
			err := setRDToInactive(deploy.radixclient, rd, prevRDActiveFrom)
			if err != nil {
				return err
			}
			prevRDActiveFrom = getActiveFrom(rd)
		} else {
			prevRDActiveFrom = getActiveFrom(deploy.radixDeployment)
		}
	}
	return nil
}

func sortRDsByActiveFromTimestampDesc(rds []*v1.RadixDeployment) []*v1.RadixDeployment {
	sort.Slice(rds, func(i, j int) bool {
		return isRD1ActiveBeforeRD2(rds[j], rds[i])
	})
	return rds
}

func sortRDsByActiveFromTimestampAsc(rds []*v1.RadixDeployment) []*v1.RadixDeployment {
	sort.Slice(rds, func(i, j int) bool {
		return isRD1ActiveBeforeRD2(rds[i], rds[j])
	})
	return rds
}

// isLatestInTheEnvironment Checks if the deployment is the latest in the same namespace as specified in the deployment
func (deploy *Deployment) isLatestInTheEnvironment(allRDs []*v1.RadixDeployment) bool {
	return isLatest(deploy.radixDeployment, allRDs)
}

// isLatest Checks if the deployment is the latest in the same namespace as specified in the deployment
func isLatest(deploy *v1.RadixDeployment, allRDs []*v1.RadixDeployment) bool {
	for _, rd := range allRDs {
		if rd.GetName() != deploy.GetName() && isRD1ActiveBeforeRD2(deploy, rd) {
			return false
		}
	}

	return true
}

func isRD1ActiveBeforeRD2(rd1 *v1.RadixDeployment, rd2 *v1.RadixDeployment) bool {
	rd1ActiveFrom := getActiveFrom(rd1)
	rd2ActiveFrom := getActiveFrom(rd2)

	return rd1ActiveFrom.Before(&rd2ActiveFrom)
}

func getActiveFrom(rd *v1.RadixDeployment) metav1.Time {
	if rd.Status.ActiveFrom.IsZero() {
		return rd.CreationTimestamp
	}
	return rd.Status.ActiveFrom
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

func constructRadixDeployment(radixApplication *v1.RadixApplication, env, jobName, imageTag, branch, commitID string, components []v1.RadixDeployComponent) v1.RadixDeployment {
	appName := radixApplication.GetName()
	deployName := utils.GetDeploymentName(appName, env, imageTag)
	imagePullSecrets := []corev1.LocalObjectReference{}
	if len(radixApplication.Spec.PrivateImageHubs) > 0 {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: applicationconfig.PrivateImageHubSecretName})
	}

	radixDeployment := v1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployName,
			Namespace: utils.GetEnvironmentNamespace(appName, env),
			Labels: map[string]string{
				kube.RadixAppLabel:     appName,
				kube.RadixEnvLabel:     env,
				kube.RadixCommitLabel:  commitID,
				kube.RadixJobNameLabel: jobName,
			},
			Annotations: map[string]string{
				kube.RadixBranchAnnotation: branch,
			},
		},
		Spec: v1.RadixDeploymentSpec{
			AppName:          appName,
			Environment:      env,
			Components:       components,
			ImagePullSecrets: imagePullSecrets,
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

func (deploy *Deployment) maintainHistoryLimit() {
	historyLimit := os.Getenv(defaults.DeploymentsHistoryLimitEnvironmentVariable)
	if historyLimit != "" {
		limit, err := strconv.Atoi(historyLimit)
		if err != nil {
			log.Warnf("'%s' is not set to a proper number, %s, and cannot be parsed.", defaults.DeploymentsHistoryLimitEnvironmentVariable, historyLimit)
			return
		}

		deployments, err := deploy.kubeutil.ListRadixDeployments(deploy.getNamespace())
		if err != nil {
			log.Errorf("Failed to get all RadixDeployments. Error was %v", err)
			return
		}

		if len(deployments) > limit {
			numToDelete := len(deployments) - limit
			if numToDelete <= 0 {
				return
			}

			deployments = sortRDsByActiveFromTimestampAsc(deployments)
			for i := 0; i < numToDelete; i++ {
				log.Infof("Removing deployment %s from %s", deployments[i].Name, deployments[i].Namespace)
				deploy.radixclient.RadixV1().RadixDeployments(deploy.getNamespace()).Delete(deployments[i].Name, &metav1.DeleteOptions{})
			}
		}
	}
}

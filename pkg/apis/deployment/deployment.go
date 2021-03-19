package deployment

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"k8s.io/client-go/util/retry"

	monitoring "github.com/coreos/prometheus-operator/pkg/client/versioned"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/metrics"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	errorUtils "github.com/equinor/radix-operator/pkg/apis/utils/errors"
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

// GetDeploymentCommonComponent Gets the index and component/job from a RadixDeployment
// The index refers the the index in either the components or jobs spec of the RadixDeployment
// Use GetType() to identify if the index refers to the components or jobs section
// Returns -1, nil if component/job does not exist
func GetDeploymentCommonComponent(rd *v1.RadixDeployment, name string) (int, v1.RadixCommonDeployComponent) {
	for index, component := range rd.Spec.Components {
		if strings.EqualFold(component.Name, name) {
			return index, &component
		}
	}
	for index, job := range rd.Spec.Jobs {
		if strings.EqualFold(job.Name, name) {
			return index, &job
		}
	}
	return -1, nil
}

// ConstructForTargetEnvironment Will build a deployment for target environment
func ConstructForTargetEnvironment(config *v1.RadixApplication, jobName, imageTag, branch, commitID string, componentImages map[string]pipeline.ComponentImage, env string) (v1.RadixDeployment, error) {
	components := getRadixComponentsForEnv(config, env, componentImages)
	jobs := NewJobComponentsBuilder(config, env, componentImages).JobComponents()
	radixDeployment := constructRadixDeployment(config, env, jobName, imageTag, branch, commitID, components, jobs)
	return radixDeployment, nil
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
		log.Debugf("Ignoring RadixDeployment %s/%s as it's inactive.", deploy.getNamespace(), deploy.getName())
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
		metrics.RequestedResources(deploy.registration, deploy.radixDeployment)
	}

	return err
}

// IsRadixDeploymentInactive checks if deployment is inactive
func IsRadixDeploymentInactive(rd *v1.RadixDeployment) bool {
	return rd == nil || rd.Status.Condition == v1.DeploymentInactive
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

			err = deploy.updateRadixDeploymentStatus(deploy.radixDeployment, func(currStatus *v1.RadixDeployStatus) {
				currStatus.Condition = status.Condition
				currStatus.ActiveFrom = status.ActiveFrom
				currStatus.ActiveTo = status.ActiveTo
			})
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

	deploy.configureRbac()

	err = deploy.createOrUpdateSecrets(deploy.registration, deploy.radixDeployment)
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

	for _, component := range deploy.radixDeployment.Spec.Components {
		if err := deploy.syncDeploymentForRadixComponent(&component); err != nil {
			errs = append(errs, err)
		}
	}

	for _, jobComponent := range deploy.radixDeployment.Spec.Jobs {
		jobSchedulerComponent := NewJobSchedulerComponent(&jobComponent, deploy.radixDeployment)
		if err := deploy.syncDeploymentForRadixComponent(jobSchedulerComponent); err != nil {
			errs = append(errs, err)
		}
	}

	// If any error occurred when syncing of components
	if len(errs) > 0 {
		return errorUtils.Concat(errs)
	}

	return nil
}

func (deploy *Deployment) configureRbac() {
	rbacFunc := GetDeploymentRbacConfigurators(deploy)
	for _, rbac := range rbacFunc {
		rbac()
	}
}

func (deploy *Deployment) updateStatusOnActiveDeployment() error {
	return deploy.updateRadixDeploymentStatus(deploy.radixDeployment, func(currStatus *v1.RadixDeployStatus) {
		if deploy.radixDeployment.Status.Condition == v1.DeploymentActive {
			currStatus.Reconciled = metav1.NewTime(time.Now().UTC())
		} else {
			currStatus.Condition = v1.DeploymentActive
			currStatus.ActiveFrom = metav1.NewTime(time.Now().UTC())
		}
	})
}

func (deploy *Deployment) setRDToInactive(rd *v1.RadixDeployment, activeTo metav1.Time) error {
	if rd.Status.Condition == v1.DeploymentInactive {
		return nil
	}
	return deploy.updateRadixDeploymentStatus(rd, func(currStatus *v1.RadixDeployStatus) {
		currStatus.Condition = v1.DeploymentInactive
		currStatus.ActiveTo = metav1.NewTime(activeTo.Time)
		currStatus.ActiveFrom = getActiveFrom(rd)
	})
}

func (deploy *Deployment) updateRadixDeploymentStatus(rd *v1.RadixDeployment, changeStatusFunc func(currStatus *v1.RadixDeployStatus)) error {
	rdInterface := deploy.radixclient.RadixV1().RadixDeployments(rd.GetNamespace())
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		currentRD, err := rdInterface.Get(rd.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}
		changeStatusFunc(&currentRD.Status)
		_, err = rdInterface.UpdateStatus(currentRD)

		if err == nil && rd.GetName() == deploy.radixDeployment.GetName() {
			currentRD, err = rdInterface.Get(rd.GetName(), metav1.GetOptions{})
			if err == nil {
				deploy.radixDeployment = currentRD
			}
		}
		return err
	})
	return err
}

func (deploy *Deployment) setOtherRDsToInactive(allRDs []*v1.RadixDeployment) error {
	sortedRDs := sortRDsByActiveFromTimestampDesc(allRDs)
	if len(sortedRDs) == 0 {
		return nil
	}
	prevRD := sortedRDs[0]

	for _, rd := range sortedRDs {
		if strings.EqualFold(rd.GetName(), deploy.getName()) {
			prevRD = deploy.radixDeployment
			continue
		}
		activeTo := getActiveFrom(prevRD)
		err := deploy.setRDToInactive(rd, activeTo)
		if err != nil {
			return err
		}
		prevRD = rd
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

	err = deploy.garbageCollectHPAsNoLongerInSpec()
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

func constructRadixDeployment(radixApplication *v1.RadixApplication, env, jobName, imageTag, branch, commitID string, components []v1.RadixDeployComponent, jobs []v1.RadixDeployJobComponent) v1.RadixDeployment {
	appName := radixApplication.GetName()
	deployName := utils.GetDeploymentName(appName, env, imageTag)
	imagePullSecrets := []corev1.LocalObjectReference{}
	if len(radixApplication.Spec.PrivateImageHubs) > 0 {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: defaults.PrivateImageHubSecretName})
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
			Jobs:             jobs,
			ImagePullSecrets: imagePullSecrets,
		},
	}
	return radixDeployment
}

func getLabelSelectorForComponent(component v1.RadixCommonDeployComponent) string {
	return fmt.Sprintf("%s=%s", kube.RadixComponentLabel, component.GetName())
}

func getLabelSelectorForExternalAlias(component v1.RadixCommonDeployComponent) string {
	return fmt.Sprintf("%s=%s, %s=%s", kube.RadixComponentLabel, component.GetName(), kube.RadixExternalAliasLabel, "true")
}

func getLabelSelectorForBlobVolumeMountSecret(component v1.RadixCommonDeployComponent) string {
	return fmt.Sprintf("%s=%s, %s=%s", kube.RadixComponentLabel, component.GetName(), kube.RadixMountTypeLabel, string(v1.MountTypeBlob))
}

func getRadixJobSchedulerImage() (string, error) {
	image := os.Getenv(defaults.OperatorRadixJobSchedulerEnvironmentVariable)

	if image == "" {
		err := fmt.Errorf("Cannot obtain radix-job-builder image tag as %s has not been set for the operator", defaults.OperatorRadixJobSchedulerEnvironmentVariable)
		log.Error(err)
	}

	return image, nil
}

func (deploy *Deployment) maintainHistoryLimit() {
	historyLimit := strings.TrimSpace(os.Getenv(defaults.DeploymentsHistoryLimitEnvironmentVariable))
	if historyLimit == "" {
		return
	}

	limit, err := strconv.Atoi(historyLimit)
	if err != nil {
		log.Warnf("'%s' is not set to a proper number, %s, and cannot be parsed.", defaults.DeploymentsHistoryLimitEnvironmentVariable, historyLimit)
		return
	}

	deployments, err := deploy.kubeutil.ListRadixDeployments(deploy.getNamespace())
	if err != nil {
		log.Warnf("failed to get all Radix deployments. Error was %v", err)
		return
	}

	numToDelete := len(deployments) - limit
	if numToDelete <= 0 {
		return
	}

	deployments = sortRDsByActiveFromTimestampAsc(deployments)
	for i := 0; i < numToDelete; i++ {
		log.Infof("Removing deployment %s from %s", deployments[i].Name, deployments[i].Namespace)
		err := deploy.radixclient.RadixV1().RadixDeployments(deploy.getNamespace()).Delete(deployments[i].Name, &metav1.DeleteOptions{})
		if err != nil {
			log.Warnf("failed to delete old deployment %s: %v", deployments[i].Name, err)
		}
	}
}

func (deploy *Deployment) syncDeploymentForRadixComponent(component v1.RadixCommonDeployComponent) error {
	// Deploy to current radixDeploy object's namespace
	err := deploy.createOrUpdateDeployment(component)
	if err != nil {
		log.Infof("Failed to create deployment: %v", err)
		return fmt.Errorf("Failed to create deployment: %v", err)
	}
	err = deploy.createOrUpdateHPA(component)
	if err != nil {
		log.Infof("Failed to create horizontal pod autoscaler: %v", err)
		return fmt.Errorf("Failed to create deployment: %v", err)
	}
	err = deploy.createOrUpdateService(component)
	if err != nil {
		log.Infof("Failed to create service: %v", err)
		return fmt.Errorf("Failed to create service: %v", err)
	}
	if component.GetPublicPort() != "" || component.IsPublic() {
		err = deploy.createOrUpdateIngress(component)
		if err != nil {
			log.Infof("Failed to create ingress: %v", err)
			return fmt.Errorf("Failed to create ingress: %v", err)
		}
	} else {
		err = deploy.garbageCollectIngressNoLongerInSpecForComponent(component)
		if err != nil {
			log.Infof("Failed to delete ingress: %v", err)
			return fmt.Errorf("Failed to delete ingress: %v", err)
		}
	}

	if component.GetMonitoring() {
		err = deploy.createOrUpdateServiceMonitor(component)
		if err != nil {
			log.Infof("Failed to create service monitor: %v", err)
			return fmt.Errorf("Failed to create service monitor: %v", err)
		}
	} else {
		err = deploy.deleteServiceMonitorForComponent(component)
		if err != nil {
			log.Infof("Failed to delete servicemonitor: %v", err)
			return fmt.Errorf("Failed to delete servicemonitor: %v", err)
		}
	}
	return nil
}

func (deploy *Deployment) getPodSpecAffinity(deployComponent v1.RadixCommonDeployComponent) *corev1.Affinity {
	affinity := &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{}},
		},
	}

	deploy.addGpuNodeSelectorTerms(deployComponent, affinity.NodeAffinity)

	if len(affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms) > 0 {
		return affinity
	}
	return nil
}

func (deploy *Deployment) addGpuNodeSelectorTerms(deployComponent v1.RadixCommonDeployComponent, nodeAffinity *corev1.NodeAffinity) {
	nodeGpuValue := strings.ReplaceAll(deployComponent.GetNode().Gpu, " ", "")
	if len(nodeGpuValue) == 0 {
		return
	}
	nodeGpuList := strings.Split(nodeGpuValue, ",")
	if len(nodeGpuList) == 0 {
		return
	}
	includingGpus := make([]string, 0)
	excludingGpus := make([]string, 0)
	for _, gpu := range nodeGpuList {
		if strings.HasPrefix(gpu, "-") {
			excludingGpus = append(excludingGpus, strings.ToLower(gpu[1:]))
			continue
		}
		includingGpus = append(includingGpus, strings.ToLower(gpu))
	}
	nodeSelectorTerm := corev1.NodeSelectorTerm{}
	if len(includingGpus) > 0 {
		nodeSelectorTerm.MatchExpressions = append(nodeSelectorTerm.MatchExpressions, corev1.NodeSelectorRequirement{Key: "gpu", Operator: corev1.NodeSelectorOpIn, Values: includingGpus})
	}
	if len(excludingGpus) > 0 {
		nodeSelectorTerm.MatchExpressions = append(nodeSelectorTerm.MatchExpressions, corev1.NodeSelectorRequirement{Key: "gpu", Operator: corev1.NodeSelectorOpNotIn, Values: excludingGpus})
	}
	nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = append(nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms, nodeSelectorTerm)
}

package deployment

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/equinor/radix-common/utils/errors"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/metrics"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

const (
	// DefaultReplicas Hold the default replicas for the deployment if nothing is stated in the radix config
	DefaultReplicas         = 1
	prometheusInstanceLabel = "LABEL_PROMETHEUS_INSTANCE"
)

// DeploymentSyncer defines interface for syncing a RadixDeployment
type DeploymentSyncer interface {
	OnSync() error
}

// Deployment Instance variables
type Deployment struct {
	kubeclient                 kubernetes.Interface
	radixclient                radixclient.Interface
	kubeutil                   *kube.Kube
	prometheusperatorclient    monitoring.Interface
	registration               *v1.RadixRegistration
	radixDeployment            *v1.RadixDeployment
	auxResourceManagers        []AuxiliaryResourceManager
	ingressAnnotationProviders []IngressAnnotationProvider
	tenantId                   string
	kubernetesApiPort          int32
}

// Test if NewDeployment implements DeploymentSyncerFactory
var _ DeploymentSyncerFactory = DeploymentSyncerFactoryFunc(NewDeployment)

// NewDeployment Constructor
func NewDeployment(kubeclient kubernetes.Interface, kubeutil *kube.Kube, radixclient radixclient.Interface, prometheusperatorclient monitoring.Interface, registration *v1.RadixRegistration, radixDeployment *v1.RadixDeployment, tenantId string, kubernetesApiPort int32, ingressAnnotationProviders []IngressAnnotationProvider, auxResourceManagers []AuxiliaryResourceManager) DeploymentSyncer {
	return &Deployment{
		kubeclient:                 kubeclient,
		radixclient:                radixclient,
		kubeutil:                   kubeutil,
		prometheusperatorclient:    prometheusperatorclient,
		registration:               registration,
		radixDeployment:            radixDeployment,
		auxResourceManagers:        auxResourceManagers,
		ingressAnnotationProviders: ingressAnnotationProviders,
		tenantId:                   tenantId,
		kubernetesApiPort:          kubernetesApiPort,
	}
}

// GetDeploymentComponent Gets the index of and the component given name
func GetDeploymentComponent(rd *v1.RadixDeployment, name string) (int, *v1.RadixDeployComponent) {
	for index, component := range rd.Spec.Components {
		if strings.EqualFold(component.Name, name) {
			return index, &component
		}
	}
	return -1, nil
}

// GetDeploymentJobComponent Gets the index of and the job component given name
func GetDeploymentJobComponent(rd *v1.RadixDeployment, name string) (int, *v1.RadixDeployJobComponent) {
	for index, job := range rd.Spec.Jobs {
		if strings.EqualFold(job.Name, name) {
			return index, &job
		}
	}
	return -1, nil
}

// ConstructForTargetEnvironment Will build a deployment for target environment
func ConstructForTargetEnvironment(config *v1.RadixApplication, jobName string, imageTag string, branch string, componentImages map[string]pipeline.ComponentImage, env string, defaultEnvVars v1.EnvVarsMap) (*v1.RadixDeployment, error) {
	commitID := defaultEnvVars[defaults.RadixCommitHashEnvironmentVariable]
	gitTags := defaultEnvVars[defaults.RadixGitTagsEnvironmentVariable]
	components, err := GetRadixComponentsForEnv(config, env, componentImages, defaultEnvVars)
	if err != nil {
		return nil, err
	}
	jobs, err := NewJobComponentsBuilder(config, env, componentImages, defaultEnvVars).JobComponents()
	if err != nil {
		return nil, err
	}
	radixDeployment := constructRadixDeployment(config, env, jobName, imageTag, branch, commitID, gitTags, components, jobs)
	return radixDeployment, nil
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
		return fmt.Errorf("requeue, status was restored for active deployment, %s, and we need to trigger a new sync", deploy.getName())
	}

	stopReconciliation, err := deploy.syncStatuses()
	if err != nil {
		return err
	}
	if stopReconciliation {
		log.Infof("stop reconciliation, status updated triggering new sync")
		return nil
	}

	if err := deploy.syncDeployment(); err != nil {
		return err
	}

	deploy.maintainHistoryLimit()
	metrics.RequestedResources(deploy.registration, deploy.radixDeployment)
	return nil
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
		err = fmt.Errorf("failed to get all RadixDeployments. Error was %v", err)
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
		return fmt.Errorf("failed to perform garbage collection of removed components: %v", err)
	}

	if err := deploy.garbageCollectAuxiliaryResources(); err != nil {
		return fmt.Errorf("failed to perform auxiliary resource garbage collection: %v", err)
	}

	if err := deploy.configureRbac(); err != nil {
		return err
	}

	err = deploy.createOrUpdateSecrets()
	if err != nil {
		return fmt.Errorf("failed to provision secrets: %v", err)
	}

	errs := deploy.setDefaultNetworkPolicies()

	// If any error occurred when setting network policies
	if len(errs) > 0 {
		combinedErrs := errors.Concat(errs)
		log.Errorf("%s", combinedErrs)
		return combinedErrs
	}

	for _, component := range deploy.radixDeployment.Spec.Components {
		if err := deploy.syncDeploymentForRadixComponent(&component); err != nil {
			errs = append(errs, err)
		}
	}

	for _, jobComponent := range deploy.radixDeployment.Spec.Jobs {
		jobSchedulerComponent := newJobSchedulerComponent(&jobComponent, deploy.radixDeployment)
		if err := deploy.syncDeploymentForRadixComponent(jobSchedulerComponent); err != nil {
			errs = append(errs, err)
		}
	}

	// If any error occurred when syncing of components
	if len(errs) > 0 {
		return errors.Concat(errs)
	}

	if err := deploy.syncAuxiliaryResources(); err != nil {
		return fmt.Errorf("failed to sync auxiliary resource : %v", err)
	}

	return nil
}

func (deploy *Deployment) syncAuxiliaryResources() error {
	for _, aux := range deploy.auxResourceManagers {
		if err := aux.Sync(); err != nil {
			return err
		}
	}
	return nil
}

func (deploy *Deployment) configureRbac() error {
	rbacFunc := GetDeploymentRbacConfigurators(deploy)
	for _, rbac := range rbacFunc {
		if err := rbac(); err != nil {
			return err
		}
	}
	return nil
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
		currentRD, err := rdInterface.Get(context.TODO(), rd.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}
		changeStatusFunc(&currentRD.Status)
		_, err = rdInterface.UpdateStatus(context.TODO(), currentRD, metav1.UpdateOptions{})

		if err == nil && rd.GetName() == deploy.radixDeployment.GetName() {
			currentRD, err = rdInterface.Get(context.TODO(), rd.GetName(), metav1.GetOptions{})
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

	err = deploy.garbageCollectPodDisruptionBudgetsNoLongerInSpec()
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

	err = deploy.garbageCollectScheduledJobsNoLongerInSpec()
	if err != nil {
		return err
	}

	err = deploy.garbageCollectRadixBatchesNoLongerInSpec()
	if err != nil {
		return err
	}

	err = deploy.garbageCollectConfigMapsNoLongerInSpec()
	if err != nil {
		return err
	}

	err = deploy.garbageCollectServiceAccountNoLongerInSpec()
	if err != nil {
		return err
	}

	return nil
}

func (deploy *Deployment) garbageCollectAuxiliaryResources() error {
	for _, aux := range deploy.auxResourceManagers {
		if err := aux.GarbageCollect(); err != nil {
			return err
		}
	}
	return nil
}

func constructRadixDeployment(radixApplication *v1.RadixApplication, env, jobName, imageTag, branch, commitID, gitTags string, components []v1.RadixDeployComponent, jobs []v1.RadixDeployJobComponent) *v1.RadixDeployment {
	appName := radixApplication.GetName()
	deployName := utils.GetDeploymentName(appName, env, imageTag)
	imagePullSecrets := []corev1.LocalObjectReference{}
	if len(radixApplication.Spec.PrivateImageHubs) > 0 {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: defaults.PrivateImageHubSecretName})
	}

	radixDeployment := &v1.RadixDeployment{
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
				kube.RadixBranchAnnotation:  branch,
				kube.RadixGitTagsAnnotation: gitTags,
				kube.RadixCommitAnnotation:  commitID,
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

func getLabelSelectorForCsiAzureVolumeMountSecret(component v1.RadixCommonDeployComponent) string {
	return fmt.Sprintf("%s=%s, %s in (%s, %s, %s)", kube.RadixComponentLabel, component.GetName(), kube.RadixMountTypeLabel, string(v1.MountTypeBlobCsiAzure), string(v1.MountTypeBlob2CsiAzure), string(v1.MountTypeFileCsiAzure))
}

func (deploy *Deployment) maintainHistoryLimit() {
	historyLimit := strings.TrimSpace(os.Getenv(defaults.DeploymentsHistoryLimitEnvironmentVariable))
	if historyLimit == "" {
		return
	}

	limit, err := strconv.Atoi(historyLimit)
	if err != nil {
		log.Warnf("%s is not set to a proper number, %s, and cannot be parsed.", defaults.DeploymentsHistoryLimitEnvironmentVariable, historyLimit)
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
		err := deploy.radixclient.RadixV1().RadixDeployments(deploy.getNamespace()).Delete(context.TODO(), deployments[i].Name, metav1.DeleteOptions{})
		if err != nil {
			log.Warnf("failed to delete old deployment %s: %v", deployments[i].Name, err)
		}
	}
}

func (deploy *Deployment) syncDeploymentForRadixComponent(component v1.RadixCommonDeployComponent) error {
	err := deploy.createOrUpdateEnvironmentVariableConfigMaps(component)
	if err != nil {
		return err
	}

	err = deploy.createOrUpdateServiceAccount(component)
	if err != nil {
		return fmt.Errorf("failed to create service account: %w", err)
	}

	err = deploy.createOrUpdateDeployment(component)
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	err = deploy.createOrUpdateHPA(component)
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	err = deploy.createOrUpdateService(component)
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	err = deploy.createOrUpdatePodDisruptionBudget(component)
	if err != nil {
		return fmt.Errorf("failed to create PDB: %w", err)
	}

	err = deploy.garbageCollectPodDisruptionBudgetNoLongerInSpecForComponent(component)
	if err != nil {
		return fmt.Errorf("failed to garbage collect PDB: %w", err)
	}

	err = deploy.garbageCollectServiceAccountNoLongerInSpecForComponent(component)
	if err != nil {
		return fmt.Errorf("failed to garbage collect service account: %w", err)
	}

	if component.IsPublic() {
		err = deploy.createOrUpdateIngress(component)
		if err != nil {
			return fmt.Errorf("failed to create ingress: %w", err)
		}
	} else {
		err = deploy.garbageCollectIngressNoLongerInSpecForComponent(component)
		if err != nil {
			return fmt.Errorf("failed to delete ingress: %w", err)
		}
	}

	if component.GetMonitoring() {
		err = deploy.createOrUpdateServiceMonitor(component)
		if err != nil {
			return fmt.Errorf("failed to create service monitor: %w", err)
		}
	} else {
		err = deploy.deleteServiceMonitorForComponent(component)
		if err != nil {
			return fmt.Errorf("failed to delete servicemonitor: %w", err)
		}
	}

	return nil
}

func (deploy *Deployment) createOrUpdateJobStub(deployComponent v1.RadixCommonDeployComponent, desiredDeployment *appsv1.Deployment) (*appsv1.Deployment, *appsv1.Deployment, error) {
	currentJobStubDeployment, err := deploy.kubeutil.GetDeployment(desiredDeployment.Namespace, getJobStubName(desiredDeployment.Name))
	var desiredJobStubDeployment *appsv1.Deployment
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			return nil, nil, err
		}
		currentJobStubDeployment = nil
		if desiredDeployment == nil {
			return nil, nil, nil
		}
		desiredJobStubDeployment, err = deploy.createJobStubDeployment(deployComponent)
		if err != nil {
			return nil, nil, err
		}
	} else {
		desiredJobStubDeployment = currentJobStubDeployment.DeepCopy()
	}
	if desiredDeployment == nil {
		return currentJobStubDeployment, nil, nil
	}
	desiredJobStubDeployment.Spec.Template.Spec.Volumes = desiredDeployment.Spec.Template.Spec.Volumes
	desiredDeployment.Spec.Template.Spec.Volumes = nil
	return currentJobStubDeployment, desiredJobStubDeployment, nil
}

func getJobStubName(jobName string) string {
	return fmt.Sprintf("%s-stub", jobName)
}

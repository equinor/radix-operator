package deployment

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
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
	ingressAnnotationProviders []ingress.AnnotationProvider
	tenantId                   string
	kubernetesApiPort          int32
	deploymentHistoryLimit     int
}

// Test if NewDeploymentSyncer implements DeploymentSyncerFactory
var _ DeploymentSyncerFactory = DeploymentSyncerFactoryFunc(NewDeploymentSyncer)

// NewDeploymentSyncer Constructor
func NewDeploymentSyncer(kubeclient kubernetes.Interface, kubeutil *kube.Kube, radixclient radixclient.Interface, prometheusperatorclient monitoring.Interface, registration *v1.RadixRegistration, radixDeployment *v1.RadixDeployment, tenantId string, kubernetesApiPort int32, deploymentHistoryLimit int, ingressAnnotationProviders []ingress.AnnotationProvider, auxResourceManagers []AuxiliaryResourceManager) DeploymentSyncer {
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
		deploymentHistoryLimit:     deploymentHistoryLimit,
	}
}

// GetDeploymentComponent Gets the index of and the component given name. Used by Radix API.
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
func ConstructForTargetEnvironment(config *v1.RadixApplication, jobName, imageTag, branch string, componentImages pipeline.DeployComponentImages, env string, defaultEnvVars v1.EnvVarsMap, radixConfigHash, buildSecretHash string, preservingDeployComponents []v1.RadixDeployComponent, preservingDeployJobComponents []v1.RadixDeployJobComponent) (*v1.RadixDeployment, error) {
	commitID := defaultEnvVars[defaults.RadixCommitHashEnvironmentVariable]
	gitTags := defaultEnvVars[defaults.RadixGitTagsEnvironmentVariable]
	deployComponents, err := GetRadixComponentsForEnv(config, env, componentImages, defaultEnvVars, preservingDeployComponents)
	if err != nil {
		return nil, err
	}
	jobs, err := NewJobComponentsBuilder(config, env, componentImages, defaultEnvVars, preservingDeployJobComponents).JobComponents()
	if err != nil {
		return nil, err
	}
	radixDeployment := constructRadixDeployment(config, env, jobName, imageTag, branch, commitID, gitTags, deployComponents, jobs, radixConfigHash, buildSecretHash)
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

	deploy.maintainHistoryLimit(deploy.deploymentHistoryLimit)
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
		combinedErrs := stderrors.Join(errs...)
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
		return stderrors.Join(errs...)
	}

	if err := deploy.syncAuxiliaryResources(); err != nil {
		return fmt.Errorf("failed to sync auxiliary resource : %v", err)
	}

	return nil
}

func (deploy *Deployment) syncAuxiliaryResources() error {
	for _, aux := range deploy.auxResourceManagers {
		log.Debugf("sync AuxiliaryResource for the RadixDeployment %s", deploy.radixDeployment.GetName())
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

	err = deploy.garbageCollectScheduledJobAuxDeploymentsNoLongerInSpec()
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

func constructRadixDeployment(radixApplication *v1.RadixApplication, env, jobName, imageTag, branch, commitID, gitTags string, components []v1.RadixDeployComponent, jobs []v1.RadixDeployJobComponent, radixConfigHash, buildSecretHash string) *v1.RadixDeployment {
	appName := radixApplication.GetName()
	deployName := utils.GetDeploymentName(appName, env, imageTag)
	imagePullSecrets := []corev1.LocalObjectReference{}
	if len(radixApplication.Spec.PrivateImageHubs) > 0 {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: defaults.PrivateImageHubSecretName})
	}
	annotations := map[string]string{
		kube.RadixBranchAnnotation:  branch,
		kube.RadixGitTagsAnnotation: gitTags,
		kube.RadixCommitAnnotation:  commitID,
		kube.RadixBuildSecretHash:   buildSecretHash,
		kube.RadixConfigHash:        radixConfigHash,
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
			Annotations: annotations,
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
	return fmt.Sprintf("%s=%s, %s in (%s, %s, %s, %s)", kube.RadixComponentLabel, component.GetName(), kube.RadixMountTypeLabel, string(v1.MountTypeBlobFuse2FuseCsiAzure), string(v1.MountTypeBlobFuse2Fuse2CsiAzure), string(v1.MountTypeBlobFuse2NfsCsiAzure), string(v1.MountTypeAzureFileCsiAzure))
}

func (deploy *Deployment) maintainHistoryLimit(deploymentHistoryLimit int) {
	if deploymentHistoryLimit <= 0 {
		return
	}

	deployments, err := deploy.kubeutil.ListRadixDeployments(deploy.getNamespace())
	if err != nil {
		log.Warnf("failed to get all Radix deployments. Error was %v", err)
		return
	}
	numToDelete := len(deployments) - deploymentHistoryLimit
	if numToDelete <= 0 {
		return
	}

	radixDeploymentsReferencedByJobs, err := deploy.getRadixDeploymentsReferencedByJobs()
	if err != nil {
		log.Warnf("failed to get all Radix batches. Error was %v", err)
		return
	}
	deployments = sortRDsByActiveFromTimestampAsc(deployments)
	for _, deployment := range deployments[:numToDelete] {
		if _, ok := radixDeploymentsReferencedByJobs[deployment.Name]; ok {
			log.Infof("Not deleting deployment %s as it is referenced by scheduled jobs", deployment.Name)
			continue
		}
		log.Infof("Removing deployment %s from %s", deployment.Name, deployment.Namespace)
		err := deploy.radixclient.RadixV1().RadixDeployments(deploy.getNamespace()).Delete(context.TODO(), deployment.Name, metav1.DeleteOptions{})
		if err != nil {
			log.Warnf("failed to delete old deployment %s: %v", deployment.Name, err)
		}
	}
}

func (deploy *Deployment) getRadixDeploymentsReferencedByJobs() (map[string]bool, error) {
	radixBatches, err := deploy.kubeutil.ListRadixBatches(deploy.getNamespace())
	if err != nil {
		return nil, err
	}
	radixDeploymentsReferencedByJobs := slice.Reduce(radixBatches, make(map[string]bool), func(acc map[string]bool, radixBatch *v1.RadixBatch) map[string]bool {
		acc[radixBatch.Spec.RadixDeploymentJobRef.Name] = true
		return acc
	})
	return radixDeploymentsReferencedByJobs, nil
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
		return fmt.Errorf("failed to create hpa: %w", err)
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

func (deploy *Deployment) createOrUpdateJobAuxDeployment(deployComponent v1.RadixCommonDeployComponent, desiredDeployment *appsv1.Deployment) (*appsv1.Deployment, *appsv1.Deployment, error) {
	currentJobAuxDeployment, desiredJobAuxDeployment, err := deploy.getCurrentAndDesiredJobAuxDeployment(deployComponent, desiredDeployment)
	if err != nil {
		return nil, nil, err
	}
	desiredJobAuxDeployment.ObjectMeta.Labels = deploy.getJobAuxDeploymentLabels(deployComponent)
	desiredJobAuxDeployment.Spec.Template.Labels = deploy.getJobAuxDeploymentPodLabels(deployComponent)
	desiredJobAuxDeployment.Spec.Template.Spec.ServiceAccountName = (&radixComponentServiceAccountSpec{component: deployComponent}).ServiceAccountName()
	// Copy volumes and volume mounts from desired deployment to job aux deployment
	desiredJobAuxDeployment.Spec.Template.Spec.Volumes = desiredDeployment.Spec.Template.Spec.Volumes
	desiredJobAuxDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = desiredDeployment.Spec.Template.Spec.Containers[0].VolumeMounts
	// Remove volumes and volume mounts from job scheduler deployment
	desiredDeployment.Spec.Template.Spec.Volumes = nil
	desiredDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = nil

	return currentJobAuxDeployment, desiredJobAuxDeployment, nil
}

func (deploy *Deployment) getCurrentAndDesiredJobAuxDeployment(deployComponent v1.RadixCommonDeployComponent, desiredDeployment *appsv1.Deployment) (*appsv1.Deployment, *appsv1.Deployment, error) {
	currentJobAuxDeployment, err := deploy.kubeutil.GetDeployment(desiredDeployment.Namespace, getJobAuxObjectName(desiredDeployment.Name))
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil, deploy.createJobAuxDeployment(deployComponent), nil
		}
		return nil, nil, err
	}
	return currentJobAuxDeployment, currentJobAuxDeployment.DeepCopy(), nil
}

func getJobAuxObjectName(jobName string) string {
	return fmt.Sprintf("%s-aux", jobName)
}

package deployment

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	certclient "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	internal "github.com/equinor/radix-operator/pkg/apis/internal/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/metrics"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/volumemount"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeploymentSyncer defines interface for syncing a RadixDeployment
type DeploymentSyncer interface {
	OnSync(ctx context.Context) error
}

// Deployment Instance variables
type Deployment struct {
	kubeclient                 kubernetes.Interface
	radixclient                radixclient.Interface
	kubeutil                   *kube.Kube
	dynamicClient              client.Client
	certClient                 certclient.Interface
	registration               *v1.RadixRegistration
	radixDeployment            *v1.RadixDeployment
	auxResourceManagers        []AuxiliaryResourceManager
	ingressAnnotationProviders []ingress.AnnotationProvider
	config                     *config.Config
}

// Test if NewDeploymentSyncer implements DeploymentSyncerFactory
var _ DeploymentSyncerFactory = DeploymentSyncerFactoryFunc(NewDeploymentSyncer)

// NewDeploymentSyncer Constructor
func NewDeploymentSyncer(kubeclient kubernetes.Interface, kubeutil *kube.Kube, radixclient radixclient.Interface, dynamicClient client.Client, certClient certclient.Interface, registration *v1.RadixRegistration, radixDeployment *v1.RadixDeployment, ingressAnnotationProviders []ingress.AnnotationProvider, auxResourceManagers []AuxiliaryResourceManager, config *config.Config) DeploymentSyncer {
	return &Deployment{
		kubeclient:                 kubeclient,
		radixclient:                radixclient,
		kubeutil:                   kubeutil,
		dynamicClient:              dynamicClient,
		certClient:                 certClient,
		registration:               registration,
		radixDeployment:            radixDeployment,
		auxResourceManagers:        auxResourceManagers,
		ingressAnnotationProviders: ingressAnnotationProviders,
		config:                     config,
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

// OnSync compares the actual state with the desired, and attempts to
// converge the two
func (deploy *Deployment) OnSync(ctx context.Context) error {
	ctx = log.Ctx(ctx).With().Str("resource_kind", v1.KindRadixDeployment).Logger().WithContext(ctx)
	log.Ctx(ctx).Info().Msg("Syncing")

	if deploy.radixDeployment.Status.Condition == v1.DeploymentInactive {
		log.Ctx(ctx).Debug().Msg("Ignoring RadixDeployment as it is inactive")
		return nil
	}

	if isActiveCondition, err := deploy.trySetActiveCondition(ctx); err != nil {
		return err
	} else if !isActiveCondition {
		log.Ctx(ctx).Info().Msgf("stop reconciliation, status updated triggering new sync")
		return nil
	}

	if err := deploy.syncStatus(ctx, deploy.syncDeployment(ctx)); err != nil {
		return err
	}

	deploy.maintainHistoryLimit(ctx, deploy.config.DeploymentSyncer.DeploymentHistoryLimit)
	return metrics.RequestedResources(deploy.registration, deploy.radixDeployment)
}

func (deploy *Deployment) syncStatus(ctx context.Context, reconcileErr error) error {
	err := deploy.updateStatus(ctx, func(currStatus *v1.RadixDeployStatus) {
		currStatus.Reconciled = metav1.Now()
		currStatus.ObservedGeneration = deploy.radixDeployment.Generation

		if reconcileErr != nil {
			currStatus.ReconcileStatus = v1.RadixDeploymentReconcileFailed
			currStatus.Message = reconcileErr.Error()
		} else {
			currStatus.ReconcileStatus = v1.RadixDeploymentReconcileSucceeded
			currStatus.Message = ""
		}
	})
	if err != nil {
		return fmt.Errorf("failed to sync status: %w", err)
	}

	return reconcileErr
}

// trySetActiveCondition sets RadixDeployment condition to active if it is the latest, and sets all other RDs in the namespace to inactive.
// It returns true if the RadixDeployment is active.
func (deploy *Deployment) trySetActiveCondition(ctx context.Context) (bool, error) {
	allRDs, err := deploy.kubeutil.ListRadixDeployments(ctx, deploy.radixDeployment.Namespace)
	if err != nil {
		return false, fmt.Errorf("failed to list RadixDeployments: %w", err)
	}

	if !deploy.isLatestInTheEnvironment(allRDs) {
		log.Ctx(ctx).Info().Msgf("RadixDeployment %s was not the latest. Ignoring", deploy.radixDeployment.Name)
		return false, nil
	}

	err = deploy.updateStatus(ctx, func(currStatus *v1.RadixDeployStatus) {
		if currStatus.Condition != v1.DeploymentActive {
			currStatus.ActiveFrom = metav1.NewTime(time.Now().UTC())
		}
		currStatus.Condition = v1.DeploymentActive
	})
	if err != nil {
		return false, fmt.Errorf("failed to set RadixDeployment %s to active: %w", deploy.radixDeployment.Name, err)
	}

	if err := deploy.setOtherRDsToInactive(ctx, allRDs); err != nil {
		return false, fmt.Errorf("failed to set old RadixDeployments to inactive: %w", err)
	}

	return true, nil
}

func (deploy *Deployment) syncDeployment(ctx context.Context) error {
	// can garbageCollectComponentsNoLongerInSpec be moved to syncComponents()?
	err := deploy.garbageCollectComponentsNoLongerInSpec(ctx)
	if err != nil {
		return fmt.Errorf("failed to perform garbage collection of removed components: %w", err)
	}

	if err := deploy.garbageCollectAuxiliaryResources(ctx); err != nil {
		return fmt.Errorf("failed to perform auxiliary resource garbage collection: %w", err)
	}

	if err := deploy.configureRbac(ctx); err != nil {
		return fmt.Errorf("failed to configure rbac: %w", err)
	}

	if err := deploy.syncSecrets(ctx); err != nil {
		return fmt.Errorf("failed to provision secrets: %w", err)
	}

	if err := deploy.setDefaultNetworkPolicies(ctx); err != nil {
		return fmt.Errorf("failed to set default network policies: %w", err)
	}

	var errs []error
	for _, component := range deploy.radixDeployment.Spec.Components {
		ctx := log.Ctx(ctx).With().Str("component", component.Name).Logger().WithContext(ctx)
		if err := deploy.syncDeploymentForRadixComponent(ctx, &component); err != nil {
			errs = append(errs, err)
		}
	}
	for _, jobComponent := range deploy.radixDeployment.Spec.Jobs {
		ctx := log.Ctx(ctx).With().Str("jobComponent", jobComponent.Name).Logger().WithContext(ctx)
		jobSchedulerComponent := internal.NewJobSchedulerComponent(&jobComponent, deploy.radixDeployment)
		if err := deploy.syncDeploymentForRadixComponent(ctx, jobSchedulerComponent); err != nil {
			errs = append(errs, err)
		}
	}
	// If any error occurred when syncing of components
	if len(errs) > 0 {
		return fmt.Errorf("failed to sync deployments: %w", errors.Join(errs...))
	}

	if err := deploy.syncExternalDnsResources(ctx); err != nil {
		return fmt.Errorf("failed to sync external DNS resources: %w", err)
	}

	if err := deploy.syncAuxiliaryResources(ctx); err != nil {
		return fmt.Errorf("failed to sync auxiliary resource : %w", err)
	}

	return nil
}

func (deploy *Deployment) syncAuxiliaryResources(ctx context.Context) error {
	for _, aux := range deploy.auxResourceManagers {
		log.Ctx(ctx).Debug().Msgf("sync AuxiliaryResource for the RadixDeployment %s", deploy.radixDeployment.GetName())
		if err := aux.Sync(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (deploy *Deployment) configureRbac(ctx context.Context) error {
	rbacFunc := GetDeploymentRbacConfigurators(ctx, deploy)
	for _, rbac := range rbacFunc {
		if err := rbac(); err != nil {
			return err
		}
	}
	return nil
}

func (deploy *Deployment) updateStatus(ctx context.Context, changeStatusFunc func(currStatus *v1.RadixDeployStatus)) error {
	updatedRD, err := updateRadixDeploymentStatus(ctx, deploy.radixclient, deploy.radixDeployment, changeStatusFunc)
	if err != nil {
		return err
	}
	deploy.radixDeployment = updatedRD
	return nil
}

func (deploy *Deployment) setOtherRDsToInactive(ctx context.Context, allRDs []*v1.RadixDeployment) error {
	sortedRDs := sortRDsByActiveFromTimestampDesc(allRDs)
	if len(sortedRDs) == 0 {
		return nil
	}
	prevRD := sortedRDs[0]

	for _, rd := range sortedRDs {
		if strings.EqualFold(rd.GetName(), deploy.radixDeployment.Name) {
			prevRD = deploy.radixDeployment
			continue
		}

		if rd.Status.Condition != v1.DeploymentInactive {
			_, err := updateRadixDeploymentStatus(ctx, deploy.radixclient, rd, func(currStatus *v1.RadixDeployStatus) {
				currStatus.Condition = v1.DeploymentInactive
				currStatus.ActiveTo = getActiveFrom(prevRD)
				currStatus.ActiveFrom = getActiveFrom(rd)
			})
			if err != nil {
				return err
			}
		}

		prevRD = rd
	}
	return nil
}

func (deploy *Deployment) garbageCollectComponentsNoLongerInSpec(ctx context.Context) error {
	err := deploy.garbageCollectDeploymentsNoLongerInSpec(ctx)
	if err != nil {
		return err
	}

	err = deploy.garbageCollectDeprecatedHPAs(ctx)
	if err != nil {
		return err
	}

	err = deploy.garbageCollectScalersNoLongerInSpec(ctx)
	if err != nil {
		return err
	}

	err = deploy.garbageCollectTriggerAuthsNoLongerInSpec(ctx)
	if err != nil {
		return err
	}

	err = deploy.garbageCollectServicesNoLongerInSpec(ctx)
	if err != nil {
		return err
	}

	err = deploy.garbageCollectIngressesNoLongerInSpec(ctx)
	if err != nil {
		return err
	}

	err = deploy.garbageCollectPodDisruptionBudgetsNoLongerInSpec(ctx)
	if err != nil {
		return err
	}

	err = deploy.garbageCollectSecretsNoLongerInSpec(ctx)
	if err != nil {
		return err
	}

	err = deploy.garbageCollectRoleBindingsNoLongerInSpec(ctx)
	if err != nil {
		return err
	}

	err = deploy.garbageCollectServiceMonitorsNoLongerInSpec(ctx)
	if err != nil {
		return err
	}

	err = deploy.garbageCollectScheduledJobsNoLongerInSpec(ctx)
	if err != nil {
		return err
	}

	err = deploy.garbageCollectScheduledJobAuxDeploymentsNoLongerInSpec(ctx)
	if err != nil {
		return err
	}

	err = deploy.garbageCollectRadixBatchesNoLongerInSpec(ctx)
	if err != nil {
		return err
	}

	err = deploy.garbageCollectConfigMapsNoLongerInSpec(ctx)
	if err != nil {
		return err
	}

	err = deploy.garbageCollectServiceAccountNoLongerInSpec(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (deploy *Deployment) garbageCollectAuxiliaryResources(ctx context.Context) error {
	for _, aux := range deploy.auxResourceManagers {
		if err := aux.GarbageCollect(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (deploy *Deployment) maintainHistoryLimit(ctx context.Context, deploymentHistoryLimit int) {
	if deploymentHistoryLimit <= 0 {
		return
	}

	deployments, err := deploy.kubeutil.ListRadixDeployments(ctx, deploy.radixDeployment.Namespace)
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Msg("Failed to list RadixDeployments")
		return
	}
	numToDelete := len(deployments) - deploymentHistoryLimit
	if numToDelete <= 0 {
		return
	}

	radixDeploymentsReferencedByJobs, err := deploy.getRadixDeploymentsReferencedByJobs(ctx)
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Msg("failed to list RadixBatches")
		return
	}
	deployments = sortRDsByActiveFromTimestampAsc(deployments)
	for _, deployment := range deployments[:numToDelete] {
		if _, ok := radixDeploymentsReferencedByJobs[deployment.Name]; ok {
			log.Ctx(ctx).Info().Msgf("Not deleting deployment %s as it is referenced by scheduled jobs", deployment.Name)
			continue
		}
		if deployment.IsActive() {
			log.Ctx(ctx).Info().Msgf("Ignoring active RadixDeployment %s", deployment.Name)
			continue
		}
		log.Ctx(ctx).Info().Msgf("Removing deployment %s from %s", deployment.Name, deployment.Namespace)
		err := deploy.radixclient.RadixV1().RadixDeployments(deploy.radixDeployment.Namespace).Delete(ctx, deployment.Name, metav1.DeleteOptions{})
		if err != nil {
			log.Ctx(ctx).Warn().Err(err).Msgf("Failed to delete old RadixDeployment %s", deployment.Name)
		}
	}
}

func (deploy *Deployment) getRadixDeploymentsReferencedByJobs(ctx context.Context) (map[string]bool, error) {
	radixBatches, err := deploy.kubeutil.ListRadixBatches(ctx, deploy.radixDeployment.Namespace)
	if err != nil {
		return nil, err
	}
	radixDeploymentsReferencedByJobs := slice.Reduce(radixBatches, make(map[string]bool), func(acc map[string]bool, radixBatch *v1.RadixBatch) map[string]bool {
		acc[radixBatch.Spec.RadixDeploymentJobRef.Name] = true
		return acc
	})
	return radixDeploymentsReferencedByJobs, nil
}

func (deploy *Deployment) syncDeploymentForRadixComponent(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	err := deploy.createOrUpdateEnvironmentVariableConfigMaps(ctx, component)
	if err != nil {
		return err
	}

	err = deploy.createOrUpdateServiceAccount(ctx, component)
	if err != nil {
		return fmt.Errorf("failed to create service account: %w", err)
	}

	err = deploy.reconcileDeployComponent(ctx, component)
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	err = deploy.createOrUpdateScaledObject(ctx, component)
	if err != nil {
		return fmt.Errorf("failed to create hpa: %w", err)
	}

	err = deploy.createOrUpdateService(ctx, component)
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	err = deploy.createOrUpdatePodDisruptionBudget(ctx, component)
	if err != nil {
		return fmt.Errorf("failed to create PDB: %w", err)
	}

	err = deploy.garbageCollectPodDisruptionBudgetNoLongerInSpecForComponent(ctx, component)
	if err != nil {
		return fmt.Errorf("failed to garbage collect PDB: %w", err)
	}

	err = deploy.garbageCollectServiceAccountNoLongerInSpecForComponent(ctx, component)
	if err != nil {
		return fmt.Errorf("failed to garbage collect service account: %w", err)
	}

	if err := deploy.reconcileIngresses(ctx, component); err != nil {
		return err
	}

	if component.GetMonitoring() {
		err = deploy.createOrUpdateServiceMonitor(ctx, component)
		if err != nil {
			return fmt.Errorf("failed to create service monitor: %w", err)
		}
	} else {
		err = deploy.deleteServiceMonitorForComponent(ctx, component)
		if err != nil {
			return fmt.Errorf("failed to delete servicemonitor: %w", err)
		}
	}

	if err = volumemount.GarbageCollectCsiAzureVolumeResourcesForDeployComponent(ctx, deploy.kubeutil.KubeClient(), deploy.radixDeployment, deploy.radixDeployment.GetNamespace()); err != nil {
		return fmt.Errorf("failed to garbage collect persistent volumes or persistent volume claims: %w", err)
	}

	return nil
}

func (deploy *Deployment) createOrUpdateJobAuxDeployment(ctx context.Context, deployComponent v1.RadixCommonDeployComponent, namespace, jobKubeDeploymentName string, volumes []corev1.Volume, volumeMounts []corev1.VolumeMount) (*appsv1.Deployment, *appsv1.Deployment, error) {
	currentJobAuxDeployment, desiredJobAuxDeployment, err := deploy.getCurrentAndDesiredJobAuxDeployment(ctx, namespace, jobKubeDeploymentName)
	if err != nil {
		return nil, nil, err
	}
	desiredJobAuxDeployment.ObjectMeta.Labels = deploy.getJobAuxDeploymentLabels(deployComponent)
	desiredJobAuxDeployment.Spec.Selector.MatchLabels = deploy.getJobAuxDeploymentPodLabels(deployComponent)
	desiredJobAuxDeployment.Spec.Template.Labels = deploy.getJobAuxDeploymentPodLabels(deployComponent)
	desiredJobAuxDeployment.Spec.Template.Spec.ServiceAccountName = (&radixComponentServiceAccountSpec{component: deployComponent}).ServiceAccountName()
	desiredJobAuxDeployment.Spec.Template.Spec.Affinity = utils.GetAffinityForJobAPIAuxComponent()
	// Move volumes and volume mounts from desired deployment to job aux deployment
	desiredJobAuxDeployment.Spec.Template.Spec.Volumes = volumes
	desiredJobAuxDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts

	syncRadixRestartEnvironmentVariable(deployComponent, desiredJobAuxDeployment)
	return currentJobAuxDeployment, desiredJobAuxDeployment, nil
}

func (deploy *Deployment) getCurrentAndDesiredJobAuxDeployment(ctx context.Context, namespace, jobKubeDeploymentName string) (*appsv1.Deployment, *appsv1.Deployment, error) {
	jobAuxKubeDeploymentName := defaults.GetJobAuxKubeDeployName(jobKubeDeploymentName)
	desiredJobAuxDeployment := deploy.createJobAuxDeployment(jobKubeDeploymentName, jobAuxKubeDeploymentName)
	currentJobAuxDeployment, err := deploy.kubeutil.KubeClient().AppsV1().Deployments(namespace).Get(ctx, jobAuxKubeDeploymentName, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil, desiredJobAuxDeployment, nil
		}
		return nil, nil, err
	}
	return currentJobAuxDeployment, desiredJobAuxDeployment, nil
}

func getLabelSelectorForComponent(component v1.RadixCommonDeployComponent) string {
	return fmt.Sprintf("%s=%s", kube.RadixComponentLabel, component.GetName())
}

// isLatestInTheEnvironment Checks if the deployment is the latest in the same namespace as specified in the deployment
func (deploy *Deployment) isLatestInTheEnvironment(allRDs []*v1.RadixDeployment) bool {
	return isLatest(deploy.radixDeployment, allRDs)
}

func updateRadixDeploymentStatus(ctx context.Context, client radixclient.Interface, rd *v1.RadixDeployment, changeStatusFunc func(currStatus *v1.RadixDeployStatus)) (*v1.RadixDeployment, error) {
	updateObj := rd.DeepCopy()
	changeStatusFunc(&updateObj.Status)
	return client.RadixV1().RadixDeployments(rd.GetNamespace()).UpdateStatus(ctx, updateObj, metav1.UpdateOptions{})
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

func syncRadixRestartEnvironmentVariable(deployComponent v1.RadixCommonDeployComponent, desiredJobAuxDeployment *appsv1.Deployment) {
	auxDeploymentEnvVars := desiredJobAuxDeployment.Spec.Template.Spec.Containers[0].Env
	if restartComponentValue, ok := deployComponent.GetEnvironmentVariables()[defaults.RadixRestartEnvironmentVariable]; ok {
		if index := slice.FindIndex(auxDeploymentEnvVars, func(envVar corev1.EnvVar) bool {
			return strings.EqualFold(envVar.Name, defaults.RadixRestartEnvironmentVariable)
		}); index >= 0 {
			desiredJobAuxDeployment.Spec.Template.Spec.Containers[0].Env[index].Value = restartComponentValue
			return
		}
		desiredJobAuxDeployment.Spec.Template.Spec.Containers[0].Env = append(auxDeploymentEnvVars, corev1.EnvVar{Name: defaults.RadixRestartEnvironmentVariable, Value: restartComponentValue})
	}
}

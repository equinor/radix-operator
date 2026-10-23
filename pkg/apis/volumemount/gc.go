package volumemount

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubelabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
)

// GarbageCollectVolumes deletes Radix owned PersistentVolumeClaims, and their PersistentVolumes, in the namespace of the active RadixDeployment,
// unless they are still needed. A PVC is kept if any of these apply:
//
//	K0: it was created less than gracePeriod ago. This covers stale informer caches, and resources created by another controller
//	    that are not yet used by a workload
//	K1: it matches the desired spec of a volume mount of a component or job in the active RadixDeployment
//	K2: it matches the desired spec of the job component of an unfinished RadixBatch, in the RadixDeployment it references
//	K3: it is referenced by the Pod template of an existing Kubernetes Job, Deployment, or ReplicaSet with replicas > 0,
//	    so a workload that has no Pods (e.g. scaled to zero, or not updated because the sync failed) can still create Pods
//	K4: it is mounted by a Pod that is not Succeeded or Failed
//
// Afterwards, Radix owned PVs in the namespace that are Released, Failed or Available are deleted, unless their claimRef refers to a kept PVC,
// or they were created less than gracePeriod ago.
// Blob data is never deleted, as all PVs use the reclaim policy Retain.
func GarbageCollectVolumes(ctx context.Context, kubeUtil *kube.Kube, activeRd *radixv1.RadixDeployment, now time.Time, gracePeriod time.Duration) error {
	gc := &volumeGarbageCollector{
		kubeUtil:   kubeUtil,
		kubeClient: kubeUtil.KubeClient(),
		activeRd:   activeRd,
		appName:    activeRd.Spec.AppName, //nolint:staticcheck
		namespace:  activeRd.GetNamespace(),
		keepAfter:  now.Add(-gracePeriod),
	}
	return gc.run(ctx)
}

type volumeGarbageCollector struct {
	kubeUtil   *kube.Kube
	kubeClient kubernetes.Interface
	activeRd   *radixv1.RadixDeployment
	appName    string
	namespace  string
	keepAfter  time.Time
}

type componentVolumeMountKey struct {
	componentName   string
	volumeMountName string
}

func (gc *volumeGarbageCollector) run(ctx context.Context) error {
	pvcs, err := gc.listRadixPvcs(ctx)
	if err != nil {
		return err
	}
	pvs, err := gc.listRadixPvs(ctx)
	if err != nil {
		return err
	}
	if len(pvcs) == 0 && len(pvs) == 0 {
		return nil
	}

	// Everything needed to decide what to keep is read before anything is deleted. Any error aborts the garbage collection.
	desired, err := gc.getDesiredVolumeResources(ctx)
	if err != nil {
		return err
	}
	usedClaimNames, err := gc.getClaimNamesUsedByJobsAndPods(ctx)
	if err != nil {
		return err
	}

	keptPvcs := make(map[string]corev1.PersistentVolumeClaim)
	var deletePvcs []corev1.PersistentVolumeClaim
	for _, pvc := range pvcs {
		keep, err := gc.keepPvc(ctx, pvc, desired, usedClaimNames)
		if err != nil {
			return err
		}
		if keep {
			keptPvcs[pvc.GetName()] = pvc
		} else {
			deletePvcs = append(deletePvcs, pvc)
		}
	}

	var errs []error
	for _, pvc := range deletePvcs {
		if err := gc.deletePvcAndBoundPv(ctx, pvc); err != nil {
			errs = append(errs, err)
		}
	}
	for _, pv := range pvs {
		if err := gc.deleteOrphanedPv(ctx, pv, keptPvcs); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// listRadixPvcs lists PVCs in the namespace, labelled for the app and with a volume mount name label
func (gc *volumeGarbageCollector) listRadixPvcs(ctx context.Context) ([]corev1.PersistentVolumeClaim, error) {
	selector := kubelabels.SelectorFromSet(kubelabels.Set{kube.RadixAppLabel: gc.appName}).Add(volumeMountNameLabelExists())
	pvcList, err := gc.kubeClient.CoreV1().PersistentVolumeClaims(gc.namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, fmt.Errorf("failed to list PersistentVolumeClaims: %w", err)
	}
	return pvcList.Items, nil
}

// listRadixPvs lists PVs labelled for the app and the namespace, and with a volume mount name label
func (gc *volumeGarbageCollector) listRadixPvs(ctx context.Context) ([]corev1.PersistentVolume, error) {
	selector := kubelabels.SelectorFromSet(kubelabels.Set{kube.RadixAppLabel: gc.appName, kube.RadixNamespace: gc.namespace}).Add(volumeMountNameLabelExists())
	pvList, err := gc.kubeClient.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, fmt.Errorf("failed to list PersistentVolumes: %w", err)
	}
	return pvList.Items, nil
}

func volumeMountNameLabelExists() kubelabels.Requirement {
	requirement, err := kubelabels.NewRequirement(kube.RadixVolumeMountNameLabel, selection.Exists, nil)
	if err != nil {
		panic(err)
	}
	return *requirement
}

// getDesiredVolumeResources returns the desired PV/PVCs for the active RadixDeployment (K1) and for unfinished RadixBatches (K2)
func (gc *volumeGarbageCollector) getDesiredVolumeResources(ctx context.Context) (map[componentVolumeMountKey][]*desiredVolumeResources, error) {
	desired := make(map[componentVolumeMountKey][]*desiredVolumeResources)
	add := func(rd *radixv1.RadixDeployment, component radixv1.RadixCommonDeployComponent) error {
		for _, volumeMount := range component.GetVolumeMounts() {
			if !volumeMount.HasBlobFuse2() {
				continue
			}
			resources, err := buildDesiredVolumeResources(rd, component, volumeMount)
			if err != nil {
				return fmt.Errorf("failed to build desired storage for volume mount %s in component %s in RadixDeployment %s: %w", volumeMount.Name, component.GetName(), rd.GetName(), err)
			}
			key := componentVolumeMountKey{componentName: component.GetName(), volumeMountName: volumeMount.Name}
			desired[key] = append(desired[key], resources)
		}
		return nil
	}

	for _, component := range gc.activeRd.Spec.Components {
		if err := add(gc.activeRd, &component); err != nil {
			return nil, err
		}
	}
	for _, job := range gc.activeRd.Spec.Jobs {
		if err := add(gc.activeRd, &job); err != nil {
			return nil, err
		}
	}

	batches, err := gc.kubeUtil.ListRadixBatches(ctx, gc.namespace)
	if err != nil {
		return nil, err
	}
	rdsByName := map[string]*radixv1.RadixDeployment{gc.activeRd.GetName(): gc.activeRd}
	for _, batch := range batches {
		if !isRadixBatchUnfinished(batch) {
			continue
		}
		rdName := batch.Spec.RadixDeploymentJobRef.Name
		rd, ok := rdsByName[rdName]
		if !ok {
			rd, err = gc.kubeUtil.GetRadixDeployment(ctx, gc.namespace, rdName)
			if err != nil {
				if k8serrors.IsNotFound(err) {
					// The batch cannot create jobs without its RadixDeployment
					continue
				}
				return nil, fmt.Errorf("failed to get RadixDeployment %s for RadixBatch %s: %w", rdName, batch.GetName(), err)
			}
			rdsByName[rdName] = rd
		}
		job := rd.GetJobComponentByName(batch.Spec.RadixDeploymentJobRef.Job)
		if job == nil {
			continue
		}
		if err := add(rd, job); err != nil {
			return nil, err
		}
	}
	return desired, nil
}

// isRadixBatchUnfinished returns true if the batch is not Completed, or a job restart is pending
func isRadixBatchUnfinished(batch *radixv1.RadixBatch) bool {
	if batch.Status.Condition.Type != radixv1.BatchConditionTypeCompleted {
		return true
	}
	for _, job := range batch.Spec.Jobs {
		if job.Restart == "" {
			continue
		}
		restartHandled := false
		for _, jobStatus := range batch.Status.JobStatuses {
			if jobStatus.Name == job.Name && jobStatus.Restart == job.Restart {
				restartHandled = true
				break
			}
		}
		if !restartHandled {
			return true
		}
	}
	return false
}

// getClaimNamesUsedByJobsAndPods returns the claim names referenced by the Pod template of any Kubernetes Job,
// Deployment or ReplicaSet with replicas > 0 (K3), or mounted by a Pod that is not Succeeded or Failed (K4)
func (gc *volumeGarbageCollector) getClaimNamesUsedByJobsAndPods(ctx context.Context) (map[string]struct{}, error) {
	claimNames := make(map[string]struct{})
	jobList, err := gc.kubeClient.BatchV1().Jobs(gc.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list Jobs: %w", err)
	}
	for _, job := range jobList.Items {
		addClaimNames(claimNames, job.Spec.Template.Spec.Volumes)
	}

	deploymentList, err := gc.kubeClient.AppsV1().Deployments(gc.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list Deployments: %w", err)
	}
	for _, deployment := range deploymentList.Items {
		addClaimNames(claimNames, deployment.Spec.Template.Spec.Volumes)
	}

	replicaSetList, err := gc.kubeClient.AppsV1().ReplicaSets(gc.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list ReplicaSets: %w", err)
	}
	for _, replicaSet := range replicaSetList.Items {
		if replicaSet.Spec.Replicas != nil && *replicaSet.Spec.Replicas == 0 {
			continue
		}
		addClaimNames(claimNames, replicaSet.Spec.Template.Spec.Volumes)
	}

	podList, err := gc.kubeClient.CoreV1().Pods(gc.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list Pods: %w", err)
	}
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		addClaimNames(claimNames, pod.Spec.Volumes)
	}
	return claimNames, nil
}

func addClaimNames(claimNames map[string]struct{}, volumes []corev1.Volume) {
	for name := range getClaimNames(volumes) {
		claimNames[name] = struct{}{}
	}
}

func (gc *volumeGarbageCollector) keepPvc(ctx context.Context, pvc corev1.PersistentVolumeClaim, desired map[componentVolumeMountKey][]*desiredVolumeResources, usedClaimNames map[string]struct{}) (bool, error) {
	if pvc.GetCreationTimestamp().After(gc.keepAfter) {
		return true, nil
	}
	if _, ok := usedClaimNames[pvc.GetName()]; ok {
		return true, nil
	}

	key := componentVolumeMountKey{componentName: pvc.GetLabels()[kube.RadixComponentLabel], volumeMountName: pvc.GetLabels()[kube.RadixVolumeMountNameLabel]}
	candidates := desired[key]
	if len(candidates) == 0 || len(pvc.Spec.VolumeName) == 0 {
		return false, nil
	}
	pv, err := gc.kubeClient.CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to get PersistentVolume %s: %w", pvc.Spec.VolumeName, err)
	}
	for _, d := range candidates {
		if matchesDesired(pv, &pvc, d.pv, d.pvc) {
			return true, nil
		}
	}
	return false, nil
}

// deletePvcAndBoundPv deletes the PVC, then the PV it is bound to, if the PV is Radix owned and its claimRef refers to the PVC
func (gc *volumeGarbageCollector) deletePvcAndBoundPv(ctx context.Context, pvc corev1.PersistentVolumeClaim) error {
	if pvc.GetDeletionTimestamp() == nil {
		log.Ctx(ctx).Info().Msgf("Delete unused PersistentVolumeClaim %s in namespace %s", pvc.GetName(), pvc.GetNamespace())
		if err := gc.kubeClient.CoreV1().PersistentVolumeClaims(pvc.GetNamespace()).Delete(ctx, pvc.GetName(), metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &pvc.UID}}); err != nil && !k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete PersistentVolumeClaim %s: %w", pvc.GetName(), err)
		}
	}

	if len(pvc.Spec.VolumeName) == 0 {
		return nil
	}
	pv, err := gc.kubeClient.CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get PersistentVolume %s: %w", pvc.Spec.VolumeName, err)
	}
	if !gc.isRadixPvForNamespace(pv) || pv.Spec.ClaimRef == nil || pv.Spec.ClaimRef.Namespace != pvc.GetNamespace() || pv.Spec.ClaimRef.Name != pvc.GetName() {
		return nil
	}
	return gc.deletePv(ctx, pv)
}

// deleteOrphanedPv deletes a Radix owned PV that is Released, Failed or Available, unless it is new or its claimRef refers to a kept PVC.
// A claimRef refers to a kept PVC if the PVC is bound to the PV, or the claimRef uid is empty or the uid of the PVC.
// A Released PV with the uid of a deleted PVC is not kept by a new PVC with the same name, that is bound to another PV.
func (gc *volumeGarbageCollector) deleteOrphanedPv(ctx context.Context, pv corev1.PersistentVolume, keptPvcs map[string]corev1.PersistentVolumeClaim) error {
	if pv.GetDeletionTimestamp() != nil || pv.GetCreationTimestamp().After(gc.keepAfter) {
		return nil
	}
	switch pv.Status.Phase {
	case corev1.VolumeReleased, corev1.VolumeFailed, corev1.VolumeAvailable:
	default:
		return nil
	}
	if claimRef := pv.Spec.ClaimRef; claimRef != nil {
		if claimRef.Namespace != gc.namespace {
			return nil
		}
		if keptPvc, ok := keptPvcs[claimRef.Name]; ok && (keptPvc.Spec.VolumeName == pv.GetName() || len(claimRef.UID) == 0 || claimRef.UID == keptPvc.GetUID()) {
			return nil
		}
	}
	return gc.deletePv(ctx, &pv)
}

func (gc *volumeGarbageCollector) isRadixPvForNamespace(pv *corev1.PersistentVolume) bool {
	return pv.GetLabels()[kube.RadixAppLabel] == gc.appName && pv.GetLabels()[kube.RadixNamespace] == gc.namespace
}

func (gc *volumeGarbageCollector) deletePv(ctx context.Context, pv *corev1.PersistentVolume) error {
	log.Ctx(ctx).Info().Msgf("Delete unused PersistentVolume %s (phase %s) for namespace %s", pv.GetName(), pv.Status.Phase, gc.namespace)
	if err := gc.kubeClient.CoreV1().PersistentVolumes().Delete(ctx, pv.GetName(), metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete PersistentVolume %s: %w", pv.GetName(), err)
	}
	return nil
}

// DeletePersistentVolumesForDeletedNamespace deletes the Radix owned PersistentVolumes labelled for the namespace,
// when the namespace is being deleted or no longer exists. Nothing is deleted while the namespace exists and is not being deleted.
// PersistentVolumes are cluster scoped, and are therefore not deleted with the namespace.
// A PersistentVolume that is still bound to a PersistentVolumeClaim is protected by Kubernetes (kubernetes.io/pv-protection),
// and is removed when the PersistentVolumeClaim is deleted with the namespace. Blob data is never deleted, as all PVs use the reclaim policy Retain.
func DeletePersistentVolumesForDeletedNamespace(ctx context.Context, kubeClient kubernetes.Interface, namespace string) error {
	ns, err := kubeClient.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("failed to get namespace %s: %w", namespace, err)
	}
	if err == nil && ns.GetDeletionTimestamp() == nil {
		return nil
	}

	appLabelExists, err := kubelabels.NewRequirement(kube.RadixAppLabel, selection.Exists, nil)
	if err != nil {
		return err
	}
	selector := kubelabels.SelectorFromSet(kubelabels.Set{kube.RadixNamespace: namespace}).Add(*appLabelExists, volumeMountNameLabelExists())
	pvList, err := kubeClient.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return fmt.Errorf("failed to list PersistentVolumes: %w", err)
	}

	var errs []error
	for _, pv := range pvList.Items {
		if pv.GetDeletionTimestamp() != nil {
			continue
		}
		log.Ctx(ctx).Info().Msgf("Delete PersistentVolume %s (phase %s) for deleted namespace %s", pv.GetName(), pv.Status.Phase, namespace)
		if err := kubeClient.CoreV1().PersistentVolumes().Delete(ctx, pv.GetName(), metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("failed to delete PersistentVolume %s: %w", pv.GetName(), err))
		}
	}
	return errors.Join(errs...)
}

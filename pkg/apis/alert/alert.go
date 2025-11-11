package alert

import (
	"context"
	"fmt"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

// AlertSyncer defines interface for syncing a RadixAlert
type AlertSyncer interface {
	OnSync(ctx context.Context) error
}

type alertSyncer struct {
	kubeClient           kubernetes.Interface
	radixClient          radixclient.Interface
	kubeUtil             *kube.Kube
	prometheusClient     monitoring.Interface
	radixAlert           *radixv1.RadixAlert
	slackMessageTemplate slackMessageTemplate
	alertConfigs         AlertConfigs
}

// New creates a new alert syncer
func New(kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	prometheusperatorclient monitoring.Interface,
	radixAlert *radixv1.RadixAlert) AlertSyncer {

	return &alertSyncer{
		kubeClient:           kubeclient,
		radixClient:          radixclient,
		kubeUtil:             kubeutil,
		prometheusClient:     prometheusperatorclient,
		radixAlert:           radixAlert,
		slackMessageTemplate: defaultSlackMessageTemplate,
		alertConfigs:         defaultAlertConfigs,
	}
}

// OnSync compares the actual state with the desired, and attempts to reconcile the two
func (syncer *alertSyncer) OnSync(ctx context.Context) error {
	ctx = log.Ctx(ctx).With().Str("resource_kind", radixv1.KindRadixAlert).Logger().WithContext(ctx)
	log.Ctx(ctx).Info().Msg("Syncing")

	return syncer.syncStatus(ctx, syncer.reconcile(ctx))
}

func (syncer *alertSyncer) reconcile(ctx context.Context) error {
	if err := syncer.createOrUpdateSecret(ctx); err != nil {
		return fmt.Errorf("failed to sync secrets: %w", err)
	}

	if err := syncer.configureRbac(ctx); err != nil {
		return fmt.Errorf("failed to configure RBAC: %w", err)
	}

	if err := syncer.createOrUpdateAlertManagerConfig(ctx); err != nil {
		return fmt.Errorf("failed to sync alertmanagerconfigs: %w", err)
	}

	return nil
}

func (syncer *alertSyncer) syncStatus(ctx context.Context, reconcileErr error) error {
	err := syncer.updateStatus(ctx, func(currStatus *radixv1.RadixAlertStatus) {
		currStatus.Reconciled = metav1.Now()
		currStatus.ObservedGeneration = syncer.radixAlert.Generation

		if reconcileErr != nil {
			currStatus.ReconcileStatus = radixv1.RadixAlertReconcileFailed
			currStatus.Message = reconcileErr.Error()
		} else {
			currStatus.ReconcileStatus = radixv1.RadixAlertReconcileSucceeded
			currStatus.Message = ""
		}
	})
	if err != nil {
		return fmt.Errorf("failed to sync status: %w", err)
	}

	return reconcileErr
}

func (syncer *alertSyncer) updateStatus(ctx context.Context, changeStatusFunc func(currStatus *radixv1.RadixAlertStatus)) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		updateObj := syncer.radixAlert.DeepCopy()
		changeStatusFunc(&updateObj.Status)
		updateRAL, err := syncer.radixClient.RadixV1().RadixAlerts(syncer.radixAlert.GetNamespace()).UpdateStatus(ctx, updateObj, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		syncer.radixAlert = updateRAL
		return nil
	})
	return err
}

func (syncer *alertSyncer) getOwnerReference() []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			APIVersion: radixv1.SchemeGroupVersion.Identifier(),
			Kind:       radixv1.KindRadixAlert,
			Name:       syncer.radixAlert.Name,
			UID:        syncer.radixAlert.UID,
			Controller: commonUtils.BoolPtr(true),
		},
	}
}

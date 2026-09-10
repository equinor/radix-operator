package alert

import (
	"context"
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AlertSyncer defines interface for syncing a RadixAlert
type AlertSyncer interface {
	OnSync(ctx context.Context) error
}

type alertSyncer struct {
	dynamicClient        client.Client
	radixAlert           *radixv1.RadixAlert
	slackMessageTemplate slackMessageTemplate
	alertConfigs         AlertConfigs
}

// New creates a new alert syncer
func New(dynamicClient client.Client, radixAlert *radixv1.RadixAlert) AlertSyncer {

	return &alertSyncer{
		dynamicClient:        dynamicClient,
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
	if err := syncer.reconcileSecret(ctx); err != nil {
		return fmt.Errorf("failed to sync secrets: %w", err)
	}

	if err := syncer.configureRbac(ctx); err != nil {
		return fmt.Errorf("failed to configure RBAC: %w", err)
	}

	if err := syncer.reconcileAlertManagerConfig(ctx); err != nil {
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
	updateObj := syncer.radixAlert.DeepCopy()
	changeStatusFunc(&updateObj.Status)
	if err := syncer.dynamicClient.Status().Update(ctx, updateObj); err != nil {
		return err
	}
	syncer.radixAlert = updateObj
	return nil
}

package alert

import (
	"context"
	"fmt"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
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
	logger               zerolog.Logger
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
		logger:               log.Logger.With().Str("resource_kind", radixv1.KindRadixAlert).Str("resource_name", cache.MetaObjectToName(&radixAlert.ObjectMeta).String()).Logger(),
	}
}

// OnSync compares the actual state with the desired, and attempts to reconcile the two
func (syncer *alertSyncer) OnSync(ctx context.Context) error {
	log.Ctx(ctx).Info().Msg("Syncing")

	if err := syncer.syncAlert(); err != nil {
		return err
	}

	return syncer.syncStatus()
}

func (syncer *alertSyncer) syncAlert() error {
	if err := syncer.createOrUpdateSecret(); err != nil {
		return fmt.Errorf("failed to sync secrets: %v", err)
	}

	if err := syncer.configureRbac(); err != nil {
		return fmt.Errorf("failed to configure RBAC: %v", err)
	}

	if err := syncer.createOrUpdateAlertManagerConfig(); err != nil {
		return fmt.Errorf("failed to sync alertmanagerconfigs: %v", err)
	}

	return nil
}

func (syncer *alertSyncer) syncStatus() error {
	syncCompleteTime := metav1.Now()
	err := syncer.updateRadixAlertStatus(func(currStatus *radixv1.RadixAlertStatus) {
		currStatus.Reconciled = &syncCompleteTime
	})
	if err != nil {
		return fmt.Errorf("failed to sync status: %v", err)
	}

	return nil
}

func (syncer *alertSyncer) updateRadixAlertStatus(changeStatusFunc func(currStatus *radixv1.RadixAlertStatus)) error {
	ralInterface := syncer.radixClient.RadixV1().RadixAlerts(syncer.radixAlert.GetNamespace())

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		currentRAL, err := ralInterface.Get(context.TODO(), syncer.radixAlert.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}
		changeStatusFunc(&currentRAL.Status)
		updateRAL, err := ralInterface.UpdateStatus(context.TODO(), currentRAL, metav1.UpdateOptions{})
		if err == nil {
			syncer.radixAlert = updateRAL
		}
		return err
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

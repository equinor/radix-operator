package alert

import (
	"context"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

var (
	defaultSlackTemplate slackMessageTemplate = slackMessageTemplate{
		title:     "{{ template \"radix-slack-alert-title\" .}}",
		titleLink: "{{ template \"radix-slack-alert-titlelink\" .}}",
		text:      "{{ template \"radix-slack-alert-text\" .}}",
	}
	defaultAlertConfigs alertConfigs = alertConfigs{
		"RadixAppComponentCrashLooping": {
			groupBy:    []string{radixApplicationNameLabel, radixComponentNameLabel},
			resolvable: true,
		},
		"RadixAppComponentNotReady": {
			groupBy:    []string{radixApplicationNameLabel, radixComponentNameLabel},
			resolvable: true,
		},
		"RadixAppJobNotReady": {
			groupBy:    []string{radixApplicationNameLabel, radixComponentNameLabel},
			resolvable: true,
		},
		"RadixAppJobFailed": {
			groupBy:    []string{radixApplicationNameLabel, radixComponentNameLabel},
			resolvable: false,
		},
		"RadixAppPipelineJobFailed": {
			groupBy:    []string{radixApplicationNameLabel, radixJobNameLabel},
			resolvable: false,
		},
	}
)

type alertConfig struct {
	groupBy    []string
	resolvable bool
}

type alertConfigs map[string]alertConfig

type slackMessageTemplate struct {
	title     string
	titleLink string
	text      string
}

//AlertSyncer defines interface for syncing a RadixAlert
type AlertSyncer interface {
	OnSync() error
}

type alertSyncer struct {
	kubeclient              kubernetes.Interface
	radixclient             radixclient.Interface
	kubeutil                *kube.Kube
	prometheusperatorclient monitoring.Interface
	radixAlert              *radixv1.RadixAlert
	slackMessageTemplate    slackMessageTemplate
	alertConfigs            alertConfigs
	logger                  *log.Entry
}

// New creates a new alert syncer
func New(kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	prometheusperatorclient monitoring.Interface,
	radixAlert *radixv1.RadixAlert) AlertSyncer {
	return &alertSyncer{
		kubeclient:              kubeclient,
		radixclient:             radixclient,
		kubeutil:                kubeutil,
		prometheusperatorclient: prometheusperatorclient,
		radixAlert:              radixAlert,
		slackMessageTemplate:    defaultSlackTemplate,
		alertConfigs:            defaultAlertConfigs,
		logger:                  log.WithFields(log.Fields{"radixAlert": radixAlert.GetName(), "namespace": radixAlert.GetNamespace()}),
	}
}

// OnSync compares the actual state with the desired, and attempts to reconcile the two
func (syncer *alertSyncer) OnSync() error {
	syncer.logger.Infof("Syncing")

	if err := syncer.syncAlert(); err != nil {
		return err
	}

	return syncer.syncStatus()
}

func (syncer *alertSyncer) syncAlert() error {
	if err := syncer.createOrUpdateSecret(); err != nil {
		syncer.logger.Errorf("Failed to sync secrets: %v", err)
		return err
	}

	if err := syncer.configureRbac(); err != nil {
		syncer.logger.Errorf("Failed to configure RBAC: %v", err)
		return err
	}

	if err := syncer.createOrUpdateAlertManagerConfig(); err != nil {
		syncer.logger.Errorf("Failed to sync alertmanagerconfigs: %v", err)
		return err
	}

	return nil
}

func (syncer *alertSyncer) syncStatus() error {
	syncCompleteTime := metav1.Now()
	err := syncer.updateRadixAlertStatus(func(currStatus *radixv1.RadixAlertStatus) {
		currStatus.Reconciled = &syncCompleteTime
	})
	if err != nil {
		syncer.logger.Errorf("Failed to sync status: %v", err)
	}

	return err
}

func (syncer *alertSyncer) updateRadixAlertStatus(changeStatusFunc func(currStatus *radixv1.RadixAlertStatus)) error {
	ralInterface := syncer.radixclient.RadixV1().RadixAlerts(syncer.radixAlert.GetNamespace())

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
			APIVersion: "radix.equinor.com/v1",
			Kind:       "RadixAlert",
			Name:       syncer.radixAlert.Name,
			UID:        syncer.radixAlert.UID,
		},
	}
}

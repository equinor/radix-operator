package alert

import (
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

type AlertSyncer interface {
	OnSync() error
}

type alertSyncer struct {
	kubeclient              kubernetes.Interface
	radixclient             radixclient.Interface
	kubeutil                *kube.Kube
	prometheusperatorclient monitoring.Interface
	radixAlert              *v1.RadixAlert
}

func New(kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	prometheusperatorclient monitoring.Interface,
	radixAlert *v1.RadixAlert) AlertSyncer {
	return &alertSyncer{
		kubeclient:              kubeclient,
		radixclient:             radixclient,
		kubeutil:                kubeutil,
		prometheusperatorclient: prometheusperatorclient,
		radixAlert:              radixAlert,
	}
}

func (syncer *alertSyncer) OnSync() error {
	log.Infof("Syncing %s", syncer.radixAlert.Name)

	/*
		Apply empty secret with ownerReference to RadixAlert
		Apply role and rolebinding for secrets
		Apply alertmanagerconfig
	*/
	return nil
}

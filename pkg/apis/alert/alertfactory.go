package alert

import (
	kube "github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"k8s.io/client-go/kubernetes"
)

// AlertSyncerFactoryFunc is an adapter that can be used to convert
// a function into a DeploymentSyncerFactory
type AlertSyncerFactoryFunc func(
	kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	prometheusperatorclient monitoring.Interface,
	radixAlert *v1.RadixAlert,
) AlertSyncer

func (f AlertSyncerFactoryFunc) CreateAlertSyncer(
	kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	prometheusperatorclient monitoring.Interface,
	radixAlert *v1.RadixAlert,
) AlertSyncer {
	return f(kubeclient, kubeutil, radixclient, prometheusperatorclient, radixAlert)
}

//AlertSyncerFactory defines a factory to create a DeploymentSyncer
type AlertSyncerFactory interface {
	CreateAlertSyncer(
		kubeclient kubernetes.Interface,
		kubeutil *kube.Kube,
		radixclient radixclient.Interface,
		prometheusperatorclient monitoring.Interface,
		radixAlert *v1.RadixAlert) AlertSyncer
}

package scheduledjob

import (
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

// SyncerFactory defines a factory to create a DeploymentSyncer
type SyncerFactory interface {
	CreateSyncer(
		kubeclient kubernetes.Interface,
		kubeutil *kube.Kube,
		radixclient radixclient.Interface,
		radixScheduleJob *v1.RadixScheduledJob) Syncer
}

// AlertSyncerFactoryFunc is an adapter that can be used to convert
// a function into a DeploymentSyncerFactory
type SyncerFactoryFunc func(
	kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	radixScheduledJob *v1.RadixScheduledJob,
) Syncer

func (f SyncerFactoryFunc) CreateSyncer(
	kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	radixScheduledJob *v1.RadixScheduledJob,
) Syncer {
	return f(kubeclient, kubeutil, radixclient, radixScheduledJob)
}

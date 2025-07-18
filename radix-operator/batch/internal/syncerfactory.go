package internal

import (
	"github.com/equinor/radix-operator/pkg/apis/batch"
	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

// SyncerFactory defines a factory to create a RadixBatches Syncer
type SyncerFactory interface {
	CreateSyncer(
		kubeclient kubernetes.Interface,
		kubeutil *kube.Kube,
		radixclient radixclient.Interface,
		registration *radixv1.RadixRegistration,
		radixBatch *radixv1.RadixBatch,
		config *config.Config,
		options ...batch.SyncerOption,
	) batch.Syncer
}

// SyncerFactoryFunc is an adapter that can be used to convert
// a function into a SyncerFactory
type SyncerFactoryFunc func(
	kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	registration *radixv1.RadixRegistration,
	radixBatch *radixv1.RadixBatch,
	config *config.Config,
	options ...batch.SyncerOption,
) batch.Syncer

func (f SyncerFactoryFunc) CreateSyncer(
	kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	registration *radixv1.RadixRegistration,
	radixBatch *radixv1.RadixBatch,
	config *config.Config,
	options ...batch.SyncerOption,
) batch.Syncer {
	return f(kubeclient, kubeutil, radixclient, registration, radixBatch, config, options...)
}

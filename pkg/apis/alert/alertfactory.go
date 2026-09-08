package alert

import (
	"github.com/equinor/radix-operator/pkg/apis/config2"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AlertSyncerFactoryFunc is an adapter that can be used to convert
// a function into a AlertSyncerFactory
type AlertSyncerFactoryFunc func(
	cfg config2.Config,
	dynamicClient client.Client,
	radixAlert *v1.RadixAlert,
) AlertSyncer

func (f AlertSyncerFactoryFunc) CreateAlertSyncer(
	cfg config2.Config,
	dynamicClient client.Client,
	radixAlert *v1.RadixAlert,
) AlertSyncer {
	return f(cfg, dynamicClient, radixAlert)
}

// AlertSyncerFactory defines a factory to create a AlertSyncer
type AlertSyncerFactory interface {
	CreateAlertSyncer(
		cfg config2.Config,
		dynamicClient client.Client,
		radixAlert *v1.RadixAlert) AlertSyncer
}

package controller

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Kinds of objects reconciled by the controller, used as the kind label of the metrics
const (
	kindRadixApplication = "RadixApplication"
	kindRadixDeployment  = "RadixDeployment"
	kindDexClient        = "DexClient"
	kindSecret           = "Secret"
)

var (
	objectsReconciled = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "radix_easyauth_objects_reconciled_total",
		Help: "The total number of objects successfully reconciled, by kind",
	}, []string{"kind"})
	reconcileErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "radix_easyauth_reconcile_errors_total",
		Help: "The total number of errors reconciling objects, by kind",
	}, []string{"kind"})
)

func init() {
	// The controller-runtime registry is served by the metrics server of the manager
	ctrlmetrics.Registry.MustRegister(objectsReconciled, reconcileErrors)
	for _, kind := range []string{kindRadixApplication, kindRadixDeployment, kindDexClient, kindSecret} {
		objectsReconciled.WithLabelValues(kind)
		reconcileErrors.WithLabelValues(kind)
	}
}

// observeReconcile counts a reconciled object of kind, or a reconcile error when err is not nil
func observeReconcile(kind string, err error) {
	if err != nil {
		reconcileErrors.WithLabelValues(kind).Inc()
		return
	}
	objectsReconciled.WithLabelValues(kind).Inc()
}

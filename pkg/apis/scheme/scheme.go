package scheme

import (
	certmanageracmev1 "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayapixv1alpha1 "sigs.k8s.io/gateway-api/apisx/v1alpha1"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

// NewScheme creates a new runtime.Scheme with all resource types registered.
// This unified scheme supports all Kubernetes resources and CRDs used by radix-operator:
func NewScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()

	// Core Kubernetes types
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	// Radix CRDs
	utilruntime.Must(radixv1.AddToScheme(scheme))

	// KEDA
	utilruntime.Must(kedav1alpha1.AddToScheme(scheme))

	// Prometheus Operator
	utilruntime.Must(monitoringv1.AddToScheme(scheme))
	utilruntime.Must(monitoringv1alpha1.AddToScheme(scheme))

	// Cert-manager
	utilruntime.Must(certmanagerv1.AddToScheme(scheme))
	utilruntime.Must(certmanageracmev1.AddToScheme(scheme))

	// Secrets Store CSI Driver
	utilruntime.Must(secretsstorev1.AddToScheme(scheme))

	// Gateway API
	utilruntime.Must(gatewayapiv1.Install(scheme))
	utilruntime.Must(gatewayapixv1alpha1.Install(scheme))

	// Tekton
	utilruntime.Must(tektonv1.AddToScheme(scheme))

	return scheme
}

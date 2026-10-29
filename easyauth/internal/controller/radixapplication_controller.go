package controller

import (
	"context"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// RadixApplicationReconciler reconciles a RadixApplication object
type RadixApplicationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=radix.equinor.com.easyauth.radix.equinor.com,resources=radixapplications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=radix.equinor.com.easyauth.radix.equinor.com,resources=radixapplications/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=radix.equinor.com.easyauth.radix.equinor.com,resources=radixapplications/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the RadixApplication object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.25.0/pkg/reconcile
func (r *RadixApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)

	// TODO(user): your logic here
	// - Ensure Application have a client in Dex
	// - Ensure Secrets are rotated if needed and synced
	// - Ensure All radixdeployments belonging to the application have correct oauth2 config

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RadixApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// Uncomment the following line adding a pointer to an instance of the controlled resource as an argument
		For(&radixv1.RadixApplication{}).
		Named("radixapplication").
		Complete(r)
}

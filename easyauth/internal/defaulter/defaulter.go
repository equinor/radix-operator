package defaulter

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

const Path = "/radix/v1/radixapplication/mutate"

// +kubebuilder:webhook:name=radixapplication.mutate.easyauth.radix.equinor.com,path=/radix/v1/radixapplication/mutate,mutating=true,failurePolicy=fail,sideEffects=None,groups=radix.equinor.com,resources=radixapplications,verbs=create;update,versions=v1,admissionReviewVersions={v1}

// RadixApplicationDefaulter mutates RadixApplications that have the configured Annotation
type RadixApplicationDefaulter struct {
	Annotation string
}

func RegisterWithManager(mgr ctrl.Manager, d *RadixApplicationDefaulter) error {
	return ctrl.NewWebhookManagedBy(mgr, &radixv1.RadixApplication{}).
		WithDefaulter(d).
		WithDefaulterCustomPath(Path).
		Complete()
}

func (d *RadixApplicationDefaulter) Default(_ context.Context, obj *radixv1.RadixApplication) error {
	if _, ok := obj.GetAnnotations()[d.Annotation]; !ok {
		return nil
	}

	if obj.Labels == nil {
		obj.Labels = map[string]string{}
	}
	obj.Labels["testMutate"] = obj.Name

	return nil
}

var _ admission.Defaulter[*radixv1.RadixApplication] = &RadixApplicationDefaulter{}

package validation

import (
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	internalconfig "github.com/equinor/radix-operator/webhook/internal/config"
	"github.com/equinor/radix-operator/webhook/validation/radixregistration"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:path=/radix/v1/radixregistration/validation,mutating=false,failurePolicy=fail,sideEffects=None,groups=radix.equinor.com,resources=radixregistrations,verbs=create;update,versions=v1,name=validate.radix.equinor.com,admissionReviewVersions={v1}

const radixRegistrationValidatorWebhookPath = "/radix/v1/radixregistration/validation"

func SetupRadixRegistrationWebhookWithManager(mgr manager.Manager, c internalconfig.Config, client radixclient.Interface) {
	rrValidator := radixregistration.NewAdmissionCustomValidator(radixregistration.CreateOnlineValidator(client, c.RequireAdGroups, c.RequireConfigurationItem))
	mgr.GetWebhookServer().Register(radixRegistrationValidatorWebhookPath, admission.WithCustomValidator(mgr.GetScheme(), &radixv1.RadixRegistration{}, rrValidator))
}

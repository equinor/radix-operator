package validation

import (
	internalconfig "github.com/equinor/radix-operator/webhook/internal/config"
	"github.com/equinor/radix-operator/webhook/validation/genericvalidator"
	"github.com/equinor/radix-operator/webhook/validation/radixregistration"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const RadixRegistrationValidatorWebhookPath = "/radix/v1/radixregistration/validation"

//+kubebuilder:webhook:path=/radix/v1/radixregistration/validation,mutating=false,failurePolicy=fail,sideEffects=None,groups=radix.equinor.com,resources=radixregistrations,verbs=create;update,versions=v1,name=validate.radix.equinor.com,admissionReviewVersions={v1}

func SetupWebhook(mgr manager.Manager, c internalconfig.Config) {
	rrValidator := radixregistration.CreateOnlineValidator(mgr.GetClient(), mgr.GetHTTPClient(), c.RequireAdGroups, c.RequireConfigurationItem, c.ValidateConfigurationItemUrl)
	genericvalidator.
		NewGenericAdmissionValidator(rrValidator, rrValidator, nil).
		Register(mgr, RadixRegistrationValidatorWebhookPath)

}

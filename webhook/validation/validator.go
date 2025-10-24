package validation

import (
	internalconfig "github.com/equinor/radix-operator/webhook/internal/config"
	"github.com/equinor/radix-operator/webhook/validation/genericvalidator"
	"github.com/equinor/radix-operator/webhook/validation/radixapplication"
	"github.com/equinor/radix-operator/webhook/validation/radixregistration"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const RadixRegistrationValidatorWebhookPath = "/radix/v1/radixregistration/validation"
const RadixApplicationValidatorWebhookPath = "/radix/v1/radixapplication/validation"

//+kubebuilder:webhook:name=radixregistration.validate.radix.equinor.com,path=/radix/v1/radixregistration/validation,mutating=false,failurePolicy=fail,sideEffects=None,groups=radix.equinor.com,resources=radixregistrations,verbs=create;update,versions=v1,admissionReviewVersions={v1}
//+kubebuilder:webhook:name=radixapplication.validate.radix.equinor.com,path=/radix/v1/radixapplication/validation,mutating=false,failurePolicy=fail,sideEffects=None,groups=radix.equinor.com,resources=radixapplications,verbs=create;update,versions=v1,admissionReviewVersions={v1}

func SetupWebhook(mgr manager.Manager, c internalconfig.Config) {
	rrValidator := radixregistration.CreateOnlineValidator(mgr.GetClient(), c.RequireAdGroups, c.RequireConfigurationItem)
	genericvalidator.
		NewGenericAdmissionValidator(rrValidator, rrValidator, nil).
		Register(mgr, RadixRegistrationValidatorWebhookPath)

	raValidator := radixapplication.CreateOnlineValidator(mgr.GetClient(), c.ReservedDNSAliases, c.ReservedDNSAppAliases)
	genericvalidator.
		NewGenericAdmissionValidator(raValidator, raValidator, nil).
		Register(mgr, RadixApplicationValidatorWebhookPath)

}

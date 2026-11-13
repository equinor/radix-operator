package validation

import (
	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/webhook/validation/genericvalidator"
	"github.com/equinor/radix-operator/webhook/validation/httproute"
	"github.com/equinor/radix-operator/webhook/validation/radixapplication"
	"github.com/equinor/radix-operator/webhook/validation/radixbatchjob"
	"github.com/equinor/radix-operator/webhook/validation/radixregistration"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const RadixRegistrationValidatorWebhookPath = "/radix/v1/radixregistration/validation"
const RadixApplicationValidatorWebhookPath = "/radix/v1/radixapplication/validation"
const RadixBatchValidatorWebhookPath = "/radix/v1/radixbatch/validation"
const RadixDeploymentValidatorWebhookPath = "/radix/v1/radixdeployment/validation"
const HttpRouteValidatorWebhookPath = "/gateway/v1/httproute/validation"

func SetupWebhook(mgr manager.Manager, cfg config.Config) {
	rrValidator := radixregistration.CreateOnlineValidator(mgr.GetClient(), cfg)
	genericvalidator.
		NewGenericAdmissionValidator(rrValidator, rrValidator, nil).
		Register(mgr, RadixRegistrationValidatorWebhookPath)

	raValidator := radixapplication.CreateOnlineValidator(mgr.GetClient(), cfg)
	genericvalidator.
		NewGenericAdmissionValidator(raValidator, raValidator, nil).
		Register(mgr, RadixApplicationValidatorWebhookPath)

	rbValidator := radixbatchjob.CreateValidator()
	genericvalidator.
		NewGenericAdmissionValidator(rbValidator, rbValidator, nil).
		Register(mgr, RadixBatchValidatorWebhookPath)

	hrValidator := httproute.CreateOnlineValidator(mgr.GetAPIReader())
	genericvalidator.
		NewGenericAdmissionValidator(hrValidator, hrValidator, nil).
		Register(mgr, HttpRouteValidatorWebhookPath)

}

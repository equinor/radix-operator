package alerting

import (
	"errors"
	"fmt"

	radixhttp "github.com/equinor/radix-common/net/http"
)

func MultipleAlertingConfigurationsError() error {
	return radixhttp.CoverAllError(errors.New("multiple alert configurations found"), radixhttp.Server)
}

func AlertingNotEnabledError() error {
	return radixhttp.CoverAllError(errors.New("alerting is not enabled"), radixhttp.User)
}

func InvalidAlertReceiverError(alert, receiver string) error {
	return radixhttp.CoverAllError(fmt.Errorf("invalid receiver %s for alert %s", receiver, alert), radixhttp.User)
}

func InvalidAlertError(alert string) error {
	return radixhttp.CoverAllError(fmt.Errorf("alert %s is not valid", alert), radixhttp.User)
}

func AlertingAlreadyEnabledError() error {
	return radixhttp.CoverAllError(errors.New("alerting already enabled"), radixhttp.User)
}

func UpdateReceiverSecretNotDefinedError(receiverName string) error {
	return radixhttp.CoverAllError(fmt.Errorf("receiver %s in receiverSecrets is not defined in receivers", receiverName), radixhttp.User)
}

func InvalidSlackURLError(underlyingError error) error {
	return radixhttp.CoverAllError(fmt.Errorf("invalid slack url: %v", underlyingError), radixhttp.User)
}

func InvalidSlackURLSchemeError() error {
	return InvalidSlackURLError(errors.New("invalid scheme, must be https"))
}

package radixvalidators

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
)

const (
	azureClientIdResourceName = "identity.azure.clientId"
)

var (
	validOAuthSessionStoreTypes = []string{string(radixv1.SessionStoreCookie), string(radixv1.SessionStoreRedis), string(radixv1.SessionStoreSystemManaged)}
	validOAuthCookieSameSites   = []string{string(radixv1.SameSiteStrict), string(radixv1.SameSiteLax), string(radixv1.SameSiteNone), string(radixv1.SameSiteEmpty)}

	requiredRadixApplicationValidators = []RadixApplicationValidator{
		validateVolumeMountConfigForRA,
		ValidateNotificationsForRA,
	}

	storageAccountNameRegExp = regexp.MustCompile(`^[a-z0-9]{3,24}$`)
)

// RadixApplicationValidator defines a validator function for a RadixApplication
type RadixApplicationValidator func(radixApplication *radixv1.RadixApplication) error

// CanRadixApplicationBeInserted Checks if application config is valid. Returns a single error, if this is the case
func CanRadixApplicationBeInserted(ctx context.Context, radixClient radixclient.Interface, app *radixv1.RadixApplication, dnsAliasConfig *dnsalias.DNSConfig, additionalValidators ...RadixApplicationValidator) error {

	validators := append(requiredRadixApplicationValidators, additionalValidators...)

	return validateRadixApplication(app, validators...)
}

// IsRadixApplicationValid Checks if application config is valid without server validation
func IsRadixApplicationValid(app *radixv1.RadixApplication, additionalValidators ...RadixApplicationValidator) error {
	validators := append(requiredRadixApplicationValidators, additionalValidators...)
	return validateRadixApplication(app, validators...)
}

func validateRadixApplication(radixApplication *radixv1.RadixApplication, validators ...RadixApplicationValidator) error {
	var errs []error
	for _, v := range validators {
		if err := v(radixApplication); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// ValidateNotificationsForRA Validate all notifications in the RadixApplication
func ValidateNotificationsForRA(app *radixv1.RadixApplication) error {
	var errs []error
	for _, job := range app.Spec.Jobs {
		if err := ValidateNotifications(app, job.Notifications, job.GetName(), ""); err != nil {
			errs = append(errs, err)
		}
		for _, envConfig := range job.EnvironmentConfig {
			if err := ValidateNotifications(app, envConfig.Notifications, job.GetName(), envConfig.Environment); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// ValidateNotifications Validate specified Notifications for the RadixApplication
func ValidateNotifications(app *radixv1.RadixApplication, notifications *radixv1.Notifications, jobComponentName string, environment string) error {
	if notifications == nil || notifications.Webhook == nil || len(*notifications.Webhook) == 0 {
		return nil
	}
	webhook := strings.ToLower(strings.TrimSpace(*notifications.Webhook))
	webhookUrl, err := url.Parse(webhook)
	if err != nil {
		return InvalidWebhookUrlWithMessage(jobComponentName, environment)
	}
	if len(webhookUrl.Scheme) > 0 && webhookUrl.Scheme != "https" && webhookUrl.Scheme != "http" {
		return NotAllowedSchemeInWebhookUrlWithMessage(webhookUrl.Scheme, jobComponentName, environment)
	}
	if len(webhookUrl.Port()) == 0 {
		return MissingPortInWebhookUrlWithMessage(jobComponentName, environment)
	}
	targetRadixComponent, targetRadixJobComponent := getRadixCommonComponentByName(app, webhookUrl.Hostname())
	if targetRadixComponent == nil && targetRadixJobComponent == nil {
		return OnlyAppComponentAllowedInWebhookUrlWithMessage(jobComponentName, environment)
	}
	if targetRadixComponent != nil {
		componentPort := getComponentPort(targetRadixComponent, webhookUrl.Port())
		if componentPort == nil {
			return InvalidPortInWebhookUrlWithMessage(webhookUrl.Port(), targetRadixComponent.GetName(), jobComponentName, environment)
		}
		if strings.EqualFold(componentPort.Name, targetRadixComponent.PublicPort) {
			return InvalidUseOfPublicPortInWebhookUrlWithMessage(webhookUrl.Port(), targetRadixComponent.GetName(), jobComponentName, environment)
		}
	} else if targetRadixJobComponent != nil {
		componentPort := getComponentPort(targetRadixJobComponent, webhookUrl.Port())
		if componentPort == nil {
			return InvalidPortInWebhookUrlWithMessage(webhookUrl.Port(), targetRadixJobComponent.GetName(), jobComponentName, environment)
		}
	}
	return nil
}

func getComponentPort(radixComponent radixv1.RadixCommonComponent, port string) *radixv1.ComponentPort {
	for _, componentPort := range radixComponent.GetPorts() {
		if strings.EqualFold(strconv.Itoa(int(componentPort.Port)), port) {
			return &componentPort
		}
	}
	return nil
}

func getRadixCommonComponentByName(app *radixv1.RadixApplication, componentName string) (*radixv1.RadixComponent, *radixv1.RadixJobComponent) {
	for _, radixComponent := range app.Spec.Components {
		if strings.EqualFold(radixComponent.GetName(), componentName) {
			return &radixComponent, nil
		}
	}
	for _, radixJobComponent := range app.Spec.Jobs {
		if strings.EqualFold(radixJobComponent.GetName(), componentName) {
			return nil, &radixJobComponent
		}
	}
	return nil, nil
}

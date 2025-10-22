package radixvalidators

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/equinor/radix-common/utils/slice"
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

func validateVolumeMountConfigForRA(app *radixv1.RadixApplication) error {
	var errs []error
	for _, component := range app.Spec.Components {
		hasComponentIdentityAzureClientId := len(component.Identity.GetAzure().GetClientId()) > 0
		if err := validateVolumeMounts(component.VolumeMounts, hasComponentIdentityAzureClientId); err != nil {
			errs = append(errs, volumeMountValidationFailedForComponent(component.Name, err))
		}
		for _, envConfig := range component.EnvironmentConfig {
			hasEnvIdentityAzureClientId := hasComponentIdentityAzureClientId || len(envConfig.GetIdentity().GetAzure().GetClientId()) > 0
			if err := validateVolumeMounts(envConfig.VolumeMounts, hasEnvIdentityAzureClientId); err != nil {
				errs = append(errs, volumeMountValidationFailedForComponentInEnvironment(component.Name, envConfig.Environment, err))
			}
		}
	}
	for _, job := range app.Spec.Jobs {
		hasJobIdentityAzureClientId := len(job.Identity.GetAzure().GetClientId()) > 0
		if err := validateVolumeMounts(job.VolumeMounts, hasJobIdentityAzureClientId); err != nil {
			errs = append(errs, volumeMountValidationFailedForJobComponent(job.Name, err))
		}
		for _, envConfig := range job.EnvironmentConfig {
			hasEnvIdentityAzureClientId := hasJobIdentityAzureClientId || len(envConfig.GetIdentity().GetAzure().GetClientId()) > 0
			if err := validateVolumeMounts(envConfig.VolumeMounts, hasEnvIdentityAzureClientId); err != nil {
				errs = append(errs, volumeMountValidationFailedForJobComponentInEnvironment(job.Name, envConfig.Environment, err))
			}
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

func validateVolumeMounts(volumeMounts []radixv1.RadixVolumeMount, hasIdentityAzureClientId bool) error {
	if len(volumeMounts) == 0 {
		return nil
	}

	for _, v := range volumeMounts {
		if len(strings.TrimSpace(v.Name)) == 0 {
			return ErrVolumeMountMissingName
		}

		if len(strings.TrimSpace(v.Path)) == 0 {
			return volumeMountValidationError(v.Name, ErrVolumeMountMissingPath)
		}

		if len(slice.FindAll(volumeMounts, func(rvm radixv1.RadixVolumeMount) bool { return rvm.Name == v.Name })) > 1 {
			return volumeMountValidationError(v.Name, ErrVolumeMountDuplicateName)
		}

		if len(slice.FindAll(volumeMounts, func(rvm radixv1.RadixVolumeMount) bool { return rvm.Path == v.Path })) > 1 {
			return volumeMountValidationError(v.Name, ErrVolumeMountDuplicatePath)
		}

		volumeSourceCount := len(slice.FindAll(
			[]bool{v.HasDeprecatedVolume(), v.HasBlobFuse2(), v.HasEmptyDir()},
			func(b bool) bool { return b }),
		)
		if volumeSourceCount > 1 {
			return volumeMountValidationError(v.Name, ErrVolumeMountMultipleTypes)
		}
		if volumeSourceCount == 0 {
			return volumeMountValidationError(v.Name, ErrVolumeMountMissingType)
		}

		switch {
		case v.HasDeprecatedVolume():
			if err := validateVolumeMountDeprecatedSource(&v); err != nil {
				return volumeMountValidationError(v.Name, err)
			}
		case v.HasBlobFuse2():
			if err := validateVolumeMountBlobFuse2(v.BlobFuse2, hasIdentityAzureClientId); err != nil {
				return volumeMountValidationError(v.Name, err)
			}
		case v.HasEmptyDir():
			if err := validateVolumeMountEmptyDir(v.EmptyDir); err != nil {
				return volumeMountValidationError(v.Name, err)
			}
		}
	}

	return nil
}

func validateVolumeMountDeprecatedSource(v *radixv1.RadixVolumeMount) error {
	//nolint:staticcheck
	if v.Type != radixv1.MountTypeBlobFuse2FuseCsiAzure {
		return volumeMountDeprecatedSourceValidationError(ErrVolumeMountInvalidType)
	}
	//nolint:staticcheck
	if v.Type == radixv1.MountTypeBlobFuse2FuseCsiAzure && len(v.Storage) == 0 {
		return volumeMountDeprecatedSourceValidationError(ErrVolumeMountMissingStorage)
	}
	return nil
}

func validateVolumeMountBlobFuse2(fuse2 *radixv1.RadixBlobFuse2VolumeMount, hasIdentityAzureClientId bool) error {
	if !slices.Contains([]radixv1.BlobFuse2Protocol{radixv1.BlobFuse2ProtocolFuse2, ""}, fuse2.Protocol) {
		return volumeMountBlobFuse2ValidationError(ErrVolumeMountInvalidProtocol)
	}

	if len(fuse2.Container) == 0 {
		return volumeMountBlobFuse2ValidationError(ErrVolumeMountMissingContainer)
	}

	if len(fuse2.StorageAccount) > 0 && !storageAccountNameRegExp.Match([]byte(fuse2.StorageAccount)) {
		return volumeMountBlobFuse2ValidationError(ErrVolumeMountInvalidStorageAccount)
	}
	if fuse2.UseAzureIdentity != nil && *fuse2.UseAzureIdentity {
		if !hasIdentityAzureClientId {
			return volumeMountBlobFuse2ValidationError(ErrVolumeMountMissingAzureIdentity)
		}
		if fuse2.SubscriptionId == "" {
			return volumeMountBlobFuse2ValidationError(ErrVolumeMountWithUseAzureIdentityMissingSubscriptionId)
		}
		if fuse2.ResourceGroup == "" {
			return volumeMountBlobFuse2ValidationError(ErrVolumeMountWithUseAzureIdentityMissingResourceGroup)
		}
		if fuse2.StorageAccount == "" {
			return volumeMountBlobFuse2ValidationError(ErrVolumeMountWithUseAzureIdentityMissingStorageAccount)
		}
	}

	if err := validateBlobFuse2BlockCache(fuse2.BlockCacheOptions); err != nil {
		return fmt.Errorf("invalid blockCache configuration: %w", err)
	}

	return nil
}

func validateBlobFuse2BlockCache(blockCache *radixv1.BlobFuse2BlockCacheOptions) error {
	if blockCache == nil {
		return nil
	}

	if prefetchCount := blockCache.PrefetchCount; prefetchCount != nil && !(*prefetchCount == 0 || *prefetchCount > 10) {
		return ErrInvalidBlobFuse2BlockCachePrefetchCount
	}

	return nil
}

func validateVolumeMountEmptyDir(emptyDir *radixv1.RadixEmptyDirVolumeMount) error {
	if emptyDir.SizeLimit.IsZero() {
		return volumeMountEmptyDirValidationError(ErrVolumeMountMissingSizeLimit)
	}
	return nil
}

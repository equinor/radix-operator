package radixvalidators

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/branch"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/robfig/cron/v3"
	corev1 "k8s.io/api/core/v1"
)

const (
	maximumNumberOfEgressRules = 1000
	azureClientIdResourceName  = "identity.azure.clientId"
)

var (
	validOAuthSessionStoreTypes          = []string{string(radixv1.SessionStoreCookie), string(radixv1.SessionStoreRedis), string(radixv1.SessionStoreSystemManaged)}
	validOAuthCookieSameSites            = []string{string(radixv1.SameSiteStrict), string(radixv1.SameSiteLax), string(radixv1.SameSiteNone), string(radixv1.SameSiteEmpty)}
	validAzureEventHubTriggerCheckpoints = map[radixv1.AzureEventHubTriggerCheckpointStrategy]any{radixv1.AzureEventHubTriggerCheckpointStrategyAzureFunction: struct{}{},
		radixv1.AzureEventHubTriggerCheckpointStrategyBlobMetadata: struct{}{}, radixv1.AzureEventHubTriggerCheckpointStrategyGoSdk: struct{}{}}

	requiredRadixApplicationValidators = []RadixApplicationValidator{
		validateEnvironmentEgressRules,
		validateVariables,
		validateBranchNames,
		validateHorizontalScalingConfigForRA,
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

func validateJobComponent(app *radixv1.RadixApplication, job radixv1.RadixJobComponent) error {
	var errs []error

	return errors.Join(errs...)
}

func validateVariables(app *radixv1.RadixApplication) error {
	for _, component := range app.Spec.Components {
		err := validateRadixComponentVariables(&component)
		if err != nil {
			return err
		}
	}
	for _, job := range app.Spec.Jobs {
		err := validateRadixComponentVariables(&job)
		if err != nil {
			return err
		}
	}
	return nil
}

func validateRadixComponentVariables(component radixv1.RadixCommonComponent) error {
	if err := validateVariableNames("environment variable name", component.GetVariables()); err != nil {
		return err
	}

	for _, envConfig := range component.GetEnvironmentConfig() {
		if err := validateVariableNames("environment variable name", envConfig.GetVariables()); err != nil {
			return err
		}
	}
	return nil
}

func validateVariableNames(resourceName string, variables radixv1.EnvVarsMap) error {
	existingVariableName := make(map[string]bool)
	for envVarName := range variables {
		if _, exists := existingVariableName[envVarName]; exists {
			return duplicateEnvVarNameWithMessage(envVarName)
		}
		existingVariableName[envVarName] = true
		if err := validateVariableName(resourceName, envVarName); err != nil {
			return err
		}
	}
	return nil
}

func validateBranchNames(app *radixv1.RadixApplication) error {
	for _, env := range app.Spec.Environments {
		if env.Build.From == "" {
			continue
		}

		if len(env.Build.From) > 253 {
			return InvalidStringValueMaxLengthErrorWithMessage("branch from", env.Build.From, 253)
		}

		isValid := branch.IsValidPattern(env.Build.From)
		if !isValid {
			return InvalidBranchNameErrorWithMessage(env.Build.From)
		}
	}
	return nil
}

func validateEnvironmentEgressRules(app *radixv1.RadixApplication) error {
	var errs []error
	for _, env := range app.Spec.Environments {
		if len(env.Egress.Rules) > maximumNumberOfEgressRules {
			errs = append(errs, fmt.Errorf("number of egress rules for env %s exceeds max nr %d", env.Name, maximumNumberOfEgressRules))
			continue
		}
		for _, egressRule := range env.Egress.Rules {
			if len(egressRule.Destinations) < 1 {
				errs = append(errs, fmt.Errorf("egress rule must contain at least one destination"))
			}
			for _, ipMask := range egressRule.Destinations {
				err := validateEgressRuleIpMask(string(ipMask))
				if err != nil {
					errs = append(errs, err)
				}
			}
			for _, port := range egressRule.Ports {
				err := validateEgressRulePortProtocol(port.Protocol)
				if err != nil {
					errs = append(errs, err)
				}
				err = validateEgressRulePort(port.Port)
				if err != nil {
					errs = append(errs, err)
				}
			}
		}
	}

	return errors.Join(errs...)
}

func validateEgressRulePort(port int32) error {
	if port < 1 || port > maximumPortNumber {
		return fmt.Errorf("%d must be equal to or greater than 1 and lower than or equal to %d", port, maximumPortNumber)
	}
	return nil
}

func validateEgressRulePortProtocol(protocol string) error {
	upperCaseProtocol := strings.ToUpper(protocol)
	validProtocols := []string{string(corev1.ProtocolTCP), string(corev1.ProtocolUDP)}
	if commonUtils.ContainsString(validProtocols, upperCaseProtocol) {
		return nil
	} else {
		return InvalidEgressPortProtocolErrorWithMessage(protocol, validProtocols)
	}
}

func validateEgressRuleIpMask(ipMask string) error {
	ipAddr, _, err := net.ParseCIDR(ipMask)
	if err != nil {
		return NotValidCidrErrorWithMessage(err.Error())
	}
	ipV4Addr := ipAddr.To4()
	if ipV4Addr == nil {
		return NotValidIPv4CidrErrorWithMessage(ipMask)
	}

	return nil
}
func validateIllegalPrefixInVariableName(resourceName string, value string) error {
	if utils.IsRadixEnvVar(value) {
		return fmt.Errorf("%s %s can not start with prefix reserved for platform", resourceName, value)
	}
	return nil
}

func validateResourceWithRegexp(resourceName, value, regexpExpression string) error {
	if len(value) > 253 {
		return InvalidStringValueMaxLengthErrorWithMessage(resourceName, value, 253)
	}

	re := regexp.MustCompile(regexpExpression)

	isValid := re.MatchString(value)
	if isValid {
		return nil
	}
	return InvalidResourceNameErrorWithMessage(resourceName, value)
}

func validateHorizontalScalingConfigForRA(app *radixv1.RadixApplication) error {
	var errs []error

	for _, component := range app.Spec.Components {
		if component.HorizontalScaling != nil {
			err := validateHorizontalScalingPart(component.HorizontalScaling)
			if err != nil {
				errs = append(errs, fmt.Errorf("error validating horizontal scaling for component: %s: %w", component.Name, err))
			}
		}
		for _, envConfig := range component.EnvironmentConfig {
			if envConfig.HorizontalScaling == nil {
				continue
			}

			err := validateHorizontalScalingPart(envConfig.HorizontalScaling)
			if err != nil {
				errs = append(errs, fmt.Errorf("error validating horizontal scaling for environment %s in component %s: %w", envConfig.Environment, component.Name, err))
			}
		}
	}

	return errors.Join(errs...)
}

func validateHorizontalScalingPart(config *radixv1.RadixHorizontalScaling) error {
	var errs []error

	if config.RadixHorizontalScalingResources != nil && len(config.Triggers) > 0 { //nolint:staticcheck // backward compatibility support
		errs = append(errs, ErrCombiningTriggersWithResourcesIsIllegal)
	}

	config = config.NormalizeConfig()

	if config.MaxReplicas == 0 {
		errs = append(errs, ErrMaxReplicasForHPANotSetOrZero)
	}
	if *config.MinReplicas > config.MaxReplicas {
		errs = append(errs, ErrMinReplicasGreaterThanMaxReplicas)
	}

	if *config.MinReplicas == 0 && !hasNonResourceTypeTriggers(config) {
		errs = append(errs, ErrInvalidMinimumReplicasConfigurationWithMemoryAndCPUTriggers)
	}

	if err := validateTriggerDefinition(config); err != nil {
		errs = append(errs, err)
	}

	if err := validateUniqueTriggerNames(config); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func validateUniqueTriggerNames(config *radixv1.RadixHorizontalScaling) error {
	if config == nil {
		return nil
	}

	var errs []error
	var names []string

	for _, trigger := range config.Triggers {
		if slices.Contains(names, trigger.Name) {
			errs = append(errs, fmt.Errorf("%w: %s", ErrDuplicateTriggerName, trigger.Name))
		} else {
			names = append(names, trigger.Name)
		}
	}

	return errors.Join(errs...)
}

func validateTriggerDefinition(config *radixv1.RadixHorizontalScaling) error {
	var errs []error

	for _, trigger := range config.Triggers {
		var definitions int

		if trigger.Cpu != nil {
			definitions++
			if trigger.Cpu.Value == 0 {
				errs = append(errs, fmt.Errorf("invalid trigger %s: value must be set: %w", trigger.Name, ErrInvalidTriggerDefinition))
			}
		}
		if trigger.Memory != nil {
			definitions++
			if trigger.Memory.Value == 0 {
				errs = append(errs, fmt.Errorf("invalid trigger %s: value must be set: %w", trigger.Name, ErrInvalidTriggerDefinition))
			}
		}
		if trigger.Cron != nil {
			definitions++
			errs = append(errs, validateCronTrigger(trigger)...)
		}
		if trigger.AzureServiceBus != nil {
			definitions++
			errs = append(errs, validateAzureServiceBusTrigger(trigger)...)
		}
		if trigger.AzureEventHub != nil {
			definitions++
			errs = append(errs, validateAzureEventHubTrigger(trigger)...)
		}
		if definitions == 0 {
			errs = append(errs, fmt.Errorf("invalid trigger %s: %w", trigger.Name, ErrNoDefinitionInTrigger))
		} else if definitions > 1 {
			errs = append(errs, fmt.Errorf("invalid trigger %s: %w (found %d definitions)", trigger.Name, ErrMoreThanOneDefinitionInTrigger, definitions))
		}
	}

	return errors.Join(errs...)
}

func validateCronTrigger(trigger radixv1.RadixHorizontalScalingTrigger) []error {
	var errs []error
	if trigger.Cron.Start == "" {
		errs = append(errs, fmt.Errorf("invalid trigger %s: start must be set: %w", trigger.Name, ErrInvalidTriggerDefinition))
	} else if err := validateKedaCronSchedule(trigger.Cron.Start); err != nil {
		errs = append(errs, fmt.Errorf("invalid trigger %s: start is invalid: %w: %w", trigger.Name, err, ErrInvalidTriggerDefinition))
	}

	if trigger.Cron.End == "" {
		errs = append(errs, fmt.Errorf("invalid trigger %s: end must be set: %w", trigger.Name, ErrInvalidTriggerDefinition))
	} else if err := validateKedaCronSchedule(trigger.Cron.End); err != nil {
		errs = append(errs, fmt.Errorf("invalid trigger %s: end is invalid: %w: %w", trigger.Name, err, ErrInvalidTriggerDefinition))
	}

	if trigger.Cron.Timezone == "" {
		errs = append(errs, fmt.Errorf("invalid trigger %s: timezone must be set: %w", trigger.Name, ErrInvalidTriggerDefinition))
	}

	if trigger.Cron.DesiredReplicas < 1 {
		errs = append(errs, fmt.Errorf("invalid trigger %s: desiredReplicas must be positive integer: %w", trigger.Name, ErrInvalidTriggerDefinition))
	}
	return errs
}

func validateAzureServiceBusTrigger(trigger radixv1.RadixHorizontalScalingTrigger) []error {
	var errs []error
	// TODO: this is only requrired when using WorkloadIdentity
	if trigger.AzureServiceBus.Namespace == "" {
		errs = append(errs, fmt.Errorf("invalid trigger %s: Name of the Azure Service Bus namespace that contains your queue or topic: %w", trigger.Name, ErrInvalidTriggerDefinition))
	}

	if trigger.AzureServiceBus.QueueName != "" && (trigger.AzureServiceBus.TopicName != "" || trigger.AzureServiceBus.SubscriptionName != "") {
		errs = append(errs, fmt.Errorf("invalid trigger %s: queueName cannot be used with topicName or subscriptionName: %w", trigger.Name, ErrInvalidTriggerDefinition))
	}

	if trigger.AzureServiceBus.QueueName == "" && (trigger.AzureServiceBus.TopicName == "" || trigger.AzureServiceBus.SubscriptionName == "") {
		errs = append(errs, fmt.Errorf("invalid trigger %s: both topicName and subscriptionName must be set if queueName is not used: %w", trigger.Name, ErrInvalidTriggerDefinition))
	}
	if trigger.AzureServiceBus.Authentication.Identity.Azure.ClientId == "" && trigger.AzureServiceBus.ConnectionFromEnv == "" {
		errs = append(errs, fmt.Errorf("invalid trigger %s: azure workload identity or connectionFromEnv are required: %w", trigger.Name, ErrInvalidTriggerDefinition))
	}
	return errs
}

func validateAzureEventHubTrigger(trigger radixv1.RadixHorizontalScalingTrigger) []error {
	var errs []error

	if auth := trigger.AzureEventHub.Authentication; auth != nil && (*auth).Identity.Azure.ClientId != "" {
		if trigger.AzureEventHub.EventHubNamespace == "" && trigger.AzureEventHub.EventHubNamespaceFromEnv == "" {
			errs = append(errs, fmt.Errorf("invalid trigger %s: event hub namespace is required when used workload identity: %w", trigger.Name, ErrInvalidTriggerDefinition))
		}
		if trigger.AzureEventHub.EventHubName == "" && trigger.AzureEventHub.EventHubNameFromEnv == "" {
			errs = append(errs, fmt.Errorf("invalid trigger %s: event hub name is required when used workload identity: %w", trigger.Name, ErrInvalidTriggerDefinition))
		}
		if trigger.AzureEventHub.StorageAccount == "" {
			errs = append(errs, fmt.Errorf("invalid trigger %s: both storage account name and storage connection are required: %w", trigger.Name, ErrInvalidTriggerDefinition))
		}
	} else {
		if trigger.AzureEventHub.EventHubConnectionFromEnv == "" {
			errs = append(errs, fmt.Errorf("invalid trigger %s: event hub connection string is required when not used workload identity: %w", trigger.Name, ErrInvalidTriggerDefinition))
		}
		if trigger.AzureEventHub.StorageConnectionFromEnv == "" {
			errs = append(errs, fmt.Errorf("invalid trigger %s: storage account connection string when not used workload identity: %w", trigger.Name, ErrInvalidTriggerDefinition))
		}
	}

	if trigger.AzureEventHub.Container == "" && trigger.AzureEventHub.CheckpointStrategy != radixv1.AzureEventHubTriggerCheckpointStrategyAzureFunction {
		errs = append(errs, fmt.Errorf("invalid trigger %s: storage account container name is required for not azureFunction checkpointStrategy: %w", trigger.Name, ErrInvalidTriggerDefinition))
	}
	if !isValidAzureEventHubTriggerCheckpoints(trigger.AzureEventHub.CheckpointStrategy) {
		errs = append(errs, fmt.Errorf("invalid trigger %s: invalid checkpoint strategy: %w", trigger.Name, ErrInvalidTriggerDefinition))
	}
	return errs
}

func isValidAzureEventHubTriggerCheckpoints(checkpointStrategy radixv1.AzureEventHubTriggerCheckpointStrategy) bool {
	if checkpointStrategy == "" {
		return true
	}
	_, ok := validAzureEventHubTriggerCheckpoints[checkpointStrategy]
	return ok
}

func validateKedaCronSchedule(schedule string) error {
	// Validate same schedule as KEDA: github.com/kedacore/keda/pkg/scalers/cron_scaler.go:71
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	_, err := parser.Parse(schedule)
	return err
}

// hasNonResourceTypeTriggers returns true if atleast one non resource type triggers found
func hasNonResourceTypeTriggers(config *radixv1.RadixHorizontalScaling) bool {
	for _, trigger := range config.Triggers {
		if trigger.Cron != nil {
			return true
		}
		if trigger.AzureServiceBus != nil {
			return true
		}
		if trigger.AzureEventHub != nil {
			return true
		}
	}

	return false
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

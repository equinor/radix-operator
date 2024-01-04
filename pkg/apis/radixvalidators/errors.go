package radixvalidators

import (
	"fmt"
	"strings"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/pkg/errors"
)

var (
	ErrMissingPrivateImageHubUsername                                      = errors.New("missing private image hub username")
	ErrEnvForDNSAppAliasNotDefined                                         = errors.New("env for dns app alias not defined")
	ErrComponentForDNSAppAliasNotDefined                                   = errors.New("component for dns app alias not defined")
	ErrExternalAliasCannotBeEmpty                                          = errors.New("external alias cannot be empty")
	ErrEnvForDNSExternalAliasNotDefined                                    = errors.New("env for dns external alias not defined")
	ErrComponentForDNSExternalAliasNotDefined                              = errors.New("component for dns external alias not defined")
	ErrComponentForDNSExternalAliasIsNotMarkedAsPublic                     = errors.New("component for dns external alias is not marked as public")
	ErrEnvironmentReferencedByComponentDoesNotExist                        = errors.New("environment referenced by component does not exist")
	ErrInvalidPortNameLength                                               = errors.New("invalid port name length")
	ErrPortNameIsRequiredForPublicComponent                                = errors.New("port name is required for public component")
	ErrMonitoringPortNameIsNotFoundComponent                               = errors.New("monitoring port name is not found component")
	ErrMultipleMatchingPortNames                                           = errors.New("multiple matching port names")
	ErrSchedulerPortCannotBeEmptyForJob                                    = errors.New("scheduler port cannot be empty for job")
	ErrPayloadPathCannotBeEmptyForJob                                      = errors.New("payload path cannot be empty for job")
	ErrMemoryResourceRequirementFormat                                     = errors.New("memory resource requirement format")
	ErrCPUResourceRequirementFormat                                        = errors.New("cpu resource requirement format")
	ErrInvalidVerificationType                                             = errors.New("invalid verification")
	ErrResourceRequestOverLimit                                            = errors.New("resource request over limit")
	ErrInvalidResource                                                     = errors.New("invalid resource")
	ErrDuplicateExternalAlias                                              = errors.New("duplicate external alias")
	ErrInvalidBranchName                                                   = errors.New("invalid branch name")
	ErrMaxReplicasForHPANotSetOrZero                                       = errors.New("max replicas for hpanot set or zero")
	ErrMinReplicasGreaterThanMaxReplicas                                   = errors.New("min replicas greater than max replicas")
	ErrNoScalingResourceSet                                                = errors.New("no scaling resource set")
	ErrEmptyVolumeMountTypeOrDriverSection                                 = errors.New("empty volume mount type or driver section")
	ErrMultipleVolumeMountTypesDefined                                     = errors.New("multiple volume mount types defined")
	ErrEmptyVolumeMountNameOrPath                                          = errors.New("empty volume mount name or path")
	ErrEmptyVolumeMountStorage                                             = errors.New("empty volume mount storage")
	ErrEmptyBlobFuse2VolumeMountContainer                                  = errors.New("empty blob fuse 2 volume mount container")
	ErrUnsupportedBlobFuse2VolumeMountProtocol                             = errors.New("unsupported blob fuse 2 volume mount protocol")
	ErrDuplicatePathForVolumeMountType                                     = errors.New("duplicate path for volume mount")
	ErrDuplicateNameForVolumeMountType                                     = errors.New("duplicate name for volume mount")
	ErrunknownVolumeMountType                                              = errors.New("unknown volume mount type")
	ErrApplicationNameNotLowercase                                         = errors.New("application name not lowercase")
	ErrPublicImageComponentCannotHaveSourceOrDockerfileSet                 = errors.New("public image component cannot have source or dockerfile")
	ErrComponentWithTagInEnvironmentConfigForEnvironmentRequiresDynamicTag = errors.New("component with tag in environment config for environment requires dynamic")
	ErrComponentNameReservedSuffix                                         = errors.New("component name reserved suffix")
	ErrSecretNameConflictsWithEnvironmentVariable                          = errors.New("secret name conflicts with environment")
	ErrInvalidStringValueMaxLength                                         = errors.New("invalid string value max length")
	ErrInvalidStringValueMinLength                                         = errors.New("invalid string value min length")
	ErrResourceNameCannotBeEmpty                                           = errors.New("resource name cannot be empty")
	ErrInvalidUUID                                                         = errors.New("invalid")
	ErrInvalidEmail                                                        = errors.New("invalid email")
	ErrInvalidResourceName                                                 = errors.New("invalid resource name")
	ErrInvalidLowerCaseAlphaNumericDashResourceName                        = errors.New("invalid lower case alpha numeric dash resource name")
	ErrNoRegistrationExistsForApplication                                  = errors.New("no registration exists for application")
	ErrInvalidConfigBranchName                                             = errors.New("invalid config branch")
	ErrOauth                                                               = errors.New("oauth error")
	ErrOAuthClientIdEmpty                                                  = errors.Wrap(ErrOauth, "oauth client id empty")
	ErrOAuthRequiresPublicPort                                             = errors.Wrap(ErrOauth, "oauth requires public port")
	ErrOAuthProxyPrefixEmpty                                               = errors.Wrap(ErrOauth, "oauth proxy prefix empty")
	ErrOAuthProxyPrefixIsRoot                                              = errors.Wrap(ErrOauth, "oauth proxy prefix is root")
	ErrOAuthSessionStoreTypeInvalid                                        = errors.Wrap(ErrOauth, "oauth session store type invalid")
	ErrOAuthOidcJwksUrlEmpty                                               = errors.Wrap(ErrOauth, "oauth oidc jwks url empty")
	ErrOAuthLoginUrlEmpty                                                  = errors.Wrap(ErrOauth, "oauth login url empty")
	ErrOAuthRedeemUrlEmpty                                                 = errors.Wrap(ErrOauth, "oauth redeem url empty")
	ErrOAuthOidcEmpty                                                      = errors.Wrap(ErrOauth, "oauth oidc empty")
	ErrOAuthOidcSkipDiscoveryEmpty                                         = errors.Wrap(ErrOauth, "oauth oidc skip discovery empty")
	ErrOAuthRedisStoreEmpty                                                = errors.Wrap(ErrOauth, "oauth redis store empty")
	ErrOAuthRedisStoreConnectionURLEmpty                                   = errors.Wrap(ErrOauth, "oauth redis store connection urlempty")
	ErrOAuthCookieStoreMinimalIncorrectSetXAuthRequestHeaders              = errors.Wrap(ErrOauth, "oauth cookie store minimal incorrect set xauth request headers")
	ErrOAuthCookieStoreMinimalIncorrectSetAuthorizationHeader              = errors.Wrap(ErrOauth, "oauth cookie store minimal incorrect set authorization header")
	ErrOAuthCookieStoreMinimalIncorrectCookieRefreshInterval               = errors.Wrap(ErrOauth, "oauth cookie store minimal incorrect cookie refresh interval")
	ErrOAuthCookieEmpty                                                    = errors.Wrap(ErrOauth, "oauth cookie empty")
	ErrOAuthCookieNameEmpty                                                = errors.Wrap(ErrOauth, "oauth cookie name empty")
	ErrOAuthCookieSameSiteInvalid                                          = errors.Wrap(ErrOauth, "oauth cookie same site invalid")
	ErrOAuthCookieExpireInvalid                                            = errors.Wrap(ErrOauth, "oauth cookie expire invalid")
	ErrOAuthCookieRefreshInvalid                                           = errors.Wrap(ErrOauth, "oauth cookie refresh invalid")
	ErrOAuthCookieRefreshMustBeLessThanExpire                              = errors.New("oauth cookie refresh must be less than expire")
	ErrDuplicateComponentOrJobName                                         = errors.New("duplicate component or job name")
	ErrInvalidPortNumber                                                   = errors.New("invalid port number")
	ErrDuplicateSecretName                                                 = errors.New("duplicate secret")
	DuplicateEnvVarName                                                    = errors.New("duplicate env var")
	ErrDuplicateAlias                                                      = errors.New("duplicate")
	ErrDuplicateAzureKeyVaultName                                          = errors.New("duplicate azure key vault")
	ErrSecretRefEnvVarNameConflictsWithEnvironmentVariable                 = errors.New("secret ref env var name conflicts with environment")
	ErrNotValidCidr                                                        = errors.New("not valid cidr")
	ErrNotValidIPv4Cidr                                                    = errors.New("not valid ipv 4 cidr")
	ErrInvalidEgressPortProtocol                                           = errors.New("invalid egress port protocol")
	ErrWebhook                                                             = errors.New("get webhook")
	ErrInvalidWebhookUrl                                                   = errors.Wrap(ErrWebhook, "invalid webhook")
	ErrNotAllowedSchemeInWebhookUrl                                        = errors.Wrap(ErrWebhook, "not allowed scheme in webhook")
	ErrMissingPortInWebhookUrl                                             = errors.Wrap(ErrWebhook, "missing port in webhook")
	ErrOnlyAppComponentAllowedInWebhookUrl                                 = errors.Wrap(ErrWebhook, "only app component allowed in webhook")
	ErrInvalidPortInWebhookUrl                                             = errors.Wrap(ErrWebhook, "invalid port in webhook")
	ErrInvalidUseOfPublicPortInWebhookUrl                                  = errors.New("invalid use of public port in webhook")
	ErrMissingAzureIdentity                                                = errors.New("missing identity")
)

// AliasForDNSAliasNotDefinedError Error when alias is not valid
func AliasForDNSAliasNotDefinedError() error {
	return fmt.Errorf("invalid or missing alias for dnsAlias")
}

// DuplicateAliasForDNSAliasError Error when aliases are duplicate
func DuplicateAliasForDNSAliasError(alias string) error {
	return fmt.Errorf("duplicate aliases %s in dnsAliases are not allowed", alias)
}

// EnvForDNSAliasNotDefinedError Error when env not defined
func EnvForDNSAliasNotDefinedError(env string) error {
	return fmt.Errorf("environment %s referred to by dnsAlias is not defined", env)
}

// ComponentForDNSAliasNotDefinedError Error when component not defined
func ComponentForDNSAliasNotDefinedError(component string) error {
	return fmt.Errorf("component %s referred to by dnsAlias is not defined or it is disabled", component)
}

// ComponentForDNSAliasIsNotMarkedAsPublicError Component is not marked as public
func ComponentForDNSAliasIsNotMarkedAsPublicError(component string) error {
	return fmt.Errorf("component %s referred to by dnsAlias is not marked as public", component)
}

// RadixDNSAliasAlreadyUsedByAnotherApplicationError Error when RadixDNSAlias already used by another application
func RadixDNSAliasAlreadyUsedByAnotherApplicationError(alias string) error {
	return fmt.Errorf("DNS alias %s already used by another application", alias)
}

// RadixDNSAliasIsReservedForRadixPlatformApplicationError Error when RadixDNSAlias is reserved by Radix platform for a Radix application
func RadixDNSAliasIsReservedForRadixPlatformApplicationError(alias string) error {
	return fmt.Errorf("DNS alias %s is reserved by Radix platform application", alias)
}

// RadixDNSAliasIsReservedForRadixPlatformServiceError Error when RadixDNSAlias is reserved by Radix platform for a Radix service
func RadixDNSAliasIsReservedForRadixPlatformServiceError(alias string) error {
	return fmt.Errorf("DNS alias %s is reserved by Radix platform service", alias)
}

// MissingPrivateImageHubUsernameErrorWithMessage Error when username for private image hubs is not defined
func MissingPrivateImageHubUsernameErrorWithMessage(server string) error {
	return errors.WithMessage(ErrMissingPrivateImageHubUsername, server)
}

// EnvForDNSAppAliasNotDefinedErrorWithMessage Error when env not defined
func EnvForDNSAppAliasNotDefinedErrorWithMessage(env string) error {
	return errors.WithMessagef(ErrEnvForDNSAppAliasNotDefined, "env %s referred to by dnsAppAlias is not defined", env)
}

// ComponentForDNSAppAliasNotDefinedError Error when component not defined
func ComponentForDNSAppAliasNotDefinedError(component string) error {
	return fmt.Errorf("component %s referred to by dnsAppAlias is not defined", component)
}

// ComponentForDNSAppAliasNotDefinedErrorWithMessage Error when env not defined
func ComponentForDNSAppAliasNotDefinedErrorWithMessage(component string) error {
	return errors.WithMessagef(ErrComponentForDNSAppAliasNotDefined, "component %s referred to by dnsAppAlias is not defined or it is disabled", component)
}

// EnvForDNSExternalAliasNotDefinedErrorWithMessage Error when env not defined
func EnvForDNSExternalAliasNotDefinedErrorWithMessage(env string) error {
	return errors.WithMessagef(ErrEnvForDNSExternalAliasNotDefined, "env %s referred to by dnsExternalAlias is not defined", env)
}

// ComponentForDNSExternalAliasNotDefinedErrorWithMessage Error when env not defined
func ComponentForDNSExternalAliasNotDefinedErrorWithMessage(component string) error {
	return errors.WithMessagef(ErrComponentForDNSExternalAliasNotDefined, "component %s referred to by dnsExternalAlias is not defined or it is disabled", component)
}

// ComponentForDNSExternalAliasIsNotMarkedAsPublicErrorWithMessage Component is not marked as public
func ComponentForDNSExternalAliasIsNotMarkedAsPublicErrorWithMessage(component string) error {
	return errors.WithMessagef(ErrComponentForDNSExternalAliasIsNotMarkedAsPublic, "component %s referred to by dnsExternalAlias is not marked as public", component)
}

// EnvironmentReferencedByComponentDoesNotExistErrorWithMessage Environment does not exists
func EnvironmentReferencedByComponentDoesNotExistErrorWithMessage(environment, component string) error {
	return errors.WithMessagef(ErrEnvironmentReferencedByComponentDoesNotExist, "env %s refered to by component %s is not defined", environment, component)
}

// InvalidPortNameLengthErrorWithMessage Invalid resource length
func InvalidPortNameLengthErrorWithMessage(value string) error {
	return errors.WithMessagef(ErrInvalidPortNameLength, "%s (%s) max length is %d", "port name", value, maxPortNameLength)
}

// PortNameIsRequiredForPublicComponentErrorWithMessage Port name cannot be empty
func PortNameIsRequiredForPublicComponentErrorWithMessage(publicPortName, component string) error {
	return errors.WithMessagef(ErrPortNameIsRequiredForPublicComponent, "%s port name is required for public component %s", publicPortName, component)
}

// MonitoringPortNameIsNotFoundComponentErrorWithMessage Monitoring port name not found on component
func MonitoringPortNameIsNotFoundComponentErrorWithMessage(portName, component string) error {
	return errors.WithMessagef(ErrMonitoringPortNameIsNotFoundComponent, "%s port name referred to in MonitoringConfig not found on component %s", portName, component)
}

// MultipleMatchingPortNamesErrorWithMessage Multiple matching port names
func MultipleMatchingPortNamesErrorWithMessage(matchingPortName int, publicPortName, component string) error {
	return errors.WithMessagef(ErrMultipleMatchingPortNames, "there are %d ports with name %s for component %s. Only 1 is allowed", matchingPortName, publicPortName, component)
}

// SchedulerPortCannotBeEmptyForJobErrorWithMessage Scheduler port cannot be empty for job
func SchedulerPortCannotBeEmptyForJobErrorWithMessage(jobName string) error {
	return errors.WithMessagef(ErrSchedulerPortCannotBeEmptyForJob, "scheduler port cannot be empty for %s", jobName)
}

// PayloadPathCannotBeEmptyForJobErrorWithMessage Payload path cannot be empty for job
func PayloadPathCannotBeEmptyForJobErrorWithMessage(jobName string) error {
	return errors.WithMessagef(ErrPayloadPathCannotBeEmptyForJob, "payload path cannot be empty for %s", jobName)
}

// MemoryResourceRequirementFormatErrorWithMessage Invalid memory resource requirement error
func MemoryResourceRequirementFormatErrorWithMessage(value string) error {
	return errors.WithMessagef(ErrMemoryResourceRequirementFormat, "format of memory resource requirement %s (value %s) is wrong. Value must be a valid Kubernetes quantity", "memory", value)
}

// CPUResourceRequirementFormatErrorWithMessage Invalid CPU resource requirement
func CPUResourceRequirementFormatErrorWithMessage(value string) error {
	return errors.WithMessagef(ErrCPUResourceRequirementFormat, "format of cpu resource requirement %s (value %s) is wrong. Must match regex %s", "cpu", value, cpuRegex)
}

func InvalidVerificationTypeWithMessage(verification string) error {
	return errors.WithMessagef(ErrInvalidVerificationType, "invalid VerificationType (value %s)", verification)
}

// ResourceRequestOverLimitErrorWithMessage Invalid resource requirement error
func ResourceRequestOverLimitErrorWithMessage(resource string, require string, limit string) error {
	return errors.WithMessagef(ErrResourceRequestOverLimit, "%s resource requirement (value %s) is larger than the limit (value %s)", resource, require, limit)
}

// InvalidResourceErrorWithMessage Invalid resource type
func InvalidResourceErrorWithMessage(name string) error {
	return errors.WithMessagef(ErrInvalidResource, "only support resource requirement type 'memory' and 'cpu' (not %s)", name)
}

// DuplicateExternalAliasErrorWithMessage Cannot have duplicate external alias
func DuplicateExternalAliasErrorWithMessage() error {
	return errors.WithMessage(ErrDuplicateExternalAlias, "cannot have duplicate aliases for dnsExternalAlias")
}

// InvalidBranchNameErrorWithMessage Indicates that branch name is invalid
func InvalidBranchNameErrorWithMessage(branch string) error {
	return errors.WithMessagef(ErrInvalidBranchName, "invalid branch name %s. See documentation for more info", branch)
}

// MaxReplicasForHPANotSetOrZeroErrorWithMessage Indicates that minReplicas of horizontalScaling is not set or set to 0
func MaxReplicasForHPANotSetOrZeroErrorWithMessage(component, environment string) error {
	return errors.WithMessagef(ErrMaxReplicasForHPANotSetOrZero, "maxReplicas is not set or set to 0 for component %s in environment %s. See documentation for more info", component, environment)
}

// MinReplicasGreaterThanMaxReplicasErrorWithMessage Indicates that minReplicas is greater than maxReplicas
func MinReplicasGreaterThanMaxReplicasErrorWithMessage(component, environment string) error {
	return errors.WithMessagef(ErrMinReplicasGreaterThanMaxReplicas, "minReplicas is greater than maxReplicas for component %s in environment %s. See documentation for more info", component, environment)
}

// NoScalingResourceSetErrorWithMessage Indicates that no scaling resource is set for horizontal scaling
func NoScalingResourceSetErrorWithMessage(component, environment string) error {
	return errors.WithMessagef(ErrNoScalingResourceSet, "no scaling resource is set for component %s in environment %s. See documentation for more info", component, environment)
}

func emptyVolumeMountTypeOrDriverSectionErrorWithMessage(component, environment string) error {
	return errors.WithMessagef(ErrEmptyVolumeMountTypeOrDriverSection, "non of volume mount type, blobfuse2 or azureFile options are defined in the volumeMount for component %s in environment %s. See documentation for more info", component, environment)
}

func multipleVolumeMountTypesDefinedErrorWithMessage(component, environment string) error {
	return errors.WithMessagef(ErrMultipleVolumeMountTypesDefined, "multiple volume mount types defined in the volumeMount for component %s in environment %s. See documentation for more info", component, environment)
}

func emptyVolumeMountNameOrPathErrorWithMessage(component, environment string) error {
	return errors.WithMessagef(ErrEmptyVolumeMountNameOrPath, "missing volume mount name and path of volumeMount for component %s in environment %s. See documentation for more info", component, environment)
}

func emptyVolumeMountStorageErrorWithMessage(component, environment string) error {
	return errors.WithMessagef(ErrEmptyVolumeMountStorage, "missing volume mount storage of volumeMount for component %s in environment %s. See documentation for more info", component, environment)
}

func emptyBlobFuse2VolumeMountContainerErrorWithMessage(component, environment string) error {
	return errors.WithMessagef(ErrEmptyBlobFuse2VolumeMountContainer, "missing BlobFuse2 volume mount container of volumeMount for component %s in environment %s. See documentation for more info", component, environment)
}

func unsupportedBlobFuse2VolumeMountProtocolErrorWithMessage(component, environment string) error {
	return errors.WithMessagef(ErrUnsupportedBlobFuse2VolumeMountProtocol, "unsupported BlobFuse2 volume mount protocol of volumeMount for component %s in environment %s. See documentation for more info", component, environment)
}

func duplicatePathForVolumeMountTypeWithMessage(path, volumeMountType, component, environment string) error {
	return errors.WithMessagef(ErrDuplicatePathForVolumeMountType, "duplicate path %s for volume mount type %s, for component %s in environment %s. See documentation for more info",
		path, volumeMountType, component, environment)
}

func duplicateNameForVolumeMountTypeWithMessage(name, volumeMountType, component, environment string) error {
	return errors.WithMessagef(ErrDuplicateNameForVolumeMountType, "duplicate names %s for volume mount type %s, for component %s in environment %s. See documentation for more info",
		name, volumeMountType, component, environment)
}

func unknownVolumeMountTypeErrorWithMessage(volumeMountType, component, environment string) error {
	return errors.WithMessagef(ErrunknownVolumeMountType, "not recognized volume mount type %s for component %s in environment %s. See documentation for more info",
		volumeMountType, component, environment)
}

// ApplicationNameNotLowercaseErrorWithMessage Indicates that application name contains upper case letters
func ApplicationNameNotLowercaseErrorWithMessage(appName string) error {
	return errors.WithMessagef(ErrApplicationNameNotLowercase, "application with name %s contains uppercase letters", appName)
}

// PublicImageComponentCannotHaveSourceOrDockerfileSetWithMessage Error if image is set and radix config contains src or dockerfile
func PublicImageComponentCannotHaveSourceOrDockerfileSetWithMessage(componentName string) error {
	return errors.WithMessagef(ErrPublicImageComponentCannotHaveSourceOrDockerfileSet, "component %s cannot have neither 'src' nor 'Dockerfile' set", componentName)
}

// ComponentWithTagInEnvironmentConfigForEnvironmentRequiresDynamicTagWithMessage If tag is set then the dynamic tag needs to be set on the image
func ComponentWithTagInEnvironmentConfigForEnvironmentRequiresDynamicTagWithMessage(componentName, environment string) error {
	return errors.WithMessagef(ErrComponentWithTagInEnvironmentConfigForEnvironmentRequiresDynamicTag, "component %s with image tag set on environment config for environment %s requires %s on image setting", componentName, environment, radixv1.DynamicTagNameInEnvironmentConfig)
}

// ComponentWithDynamicTagRequiresTagInEnvironmentConfigWithMessage Error if image is set with dynamic tag and tag is missing
func ComponentNameReservedSuffixErrorWithMessage(componentName, componentType, suffix string) error {
	return errors.WithMessagef(ErrComponentNameReservedSuffix, "%s %s using reserved suffix %s", componentType, componentName, suffix)
}

// SecretNameConflictsWithEnvironmentVariableWithMessage If secret name is the same as environment variable fail validation
func SecretNameConflictsWithEnvironmentVariableWithMessage(componentName, secretName string) error {
	return errors.WithMessagef(ErrSecretNameConflictsWithEnvironmentVariable, "component %s has a secret with name %s which exists as an environment variable", componentName, secretName)
}

// InvalidAppNameLengthErrorWithMessage Invalid app length
func InvalidAppNameLengthErrorWithMessage(value string) error {
	return InvalidStringValueMaxLengthErrorWithMessage("app name", value, 253)
}

// InvalidStringValueMinLengthErrorWithMessage Invalid string value min length
func InvalidStringValueMinLengthErrorWithMessage(resourceName, value string, minValue int) error {
	return errors.WithMessagef(ErrInvalidStringValueMinLength, "%s (\"%s\") min length is %d", resourceName, value, minValue)
}

// InvalidStringValueMaxLengthErrorWithMessage Invalid string value max length
func InvalidStringValueMaxLengthErrorWithMessage(resourceName, value string, maxValue int) error {
	return errors.WithMessagef(ErrInvalidStringValueMaxLength, "%s (\"%s\") max length is %d", resourceName, value, maxValue)
}

// ResourceNameCannotBeEmptyErrorWithMessage Resource name cannot be left empty
func ResourceNameCannotBeEmptyErrorWithMessage(resourceName string) error {
	return errors.WithMessagef(ErrResourceNameCannotBeEmpty, "%s is empty", resourceName)
}

// InvalidUUIDErrorWithMessage Invalid UUID
func InvalidUUIDErrorWithMessage(resourceName string, uuid string) error {
	return errors.WithMessagef(ErrInvalidUUID, "field %s does not contain a valid UUID (value: %s)", resourceName, uuid)
}

// InvalidEmailErrorWithMessage Invalid email
func InvalidEmailErrorWithMessage(resourceName, email string) error {
	return errors.WithMessagef(ErrInvalidEmail, "field %s does not contain a valid email (value: %s)", resourceName, email)
}

// InvalidResourceNameErrorWithMessage Invalid resource name
func InvalidResourceNameErrorWithMessage(resourceName, value string) error {
	return errors.WithMessagef(ErrInvalidResourceName, "%s %s can only consist of alphanumeric characters and '-'", resourceName, value)
}

// InvalidLowerCaseAlphaNumericDashResourceNameErrorWithMessage Invalid lower case alphanumeric, dash resource name error
func InvalidLowerCaseAlphaNumericDashResourceNameErrorWithMessage(resourceName, value string) error {
	return errors.WithMessagef(ErrInvalidLowerCaseAlphaNumericDashResourceName, "%s %s can only consist of lower case alphanumeric characters and '-'", resourceName, value)
}

// NoRegistrationExistsForApplicationErrorWithMessage No registration exists
func NoRegistrationExistsForApplicationErrorWithMessage(appName string) error {
	return errors.WithMessagef(ErrNoRegistrationExistsForApplication, "no application found with name %s. Name of the application in radixconfig.yaml needs to be exactly the same as used when defining the app in the console", appName)
}

func InvalidConfigBranchNameWithMessage(configBranch string) error {
	return errors.WithMessagef(ErrInvalidConfigBranchName, "config branch name is not valid (value: %s)", configBranch)
}

// *********** OAuth2 config errors ***********

func oauthRequiredPropertyEmptyErrorWithMessage(err error, componentName, environmentName, propertyName string) error {
	return errors.WithMessagef(err, "component %s in environment %s: required property %s in oauth2 configuration not set", componentName, environmentName, propertyName)
}

func oauthPropertyInvalidValueErrorWithMessage(err error, componentName, environmentName, propertyName, value string) error {
	return errors.WithMessagef(err, "component %s in environment %s: invalid value %s for property %s in oauth2 configuration", componentName, environmentName, value, propertyName)
}

func oauthRequiredPropertyEmptyWithConditionErrorWithMessage(err error, componentName, environmentName, propertyName, whenCondition string) error {
	return errors.WithMessagef(err, "component %s in environment %s: property %s in oauth2 configuration must be set when %s", componentName, environmentName, propertyName, whenCondition)
}

func oauthRequiredPropertyEmptyWithSkipDiscoveryEnabledErrorWithMessage(err error, componentName, environmentName, propertyName string) error {
	return oauthRequiredPropertyEmptyWithConditionErrorWithMessage(err, componentName, environmentName, propertyName, "oidc.skipDiscovery is true")
}

func oauthRequiredPropertyEmptyWithSessionStoreRedisErrorWithMessage(err error, componentName, environmentName, propertyName string) error {
	return oauthRequiredPropertyEmptyWithConditionErrorWithMessage(err, componentName, environmentName, propertyName, "sessionStoreType is redis")
}

func oauthCookieStoreMinimalIncorrectPropertyValueErrorWithMessage(err error, componentName, environmentName, propertyName, expectedValue string) error {
	return errors.WithMessagef(err, "component %s in environment %s: property %s in oauth configuration must be set to %s when cookieStore.minimal is true", componentName, environmentName, propertyName, expectedValue)
}

func OAuthClientIdEmptyErrorWithMessage(componentName, environmentName string) error {
	return oauthRequiredPropertyEmptyErrorWithMessage(ErrOAuthClientIdEmpty, componentName, environmentName, "clientId")
}

func OAuthRequiresPublicPortErrorWithMessage(componentName, environmentName string) error {
	return errors.WithMessagef(ErrOAuthRequiresPublicPort, "component %s in environment %s: required public port when oauth2 with clientId configuration is set", componentName, environmentName)
}

func OAuthProxyPrefixEmptyErrorWithMessage(componentName, environmentName string) error {
	return oauthRequiredPropertyEmptyErrorWithMessage(ErrOAuthProxyPrefixEmpty, componentName, environmentName, "proxyPrefix")
}

func OAuthProxyPrefixIsRootErrorWithMessage(componentName, environmentName string) error {
	return errors.WithMessagef(ErrOAuthProxyPrefixIsRoot, "component %s in environment %s: property proxyPrefix in oauth configuration cannot be set to '/' (root)", componentName, environmentName)
}

func OAuthSessionStoreTypeInvalidErrorWithMessage(componentName, environmentName string, actualSessionStoreType radixv1.SessionStoreType) error {
	return oauthPropertyInvalidValueErrorWithMessage(ErrOAuthSessionStoreTypeInvalid, componentName, environmentName, "sessionStoreType", string(actualSessionStoreType))
}

func OAuthOidcJwksUrlEmptyErrorWithMessage(componentName, environmentName string) error {
	return oauthRequiredPropertyEmptyWithSkipDiscoveryEnabledErrorWithMessage(ErrOAuthOidcJwksUrlEmpty, componentName, environmentName, "oidc.jwksUrl")
}

func OAuthLoginUrlEmptyErrorWithMessage(componentName, environmentName string) error {
	return oauthRequiredPropertyEmptyWithSkipDiscoveryEnabledErrorWithMessage(ErrOAuthLoginUrlEmpty, componentName, environmentName, "oidc.loginUrl")
}

func OAuthRedeemUrlEmptyErrorWithMessage(componentName, environmentName string) error {
	return oauthRequiredPropertyEmptyWithSkipDiscoveryEnabledErrorWithMessage(ErrOAuthRedeemUrlEmpty, componentName, environmentName, "oidc.redeemUrl")
}

func OAuthOidcEmptyErrorWithMessage(componentName, environmentName string) error {
	return oauthRequiredPropertyEmptyErrorWithMessage(ErrOAuthOidcEmpty, componentName, environmentName, "oidc")
}

func OAuthOidcSkipDiscoveryEmptyErrorWithMessage(componentName, environmentName string) error {
	return oauthRequiredPropertyEmptyErrorWithMessage(ErrOAuthOidcSkipDiscoveryEmpty, componentName, environmentName, "oidc.skipDiscovery")
}

func OAuthRedisStoreEmptyErrorWithMessage(componentName, environmentName string) error {
	return oauthRequiredPropertyEmptyWithSessionStoreRedisErrorWithMessage(ErrOAuthRedisStoreEmpty, componentName, environmentName, "redisStore")
}

func OAuthRedisStoreConnectionURLEmptyErrorWithMessage(componentName, environmentName string) error {
	return oauthRequiredPropertyEmptyWithSessionStoreRedisErrorWithMessage(ErrOAuthRedisStoreConnectionURLEmpty, componentName, environmentName, "redisStore.connectionUrl")
}

func OAuthCookieStoreMinimalIncorrectSetXAuthRequestHeadersErrorWithMessage(componentName, environmentName string) error {
	return oauthCookieStoreMinimalIncorrectPropertyValueErrorWithMessage(ErrOAuthCookieStoreMinimalIncorrectSetXAuthRequestHeaders, componentName, environmentName, "setXAuthRequestHeaders", "false")
}

func OAuthCookieStoreMinimalIncorrectSetAuthorizationHeaderErrorWithMessage(componentName, environmentName string) error {
	return oauthCookieStoreMinimalIncorrectPropertyValueErrorWithMessage(ErrOAuthCookieStoreMinimalIncorrectSetAuthorizationHeader, componentName, environmentName, "setAuthorizationHeader", "false")
}

func OAuthCookieStoreMinimalIncorrectCookieRefreshIntervalErrorWithMessage(componentName, environmentName string) error {
	return oauthCookieStoreMinimalIncorrectPropertyValueErrorWithMessage(ErrOAuthCookieStoreMinimalIncorrectCookieRefreshInterval, componentName, environmentName, "cookie.refresh", "0")
}

func OAuthCookieEmptyErrorWithMessage(componentName, environmentName string) error {
	return oauthRequiredPropertyEmptyErrorWithMessage(ErrOAuthCookieEmpty, componentName, environmentName, "cookie")
}

func OAuthCookieNameEmptyErrorWithMessage(componentName, environmentName string) error {
	return oauthRequiredPropertyEmptyErrorWithMessage(ErrOAuthCookieNameEmpty, componentName, environmentName, "cookie.name")
}

func OAuthCookieSameSiteInvalidErrorWithMessage(componentName, environmentName string, actualSameSite radixv1.CookieSameSiteType) error {
	return oauthPropertyInvalidValueErrorWithMessage(ErrOAuthCookieSameSiteInvalid, componentName, environmentName, "cookie.sameSite", string(actualSameSite))
}

func OAuthCookieExpireInvalidErrorWithMessage(componentName, environmentName, actualExpire string) error {
	return oauthPropertyInvalidValueErrorWithMessage(ErrOAuthCookieExpireInvalid, componentName, environmentName, "cookie.expire", actualExpire)
}

func OAuthCookieRefreshInvalidErrorWithMessage(componentName, environmentName, actualRefresh string) error {
	return oauthPropertyInvalidValueErrorWithMessage(ErrOAuthCookieRefreshInvalid, componentName, environmentName, "cookie.refresh", actualRefresh)
}

func OAuthCookieRefreshMustBeLessThanExpireErrorWithMessage(componentName, environmentName string) error {
	return errors.WithMessagef(ErrOAuthCookieRefreshMustBeLessThanExpire, "component %s in environment %s: property cookie.refresh in oauth configuration must be less than cookie.expire", componentName, environmentName)
}

// ********************************************

func DuplicateComponentOrJobNameErrorWithMessage(duplicates []string) error {
	return errors.WithMessagef(ErrDuplicateComponentOrJobName, "duplicate component/job names %s not allowed", duplicates)
}

// InvalidPortNumberErrorWithMessage Invalid port number
func InvalidPortNumberErrorWithMessage(value int32) error {
	return errors.WithMessagef(ErrInvalidPortNumber, "submitted configuration contains port number %d. Port numbers must be greater than or equal to %d and lower than or equal to %d", value, minimumPortNumber, maximumPortNumber)
}

func duplicateSecretNameWithMessage(name string) error {
	return errors.WithMessagef(ErrDuplicateSecretName, "secret has a duplicate name %s", name)
}

func duplicateEnvVarNameWithMessage(name string) error {
	return errors.WithMessagef(DuplicateEnvVarName, "environment variable has a duplicate name %s", name)
}

func duplicateAliasWithMessage(alias string) error {
	return errors.WithMessagef(ErrDuplicateAlias, "alias has a duplicate %s", alias)
}

func duplicateAzureKeyVaultNameWithMessage(name string) error {
	return errors.WithMessagef(ErrDuplicateAzureKeyVaultName, "azure Key vault has a duplicate name %s", name)
}

func secretRefEnvVarNameConflictsWithEnvironmentVariableWithMessage(componentName, secretRefEnvVarName string) error {
	return errors.WithMessagef(ErrSecretRefEnvVarNameConflictsWithEnvironmentVariable, "component %s has a secret reference with environment variable name %s which exists as a regular environment variable or a secret", componentName, secretRefEnvVarName)
}

func NotValidCidrErrorWithMessage(s string) error {
	return errors.WithMessagef(ErrNotValidCidr, s)
}

func NotValidIPv4CidrErrorWithMessage(ipMask string) error {
	return errors.WithMessagef(ErrNotValidIPv4Cidr, "%s is not a valid IPv4 mask", ipMask)
}

func InvalidEgressPortProtocolErrorWithMessage(protocol string, validProtocols []string) error {
	return errors.WithMessagef(ErrInvalidEgressPortProtocol, "protocol %s must be one of {%s}", protocol, strings.Join(validProtocols, ", "))
}

func InvalidWebhookUrlWithMessage(jobComponentName, environment string) error {
	return getWebhookErrorWithMessage(ErrInvalidWebhookUrl, "invalid webhook URL", jobComponentName, environment)
}

func NotAllowedSchemeInWebhookUrlWithMessage(schema, jobComponentName, environment string) error {
	return getWebhookErrorWithMessage(ErrNotAllowedSchemeInWebhookUrl, fmt.Sprintf("not allowed scheme %s in the webhook in the notifications", schema), jobComponentName, environment)
}

func MissingPortInWebhookUrlWithMessage(jobComponentName, environment string) error {
	return getWebhookErrorWithMessage(ErrMissingPortInWebhookUrl, "missing port in the webhook in the notifications", jobComponentName, environment)
}

func OnlyAppComponentAllowedInWebhookUrlWithMessage(jobComponentName, environment string) error {
	return getWebhookErrorWithMessage(ErrOnlyAppComponentAllowedInWebhookUrl, "webhook can only reference to an application component", jobComponentName, environment)
}

func InvalidPortInWebhookUrlWithMessage(webhookUrlPort, targetComponentName, jobComponentName, environment string) error {
	return getWebhookErrorWithMessage(ErrInvalidPortInWebhookUrl, fmt.Sprintf("webhook port %s does not exist in an application component %s", webhookUrlPort, targetComponentName), jobComponentName, environment)
}

func InvalidUseOfPublicPortInWebhookUrlWithMessage(webhookUrlPort, targetComponentName, jobComponentName, environment string) error {
	return getWebhookErrorWithMessage(ErrInvalidUseOfPublicPortInWebhookUrl, fmt.Sprintf("not allowed to use in the webhook a public port %s of the component %s", webhookUrlPort, targetComponentName), jobComponentName, environment)
}

func getWebhookErrorWithMessage(err error, message, jobComponentName, environment string) error {
	componentAndEnvironmentNames := fmt.Sprintf("in the job component %s", jobComponentName)
	if len(environment) == 0 {
		return errors.WithMessagef(err, "%s %s", message, componentAndEnvironmentNames)
	}
	return errors.WithMessagef(err, "%s %s in environment %s", message, componentAndEnvironmentNames, environment)
}

func MissingAzureIdentityErrorWithMessage(keyVaultName, componentName string) error {
	return errors.WithMessagef(ErrMissingAzureIdentity, "missing Azure identity for Azure Key Vault %s in the component %s", keyVaultName, componentName)
}

package radixvalidators

import (
	"fmt"
	"strings"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/pkg/errors"
)

var (
	MissingPrivateImageHubUsernameError                                 = errors.New("missing private image hub username")
	EnvForDNSAppAliasNotDefinedError                                    = errors.New("env for dnsapp alias not defined")
	ComponentForDNSAppAliasNotDefinedError                              = errors.New("component for dnsapp alias not defined")
	ExternalAliasCannotBeEmptyError                                     = errors.New("external alias cannot be empty")
	EnvForDNSExternalAliasNotDefinedError                               = errors.New("env for dnsexternal alias not defined")
	ComponentForDNSExternalAliasNotDefinedError                         = errors.New("component for dnsexternal alias not defined")
	ComponentForDNSExternalAliasIsNotMarkedAsPublicError                = errors.New("component for dnsexternal alias is not marked as public")
	EnvironmentReferencedByComponentDoesNotExistError                   = errors.New("environment referenced by component does not exist")
	InvalidPortNameLengthError                                          = errors.New("invalid port name length")
	PortSpecificationCannotBeEmptyForComponentError                     = errors.New("port specification cannot be empty for component")
	PortNameIsRequiredForPublicComponentError                           = errors.New("port name is required for public component")
	MonitoringPortNameIsNotFoundComponentError                          = errors.New("monitoring port name is not found component")
	MultipleMatchingPortNamesError                                      = errors.New("multiple matching port names")
	SchedulerPortCannotBeEmptyForJobError                               = errors.New("scheduler port cannot be empty for job")
	PayloadPathCannotBeEmptyForJobError                                 = errors.New("payload path cannot be empty for job")
	MemoryResourceRequirementFormatError                                = errors.New("memory resource requirement format")
	CPUResourceRequirementFormatError                                   = errors.New("cpuresource requirement format")
	InvalidVerificationType                                             = errors.New("invalid verification")
	ResourceRequestOverLimitError                                       = errors.New("resource request over limit")
	InvalidResourceError                                                = errors.New("invalid resource")
	DuplicateExternalAliasError                                         = errors.New("duplicate external alias")
	InvalidBranchNameError                                              = errors.New("invalid branch name")
	MaxReplicasForHPANotSetOrZeroError                                  = errors.New("max replicas for hpanot set or zero")
	MinReplicasGreaterThanMaxReplicasError                              = errors.New("min replicas greater than max replicas")
	NoScalingResourceSetError                                           = errors.New("no scaling resource set")
	EmptyVolumeMountTypeOrDriverSectionError                            = errors.New("empty volume mount type or driver section")
	MultipleVolumeMountTypesDefinedError                                = errors.New("multiple volume mount types defined")
	EmptyVolumeMountNameOrPathError                                     = errors.New("empty volume mount name or path")
	EmptyVolumeMountStorageError                                        = errors.New("empty volume mount storage")
	EmptyBlobFuse2VolumeMountContainerError                             = errors.New("empty blob fuse 2 volume mount container")
	UnsupportedBlobFuse2VolumeMountProtocolError                        = errors.New("unsupported blob fuse 2 volume mount protocol")
	DuplicatePathForVolumeMountType                                     = errors.New("duplicate path for volume mount")
	DuplicateNameForVolumeMountType                                     = errors.New("duplicate name for volume mount")
	unknownVolumeMountTypeError                                         = errors.New("unknown volume mount type")
	ApplicationNameNotLowercaseError                                    = errors.New("application name not lowercase")
	PublicImageComponentCannotHaveSourceOrDockerfileSet                 = errors.New("public image component cannot have source or dockerfile")
	ComponentWithDynamicTagRequiresTagInEnvironmentConfig               = errors.New("component with dynamic tag requires tag in environment")
	ComponentWithDynamicTagRequiresTagInEnvironmentConfigForEnvironment = errors.New("component with dynamic tag requires tag in environment config for")
	ComponentWithTagInEnvironmentConfigForEnvironmentRequiresDynamicTag = errors.New("component with tag in environment config for environment requires dynamic")
	ComponentNameReservedSuffixError                                    = errors.New("component name reserved suffix")
	SecretNameConflictsWithEnvironmentVariable                          = errors.New("secret name conflicts with environment")
	InvalidStringValueMaxLengthError                                    = errors.New("invalid string value max length")
	InvalidStringValueMinLengthError                                    = errors.New("invalid string value min length")
	ResourceNameCannotBeEmptyError                                      = errors.New("resource name cannot be empty")
	InvalidUUIDError                                                    = errors.New("invalid")
	InvalidEmailError                                                   = errors.New("invalid email")
	InvalidResourceNameError                                            = errors.New("invalid resource name")
	InvalidLowerCaseAlphaNumericDotDashResourceNameError                = errors.New("invalid lower case alpha numeric dot dash resource name")
	NoRegistrationExistsForApplicationError                             = errors.New("no registration exists for application")
	InvalidConfigBranchName                                             = errors.New("invalid config branch")
	OauthError                                                          = errors.New("oauth error")
	OAuthClientIdEmptyError                                             = errors.Wrap(OauthError, "oauth client id empty")
	OAuthProxyPrefixEmptyError                                          = errors.Wrap(OauthError, "oauth proxy prefix empty")
	OAuthProxyPrefixIsRootError                                         = errors.Wrap(OauthError, "oauth proxy prefix is root")
	OAuthSessionStoreTypeInvalidError                                   = errors.Wrap(OauthError, "oauth session store type invalid")
	OAuthOidcJwksUrlEmptyError                                          = errors.Wrap(OauthError, "oauth oidc jwks url empty")
	OAuthLoginUrlEmptyError                                             = errors.Wrap(OauthError, "oauth login url empty")
	OAuthRedeemUrlEmptyError                                            = errors.Wrap(OauthError, "oauth redeem url empty")
	OAuthOidcEmptyError                                                 = errors.Wrap(OauthError, "oauth oidc empty")
	OAuthOidcSkipDiscoveryEmptyError                                    = errors.Wrap(OauthError, "oauth oidc skip discovery empty")
	OAuthRedisStoreEmptyError                                           = errors.Wrap(OauthError, "oauth redis store empty")
	OAuthRedisStoreConnectionURLEmptyError                              = errors.Wrap(OauthError, "oauth redis store connection urlempty")
	OAuthCookieStoreMinimalIncorrectSetXAuthRequestHeadersError         = errors.Wrap(OauthError, "oauth cookie store minimal incorrect set xauth request headers")
	OAuthCookieStoreMinimalIncorrectSetAuthorizationHeaderError         = errors.Wrap(OauthError, "oauth cookie store minimal incorrect set authorization header")
	OAuthCookieStoreMinimalIncorrectCookieRefreshIntervalError          = errors.Wrap(OauthError, "oauth cookie store minimal incorrect cookie refresh interval")
	OAuthCookieEmptyError                                               = errors.Wrap(OauthError, "oauth cookie empty")
	OAuthCookieNameEmptyError                                           = errors.Wrap(OauthError, "oauth cookie name empty")
	OAuthCookieSameSiteInvalidError                                     = errors.Wrap(OauthError, "oauth cookie same site invalid")
	OAuthCookieExpireInvalidError                                       = errors.Wrap(OauthError, "oauth cookie expire invalid")
	OAuthCookieRefreshInvalidError                                      = errors.Wrap(OauthError, "oauth cookie refresh invalid")
	OAuthCookieRefreshMustBeLessThanExpireError                         = errors.New("oauth cookie refresh must be less than expire")
	DuplicateComponentOrJobNameError                                    = errors.New("duplicate component or job name")
	InvalidPortNumberError                                              = errors.New("invalid port number")
	DuplicateSecretName                                                 = errors.New("duplicate secret")
	DuplicateEnvVarName                                                 = errors.New("duplicate env var")
	DuplicateAlias                                                      = errors.New("duplicate")
	DuplicateAzureKeyVaultName                                          = errors.New("duplicate azure key vault")
	SecretRefEnvVarNameConflictsWithEnvironmentVariable                 = errors.New("secret ref env var name conflicts with environment")
	NotValidCidrError                                                   = errors.New("not valid cidr")
	NotValidIPv4CidrError                                               = errors.New("not valid ipv 4 cidr")
	InvalidEgressPortProtocolError                                      = errors.New("invalid egress port protocol")
	WebhookError                                                        = errors.New("get webhook")
	InvalidWebhookUrl                                                   = errors.Wrap(WebhookError, "invalid webhook")
	NotAllowedSchemeInWebhookUrl                                        = errors.Wrap(WebhookError, "not allowed scheme in webhook")
	MissingPortInWebhookUrl                                             = errors.Wrap(WebhookError, "missing port in webhook")
	OnlyAppComponentAllowedInWebhookUrl                                 = errors.Wrap(WebhookError, "only app component allowed in webhook")
	InvalidPortInWebhookUrl                                             = errors.Wrap(WebhookError, "invalid port in webhook")
	InvalidUseOfPublicPortInWebhookUrl                                  = errors.New("invalid use of public port in webhook")
	MissingAzureIdentityError                                           = errors.New("missing identity")
)

// MissingPrivateImageHubUsernameErrorWithMessage Error when username for private image hubs is not defined
func MissingPrivateImageHubUsernameErrorWithMessage(server string) error {
	return errors.WithMessage(MissingPrivateImageHubUsernameError, server)
}

// EnvForDNSAppAliasNotDefinedErrorWithMessage Error when env not defined
func EnvForDNSAppAliasNotDefinedErrorWithMessage(env string) error {
	return errors.WithMessagef(EnvForDNSAppAliasNotDefinedError, "env %s referred to by dnsAppAlias is not defined", env)
}

// ComponentForDNSAppAliasNotDefinedErrorWithMessage Error when env not defined
func ComponentForDNSAppAliasNotDefinedErrorWithMessage(component string) error {
	return errors.WithMessagef(ComponentForDNSAppAliasNotDefinedError, "component %s referred to by dnsAppAlias is not defined", component)
}

// EnvForDNSExternalAliasNotDefinedErrorWithMessage Error when env not defined
func EnvForDNSExternalAliasNotDefinedErrorWithMessage(env string) error {
	return errors.WithMessagef(EnvForDNSExternalAliasNotDefinedError, "env %s referred to by dnsExternalAlias is not defined", env)
}

// ComponentForDNSExternalAliasNotDefinedErrorWithMessage Error when env not defined
func ComponentForDNSExternalAliasNotDefinedErrorWithMessage(component string) error {
	return errors.WithMessagef(ComponentForDNSExternalAliasNotDefinedError, "component %s referred to by dnsExternalAlias is not defined", component)
}

// ComponentForDNSExternalAliasIsNotMarkedAsPublicErrorWithMessage Component is not marked as public
func ComponentForDNSExternalAliasIsNotMarkedAsPublicErrorWithMessage(component string) error {
	return errors.WithMessagef(ComponentForDNSExternalAliasIsNotMarkedAsPublicError, "component %s referred to by dnsExternalAlias is not marked as public", component)
}

// EnvironmentReferencedByComponentDoesNotExistErrorWithMessage Environment does not exists
func EnvironmentReferencedByComponentDoesNotExistErrorWithMessage(environment, component string) error {
	return errors.WithMessagef(EnvironmentReferencedByComponentDoesNotExistError, "env %s refered to by component %s is not defined", environment, component)
}

// InvalidPortNameLengthErrorWithMessage Invalid resource length
func InvalidPortNameLengthErrorWithMessage(value string) error {
	return errors.WithMessagef(InvalidPortNameLengthError, "%s (%s) max length is %d", "port name", value, maxPortNameLength)
}

// PortSpecificationCannotBeEmptyForComponentErrorWithMessage Port cannot be empty for component
func PortSpecificationCannotBeEmptyForComponentErrorWithMessage(component string) error {
	return errors.WithMessagef(PortSpecificationCannotBeEmptyForComponentError, "port specification cannot be empty for %s", component)
}

// PortNameIsRequiredForPublicComponentErrorWithMessage Port name cannot be empty
func PortNameIsRequiredForPublicComponentErrorWithMessage(publicPortName, component string) error {
	return errors.WithMessagef(PortNameIsRequiredForPublicComponentError, "%s port name is required for public component %s", publicPortName, component)
}

// MonitoringPortNameIsNotFoundComponentErrorWithMessage Monitoring port name not found on component
func MonitoringPortNameIsNotFoundComponentErrorWithMessage(portName, component string) error {
	return errors.WithMessagef(MonitoringPortNameIsNotFoundComponentError, "%s port name referred to in MonitoringConfig not found on component %s", portName, component)
}

// MultipleMatchingPortNamesErrorWithMessage Multiple matching port names
func MultipleMatchingPortNamesErrorWithMessage(matchingPortName int, publicPortName, component string) error {
	return errors.WithMessagef(MultipleMatchingPortNamesError, "there are %d ports with name %s for component %s. Only 1 is allowed", matchingPortName, publicPortName, component)
}

// SchedulerPortCannotBeEmptyForJobErrorWithMessage Scheduler port cannot be empty for job
func SchedulerPortCannotBeEmptyForJobErrorWithMessage(jobName string) error {
	return errors.WithMessagef(SchedulerPortCannotBeEmptyForJobError, "scheduler port cannot be empty for %s", jobName)
}

// PayloadPathCannotBeEmptyForJobErrorWithMessage Payload path cannot be empty for job
func PayloadPathCannotBeEmptyForJobErrorWithMessage(jobName string) error {
	return errors.WithMessagef(PayloadPathCannotBeEmptyForJobError, "payload path cannot be empty for %s", jobName)
}

// MemoryResourceRequirementFormatErrorWithMessage Invalid memory resource requirement error
func MemoryResourceRequirementFormatErrorWithMessage(value string) error {
	return errors.WithMessagef(MemoryResourceRequirementFormatError, "format of memory resource requirement %s (value %s) is wrong. Value must be a valid Kubernetes quantity", "memory", value)
}

// CPUResourceRequirementFormatErrorWithMessage Invalid CPU resource requirement
func CPUResourceRequirementFormatErrorWithMessage(value string) error {
	return errors.WithMessagef(CPUResourceRequirementFormatError, "format of cpu resource requirement %s (value %s) is wrong. Must match regex %s", "cpu", value, cpuRegex)
}

func InvalidVerificationTypeWithMessage(verification string) error {
	return errors.WithMessagef(InvalidVerificationType, "invalid VerificationType (value %s)", verification)
}

// ResourceRequestOverLimitErrorWithMessage Invalid resource requirement error
func ResourceRequestOverLimitErrorWithMessage(resource string, require string, limit string) error {
	return errors.WithMessagef(ResourceRequestOverLimitError, "%s resource requirement (value %s) is larger than the limit (value %s)", resource, require, limit)
}

// InvalidResourceErrorWithMessage Invalid resource type
func InvalidResourceErrorWithMessage(name string) error {
	return errors.WithMessagef(InvalidResourceError, "only support resource requirement type 'memory' and 'cpu' (not %s)", name)
}

// DuplicateExternalAliasErrorWithMessage Cannot have duplicate external alias
func DuplicateExternalAliasErrorWithMessage() error {
	return errors.WithMessage(DuplicateExternalAliasError, "cannot have duplicate aliases for dnsExternalAlias")
}

// InvalidBranchNameErrorWithMessage Indicates that branch name is invalid
func InvalidBranchNameErrorWithMessage(branch string) error {
	return errors.WithMessagef(InvalidBranchNameError, "invalid branch name %s. See documentation for more info", branch)
}

// MaxReplicasForHPANotSetOrZeroErrorWithMessage Indicates that minReplicas of horizontalScaling is not set or set to 0
func MaxReplicasForHPANotSetOrZeroErrorWithMessage(component, environment string) error {
	return errors.WithMessagef(MaxReplicasForHPANotSetOrZeroError, "maxReplicas is not set or set to 0 for component %s in environment %s. See documentation for more info", component, environment)
}

// MinReplicasGreaterThanMaxReplicasErrorWithMessage Indicates that minReplicas is greater than maxReplicas
func MinReplicasGreaterThanMaxReplicasErrorWithMessage(component, environment string) error {
	return errors.WithMessagef(MinReplicasGreaterThanMaxReplicasError, "minReplicas is greater than maxReplicas for component %s in environment %s. See documentation for more info", component, environment)
}

// NoScalingResourceSetErrorWithMessage Indicates that no scaling resource is set for horizontal scaling
func NoScalingResourceSetErrorWithMessage(component, environment string) error {
	return errors.WithMessagef(NoScalingResourceSetError, "no scaling resource is set for component %s in environment %s. See documentation for more info", component, environment)
}

func emptyVolumeMountTypeOrDriverSectionErrorWithMessage(component, environment string) error {
	return errors.WithMessagef(EmptyVolumeMountTypeOrDriverSectionError, "non of volume mount type, blobfuse2 or azureFile options are defined in the volumeMount for component %s in environment %s. See documentation for more info", component, environment)
}

func multipleVolumeMountTypesDefinedErrorWithMessage(component, environment string) error {
	return errors.WithMessagef(MultipleVolumeMountTypesDefinedError, "multiple volume mount types defined in the volumeMount for component %s in environment %s. See documentation for more info", component, environment)
}

func emptyVolumeMountNameOrPathErrorWithMessage(component, environment string) error {
	return errors.WithMessagef(EmptyVolumeMountNameOrPathError, "missing volume mount name and path of volumeMount for component %s in environment %s. See documentation for more info", component, environment)
}

func emptyVolumeMountStorageErrorWithMessage(component, environment string) error {
	return errors.WithMessagef(EmptyVolumeMountStorageError, "missing volume mount storage of volumeMount for component %s in environment %s. See documentation for more info", component, environment)
}

func emptyBlobFuse2VolumeMountContainerErrorWithMessage(component, environment string) error {
	return errors.WithMessagef(EmptyBlobFuse2VolumeMountContainerError, "missing BlobFuse2 volume mount container of volumeMount for component %s in environment %s. See documentation for more info", component, environment)
}

func unsupportedBlobFuse2VolumeMountProtocolErrorWithMessage(component, environment string) error {
	return errors.WithMessagef(UnsupportedBlobFuse2VolumeMountProtocolError, "unsupported BlobFuse2 volume mount protocol of volumeMount for component %s in environment %s. See documentation for more info", component, environment)
}

func duplicatePathForVolumeMountTypeWithMessage(path, volumeMountType, component, environment string) error {
	return errors.WithMessagef(DuplicatePathForVolumeMountType, "duplicate path %s for volume mount type %s, for component %s in environment %s. See documentation for more info",
		path, volumeMountType, component, environment)
}

func duplicateNameForVolumeMountTypeWithMessage(name, volumeMountType, component, environment string) error {
	return errors.WithMessagef(DuplicateNameForVolumeMountType, "duplicate names %s for volume mount type %s, for component %s in environment %s. See documentation for more info",
		name, volumeMountType, component, environment)
}

func unknownVolumeMountTypeErrorWithMessage(volumeMountType, component, environment string) error {
	return errors.WithMessagef(unknownVolumeMountTypeError, "not recognized volume mount type %s for component %s in environment %s. See documentation for more info",
		volumeMountType, component, environment)
}

// ApplicationNameNotLowercaseErrorWithMessage Indicates that application name contains upper case letters
func ApplicationNameNotLowercaseErrorWithMessage(appName string) error {
	return errors.WithMessagef(ApplicationNameNotLowercaseError, "application with name %s contains uppercase letters", appName)
}

// PublicImageComponentCannotHaveSourceOrDockerfileSetWithMessage Error if image is set and radix config contains src or dockerfile
func PublicImageComponentCannotHaveSourceOrDockerfileSetWithMessage(componentName string) error {
	return errors.WithMessagef(PublicImageComponentCannotHaveSourceOrDockerfileSet, "component %s cannot have neither 'src' nor 'Dockerfile' set", componentName)
}

// ComponentWithDynamicTagRequiresTagInEnvironmentConfigWithMessage Error if image is set with dynamic tag and tag is missing
func ComponentWithDynamicTagRequiresTagInEnvironmentConfigWithMessage(componentName string) error {
	return errors.WithMessagef(ComponentWithDynamicTagRequiresTagInEnvironmentConfig, "component %s with %s on image requires an image tag set on environment config",
		componentName, radixv1.DynamicTagNameInEnvironmentConfig)
}

// ComponentWithDynamicTagRequiresTagInEnvironmentConfigForEnvironmentWithMessage Error if image is set with dynamic tag and tag is missing
func ComponentWithDynamicTagRequiresTagInEnvironmentConfigForEnvironmentWithMessage(componentName, environment string) error {
	return errors.WithMessagef(ComponentWithDynamicTagRequiresTagInEnvironmentConfigForEnvironment, "component %s with %s on image requires an image tag set on environment config for environment %s", componentName, radixv1.DynamicTagNameInEnvironmentConfig, environment)
}

// ComponentWithTagInEnvironmentConfigForEnvironmentRequiresDynamicTagWithMessage If tag is set then the dynamic tag needs to be set on the image
func ComponentWithTagInEnvironmentConfigForEnvironmentRequiresDynamicTagWithMessage(componentName, environment string) error {
	return errors.WithMessagef(ComponentWithTagInEnvironmentConfigForEnvironmentRequiresDynamicTag, "component %s with image tag set on environment config for environment %s requires %s on image setting", componentName, environment, radixv1.DynamicTagNameInEnvironmentConfig)
}

// ComponentWithDynamicTagRequiresTagInEnvironmentConfigWithMessage Error if image is set with dynamic tag and tag is missing
func ComponentNameReservedSuffixErrorWithMessage(componentName, componentType, suffix string) error {
	return errors.WithMessagef(ComponentNameReservedSuffixError, "%s %s using reserved suffix %s", componentType, componentName, suffix)
}

// SecretNameConflictsWithEnvironmentVariableWithMessage If secret name is the same as environment variable fail validation
func SecretNameConflictsWithEnvironmentVariableWithMessage(componentName, secretName string) error {
	return errors.WithMessagef(SecretNameConflictsWithEnvironmentVariable, "component %s has a secret with name %s which exists as an environment variable", componentName, secretName)
}

// InvalidAppNameLengthErrorWithMessage Invalid app length
func InvalidAppNameLengthErrorWithMessage(value string) error {
	return InvalidStringValueMaxLengthErrorWithMessage("app name", value, 253)
}

// InvalidStringValueMinLengthErrorWithMessage Invalid string value min length
func InvalidStringValueMinLengthErrorWithMessage(resourceName, value string, minValue int) error {
	return errors.WithMessagef(InvalidStringValueMinLengthError, "%s (\"%s\") min length is %d", resourceName, value, minValue)
}

// InvalidStringValueMaxLengthErrorWithMessage Invalid string value max length
func InvalidStringValueMaxLengthErrorWithMessage(resourceName, value string, maxValue int) error {
	return errors.WithMessagef(InvalidStringValueMaxLengthError, "%s (\"%s\") max length is %d", resourceName, value, maxValue)
}

// ResourceNameCannotBeEmptyErrorWithMessage Resource name cannot be left empty
func ResourceNameCannotBeEmptyErrorWithMessage(resourceName string) error {
	return errors.WithMessagef(ResourceNameCannotBeEmptyError, "%s cannot be empty", resourceName)
}

// InvalidUUIDErrorWithMessage Invalid UUID
func InvalidUUIDErrorWithMessage(resourceName string, uuid string) error {
	return errors.WithMessagef(InvalidUUIDError, "field %s does not contain a valid UUID (value: %s)", resourceName, uuid)
}

// InvalidEmailErrorWithMessage Invalid email
func InvalidEmailErrorWithMessage(resourceName, email string) error {
	return errors.WithMessagef(InvalidEmailError, "field %s does not contain a valid email (value: %s)", resourceName, email)
}

// InvalidResourceNameErrorWithMessage Invalid resource name
func InvalidResourceNameErrorWithMessage(resourceName, value string) error {
	return errors.WithMessagef(InvalidResourceNameError, "%s %s can only consist of alphanumeric characters, '.' and '-'", resourceName, value)
}

// InvalidLowerCaseAlphaNumericDotDashResourceNameErrorWithMessage Invalid lower case alpha-numeric, dot, dash resource name error
func InvalidLowerCaseAlphaNumericDotDashResourceNameErrorWithMessage(resourceName, value string) error {
	return errors.WithMessagef(InvalidLowerCaseAlphaNumericDotDashResourceNameError, "%s %s can only consist of lower case alphanumeric characters, '.' and '-'", resourceName, value)
}

// NoRegistrationExistsForApplicationErrorWithMessage No registration exists
func NoRegistrationExistsForApplicationErrorWithMessage(appName string) error {
	return errors.WithMessagef(NoRegistrationExistsForApplicationError, "no application found with name %s. Name of the application in radixconfig.yaml needs to be exactly the same as used when defining the app in the console", appName)
}

func InvalidConfigBranchNameWithMessage(configBranch string) error {
	return errors.WithMessagef(InvalidConfigBranchName, "config branch name is not valid (value: %s)", configBranch)
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
	return oauthRequiredPropertyEmptyErrorWithMessage(OAuthClientIdEmptyError, componentName, environmentName, "clientId")
}

func OAuthProxyPrefixEmptyErrorWithMessage(componentName, environmentName string) error {
	return oauthRequiredPropertyEmptyErrorWithMessage(OAuthProxyPrefixEmptyError, componentName, environmentName, "proxyPrefix")
}

func OAuthProxyPrefixIsRootErrorWithMessage(componentName, environmentName string) error {
	return errors.WithMessagef(OAuthProxyPrefixIsRootError, "component %s in environment %s: property proxyPrefix in oauth configuration cannot be set to '/' (root)", componentName, environmentName)
}

func OAuthSessionStoreTypeInvalidErrorWithMessage(componentName, environmentName string, actualSessionStoreType radixv1.SessionStoreType) error {
	return oauthPropertyInvalidValueErrorWithMessage(OAuthSessionStoreTypeInvalidError, componentName, environmentName, "sessionStoreType", string(actualSessionStoreType))
}

func OAuthOidcJwksUrlEmptyErrorWithMessage(componentName, environmentName string) error {
	return oauthRequiredPropertyEmptyWithSkipDiscoveryEnabledErrorWithMessage(OAuthOidcJwksUrlEmptyError, componentName, environmentName, "oidc.jwksUrl")
}

func OAuthLoginUrlEmptyErrorWithMessage(componentName, environmentName string) error {
	return oauthRequiredPropertyEmptyWithSkipDiscoveryEnabledErrorWithMessage(OAuthLoginUrlEmptyError, componentName, environmentName, "oidc.loginUrl")
}

func OAuthRedeemUrlEmptyErrorWithMessage(componentName, environmentName string) error {
	return oauthRequiredPropertyEmptyWithSkipDiscoveryEnabledErrorWithMessage(OAuthRedeemUrlEmptyError, componentName, environmentName, "oidc.redeemUrl")
}

func OAuthOidcEmptyErrorWithMessage(componentName, environmentName string) error {
	return oauthRequiredPropertyEmptyErrorWithMessage(OAuthOidcEmptyError, componentName, environmentName, "oidc")
}

func OAuthOidcSkipDiscoveryEmptyErrorWithMessage(componentName, environmentName string) error {
	return oauthRequiredPropertyEmptyErrorWithMessage(OAuthOidcSkipDiscoveryEmptyError, componentName, environmentName, "oidc.skipDiscovery")
}

func OAuthRedisStoreEmptyErrorWithMessage(componentName, environmentName string) error {
	return oauthRequiredPropertyEmptyWithSessionStoreRedisErrorWithMessage(OAuthRedisStoreEmptyError, componentName, environmentName, "redisStore")
}

func OAuthRedisStoreConnectionURLEmptyErrorWithMessage(componentName, environmentName string) error {
	return oauthRequiredPropertyEmptyWithSessionStoreRedisErrorWithMessage(OAuthRedisStoreConnectionURLEmptyError, componentName, environmentName, "redisStore.connectionUrl")
}

func OAuthCookieStoreMinimalIncorrectSetXAuthRequestHeadersErrorWithMessage(componentName, environmentName string) error {
	return oauthCookieStoreMinimalIncorrectPropertyValueErrorWithMessage(OAuthCookieStoreMinimalIncorrectSetXAuthRequestHeadersError, componentName, environmentName, "setXAuthRequestHeaders", "false")
}

func OAuthCookieStoreMinimalIncorrectSetAuthorizationHeaderErrorWithMessage(componentName, environmentName string) error {
	return oauthCookieStoreMinimalIncorrectPropertyValueErrorWithMessage(OAuthCookieStoreMinimalIncorrectSetAuthorizationHeaderError, componentName, environmentName, "setAuthorizationHeader", "false")
}

func OAuthCookieStoreMinimalIncorrectCookieRefreshIntervalErrorWithMessage(componentName, environmentName string) error {
	return oauthCookieStoreMinimalIncorrectPropertyValueErrorWithMessage(OAuthCookieStoreMinimalIncorrectCookieRefreshIntervalError, componentName, environmentName, "cookie.refresh", "0")
}

func OAuthCookieEmptyErrorWithMessage(componentName, environmentName string) error {
	return oauthRequiredPropertyEmptyErrorWithMessage(OAuthCookieEmptyError, componentName, environmentName, "cookie")
}

func OAuthCookieNameEmptyErrorWithMessage(componentName, environmentName string) error {
	return oauthRequiredPropertyEmptyErrorWithMessage(OAuthCookieNameEmptyError, componentName, environmentName, "cookie.name")
}

func OAuthCookieSameSiteInvalidErrorWithMessage(componentName, environmentName string, actualSameSite radixv1.CookieSameSiteType) error {
	return oauthPropertyInvalidValueErrorWithMessage(OAuthCookieSameSiteInvalidError, componentName, environmentName, "cookie.sameSite", string(actualSameSite))
}

func OAuthCookieExpireInvalidErrorWithMessage(componentName, environmentName, actualExpire string) error {
	return oauthPropertyInvalidValueErrorWithMessage(OAuthCookieExpireInvalidError, componentName, environmentName, "cookie.expire", actualExpire)
}

func OAuthCookieRefreshInvalidErrorWithMessage(componentName, environmentName, actualRefresh string) error {
	return oauthPropertyInvalidValueErrorWithMessage(OAuthCookieRefreshInvalidError, componentName, environmentName, "cookie.refresh", actualRefresh)
}

func OAuthCookieRefreshMustBeLessThanExpireErrorWithMessage(componentName, environmentName string) error {
	return errors.WithMessagef(OAuthCookieRefreshMustBeLessThanExpireError, "component %s in environment %s: property cookie.refresh in oauth configuration must be less than cookie.expire", componentName, environmentName)
}

// ********************************************

func DuplicateComponentOrJobNameErrorWithMessage(duplicates []string) error {
	return errors.WithMessagef(DuplicateComponentOrJobNameError, "duplicate component/job names %s not allowed", duplicates)
}

// InvalidPortNumberErrorWithMessage Invalid port number
func InvalidPortNumberErrorWithMessage(value int32) error {
	return errors.WithMessagef(InvalidPortNumberError, "submitted configuration contains port number %d. Port numbers must be greater than or equal to %d and lower than or equal to %d", value, minimumPortNumber, maximumPortNumber)
}

func duplicateSecretNameWithMessage(name string) error {
	return errors.WithMessagef(DuplicateSecretName, "secret has a duplicate name %s", name)
}

func duplicateEnvVarNameWithMessage(name string) error {
	return errors.WithMessagef(DuplicateEnvVarName, "environment variable has a duplicate name %s", name)
}

func duplicateAliasWithMessage(alias string) error {
	return errors.WithMessagef(DuplicateAlias, "alias has a duplicate %s", alias)
}

func duplicateAzureKeyVaultNameWithMessage(name string) error {
	return errors.WithMessagef(DuplicateAzureKeyVaultName, "azure Key vault has a duplicate name %s", name)
}

func secretRefEnvVarNameConflictsWithEnvironmentVariableWithMessage(componentName, secretRefEnvVarName string) error {
	return errors.WithMessagef(SecretRefEnvVarNameConflictsWithEnvironmentVariable, "component %s has a secret reference with environment variable name %s which exists as a regular environment variable or a secret", componentName, secretRefEnvVarName)
}

func NotValidCidrErrorWithMessage(s string) error {
	return errors.WithMessagef(NotValidCidrError, s)
}

func NotValidIPv4CidrErrorWithMessage(ipMask string) error {
	return errors.WithMessagef(NotValidIPv4CidrError, "%s is not a valid IPv4 mask", ipMask)
}

func InvalidEgressPortProtocolErrorWithMessage(protocol string, validProtocols []string) error {
	return errors.WithMessagef(InvalidEgressPortProtocolError, "protocol %s must be one of {%s}", protocol, strings.Join(validProtocols, ", "))
}

func InvalidWebhookUrlWithMessage(jobComponentName, environment string) error {
	return getWebhookErrorWithMessage(InvalidWebhookUrl, "invalid webhook URL", jobComponentName, environment)
}

func NotAllowedSchemeInWebhookUrlWithMessage(schema, jobComponentName, environment string) error {
	return getWebhookErrorWithMessage(NotAllowedSchemeInWebhookUrl, fmt.Sprintf("not allowed scheme %s in the webhook in the notifications", schema), jobComponentName, environment)
}

func MissingPortInWebhookUrlWithMessage(jobComponentName, environment string) error {
	return getWebhookErrorWithMessage(MissingPortInWebhookUrl, "missing port in the webhook in the notifications", jobComponentName, environment)
}

func OnlyAppComponentAllowedInWebhookUrlWithMessage(jobComponentName, environment string) error {
	return getWebhookErrorWithMessage(OnlyAppComponentAllowedInWebhookUrl, "webhook can only reference to an application component", jobComponentName, environment)
}

func InvalidPortInWebhookUrlWithMessage(webhookUrlPort, targetComponentName, jobComponentName, environment string) error {
	return getWebhookErrorWithMessage(InvalidPortInWebhookUrl, fmt.Sprintf("webhook port %s does not exist in an application component %s", webhookUrlPort, targetComponentName), jobComponentName, environment)
}

func InvalidUseOfPublicPortInWebhookUrlWithMessage(webhookUrlPort, targetComponentName, jobComponentName, environment string) error {
	return getWebhookErrorWithMessage(InvalidUseOfPublicPortInWebhookUrl, fmt.Sprintf("not allowed to use in the webhook a public port %s of the component %s", webhookUrlPort, targetComponentName), jobComponentName, environment)
}

func getWebhookErrorWithMessage(err error, message, jobComponentName, environment string) error {
	componentAndEnvironmentNames := fmt.Sprintf("in the job component %s", jobComponentName)
	if len(environment) == 0 {
		return errors.WithMessagef(err, "%s %s", message, componentAndEnvironmentNames)
	}
	return errors.WithMessagef(err, "%s %s in environment %s", message, componentAndEnvironmentNames, environment)
}

func MissingAzureIdentityErrorWithMessage(keyVaultName, componentName string) error {
	return errors.WithMessagef(MissingAzureIdentityError, "missing Azure identity for Azure Key Vault %s in the component %s", keyVaultName, componentName)
}

package radixvalidators

import (
	"errors"
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// MissingPrivateImageHubUsernameError Error when username for private image hubs is not defined
func MissingPrivateImageHubUsernameError(server string) error {
	return fmt.Errorf("username is required for private image hub %s", server)
}

// MissingPrivateImageHubEmailError Error when email for private image hubs is not defined
func MissingPrivateImageHubEmailError(server string) error {
	return fmt.Errorf("email is required for private image hub %s", server)
}

// EnvForDNSAppAliasNotDefinedError Error when env not defined
func EnvForDNSAppAliasNotDefinedError(env string) error {
	return fmt.Errorf("env %s referred to by dnsAppAlias is not defined", env)
}

// ComponentForDNSAppAliasNotDefinedError Error when env not defined
func ComponentForDNSAppAliasNotDefinedError(component string) error {
	return fmt.Errorf("component %s referred to by dnsAppAlias is not defined", component)
}

// ExternalAliasCannotBeEmptyError Structure cannot be left empty
func ExternalAliasCannotBeEmptyError() error {
	return errors.New("external alias cannot be empty")
}

// EnvForDNSExternalAliasNotDefinedError Error when env not defined
func EnvForDNSExternalAliasNotDefinedError(env string) error {
	return fmt.Errorf("env %s referred to by dnsExternalAlias is not defined", env)
}

// ComponentForDNSExternalAliasNotDefinedError Error when env not defined
func ComponentForDNSExternalAliasNotDefinedError(component string) error {
	return fmt.Errorf("component %s referred to by dnsExternalAlias is not defined", component)
}

// ComponentForDNSExternalAliasIsNotMarkedAsPublicError Component is not marked as public
func ComponentForDNSExternalAliasIsNotMarkedAsPublicError(component string) error {
	return fmt.Errorf("component %s referred to by dnsExternalAlias is not marked as public", component)
}

// EnvironmentReferencedByComponentDoesNotExistError Environment does not exists
func EnvironmentReferencedByComponentDoesNotExistError(environment, component string) error {
	return fmt.Errorf("env %s refered to by component %s is not defined", environment, component)
}

// InvalidPortNameLengthError Invalid resource length
func InvalidPortNameLengthError(value string) error {
	return fmt.Errorf("%s (%s) max length is %d", "port name", value, maxPortNameLength)
}

// PortSpecificationCannotBeEmptyForComponentError Port cannot be empty for component
func PortSpecificationCannotBeEmptyForComponentError(component string) error {
	return fmt.Errorf("port specification cannot be empty for %s", component)
}

// PortNameIsRequiredForPublicComponentError Port name cannot be empty
func PortNameIsRequiredForPublicComponentError(publicPortName, component string) error {
	return fmt.Errorf("%s port name is required for public component %s", publicPortName, component)
}

// MultipleMatchingPortNamesError Multiple matching port names
func MultipleMatchingPortNamesError(matchingPortName int, publicPortName, component string) error {
	return fmt.Errorf("there are %d ports with name %s for component %s. Only 1 is allowed", matchingPortName, publicPortName, component)
}

// SchedulerPortCannotBeEmptyForJobError Scheduler port cannot be empty for job
func SchedulerPortCannotBeEmptyForJobError(jobName string) error {
	return fmt.Errorf("scheduler port cannot be empty for %s", jobName)
}

// PayloadPathCannotBeEmptyForJobError Payload path cannot be empty for job
func PayloadPathCannotBeEmptyForJobError(jobName string) error {
	return fmt.Errorf("payload path cannot be empty for %s", jobName)
}

// MemoryResourceRequirementFormatError Invalid memory resource requirement error
func MemoryResourceRequirementFormatError(value string) error {
	return fmt.Errorf("format of memory resource requirement %s (value %s) is wrong. Value must be a valid Kubernetes quantity", "memory", value)
}

// CPUResourceRequirementFormatError Invalid CPU resource requirement
func CPUResourceRequirementFormatError(value string) error {
	return fmt.Errorf("format of cpu resource requirement %s (value %s) is wrong. Must match regex '%s'", "cpu", value, cpuRegex)
}

func InvalidVerificationType(verification string) error {
	return fmt.Errorf("invalid VerificationType (value %s)", verification)
}

// ResourceRequestOverLimitError Invalid resource requirement error
func ResourceRequestOverLimitError(resource string, require string, limit string) error {
	return fmt.Errorf("%s resource requirement (value %s) is larger than the limit (value %s)", resource, require, limit)
}

// InvalidResourceError Invalid resource type
func InvalidResourceError(name string) error {
	return fmt.Errorf("only support resource requirement type 'memory' and 'cpu' (not '%s')", name)
}

// DuplicateExternalAliasError Cannot have duplicate external alias
func DuplicateExternalAliasError() error {
	return errors.New("cannot have duplicate aliases for dnsExternalAlias")
}

// InvalidBranchNameError Indicates that branch name is invalid
func InvalidBranchNameError(branch string) error {
	return fmt.Errorf("invalid branch name %s. See documentation for more info", branch)
}

// MaxReplicasForHPANotSetOrZeroError Indicates that minReplicas of horizontalScaling is not set or set to 0
func MaxReplicasForHPANotSetOrZeroError(component, environment string) error {
	return fmt.Errorf("maxReplicas is not set or set to 0 for component %s in environment %s. See documentation for more info", component, environment)
}

// MinReplicasGreaterThanMaxReplicasError Indicates that minReplicas is greater than maxReplicas
func MinReplicasGreaterThanMaxReplicasError(component, environment string) error {
	return fmt.Errorf("minReplicas is greater than maxReplicas for component %s in environment %s. See documentation for more info", component, environment)
}

func emptyVolumeMountTypeContainerNameOrTempPathError(component, environment string) error {
	return fmt.Errorf("volume mount type, name, containers and temp-path of volumeMount for component %s in environment %s cannot be empty. See documentation for more info", component, environment)
}

func duplicatePathForVolumeMountType(path, volumeMountType, component, environment string) error {
	return fmt.Errorf("duplicate path %s for volume mount type %s, for component %s in environment %s. See documentation for more info",
		path, volumeMountType, component, environment)
}

func duplicateNameForVolumeMountType(name, volumeMountType, component, environment string) error {
	return fmt.Errorf("duplicate names %s for volume mount type %s, for component %s in environment %s. See documentation for more info",
		name, volumeMountType, component, environment)
}

func unknownVolumeMountTypeError(volumeMountType, component, environment string) error {
	return fmt.Errorf("not recognized volume mount type %s for component %s in environment %s. See documentation for more info",
		volumeMountType, component, environment)
}

//ApplicationNameNotLowercaseError Indicates that application name contains upper case letters
func ApplicationNameNotLowercaseError(appName string) error {
	return fmt.Errorf("application with name %s contains uppercase letters", appName)
}

// PublicImageComponentCannotHaveSourceOrDockerfileSet Error if image is set and radix config contains src or dockerfile
func PublicImageComponentCannotHaveSourceOrDockerfileSet(componentName string) error {
	return fmt.Errorf("component %s cannot have neither 'src' nor 'Dockerfile' set", componentName)
}

// ComponentWithDynamicTagRequiresTagInEnvironmentConfig Error if image is set with dynamic tag and tag is missing
func ComponentWithDynamicTagRequiresTagInEnvironmentConfig(componentName string) error {
	return fmt.Errorf("component %s with %s on image requires an image tag set on environment config",
		componentName, radixv1.DynamicTagNameInEnvironmentConfig)
}

// ComponentWithDynamicTagRequiresTagInEnvironmentConfigForEnvironment Error if image is set with dynamic tag and tag is missing
func ComponentWithDynamicTagRequiresTagInEnvironmentConfigForEnvironment(componentName, environment string) error {
	return fmt.Errorf(
		"component %s with %s on image requires an image tag set on environment config for environment %s",
		componentName, radixv1.DynamicTagNameInEnvironmentConfig, environment)
}

// ComponentWithTagInEnvironmentConfigForEnvironmentRequiresDynamicTag If tag is set then the dynamic tag needs to be set on the image
func ComponentWithTagInEnvironmentConfigForEnvironmentRequiresDynamicTag(componentName, environment string) error {
	return fmt.Errorf(
		"component %s with image tag set on environment config for environment %s requires %s on image setting",
		componentName, environment, radixv1.DynamicTagNameInEnvironmentConfig)
}

// SecretNameConflictsWithEnvironmentVariable If secret name is the same as environment variable fail validation
func SecretNameConflictsWithEnvironmentVariable(componentName, secretName string) error {
	return fmt.Errorf(
		"component %s has a secret with name %s which exists as an environment variable",
		componentName, secretName)
}

// InvalidAppNameLengthError Invalid app length
func InvalidAppNameLengthError(value string) error {
	return InvalidStringValueMaxLengthError("app name", value, 253)
}

// AppNameCannotBeEmptyError App name cannot be empty
func AppNameCannotBeEmptyError() error {
	return ResourceNameCannotBeEmptyError("app name")
}

// InvalidStringValueMinLengthError Invalid string value min length
func InvalidStringValueMinLengthError(resourceName, value string, minValue int) error {
	return fmt.Errorf("%s (\"%s\") min length is %d", resourceName, value, minValue)
}

// InvalidStringValueMaxLengthError Invalid string value max length
func InvalidStringValueMaxLengthError(resourceName, value string, maxValue int) error {
	return fmt.Errorf("%s (\"%s\") max length is %d", resourceName, value, maxValue)
}

// ResourceNameCannotBeEmptyError Resource name cannot be left empty
func ResourceNameCannotBeEmptyError(resourceName string) error {
	return fmt.Errorf("%s cannot be empty", resourceName)
}

// InvalidEmailError Invalid email
func InvalidEmailError(resourceName, email string) error {
	return fmt.Errorf("field %s does not contain a valid email (value: %s)", resourceName, email)
}

// InvalidResourceNameError Invalid resource name
func InvalidResourceNameError(resourceName, value string) error {
	return fmt.Errorf("%s %s can only consist of alphanumeric characters, '.' and '-'", resourceName, value)
}

//InvalidLowerCaseAlphaNumericDotDashResourceNameError Invalid lower case alpha-numeric, dot, dash resource name error
func InvalidLowerCaseAlphaNumericDotDashResourceNameError(resourceName, value string) error {
	return fmt.Errorf("%s %s can only consist of lower case alphanumeric characters, '.' and '-'", resourceName, value)
}

// NoRegistrationExistsForApplicationError No registration exists
func NoRegistrationExistsForApplicationError(appName string) error {
	return fmt.Errorf("no application found with name %s. Name of the application in radixconfig.yaml needs to be exactly the same as used when defining the app in the console", appName)
}

func InvalidConfigBranchName(configBranch string) error {
	return fmt.Errorf("config branch name is not valid (value: %s)", configBranch)
}

func InvalidOAuthSessionStoreType(actualSessionStoreType string) error {
	return fmt.Errorf("invalid session store type '%s'", actualSessionStoreType)
}

func InvalidOAuthCookieSameSite(actualSameSite string) error {
	return fmt.Errorf("invalid cookie samesite '%s'", actualSameSite)
}

func InvalidOAuthCookieExpire(actualExpire string) error {
	return fmt.Errorf("invalid cookie expire timeframe '%s'", actualExpire)
}

func InvalidOAuthCookieRefresh(actualRefresh string) error {
	return fmt.Errorf("invalid cookie refresh duration '%s'", actualRefresh)
}

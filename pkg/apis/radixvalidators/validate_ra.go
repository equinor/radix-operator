package radixvalidators

import (
	"errors"
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"regexp"
	"strings"
	"unicode"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/branch"
	errorUtils "github.com/equinor/radix-operator/pkg/apis/utils/errors"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	maxPortNameLength = 15
	minimumPortNumber = 1024
	cpuRegex          = "^[0-9]+m$"
)

// MissingPrivateImageHubUsernameError Error when username for private image hubs is not defined
func MissingPrivateImageHubUsernameError(server string) error {
	return fmt.Errorf("Username is required for private image hub %s", server)
}

// MissingPrivateImageHubEmailError Error when email for private image hubs is not defined
func MissingPrivateImageHubEmailError(server string) error {
	return fmt.Errorf("Email is required for private image hub %s", server)
}

// EnvForDNSAppAliasNotDefinedError Error when env not defined
func EnvForDNSAppAliasNotDefinedError(env string) error {
	return fmt.Errorf("Env %s refered to by dnsAppAlias is not defined", env)
}

// ComponentForDNSAppAliasNotDefinedError Error when env not defined
func ComponentForDNSAppAliasNotDefinedError(component string) error {
	return fmt.Errorf("Component %s refered to by dnsAppAlias is not defined", component)
}

// ExternalAliasCannotBeEmptyError Structure cannot be left empty
func ExternalAliasCannotBeEmptyError() error {
	return errors.New("External alias cannot be empty")
}

// EnvForDNSExternalAliasNotDefinedError Error when env not defined
func EnvForDNSExternalAliasNotDefinedError(env string) error {
	return fmt.Errorf("Env %s refered to by dnsExternalAlias is not defined", env)
}

// ComponentForDNSExternalAliasNotDefinedError Error when env not defined
func ComponentForDNSExternalAliasNotDefinedError(component string) error {
	return fmt.Errorf("Component %s refered to by dnsExternalAlias is not defined", component)
}

// ComponentForDNSExternalAliasIsNotMarkedAsPublicError Component is not marked as public
func ComponentForDNSExternalAliasIsNotMarkedAsPublicError(component string) error {
	return fmt.Errorf("Component %s refered to by dnsExternalAlias is not marked as public", component)
}

// EnvironmentReferencedByComponentDoesNotExistError Environment does not exists
func EnvironmentReferencedByComponentDoesNotExistError(environment, component string) error {
	return fmt.Errorf("Env %s refered to by component %s is not defined", environment, component)
}

// InvalidPortNameLengthError Invalid resource length
func InvalidPortNameLengthError(value string) error {
	return fmt.Errorf("%s (%s) max length is %d", "port name", value, maxPortNameLength)
}

// InvalidPortNumberError Invalid port number
func InvalidPortNumberError(value int32) error {
	return fmt.Errorf("Port number is %d, which is a privileged port. Minimum port number is %d.", value, minimumPortNumber)
}

// PortSpecificationCannotBeEmptyForComponentError Port cannot be empty for component
func PortSpecificationCannotBeEmptyForComponentError(component string) error {
	return fmt.Errorf("Port specification cannot be empty for %s", component)
}

// PortNameIsRequiredForPublicComponentError Port name cannot be empty
func PortNameIsRequiredForPublicComponentError(publicPortName, component string) error {
	return fmt.Errorf("%s port name is required for public component %s", publicPortName, component)
}

// MultipleMatchingPortNamesError Multiple matching port names
func MultipleMatchingPortNamesError(matchingPortName int, publicPortName, component string) error {
	return fmt.Errorf("There are %d ports with name %s for component %s. Only 1 is allowed", matchingPortName, publicPortName, component)
}

// SchedulerPortCannotBeEmptyForJobError Scheduler port cannot be empty for job
func SchedulerPortCannotBeEmptyForJobError(jobName string) error {
	return fmt.Errorf("Scheduler port cannot be empty for %s", jobName)
}

// PayloadPathCannotBeEmptyForJobError Payload path cannot be empty for job
func PayloadPathCannotBeEmptyForJobError(jobName string) error {
	return fmt.Errorf("Payload path cannot be empty for %s", jobName)
}

// MemoryResourceRequirementFormatError Invalid memory resource requirement error
func MemoryResourceRequirementFormatError(value string) error {
	return fmt.Errorf("Format of memory resource requirement %s (value %s) is wrong. Value must be a valid Kubernetes quantity", "memory", value)
}

// CPUResourceRequirementFormatError Invalid CPU resource requirement
func CPUResourceRequirementFormatError(value string) error {
	return fmt.Errorf("Format of cpu resource requirement %s (value %s) is wrong. Must match regex '%s'", "cpu", value, cpuRegex)
}

func InvalidVerificationType(verification string) error {
	return fmt.Errorf("Invalid VerificationType (value %s)", verification)
}

// ResourceRequestOverLimitError Invalid resource requirement error
func ResourceRequestOverLimitError(resource string, require string, limit string) error {
	return fmt.Errorf("%s resource requirement (value %s) is larger than the limit (value %s)", resource, require, limit)
}

// InvalidResourceError Invalid resource type
func InvalidResourceError(name string) error {
	return fmt.Errorf("Only support resource requirement type 'memory' and 'cpu' (not '%s')", name)
}

// DuplicateExternalAliasError Cannot have duplicate external alias
func DuplicateExternalAliasError() error {
	return errors.New("Cannot have duplicate aliases for dnsExternalAlias")
}

// InvalidBranchNameError Indicates that branch name is invalid
func InvalidBranchNameError(branch string) error {
	return fmt.Errorf("Invalid branch name %s. See documentation for more info", branch)
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

func duplicateVolumeMountType(component, environment string) error {
	return fmt.Errorf("duplicate type of volume mount type for component %s in environment %s. See documentation for more info", component, environment)
}

func duplicateContainerForVolumeMountType(storage, volumeMountType, component, environment string) error {
	return fmt.Errorf("duplicate containers %s for volume mount type %s, for component %s in environment %s. See documentation for more info",
		storage, volumeMountType, component, environment)
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

// CanRadixApplicationBeInserted Checks if application config is valid. Returns a single error, if this is the case
func CanRadixApplicationBeInserted(client radixclient.Interface, app *radixv1.RadixApplication) (bool, error) {
	isValid, errs := CanRadixApplicationBeInsertedErrors(client, app)
	if isValid {
		return true, nil
	}

	return false, errorUtils.Concat(errs)
}

// IsApplicationNameLowercase checks if the application name has any uppercase letters
func IsApplicationNameLowercase(appName string) (bool, error) {
	for _, r := range appName {
		if unicode.IsUpper(r) && unicode.IsLetter(r) {
			return false, ApplicationNameNotLowercaseError(appName)
		}
	}
	return true, nil
}

//ApplicationNameNotLowercaseError Indicates that application name contains upper case letters
func ApplicationNameNotLowercaseError(appName string) error {
	return fmt.Errorf("Application with name %s contains uppercase letters", appName)
}

// PublicImageComponentCannotHaveSourceOrDockerfileSet Error if image is set and radix config contains src or dockerfile
func PublicImageComponentCannotHaveSourceOrDockerfileSet(componentName string) error {
	return fmt.Errorf("Component %s cannot have neither 'src' nor 'Dockerfile' set", componentName)
}

// ComponentWithDynamicTagRequiresTagInEnvironmentConfig Error if image is set with dynamic tag and tag is missing
func ComponentWithDynamicTagRequiresTagInEnvironmentConfig(componentName string) error {
	return fmt.Errorf("Component %s with %s on image requires an image tag set on environment config",
		componentName, radixv1.DynamicTagNameInEnvironmentConfig)
}

// ComponentWithDynamicTagRequiresTagInEnvironmentConfigForEnvironment Error if image is set with dynamic tag and tag is missing
func ComponentWithDynamicTagRequiresTagInEnvironmentConfigForEnvironment(componentName, environment string) error {
	return fmt.Errorf(
		"Component %s with %s on image requires an image tag set on environment config for environment %s",
		componentName, radixv1.DynamicTagNameInEnvironmentConfig, environment)
}

// ComponentWithTagInEnvironmentConfigForEnvironmentRequiresDynamicTag If tag is set then the dynamic tag needs to be set on the image
func ComponentWithTagInEnvironmentConfigForEnvironmentRequiresDynamicTag(componentName, environment string) error {
	return fmt.Errorf(
		"Component %s with image tag set on environment config for environment %s requires %s on image setting",
		componentName, environment, radixv1.DynamicTagNameInEnvironmentConfig)
}

// SecretNameConflictsWithEnvironmentVariable If secret name is the same as environment variable fail validation
func SecretNameConflictsWithEnvironmentVariable(componentName, secretName string) error {
	return fmt.Errorf(
		"Component %s has a secret with name %s which exists as an environment variable",
		componentName, secretName)
}

// CanRadixApplicationBeInsertedErrors Checks if application config is valid. Returns list of errors, if present
func CanRadixApplicationBeInsertedErrors(client radixclient.Interface, app *radixv1.RadixApplication) (bool, []error) {
	errs := []error{}
	err := validateAppName(app.Name)
	if err != nil {
		errs = append(errs, err)
	}

	componentErrs := validateComponents(app)
	if len(componentErrs) > 0 {
		errs = append(errs, componentErrs...)
	}

	jobErrs := validateJobComponents(app)
	if len(jobErrs) > 0 {
		errs = append(errs, jobErrs...)
	}

	err = validateEnvNames(app)
	if err != nil {
		errs = append(errs, err)
	}

	err = validateSecrets(app)
	if err != nil {
		errs = append(errs, err)
	}

	err = validateVariables(app)
	if err != nil {
		errs = append(errs, err)
	}

	err = validateBranchNames(app)
	if err != nil {
		errs = append(errs, err)
	}

	err = validateDoesRRExist(client, app.Name)
	if err != nil {
		errs = append(errs, err)
	}

	dnsErrors := validateDNSAppAlias(app)
	if len(dnsErrors) > 0 {
		errs = append(errs, dnsErrors...)
	}

	dnsErrors = validateDNSExternalAlias(app)
	if len(dnsErrors) > 0 {
		errs = append(errs, dnsErrors...)
	}

	dnsErrors = validatePrivateImageHubs(app)
	if len(dnsErrors) > 0 {
		errs = append(errs, dnsErrors...)
	}

	err = validateHPAConfigForRA(app)
	if err != nil {
		errs = append(errs, err)
	}

	err = validateVolumeMountConfigForRA(app)
	if err != nil {
		errs = append(errs, err)
	}

	if len(errs) <= 0 {
		return true, nil
	}
	return false, errs
}

func validatePrivateImageHubs(app *radixv1.RadixApplication) []error {
	errs := []error{}
	for server, config := range app.Spec.PrivateImageHubs {
		if config.Username == "" {
			errs = append(errs, MissingPrivateImageHubUsernameError(server))
		}
		if config.Email == "" {
			errs = append(errs, MissingPrivateImageHubEmailError(server))
		}
	}
	return errs
}

// RAContainsOldPublic Checks to see if the radix config is using the deprecated config for public port
func RAContainsOldPublic(app *radixv1.RadixApplication) bool {
	for _, component := range app.Spec.Components {
		if component.Public {
			return true
		}
	}
	return false
}

func validateDNSAppAlias(app *radixv1.RadixApplication) []error {
	errs := []error{}
	alias := app.Spec.DNSAppAlias
	if alias.Component == "" && alias.Environment == "" {
		return errs
	}

	if !doesEnvExist(app, alias.Environment) {
		errs = append(errs, EnvForDNSAppAliasNotDefinedError(alias.Environment))
	}
	if !doesComponentExist(app, alias.Component) {
		errs = append(errs, ComponentForDNSAppAliasNotDefinedError(alias.Component))
	}
	return errs
}

func validateDNSExternalAlias(app *radixv1.RadixApplication) []error {
	errs := []error{}

	distinctAlias := make(map[string]bool)

	for _, externalAlias := range app.Spec.DNSExternalAlias {
		if externalAlias.Alias == "" && externalAlias.Component == "" && externalAlias.Environment == "" {
			return errs
		}

		distinctAlias[externalAlias.Alias] = true

		if externalAlias.Alias == "" {
			errs = append(errs, ExternalAliasCannotBeEmptyError())
		}

		if !doesEnvExist(app, externalAlias.Environment) {
			errs = append(errs, EnvForDNSExternalAliasNotDefinedError(externalAlias.Environment))
		}
		if !doesComponentExist(app, externalAlias.Component) {
			errs = append(errs, ComponentForDNSExternalAliasNotDefinedError(externalAlias.Component))
		}

		if !doesComponentHaveAPublicPort(app, externalAlias.Component) {
			errs = append(errs, ComponentForDNSExternalAliasIsNotMarkedAsPublicError(externalAlias.Component))
		}
	}

	if len(distinctAlias) < len(app.Spec.DNSExternalAlias) {
		errs = append(errs, DuplicateExternalAliasError())
	}

	return errs
}

func validateComponents(app *radixv1.RadixApplication) []error {
	errs := []error{}
	for _, component := range app.Spec.Components {
		if component.Image != "" &&
			(component.SourceFolder != "" || component.DockerfileName != "") {
			errs = append(errs, PublicImageComponentCannotHaveSourceOrDockerfileSet(component.Name))
		}

		if usesDynamicTaggingForDeployOnly(component.Image) {
			if len(component.EnvironmentConfig) == 0 {
				errs = append(errs, ComponentWithDynamicTagRequiresTagInEnvironmentConfig(component.Name))
			} else {
				for _, environment := range component.EnvironmentConfig {
					if doesEnvExistAndIsMappedToBranch(app, environment.Environment) && environment.ImageTagName == "" {
						errs = append(errs,
							ComponentWithDynamicTagRequiresTagInEnvironmentConfigForEnvironment(component.Name, environment.Environment))
					}
				}
			}
		}

		err := validateRequiredResourceName("component name", component.Name)
		if err != nil {
			errs = append(errs, err)
		}

		errList := validatePorts(component.Name, component.Ports)
		if errList != nil && len(errList) > 0 {
			errs = append(errs, errList...)
		}

		errList = validatePublicPort(component)
		if errList != nil && len(errList) > 0 {
			errs = append(errs, errList...)
		}

		// Common resource requirements
		errList = validateResourceRequirements(&component.Resources)
		if errList != nil && len(errList) > 0 {
			errs = append(errs, errList...)
		}

		err = validateAuthentication(component.Authentication)
		if err != nil {
			errs = append(errs, err)
		}

		for _, environment := range component.EnvironmentConfig {
			if !doesEnvExist(app, environment.Environment) {
				err = EnvironmentReferencedByComponentDoesNotExistError(environment.Environment, component.Name)
				errs = append(errs, err)
			}

			err = validateReplica(environment.Replicas)
			if err != nil {
				errs = append(errs, err)
			}

			errList = validateResourceRequirements(&environment.Resources)
			if errList != nil && len(errList) > 0 {
				errs = append(errs, errList...)
			}

			if environmentHasDynamicTaggingButImageLacksTag(environment.ImageTagName, component.Image) {
				errs = append(errs,
					ComponentWithTagInEnvironmentConfigForEnvironmentRequiresDynamicTag(component.Name, environment.Environment))
			}

			err = validateAuthentication(environment.Authentication)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errs
}

func validateJobComponents(app *radixv1.RadixApplication) []error {
	errs := []error{}
	for _, job := range app.Spec.Jobs {
		if job.Image != "" && (job.SourceFolder != "" || job.DockerfileName != "") {
			errs = append(errs, PublicImageComponentCannotHaveSourceOrDockerfileSet(job.Name))
		}

		if usesDynamicTaggingForDeployOnly(job.Image) {
			if len(job.EnvironmentConfig) == 0 {
				errs = append(errs, ComponentWithDynamicTagRequiresTagInEnvironmentConfig(job.Name))
			} else {
				for _, environment := range job.EnvironmentConfig {
					if doesEnvExistAndIsMappedToBranch(app, environment.Environment) && environment.ImageTagName == "" {
						errs = append(errs,
							ComponentWithDynamicTagRequiresTagInEnvironmentConfigForEnvironment(job.Name, environment.Environment))
					}
				}
			}
		}

		err := validateRequiredResourceName("job name", job.Name)
		if err != nil {
			errs = append(errs, err)
		}

		if err = validateJobSchedulerPort(&job); err != nil {
			errs = append(errs, err)
		}

		if err = validateJobPayload(&job); err != nil {
			errs = append(errs, err)
		}

		if job.Ports != nil && len(job.Ports) > 0 {
			errList := validatePorts(job.Name, job.Ports)
			if errList != nil && len(errList) > 0 {
				errs = append(errs, errList...)
			}
		}

		// Common resource requirements
		errList := validateResourceRequirements(&job.Resources)
		if errList != nil && len(errList) > 0 {
			errs = append(errs, errList...)
		}

		for _, environment := range job.EnvironmentConfig {
			if !doesEnvExist(app, environment.Environment) {
				err = EnvironmentReferencedByComponentDoesNotExistError(environment.Environment, job.Name)
				errs = append(errs, err)
			}

			errList = validateResourceRequirements(&environment.Resources)
			if errList != nil && len(errList) > 0 {
				errs = append(errs, errList...)
			}

			if environmentHasDynamicTaggingButImageLacksTag(environment.ImageTagName, job.Image) {
				errs = append(errs,
					ComponentWithTagInEnvironmentConfigForEnvironmentRequiresDynamicTag(job.Name, environment.Environment))
			}
		}
	}

	return errs
}

func validateAuthentication(authentication *v1.Authentication) error {
	if authentication == nil {
		return nil
	}

	return validateClientCertificate(authentication.ClientCertificate)
}

func validateClientCertificate(clientCertificate *v1.ClientCertificate) error {
	if clientCertificate == nil {
		return nil
	}

	return validateVerificationType(clientCertificate.Verification)
}

func validateVerificationType(verificationType *v1.VerificationType) error {
	if verificationType == nil {
		return nil
	}

	validValues := []string{
		string(v1.VerificationTypeOff),
		string(v1.VerificationTypeOn),
		string(v1.VerificationTypeOptional),
		string(v1.VerificationTypeOptionalNoCa),
	}

	actualValue := string(*verificationType)
	if !slice.ContainsString(validValues, actualValue) {
		return InvalidVerificationType(actualValue)
	} else {
		return nil
	}
}

func usesDynamicTaggingForDeployOnly(componentImage string) bool {
	return componentImage != "" &&
		strings.HasSuffix(componentImage, radixv1.DynamicTagNameInEnvironmentConfig)
}

func environmentHasDynamicTaggingButImageLacksTag(environmentImageTag, componentImage string) bool {
	return environmentImageTag != "" &&
		(componentImage == "" ||
			!strings.HasSuffix(componentImage, radixv1.DynamicTagNameInEnvironmentConfig))
}

func validateJobSchedulerPort(job *v1.RadixJobComponent) error {
	if job.SchedulerPort == nil {
		return SchedulerPortCannotBeEmptyForJobError(job.Name)
	}

	return nil
}

func validateJobPayload(job *v1.RadixJobComponent) error {
	if job.Payload != nil && job.Payload.Path == "" {
		return PayloadPathCannotBeEmptyForJobError(job.Name)
	}

	return nil
}

func validatePorts(componentName string, ports []v1.ComponentPort) []error {
	errs := []error{}

	if ports == nil || len(ports) == 0 {
		err := PortSpecificationCannotBeEmptyForComponentError(componentName)
		errs = append(errs, err)
	}

	for _, port := range ports {
		if len(port.Name) > maxPortNameLength {
			err := InvalidPortNameLengthError(port.Name)
			if err != nil {
				errs = append(errs, err)
			}
		}

		err := validateRequiredResourceName("port name", port.Name)
		if err != nil {
			errs = append(errs, err)
		}

		if port.Port < 1024 {
			err := InvalidPortNumberError(port.Port)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errs
}

func validatePublicPort(component radixv1.RadixComponent) []error {
	errs := []error{}

	publicPortName := component.PublicPort
	if publicPortName != "" {
		matchingPortName := 0
		for _, port := range component.Ports {
			if strings.EqualFold(port.Name, publicPortName) {
				matchingPortName++
			}
		}
		if matchingPortName < 1 {
			errs = append(errs, PortNameIsRequiredForPublicComponentError(publicPortName, component.Name))
		}
		if matchingPortName > 1 {
			errs = append(errs, MultipleMatchingPortNamesError(matchingPortName, publicPortName, component.Name))
		}
	}

	return errs
}

func validateResourceRequirements(resourceRequirements *radixv1.ResourceRequirements) []error {
	errs := []error{}
	if resourceRequirements == nil {
		return errs
	}
	limitQuantities := make(map[string]resource.Quantity)
	for name, value := range resourceRequirements.Limits {
		if len(value) > 0 {
			q, err := validateQuantity(name, value)
			if err != nil {
				errs = append(errs, err)
			}
			limitQuantities[name] = q
		}
	}
	for name, value := range resourceRequirements.Requests {
		q, err := validateQuantity(name, value)
		if err != nil {
			errs = append(errs, err)
		}
		if limit, limitExist := limitQuantities[name]; limitExist && q.Cmp(limit) == 1 {
			errs = append(errs, ResourceRequestOverLimitError(name, value, limit.String()))
		}
	}
	return errs
}

func validateQuantity(name, value string) (resource.Quantity, error) {
	var quantity resource.Quantity
	var err error
	if name == "memory" {
		quantity, err = resource.ParseQuantity(value)
		if err != nil {
			return quantity, MemoryResourceRequirementFormatError(value)
		}
	} else if name == "cpu" {
		quantity, err = resource.ParseQuantity(value)
		re := regexp.MustCompile(cpuRegex)
		isValid := re.MatchString(value)
		if err != nil || !isValid {
			return quantity, CPUResourceRequirementFormatError(value)
		}
	} else {
		return quantity, InvalidResourceError(name)
	}

	return quantity, nil
}

func validateSecrets(app *radixv1.RadixApplication) error {
	if app.Spec.Build != nil {
		if err := validateSecretNames("build secret name", app.Spec.Build.Secrets); err != nil {
			return err
		}
	}

	for _, component := range app.Spec.Components {
		if err := validateSecretNames("secret name", component.Secrets); err != nil {
			return err
		}

		envVars := []radixv1.EnvVarsMap{component.Variables}
		for _, env := range component.EnvironmentConfig {
			envVars = append(envVars, env.Variables)
		}

		if err := validateConflictingEnvironmentAndSecretNames(component.Name, component.Secrets, envVars); err != nil {
			return err
		}
	}

	for _, job := range app.Spec.Jobs {
		if err := validateSecretNames("secret name", job.Secrets); err != nil {
			return err
		}

		envVars := []radixv1.EnvVarsMap{job.Variables}
		for _, env := range job.EnvironmentConfig {
			envVars = append(envVars, env.Variables)
		}

		if err := validateConflictingEnvironmentAndSecretNames(job.Name, job.Secrets, envVars); err != nil {
			return err
		}
	}

	return nil
}

func validateSecretNames(resourceName string, secrets []string) error {
	for _, secret := range secrets {
		if err := validateVariableName(resourceName, secret); err != nil {
			return err
		}
	}
	return nil
}

func validateVariables(app *radixv1.RadixApplication) error {
	for _, component := range app.Spec.Components {
		if err := validateVariableNames("environment variable name", component.Variables); err != nil {
			return err
		}

		for _, envConfig := range component.EnvironmentConfig {
			if err := validateVariableNames("environment variable name", envConfig.Variables); err != nil {
				return err
			}
		}
	}

	for _, job := range app.Spec.Jobs {
		if err := validateVariableNames("environment variable name", job.Variables); err != nil {
			return err
		}

		for _, envConfig := range job.EnvironmentConfig {
			if err := validateVariableNames("environment variable name", envConfig.Variables); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateVariableNames(resourceName string, variables radixv1.EnvVarsMap) error {
	for v := range variables {
		if err := validateVariableName(resourceName, v); err != nil {
			return err
		}
	}
	return nil
}

func validateConflictingEnvironmentAndSecretNames(componentName string, secrets []string, variables []radixv1.EnvVarsMap) error {
	secretNames := make(map[string]struct{})
	for _, secret := range secrets {
		secretNames[secret] = struct{}{}
	}

	for _, envVars := range variables {
		for envVarKey := range envVars {
			if _, contains := secretNames[envVarKey]; contains {
				return SecretNameConflictsWithEnvironmentVariable(componentName, envVarKey)
			}
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
			return InvalidStringValueMaxLengthError("branch from", env.Build.From, 253)
		}

		isValid := branch.IsValidPattern(env.Build.From)
		if !isValid {
			return InvalidBranchNameError(env.Build.From)
		}
	}
	return nil
}

func validateEnvNames(app *radixv1.RadixApplication) error {
	for _, env := range app.Spec.Environments {
		err := validateRequiredResourceName("env name", env.Name)
		if err != nil {
			return err
		}
		err = validateMaxNameLengthForAppAndEnv(app.Name, env.Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func validateMaxNameLengthForAppAndEnv(appName, envName string) error {
	if len(appName)+len(envName) > 62 {
		return fmt.Errorf("summary length of app name and environment together should not exceed 62 characters")
	}
	return nil
}

func validateVariableName(resourceName, value string) error {
	return validateResourceWithRegexp(resourceName, value, "^(([A-Za-z0-9][-._A-Za-z0-9.]*)?[A-Za-z0-9])?$")
}

func validateResourceWithRegexp(resourceName, value, regexpExpression string) error {
	if len(value) > 253 {
		return InvalidStringValueMaxLengthError(resourceName, value, 253)
	}

	re := regexp.MustCompile(regexpExpression)

	isValid := re.MatchString(value)
	if isValid {
		return nil
	}
	return InvalidResourceNameError(resourceName, value)
}

func validateHPAConfigForRA(app *radixv1.RadixApplication) error {
	for _, component := range app.Spec.Components {
		for _, envConfig := range component.EnvironmentConfig {
			componentName := component.Name
			environment := envConfig.Environment
			if envConfig.HorizontalScaling == nil {
				continue
			}
			maxReplicas := envConfig.HorizontalScaling.MaxReplicas
			minReplicas := envConfig.HorizontalScaling.MinReplicas
			if maxReplicas == 0 {
				return MaxReplicasForHPANotSetOrZeroError(componentName, environment)
			}
			if minReplicas != nil && *minReplicas > maxReplicas {
				return MinReplicasGreaterThanMaxReplicasError(componentName, environment)
			}
		}
	}

	return nil
}

func validateVolumeMountConfigForRA(app *radixv1.RadixApplication) error {
	for _, component := range app.Spec.Components {
		for _, envConfig := range component.EnvironmentConfig {
			if err := validateVolumeMounts(component.Name, envConfig.Environment, envConfig.VolumeMounts); err != nil {
				return err
			}
		}
	}

	for _, job := range app.Spec.Jobs {
		for _, envConfig := range job.EnvironmentConfig {
			if err := validateVolumeMounts(job.Name, envConfig.Environment, envConfig.VolumeMounts); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateVolumeMounts(componentName, environment string, volumeMounts []radixv1.RadixVolumeMount) error {
	if volumeMounts == nil || len(volumeMounts) == 0 {
		return nil
	}

	mountsInComponent := make(map[string]volumeMountConfigMaps)

	for _, volumeMount := range volumeMounts {
		volumeMountType := strings.TrimSpace(string(volumeMount.Type))
		volumeMountStorage := deployment.GetRadixVolumeMountStorage(&volumeMount)
		switch {
		case volumeMountType == "" ||
			strings.TrimSpace(volumeMount.Name) == "" ||
			strings.TrimSpace(volumeMountStorage) == "" ||
			strings.TrimSpace(volumeMount.Path) == "":
			{
				return emptyVolumeMountTypeContainerNameOrTempPathError(componentName, environment)
			}
		case v1.IsKnownVolumeMount(volumeMountType):
			{
				if _, exists := mountsInComponent[volumeMountType]; !exists {
					mountsInComponent[volumeMountType] = volumeMountConfigMaps{names: make(map[string]bool), path: make(map[string]bool)}
				}
				volumeMountConfigMap := mountsInComponent[volumeMountType]
				if _, exists := volumeMountConfigMap.names[volumeMount.Name]; exists {
					return duplicateNameForVolumeMountType(volumeMount.Name, volumeMountType, componentName, environment)
				}
				volumeMountConfigMap.names[volumeMount.Name] = true
				if _, exists := volumeMountConfigMap.path[volumeMount.Path]; exists {
					return duplicatePathForVolumeMountType(volumeMount.Path, volumeMountType, componentName, environment)
				}
				volumeMountConfigMap.path[volumeMount.Path] = true
			}
		default:
			return unknownVolumeMountTypeError(volumeMountType, componentName, environment)
		}
	}

	return nil
}

type volumeMountConfigMaps struct {
	names map[string]bool
	path  map[string]bool
}

func doesComponentExist(app *radixv1.RadixApplication, name string) bool {
	for _, component := range app.Spec.Components {
		if component.Name == name {
			return true
		}
	}
	return false
}

func doesEnvExist(app *radixv1.RadixApplication, name string) bool {
	env := getEnv(app, name)
	if env != nil {
		return true
	}

	return false
}

func doesEnvExistAndIsMappedToBranch(app *radixv1.RadixApplication, name string) bool {
	env := getEnv(app, name)
	if env != nil && env.Build.From != "" {
		return true
	}

	return false
}

func getEnv(app *radixv1.RadixApplication, name string) *radixv1.Environment {
	for _, env := range app.Spec.Environments {
		if env.Name == name {
			return &env
		}
	}
	return nil
}

func doesComponentHaveAPublicPort(app *radixv1.RadixApplication, name string) bool {
	for _, component := range app.Spec.Components {
		if component.Name == name {
			if component.Public || component.PublicPort != "" {
				return true
			}

			return false
		}
	}
	return false
}

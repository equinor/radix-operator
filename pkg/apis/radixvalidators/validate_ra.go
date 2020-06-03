package radixvalidators

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/branch"
	errorUtils "github.com/equinor/radix-operator/pkg/apis/utils/errors"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	maxPortNameLength = 15
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

// MemoryResourceRequirementFormatError Invalid memory resource requirement error
func MemoryResourceRequirementFormatError(value string) error {
	return fmt.Errorf("Format of memory resource requirement %s (value %s) is wrong. Value must be a valid Kubernetes quantity", "memory", value)
}

// CPUResourceRequirementFormatError Invalid CPU resource requirement
func CPUResourceRequirementFormatError(value string) error {
	return fmt.Errorf("Format of cpu resource requirement %s (value %s) is wrong. Must match regex '%s'", "cpu", value, cpuRegex)
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

// CanRadixApplicationBeInserted Checks if application config is valid. Returns a single error, if this is the case
func CanRadixApplicationBeInserted(client radixclient.Interface, app *radixv1.RadixApplication) (bool, error) {
	isValid, errs := CanRadixApplicationBeInsertedErrors(client, app)
	if isValid {
		return true, nil
	}

	return false, errorUtils.Concat(errs)
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

// SecretNameConfictsWithEnvironmentVariable If secret name is the same as environment variable fail validation
func SecretNameConfictsWithEnvironmentVariable(componentName, secretName string) error {
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

	err = validateEnvNames(app)
	if err != nil {
		errs = append(errs, err)
	}

	err = validateSecretNames(app)
	if err != nil {
		errs = append(errs, err)
	}

	err = validateEnvironmentVariableNames(app)
	if err != nil {
		errs = append(errs, err)
	}

	err = validateConflictingEnvironmentAndSecretNames(app)
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
					if doesEnvExistAndIsMappedToBranch(app, environment.Environment) &&
						environment.ImageTagName == "" {
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

		errList := validatePorts(component)
		if errList != nil && len(errList) > 0 {
			errs = append(errs, errList...)
		}

		// Common resource requirements
		errList = validateResourceRequirements(&component.Resources)
		if errList != nil && len(errList) > 0 {
			errs = append(errs, errList...)
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
		}
	}

	return errs
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

func validatePorts(component radixv1.RadixComponent) []error {
	errs := []error{}

	if component.Ports == nil || len(component.Ports) == 0 {
		err := PortSpecificationCannotBeEmptyForComponentError(component.Name)
		errs = append(errs, err)
	}

	for _, port := range component.Ports {
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
	}

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
		q, err := validateQuantity(name, value)
		if err != nil {
			errs = append(errs, err)
		}
		limitQuantities[name] = q
	}
	for name, value := range resourceRequirements.Requests {
		limit := limitQuantities[name]
		q, err := validateQuantity(name, value)
		if err != nil {
			errs = append(errs, err)
		}
		if q.Cmp(limit) == 1 {
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

func validateSecretNames(app *radixv1.RadixApplication) error {
	if app.Spec.Build != nil {
		for _, buildSecret := range app.Spec.Build.Secrets {
			err := validateVariableName("build secret name", buildSecret)
			if err != nil {
				return err
			}
		}
	}

	for _, component := range app.Spec.Components {
		for _, secret := range component.Secrets {
			err := validateVariableName("secret name", secret)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func validateEnvironmentVariableNames(app *radixv1.RadixApplication) error {
	for _, component := range app.Spec.Components {
		// Common environment variables
		for commonEnvironmentVariable := range component.Variables {
			err := validateVariableName("environment variable name", commonEnvironmentVariable)
			if err != nil {
				return err
			}
		}
		// Per-environment environment variables
		for _, envConfig := range component.EnvironmentConfig {
			for environmentVariable := range envConfig.Variables {
				err := validateVariableName("environment variable name", environmentVariable)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func validateConflictingEnvironmentAndSecretNames(app *radixv1.RadixApplication) error {
	for _, component := range app.Spec.Components {
		secretNames := make(map[string]struct{})
		for _, secret := range component.Secrets {
			secretNames[secret] = struct{}{}
		}

		// Common environment variables
		for commonEnvironmentVariable := range component.Variables {
			if _, contains := secretNames[commonEnvironmentVariable]; contains {
				return SecretNameConfictsWithEnvironmentVariable(component.Name, commonEnvironmentVariable)
			}
		}

		// Per-environment environment variables
		for _, envConfig := range component.EnvironmentConfig {
			for environmentVariable := range envConfig.Variables {
				if _, contains := secretNames[environmentVariable]; contains {
					return SecretNameConfictsWithEnvironmentVariable(component.Name, environmentVariable)
				}
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
			return InvalidResourceNameLengthError("branch from", env.Build.From)
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
	}
	return nil
}

func validateLabelName(resourceName, value string) error {
	return validateResourceWithRegexp(resourceName, value, "^(([A-Za-z0-9][-A-Za-z0-9.]*)?[A-Za-z0-9])?$")
}

func validateVariableName(resourceName, value string) error {
	return validateResourceWithRegexp(resourceName, value, "^(([A-Za-z0-9][-._A-Za-z0-9.]*)?[A-Za-z0-9])?$")
}

func validateResourceWithRegexp(resourceName, value, regexpExpression string) error {
	if len(value) > 253 {
		return InvalidResourceNameLengthError(resourceName, value)
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

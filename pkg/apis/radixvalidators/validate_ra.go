package radixvalidators

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/equinor/radix-operator/pkg/apis/deployment"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	// v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/branch"
	errorUtils "github.com/equinor/radix-operator/pkg/apis/utils/errors"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	maxPortNameLength = 15
	cpuRegex          = "^[0-9]+m$"
)

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

	if len(errs) == 0 {
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
		if len(errList) > 0 {
			errs = append(errs, errList...)
		}

		errList = validatePublicPort(component)
		if len(errList) > 0 {
			errs = append(errs, errList...)
		}

		// Common resource requirements
		errList = validateResourceRequirements(&component.Resources)
		if len(errList) > 0 {
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
			if len(errList) > 0 {
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
			if len(errList) > 0 {
				errs = append(errs, errList...)
			}
		}

		// Common resource requirements
		errList := validateResourceRequirements(&job.Resources)
		if len(errList) > 0 {
			errs = append(errs, errList...)
		}

		for _, environment := range job.EnvironmentConfig {
			if !doesEnvExist(app, environment.Environment) {
				err = EnvironmentReferencedByComponentDoesNotExistError(environment.Environment, job.Name)
				errs = append(errs, err)
			}

			errList = validateResourceRequirements(&environment.Resources)
			if len(errList) > 0 {
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

func validateAuthentication(authentication *radixv1.Authentication) error {
	if authentication == nil {
		return nil
	}

	return validateClientCertificate(authentication.ClientCertificate)
}

func validateClientCertificate(clientCertificate *radixv1.ClientCertificate) error {
	if clientCertificate == nil {
		return nil
	}

	return validateVerificationType(clientCertificate.Verification)
}

func validateVerificationType(verificationType *radixv1.VerificationType) error {
	if verificationType == nil {
		return nil
	}

	validValues := []string{
		string(radixv1.VerificationTypeOff),
		string(radixv1.VerificationTypeOn),
		string(radixv1.VerificationTypeOptional),
		string(radixv1.VerificationTypeOptionalNoCa),
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

func validateJobSchedulerPort(job *radixv1.RadixJobComponent) error {
	if job.SchedulerPort == nil {
		return SchedulerPortCannotBeEmptyForJobError(job.Name)
	}

	return nil
}

func validateJobPayload(job *radixv1.RadixJobComponent) error {
	if job.Payload != nil && job.Payload.Path == "" {
		return PayloadPathCannotBeEmptyForJobError(job.Name)
	}

	return nil
}

func validatePorts(componentName string, ports []radixv1.ComponentPort) []error {
	errs := []error{}

	if len(ports) == 0 {
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
	if len(volumeMounts) == 0 {
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
		case radixv1.IsKnownVolumeMount(volumeMountType):
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
	return getEnv(app, name) != nil
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

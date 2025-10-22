package radixapplication

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/webhook/validation/genericvalidator"
	"github.com/rs/zerolog/log"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	minReplica = 0 // default is 1

	// MaxReplica Max number of replicas a deployment is allowed to have
	MaxReplica = 64
)

var (
	validOAuthSessionStoreTypes = []string{string(radixv1.SessionStoreCookie), string(radixv1.SessionStoreRedis), string(radixv1.SessionStoreSystemManaged)}
	validOAuthCookieSameSites   = []string{string(radixv1.SameSiteStrict), string(radixv1.SameSiteLax), string(radixv1.SameSiteNone), string(radixv1.SameSiteEmpty)}
	propertyNameRegex           = regexp.MustCompile(`^(([A-Za-z0-9][-._A-Za-z0-9.]*)?[A-Za-z0-9])?$`)
)

// validatorFunc defines a validatorFunc function for a RadixApplication
type validatorFunc func(ctx context.Context, radixApplication *radixv1.RadixApplication) (string, error)

type Validator struct {
	validators []validatorFunc
}

var _ genericvalidator.Validator[*radixv1.RadixApplication] = &Validator{}

func CreateOnlineValidator(client client.Client, dnsConfig *dnsalias.DNSConfig) *Validator {
	return &Validator{
		validators: []validatorFunc{
			createDeprecatedPublicUsageValidator(),
			createComponentValidator(),
			createJobValidator(),
			createExternalDNSAliasValidator(),
			createDNSAliasValidator(),
			createRRExistValidator(client),
			createDNSAliasAvailableValidator(client, dnsConfig),
			createSecretValidator(),
			createEnvNameValidator(),
		},
	}
}

func CreateOfflineValidator() Validator {
	return Validator{
		validators: []validatorFunc{
			createDeprecatedPublicUsageValidator(),
			createComponentValidator(),
			createJobValidator(),
			createExternalDNSAliasValidator(),
			createDNSAliasValidator(),
			createSecretValidator(),
			createEnvNameValidator(),
		},
	}
}

func (validator *Validator) Validate(ctx context.Context, ra *radixv1.RadixApplication) (admission.Warnings, error) {
	var errs []error
	var wrns admission.Warnings
	for _, v := range validator.validators {
		wrn, err := v(ctx, ra)
		if err != nil {
			errs = append(errs, err)
		}
		if wrn != "" {
			wrns = append(wrns, wrn)
		}
	}

	return wrns, errors.Join(errs...)
}

func createRRExistValidator(kubeClient client.Client) validatorFunc {
	return func(ctx context.Context, ra *radixv1.RadixApplication) (string, error) {
		err := kubeClient.Get(ctx, client.ObjectKey{Name: ra.Name}, &radixv1.RadixRegistration{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return "", ErrNoRadixApplication
			}

			log.Ctx(ctx).Error().Err(err).Msg("failed to list existing RadixRegistrations")
			return "", err // Something went wrong while listing existing RadixRegistrations, let the user try again
		}

		return "", nil
	}
}

func createDeprecatedPublicUsageValidator() validatorFunc {
	return func(_ context.Context, ra *radixv1.RadixApplication) (string, error) {
		for _, component := range ra.Spec.Components {
			//nolint:staticcheck
			if component.Public {
				return fmt.Sprintf("component %s is using deprecated public field. use publicPort and ports.name instead", component.Name), nil
			}
		}
		return "", nil
	}
}

func createJobValidator() validatorFunc {
	return func(ctx context.Context, app *radixv1.RadixApplication) (string, error) {
		var wrns []string
		var errs []error
		for _, job := range app.Spec.Jobs {

			if job.Image != "" && (job.SourceFolder != "" || job.DockerfileName != "") {
				errs = append(errs, fmt.Errorf("job %s: %w", job.Name, ErrPublicImageComponentCannotHaveSourceOrDockerfileSetWithImage))
			}

			// Common resource requirements
			if err := validateResourceRequirements(job.Resources); err != nil {
				errs = append(errs, fmt.Errorf("job %s: %w", job.Name, err))
			}

			if err := validateMonitoring(&job); err != nil {
				errs = append(errs, fmt.Errorf("job %s: %w", job.Name, err))
			}

			if err := validateRuntime(job.Runtime); err != nil {
				errs = append(errs, fmt.Errorf("job %s: %w", job.Name, err))
			}

			if err := validateFailurePolicy(job.FailurePolicy); err != nil {
				errs = append(errs, fmt.Errorf("job %s: %w", job.Name, err))
			}

			for _, environment := range job.EnvironmentConfig {
				if err := validateJobComponentEnvironment(app, job, environment); err != nil {
					errs = append(errs, fmt.Errorf("invalid configuration for environment %s: %w", environment.Environment, err))
				}
			}

		}
		return strings.Join(wrns, "\n"), errors.Join(errs...)
	}
}

func validateJobComponentEnvironment(app *radixv1.RadixApplication, job radixv1.RadixJobComponent, environment radixv1.RadixJobComponentEnvironmentConfig) error {
	var errs []error

	if !doesEnvExist(app, environment.Environment) {
		errs = append(errs, fmt.Errorf("job %s in environment %s: %w", job.Name, environment.Environment, ErrEnvironmentReferencedByComponentDoesNotExist))
	}

	if err := validateResourceRequirements(environment.Resources); err != nil {
		errs = append(errs, fmt.Errorf("job %s in environment %s: %w", job.Name, environment.Environment, err))
	}

	if environmentHasDynamicTaggingButImageLacksTag(environment.ImageTagName, job.Image) {
		errs = append(errs, fmt.Errorf("job %s in environment %s: %w", job.Name, environment.Environment, ErrComponentWithDynamicTagRequiresImageTag))
	}

	if err := validateRuntime(environment.Runtime); err != nil {
		errs = append(errs, fmt.Errorf("job %s in environment %s: %w", job.Name, environment.Environment, err))
	}

	if err := validateFailurePolicy(environment.FailurePolicy); err != nil {
		errs = append(errs, fmt.Errorf("job %s in environment %s: %w", job.Name, environment.Environment, err))
	}

	return errors.Join(errs...)
}

func createComponentValidator() validatorFunc {
	return func(ctx context.Context, app *radixv1.RadixApplication) (string, error) {
		var wrns []string
		var errs []error
		for _, component := range app.Spec.Components {

			if component.Image != "" && (component.SourceFolder != "" || component.DockerfileName != "") {
				wrns = append(wrns, fmt.Sprintf("component %s: component image will take precedens. src and dockerfile will be ignored.", component.Name))
			}

			if err := validatePublicPort(component); err != nil {
				errs = append(errs, err)
			}

			// Common resource requirements
			if err := validateResourceRequirements(component.GetResources()); err != nil {
				errs = append(errs, fmt.Errorf("component %s: %w", component.Name, err))
			}

			if err := validateMonitoring(&component); err != nil {
				errs = append(errs, err)
			}

			errs = append(errs, validateAuthentication(&component, app.Spec.Environments)...)

			if err := validateRuntime(component.GetRuntime()); err != nil {
				errs = append(errs, err)
			}

			if err := validateHealthChecks(component.HealthChecks); err != nil {
				errs = append(errs, fmt.Errorf("component %s: %w", component.Name, err))
			}

			for _, environment := range component.EnvironmentConfig {
				if err := validateComponentEnvironment(app, component, environment); err != nil {
					errs = append(errs, fmt.Errorf("invalid configuration for environment %s: %w", environment.Environment, err))
				}
			}
		}

		return strings.Join(wrns, ", "), errors.Join(errs...)
	}
}

func validateComponentEnvironment(app *radixv1.RadixApplication, component radixv1.RadixComponent, environment radixv1.RadixEnvironmentConfig) error {
	var errs []error

	if !doesEnvExist(app, environment.Environment) {
		errs = append(errs, fmt.Errorf("environment %s referenced by component %s: %w", environment.Environment, component.Name, ErrEnvironmentReferencedByComponentDoesNotExist))
	}

	if err := validateReplica(component.Replicas); err != nil {
		errs = append(errs, fmt.Errorf("component %s replicas is invalid: %w", component.Name, err))
	}

	if err := validateReplica(environment.Replicas); err != nil {
		errs = append(errs, fmt.Errorf("component %s environment %s replicas is invalid: %w", component.Name, environment.Environment, err))
	}

	if err := validateResourceRequirements(environment.Resources); err != nil {
		errs = append(errs, fmt.Errorf("component %s in environment %s: %w", component.Name, environment.Environment, err))
	}

	if environmentHasDynamicTaggingButImageLacksTag(environment.ImageTagName, component.Image) {
		errs = append(errs, fmt.Errorf("component %s in environment %s: %w", component.Name, environment.Environment, ErrComponentWithDynamicTagRequiresImageTag))
	}

	if err := validateRuntime(environment.Runtime); err != nil {
		errs = append(errs, fmt.Errorf("component %s in environment %s: %w", component.Name, environment.Environment, err))
	}

	if err := validateHealthChecks(environment.HealthChecks); err != nil {
		errs = append(errs, fmt.Errorf("component %s in environment %s: %w", component.Name, environment.Environment, err))
	}

	return errors.Join(errs...)
}

func createEnvNameValidator() validatorFunc {
	return func(ctx context.Context, ra *radixv1.RadixApplication) (string, error) {
		for _, env := range ra.Spec.Environments {
			if len(ra.Name)+len(env.Name) > 62 {
				return "", fmt.Errorf("environment %s: %w", env.Name, ErrInvalidEnvironmentNameLength)
			}
		}
		return "", nil
	}
}

func environmentHasDynamicTaggingButImageLacksTag(environmentImageTag, componentImage string) bool {
	return environmentImageTag != "" &&
		(componentImage == "" ||
			!strings.HasSuffix(componentImage, radixv1.DynamicTagNameInEnvironmentConfig))
}

func doesEnvExist(app *radixv1.RadixApplication, name string) bool {
	return slice.Any(app.Spec.Environments, func(e radixv1.Environment) bool { return e.Name == name })
}

func doesComponentExistAndEnabled(app *radixv1.RadixApplication, componentName string, environment string) bool {
	for _, component := range app.Spec.Components {
		if component.Name == componentName {
			environmentConfig := component.GetEnvironmentConfigByName(environment)
			return component.GetEnabledForEnvironmentConfig(environmentConfig)
		}
	}
	return false
}

func doesComponentHaveAPublicPort(app *radixv1.RadixApplication, name string) bool {
	for _, component := range app.Spec.Components {
		if component.Name == name {
			//nolint:staticcheck
			return component.Public || component.PublicPort != ""
		}
	}
	return false
}

func validateResourceRequirements(resources radixv1.ResourceRequirements) error {
	var errs []error
	limitQuantities := make(map[string]resource.Quantity)
	for name, value := range resources.Limits {
		if len(value) > 0 {
			q, err := resource.ParseQuantity(value)
			if err != nil {
				errs = append(errs, fmt.Errorf("invalid limit resource %s quantity %s: %w", name, value, err))
			} else {
				limitQuantities[name] = q
			}
		}
	}
	for name, value := range resources.Requests {
		q, err := resource.ParseQuantity(value)
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid requested resource %s quantity %s: %w", name, value, err))
		}
		if limit, limitExist := limitQuantities[name]; limitExist && q.Cmp(limit) == 1 {
			errs = append(errs, fmt.Errorf("resource %s (req: %s, limit: %s): %w", name, q.String(), limit.String(), ErrRequestedResourceExceedsLimit))
		}
	}

	return errors.Join(errs...)
}

func validatePublicPort(component radixv1.RadixComponent) error {
	if component.PublicPort == "" {
		return nil
	}

	for _, port := range component.Ports {
		if port.Name == component.PublicPort {
			return nil // we found a match
		}
	}

	return fmt.Errorf("component %s: %w", component.Name, ErrPublicPortNotFound)
}

func validateMonitoring(component radixv1.RadixCommonComponent) error {
	monitoring := component.GetMonitoring()
	if monitoring == nil || !*monitoring {
		return nil // monitoring disabled
	}

	if len(component.GetPorts()) == 0 {
		return fmt.Errorf("component %s: %w", component.GetName(), ErrMonitoringNoPortsDefined)
	}

	monitoringConfig := component.GetMonitoringConfig()
	if monitoringConfig.PortName == "" {
		return nil // first port will be used
	}

	for _, p := range component.GetPorts() {
		if monitoringConfig.PortName == p.Name {
			return nil // we found a match
		}
	}

	return fmt.Errorf("component %s: %w", component.GetName(), ErrMonitoringNamedPortNotFound)
}

func validateAuthentication(component *radixv1.RadixComponent, environments []radixv1.Environment) []error {
	componentAuth := component.Authentication
	envAuthConfigGetter := func(name string) *radixv1.Authentication {
		for _, envConfig := range component.EnvironmentConfig {
			if envConfig.Environment == name {
				return envConfig.Authentication
			}
		}
		return nil
	}

	var errs []error
	for _, environment := range environments {
		environmentAuth := envAuthConfigGetter(environment.Name)
		if componentAuth == nil && environmentAuth == nil {
			continue
		}
		combinedAuth, err := deployment.GetAuthenticationForComponent(componentAuth, environmentAuth)
		if err != nil {
			errs = append(errs, err)
		}
		if combinedAuth == nil {
			continue
		}

		if err := validateClientCertificate(combinedAuth.ClientCertificate); err != nil {
			errs = append(errs, err)
		}

		errs = append(errs, validateOAuth(combinedAuth.OAuth2, component, environment.Name)...)
	}
	return errs
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
	if !commonUtils.ContainsString(validValues, actualValue) {
		return fmt.Errorf("verification type '%s': %w", actualValue, ErrInvalidVerificationType)
	}
	return nil
}

func componentHasPublicPort(component *radixv1.RadixComponent) bool {
	//nolint:staticcheck
	if component.Public && len(component.Ports) >= 1 {
		return true // first port is public
	}

	if component.PublicPort == "" {
		return false
	}

	for _, p := range component.GetPorts() {
		if p.Name == component.PublicPort {
			return true
		}
	}

	return false
}

func validateRuntime(runtime *radixv1.Runtime) error {
	if runtime == nil {
		return nil
	}

	if runtime.Architecture != "" && runtime.NodeType != nil {
		return ErrRuntimeArchitectureWithNodeType
	}
	return nil
}

func validateReplica(replica *int) error {
	if replica == nil {
		return nil
	}
	replicaValue := *replica
	if replicaValue > MaxReplica || replicaValue < minReplica {
		return ErrInvalidNumberOfReplicas
	}
	return nil
}

func validateHealthChecks(healthChecks *radixv1.RadixHealthChecks) error {
	if healthChecks == nil {
		return nil
	}

	var errs []error

	if err := validateProbe(healthChecks.StartupProbe); err != nil {
		errs = append(errs, fmt.Errorf("probe StartupProbe is invalid: %w", err))
	}
	if err := validateProbe(healthChecks.ReadinessProbe); err != nil {
		errs = append(errs, fmt.Errorf("probe ReadinessProbe is invalid: %w", err))
	}
	if err := validateProbe(healthChecks.LivenessProbe); err != nil {
		errs = append(errs, fmt.Errorf("probe LivenessProbe is invalid: %w", err))
	}

	// SuccessTreshold must be 0 (unset) or 1 for Startup Probe
	if healthChecks.StartupProbe != nil && healthChecks.StartupProbe.SuccessThreshold > 1 {
		errs = append(errs, fmt.Errorf("probe StartupProbe is invalid: %w", ErrSuccessThresholdMustBeOne))
	}

	// SuccessTreshold must be 0 (unset) or 1 for Liveness Probe
	if healthChecks.LivenessProbe != nil && healthChecks.LivenessProbe.SuccessThreshold > 1 {
		errs = append(errs, fmt.Errorf("probe LivenessProbe is invalid: %w", ErrSuccessThresholdMustBeOne))
	}

	return errors.Join(errs...)
}

func validateProbe(probe *radixv1.RadixProbe) error {
	if probe == nil {
		return nil
	}

	definedProbes := 0
	if probe.HTTPGet != nil {
		definedProbes++
	}

	if probe.TCPSocket != nil {
		definedProbes++
	}

	if probe.Exec != nil {
		definedProbes++
	}

	if probe.GRPC != nil {
		definedProbes++
	}

	if definedProbes > 1 {
		return ErrInvalidHealthCheckProbe
	}

	return nil
}

func validateFailurePolicy(failurePolicy *radixv1.RadixJobComponentFailurePolicy) error {
	if failurePolicy == nil || len(failurePolicy.Rules) == 0 {
		return nil
	}

	var errs []error
	for _, rule := range failurePolicy.Rules {
		if err := validateFailurePolicyRuleOnExitCodes(rule.OnExitCodes); err != nil {
			errs = append(errs, fmt.Errorf("invalid failure policy onExitCodes configuration: %w", err))
		}
	}

	return errors.Join(errs...)
}

func validateFailurePolicyRuleOnExitCodes(onExitCodes radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes) error {
	if onExitCodes.Operator == radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOpIn &&
		slices.Contains(onExitCodes.Values, 0) {
		return ErrFailurePolicyRuleExitCodeZeroNotAllowedForInOperator
	}

	return nil
}

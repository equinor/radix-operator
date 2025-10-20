package radixapplication

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	oauthutil "github.com/equinor/radix-operator/pkg/apis/utils/oauth"
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
			checkDeprecatedPublicUsage,
			createComponentValidator(),
			createExternalDNSAliasValidator(),
			createDNSAliasValidator(),
			createRRExistValidator(client),
			createDNSAliasAvailableValidator(client, dnsConfig),
		},
	}
}

func CreateOfflineValidator() Validator {
	return Validator{
		validators: []validatorFunc{
			checkDeprecatedPublicUsage,
			createComponentValidator(),
			createExternalDNSAliasValidator(),
			createDNSAliasValidator(),
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

func createDNSAliasAvailableValidator(kubeClient client.Client, dnsAliasConfig *dnsalias.DNSConfig) validatorFunc {
	return func(ctx context.Context, ra *radixv1.RadixApplication) (string, error) {
		var errs []error
		list := radixv1.RadixDNSAliasList{}
		err := kubeClient.List(ctx, &list)
		if err != nil {
			return "", err
		}

		for _, dnsAlias := range ra.Spec.DNSAlias {

			existingAliasForDifferentApp := slice.Any(list.Items, func(item radixv1.RadixDNSAlias) bool {
				return item.Spec.AppName != ra.Name && item.Name == dnsAlias.Alias
			})
			if existingAliasForDifferentApp {
				errs = append(errs, fmt.Errorf("dns alias %s is already used. %w", dnsAlias.Alias, ErrDNSAliasAlreadyUsedByAnotherApplication))
			}

			if reservingAppName, aliasReserved := dnsAliasConfig.ReservedAppDNSAliases[dnsAlias.Alias]; aliasReserved && reservingAppName != ra.Name {
				errs = append(errs, fmt.Errorf("dns alias %s is reserved. %w", dnsAlias.Alias, ErrDNSAliasReservedForRadixPlatformApplication))
			}

			if slice.Any(dnsAliasConfig.ReservedDNSAliases, func(reservedAlias string) bool { return reservedAlias == dnsAlias.Alias }) {
				errs = append(errs, fmt.Errorf("dns alias %s is reserved. %w", dnsAlias.Alias, ErrDNSAliasReservedForRadixPlatformService))
			}
		}
		return "", errors.Join(errs...)
	}
}

func createDNSAliasValidator() validatorFunc {
	return func(ctx context.Context, ra *radixv1.RadixApplication) (string, error) {
		var errs []error

		for _, dnsAlias := range ra.Spec.DNSAlias {
			if err := validateDNSAliasComponentAndEnvironmentAvailable(ra, dnsAlias.Alias, dnsAlias.Component, dnsAlias.Environment); err != nil {
				errs = append(errs, err)
				continue
			}
			if !doesComponentHaveAPublicPort(ra, dnsAlias.Component) {
				errs = append(errs, fmt.Errorf("component %s is not public. %w", dnsAlias.Component, ErrDNSAliasComponentIsNotMarkedAsPublic))
				continue
			}
		}
		return "", errors.Join(errs...)
	}
}

func createExternalDNSAliasValidator() validatorFunc {
	return func(ctx context.Context, ra *radixv1.RadixApplication) (string, error) {
		var errs []error

		for _, externalAlias := range ra.Spec.DNSExternalAlias {
			if !doesEnvExist(ra, externalAlias.Environment) {
				errs = append(errs, fmt.Errorf("%s: %w", externalAlias.Alias, ErrExternalAliasEnvironmentNotDefined))
			}
			if !doesComponentExistAndEnabled(ra, externalAlias.Component, externalAlias.Environment) {
				errs = append(errs, fmt.Errorf("%s: %w", externalAlias.Alias, ErrExternalAliasComponentNotDefined))
			}

			if !doesComponentHaveAPublicPort(ra, externalAlias.Component) {
				errs = append(errs, fmt.Errorf("%s: %w", externalAlias.Alias, ErrExternalAliasComponentNotMarkedAsPublic))
			}
		}
		return "", errors.Join(errs...)
	}
}

func validateDNSAliasComponentAndEnvironmentAvailable(ra *radixv1.RadixApplication, dnsAlias, component, environment string) error {
	if !doesEnvExist(ra, environment) {
		return fmt.Errorf("%s: %w", dnsAlias, ErrDNSAliasEnvironmentNotDefined)
	}
	if !doesComponentExistAndEnabled(ra, component, environment) {
		return fmt.Errorf("%s: %w", dnsAlias, ErrDNSAliasComponentNotDefinedOrDisabled)
	}
	return nil
}

func checkDeprecatedPublicUsage(_ context.Context, ra *radixv1.RadixApplication) (string, error) {
	for _, component := range ra.Spec.Components {
		//nolint:staticcheck
		if component.Public {
			return fmt.Sprintf("component %s is using deprecated public field. use publicPort and ports.name instead", component.Name), nil
		}
	}
	return "", nil
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

		// TODO: Can wrns contain newline?
		return strings.Join(wrns, "\n"), errors.Join(errs...)
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

func validateSkipAuthRoutes(skipAuthRoutes []string) error {
	var invalidRegexes []string
	for _, route := range skipAuthRoutes {
		if strings.Contains(route, ",") {
			return fmt.Errorf("comma is not allowed in route: %s", route)
		}
		var regex string
		parts := strings.SplitN(route, "=", 2)
		if len(parts) == 1 {
			regex = parts[0]
		} else {
			regex = parts[1]
		}
		_, err := regexp.Compile(regex)
		if err != nil {
			invalidRegexes = append(invalidRegexes, regex)
		}
	}
	if len(invalidRegexes) > 0 {
		return fmt.Errorf("invalid regex(es): %s", strings.Join(invalidRegexes, ","))
	}
	return nil
}

func validateOAuth(oauth *radixv1.OAuth2, component *radixv1.RadixComponent, environmentName string) (errors []error) {
	if oauth == nil {
		return
	}

	oauthWithDefaults, err := defaults.NewOAuth2Config(defaults.WithOAuth2Defaults()).MergeWith(oauth)
	if err != nil {
		errors = append(errors, err)
		return
	}
	componentName := component.Name
	// Validate ClientID
	if len(strings.TrimSpace(oauthWithDefaults.ClientID)) == 0 {
		errors = append(errors, fmt.Errorf("component %s in environment %s: %w", componentName, environmentName, ErrOAuthClientIdEmpty))
	} else if !componentHasPublicPort(component) {
		errors = append(errors, fmt.Errorf("component %s in environment %s: %w", componentName, environmentName, ErrOAuthRequiresPublicPort))
	}

	// Validate ProxyPrefix
	if len(strings.TrimSpace(oauthWithDefaults.ProxyPrefix)) == 0 {
		errors = append(errors, fmt.Errorf("component %s in environment %s: %w", componentName, environmentName, ErrOAuthProxyPrefixEmpty))
	} else if oauthutil.SanitizePathPrefix(oauthWithDefaults.ProxyPrefix) == "/" {
		errors = append(errors, fmt.Errorf("component %s in environment %s: %w", componentName, environmentName, ErrOAuthProxyPrefixIsRoot))
	}

	// Validate SessionStoreType
	if !commonUtils.ContainsString(validOAuthSessionStoreTypes, string(oauthWithDefaults.SessionStoreType)) {
		errors = append(errors, fmt.Errorf("component %s in environment %s: sessionStoreType '%s': %w", componentName, environmentName, oauthWithDefaults.SessionStoreType, ErrOAuthSessionStoreTypeInvalid))
	}

	// Validate RedisStore
	if oauthWithDefaults.IsSessionStoreTypeIsManuallyConfiguredRedis() {
		if redisStore := oauthWithDefaults.RedisStore; redisStore == nil {
			errors = append(errors, fmt.Errorf("component %s in environment %s: %w", componentName, environmentName, ErrOAuthRedisStoreEmpty))
		} else if len(strings.TrimSpace(redisStore.ConnectionURL)) == 0 {
			errors = append(errors, fmt.Errorf("component %s in environment %s: %w", componentName, environmentName, ErrOAuthRedisStoreConnectionURLEmpty))
		}
	}

	// Validate OIDC config
	if oidc := oauthWithDefaults.OIDC; oidc == nil {
		errors = append(errors, fmt.Errorf("component %s in environment %s: %w", componentName, environmentName, ErrOAuthOidcEmpty))
	} else {
		if oidc.SkipDiscovery == nil {
			errors = append(errors, fmt.Errorf("component %s in environment %s: %w", componentName, environmentName, ErrOAuthOidcSkipDiscoveryEmpty))
		} else if *oidc.SkipDiscovery {
			// Validate URLs when SkipDiscovery=true
			if len(strings.TrimSpace(oidc.JWKSURL)) == 0 {
				errors = append(errors, fmt.Errorf("component %s in environment %s: %w", componentName, environmentName, ErrOAuthOidcJwksUrlEmpty))
			}
			if len(strings.TrimSpace(oauthWithDefaults.LoginURL)) == 0 {
				errors = append(errors, fmt.Errorf("component %s in environment %s: %w", componentName, environmentName, ErrOAuthLoginUrlEmpty))
			}
			if len(strings.TrimSpace(oauthWithDefaults.RedeemURL)) == 0 {
				errors = append(errors, fmt.Errorf("component %s in environment %s: %w", componentName, environmentName, ErrOAuthRedeemUrlEmpty))
			}
		}
	}

	// Validate Cookie
	if cookie := oauthWithDefaults.Cookie; cookie == nil {
		errors = append(errors, fmt.Errorf("component %s in environment %s: %w", componentName, environmentName, ErrOAuthCookieEmpty))
	} else {
		if len(strings.TrimSpace(cookie.Name)) == 0 {
			errors = append(errors, fmt.Errorf("component %s in environment %s: %w", componentName, environmentName, ErrOAuthCookieNameEmpty))
		}
		if !commonUtils.ContainsString(validOAuthCookieSameSites, string(cookie.SameSite)) {
			errors = append(errors, fmt.Errorf("component %s in environment %s: sameSite '%s': %w", componentName, environmentName, cookie.SameSite, ErrOAuthCookieSameSiteInvalid))
		}

		// Validate Expire and Refresh
		expireValid, refreshValid := true, true

		expire, err := time.ParseDuration(cookie.Expire)
		if err != nil || expire < 0 {
			errors = append(errors, fmt.Errorf("component %s in environment %s: expire '%s': %w", componentName, environmentName, cookie.Expire, ErrOAuthCookieExpireInvalid))
			expireValid = false
		}
		refresh, err := time.ParseDuration(cookie.Refresh)
		if err != nil || refresh < 0 {
			errors = append(errors, fmt.Errorf("component %s in environment %s: refresh '%s': %w", componentName, environmentName, cookie.Refresh, ErrOAuthCookieRefreshInvalid))
			refreshValid = false
		}
		if expireValid && refreshValid && !(refresh < expire) {
			errors = append(errors, fmt.Errorf("component %s in environment %s: %w", componentName, environmentName, ErrOAuthCookieRefreshMustBeLessThanExpire))
		}

		// Validate required settings when sessionStore=cookie and cookieStore.minimal=true
		if oauthWithDefaults.SessionStoreType == radixv1.SessionStoreCookie && oauthWithDefaults.CookieStore != nil && oauthWithDefaults.CookieStore.Minimal != nil && *oauthWithDefaults.CookieStore.Minimal {
			// Refresh must be 0
			if refreshValid && refresh != 0 {
				errors = append(errors, fmt.Errorf("component %s in environment %s: %w", componentName, environmentName, ErrOAuthCookieStoreMinimalIncorrectCookieRefreshInterval))
			}
			// SetXAuthRequestHeaders must be false
			if oauthWithDefaults.SetXAuthRequestHeaders == nil || *oauthWithDefaults.SetXAuthRequestHeaders {
				errors = append(errors, fmt.Errorf("component %s in environment %s: %w", componentName, environmentName, ErrOAuthCookieStoreMinimalIncorrectSetXAuthRequestHeaders))
			}
			// SetAuthorizationHeader must be false
			if oauthWithDefaults.SetAuthorizationHeader == nil || *oauthWithDefaults.SetAuthorizationHeader {
				errors = append(errors, fmt.Errorf("component %s in environment %s: %w", componentName, environmentName, ErrOAuthCookieStoreMinimalIncorrectSetAuthorizationHeader))
			}
		}
	}

	if err = validateSkipAuthRoutes(oauthWithDefaults.SkipAuthRoutes); err != nil {
		errors = append(errors, fmt.Errorf("component %s in environment %s: %w", componentName, environmentName, fmt.Errorf("%w: %w", ErrOAuthSkipAuthRoutesInvalid, err)))
	}
	return
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

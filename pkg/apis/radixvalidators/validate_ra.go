package radixvalidators

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/branch"
	oauthutil "github.com/equinor/radix-operator/pkg/apis/utils/oauth"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	maximumNumberOfEgressRules = 1000
	azureClientIdResourceName  = "identity.azure.clientId"
)

var (
	validOAuthSessionStoreTypes = []string{string(radixv1.SessionStoreCookie), string(radixv1.SessionStoreRedis)}
	validOAuthCookieSameSites   = []string{string(radixv1.SameSiteStrict), string(radixv1.SameSiteLax), string(radixv1.SameSiteNone), string(radixv1.SameSiteEmpty)}
	blobFuse2Protocols          = []string{string(radixv1.BlobFuse2ProtocolFuse2), string(radixv1.BlobFuse2ProtocolNfs)}

	requiredRadixApplicationValidators = []RadixApplicationValidator{
		validateRadixApplicationAppName,
		validateComponents,
		validateJobComponents,
		validateNoDuplicateComponentAndJobNames,
		validateEnvNames,
		validateEnvironmentEgressRules,
		validateVariables,
		validateSecrets,
		validateBranchNames,
		validateDNSAppAlias,
		validateDNSExternalAlias,
		validatePrivateImageHubs,
		validateHPAConfigForRA,
		validateVolumeMountConfigForRA,
		ValidateNotificationsForRA,
	}
)

// RadixApplicationValidator defines a validator function for a RadixApplication
type RadixApplicationValidator func(radixApplication *radixv1.RadixApplication) error

// CanRadixApplicationBeInserted Checks if application config is valid. Returns a single error, if this is the case
func CanRadixApplicationBeInserted(radixClient radixclient.Interface, app *radixv1.RadixApplication, dnsAliasConfig *dnsalias.DNSConfig, additionalValidators ...RadixApplicationValidator) error {

	validators := append(requiredRadixApplicationValidators,
		validateDoesRRExistFactory(radixClient),
		validateDNSAliasFactory(radixClient, dnsAliasConfig),
	)
	validators = append(validators, additionalValidators...)

	return validateRadixApplication(app, validators...)
}

// IsRadixApplicationValid Checks if application config is valid without server validation
func IsRadixApplicationValid(app *radixv1.RadixApplication, additionalValidators ...RadixApplicationValidator) error {
	validators := append(requiredRadixApplicationValidators, additionalValidators...)
	return validateRadixApplication(app, validators...)
}

// IsApplicationNameLowercase checks if the application name has any uppercase letters
func IsApplicationNameLowercase(appName string) (bool, error) {
	for _, r := range appName {
		if unicode.IsUpper(r) && unicode.IsLetter(r) {
			return false, ApplicationNameNotLowercaseErrorWithMessage(appName)
		}
	}

	return true, nil
}

func duplicatePathForAzureKeyVault(path, azureKeyVaultName, component string) error {
	return fmt.Errorf("duplicate path %s for Azure Key vault %s, for component %s. See documentation for more info",
		path, azureKeyVaultName, component)
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

func validateRadixApplicationAppName(app *radixv1.RadixApplication) error {
	return validateAppName(app.Name)
}

func validateDoesRRExistFactory(client radixclient.Interface) RadixApplicationValidator {
	return func(radixApplication *radixv1.RadixApplication) error {
		return validateDoesRRExist(client, radixApplication.Name)
	}
}

func validateDNSAliasFactory(client radixclient.Interface, dnsAliasConfig *dnsalias.DNSConfig) RadixApplicationValidator {
	return func(radixApplication *radixv1.RadixApplication) error {
		return validateDNSAlias(client, radixApplication, dnsAliasConfig)
	}
}

func validatePrivateImageHubs(app *radixv1.RadixApplication) error {
	var errs []error
	for server, config := range app.Spec.PrivateImageHubs {
		if config.Username == "" {
			errs = append(errs, MissingPrivateImageHubUsernameErrorWithMessage(server))
		}
	}
	return errors.Join(errs...)
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

func validateDNSAppAlias(app *radixv1.RadixApplication) error {
	return validateDNSAppAliasComponentAndEnvironmentAvailable(app)
}

func validateDNSAlias(radixClient radixclient.Interface, app *radixv1.RadixApplication, dnsAliasConfig *dnsalias.DNSConfig) error {
	var errs []error
	radixDNSAliasMap, err := kube.GetRadixDNSAliasMap(radixClient)
	if err != nil {
		return err
	}
	uniqueAliasNames := make(map[string]struct{})
	for _, dnsAlias := range app.Spec.DNSAlias {
		if _, ok := uniqueAliasNames[dnsAlias.Alias]; ok {
			errs = append(errs, DuplicateAliasForDNSAliasError(dnsAlias.Alias))
		} else if err = validateRequiredResourceName("dnsAlias alias", dnsAlias.Alias); err != nil {
			errs = append(errs, err)
		}
		uniqueAliasNames[dnsAlias.Alias] = struct{}{}
		componentNameIsValid, environmentNameIsValid := true, true
		if err = validateRequiredResourceName("dnsAlias component", dnsAlias.Component); err != nil {
			errs = append(errs, err)
			componentNameIsValid = false
		}
		if err = validateRequiredResourceName("dnsAlias environment", dnsAlias.Environment); err != nil {
			errs = append(errs, err)
			environmentNameIsValid = false
		}
		if !componentNameIsValid || !environmentNameIsValid {
			continue
		}
		if err = validateDNSAliasComponentAndEnvironmentAvailable(app, dnsAlias.Component, dnsAlias.Environment); err != nil {
			errs = append(errs, err)
			continue
		}
		if !doesComponentHaveAPublicPort(app, dnsAlias.Component) {
			errs = append(errs, ComponentForDNSAliasIsNotMarkedAsPublicError(dnsAlias.Component))
			continue
		}
		if radixDNSAlias, ok := radixDNSAliasMap[dnsAlias.Alias]; ok && radixDNSAlias.Spec.AppName != app.Name {
			errs = append(errs, RadixDNSAliasAlreadyUsedByAnotherApplicationError(dnsAlias.Alias))
		}
		if reservingAppName, aliasReserved := dnsAliasConfig.ReservedAppDNSAliases[dnsAlias.Alias]; aliasReserved && reservingAppName != app.Name {
			errs = append(errs, RadixDNSAliasIsReservedForRadixPlatformApplicationError(dnsAlias.Alias))
		}
		if slice.Any(dnsAliasConfig.ReservedDNSAliases, func(reservedAlias string) bool { return reservedAlias == dnsAlias.Alias }) {
			errs = append(errs, RadixDNSAliasIsReservedForRadixPlatformServiceError(dnsAlias.Alias))
		}
	}
	return errors.Join(errs...)
}

func validateDNSAppAliasComponentAndEnvironmentAvailable(app *radixv1.RadixApplication) error {
	alias := app.Spec.DNSAppAlias
	if alias.Component == "" && alias.Environment == "" {
		return nil
	}

	var errs []error
	if !doesEnvExist(app, alias.Environment) {
		errs = append(errs, EnvForDNSAppAliasNotDefinedErrorWithMessage(alias.Environment))
	}
	if !doesComponentExistInEnvironment(app, alias.Component, alias.Environment) {
		errs = append(errs, ComponentForDNSAppAliasNotDefinedErrorWithMessage(alias.Component))
	}
	return errors.Join(errs...)
}

func validateDNSAliasComponentAndEnvironmentAvailable(app *radixv1.RadixApplication, component string, environment string) error {
	if !doesEnvExist(app, environment) {
		return EnvForDNSAliasNotDefinedError(environment)
	}
	if !doesComponentExistInEnvironment(app, component, environment) {
		return ComponentForDNSAliasNotDefinedError(component)
	}
	return nil
}

func validateDNSExternalAlias(app *radixv1.RadixApplication) error {
	errs := []error{}

	distinctAlias := make(map[string]bool)

	for _, externalAlias := range app.Spec.DNSExternalAlias {
		if externalAlias.Alias == "" && externalAlias.Component == "" && externalAlias.Environment == "" {
			return nil
		}

		distinctAlias[externalAlias.Alias] = true

		if externalAlias.Alias == "" {
			errs = append(errs, ErrExternalAliasCannotBeEmpty)
		}

		if !doesEnvExist(app, externalAlias.Environment) {
			errs = append(errs, EnvForDNSExternalAliasNotDefinedErrorWithMessage(externalAlias.Environment))
		}
		if !doesComponentExistInEnvironment(app, externalAlias.Component, externalAlias.Environment) {
			errs = append(errs, ComponentForDNSExternalAliasNotDefinedErrorWithMessage(externalAlias.Component))
		}

		if !doesComponentHaveAPublicPort(app, externalAlias.Component) {
			errs = append(errs, ComponentForDNSExternalAliasIsNotMarkedAsPublicErrorWithMessage(externalAlias.Component))
		}
	}

	if len(distinctAlias) < len(app.Spec.DNSExternalAlias) {
		errs = append(errs, DuplicateExternalAliasErrorWithMessage())
	}

	return errors.Join(errs...)
}

func validateNoDuplicateComponentAndJobNames(app *radixv1.RadixApplication) error {
	names := make(map[string]int)
	for _, component := range app.Spec.Components {
		names[component.Name]++
	}
	for _, job := range app.Spec.Jobs {
		names[job.Name]++
	}

	var duplicates []string
	for k, v := range names {
		if v > 1 {
			duplicates = append(duplicates, k)
		}
	}
	if len(duplicates) > 0 {
		return DuplicateComponentOrJobNameErrorWithMessage(duplicates)
	}
	return nil
}

func validateComponents(app *radixv1.RadixApplication) error {
	var errs []error
	for _, component := range app.Spec.Components {
		if component.Image != "" &&
			(component.SourceFolder != "" || component.DockerfileName != "") {
			errs = append(errs, PublicImageComponentCannotHaveSourceOrDockerfileSetWithMessage(component.Name))
		}

		err := validateComponentName(component.Name, "component")
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

		err = validateMonitoring(&component)
		if err != nil {
			errs = append(errs, err)
		}

		/*err = validateUseReadOnlyFileSystem(component.UseReadOnlyFileSystem)
		if err != nil {
			errs = append(errs, err)
		}*/

		errs = append(errs, validateAuthentication(&component, app.Spec.Environments)...)

		err = validateIdentity(component.Identity)
		if err != nil {
			errs = append(errs, err)
		}

		for _, environment := range component.EnvironmentConfig {
			if !doesEnvExist(app, environment.Environment) {
				err = EnvironmentReferencedByComponentDoesNotExistErrorWithMessage(environment.Environment, component.Name)
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
					ComponentWithTagInEnvironmentConfigForEnvironmentRequiresDynamicTagWithMessage(component.Name, environment.Environment))
			}

			err = validateIdentity(environment.Identity)
			if err != nil {
				errs = append(errs, err)
			}

			/*err = validateUseReadOnlyFileSystem(component.UseReadOnlyFileSystem)
			if err != nil {
				errs = append(errs, err)
			}*/
		}
	}

	return errors.Join(errs...)
}

func validateJobComponents(app *radixv1.RadixApplication) error {
	var errs []error
	for _, job := range app.Spec.Jobs {
		if job.Image != "" && (job.SourceFolder != "" || job.DockerfileName != "") {
			errs = append(errs, PublicImageComponentCannotHaveSourceOrDockerfileSetWithMessage(job.Name))
		}

		err := validateComponentName(job.Name, "job")
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

		err = validateMonitoring(&job)
		if err != nil {
			errs = append(errs, err)
		}

		err = validateIdentity(job.Identity)
		if err != nil {
			errs = append(errs, err)
		}

		/*err = validateUseReadOnlyFileSystem(job.UseReadOnlyFileSystem)
		if err != nil {
			errs = append(errs, err)
		}*/

		for _, environment := range job.EnvironmentConfig {
			if !doesEnvExist(app, environment.Environment) {
				err = EnvironmentReferencedByComponentDoesNotExistErrorWithMessage(environment.Environment, job.Name)
				errs = append(errs, err)
			}

			errList = validateResourceRequirements(&environment.Resources)
			if len(errList) > 0 {
				errs = append(errs, errList...)
			}

			if environmentHasDynamicTaggingButImageLacksTag(environment.ImageTagName, job.Image) {
				errs = append(errs,
					ComponentWithTagInEnvironmentConfigForEnvironmentRequiresDynamicTagWithMessage(job.Name, environment.Environment))
			}

			err = validateIdentity(environment.Identity)
			if err != nil {
				errs = append(errs, err)
			}

			/*err = validateUseReadOnlyFileSystem(job.UseReadOnlyFileSystem)
			if err != nil {
				errs = append(errs, err)
			}*/
		}
	}

	return errors.Join(errs...)
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
		return InvalidVerificationTypeWithMessage(actualValue)
	} else {
		return nil
	}
}

func componentHasPublicPort(component *radixv1.RadixComponent) bool {
	return slice.Any(component.GetPorts(),
		func(p radixv1.ComponentPort) bool {
			return len(p.Name) > 0 && (p.Name == component.PublicPort || component.Public)
		})
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
		errors = append(errors, OAuthClientIdEmptyErrorWithMessage(componentName, environmentName))
	} else if !componentHasPublicPort(component) {
		errors = append(errors, OAuthRequiresPublicPortErrorWithMessage(componentName, environmentName))
	}

	// Validate ProxyPrefix
	if len(strings.TrimSpace(oauthWithDefaults.ProxyPrefix)) == 0 {
		errors = append(errors, OAuthProxyPrefixEmptyErrorWithMessage(componentName, environmentName))
	} else if oauthutil.SanitizePathPrefix(oauthWithDefaults.ProxyPrefix) == "/" {
		errors = append(errors, OAuthProxyPrefixIsRootErrorWithMessage(componentName, environmentName))
	}

	// Validate SessionStoreType
	if !commonUtils.ContainsString(validOAuthSessionStoreTypes, string(oauthWithDefaults.SessionStoreType)) {
		errors = append(errors, OAuthSessionStoreTypeInvalidErrorWithMessage(componentName, environmentName, oauthWithDefaults.SessionStoreType))
	}

	// Validate RedisStore
	if oauthWithDefaults.SessionStoreType == radixv1.SessionStoreRedis {
		if redisStore := oauthWithDefaults.RedisStore; redisStore == nil {
			errors = append(errors, OAuthRedisStoreEmptyErrorWithMessage(componentName, environmentName))
		} else if len(strings.TrimSpace(redisStore.ConnectionURL)) == 0 {
			errors = append(errors, OAuthRedisStoreConnectionURLEmptyErrorWithMessage(componentName, environmentName))
		}
	}

	// Validate OIDC config
	if oidc := oauthWithDefaults.OIDC; oidc == nil {
		errors = append(errors, OAuthOidcEmptyErrorWithMessage(componentName, environmentName))
	} else {
		if oidc.SkipDiscovery == nil {
			errors = append(errors, OAuthOidcSkipDiscoveryEmptyErrorWithMessage(componentName, environmentName))
		} else if *oidc.SkipDiscovery {
			// Validate URLs when SkipDiscovery=true
			if len(strings.TrimSpace(oidc.JWKSURL)) == 0 {
				errors = append(errors, OAuthOidcJwksUrlEmptyErrorWithMessage(componentName, environmentName))
			}
			if len(strings.TrimSpace(oauthWithDefaults.LoginURL)) == 0 {
				errors = append(errors, OAuthLoginUrlEmptyErrorWithMessage(componentName, environmentName))
			}
			if len(strings.TrimSpace(oauthWithDefaults.RedeemURL)) == 0 {
				errors = append(errors, OAuthRedeemUrlEmptyErrorWithMessage(componentName, environmentName))
			}
		}
	}

	// Validate Cookie
	if cookie := oauthWithDefaults.Cookie; cookie == nil {
		errors = append(errors, OAuthCookieEmptyErrorWithMessage(componentName, environmentName))
	} else {
		if len(strings.TrimSpace(cookie.Name)) == 0 {
			errors = append(errors, OAuthCookieNameEmptyErrorWithMessage(componentName, environmentName))
		}
		if !commonUtils.ContainsString(validOAuthCookieSameSites, string(cookie.SameSite)) {
			errors = append(errors, OAuthCookieSameSiteInvalidErrorWithMessage(componentName, environmentName, cookie.SameSite))
		}

		// Validate Expire and Refresh
		expireValid, refreshValid := true, true

		expire, err := time.ParseDuration(cookie.Expire)
		if err != nil || expire < 0 {
			errors = append(errors, OAuthCookieExpireInvalidErrorWithMessage(componentName, environmentName, cookie.Expire))
			expireValid = false
		}
		refresh, err := time.ParseDuration(cookie.Refresh)
		if err != nil || refresh < 0 {
			errors = append(errors, OAuthCookieRefreshInvalidErrorWithMessage(componentName, environmentName, cookie.Refresh))
			refreshValid = false
		}
		if expireValid && refreshValid && !(refresh < expire) {
			errors = append(errors, OAuthCookieRefreshMustBeLessThanExpireErrorWithMessage(componentName, environmentName))
		}

		// Validate required settings when sessionStore=cookie and cookieStore.minimal=true
		if oauthWithDefaults.SessionStoreType == radixv1.SessionStoreCookie && oauthWithDefaults.CookieStore != nil && oauthWithDefaults.CookieStore.Minimal != nil && *oauthWithDefaults.CookieStore.Minimal {
			// Refresh must be 0
			if refreshValid && refresh != 0 {
				errors = append(errors, OAuthCookieStoreMinimalIncorrectCookieRefreshIntervalErrorWithMessage(componentName, environmentName))
			}
			// SetXAuthRequestHeaders must be false
			if oauthWithDefaults.SetXAuthRequestHeaders == nil || *oauthWithDefaults.SetXAuthRequestHeaders {
				errors = append(errors, OAuthCookieStoreMinimalIncorrectSetXAuthRequestHeadersErrorWithMessage(componentName, environmentName))
			}
			// SetAuthorizationHeader must be false
			if oauthWithDefaults.SetAuthorizationHeader == nil || *oauthWithDefaults.SetAuthorizationHeader {
				errors = append(errors, OAuthCookieStoreMinimalIncorrectSetAuthorizationHeaderErrorWithMessage(componentName, environmentName))
			}
		}
	}

	return
}

func environmentHasDynamicTaggingButImageLacksTag(environmentImageTag, componentImage string) bool {
	return environmentImageTag != "" &&
		(componentImage == "" ||
			!strings.HasSuffix(componentImage, radixv1.DynamicTagNameInEnvironmentConfig))
}

func validateJobSchedulerPort(job *radixv1.RadixJobComponent) error {
	if job.SchedulerPort == nil {
		return SchedulerPortCannotBeEmptyForJobErrorWithMessage(job.Name)
	}

	return nil
}

func validateJobPayload(job *radixv1.RadixJobComponent) error {
	if job.Payload != nil && job.Payload.Path == "" {
		return PayloadPathCannotBeEmptyForJobErrorWithMessage(job.Name)
	}

	return nil
}

func validatePorts(componentName string, ports []radixv1.ComponentPort) []error {
	errs := []error{}

	for _, port := range ports {
		if len(port.Name) > maxPortNameLength {
			err := InvalidPortNameLengthErrorWithMessage(port.Name)
			if err != nil {
				errs = append(errs, err)
			}
		}

		err := validateRequiredResourceName("port name", port.Name)
		if err != nil {
			errs = append(errs, err)
		}

		if port.Port < minimumPortNumber || port.Port > maximumPortNumber {
			if err := InvalidPortNumberErrorWithMessage(port.Port); err != nil {
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
			errs = append(errs, PortNameIsRequiredForPublicComponentErrorWithMessage(publicPortName, component.Name))
		}
		if matchingPortName > 1 {
			errs = append(errs, MultipleMatchingPortNamesErrorWithMessage(matchingPortName, publicPortName, component.Name))
		}
	}

	return errs
}

func validateMonitoring(component radixv1.RadixCommonComponent) error {
	monitoringConfig := component.GetMonitoringConfig()
	if monitoringConfig.PortName != "" {
		ports := component.GetPorts()
		isValidPort := false
		for i := range ports {
			if strings.EqualFold(ports[i].Name, monitoringConfig.PortName) {
				isValidPort = true
				break
			}
		}

		if !isValidPort {
			return MonitoringPortNameIsNotFoundComponentErrorWithMessage(monitoringConfig.PortName, component.GetName())
		}
	}
	return nil
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
			errs = append(errs, ResourceRequestOverLimitErrorWithMessage(name, value, limit.String()))
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
			return quantity, MemoryResourceRequirementFormatErrorWithMessage(value)
		}
	} else if name == "cpu" {
		quantity, err = resource.ParseQuantity(value)
		re := regexp.MustCompile(cpuRegex)
		isValid := re.MatchString(value)
		if err != nil || !isValid {
			return quantity, CPUResourceRequirementFormatErrorWithMessage(value)
		}
	} else {
		return quantity, InvalidResourceErrorWithMessage(name)
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
		if err := validateRadixComponentSecrets(&component, app); err != nil {
			return err
		}
	}
	for _, job := range app.Spec.Jobs {
		if err := validateRadixComponentSecrets(&job, app); err != nil {
			return err
		}
	}
	return nil
}

func validateRadixComponentSecrets(component radixv1.RadixCommonComponent, app *radixv1.RadixApplication) error {
	if err := validateSecretNames("secret name", component.GetSecrets()); err != nil {
		return err
	}

	envsEnvVarsMap := make(map[string]map[string]bool)

	for _, env := range app.Spec.Environments {
		var envEnvVars radixv1.EnvVarsMap
		if envConfig := component.GetEnvironmentConfigByName(env.Name); envConfig != radixv1.RadixCommonEnvironmentConfig(nil) {
			envEnvVars = envConfig.GetVariables()
		}
		envsEnvVarsMap[env.Name] = getEnvVarNameMap(component.GetVariables(), envEnvVars)
	}

	if err := validateConflictingEnvironmentAndSecretNames(component.GetName(), component.GetSecrets(), envsEnvVarsMap); err != nil {
		return err
	}

	for _, env := range component.GetEnvironmentConfig() {
		envsEnvVarsWithSecretsMap, ok := envsEnvVarsMap[env.GetEnvironment()]
		if !ok {
			continue
		}
		for _, secret := range component.GetSecrets() {
			envsEnvVarsWithSecretsMap[secret] = true
		}
		envsEnvVarsMap[env.GetEnvironment()] = envsEnvVarsWithSecretsMap
	}

	if err := validateRadixComponentSecretRefs(component); err != nil {
		return err
	}

	return validateConflictingEnvironmentAndSecretRefsNames(component, envsEnvVarsMap)
}

func getEnvVarNameMap(componentEnvVarsMap radixv1.EnvVarsMap, envsEnvVarsMap radixv1.EnvVarsMap) map[string]bool {
	envVarsMap := make(map[string]bool)
	for name := range componentEnvVarsMap {
		envVarsMap[name] = true
	}
	for name := range envsEnvVarsMap {
		envVarsMap[name] = true
	}
	return envVarsMap
}

func validateSecretNames(resourceName string, secrets []string) error {
	existingSecret := make(map[string]bool)
	for _, secret := range secrets {
		if _, exists := existingSecret[secret]; exists {
			return duplicateSecretNameWithMessage(secret)
		}
		existingSecret[secret] = true
		if err := validateVariableName(resourceName, secret); err != nil {
			return err
		}
	}
	return nil
}

func validateRadixComponentSecretRefs(radixComponent radixv1.RadixCommonComponent) error {
	err := validateSecretRefs(radixComponent, radixComponent.GetSecretRefs())
	if err != nil {
		return err
	}
	for _, envConfig := range radixComponent.GetEnvironmentConfig() {
		err := validateSecretRefs(radixComponent, envConfig.GetSecretRefs())
		if err != nil {
			return err
		}
	}
	return validateSecretRefsPath(radixComponent)
}

func validateSecretRefs(commonComponent radixv1.RadixCommonComponent, secretRefs radixv1.RadixSecretRefs) error {
	existingVariableName := make(map[string]bool)
	existingAlias := make(map[string]bool)
	existingAzureKeyVaultName := make(map[string]bool)
	existingAzureKeyVaultPath := make(map[string]bool)
	for _, azureKeyVault := range secretRefs.AzureKeyVaults {
		if _, exists := existingAzureKeyVaultName[azureKeyVault.Name]; exists {
			return duplicateAzureKeyVaultNameWithMessage(azureKeyVault.Name)
		}
		existingAzureKeyVaultName[azureKeyVault.Name] = true
		path := azureKeyVault.Path
		if path != nil && len(*path) > 0 {
			if _, exists := existingAzureKeyVaultPath[*path]; exists {
				return duplicatePathForAzureKeyVault(*path, azureKeyVault.Name, commonComponent.GetName())
			}
			existingAzureKeyVaultPath[*path] = true
		}
		useAzureIdentity := azureKeyVault.UseAzureIdentity
		if useAzureIdentity != nil && *useAzureIdentity {
			if !azureIdentityIsSet(commonComponent) {
				return MissingAzureIdentityErrorWithMessage(azureKeyVault.Name, commonComponent.GetName())
			}
			// TODO: validate for env-chain
		}
		for _, keyVaultItem := range azureKeyVault.Items {
			if len(keyVaultItem.EnvVar) > 0 {
				if _, exists := existingVariableName[keyVaultItem.EnvVar]; exists {
					return duplicateEnvVarNameWithMessage(keyVaultItem.EnvVar)
				}
				existingVariableName[keyVaultItem.EnvVar] = true
				if err := validateVariableName("Azure Key vault secret references environment variable name", keyVaultItem.EnvVar); err != nil {
					return err
				}
			}
			if err := validateVariableName("Azure Key vault secret references name", keyVaultItem.Name); err != nil {
				return err
			}
			if keyVaultItem.Alias != nil && len(*keyVaultItem.Alias) > 0 {
				if _, exists := existingAlias[*keyVaultItem.Alias]; exists {
					return duplicateAliasWithMessage(*keyVaultItem.Alias)
				}
				existingAlias[*keyVaultItem.Alias] = true
				if err := validateVariableName("Azure Key vault item alias name", *keyVaultItem.Alias); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func azureIdentityIsSet(commonComponent radixv1.RadixCommonComponent) bool {
	identity := commonComponent.GetIdentity()
	if identity != nil && identity.Azure != nil && validateExpectedAzureIdentity(*identity.Azure) == nil {
		return true
	}
	for _, envConfig := range commonComponent.GetEnvironmentConfig() {
		if !commonComponent.GetEnabledForEnv(envConfig) {
			continue
		}
		envIdentity := envConfig.GetIdentity()
		if envIdentity != nil && envIdentity.Azure != nil && validateExpectedAzureIdentity(*envIdentity.Azure) == nil {
			return true
		}
	}
	return false
}

func validateSecretRefsPath(radixComponent radixv1.RadixCommonComponent) error {
	commonAzureKeyVaultPathMap := make(map[string]string)
	for _, azureKeyVault := range radixComponent.GetSecretRefs().AzureKeyVaults {
		path := azureKeyVault.Path
		if path != nil && len(*path) > 0 { // set only non-empty common path
			commonAzureKeyVaultPathMap[azureKeyVault.Name] = *path
		}
	}
	for _, environmentConfig := range radixComponent.GetEnvironmentConfig() {
		envAzureKeyVaultPathMap := make(map[string]string)
		for commonAzureKeyVaultName, path := range commonAzureKeyVaultPathMap {
			envAzureKeyVaultPathMap[commonAzureKeyVaultName] = path
		}
		for _, envAzureKeyVault := range environmentConfig.GetSecretRefs().AzureKeyVaults {
			if envAzureKeyVault.Path != nil && len(*envAzureKeyVault.Path) > 0 { // override common path by non-empty env-path, or set non-empty env path
				envAzureKeyVaultPathMap[envAzureKeyVault.Name] = *envAzureKeyVault.Path
			}
		}
		envPathMap := make(map[string]bool)
		for azureKeyVaultName, path := range envAzureKeyVaultPathMap {
			if _, existsForOtherKeyVault := envPathMap[path]; existsForOtherKeyVault {
				return duplicatePathForAzureKeyVault(path, azureKeyVaultName, radixComponent.GetName())
			}
			envPathMap[path] = true
		}
	}
	return nil
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

func validateConflictingEnvironmentAndSecretNames(componentName string, secrets []string, envsEnvVarMap map[string]map[string]bool) error {
	for _, secret := range secrets {
		for _, envVarMap := range envsEnvVarMap {
			if _, contains := envVarMap[secret]; contains {
				return SecretNameConflictsWithEnvironmentVariableWithMessage(componentName, secret)
			}
		}
	}
	return nil
}

func validateConflictingEnvironmentAndSecretRefsNames(component radixv1.RadixCommonComponent, envsEnvVarMap map[string]map[string]bool) error {
	for _, azureKeyVault := range component.GetSecretRefs().AzureKeyVaults {
		for _, item := range azureKeyVault.Items {
			for _, envVarMap := range envsEnvVarMap {
				if _, contains := envVarMap[item.EnvVar]; contains {
					return secretRefEnvVarNameConflictsWithEnvironmentVariableWithMessage(component.GetName(), item.EnvVar)
				}
			}
		}
	}
	for _, environmentConfig := range component.GetEnvironmentConfig() {
		for _, azureKeyVault := range environmentConfig.GetSecretRefs().AzureKeyVaults {
			for _, item := range azureKeyVault.Items {
				if envVarMap, ok := envsEnvVarMap[environmentConfig.GetEnvironment()]; ok {
					if _, contains := envVarMap[item.EnvVar]; contains {
						return secretRefEnvVarNameConflictsWithEnvironmentVariableWithMessage(component.GetName(), item.EnvVar)
					}
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
			return InvalidStringValueMaxLengthErrorWithMessage("branch from", env.Build.From, 253)
		}

		isValid := branch.IsValidPattern(env.Build.From)
		if !isValid {
			return InvalidBranchNameErrorWithMessage(env.Build.From)
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

func validateVariableName(resourceName, value string) error {
	if err := validateIllegalPrefixInVariableName(resourceName, value); err != nil {
		return err
	}
	return validateResourceWithRegexp(resourceName, value, "^(([A-Za-z0-9][-._A-Za-z0-9.]*)?[A-Za-z0-9])?$")
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
				return MaxReplicasForHPANotSetOrZeroErrorWithMessage(componentName, environment)
			}
			if minReplicas != nil && *minReplicas > maxReplicas {
				return MinReplicasGreaterThanMaxReplicasErrorWithMessage(componentName, environment)
			}
			if envConfig.HorizontalScaling.RadixHorizontalScalingResources != nil && envConfig.HorizontalScaling.RadixHorizontalScalingResources.Cpu == nil && envConfig.HorizontalScaling.RadixHorizontalScalingResources.Memory == nil {
				return NoScalingResourceSetErrorWithMessage(componentName, environment)
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

func validateVolumeMounts(componentName, environment string, volumeMounts []radixv1.RadixVolumeMount) error {
	if len(volumeMounts) == 0 {
		return nil
	}

	mountsInComponent := make(map[string]volumeMountConfigMaps)

	for _, volumeMount := range volumeMounts {
		volumeMountType := string(deployment.GetCsiAzureVolumeMountType(&volumeMount))
		volumeMountStorage := deployment.GetRadixVolumeMountStorage(&volumeMount)
		switch {
		case len(volumeMount.Type) == 0 && volumeMount.BlobFuse2 == nil && volumeMount.AzureFile == nil:
			return emptyVolumeMountTypeOrDriverSectionErrorWithMessage(componentName, environment)
		case multipleVolumeTypesDefined(&volumeMount):
			return multipleVolumeMountTypesDefinedErrorWithMessage(componentName, environment)
		case strings.TrimSpace(volumeMount.Name) == "" ||
			strings.TrimSpace(volumeMount.Path) == "":
			return emptyVolumeMountNameOrPathErrorWithMessage(componentName, environment)
		case volumeMount.BlobFuse2 == nil && volumeMount.AzureFile == nil && len(volumeMount.Type) > 0 && len(volumeMountStorage) == 0:
			return emptyVolumeMountStorageErrorWithMessage(componentName, environment)
		case volumeMount.BlobFuse2 != nil:
			switch {
			case len(volumeMount.BlobFuse2.Container) == 0:
				return emptyBlobFuse2VolumeMountContainerErrorWithMessage(componentName, environment)
			case len(string(volumeMount.BlobFuse2.Protocol)) > 0 && !commonUtils.ContainsString(blobFuse2Protocols, string(volumeMount.BlobFuse2.Protocol)):
				return unsupportedBlobFuse2VolumeMountProtocolErrorWithMessage(componentName, environment)
			}
			fallthrough
		case radixv1.IsKnownVolumeMount(volumeMountType):
			{
				if _, exists := mountsInComponent[volumeMountType]; !exists {
					mountsInComponent[volumeMountType] = volumeMountConfigMaps{names: make(map[string]bool), path: make(map[string]bool)}
				}
				volumeMountConfigMap := mountsInComponent[volumeMountType]
				if _, exists := volumeMountConfigMap.names[volumeMount.Name]; exists {
					return duplicateNameForVolumeMountTypeWithMessage(volumeMount.Name, volumeMountType, componentName, environment)
				}
				volumeMountConfigMap.names[volumeMount.Name] = true
				if _, exists := volumeMountConfigMap.path[volumeMount.Path]; exists {
					return duplicatePathForVolumeMountTypeWithMessage(volumeMount.Path, volumeMountType, componentName, environment)
				}
				volumeMountConfigMap.path[volumeMount.Path] = true
			}
		default:
			return unknownVolumeMountTypeErrorWithMessage(volumeMountType, componentName, environment)
		}
	}

	return nil
}

/*
	func validateUseReadOnlyFileSystem(useReadOnlyFileSystem *bool) error {
		if useReadOnlyFileSystem == nil {
			return nil
		}
		return nil
	}
*/
func multipleVolumeTypesDefined(volumeMount *radixv1.RadixVolumeMount) bool {
	count := 0
	if len(volumeMount.Type) > 0 {
		count++
	}
	if volumeMount.BlobFuse2 != nil {
		count++
	}
	if volumeMount.AzureFile != nil {
		count++
	}
	return count > 1
}

func validateIdentity(identity *radixv1.Identity) error {
	if identity == nil {
		return nil
	}

	return validateAzureIdentity(identity.Azure)
}

func validateAzureIdentity(azureIdentity *radixv1.AzureIdentity) error {

	if azureIdentity == nil {
		return nil
	}

	return validateExpectedAzureIdentity(*azureIdentity)
}

func validateExpectedAzureIdentity(azureIdentity radixv1.AzureIdentity) error {
	if len(strings.TrimSpace(azureIdentity.ClientId)) == 0 {
		return ResourceNameCannotBeEmptyErrorWithMessage(azureClientIdResourceName)
	}
	if err := uuid.Validate(azureIdentity.ClientId); err != nil {
		return InvalidUUIDErrorWithMessage(azureClientIdResourceName, azureIdentity.ClientId)
	}
	return nil
}

type volumeMountConfigMaps struct {
	names map[string]bool
	path  map[string]bool
}

func doesComponentExistInEnvironment(app *radixv1.RadixApplication, componentName string, environment string) bool {
	for _, component := range app.Spec.Components {
		if component.Name == componentName {
			environmentConfig := component.GetEnvironmentConfigByName(environment)
			return component.GetEnabledForEnv(environmentConfig)
		}
	}
	return false
}

func doesEnvExist(app *radixv1.RadixApplication, name string) bool {
	return getEnv(app, name) != nil
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
			return component.Public || component.PublicPort != ""
		}
	}
	return false
}

func validateComponentName(componentName, componentType string) error {
	if err := validateRequiredResourceName(fmt.Sprintf("%s name", componentType), componentName); err != nil {
		return err
	}

	for _, aux := range []string{defaults.OAuthProxyAuxiliaryComponentSuffix} {
		if strings.HasSuffix(componentName, fmt.Sprintf("-%s", aux)) {
			return ComponentNameReservedSuffixErrorWithMessage(componentName, componentType, string(aux))
		}
	}
	return nil
}

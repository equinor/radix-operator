package radixapplication

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	oauthutil "github.com/equinor/radix-operator/pkg/apis/utils/oauth"
)

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

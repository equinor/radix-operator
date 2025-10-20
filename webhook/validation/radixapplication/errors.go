package radixapplication

import (
	"errors"
)

var (
	ErrUnknownServerError                          = errors.New("server error, please try again later")
	ErrNoRadixApplication                          = errors.New("no corresponding radix application found. Name of the application in radixconfig.yaml needs to be exactly the same as used when defining the app in the console")
	ErrDNSAliasComponentIsNotMarkedAsPublic        = errors.New("component for dns alias is not marked as public")
	ErrDNSAliasAlreadyUsedByAnotherApplication     = errors.New("dns alias is already used by another application")
	ErrDNSAliasReservedForRadixPlatformApplication = errors.New("dns alias is reserved for Radix platform application")
	ErrDNSAliasReservedForRadixPlatformService     = errors.New("dns alias is reserved for Radix platform service")
	ErrDNSAliasEnvironmentNotDefined               = errors.New("environment for dns alias is not defined in radix application")
	ErrDNSAliasComponentNotDefinedOrDisabled       = errors.New("component for dns alias is not defined or disabled in radix application")
	ErrExternalAliasCannotBeEmpty                  = errors.New("external alias cannot be empty")

	ErrExternalAliasEnvironmentNotDefined      = errors.New("environment for external alias is not defined in radix application")
	ErrExternalAliasComponentNotDefined        = errors.New("component for external alias is not defined or not enabled in radix application")
	ErrExternalAliasComponentNotMarkedAsPublic = errors.New("component for external alias is not marked as public")
	ErrRequestedResourceExceedsLimit           = errors.New("requested resource exceeds defined limit")
	ErrNoPublicPortMarkedForComponent          = errors.New("no public port marked for component")
	ErrMonitoringNamedPortNotFound             = errors.New("monitoring named port not found in component ports")
	ErrMonitoringNoPortsDefined                = errors.New("no ports defined for component with monitoring enabled")
)

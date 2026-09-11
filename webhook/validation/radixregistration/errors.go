package radixregistration

import (
	"errors"
)

var (
	ErrInternalError                      = errors.New("internal error. Please try again later or contact support if the issue persists")
	ErrGroupIsRequired                    = errors.New("group is required")
	ErrConfigurationItemIsRequired        = errors.New("configuration item is required")
	ErrAppIdMustBeUnique                  = errors.New("app id must be unique")
	ErrAppNameTooLong                     = errors.New("application name cannot exceed 40 characters")
	WarningGroupsShouldHaveAtleastOneItem = "warning: groups should have at least one item"
	ErrEnvironmentNameIsNotAvailable      = errors.New("app name is not available. it is already in use or conflicts with reserved namespace")
)

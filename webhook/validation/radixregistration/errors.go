package radixregistration

import (
	"errors"
)

var (
	ErrInternalError                        = errors.New("internal error. Please try again later or contact support if the issue persists")
	ErrAdGroupIsRequired                    = errors.New("ad group is required")
	ErrConfigurationItemIsRequired          = errors.New("configuration item is required")
	ErrAppIdMustBeUnique                    = errors.New("app id must be unique")
	WarningAdGroupsShouldHaveAtleastOneItem = "warning: adGroups should have at least one item"
	ErrEnvironmentNameIsNotAvailable        = errors.New("app name is not available. it is already in use or conflicts with reserved namespace")
)

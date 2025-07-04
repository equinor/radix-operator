package radixregistration

import (
	"errors"
)

var (
	ErrAdGroupIsRequired                    = errors.New("ad group is required")
	ErrConfigurationItemIsRequired          = errors.New("configuration item is required")
	ErrAppIdMustBeUnique                    = errors.New("app id must be unique")
	WarningAdGroupsShouldHaveAtleastOneItem = "warning: adGroups should have at least one item"
)

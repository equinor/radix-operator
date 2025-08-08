package radixregistration

import (
	"errors"
)

var (
	ErrUnknownServerError                   = errors.New("server error, please try again later")
	ErrAdGroupIsRequired                    = errors.New("ad group is required")
	ErrConfigurationItemIsRequired          = errors.New("configuration item is required")
	ErrAppIdMustBeUnique                    = errors.New("app id must be unique")
	WarningAdGroupsShouldHaveAtleastOneItem = "warning: adGroups should have at least one item"
	ErrConfigurationItemIsNotValid          = errors.New("configuration item is not valid")
)

package token

import (
	"context"
	"errors"
	"fmt"
)

type ChainedValidator struct{ validators []ValidatorInterface }

var _ ValidatorInterface = &ChainedValidator{}
var errNoValidatorsFound = errors.New("no validators found")

func NewChainedValidator(validators ...ValidatorInterface) *ChainedValidator {
	return &ChainedValidator{validators}
}

func (v *ChainedValidator) ValidateToken(ctx context.Context, token string) (TokenPrincipal, error) {
	var errs []error

	for _, validator := range v.validators {
		principal, err := validator.ValidateToken(ctx, token)
		if principal != nil {
			return principal, nil
		}
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("no issuers could validate token: %w", errors.Join(errs...))
	}

	return nil, errNoValidatorsFound
}

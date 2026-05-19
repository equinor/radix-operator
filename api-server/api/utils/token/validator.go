package token

import (
	"context"
	"net/url"
	"time"

	"github.com/auth0/go-jwt-middleware/v2/jwks"
	"github.com/auth0/go-jwt-middleware/v2/validator"
	"github.com/equinor/radix-common/net/http"
)

type TokenPrincipal interface {
	IsAuthenticated() bool
	Token() string
	Id() string
	Name() string
}

type ValidatorInterface interface {
	// ValidateToken will return a TokenPrincipal object if token payload and signature is validated agains issuer. It will return nil principal and a error if it fails.
	ValidateToken(context.Context, string) (TokenPrincipal, error)
}

type Validator struct {
	validator *validator.Validator
}

var _ ValidatorInterface = &Validator{}

type KeyFunc func(context.Context) (interface{}, error)

func NewValidator(issuerUrl url.URL, audience string) (*Validator, error) {
	provider := jwks.NewCachingProvider(&issuerUrl, 5*time.Hour)

	validator, err := validator.New(
		provider.KeyFunc,
		validator.RS256,
		issuerUrl.String(),
		[]string{audience},
		validator.WithCustomClaims(func() validator.CustomClaims {
			return &azureClaims{}
		}),
	)
	if err != nil {
		return nil, err
	}

	return &Validator{validator: validator}, nil
}

func (v *Validator) ValidateToken(ctx context.Context, token string) (TokenPrincipal, error) {
	validateToken, err := v.validator.ValidateToken(ctx, token)
	if err != nil {
		return nil, err
	}

	claims, ok := validateToken.(*validator.ValidatedClaims)
	if !ok {
		return nil, http.ForbiddenError("invalid token")
	}

	azClaims, ok := claims.CustomClaims.(*azureClaims)
	if !ok || azClaims == nil {
		return nil, http.ForbiddenError("invalid azure token")
	}

	principal := &azurePrincipal{token: token, claims: claims.RegisteredClaims, azureClaims: *azClaims}
	return principal, nil
}

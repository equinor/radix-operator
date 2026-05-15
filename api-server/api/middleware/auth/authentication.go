package auth

import (
	"context"
	"net/http"

	"github.com/equinor/radix-common/models"
	radixhttp "github.com/equinor/radix-common/net/http"
	"github.com/equinor/radix-operator/api-server/api/utils/token"
	"github.com/rs/zerolog/log"
	"github.com/urfave/negroni/v3"
)

type ctxUserKey struct{}
type ctxImpersonationKey struct{}

func NewAuthenticationMiddleware(validator token.ValidatorInterface) negroni.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		ctx := r.Context()
		logger := log.Ctx(ctx)
		if r.Header.Get("authorization") == "" {
			next(w, r)
			return
		}

		token, err := radixhttp.GetBearerTokenFromHeader(r)
		if err != nil {
			logger.Warn().Err(err).Msg("authentication error")
			if err = radixhttp.ErrorResponse(w, r, err); err != nil {
				logger.Err(err).Msg("failed to write response")
			}
			return
		}
		principal, err := validator.ValidateToken(ctx, token)
		if err != nil {
			logger.Warn().Err(err).Msg("authentication error")
			if err = radixhttp.ErrorResponse(w, r, err); err != nil {
				logger.Err(err).Msg("failed to write response")
			}
			return
		}

		impersonation, err := radixhttp.GetImpersonationFromHeader(r)
		if err != nil {
			logger.Warn().Err(err).Msg("authorization error")
			if err = radixhttp.ErrorResponse(w, r, radixhttp.UnexpectedError("Problems impersonating", err)); err != nil {
				logger.Err(err).Msg("failed to write response")
			}
			return
		}

		ctx = context.WithValue(ctx, ctxUserKey{}, principal)
		ctx = context.WithValue(ctx, ctxImpersonationKey{}, impersonation)
		r = r.WithContext(ctx)

		next(w, r)
	}
}

func CtxTokenPrincipal(ctx context.Context) token.TokenPrincipal {
	val, ok := ctx.Value(ctxUserKey{}).(token.TokenPrincipal)

	if !ok {
		return &anonPrincipal{}
	}

	return val
}

func CtxImpersonation(ctx context.Context) models.Impersonation {
	if val, ok := ctx.Value(ctxImpersonationKey{}).(models.Impersonation); ok {
		return val
	}

	return models.Impersonation{}
}

func GetOriginator(ctx context.Context) string {
	impersonation := CtxImpersonation(ctx)
	principal := CtxTokenPrincipal(ctx)

	if impersonation.PerformImpersonation() {
		return impersonation.User
	}

	return principal.Name()
}

func NewZerologAuthenticationDetailsMiddleware() negroni.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		ctx := r.Context()
		user := CtxTokenPrincipal(ctx)
		impersonation := CtxImpersonation(ctx)

		logContext := log.Ctx(ctx).With()
		if user.IsAuthenticated() {
			logContext = logContext.Str("user_id", user.Id())
		} else {
			logContext = logContext.Bool("anonymous", true)
		}
		if impersonation.PerformImpersonation() {
			logContext = logContext.Str("impersonate_user", impersonation.User).Strs("impersonate_groups", impersonation.Groups)
		}
		ctx = logContext.Logger().WithContext(ctx)

		r = r.WithContext(ctx)
		next(w, r)
	}
}

func NewAuthorizeRequiredMiddleware() negroni.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		logger := log.Ctx(r.Context())
		user := CtxTokenPrincipal(r.Context())

		if !user.IsAuthenticated() {
			logger.Warn().Msg("authorization error")
			if err := radixhttp.ErrorResponse(w, r, radixhttp.ForbiddenError("Authorization is required")); err != nil {
				logger.Err(err).Msg("failed to write response")
			}
			return
		}

		next(w, r)
	}
}

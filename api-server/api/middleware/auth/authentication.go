package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/api-server/api/utils/token"
	"github.com/equinor/radix-operator/api-server/internal/accounts"
	radixhttp "github.com/equinor/radix-operator/api-server/internal/http"
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

		token, err := getBearerTokenFromHeader(r)
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

		impersonation, err := getImpersonationFromHeader(r)
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

func getBearerTokenFromHeader(r *http.Request) (string, error) {
	authorizationHeader := r.Header.Get("authorization")
	authArr := strings.Split(authorizationHeader, " ")
	var jwtToken string

	if len(authArr) != 2 {
		return "", errors.New("Authentication header is invalid: " + authorizationHeader)
	}

	jwtToken = authArr[1]
	return jwtToken, nil
}

// GetImpersonationFromHeader Gets Impersonation from request header
func getImpersonationFromHeader(r *http.Request) (accounts.Impersonation, error) {
	impersonateUser := r.Header.Get("Impersonate-User")
	var impersonateGroups []string
	if impersonateGroupHeader := strings.TrimSpace(r.Header.Get("Impersonate-Group")); len(impersonateGroupHeader) > 0 {
		impersonateGroups = slice.Map(strings.Split(impersonateGroupHeader, ","), func(group string) string { return strings.TrimSpace(group) })
	}

	return accounts.NewImpersonation(impersonateUser, impersonateGroups)
}

func CtxTokenPrincipal(ctx context.Context) token.TokenPrincipal {
	val, ok := ctx.Value(ctxUserKey{}).(token.TokenPrincipal)

	if !ok {
		return &anonPrincipal{}
	}

	return val
}

func CtxImpersonation(ctx context.Context) accounts.Impersonation {
	if val, ok := ctx.Value(ctxImpersonationKey{}).(accounts.Impersonation); ok {
		return val
	}

	return accounts.Impersonation{}
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

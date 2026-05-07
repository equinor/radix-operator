package warningcollector

import (
	"context"
	"net/http"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/urfave/negroni/v3"
	"k8s.io/client-go/rest"
)

type contextKey string
type warnings []string

var warningsContextKey = contextKey("warnings")

// KubernetesWarningHandler is a custom warning handler for Kubernetes API responses.
// It implements the rest.WarningHandlerWithContext interface to handle warning headers and log them.
// This handler collects warnings in a thread-safe manner and allows retrieval of warnings from the request context
type KubernetesWarningHandler struct {
	sync.Mutex
}

var _ rest.WarningHandlerWithContext = &KubernetesWarningHandler{}

// NewKubernetesWarningHandler creates a new instance of KubernetesWarningHandler
// which implements the rest.WarningHandlerWithContext interface.
// This handler is used to collect warnings from Kubernetes API responses.
func NewKubernetesWarningHandler() *KubernetesWarningHandler {
	return &KubernetesWarningHandler{}
}

func (h *KubernetesWarningHandler) HandleWarningHeaderWithContext(ctx context.Context, code int, agent string, text string) {
	if text == "" {
		return
	}
	log.Ctx(ctx).Warn().Str("warning", text).Int("code", code).Str("agent", agent).Msg("Warning header encountered")

	ctxValue := ctx.Value(warningsContextKey)
	if wrns, ok := ctxValue.(*warnings); ok {
		h.Lock()
		defer h.Unlock()
		*wrns = append(*wrns, text)
	} else {
		log.Ctx(ctx).Error().Msg("No warnings context found, unable to collect warning. Make sure NewWarningCollectorMiddleware is configured!")
	}

}

// NewWarningCollectorMiddleware creates a middleware that collects warnings from requests
// and stores them in the request context for later retrieval.
func NewWarningCollectorMiddleware() negroni.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		r = r.WithContext(WithWarningCollectionToContext(r.Context()))

		next(w, r)
	}
}

// WithWarningCollectionToContext adds a new warnings collection to the context.
func WithWarningCollectionToContext(ctx context.Context) context.Context {
	var wrns warnings
	return context.WithValue(ctx, warningsContextKey, &wrns)
}

// GetWarningCollectionFromContext retrieves the collection of warnings from the context.
// If no warnings are found, it returns an empty slice.
func GetWarningCollectionFromContext(ctx context.Context) []string {
	ctxValue := ctx.Value(warningsContextKey)
	if warnings, ok := ctxValue.(*warnings); ok {
		return *warnings
	}
	return nil
}

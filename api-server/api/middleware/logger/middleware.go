package logger

import (
	"net"
	"net/http"

	"github.com/felixge/httpsnoop"
	"github.com/rs/xid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/negroni/v3"
)

func NewZerologResponseLoggerMiddleware() negroni.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		m := httpsnoop.CaptureMetrics(next, w, r)

		logger := zerolog.Ctx(r.Context())

		var ev *zerolog.Event
		switch {
		case m.Code >= 400 && m.Code <= 499:
			ev = logger.Warn() //nolint:zerologlint // Msg for ev is called later
		case m.Code >= 500:
			ev = logger.Error() //nolint:zerologlint // Msg for ev is called later
		default:
			ev = logger.Info() //nolint:zerologlint // Msg for ev is called later
		}

		ev.
			Int("status", m.Code).
			Int64("body_size", m.Written).
			Int64("elapsed_ms", m.Duration.Milliseconds()).
			Msg(http.StatusText(m.Code))
	}
}

func NewZerologRequestIdMiddleware() negroni.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		logger := log.Ctx(r.Context()).With().Str("request_id", xid.New().String()).Logger()
		r = r.WithContext(logger.WithContext(r.Context()))

		next(w, r)
	}
}
func NewZerologRequestDetailsMiddleware() negroni.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		remoteIp, _, _ := net.SplitHostPort(r.RemoteAddr)
		logger := log.Ctx(r.Context()).With().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_addr", remoteIp).
			Str("referer", r.Referer()).
			Str("query", r.URL.RawQuery).
			Str("user_agent", r.UserAgent()).
			Logger()
		r = r.WithContext(logger.WithContext(r.Context()))

		next(w, r)
	}
}

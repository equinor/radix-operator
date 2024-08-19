package http

import (
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// LogRequests is a middleware that wraps the provided
// http.RoundTripper to observe the request duration
func LogRequests(t http.RoundTripper) http.RoundTripper {
	return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		logger := log.Ctx(r.Context()).With().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Logger()
		start := time.Now()
		resp, err := t.RoundTrip(r)
		elapsedMs := time.Since(start).Milliseconds()
		logger.Trace().Err(err).Int64("elapsed_ms", elapsedMs).Int("status", resp.StatusCode).Msg(http.StatusText(resp.StatusCode))
		return resp, err
	})
}

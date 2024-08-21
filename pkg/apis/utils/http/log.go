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
		ev := log.Ctx(r.Context()).Trace().Str("method", r.Method)
		if r.URL != nil {
			ev = ev.Str("path", r.URL.Path)
		}
		start := time.Now()
		resp, err := t.RoundTrip(r)
		ev = ev.Int64("elapsed_ms", time.Since(start).Milliseconds())
		if err == nil {
			ev.Int("status", resp.StatusCode).Msg(http.StatusText(resp.StatusCode))
		} else {
			ev.Err(err).Send()
		}
		return resp, err
	})
}

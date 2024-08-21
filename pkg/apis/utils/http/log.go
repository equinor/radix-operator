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
		var msg string
		if err == nil {
			msg = http.StatusText(resp.StatusCode)
			ev = ev.Int("status", resp.StatusCode)
		} else {
			ev = ev.Err(err)
		}
		ev.Msg(msg)
		return resp, err
	})
}

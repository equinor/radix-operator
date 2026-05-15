package logs

import (
	"net/http"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// RoundTripperFunc implements http.RoundTripper for convenient usage.
type RoundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip satisfies http.RoundTripper and calls fn.
func (fn RoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type WithFunc func(e *zerolog.Event)

// NewRoundtripLogger returns a http.RoundTripper that logs failed requests, and add traces for successfull requests
//
// nolint Zerolog complains about potential unsent event, but we send the event on the end of the function
func NewRoundtripLogger(fns ...WithFunc) func(t http.RoundTripper) http.RoundTripper {
	return func(t http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			logger := log.Ctx(r.Context()).With().
				Str("method", r.Method).
				Stringer("path", r.URL).
				Logger()

			start := time.Now()
			resp, err := t.RoundTrip(r)
			elapsedMs := time.Since(start).Milliseconds()

			if err != nil {
				errEvent := logger.Error().Err(err)
				for _, fn := range fns {
					errEvent.Func(fn)
				}
				errEvent.
					Int64("elapsed_ms", elapsedMs).
					Msg("Failed to send request")

				return resp, err
			}

			var ev *zerolog.Event
			switch {
			case resp.StatusCode >= 400 && resp.StatusCode <= 499:
				ev = logger.Warn()
			case resp.StatusCode >= 500:
				ev = logger.Error()
			default:
				ev = logger.Trace()
			}

			for _, fn := range fns {
				ev.Func(fn)
			}
			ev.Int64("elapsed_ms", elapsedMs).Int("status", resp.StatusCode).Msg(http.StatusText(resp.StatusCode))
			return resp, err
		})
	}
}

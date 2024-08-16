package http

import "net/http"

// RoundTripperFunc is a type that implements the RoundTripper interface
type RoundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip implements the RoundTripper interface
func (fn RoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

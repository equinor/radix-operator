package models

import "net/http"

// RadixHandlerFunc Pattern for handler functions
type RadixHandlerFunc func(http.ResponseWriter, *http.Request)

// Routes Holder of all routes
type Routes []Route

// Route Describe route
type Route struct {
	Path        string
	Method      string
	HandlerFunc RadixHandlerFunc
}

package controller

import (
	"net/http"

	"github.com/equinor/radix-operator/api-server/internal/accounts"
)

// KubeApiConfig configuration for K8s API REST client
type KubeApiConfig struct {
	QPS   float32
	Burst int
}

// Routes Holder of all routes
type Routes []Route

// RadixHandlerFunc Pattern for handler functions
type RadixHandlerFunc func(accounts.Accounts, http.ResponseWriter, *http.Request)

// Route Describe route
type Route struct {
	Path                      string
	Method                    string
	HandlerFunc               RadixHandlerFunc
	AllowUnauthenticatedUsers bool
	KubeApiConfig             KubeApiConfig
}

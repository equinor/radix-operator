package models

// KubeApiConfig configuration for K8s API REST client
type KubeApiConfig struct {
	QPS   float32
	Burst int
}

// Routes Holder of all routes
type Routes []Route

// Route Describe route
type Route struct {
	Path                      string
	Method                    string
	HandlerFunc               RadixHandlerFunc
	AllowUnauthenticatedUsers bool
	KubeApiConfig             KubeApiConfig
}

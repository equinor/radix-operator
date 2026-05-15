package router

import (
	"net/http"

	"github.com/equinor/radix-operator/api-server/api/middleware/logger"
	"github.com/equinor/radix-operator/api-server/api/middleware/recovery"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/urfave/negroni/v3"
)

// NewMetricsHandler Constructor function
func NewMetricsHandler() http.Handler {
	serveMux := http.NewServeMux()
	serveMux.Handle("GET /metrics", promhttp.Handler())

	n := negroni.New(
		recovery.NewMiddleware(),
		logger.NewZerologRequestIdMiddleware(),
		logger.NewZerologRequestDetailsMiddleware(),
		logger.NewZerologResponseLoggerMiddleware(),
	)
	n.UseHandler(serveMux)

	return n
}

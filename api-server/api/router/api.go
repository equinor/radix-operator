package router

import (
	"net/http"

	"github.com/equinor/radix-operator/api-server/api/middleware/auth"
	"github.com/equinor/radix-operator/api-server/api/middleware/logger"
	"github.com/equinor/radix-operator/api-server/api/middleware/radix"
	"github.com/equinor/radix-operator/api-server/api/middleware/recovery"
	"github.com/equinor/radix-operator/api-server/api/utils"
	"github.com/equinor/radix-operator/api-server/api/utils/token"
	"github.com/equinor/radix-operator/api-server/api/utils/warningcollector"
	"github.com/equinor/radix-operator/api-server/internal/controller"
	"github.com/equinor/radix-operator/api-server/swaggerui"
	"github.com/gorilla/mux"
	"github.com/urfave/negroni/v3"
)

const (
	apiVersionRoute = "/api/v1"
)

// NewAPIHandler Constructor function
func NewAPIHandler(validator token.ValidatorInterface, kubeUtil utils.KubeUtil, controllers ...controller.Controller) http.Handler {
	serveMux := http.NewServeMux()

	serveMux.Handle("/health/", createHealthHandler())
	serveMux.Handle("/swaggerui/", createSwaggerHandler())
	serveMux.Handle("/api/", createApiRouter(kubeUtil, controllers))

	n := negroni.New(
		recovery.NewMiddleware(),
		logger.NewZerologRequestIdMiddleware(),
		logger.NewZerologRequestDetailsMiddleware(),
		auth.NewAuthenticationMiddleware(validator),
		auth.NewZerologAuthenticationDetailsMiddleware(),
		logger.NewZerologResponseLoggerMiddleware(),
	)
	n.UseHandler(serveMux)

	return n
}
func createApiRouter(kubeUtil utils.KubeUtil, controllers []controller.Controller) *mux.Router {
	router := mux.NewRouter().StrictSlash(true)
	for _, ctrl := range controllers {
		for _, route := range ctrl.GetRoutes() {
			path := apiVersionRoute + route.Path
			handler := radix.NewRadixMiddleware(
				kubeUtil,
				path,
				route.Method,
				route.AllowUnauthenticatedUsers,
				route.KubeApiConfig.QPS,
				route.KubeApiConfig.Burst,
				route.HandlerFunc,
			)

			n := negroni.New()
			n.Use(warningcollector.NewWarningCollectorMiddleware())
			if !route.AllowUnauthenticatedUsers {
				n.Use(auth.NewAuthorizeRequiredMiddleware())
			}
			n.UseHandler(handler)
			router.Handle(path, n).Methods(route.Method)
		}
	}
	return router
}

func createSwaggerHandler() http.Handler {
	swaggerFsHandler := http.FileServer(http.FS(swaggerui.FS()))
	swaggerui := http.StripPrefix("/swaggerui", swaggerFsHandler)

	return swaggerui
}

func createHealthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

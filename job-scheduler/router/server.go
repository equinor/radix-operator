package router

import (
	"net/http"

	commongin "github.com/equinor/radix-common/pkg/gin"
	"github.com/equinor/radix-operator/job-scheduler/api/v1/controllers"
	"github.com/equinor/radix-operator/job-scheduler/models"
	"github.com/equinor/radix-operator/job-scheduler/swaggerui"
	"github.com/gin-gonic/gin"
)

const (
	apiVersionRoute = "/api/v1"
	swaggerUIPath   = "/swaggerui"
)

// NewServer creates a new Radix job scheduler REST service
func NewServer(env *models.Env, controllers ...controllers.Controller) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.RemoveExtraSlash = true
	engine.Use(commongin.SetZerologLogger(commongin.ZerologLoggerWithRequestId))
	engine.Use(commongin.ZerologRequestLogger(), gin.Recovery())

	if env.UseSwagger {
		initializeSwaggerUI(engine)
	}

	v1Router := engine.Group(apiVersionRoute)
	{
		initializeAPIServer(v1Router, controllers)
	}

	return engine
}

func initializeSwaggerUI(engine *gin.Engine) {
	swaggerFsHandler := http.FS(swaggerui.FS())
	engine.StaticFS(swaggerUIPath, swaggerFsHandler)
}

func initializeAPIServer(router gin.IRoutes, controllers []controllers.Controller) {
	for _, controller := range controllers {
		for _, route := range controller.GetRoutes() {
			addHandlerRoute(router, route)
		}
	}
}

func addHandlerRoute(router gin.IRoutes, route controllers.Route) {
	router.Handle(route.Method, route.Path, route.Handler)
}

package http

import (
	"net/http"
	"time"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/config"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/http/handlers"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/http/middleware"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain"
	gintrace "github.com/DataDog/dd-trace-go/contrib/gin-gonic/gin/v2"
	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/gin-contrib/cors"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	swagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Router struct {
	*gin.Engine
}

func NewRouter(cfg config.Config, logger *zap.Logger, userHandler handlers.UserHandler) *Router {
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = cfg.Http.AllowedOrigins
	corsConfig.AllowMethods = []string{"GET", "PUT", "DELETE", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}

	router := gin.New()
	router.Use(gintrace.Middleware(cfg.Service,
		gintrace.WithUseGinErrors(),
		gintrace.WithAnalytics(true),
		gintrace.WithIgnoreRequest(func(c *gin.Context) bool {
			return c.Request.URL.Path == "/api/health"
		}),
		gintrace.WithStatusCheck(func(statusCode int) bool {
			return statusCode >= 400
		}),
	))
	router.Use(ginzap.GinzapWithConfig(logger, &ginzap.Config{
		UTC:        true,
		TimeFormat: time.RFC3339,
		Context: ginzap.Fn(func(c *gin.Context) []zapcore.Field {
			fields := []zapcore.Field{}
			span, ok := tracer.SpanFromContext(c.Request.Context())
			if ok {
				fields = append(fields, zap.String("trace_id", span.Context().TraceID()))
				fields = append(fields, zap.String("span_id", span.String()))
			}
			return fields
		}),
	}))
	router.Use(ginzap.RecoveryWithZap(logger, true))
	router.Use(cors.New(corsConfig))

	api := router.Group("/api")

	api.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api.Static("/swagger", "./swagger")
	api.GET("/docs/*any", swagger.WrapHandler(swaggerFiles.Handler, swagger.URL("/api/swagger/swagger.yaml")))
	api.StaticFile("/redoc", "./swagger/redoc.html")

	// Rotas autenticadas (qualquer JWT válido)
	authed := api.Group("/users")
	authed.Use(middleware.AuthRequired(cfg.JWT.Secret))
	{
		// GET /users/:id — admin ou dono do recurso (verificação no handler)
		authed.GET("/:id", userHandler.GetByID)
	}

	// Rotas administrativas (role administrator obrigatória)
	admin := api.Group("/users")
	admin.Use(middleware.AuthRequired(cfg.JWT.Secret))
	admin.Use(middleware.RequireRole(domain.RoleAdministrator))
	{
		admin.GET("", userHandler.List)
		admin.PUT("/:id", userHandler.Update)
		admin.DELETE("/:id", userHandler.Delete)
	}

	return &Router{router}
}

func (r *Router) Server(listenAddr string) error {
	return r.Run(listenAddr)
}

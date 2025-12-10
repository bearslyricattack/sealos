package router

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/sealos/service/database/internal/handler"
	"github.com/labring/sealos/service/pkg/auth"
)

// Setup 设置所有路由
func Setup(r *gin.Engine, dbHandler *handler.DatabaseHandler) {
	// 公开路由
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"time":   time.Now().Unix(),
		})
	})

	r.GET("/readyz", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	r.GET("/livez", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	authenticator := auth.NewAuthenticator(5 * time.Minute)
	middlewareConfig := auth.MiddlewareConfig{
		Authenticator: authenticator,
	}
	middleware := auth.Middleware(middlewareConfig)
	api := r.Group("")
	api.Use(middleware)
	{
		api.GET("/q", dbHandler.HandleQuery)
		api.GET("/query", dbHandler.HandleQuery)
	}
}

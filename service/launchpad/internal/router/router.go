package router

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/sealos/service/launchpad/internal/handler"
	"github.com/labring/sealos/service/pkg/auth"
)

// Setup 设置所有路由
func Setup(r *gin.Engine, launchpadHandler *handler.LaunchpadHandler) {
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
		api.GET("/query", launchpadHandler.HandleQuery)
	}
}

package router

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/sealos/service/database/internal/handler"
	"github.com/labring/sealos/service/database/internal/service"
	"github.com/labring/sealos/service/pkg/auth"
	pkgHandler "github.com/labring/sealos/service/pkg/handler"
)

// SetupRoutes configures all routes for the database service
func SetupRoutes(server *pkgHandler.Server, dbService *service.DatabaseService) {
	// Create handler
	dbHandler := handler.NewDatabaseHandler(dbService)

	// Register public routes (no authentication)
	registerPublicRoutes(server)

	// Register protected API routes (with authentication)
	registerProtectedRoutes(server, dbHandler)
}

// registerPublicRoutes registers health check endpoints
func registerPublicRoutes(server *pkgHandler.Server) {
	router := server.Router()

	// Health check
	router.GET("/health", healthHandler)

	// Readiness check
	router.GET("/readyz", readyzHandler)

	// Liveness check
	router.GET("/livez", livezHandler)
}

// registerProtectedRoutes registers API endpoints with authentication
func registerProtectedRoutes(server *pkgHandler.Server, dbHandler *handler.DatabaseHandler) {
	router := server.Router()

	// Create authenticator
	authenticator := auth.NewAuthenticator(5 * time.Minute)

	// Create auth middleware
	authMiddleware := auth.Middleware(auth.MiddlewareConfig{
		Authenticator: authenticator,
		Extractor:     &auth.DefaultExtractor{},
	})

	// Create protected API group
	api := router.Group("")
	api.Use(authMiddleware) // Apply authentication middleware
	{
		// Main query endpoint (new API)
		api.POST("/q", wrapHandler(dbHandler.HandleQuery))

		// Legacy query endpoint (deprecated but maintained for compatibility)
		api.POST("/query", wrapHandler(dbHandler.HandleQuery))
	}
}

// wrapHandler wraps the custom handler function to work with Gin
func wrapHandler(handlerFunc func(*gin.Context) (interface{}, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Call the handler
		result, err := handlerFunc(c)
		if err != nil {
			c.JSON(500, gin.H{
				"error": err.Error(),
			})
			return
		}

		// Return result as JSON
		// If result is already []byte (from metrics client), return as-is
		if data, ok := result.([]byte); ok {
			c.Data(200, "application/json", data)
			return
		}

		// Otherwise, marshal to JSON
		c.JSON(200, result)
	}
}

// Health check handlers

func healthHandler(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":  "healthy",
		"service": "database",
	})
}

func readyzHandler(c *gin.Context) {
	c.String(200, "ok")
}

func livezHandler(c *gin.Context) {
	c.String(200, "ok")
}

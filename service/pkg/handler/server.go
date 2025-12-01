// Package handler provides a unified HTTP server framework for monitoring services.
package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/sealos/service/pkg/auth"
	"github.com/labring/sealos/service/pkg/config"
)

// Server represents a high-performance HTTP server for monitoring services.
type Server struct {
	config        *config.ServerConfig
	router        *gin.Engine
	httpServer    *http.Server
	authenticator *auth.Authenticator
}

// NewServer creates a new Server with the given configuration.
func NewServer(cfg *config.ServerConfig) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Set Gin mode based on log level
	switch cfg.LogLevel {
	case "debug":
		gin.SetMode(gin.DebugMode)
	case "error":
		gin.SetMode(gin.ReleaseMode)
	default:
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Add middleware
	router.Use(gin.Recovery())
	if cfg.LogLevel == "debug" {
		router.Use(gin.Logger())
	}

	// Create authenticator with 5-minute cache TTL
	authenticator := auth.NewAuthenticator(5 * time.Minute)

	httpServer := &http.Server{
		Addr:         cfg.ListenAddress,
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.ReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeoutSeconds) * time.Second,
	}

	return &Server{
		config:        cfg,
		router:        router,
		httpServer:    httpServer,
		authenticator: authenticator,
	}, nil
}

// Router returns the underlying Gin router.
// Use this to register custom routes.
func (s *Server) Router() *gin.Engine {
	return s.router
}

// Authenticator returns the server's authenticator.
func (s *Server) Authenticator() *auth.Authenticator {
	return s.authenticator
}

// Config returns the server configuration.
func (s *Server) Config() *config.ServerConfig {
	return s.config
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	log.Printf("Starting server on %s", s.config.ListenAddress)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}
	return nil
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Shutting down server...")
	return s.httpServer.Shutdown(ctx)
}

// RegisterQueryHandler registers a query handler at the specified path.
// The handler function receives the parsed request and should return the response data and error.
func (s *Server) RegisterQueryHandler(path string, handlerFunc QueryHandlerFunc) {
	s.router.POST(path, func(c *gin.Context) {
		handleQuery(c, s.authenticator, handlerFunc)
	})
}

// QueryHandlerFunc is a function that handles a query request.
// It receives the Gin context and should return response data or an error.
type QueryHandlerFunc func(c *gin.Context) (interface{}, error)

// handleQuery is the unified query handler that handles authentication and error responses.
func handleQuery(c *gin.Context, authenticator *auth.Authenticator, handler QueryHandlerFunc) {
	// Get kubeconfig from Authorization header
	kubeconfig := c.GetHeader("Authorization")
	if kubeconfig == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "missing Authorization header",
		})
		return
	}

	// Get namespace from form or query
	namespace := c.PostForm("namespace")
	if namespace == "" {
		namespace = c.Query("namespace")
	}

	if namespace == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "namespace parameter is required",
		})
		return
	}

	// Authenticate
	if err := authenticator.Authenticate(namespace, kubeconfig); err != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"error": fmt.Sprintf("authentication failed: %v", err),
		})
		return
	}

	// Call handler
	result, err := handler(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("query failed: %v", err),
		})
		return
	}

	// Return result directly as JSON (already marshaled by metrics client)
	c.Data(http.StatusOK, "application/json", result.([]byte))
}

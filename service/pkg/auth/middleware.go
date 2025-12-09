package auth

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	ContextKeyNamespace     = "auth:namespace"
	ContextKeyKubeconfig    = "auth:kubeconfig"
	ContextKeyAuthenticated = "auth:authenticated"
)

// MiddlewareConfig configures the authentication middleware
type MiddlewareConfig struct {
	Authenticator *Authenticator
	Extractor     CredentialExtractor
	SkipPaths     []string // Paths to skip authentication
	ErrorHandler  func(*gin.Context, error)
}

// Middleware returns a Gin middleware for Kubernetes authentication
func Middleware(config MiddlewareConfig) gin.HandlerFunc {
	// Set defaults
	if config.Extractor == nil {
		config.Extractor = &DefaultExtractor{}
	}
	if config.ErrorHandler == nil {
		config.ErrorHandler = defaultErrorHandler
	}

	return func(c *gin.Context) {
		// Check if path should skip authentication
		if shouldSkip(c.Request.URL.Path, config.SkipPaths) {
			c.Next()
			return
		}

		// Extract credentials
		namespace, kubeconfig, err := config.Extractor.Extract(c)
		if err != nil {
			config.ErrorHandler(c, err)
			return
		}

		// Authenticate
		if err := config.Authenticator.Authenticate(namespace, kubeconfig); err != nil {
			log.Printf("Authentication failed for namespace %s: %v", namespace, err)
			config.ErrorHandler(c, err)
			return
		}

		// Store in context for downstream handlers
		c.Set(ContextKeyNamespace, namespace)
		c.Set(ContextKeyKubeconfig, kubeconfig)
		c.Set(ContextKeyAuthenticated, true)

		c.Next()
	}
}

// shouldSkip checks if the path should skip authentication
func shouldSkip(path string, skipPaths []string) bool {
	for _, skip := range skipPaths {
		if path == skip {
			return true
		}
	}
	return false
}

// defaultErrorHandler handles authentication errors
func defaultErrorHandler(c *gin.Context, err error) {
	var statusCode int
	var message string

	switch err {
	case ErrNilNamespace, ErrEmptyKubeconfig, ErrMissingCredentials:
		statusCode = http.StatusBadRequest
		message = "Invalid request: " + err.Error()
	case ErrNoAuth:
		statusCode = http.StatusForbidden
		message = "Access denied: insufficient permissions"
	case ErrInvalidKubeconfig:
		statusCode = http.StatusUnauthorized
		message = "Invalid authentication credentials"
	default:
		statusCode = http.StatusInternalServerError
		message = "Authentication failed"
	}

	c.JSON(statusCode, gin.H{
		"error":   message,
		"details": err.Error(),
	})
	c.Abort()
}

// Helper functions to retrieve values from context

// GetNamespace retrieves the authenticated namespace from context
func GetNamespace(c *gin.Context) (string, bool) {
	namespace, exists := c.Get(ContextKeyNamespace)
	if !exists {
		return "", false
	}
	return namespace.(string), true
}

// GetKubeconfig retrieves the kubeconfig from context
func GetKubeconfig(c *gin.Context) (string, bool) {
	kubeconfig, exists := c.Get(ContextKeyKubeconfig)
	if !exists {
		return "", false
	}
	return kubeconfig.(string), true
}

// IsAuthenticated checks if the request is authenticated
func IsAuthenticated(c *gin.Context) bool {
	authenticated, exists := c.Get(ContextKeyAuthenticated)
	if !exists {
		return false
	}
	return authenticated.(bool)
}

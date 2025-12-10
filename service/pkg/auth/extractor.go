package auth

import (
	"log"

	"github.com/gin-gonic/gin"
)

// CredentialExtractor extracts authentication credentials from requests
type CredentialExtractor interface {
	Extract(c *gin.Context) (namespace, kubeconfig string, err error)
}

// DefaultExtractor extracts credentials from form data and headers
type DefaultExtractor struct{}

func getParam(c *gin.Context, key string) string {
	if value := c.Query(key); value != "" {
		return value
	}
	return c.PostForm(key)
}

// maskString masks sensitive string for logging
func maskString(s string) string {
	if len(s) == 0 {
		return ""
	}
	if len(s) <= 10 {
		return "***"
	}
	return s[:5] + "..." + s[len(s)-5:]
}

func (e *DefaultExtractor) Extract(c *gin.Context) (string, string, error) {
	log.Printf("=== Extracting Credentials ===")
	log.Printf("Request Method: %s", c.Request.Method)
	log.Printf("Request URL: %s", c.Request.URL.String())

	// 提取 namespace
	namespace := getParam(c, "namespace")
	log.Printf("Extracted namespace: '%s'", namespace)

	// 提取 kubeconfig from Authorization header
	kubeconfig := c.GetHeader("Authorization")
	if kubeconfig != "" {
		log.Printf("Authorization header found (length: %d, preview: %s)",
			len(kubeconfig), maskString(kubeconfig))
	} else {
		log.Printf("Authorization header not found")
	}

	// 验证 namespace
	if namespace == "" {
		log.Printf("❌ Validation failed: namespace is empty")
		return "", "", ErrNilNamespace
	}
	log.Printf("✓ Namespace validation passed")

	// 验证 kubeconfig
	if kubeconfig == "" {
		log.Printf("❌ Validation failed: kubeconfig is empty")
		return "", "", ErrEmptyKubeconfig
	}
	log.Printf("✓ Kubeconfig validation passed")

	log.Printf("✅ Credentials extracted successfully")
	return namespace, kubeconfig, nil
}

// JSONExtractor extracts credentials from JSON body
type JSONExtractor struct{}

type authRequest struct {
	Namespace  string `json:"namespace"`
	Kubeconfig string `json:"kubeconfig"`
}

func (e *JSONExtractor) Extract(c *gin.Context) (string, string, error) {
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return "", "", ErrMissingCredentials
	}

	if req.Namespace == "" {
		return "", "", ErrNilNamespace
	}
	if req.Kubeconfig == "" {
		return "", "", ErrEmptyKubeconfig
	}

	return req.Namespace, req.Kubeconfig, nil
}

package auth

import (
	"github.com/gin-gonic/gin"
)

// CredentialExtractor extracts authentication credentials from requests
type CredentialExtractor interface {
	Extract(c *gin.Context) (namespace, kubeconfig string, err error)
}

// DefaultExtractor extracts credentials from form data and headers
type DefaultExtractor struct{}

// Extract implements CredentialExtractor
func (e *DefaultExtractor) Extract(c *gin.Context) (string, string, error) {
	namespace := c.PostForm("namespace")
	kubeconfig := c.PostForm("kubeconfig")
	if namespace == "" {
		return "", "", ErrNilNamespace
	}
	if kubeconfig == "" {
		return "", "", ErrEmptyKubeconfig
	}

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

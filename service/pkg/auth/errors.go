package auth

import "errors"

var (
	ErrNilNamespace       = errors.New("namespace not found")
	ErrEmptyKubeconfig    = errors.New("empty kubeconfig")
	ErrInvalidKubeconfig  = errors.New("invalid kubeconfig")
	ErrAPIServerUnhealthy = errors.New("API server is not healthy")
	ErrMissingCredentials = errors.New("missing authentication credentials")
)

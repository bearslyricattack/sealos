// Package auth provides Kubernetes authentication with caching for improved performance.
package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	authorizationapi "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	ErrNilNamespace = errors.New("namespace not found")
	ErrNoAuth       = errors.New("no permission for this namespace")
	ErrNoSealosHost = errors.New("unable to get the sealos host from environment")
)

// Authenticator handles Kubernetes authentication with caching.
type Authenticator struct {
	// Cache for authentication results
	cache sync.Map // map[string]*authCacheEntry

	// Whitelist of allowed Kubernetes API hosts
	whitelistHosts []string

	// Default Kubernetes host from environment
	defaultK8sHost string

	// Cache TTL
	cacheTTL time.Duration

	// Client pool for connection reuse
	clientPool sync.Map // map[string]*kubernetes.Clientset
}

// authCacheEntry represents a cached authentication result.
type authCacheEntry struct {
	allowed   bool
	expiresAt time.Time
	mu        sync.RWMutex
}

// NewAuthenticator creates a new Authenticator with caching.
func NewAuthenticator(cacheTTL time.Duration) *Authenticator {
	whitelistEnv := os.Getenv("WHITELIST_KUBERNETES_HOSTS")
	whitelist := []string{}
	if whitelistEnv != "" {
		whitelist = strings.Split(whitelistEnv, ",")
	}

	defaultHost := getKubernetesHostFromEnv()

	log.Printf("Authenticator initialized with whitelist: %v, default host: %s", whitelist, defaultHost)

	return &Authenticator{
		whitelistHosts: whitelist,
		defaultK8sHost: defaultHost,
		cacheTTL:       cacheTTL,
	}
}

// Authenticate verifies that the kubeconfig has access to the specified namespace.
// Results are cached to improve performance.
func (a *Authenticator) Authenticate(namespace, kubeconfig string) error {
	if namespace == "" {
		return ErrNilNamespace
	}

	if kubeconfig == "" {
		return errors.New("empty kubeconfig")
	}

	// Check cache first
	cacheKey := a.getCacheKey(namespace, kubeconfig)
	if entry, ok := a.cache.Load(cacheKey); ok {
		cached := entry.(*authCacheEntry)
		cached.mu.RLock()
		defer cached.mu.RUnlock()

		if time.Now().Before(cached.expiresAt) {
			if !cached.allowed {
				return ErrNoAuth
			}
			return nil
		}
	}

	// Parse kubeconfig
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		return fmt.Errorf("invalid kubeconfig: %w", err)
	}

	// Validate and potentially override host
	if !a.isWhitelistHost(config.Host) {
		if a.defaultK8sHost != "" {
			config.Host = a.defaultK8sHost
		} else {
			return ErrNoSealosHost
		}
	}

	// Get or create Kubernetes client
	client, err := a.getClient(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Verify API server health
	if err := a.checkAPIServer(config); err != nil {
		return fmt.Errorf("API server health check failed: %w", err)
	}

	// Check resource access
	allowed, err := a.checkResourceAccess(client, namespace)
	if err != nil {
		return fmt.Errorf("permission check failed: %w", err)
	}

	// Cache result
	entry := &authCacheEntry{
		allowed:   allowed,
		expiresAt: time.Now().Add(a.cacheTTL),
	}
	a.cache.Store(cacheKey, entry)

	if !allowed {
		return ErrNoAuth
	}

	return nil
}

// getCacheKey generates a cache key from namespace and kubeconfig.
func (a *Authenticator) getCacheKey(namespace, kubeconfig string) string {
	hash := sha256.Sum256([]byte(namespace + "::" + kubeconfig))
	return hex.EncodeToString(hash[:])
}

// isWhitelistHost checks if the host is in the whitelist.
func (a *Authenticator) isWhitelistHost(host string) bool {
	for _, h := range a.whitelistHosts {
		if h == host {
			return true
		}
	}
	return false
}

// getClient returns a Kubernetes client from the pool or creates a new one.
func (a *Authenticator) getClient(config *rest.Config) (*kubernetes.Clientset, error) {
	// Use host as pool key
	poolKey := config.Host

	if client, ok := a.clientPool.Load(poolKey); ok {
		return client.(*kubernetes.Clientset), nil
	}

	// Create new client
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	// Store in pool
	a.clientPool.Store(poolKey, client)
	return client, nil
}

// checkAPIServer verifies that the Kubernetes API server is healthy.
func (a *Authenticator) checkAPIServer(config *rest.Config) error {
	// Check cache for API server health
	healthKey := "apiserver::" + config.Host
	if entry, ok := a.cache.Load(healthKey); ok {
		cached := entry.(*authCacheEntry)
		cached.mu.RLock()
		defer cached.mu.RUnlock()

		if time.Now().Before(cached.expiresAt) {
			if !cached.allowed {
				return errors.New("API server is not healthy (cached)")
			}
			return nil
		}
	}

	// Check API server /readyz endpoint
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create discovery client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := discoveryClient.RESTClient().Get().AbsPath("/readyz").DoRaw(ctx)
	if err != nil {
		return fmt.Errorf("failed to ping API server: %w", err)
	}

	healthy := string(res) == "ok"

	// Cache health check result (shorter TTL)
	entry := &authCacheEntry{
		allowed:   healthy,
		expiresAt: time.Now().Add(30 * time.Second),
	}
	a.cache.Store(healthKey, entry)

	if !healthy {
		return fmt.Errorf("API server returned unhealthy status: %s", string(res))
	}

	return nil
}

// checkResourceAccess checks if the client has access to the specified namespace.
func (a *Authenticator) checkResourceAccess(client *kubernetes.Clientset, namespace string) (bool, error) {
	// Perform SelfSubjectAccessReview (equivalent to kubectl auth can-i)
	review := &authorizationapi.SelfSubjectAccessReview{
		Spec: authorizationapi.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationapi.ResourceAttributes{
				Namespace: namespace,
				Verb:      "get",
				Group:     "",
				Version:   "v1",
				Resource:  "pods",
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, review, metav1.CreateOptions{})
	if err != nil {
		return false, err
	}

	return resp.Status.Allowed, nil
}

// ClearCache clears all cached authentication results.
func (a *Authenticator) ClearCache() {
	a.cache = sync.Map{}
}

// GetKubeconfigHost extracts the host from a kubeconfig.
func GetKubeconfigHost(kubeconfig string) (string, error) {
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		return "", fmt.Errorf("invalid kubeconfig: %w", err)
	}
	return config.Host, nil
}

// GetKubeconfigUser extracts the username from a kubeconfig.
func GetKubeconfigUser(kubeconfig string) (string, error) {
	config, err := clientcmd.Load([]byte(kubeconfig))
	if err != nil {
		return "", fmt.Errorf("invalid kubeconfig: %w", err)
	}
	for user := range config.AuthInfos {
		return user, nil
	}
	return "", errors.New("no user found in kubeconfig")
}

// getKubernetesHostFromEnv constructs the Kubernetes API host from environment variables.
func getKubernetesHostFromEnv() string {
	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	port := os.Getenv("KUBERNETES_SERVICE_PORT")

	if host == "" || port == "" {
		return ""
	}

	return "https://" + net.JoinHostPort(host, port)
}

// Global authenticator instance with default settings
var defaultAuthenticator = NewAuthenticator(5 * time.Minute)

// Authenticate uses the global authenticator instance.
// This is provided for backward compatibility.
func Authenticate(namespace, kubeconfig string) error {
	return defaultAuthenticator.Authenticate(namespace, kubeconfig)
}

// SetCacheTTL updates the cache TTL for the global authenticator.
func SetCacheTTL(ttl time.Duration) {
	defaultAuthenticator.cacheTTL = ttl
}

// ClearCache clears the global authentication cache.
func ClearCache() {
	defaultAuthenticator.ClearCache()
}

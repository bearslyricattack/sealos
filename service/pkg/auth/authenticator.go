package auth

import (
	"context"
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

// Authenticator handles Kubernetes authentication with caching
type Authenticator struct {
	cache          *Cache
	whitelistHosts []string
	defaultK8sHost string
	clientPool     sync.Map
}

// NewAuthenticator creates a new Authenticator
func NewAuthenticator(cacheTTL time.Duration) *Authenticator {
	whitelistEnv := os.Getenv("WHITELIST_KUBERNETES_HOSTS")
	whitelist := []string{}
	if whitelistEnv != "" {
		whitelist = strings.Split(whitelistEnv, ",")
	}

	defaultHost := getKubernetesHostFromEnv()

	log.Printf("Authenticator initialized with whitelist: %v, default host: %s", whitelist, defaultHost)

	return &Authenticator{
		cache:          NewCache(cacheTTL),
		whitelistHosts: whitelist,
		defaultK8sHost: defaultHost,
	}
}

// Authenticate verifies namespace access
func (a *Authenticator) Authenticate(namespace, kubeconfig string) error {
	if namespace == "" {
		return ErrNilNamespace
	}
	if kubeconfig == "" {
		return ErrEmptyKubeconfig
	}

	// Check cache
	cacheKey := GenerateKey(namespace, kubeconfig)
	if entry, ok := a.cache.Get(cacheKey); ok {
		if !entry.allowed {
			return ErrNoAuth
		}
		return nil
	}

	// Parse kubeconfig
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidKubeconfig, err)
	}

	// Validate host
	if !a.isWhitelistHost(config.Host) {
		if a.defaultK8sHost != "" {
			config.Host = a.defaultK8sHost
		} else {
			return ErrNoSealosHost
		}
	}

	// Get client
	client, err := a.getClient(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Check API server health
	if err := a.checkAPIServer(config); err != nil {
		return err
	}

	// Check permissions
	allowed, err := a.checkResourceAccess(client, namespace)
	if err != nil {
		return fmt.Errorf("permission check failed: %w", err)
	}

	// Cache result
	a.cache.Set(cacheKey, allowed)

	if !allowed {
		return ErrNoAuth
	}

	return nil
}

// isWhitelistHost checks if host is whitelisted
func (a *Authenticator) isWhitelistHost(host string) bool {
	for _, h := range a.whitelistHosts {
		if h == host {
			return true
		}
	}
	return false
}

// getClient returns a client from pool or creates new one
func (a *Authenticator) getClient(config *rest.Config) (*kubernetes.Clientset, error) {
	poolKey := config.Host

	if client, ok := a.clientPool.Load(poolKey); ok {
		return client.(*kubernetes.Clientset), nil
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	a.clientPool.Store(poolKey, client)
	return client, nil
}

// checkAPIServer verifies API server health
func (a *Authenticator) checkAPIServer(config *rest.Config) error {
	healthKey := "apiserver::" + config.Host
	if entry, ok := a.cache.Get(healthKey); ok {
		if !entry.allowed {
			return ErrAPIServerUnhealthy
		}
		return nil
	}

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
	a.cache.Set(healthKey, healthy)

	if !healthy {
		return fmt.Errorf("%w: %s", ErrAPIServerUnhealthy, string(res))
	}

	return nil
}

// checkResourceAccess checks namespace access permissions
func (a *Authenticator) checkResourceAccess(client *kubernetes.Clientset, namespace string) (bool, error) {
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

// ClearCache clears all cached results
func (a *Authenticator) ClearCache() {
	a.cache.Clear()
}

// getKubernetesHostFromEnv constructs K8s API host from env
func getKubernetesHostFromEnv() string {
	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	port := os.Getenv("KUBERNETES_SERVICE_PORT")

	if host == "" || port == "" {
		return ""
	}

	return "https://" + net.JoinHostPort(host, port)
}

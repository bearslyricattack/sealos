package auth

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	authorizationapi "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var (
	ErrNilNs        = errors.New("namespace not found")
	ErrNoAuth       = errors.New("no permission for this namespace")
	ErrNoSealosHost = errors.New("unable to get the sealos host")
	ErrPVCNotFound  = errors.New("pvc not found in namespace")
	ErrNilPVCUID    = errors.New("pvc uid not provided")

	whiteListKubernetesHosts []string
)

func AddWhiteListKubernetesHosts(host string) {
	whiteListKubernetesHosts = append(whiteListKubernetesHosts, host)
}

func GetKcHost(kc string) (string, error) {
	config, err := clientcmd.Load([]byte(kc))
	if err != nil {
		return "", fmt.Errorf("kubeconfig failed  %w", err)
	}
	cluster, err := currentCluster(config)
	if err != nil {
		return "", err
	}
	return cluster.Server, nil
}

func GetKcUser(kc string) (string, error) {
	config, err := clientcmd.Load([]byte(kc))
	if err != nil {
		return "", fmt.Errorf("kubeconfig failed  %w", err)
	}
	ctx, err := currentContext(config)
	if err != nil {
		return "", err
	}
	if _, ok := config.AuthInfos[ctx.AuthInfo]; !ok {
		return "", errors.New("no user found for current context")
	}
	return ctx.AuthInfo, nil
}

func currentContext(config *clientcmdapi.Config) (*clientcmdapi.Context, error) {
	if config.CurrentContext == "" {
		return nil, errors.New("kubeconfig has no current context")
	}
	ctx, ok := config.Contexts[config.CurrentContext]
	if !ok || ctx == nil {
		return nil, errors.New("kubeconfig current context not found")
	}
	return ctx, nil
}

func currentCluster(config *clientcmdapi.Config) (*clientcmdapi.Cluster, error) {
	ctx, err := currentContext(config)
	if err != nil {
		return nil, err
	}
	cluster, ok := config.Clusters[ctx.Cluster]
	if !ok || cluster == nil {
		return nil, errors.New("kubeconfig cluster for current context not found")
	}
	return cluster, nil
}

// restConfigFromKubeConfig deliberately supports only inline credentials. In
// particular, never pass caller-controlled ExecConfig to client-go: exec
// credential plugins run when the first request is made.
func restConfigFromKubeConfig(kc string) (*rest.Config, error) {
	parsed, err := clientcmd.Load([]byte(kc))
	if err != nil {
		return nil, fmt.Errorf("kubeconfig failed: %w", err)
	}

	// Reject dangerous fields even in inactive entries so a context switch or
	// future refactor cannot make them executable/readable by client-go.
	for _, user := range parsed.AuthInfos {
		if user == nil {
			continue
		}
		if user.Exec != nil {
			return nil, errors.New("kubeconfig exec credential plugins are not allowed")
		}
		if user.AuthProvider != nil {
			return nil, errors.New("kubeconfig auth-provider plugins are not allowed")
		}
		if user.TokenFile != "" || user.ClientCertificate != "" || user.ClientKey != "" {
			return nil, errors.New("kubeconfig file-based credentials are not allowed")
		}
		if user.Username != "" || user.Password != "" || user.Impersonate != "" || user.ImpersonateUID != "" || len(user.ImpersonateGroups) != 0 || len(user.ImpersonateUserExtra) != 0 {
			return nil, errors.New("unsupported kubeconfig authentication fields")
		}
	}

	ctx, err := currentContext(parsed)
	if err != nil {
		return nil, err
	}
	user, ok := parsed.AuthInfos[ctx.AuthInfo]
	if !ok || user == nil {
		return nil, errors.New("kubeconfig user for current context not found")
	}
	cluster, ok := parsed.Clusters[ctx.Cluster]
	if !ok || cluster == nil {
		return nil, errors.New("kubeconfig cluster for current context not found")
	}
	if cluster.Server == "" {
		return nil, errors.New("kubeconfig server is empty")
	}
	if cluster.CertificateAuthority != "" {
		return nil, errors.New("kubeconfig CA file paths are not allowed")
	}
	if cluster.ProxyURL != "" {
		return nil, errors.New("kubeconfig proxy URLs are not allowed")
	}
	if cluster.InsecureSkipTLSVerify {
		return nil, errors.New("kubeconfig insecure TLS is not allowed")
	}
	if (len(user.ClientCertificateData) == 0) != (len(user.ClientKeyData) == 0) {
		return nil, errors.New("kubeconfig client certificate and key must be provided together")
	}
	if user.Token != "" && len(user.ClientCertificateData) != 0 {
		return nil, errors.New("kubeconfig must use either a token or a client certificate")
	}

	host := cluster.Server
	if !IsWhitelistKubernetesHost(host) {
		host = GetKubernetesHostFromEnv()
		if host == "" {
			return nil, ErrNoSealosHost
		}
	}

	return &rest.Config{
		Host:        host,
		BearerToken: user.Token,
		TLSClientConfig: rest.TLSClientConfig{
			CAData:   cluster.CertificateAuthorityData,
			CertData: user.ClientCertificateData,
			KeyData:  user.ClientKeyData,
		},
	}, nil
}

func CheckK8sHost(host string) error {
	if !IsWhitelistKubernetesHost(host) {
		if k8shost := GetKubernetesHostFromEnv(); k8shost != "" {
			if k8shost != host {
				return fmt.Errorf("k8s host not match, expect %s, got %s", k8shost, host)
			}
		} else {
			return ErrNoSealosHost
		}
	}
	return nil
}

func Authenticate(ns, kc string) error {
	if ns == "" {
		return ErrNilNs
	}
	config, err := restConfigFromKubeConfig(kc)
	if err != nil {
		return fmt.Errorf("kubeconfig failed: %w", err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to new client: %w", err)
	}
	discovery, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to new discovery client: %w", err)
	}
	res, err := discovery.RESTClient().Get().AbsPath("/readyz").DoRaw(context.Background())
	if err != nil {
		log.Println("Authenticate false, ping apiserver error")
		return fmt.Errorf("ping apiserver error: %w", err)
	}
	if string(res) != "ok" {
		log.Println("Authenticate false, response not ok")
		return fmt.Errorf("ping apiserver is no ok: %v", string(res))
	}

	if err := CheckResourceAccess(client, ns, "get", "pods"); err != nil {
		// fmt.Println(err.Error())
		return fmt.Errorf("check resource access error: %w", err)
	}

	return nil
}

func AuthenticatePVC(ns, kc string, pvcUIDs []string) error {
	if ns == "" {
		return ErrNilNs
	}
	if len(pvcUIDs) == 0 {
		return ErrNilPVCUID
	}
	config, err := restConfigFromKubeConfig(kc)
	if err != nil {
		return fmt.Errorf("kubeconfig failed: %w", err)
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to new client: %w", err)
	}
	pvcList, err := client.CoreV1().
		PersistentVolumeClaims(ns).
		List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list pvcs: %w", err)
	}
	pvcUIDSet := make(map[string]struct{}, len(pvcList.Items))
	for _, pvc := range pvcList.Items {
		pvcUIDSet["pvc-"+string(pvc.UID)] = struct{}{}
	}
	for _, uid := range pvcUIDs {
		if _, ok := pvcUIDSet[uid]; !ok {
			return fmt.Errorf("%w: %s", ErrPVCNotFound, uid)
		}
	}
	return nil
}

func IsWhitelistKubernetesHost(host string) bool {
	for _, h := range whiteListKubernetesHosts {
		if h == host {
			return true
		}
	}
	return false
}

func GetKubernetesHostFromEnv() string {
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if len(host) == 0 || len(port) == 0 {
		return ""
	}
	return "https://" + net.JoinHostPort(host, port)
}

func CheckResourceAccess(client *kubernetes.Clientset, namespace, verb, resource string) error {
	// same to kubectl auth can-i
	review := &authorizationapi.SelfSubjectAccessReview{
		Spec: authorizationapi.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationapi.ResourceAttributes{
				Namespace: namespace,
				Verb:      verb,
				Group:     "",
				Version:   "v1",
				Resource:  resource,
			},
		},
	}
	resp, err := client.AuthorizationV1().
		SelfSubjectAccessReviews().
		Create(context.TODO(), review, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	if !resp.Status.Allowed {
		return ErrNoAuth
	}
	return nil
}

func init() {
	whiteListKubernetesHosts = strings.Split(os.Getenv("WHITELIST_KUBERNETES_HOSTS"), ",")
	fmt.Println("WHITELIST_KUBERNETES_HOSTS", whiteListKubernetesHosts)
}

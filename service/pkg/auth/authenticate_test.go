package auth

import (
	"strings"
	"testing"
)

func TestRestConfigFromKubeConfigRejectsExecCredentials(t *testing.T) {
	kubeconfig := `apiVersion: v1
kind: Config
current-context: current
clusters:
- name: cluster
  cluster:
    server: https://kubernetes.default.svc
contexts:
- name: current
  context:
    cluster: cluster
    user: current-user
users:
- name: current-user
  user:
    exec:
      command: /bin/sh
      args: ["-c", "touch /tmp/kubeconfig-exec-ran"]
`
	_, err := restConfigFromKubeConfig(kubeconfig)
	if err == nil || !strings.Contains(err.Error(), "exec credential plugins are not allowed") {
		t.Fatalf("expected exec plugin to be rejected, got %v", err)
	}
}

func TestRestConfigFromKubeConfigRejectsFileCredentials(t *testing.T) {
	kubeconfig := `apiVersion: v1
kind: Config
current-context: current
clusters:
- name: cluster
  cluster:
    server: https://kubernetes.default.svc
contexts:
- name: current
  context:
    cluster: cluster
    user: current-user
users:
- name: current-user
  user:
    tokenFile: /etc/passwd
`
	_, err := restConfigFromKubeConfig(kubeconfig)
	if err == nil || !strings.Contains(err.Error(), "file-based credentials are not allowed") {
		t.Fatalf("expected file-based credential to be rejected, got %v", err)
	}
}

func TestRestConfigFromKubeConfigUsesInlineTokenAndConfiguredHost(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")
	kubeconfig := `apiVersion: v1
kind: Config
current-context: current
clusters:
- name: cluster
  cluster:
    server: https://attacker.example
contexts:
- name: current
  context:
    cluster: cluster
    user: current-user
users:
- name: current-user
  user:
    token: inline-token
`
	config, err := restConfigFromKubeConfig(kubeconfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Host != "https://10.0.0.1:6443" {
		t.Fatalf("unexpected host: %q", config.Host)
	}
	if config.BearerToken != "inline-token" {
		t.Fatalf("unexpected bearer token: %q", config.BearerToken)
	}
}

func TestRestConfigFromKubeConfigRejectsUnsafeClusterSettings(t *testing.T) {
	tests := []struct {
		name   string
		fields string
		want   string
	}{
		{name: "CA file", fields: "    certificate-authority: /etc/passwd\n", want: "CA file paths are not allowed"},
		{name: "proxy", fields: "    proxy-url: http://127.0.0.1:8080\n", want: "proxy URLs are not allowed"},
		{name: "insecure TLS", fields: "    insecure-skip-tls-verify: true\n", want: "insecure TLS is not allowed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kubeconfig := "apiVersion: v1\nkind: Config\ncurrent-context: current\nclusters:\n- name: cluster\n  cluster:\n    server: https://kubernetes.default.svc\n" + tt.fields + "contexts:\n- name: current\n  context:\n    cluster: cluster\n    user: current-user\nusers:\n- name: current-user\n  user:\n    token: inline-token\n"
			_, err := restConfigFromKubeConfig(kubeconfig)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q, got %v", tt.want, err)
			}
		})
	}
}

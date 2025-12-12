# Devbox Monitoring Service

Devbox monitoring service provides metrics querying capabilities for Sealos Devbox instances.

## Architecture

The service follows a clean layered architecture:

- `main.go` - Entry point, configuration loading
- `cmd/server.go` - Server startup and graceful shutdown
- `internal/handler/` - HTTP request handling (parsing, validation, response)
- `internal/service/` - Business logic layer
- `internal/query/` - Query building and execution (builder + executor)
- `internal/router/` - Route configuration

## Features

- Query devbox CPU, memory, disk, and network metrics
- Support for both instant and range queries
- Authentication middleware
- Health check endpoints
- Graceful shutdown

## API Endpoints

### Health Checks

- `GET /health` - Service health status
- `GET /readyz` - Readiness probe
- `GET /livez` - Liveness probe

### Metrics Query

- `GET /q` - Query devbox metrics
- `GET /query` - Query devbox metrics (legacy endpoint)

### Query Parameters

- `namespace` (required) - Kubernetes namespace
- `devboxName` (required) - Devbox instance name
- `type` (required) - Query type: `cpu`, `memory`, `disk`, `network_in`, `network_out`
- `podName` (optional) - Specific pod name for pod-level queries
- `start` - Start time for range queries (unix timestamp or RFC3339)
- `end` - End time for range queries
- `step` - Query resolution step width
- `time` - Evaluation timestamp for instant queries

## Configuration

The service uses a YAML configuration file (default: `/config/config.yml`):

```yaml
listenAddress: ":8080"
metricsHost: "http://victoria-metrics:8428"
```

## Development

### Build

```bash
make build
```

### Run locally

```bash
make run
```

### Run tests

```bash
make test
```

### Build Docker image

```bash
make docker-build TAG=v1.0.0
```

## Deployment

```bash
make deploy
```

This will apply the Kubernetes manifests in `deploy/manifests/`.

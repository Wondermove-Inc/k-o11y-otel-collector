# K-O11y OpenTelemetry Collector

[English](README.md) | [한국어](README.ko.md)

A custom OpenTelemetry Collector distribution with a CRD (Custom Resource Definition) processor.


Built by [Wondermove](https://wondermove.net) as part of the K-O11y stack, forked from [OpenTelemetry Collector](https://github.com/open-telemetry/opentelemetry-collector).

## Key Features

- **CRD Processor**: Automatically adds Kubernetes CRD labels (e.g., `k8s.rollout.name`) to traces, metrics, and logs
- **Argo Rollouts Support**: Built-in support for Argo Rollouts workloads
- **Extensible**: Easily add support for other CRDs (Knative, KEDA, etc.)
- **K8s Informer-based**: Efficient caching that minimizes API server load

## Components

Based on OTel Collector v0.109.0 with a custom CRD processor.

### Receivers (7)

| Receiver | Source | Description |
|----------|--------|-------------|
| `otlp` | Core | OTLP gRPC/HTTP receiver |
| `filelog` | Contrib | File log receiver |
| `hostmetrics` | Contrib | Host metrics (CPU, memory, disk, network) |
| `k8s_cluster` | Contrib | Kubernetes cluster metrics (nodes, pods, deployments) |
| `k8s_events` | Contrib | Kubernetes events receiver |
| `kubeletstats` | Contrib | Kubelet stats receiver |
| `prometheus` | Contrib | Prometheus scraping receiver |

### Processors (10)

| Processor | Source | Description |
|-----------|--------|-------------|
| `batch` | Core | Batch telemetry data |
| `memory_limiter` | Core | Memory limiter to prevent OOM |
| `attributes` | Contrib | Modify resource/span attributes |
| `filter` | Contrib | Filter telemetry data |
| `k8sattributes` | Contrib | Add Kubernetes metadata |
| `metricstransform` | Contrib | Transform metric names and labels |
| `resource` | Contrib | Modify resource attributes |
| `resourcedetection` | Contrib | Auto-detect host/cloud environment |
| `transform` | Contrib | OTTL-based data transformation |
| **`crd`** | **Custom** | **Add CRD owner labels (e.g., k8s.rollout.name)** |

### Exporters (4)

| Exporter | Source | Description |
|----------|--------|-------------|
| `otlp` | Core | OTLP gRPC exporter |
| `otlphttp` | Core | OTLP HTTP exporter |
| `debug` | Core | Console debug output |
| `clickhouse` | Contrib | ClickHouse database exporter |

### Extensions (3)

| Extension | Source | Description |
|-----------|--------|-------------|
| `zpages` | Core | zPages debugging extension |
| `health_check` | Contrib | Health check endpoint (port 13133) |
| `pprof` | Contrib | Go pprof profiling endpoint |

## Project Structure

```
k-o11y-otel-collector/
├── cmd/otelcol/
│   ├── main.go           # Entrypoint
│   └── components.go     # Component registration
├── processor/crdprocessor/
│   ├── config.go         # Configuration struct
│   ├── factory.go        # Factory functions
│   ├── processor.go      # Core processor logic
│   ├── cache.go          # K8s Informer cache
│   ├── config_test.go    # Config tests
│   ├── factory_test.go   # Factory tests
│   ├── processor_test.go # Processor tests
│   └── cache_test.go     # Cache tests
├── Makefile
├── Dockerfile
├── go.mod
└── README.md
```

## CRD Processor Configuration

```yaml
processors:
  crd:
    # ReplicaSet -> Owner mapping cache TTL
    cache_ttl: 60s

    # Maximum cache entries
    cache_max_size: 10000

    # Allow data passthrough on error
    passthrough_on_error: true

    # Supported CRD list
    custom_resources:
      - group: argoproj.io
        version: v1alpha1
        kind: Rollout
        label_prefix: k8s.rollout

      # Add other CRDs as needed
      # - group: serving.knative.dev
      #   version: v1
      #   kind: Revision
      #   label_prefix: k8s.knative.revision
```

### Pipeline Configuration

```yaml
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [k8sattributes, crd, batch]  # crd after k8sattributes
      exporters: [otlp]
    metrics:
      receivers: [otlp]
      processors: [k8sattributes, crd, batch]
      exporters: [otlp]
    logs:
      receivers: [otlp]
      processors: [k8sattributes, crd, batch]
      exporters: [otlp]
```

## Build

### Prerequisites

- Go 1.22+
- Docker (for container builds)
- kubectl (for K8s testing)

### Binary Build

```bash
# Build for current platform
make build

# Build for all platforms
make build-all
```

### Docker Image Build

```bash
# Build and push multi-arch image
make docker

# Local build
make docker-local
```

### Run Tests

```bash
# Run tests
make test

# Run tests with coverage
make test-coverage
```

**Test Status**:
- 43 unit tests
- Coverage: 72.3%

## RBAC Requirements

Kubernetes permissions required by the CRD processor:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: otel-collector-crd
rules:
  - apiGroups: ["apps"]
    resources: ["replicasets"]
    verbs: ["get", "list", "watch"]
```

## How It Works

1. **K8s Informer**: Watches ReplicaSet resources in the cluster
2. **OwnerReference Lookup**: Checks OwnerReferences of each ReplicaSet for supported CRDs
3. **Cache Build**: Stores ReplicaSet to CRD Owner mappings in an in-memory cache
4. **Label Injection**: During telemetry processing, looks up `k8s.replicaset.name` and adds CRD labels

```
Telemetry data (with k8s.replicaset.name)
    |
CRD Processor
    |
Cache lookup: ReplicaSet -> Rollout
    |
Add labels: k8s.rollout.name, k8s.rollout.uid
    |
Continue pipeline
```

## Troubleshooting

### CRD labels not appearing

1. Check that `k8s.replicaset.name` exists (requires `k8sattributes` processor first)
2. Verify RBAC permissions for ReplicaSet access
3. Check processor logs for cache sync status
4. Verify CRD Kind matches exactly (case-sensitive)

### High memory usage

Reduce cache size:
```yaml
processors:
  crd:
    cache_max_size: 5000
```

### Slow startup

The Informer needs to sync all ReplicaSets at startup. This is expected behavior in large clusters.

## Maintainers

Built and maintained by [Wondermove](https://wondermove.net).

## License

Apache 2.0 - See [LICENSE](LICENSE)

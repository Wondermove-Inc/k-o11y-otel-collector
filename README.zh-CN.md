# K-O11y OpenTelemetry Collector

[English](README.md) | [한국어](README.ko.md) | [日本語](README.ja.md) | [中文](README.zh-CN.md)

集成 CRD（Custom Resource Definition）处理器的自定义 OpenTelemetry Collector 发行版。

由 [Wondermove](https://wondermove.net) 作为 K-O11y 可观测性栈的组成部分开发，基于 [OpenTelemetry Collector](https://github.com/open-telemetry/opentelemetry-collector) 进行二次开发。

## 核心功能

- **CRD 处理器**：自动将 Kubernetes CRD 标签（如 `k8s.rollout.name`）添加到 traces、metrics 和 logs 中
- **Argo Rollouts 支持**：内置支持 Argo Rollouts 工作负载
- **可扩展性**：可轻松添加对其他 CRD（Knative、KEDA 等）的支持
- **基于 K8s Informer**：高效缓存机制，最大限度降低 API Server 负载

## 组件

基于 OTel Collector v0.109.0，集成自定义 CRD 处理器。

### Receivers（7个）

| Receiver | 来源 | 说明 |
|----------|------|------|
| `otlp` | Core | OTLP gRPC/HTTP 接收器 |
| `filelog` | Contrib | 文件日志接收器 |
| `hostmetrics` | Contrib | 主机指标接收器（CPU、内存、磁盘、网络） |
| `k8s_cluster` | Contrib | Kubernetes 集群指标（节点、Pod、Deployment） |
| `k8s_events` | Contrib | Kubernetes 事件接收器 |
| `kubeletstats` | Contrib | Kubelet 统计接收器 |
| `prometheus` | Contrib | Prometheus 抓取接收器 |

### Processors（10个）

| Processor | 来源 | 说明 |
|-----------|------|------|
| `batch` | Core | 遥测数据批处理 |
| `memory_limiter` | Core | 内存限制，防止 OOM |
| `attributes` | Contrib | 修改资源/Span 属性 |
| `filter` | Contrib | 过滤遥测数据 |
| `k8sattributes` | Contrib | 添加 Kubernetes 元数据 |
| `metricstransform` | Contrib | 转换指标名称和标签 |
| `resource` | Contrib | 修改资源属性 |
| `resourcedetection` | Contrib | 自动检测主机/云环境 |
| `transform` | Contrib | 基于 OTTL 的数据转换 |
| **`crd`** | **Custom** | **添加 CRD Owner 标签（如 k8s.rollout.name）** |

### Exporters（4个）

| Exporter | 来源 | 说明 |
|----------|------|------|
| `otlp` | Core | OTLP gRPC 导出器 |
| `otlphttp` | Core | OTLP HTTP 导出器 |
| `debug` | Core | 控制台调试输出 |
| `clickhouse` | Contrib | ClickHouse 数据库导出器 |

### Extensions（3个）

| Extension | 来源 | 说明 |
|-----------|------|------|
| `zpages` | Core | zPages 调试扩展 |
| `health_check` | Contrib | 健康检查端点（端口 13133） |
| `pprof` | Contrib | Go pprof 性能分析端点 |

## 项目结构

```
k-o11y-otel-collector/
├── cmd/otelcol/
│   ├── main.go           # 入口点
│   └── components.go     # 组件注册
├── processor/crdprocessor/
│   ├── config.go         # 配置结构体
│   ├── factory.go        # 工厂函数
│   ├── processor.go      # 核心处理器逻辑
│   ├── cache.go          # K8s Informer 缓存
│   ├── config_test.go    # 配置测试
│   ├── factory_test.go   # 工厂测试
│   ├── processor_test.go # 处理器测试
│   └── cache_test.go     # 缓存测试
├── Makefile
├── Dockerfile
├── go.mod
└── README.md
```

## CRD 处理器配置

```yaml
processors:
  crd:
    # ReplicaSet -> Owner 映射缓存的 TTL
    cache_ttl: 60s

    # 最大缓存条目数
    cache_max_size: 10000

    # 发生错误时允许数据透传
    passthrough_on_error: true

    # 支持的 CRD 列表
    custom_resources:
      - group: argoproj.io
        version: v1alpha1
        kind: Rollout
        label_prefix: k8s.rollout

      # 按需添加其他 CRD
      # - group: serving.knative.dev
      #   version: v1
      #   kind: Revision
      #   label_prefix: k8s.knative.revision
```

### Pipeline 配置

```yaml
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [k8sattributes, crd, batch]  # crd 须置于 k8sattributes 之后
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

## 构建

### 前置要求

- Go 1.22+
- Docker（容器构建）
- kubectl（K8s 测试）

### 二进制构建

```bash
# 构建当前平台
make build

# 构建全平台
make build-all
```

### Docker 镜像构建

```bash
# 构建并推送多架构镜像
make docker

# 本地构建
make docker-local
```

### 运行测试

```bash
# 运行测试
make test

# 运行测试并生成覆盖率报告
make test-coverage
```

**测试状态**：
- 单元测试：43个
- 覆盖率：72.3%

## RBAC 权限要求

CRD 处理器所需的 Kubernetes 权限如下：

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

## 工作原理

1. **K8s Informer**：监听集群中的 ReplicaSet 资源
2. **OwnerReference 查找**：检查每个 ReplicaSet 的 OwnerReferences，识别支持的 CRD
3. **缓存构建**：将 ReplicaSet → CRD Owner 的映射关系存储到内存缓存中
4. **标签注入**：处理遥测数据时，根据 `k8s.replicaset.name` 查找并添加 CRD 标签

```
遥测数据（包含 k8s.replicaset.name）
    |
CRD 处理器
    |
缓存查找：ReplicaSet -> Rollout
    |
添加标签：k8s.rollout.name, k8s.rollout.uid
    |
继续 Pipeline
```

## 故障排查

### CRD 标签未出现

1. 确认 `k8s.replicaset.name` 已存在（需先运行 `k8sattributes` 处理器）
2. 验证 ReplicaSet 访问的 RBAC 权限
3. 检查处理器日志中的缓存同步状态
4. 确认 CRD Kind 完全匹配（区分大小写）

### 内存使用率过高

减小缓存大小：

```yaml
processors:
  crd:
    cache_max_size: 5000
```

### 启动速度慢

Informer 在启动时需要同步所有 ReplicaSet，这在大型集群中属于预期行为。

## 维护者

由 [Wondermove](https://wondermove.net) 开发并维护。

## 许可证

Apache 2.0 - 详见 [LICENSE](LICENSE)

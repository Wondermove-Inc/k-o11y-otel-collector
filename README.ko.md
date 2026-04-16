# K-O11y OpenTelemetry Collector

[English](README.md)

CRD(Custom Resource Definition) 프로세서를 지원하는 커스텀 OpenTelemetry Collector 배포판입니다.

## 주요 기능

- **CRD 프로세서**: Kubernetes CRD 라벨(예: `k8s.rollout.name`)을 traces, metrics, logs에 자동 추가
- **Argo Rollouts 지원**: Argo Rollouts 워크로드 기본 지원
- **확장 가능**: 다른 CRD(Knative, KEDA 등) 지원 쉽게 추가 가능
- **K8s Informer 기반**: API 서버 부하를 최소화하는 효율적인 캐싱

## 컴포넌트

OTel Collector v0.109.0 기반, 커스텀 CRD 프로세서 포함.

### Receivers (7개)

| Receiver | 소스 | 설명 |
|----------|------|------|
| `otlp` | Core | OTLP gRPC/HTTP 수신기 |
| `filelog` | Contrib | 파일 로그 수신기 |
| `hostmetrics` | Contrib | 호스트 메트릭 수신기 (CPU, 메모리, 디스크, 네트워크) |
| `k8s_cluster` | Contrib | Kubernetes 클러스터 메트릭 (노드, 파드, 디플로이먼트) |
| `k8s_events` | Contrib | Kubernetes 이벤트 수신기 |
| `kubeletstats` | Contrib | Kubelet 통계 수신기 |
| `prometheus` | Contrib | Prometheus 스크래핑 수신기 |

### Processors (10개)

| Processor | 소스 | 설명 |
|-----------|------|------|
| `batch` | Core | 텔레메트리 데이터 배치 처리 |
| `memory_limiter` | Core | OOM 방지를 위한 메모리 제한 |
| `attributes` | Contrib | 리소스/스팬 속성 수정 |
| `filter` | Contrib | 텔레메트리 데이터 필터링 |
| `k8sattributes` | Contrib | Kubernetes 메타데이터 추가 |
| `metricstransform` | Contrib | 메트릭 이름 및 라벨 변환 |
| `resource` | Contrib | 리소스 속성 수정 |
| `resourcedetection` | Contrib | 호스트/클라우드 환경 자동 감지 |
| `transform` | Contrib | OTTL 기반 데이터 변환 |
| **`crd`** | **Custom** | **CRD 소유자 라벨 추가 (예: k8s.rollout.name)** |

### Exporters (4개)

| Exporter | 소스 | 설명 |
|----------|------|------|
| `otlp` | Core | OTLP gRPC 익스포터 |
| `otlphttp` | Core | OTLP HTTP 익스포터 |
| `debug` | Core | 콘솔 디버그 출력 |
| `clickhouse` | Contrib | ClickHouse 데이터베이스 익스포터 |

### Extensions (3개)

| Extension | 소스 | 설명 |
|-----------|------|------|
| `zpages` | Core | zPages 디버깅 익스텐션 |
| `health_check` | Contrib | 헬스체크 엔드포인트 (13133 포트) |
| `pprof` | Contrib | Go pprof 프로파일링 엔드포인트 |

## 프로젝트 구조

```
k-o11y-otel-collector/
├── cmd/otelcol/
│   ├── main.go           # 엔트리포인트
│   └── components.go     # 컴포넌트 등록
├── processor/crdprocessor/
│   ├── config.go         # 설정 구조체
│   ├── factory.go        # 팩토리 함수
│   ├── processor.go      # 핵심 프로세서 로직
│   ├── cache.go          # K8s Informer 캐시
│   ├── config_test.go    # 설정 테스트
│   ├── factory_test.go   # 팩토리 테스트
│   ├── processor_test.go # 프로세서 테스트
│   └── cache_test.go     # 캐시 테스트
├── Makefile
├── Dockerfile
├── go.mod
└── README.md
```

## CRD 프로세서 설정

```yaml
processors:
  crd:
    # ReplicaSet -> Owner 매핑 캐시 TTL
    cache_ttl: 60s

    # 최대 캐시 엔트리 수
    cache_max_size: 10000

    # 오류 발생 시 데이터 통과 허용
    passthrough_on_error: true

    # 지원할 CRD 목록
    custom_resources:
      - group: argoproj.io
        version: v1alpha1
        kind: Rollout
        label_prefix: k8s.rollout

      # 필요시 다른 CRD 추가
      # - group: serving.knative.dev
      #   version: v1
      #   kind: Revision
      #   label_prefix: k8s.knative.revision
```

### 파이프라인 설정

```yaml
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [k8sattributes, crd, batch]  # crd는 k8sattributes 뒤에
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

## 빌드

### 필수 조건

- Go 1.22+
- Docker (컨테이너 빌드용)
- kubectl (K8s 테스트용)

### 바이너리 빌드

```bash
# 현재 플랫폼용 빌드
make build

# 모든 플랫폼용 빌드
make build-all
```

### Docker 이미지 빌드

```bash
# 멀티 아키텍처 이미지 빌드 및 푸시
make docker

# 로컬 빌드
make docker-local
```

### 테스트 실행

```bash
# 테스트 실행
make test

# 커버리지 포함 테스트
make test-coverage
```

**테스트 현황**:
- 총 43개 단위 테스트
- 커버리지: 72.3%

## RBAC 요구사항

CRD 프로세서에 필요한 Kubernetes 권한:

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

## 동작 원리

1. **K8s Informer**: 클러스터의 ReplicaSet 리소스 감시
2. **OwnerReference 조회**: 각 ReplicaSet의 OwnerReferences에서 지원하는 CRD 확인
3. **캐시 구축**: ReplicaSet → CRD Owner 매핑을 인메모리 캐시에 저장
4. **라벨 주입**: 텔레메트리 처리 시 `k8s.replicaset.name`을 조회하여 CRD 라벨 추가

```
텔레메트리 데이터 (k8s.replicaset.name 포함)
    ↓
CRD 프로세서
    ↓
캐시 조회: ReplicaSet → Rollout
    ↓
라벨 추가: k8s.rollout.name, k8s.rollout.uid
    ↓
파이프라인 계속
```

## 트러블슈팅

### CRD 라벨이 나타나지 않음

1. `k8s.replicaset.name`이 있는지 확인 (먼저 `k8sattributes` 프로세서 필요)
2. ReplicaSet 접근을 위한 RBAC 권한 확인
3. 캐시 동기화 상태 프로세서 로그 확인
4. CRD Kind가 정확히 일치하는지 확인 (대소문자 구분)

### 높은 메모리 사용량

캐시 크기 줄이기:
```yaml
processors:
  crd:
    cache_max_size: 5000
```

### 느린 시작

Informer가 시작 시 모든 ReplicaSet을 동기화해야 합니다. 대규모 클러스터에서는 예상되는 동작입니다.

## 관리

[Wondermove](https://wondermove.net)가 개발 및 관리합니다.

## 라이선스

Apache 2.0 - [LICENSE](LICENSE) 참조

[Wondermove](https://wondermove.net)가 K-O11y 스택의 일부로 개발했으며, [OpenTelemetry Collector](https://github.com/open-telemetry/opentelemetry-collector)를 기반으로 합니다.

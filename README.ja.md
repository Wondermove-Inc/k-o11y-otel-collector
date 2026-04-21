# K-O11y OpenTelemetry Collector

[English](README.md) | [한국어](README.ko.md) | [日本語](README.ja.md) | [中文](README.zh-CN.md)

CRD（Custom Resource Definition）プロセッサーを搭載したカスタム OpenTelemetry Collector ディストリビューションです。

[Wondermove](https://wondermove.net) が K-O11y スタックの一部として開発し、[OpenTelemetry Collector](https://github.com/open-telemetry/opentelemetry-collector) をフォークして作成しています。

## 主な機能

- **CRD プロセッサー**: Kubernetes CRD ラベル（例: `k8s.rollout.name`）を traces、metrics、logs に自動付与します
- **Argo Rollouts サポート**: Argo Rollouts ワークロードをビルトインサポートしています
- **拡張性**: 他の CRD（Knative、KEDA 等）のサポートを簡単に追加できます
- **K8s Informer ベース**: API サーバーの負荷を最小化する効率的なキャッシュを実装しています

## コンポーネント

OTel Collector v0.109.0 をベースに、カスタム CRD プロセッサーを追加しています。

### Receivers（7種）

| Receiver | ソース | 説明 |
|----------|--------|------|
| `otlp` | Core | OTLP gRPC/HTTP レシーバー |
| `filelog` | Contrib | ファイルログレシーバー |
| `hostmetrics` | Contrib | ホストメトリクスレシーバー（CPU、メモリ、ディスク、ネットワーク） |
| `k8s_cluster` | Contrib | Kubernetes クラスターメトリクス（ノード、Pod、Deployment） |
| `k8s_events` | Contrib | Kubernetes イベントレシーバー |
| `kubeletstats` | Contrib | Kubelet 統計レシーバー |
| `prometheus` | Contrib | Prometheus スクレイピングレシーバー |

### Processors（10種）

| Processor | ソース | 説明 |
|-----------|--------|------|
| `batch` | Core | テレメトリデータのバッチ処理 |
| `memory_limiter` | Core | OOM 防止のためのメモリ制限 |
| `attributes` | Contrib | リソース/スパン属性の変更 |
| `filter` | Contrib | テレメトリデータのフィルタリング |
| `k8sattributes` | Contrib | Kubernetes メタデータの付与 |
| `metricstransform` | Contrib | メトリクス名およびラベルの変換 |
| `resource` | Contrib | リソース属性の変更 |
| `resourcedetection` | Contrib | ホスト/クラウド環境の自動検出 |
| `transform` | Contrib | OTTL ベースのデータ変換 |
| **`crd`** | **Custom** | **CRD オーナーラベルの付与（例: k8s.rollout.name）** |

### Exporters（4種）

| Exporter | ソース | 説明 |
|----------|--------|------|
| `otlp` | Core | OTLP gRPC エクスポーター |
| `otlphttp` | Core | OTLP HTTP エクスポーター |
| `debug` | Core | コンソールデバッグ出力 |
| `clickhouse` | Contrib | ClickHouse データベースエクスポーター |

### Extensions（3種）

| Extension | ソース | 説明 |
|-----------|--------|------|
| `zpages` | Core | zPages デバッグ拡張 |
| `health_check` | Contrib | ヘルスチェックエンドポイント（ポート 13133） |
| `pprof` | Contrib | Go pprof プロファイリングエンドポイント |

## プロジェクト構成

```
k-o11y-otel-collector/
├── cmd/otelcol/
│   ├── main.go           # エントリーポイント
│   └── components.go     # コンポーネント登録
├── processor/crdprocessor/
│   ├── config.go         # 設定構造体
│   ├── factory.go        # ファクトリー関数
│   ├── processor.go      # コアプロセッサーロジック
│   ├── cache.go          # K8s Informer キャッシュ
│   ├── config_test.go    # 設定テスト
│   ├── factory_test.go   # ファクトリーテスト
│   ├── processor_test.go # プロセッサーテスト
│   └── cache_test.go     # キャッシュテスト
├── Makefile
├── Dockerfile
├── go.mod
└── README.md
```

## CRD プロセッサーの設定

```yaml
processors:
  crd:
    # ReplicaSet -> Owner マッピングキャッシュの TTL
    cache_ttl: 60s

    # キャッシュエントリーの最大数
    cache_max_size: 10000

    # エラー発生時のデータ通過を許可
    passthrough_on_error: true

    # サポートする CRD リスト
    custom_resources:
      - group: argoproj.io
        version: v1alpha1
        kind: Rollout
        label_prefix: k8s.rollout

      # 必要に応じて他の CRD を追加
      # - group: serving.knative.dev
      #   version: v1
      #   kind: Revision
      #   label_prefix: k8s.knative.revision
```

### パイプライン設定

```yaml
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [k8sattributes, crd, batch]  # crd は k8sattributes の後に配置
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

## ビルド

### 前提条件

- Go 1.22+
- Docker（コンテナビルド用）
- kubectl（K8s テスト用）

### バイナリビルド

```bash
# 現在のプラットフォーム向けビルド
make build

# 全プラットフォーム向けビルド
make build-all
```

### Docker イメージビルド

```bash
# マルチアーキテクチャイメージのビルドとプッシュ
make docker

# ローカルビルド
make docker-local
```

### テスト実行

```bash
# テストの実行
make test

# カバレッジ付きテスト
make test-coverage
```

**テスト状況**:
- ユニットテスト: 43件
- カバレッジ: 72.3%

## RBAC 要件

CRD プロセッサーが必要とする Kubernetes 権限は以下の通りです。

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

## 動作原理

1. **K8s Informer**: クラスター内の ReplicaSet リソースを監視します
2. **OwnerReference 検索**: 各 ReplicaSet の OwnerReferences からサポート対象の CRD を確認します
3. **キャッシュ構築**: ReplicaSet → CRD Owner のマッピングをインメモリキャッシュに保存します
4. **ラベル注入**: テレメトリ処理時に `k8s.replicaset.name` を検索し、CRD ラベルを付与します

```
テレメトリデータ（k8s.replicaset.name を含む）
    |
CRD プロセッサー
    |
キャッシュ検索: ReplicaSet -> Rollout
    |
ラベル付与: k8s.rollout.name, k8s.rollout.uid
    |
パイプライン継続
```

## トラブルシューティング

### CRD ラベルが表示されない場合

1. `k8s.replicaset.name` が存在するか確認してください（事前に `k8sattributes` プロセッサーが必要です）
2. ReplicaSet アクセスに必要な RBAC 権限を確認してください
3. キャッシュ同期状態についてプロセッサーのログを確認してください
4. CRD Kind が完全に一致しているか確認してください（大文字・小文字を区別します）

### メモリ使用量が多い場合

キャッシュサイズを削減してください。

```yaml
processors:
  crd:
    cache_max_size: 5000
```

### 起動が遅い場合

Informer は起動時にすべての ReplicaSet を同期する必要があります。大規模クラスターでは想定される動作です。

## メンテナー

[Wondermove](https://wondermove.net) が開発・管理しています。

## ライセンス

Apache 2.0 - [LICENSE](LICENSE) を参照してください。

// Copyright 2024 Wondermove Inc.
// SPDX-License-Identifier: Apache-2.0

package crdprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap/zaptest"
)

// mockCache is a simple mock implementation of cache for testing
type mockCache struct {
	data map[string]*OwnerInfo
}

func newMockCache() *mockCache {
	return &mockCache{
		data: make(map[string]*OwnerInfo),
	}
}

func (m *mockCache) Get(namespace, rsName string) (*OwnerInfo, bool) {
	key := namespace + "/" + rsName
	owner, ok := m.data[key]
	return owner, ok
}

func (m *mockCache) Set(namespace, rsName string, owner *OwnerInfo) {
	key := namespace + "/" + rsName
	m.data[key] = owner
}

// createTestProcessor creates a processor with mock cache for testing
func createTestProcessor(t *testing.T, cfg *Config) (*crdProcessor, *mockCache, *consumertest.TracesSink, *consumertest.MetricsSink, *consumertest.LogsSink) {
	logger := zaptest.NewLogger(t)

	tracesSink := new(consumertest.TracesSink)
	metricsSink := new(consumertest.MetricsSink)
	logsSink := new(consumertest.LogsSink)

	// Build CRD prefix map
	crdPrefixMap := make(map[string]string)
	for _, crd := range cfg.CustomResources {
		crdPrefixMap[crd.Kind] = crd.LabelPrefix
	}

	mockC := newMockCache()

	p := &crdProcessor{
		logger:          logger,
		config:          cfg,
		tracesConsumer:  tracesSink,
		metricsConsumer: metricsSink,
		logsConsumer:    logsSink,
		crdPrefixMap:    crdPrefixMap,
		cache: &InformerCache{
			logger:   logger,
			ownerMap: mockC.data,
		},
	}

	return p, mockC, tracesSink, metricsSink, logsSink
}

func TestProcessor_Capabilities(t *testing.T) {
	cfg := &Config{
		CacheTTL:           60 * time.Second,
		CacheMaxSize:       10000,
		PassthroughOnError: true,
		CustomResources: []CRDConfig{
			{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
		},
	}

	p, _, _, _, _ := createTestProcessor(t, cfg)

	caps := p.Capabilities()
	assert.True(t, caps.MutatesData, "processor should mutate data")
}

func TestProcessor_ConsumeTraces_WithCRDLabels(t *testing.T) {
	cfg := &Config{
		CacheTTL:           60 * time.Second,
		CacheMaxSize:       10000,
		PassthroughOnError: true,
		CustomResources: []CRDConfig{
			{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
		},
	}

	p, mockC, tracesSink, _, _ := createTestProcessor(t, cfg)

	// Setup mock cache data
	mockC.Set("default", "my-app-rollout-abc123", &OwnerInfo{
		Kind:      "Rollout",
		Name:      "my-app-rollout",
		UID:       "rollout-uid-12345",
		Namespace: "default",
	})

	// Create test trace data
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	attrs := rs.Resource().Attributes()
	attrs.PutStr("k8s.namespace.name", "default")
	attrs.PutStr("k8s.replicaset.name", "my-app-rollout-abc123")

	// Process traces
	ctx := context.Background()
	err := p.ConsumeTraces(ctx, td)
	require.NoError(t, err)

	// Verify CRD labels were added
	receivedTraces := tracesSink.AllTraces()
	require.Len(t, receivedTraces, 1)

	receivedAttrs := receivedTraces[0].ResourceSpans().At(0).Resource().Attributes()

	rolloutName, ok := receivedAttrs.Get("k8s.rollout.name")
	require.True(t, ok, "k8s.rollout.name should be present")
	assert.Equal(t, "my-app-rollout", rolloutName.Str())

	rolloutUID, ok := receivedAttrs.Get("k8s.rollout.uid")
	require.True(t, ok, "k8s.rollout.uid should be present")
	assert.Equal(t, "rollout-uid-12345", rolloutUID.Str())
}

func TestProcessor_ConsumeTraces_WithoutReplicaSet(t *testing.T) {
	cfg := &Config{
		CacheTTL:           60 * time.Second,
		CacheMaxSize:       10000,
		PassthroughOnError: true,
		CustomResources: []CRDConfig{
			{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
		},
	}

	p, _, tracesSink, _, _ := createTestProcessor(t, cfg)

	// Create test trace data without replicaset
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	attrs := rs.Resource().Attributes()
	attrs.PutStr("k8s.namespace.name", "default")
	// No k8s.replicaset.name

	// Process traces
	ctx := context.Background()
	err := p.ConsumeTraces(ctx, td)
	require.NoError(t, err)

	// Verify no CRD labels were added
	receivedTraces := tracesSink.AllTraces()
	require.Len(t, receivedTraces, 1)

	receivedAttrs := receivedTraces[0].ResourceSpans().At(0).Resource().Attributes()

	_, ok := receivedAttrs.Get("k8s.rollout.name")
	assert.False(t, ok, "k8s.rollout.name should NOT be present")
}

func TestProcessor_ConsumeTraces_ReplicaSetNotInCache(t *testing.T) {
	cfg := &Config{
		CacheTTL:           60 * time.Second,
		CacheMaxSize:       10000,
		PassthroughOnError: true,
		CustomResources: []CRDConfig{
			{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
		},
	}

	p, _, tracesSink, _, _ := createTestProcessor(t, cfg)
	// Cache is empty - no mock data set

	// Create test trace data
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	attrs := rs.Resource().Attributes()
	attrs.PutStr("k8s.namespace.name", "default")
	attrs.PutStr("k8s.replicaset.name", "unknown-replicaset")

	// Process traces
	ctx := context.Background()
	err := p.ConsumeTraces(ctx, td)
	require.NoError(t, err)

	// Verify no CRD labels were added (passthrough)
	receivedTraces := tracesSink.AllTraces()
	require.Len(t, receivedTraces, 1)

	receivedAttrs := receivedTraces[0].ResourceSpans().At(0).Resource().Attributes()

	_, ok := receivedAttrs.Get("k8s.rollout.name")
	assert.False(t, ok, "k8s.rollout.name should NOT be present when not in cache")
}

func TestProcessor_ConsumeMetrics_WithCRDLabels(t *testing.T) {
	cfg := &Config{
		CacheTTL:           60 * time.Second,
		CacheMaxSize:       10000,
		PassthroughOnError: true,
		CustomResources: []CRDConfig{
			{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
		},
	}

	p, mockC, _, metricsSink, _ := createTestProcessor(t, cfg)

	// Setup mock cache data
	mockC.Set("production", "api-server-abc123", &OwnerInfo{
		Kind:      "Rollout",
		Name:      "api-server",
		UID:       "api-uid-67890",
		Namespace: "production",
	})

	// Create test metric data
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	attrs := rm.Resource().Attributes()
	attrs.PutStr("k8s.namespace.name", "production")
	attrs.PutStr("k8s.replicaset.name", "api-server-abc123")

	// Process metrics
	ctx := context.Background()
	err := p.ConsumeMetrics(ctx, md)
	require.NoError(t, err)

	// Verify CRD labels were added
	receivedMetrics := metricsSink.AllMetrics()
	require.Len(t, receivedMetrics, 1)

	receivedAttrs := receivedMetrics[0].ResourceMetrics().At(0).Resource().Attributes()

	rolloutName, ok := receivedAttrs.Get("k8s.rollout.name")
	require.True(t, ok, "k8s.rollout.name should be present")
	assert.Equal(t, "api-server", rolloutName.Str())
}

func TestProcessor_ConsumeLogs_WithCRDLabels(t *testing.T) {
	cfg := &Config{
		CacheTTL:           60 * time.Second,
		CacheMaxSize:       10000,
		PassthroughOnError: true,
		CustomResources: []CRDConfig{
			{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
		},
	}

	p, mockC, _, _, logsSink := createTestProcessor(t, cfg)

	// Setup mock cache data
	mockC.Set("staging", "frontend-xyz789", &OwnerInfo{
		Kind:      "Rollout",
		Name:      "frontend",
		UID:       "frontend-uid-11111",
		Namespace: "staging",
	})

	// Create test log data
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	attrs := rl.Resource().Attributes()
	attrs.PutStr("k8s.namespace.name", "staging")
	attrs.PutStr("k8s.replicaset.name", "frontend-xyz789")

	// Process logs
	ctx := context.Background()
	err := p.ConsumeLogs(ctx, ld)
	require.NoError(t, err)

	// Verify CRD labels were added
	receivedLogs := logsSink.AllLogs()
	require.Len(t, receivedLogs, 1)

	receivedAttrs := receivedLogs[0].ResourceLogs().At(0).Resource().Attributes()

	rolloutName, ok := receivedAttrs.Get("k8s.rollout.name")
	require.True(t, ok, "k8s.rollout.name should be present")
	assert.Equal(t, "frontend", rolloutName.Str())
}

func TestProcessor_MultipleCRDs(t *testing.T) {
	cfg := &Config{
		CacheTTL:           60 * time.Second,
		CacheMaxSize:       10000,
		PassthroughOnError: true,
		CustomResources: []CRDConfig{
			{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
			{Kind: "Revision", LabelPrefix: "k8s.knative.revision"},
		},
	}

	p, mockC, tracesSink, _, _ := createTestProcessor(t, cfg)

	// Setup mock cache data for Knative Revision
	mockC.Set("knative-serving", "hello-world-00001", &OwnerInfo{
		Kind:      "Revision",
		Name:      "hello-world-00001",
		UID:       "revision-uid-22222",
		Namespace: "knative-serving",
	})

	// Create test trace data
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	attrs := rs.Resource().Attributes()
	attrs.PutStr("k8s.namespace.name", "knative-serving")
	attrs.PutStr("k8s.replicaset.name", "hello-world-00001")

	// Process traces
	ctx := context.Background()
	err := p.ConsumeTraces(ctx, td)
	require.NoError(t, err)

	// Verify Knative labels were added
	receivedTraces := tracesSink.AllTraces()
	require.Len(t, receivedTraces, 1)

	receivedAttrs := receivedTraces[0].ResourceSpans().At(0).Resource().Attributes()

	revisionName, ok := receivedAttrs.Get("k8s.knative.revision.name")
	require.True(t, ok, "k8s.knative.revision.name should be present")
	assert.Equal(t, "hello-world-00001", revisionName.Str())

	// Verify Rollout labels were NOT added
	_, ok = receivedAttrs.Get("k8s.rollout.name")
	assert.False(t, ok, "k8s.rollout.name should NOT be present")
}

func TestProcessor_EmptyNamespace(t *testing.T) {
	cfg := &Config{
		CacheTTL:           60 * time.Second,
		CacheMaxSize:       10000,
		PassthroughOnError: true,
		CustomResources: []CRDConfig{
			{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
		},
	}

	p, mockC, tracesSink, _, _ := createTestProcessor(t, cfg)

	// Setup mock cache data
	mockC.Set("default", "my-app-abc123", &OwnerInfo{
		Kind:      "Rollout",
		Name:      "my-app",
		UID:       "uid-12345",
		Namespace: "default",
	})

	// Create test trace data with empty namespace
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	attrs := rs.Resource().Attributes()
	attrs.PutStr("k8s.namespace.name", "") // empty namespace
	attrs.PutStr("k8s.replicaset.name", "my-app-abc123")

	// Process traces
	ctx := context.Background()
	err := p.ConsumeTraces(ctx, td)
	require.NoError(t, err)

	// Verify no CRD labels were added
	receivedTraces := tracesSink.AllTraces()
	require.Len(t, receivedTraces, 1)

	receivedAttrs := receivedTraces[0].ResourceSpans().At(0).Resource().Attributes()

	_, ok := receivedAttrs.Get("k8s.rollout.name")
	assert.False(t, ok, "k8s.rollout.name should NOT be present when namespace is empty")
}

func TestProcessor_EmptyReplicaSetName(t *testing.T) {
	cfg := &Config{
		CacheTTL:           60 * time.Second,
		CacheMaxSize:       10000,
		PassthroughOnError: true,
		CustomResources: []CRDConfig{
			{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
		},
	}

	p, _, tracesSink, _, _ := createTestProcessor(t, cfg)

	// Create test trace data with empty replicaset name
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	attrs := rs.Resource().Attributes()
	attrs.PutStr("k8s.namespace.name", "default")
	attrs.PutStr("k8s.replicaset.name", "") // empty

	// Process traces
	ctx := context.Background()
	err := p.ConsumeTraces(ctx, td)
	require.NoError(t, err)

	// Verify no CRD labels were added
	receivedTraces := tracesSink.AllTraces()
	require.Len(t, receivedTraces, 1)

	receivedAttrs := receivedTraces[0].ResourceSpans().At(0).Resource().Attributes()

	_, ok := receivedAttrs.Get("k8s.rollout.name")
	assert.False(t, ok, "k8s.rollout.name should NOT be present when replicaset name is empty")
}

func TestProcessor_UnsupportedCRDKind(t *testing.T) {
	cfg := &Config{
		CacheTTL:           60 * time.Second,
		CacheMaxSize:       10000,
		PassthroughOnError: true,
		CustomResources: []CRDConfig{
			{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
			// Note: Deployment is NOT in the list
		},
	}

	p, mockC, tracesSink, _, _ := createTestProcessor(t, cfg)

	// Setup mock cache data with Deployment (unsupported)
	mockC.Set("default", "my-deployment-abc123", &OwnerInfo{
		Kind:      "Deployment", // Not in CustomResources
		Name:      "my-deployment",
		UID:       "deployment-uid-12345",
		Namespace: "default",
	})

	// Create test trace data
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	attrs := rs.Resource().Attributes()
	attrs.PutStr("k8s.namespace.name", "default")
	attrs.PutStr("k8s.replicaset.name", "my-deployment-abc123")

	// Process traces
	ctx := context.Background()
	err := p.ConsumeTraces(ctx, td)
	require.NoError(t, err)

	// Verify no CRD labels were added (Deployment not in config)
	receivedTraces := tracesSink.AllTraces()
	require.Len(t, receivedTraces, 1)

	receivedAttrs := receivedTraces[0].ResourceSpans().At(0).Resource().Attributes()

	_, ok := receivedAttrs.Get("k8s.rollout.name")
	assert.False(t, ok, "k8s.rollout.name should NOT be present")

	_, ok = receivedAttrs.Get("k8s.deployment.name")
	assert.False(t, ok, "k8s.deployment.name should NOT be present")
}

func TestNewCRDProcessor(t *testing.T) {
	cfg := &Config{
		CacheTTL:           60 * time.Second,
		CacheMaxSize:       10000,
		PassthroughOnError: true,
		CustomResources: []CRDConfig{
			{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
			{Kind: "Revision", LabelPrefix: "k8s.knative.revision"},
		},
	}

	tracesSink := new(consumertest.TracesSink)
	metricsSink := new(consumertest.MetricsSink)
	logsSink := new(consumertest.LogsSink)

	set := processortest.NewNopSettings()

	p, err := newCRDProcessor(set, cfg, tracesSink, metricsSink, logsSink)
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, cfg, p.config)
	assert.Len(t, p.crdPrefixMap, 2)
	assert.Equal(t, "k8s.rollout", p.crdPrefixMap["Rollout"])
	assert.Equal(t, "k8s.knative.revision", p.crdPrefixMap["Revision"])
}

// addCRDLabels direct test
func TestAddCRDLabels(t *testing.T) {
	cfg := &Config{
		CacheTTL:           60 * time.Second,
		CacheMaxSize:       10000,
		PassthroughOnError: true,
		CustomResources: []CRDConfig{
			{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
		},
	}

	p, mockC, _, _, _ := createTestProcessor(t, cfg)

	// Setup mock cache
	mockC.Set("default", "test-rs", &OwnerInfo{
		Kind:      "Rollout",
		Name:      "test-rollout",
		UID:       "test-uid",
		Namespace: "default",
	})

	// Create attributes
	attrs := pcommon.NewMap()
	attrs.PutStr("k8s.namespace.name", "default")
	attrs.PutStr("k8s.replicaset.name", "test-rs")

	// Call addCRDLabels directly
	p.addCRDLabels(attrs)

	// Verify labels
	name, ok := attrs.Get("k8s.rollout.name")
	require.True(t, ok)
	assert.Equal(t, "test-rollout", name.Str())

	uid, ok := attrs.Get("k8s.rollout.uid")
	require.True(t, ok)
	assert.Equal(t, "test-uid", uid.Str())
}

// TestAddCRDLabels_NilCache tests graceful handling when cache is nil
func TestAddCRDLabels_NilCache(t *testing.T) {
	cfg := &Config{
		CacheTTL:           60 * time.Second,
		CacheMaxSize:       10000,
		PassthroughOnError: true,
		CustomResources: []CRDConfig{
			{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
		},
	}

	// Create processor without initializing cache (simulating Start() failure)
	logger := zaptest.NewLogger(t)
	crdPrefixMap := make(map[string]string)
	for _, crd := range cfg.CustomResources {
		crdPrefixMap[crd.Kind] = crd.LabelPrefix
	}

	p := &crdProcessor{
		logger:       logger,
		config:       cfg,
		cache:        nil, // cache is nil
		crdPrefixMap: crdPrefixMap,
	}

	// Create attributes
	attrs := pcommon.NewMap()
	attrs.PutStr("k8s.namespace.name", "default")
	attrs.PutStr("k8s.replicaset.name", "test-rs")

	// Call addCRDLabels - should NOT panic
	assert.NotPanics(t, func() {
		p.addCRDLabels(attrs)
	})

	// Verify no CRD labels were added (passthrough)
	_, ok := attrs.Get("k8s.rollout.name")
	assert.False(t, ok, "k8s.rollout.name should NOT be present when cache is nil")
}

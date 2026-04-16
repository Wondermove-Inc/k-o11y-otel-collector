// Copyright 2024 Wondermove Inc.
// SPDX-License-Identifier: Apache-2.0

package crdprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	require.NotNil(t, factory)

	assert.Equal(t, component.MustNewType("crd"), factory.Type())
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	require.NotNil(t, cfg)

	crdCfg, ok := cfg.(*Config)
	require.True(t, ok, "config should be *Config type")

	// Verify defaults
	assert.Equal(t, 60*time.Second, crdCfg.CacheTTL)
	assert.Equal(t, 10000, crdCfg.CacheMaxSize)
	assert.Equal(t, 10*time.Second, crdCfg.APITimeout)
	assert.True(t, crdCfg.PassthroughOnError)
	assert.Len(t, crdCfg.CustomResources, 1)

	// Verify default CRD (Rollout)
	assert.Equal(t, "argoproj.io", crdCfg.CustomResources[0].Group)
	assert.Equal(t, "v1alpha1", crdCfg.CustomResources[0].Version)
	assert.Equal(t, "Rollout", crdCfg.CustomResources[0].Kind)
	assert.Equal(t, "k8s.rollout", crdCfg.CustomResources[0].LabelPrefix)
}

func TestFactory_CreateTracesProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	set := processortest.NewNopSettings()
	nextConsumer := consumertest.NewNop()

	p, err := factory.CreateTracesProcessor(context.Background(), set, cfg, nextConsumer)
	require.NoError(t, err)
	require.NotNil(t, p)

	// Verify it's the correct type
	crdProc, ok := p.(*crdProcessor)
	require.True(t, ok)
	assert.NotNil(t, crdProc.tracesConsumer)
}

func TestFactory_CreateMetricsProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	set := processortest.NewNopSettings()
	nextConsumer := consumertest.NewNop()

	p, err := factory.CreateMetricsProcessor(context.Background(), set, cfg, nextConsumer)
	require.NoError(t, err)
	require.NotNil(t, p)

	// Verify it's the correct type
	crdProc, ok := p.(*crdProcessor)
	require.True(t, ok)
	assert.NotNil(t, crdProc.metricsConsumer)
}

func TestFactory_CreateLogsProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	set := processortest.NewNopSettings()
	nextConsumer := consumertest.NewNop()

	p, err := factory.CreateLogsProcessor(context.Background(), set, cfg, nextConsumer)
	require.NoError(t, err)
	require.NotNil(t, p)

	// Verify it's the correct type
	crdProc, ok := p.(*crdProcessor)
	require.True(t, ok)
	assert.NotNil(t, crdProc.logsConsumer)
}

func TestFactory_StabilityLevel(t *testing.T) {
	factory := NewFactory()

	// TracesProcessor stability
	assert.Equal(t, component.StabilityLevelDevelopment, factory.TracesProcessorStability())

	// MetricsProcessor stability
	assert.Equal(t, component.StabilityLevelDevelopment, factory.MetricsProcessorStability())

	// LogsProcessor stability
	assert.Equal(t, component.StabilityLevelDevelopment, factory.LogsProcessorStability())
}

func TestFactory_WithCustomConfig(t *testing.T) {
	factory := NewFactory()

	cfg := &Config{
		CacheTTL:           30 * time.Second,
		CacheMaxSize:       5000,
		APITimeout:         5 * time.Second,
		PassthroughOnError: false,
		CustomResources: []CRDConfig{
			{
				Group:       "custom.example.com",
				Version:     "v1",
				Kind:        "CustomWorkload",
				LabelPrefix: "k8s.custom.workload",
			},
		},
	}

	set := processortest.NewNopSettings()
	nextConsumer := consumertest.NewNop()

	p, err := factory.CreateTracesProcessor(context.Background(), set, cfg, nextConsumer)
	require.NoError(t, err)
	require.NotNil(t, p)

	crdProc, ok := p.(*crdProcessor)
	require.True(t, ok)
	assert.Equal(t, cfg, crdProc.config)
	assert.Equal(t, "k8s.custom.workload", crdProc.crdPrefixMap["CustomWorkload"])
}

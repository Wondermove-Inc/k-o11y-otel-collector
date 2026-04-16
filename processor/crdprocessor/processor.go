// Copyright 2024 Wondermove Inc.
// SPDX-License-Identifier: Apache-2.0

package crdprocessor

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
)

// crdProcessor is the main processor struct
type crdProcessor struct {
	logger          *zap.Logger
	config          *Config
	cache           *InformerCache
	tracesConsumer  consumer.Traces
	metricsConsumer consumer.Metrics
	logsConsumer    consumer.Logs
	crdPrefixMap    map[string]string // Kind -> LabelPrefix
}

// OwnerInfo holds CRD owner information
type OwnerInfo struct {
	Kind      string
	Name      string
	UID       string
	Namespace string
}

// newCRDProcessor creates a new crdProcessor instance
func newCRDProcessor(
	set processor.Settings,
	cfg *Config,
	tracesConsumer consumer.Traces,
	metricsConsumer consumer.Metrics,
	logsConsumer consumer.Logs,
) (*crdProcessor, error) {
	// Build CRD Kind -> LabelPrefix map
	crdPrefixMap := make(map[string]string)
	for _, crd := range cfg.CustomResources {
		crdPrefixMap[crd.Kind] = crd.LabelPrefix
	}

	return &crdProcessor{
		logger:          set.Logger,
		config:          cfg,
		tracesConsumer:  tracesConsumer,
		metricsConsumer: metricsConsumer,
		logsConsumer:    logsConsumer,
		crdPrefixMap:    crdPrefixMap,
	}, nil
}

// Start implements component.Component
func (p *crdProcessor) Start(ctx context.Context, host component.Host) error {
	p.logger.Info("Starting crdProcessor",
		zap.Int("crd_count", len(p.config.CustomResources)),
		zap.Duration("cache_ttl", p.config.CacheTTL),
		zap.Int("cache_max_size", p.config.CacheMaxSize),
	)

	// Initialize the Informer cache
	cache, err := NewInformerCache(
		p.logger,
		p.config.CustomResources,
		p.config.CacheTTL,
		p.config.CacheMaxSize,
	)
	if err != nil {
		return fmt.Errorf("failed to create informer cache: %w", err)
	}
	p.cache = cache

	// Start the informer
	if err := p.cache.Start(ctx); err != nil {
		return fmt.Errorf("failed to start informer cache: %w", err)
	}

	p.logger.Info("crdProcessor started successfully")
	return nil
}

// Shutdown implements component.Component
func (p *crdProcessor) Shutdown(ctx context.Context) error {
	p.logger.Info("Shutting down crdProcessor")
	if p.cache != nil {
		p.cache.Stop()
	}
	return nil
}

// Capabilities returns the processor capabilities
func (p *crdProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// ConsumeTraces processes trace data
func (p *crdProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)
		attrs := rs.Resource().Attributes()
		p.addCRDLabels(attrs)
	}
	return p.tracesConsumer.ConsumeTraces(ctx, td)
}

// ConsumeMetrics processes metric data
func (p *crdProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		attrs := rm.Resource().Attributes()
		p.addCRDLabels(attrs)
	}
	return p.metricsConsumer.ConsumeMetrics(ctx, md)
}

// ConsumeLogs processes log data
func (p *crdProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		attrs := rl.Resource().Attributes()
		p.addCRDLabels(attrs)
	}
	return p.logsConsumer.ConsumeLogs(ctx, ld)
}

// addCRDLabels adds CRD labels to resource attributes
func (p *crdProcessor) addCRDLabels(attrs pcommon.Map) {
	// Graceful passthrough if cache is not initialized
	if p.cache == nil {
		return
	}

	// Get k8s.replicaset.name from attributes
	rsNameVal, rsOk := attrs.Get("k8s.replicaset.name")
	if !rsOk {
		return
	}
	rsName := rsNameVal.Str()
	if rsName == "" {
		return
	}

	// Get k8s.namespace.name from attributes
	nsVal, nsOk := attrs.Get("k8s.namespace.name")
	if !nsOk {
		return
	}
	namespace := nsVal.Str()
	if namespace == "" {
		return
	}

	// Lookup owner from cache
	owner, found := p.cache.Get(namespace, rsName)
	if !found {
		if !p.config.PassthroughOnError {
			p.logger.Debug("Owner not found for ReplicaSet",
				zap.String("namespace", namespace),
				zap.String("replicaset", rsName),
			)
		}
		return
	}

	// Get label prefix for this CRD kind
	prefix, ok := p.crdPrefixMap[owner.Kind]
	if !ok {
		return
	}

	// Add CRD labels
	attrs.PutStr(fmt.Sprintf("%s.name", prefix), owner.Name)
	attrs.PutStr(fmt.Sprintf("%s.uid", prefix), owner.UID)

	p.logger.Debug("Added CRD labels",
		zap.String("kind", owner.Kind),
		zap.String("name", owner.Name),
		zap.String("namespace", namespace),
		zap.String("replicaset", rsName),
	)
}

// Copyright 2024 Wondermove Inc.
// SPDX-License-Identifier: Apache-2.0

package crdprocessor

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
)

const (
	// TypeStr is the unique identifier for the crdprocessor
	TypeStr = "crd"

	// Stability level of the processor
	stability = component.StabilityLevelDevelopment
)

// NewFactory creates a new processor factory
func NewFactory() processor.Factory {
	return processor.NewFactory(
		component.MustNewType(TypeStr),
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, stability),
		processor.WithMetrics(createMetricsProcessor, stability),
		processor.WithLogs(createLogsProcessor, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		CacheTTL:           60 * time.Second,
		CacheMaxSize:       10000,
		APITimeout:         10 * time.Second,
		PassthroughOnError: true,
		CustomResources: []CRDConfig{
			{
				Group:       "argoproj.io",
				Version:     "v1alpha1",
				Kind:        "Rollout",
				LabelPrefix: "k8s.rollout",
			},
		},
	}
}

func createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	return newCRDProcessor(set, cfg.(*Config), nextConsumer, nil, nil)
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	return newCRDProcessor(set, cfg.(*Config), nil, nextConsumer, nil)
}

func createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	return newCRDProcessor(set, cfg.(*Config), nil, nil, nextConsumer)
}
